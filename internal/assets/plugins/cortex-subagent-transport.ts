import { type Plugin } from "@opencode-ai/plugin";

/**
 * Isolated system instruction injected into child subagents to replace
 * inherited parent conversation history and prevent context bloat / role confusion.
 */
const TRANSPORT_ISOLATION_SYSTEM = [
  "You are an ephemeral OpenCode subagent operating under Cortex-IA work authority and Bounded Agent Contract.",
  "- Authority Invariant: You have no session lifecycle authority. NEVER call cortex_session_start, cortex_session_end, or cortex-ia board create.",
  "- File Leases: Before modifying any file with edit or write_to_file, you MUST hold an active file lease (cortex_ia_file_reserve). Stop immediately if authority expires.",
  "- Bounded Scope: Follow strictly the assigned <minion-contract> envelope and deliver a structured JSON receipt with phase_status, verification_verdict, and summary.",
  "- Contract SLA: Complete your objective within your allocated step and token budget. Do not loop repetitively without emitting verifiable progress.",
  "- Context Hygiene: Focus exclusively on your immediate objective; do not assume orchestrator routing duties."
].join("\n");

const ROLE_DEFAULT_STEPS: Record<string, number> = {
  implement: 70,
  discovery: 60,
  planner: 60,
  orchestrator: 60,
  reviewer: 50,
  investigate: 50,
};

const DEFAULT_MAX_STEPS = 60;

function validBudget(value: unknown): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value <= 0 || value > 1000) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: invalid step budget");
  }
  return value;
}

function dispatchBudget(args: Record<string, any>, prompt: string): number {
  const envelopes = [...prompt.matchAll(/<minion-dispatch>([\s\S]*?)<\/minion-dispatch>/g)];
  let envelope: Record<string, any> = {};
  if (envelopes.length > 1 ||
      (prompt.match(/<minion-dispatch/g) || []).length !== envelopes.length ||
      (prompt.match(/<\/minion-dispatch>/g) || []).length !== envelopes.length) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: ambiguous dispatch envelope");
  }
  if (envelopes.length) {
    try {
      envelope = JSON.parse(envelopes[0][1]);
      const budgetKeys = [...envelopes[0][1].matchAll(/("(?:\\.|[^"\\])*")\s*:/g)]
        .filter((match) => JSON.parse(match[1]) === "max_steps");
      if (!envelope || typeof envelope !== "object" || Array.isArray(envelope) ||
          budgetKeys.length > 1) throw new Error();
    } catch {
      throw new Error("SUBAGENT_TRANSPORT_ERROR: malformed dispatch envelope");
    }
  }
  const budgets = [];
  for (const [object, key] of [[args, "steps"], [args, "max_steps"], [envelope, "max_steps"]] as const) {
    if (Object.prototype.hasOwnProperty.call(object, key)) budgets.push(validBudget(object[key]));
  }
  if (budgets.length > 1) throw new Error("SUBAGENT_TRANSPORT_ERROR: ambiguous step budgets");
  const role = envelope.role ?? args.agent ?? args.subagent_type;
  const fallback = typeof role === "string" && Object.prototype.hasOwnProperty.call(ROLE_DEFAULT_STEPS, role)
    ? ROLE_DEFAULT_STEPS[role] : DEFAULT_MAX_STEPS;
  return budgets[0] ?? fallback;
}

// Admission only: the bridge still validates the retained claim/lease authority.
function isCleanup(tool: string, args: Record<string, any> | undefined): boolean {
  if (/^cortex_(?:ia_)?(?:file_release|work_release|work_release_all)$/.test(tool)) return true;
  return /^cortex_(?:ia_)?work_transition$/.test(tool) &&
    (args?.to === "in_review" || args?.to === "blocked");
}

/**
 * CortexSubagentTransportPlugin provides runtime transport isolation and contract SLA bounding
 * for OpenCode subagents. It ensures child sessions spawned via the task tool receive a sanitized
 * system prompt, eliminating conversational residue from the parent orchestrator and enforcing strict
 * step limits to prevent runaway tool loops.
 */
export const CortexSubagentTransportPlugin: Plugin = async (ctx) => {
  const childSessions = new Set<string>();
  const sessionStepCounts = new Map<string, number>();
  const sessionStepLimits = new Map<string, number>();
  type Start = { callID: string; limit: number; childID?: string; part?: string; invalid?: boolean; resume?: boolean };
  const pendingStarts = new Map<string, Start>();
  // Retain dispatch identities/counters as tombstones until disposal: after is not child completion.
  const starts = new Map<string, Map<string, Start>>();
  const parents = new Map<string, string>();
  const deleted = new Set<string>();
  const owners = new Map<string, Start>();
  const cleanupCounts = new Map<string, number>();
  const fail = () => { throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: missing or conflicting child identity"); };
  const register = (info: any) => {
    if (typeof info?.id !== "string" || deleted.has(info.id)) return;
    if (typeof info.parentID === "string" && info.parentID) {
      if (parents.has(info.id) && parents.get(info.id) !== info.parentID) { deleted.add(info.id); return; }
      parents.set(info.id, info.parentID);
      childSessions.add(info.id);
    }
  };
  const metadata = (part: any) => {
    if (part?.type !== "tool" || part.tool !== "task") return;
    const dispatches = starts.get(part.sessionID);
    if (!dispatches) return;
    // v1.18.29: normal session/tools uses callID; handleSubtask uses part.id.
    // Both are exact identities, never a parent-only fallback. Collisions fail closed.
    const matches = [...dispatches.values()].filter(s => s.callID === part.callID || s.callID === part.id);
    if (!matches.length) return;
    const data = part.state?.metadata;
    if (!data?.sessionId) return;
    const identity = JSON.stringify([part.messageID, part.id, part.callID]);
    const conflict = matches.length !== 1 || !["running", "completed"].includes(part.state?.status) || typeof data.sessionId !== "string" ||
      data.parentSessionId !== part.sessionID || !part.id || !part.messageID || !part.callID;
    for (const s of matches) {
      const owner = owners.get(data.sessionId);
      if (owner && owner !== s && !s.resume) owner.invalid = true;
      if (conflict || (owner && owner !== s && !s.resume) ||
          (s.part && s.part !== identity) || (s.childID && s.childID !== data.sessionId)) s.invalid = true;
      if (!s.invalid) { s.part = identity; s.childID = data.sessionId; }
    }
  };
  // Await SDK evidence, not event delivery (OpenCode invokes event hooks without awaiting).
  // Each read has a hard deadline and abort; late results never mutate transport state.
  const lookup = async (read: (signal: AbortSignal) => Promise<any>) => {
    const abort = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      const result = await Promise.race([Promise.resolve().then(() => read(abort.signal)),
        new Promise<never>((_, reject) => { timer = setTimeout(() => { abort.abort(); reject(new Error("identity lookup timeout")); }, 1000); })]);
      if (result?.error || !result?.data) return fail();
      return result.data;
    } catch { return fail(); } finally { clearTimeout(timer); }
  };
  const identify = async (id: string) => {
    if (deleted.has(id)) return fail();
    if (!parents.has(id)) {
      const info = await lookup(signal => ctx.client.session.get({ path: { id }, signal }));
      if (info.id !== id) return fail();
      register(info);
      if (!info.parentID && !childSessions.has(id)) return; // SDK-proven root, not an unknown child.
    }
    const parent = parents.get(id);
    if (!parent || deleted.has(id) || deleted.has(parent)) return fail();
    if (!owners.has(id)) {
      const messages = await lookup(signal => ctx.client.session.messages({ path: { id: parent }, query: { limit: 100 }, signal }));
      if (!Array.isArray(messages)) return fail();
      for (const message of messages) for (const part of message.parts ?? []) metadata(part);
    }
    // No awaits between association/recheck and the caller's synchronous admission counter.
    const candidates = [...(starts.get(parent)?.values() ?? [])].filter(s => s.childID === id && s.part);
    if (!candidates.length || candidates.some(s => s.invalid) || deleted.has(id) || deleted.has(parent)) return fail();
    const owner = owners.get(id);
    if (!owner) {
      if (candidates.length !== 1) return fail();
      owners.set(id, candidates[0]);
      sessionStepLimits.set(id, candidates[0].limit);
    }
  };

  return {
    dispose: async () => {
      childSessions.clear();
      sessionStepCounts.clear();
      sessionStepLimits.clear();
      pendingStarts.clear();
      starts.clear(); parents.clear(); deleted.clear(); owners.clear();
      cleanupCounts.clear();
    },

    event: async ({ event }) => {
      if (!event) return;
      const type = event.type || event.event || "";
      const info = (event.properties as any)?.info;

      // Register child subagent sessions spawned with a parentID
      if (type === "session.created" && info?.parentID && info?.id) {
        register(info); // Creation proves ancestry only; it grants no allowance.
      } else if (type === "message.part.updated") {
        metadata((event.properties as any)?.part);
      } else if (type === "session.deleted") {
        const id = info?.id;
        if (id) {
          deleted.add(id);
          pendingStarts.delete(id);
        }
      }
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (!input?.sessionID || !output?.system) return;
      
      // If this session is an identified child subagent, replace inherited system instructions
      // with the sanitized Cortex-IA subagent isolation prompt.
      if (childSessions.has(input.sessionID)) {
        output.system.splice(0, output.system.length, TRANSPORT_ISOLATION_SYSTEM);
      }
    },

    "tool.execute.before": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      const sessionID = input?.sessionID || "";
      if (!sessionID) return fail();
      await identify(sessionID);
      // Async event callbacks can run at the await boundary: recheck before charging.
      if (deleted.has(sessionID) || deleted.has(parents.get(sessionID) ?? "") ||
          owners.get(sessionID)?.invalid || [...(starts.get(parents.get(sessionID) ?? "")?.values() ?? [])]
            .some(s => s.childID === sessionID && s.invalid)) return fail();

      // Charge all ordinary tools, including nested task dispatches, before dispatch handling.
      if (sessionID && childSessions.has(sessionID)) {
        const count = sessionStepCounts.get(sessionID) ?? 0;
        const limit = sessionStepLimits.get(sessionID) ?? 0;
        if (count >= limit) {
          const cleanup = cleanupCounts.get(sessionID) ?? 0;
          if (cleanup < 5 && isCleanup(toolName, output?.args)) {
            cleanupCounts.set(sessionID, cleanup + 1);
            return;
          }
          throw new Error(`AGENT_CONTRACT_EXCEEDED: subagent reached step budget limit (${limit}) without delivering completion receipt`);
        }
        sessionStepCounts.set(sessionID, count + 1);
      }

      // 1. Task dispatch gate: enforce non-empty prompt and detect contract limits
      if (toolName === "task") {
        const args = (output?.args || {}) as Record<string, any>;
        const prompt = args?.prompt || args?.description || "";
        if (typeof prompt !== "string" || prompt.trim().length === 0) {
          throw new Error("SUBAGENT_TRANSPORT_ERROR: task dispatch requires a non-empty prompt or envelope");
        }

        const stepBudget = dispatchBudget(args, prompt);
        if (!sessionID || !input.callID || pendingStarts.has(sessionID) || starts.get(sessionID)?.has(input.callID)) {
          throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: task requires a serialized parent and callID");
        }
        const resume = args.task_id || args.session_id;
        if (resume && (typeof resume !== "string" || !owners.has(resume) || parents.get(resume) !== sessionID || deleted.has(resume))) {
          throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: unknown resume identity");
        }
        const start: Start = { callID: input.callID, limit: stepBudget, childID: resume || undefined, resume: !!resume };
        if (!starts.has(sessionID)) starts.set(sessionID, new Map());
        starts.get(sessionID)!.set(input.callID, start);
        pendingStarts.set(sessionID, start);
      }
    },

    "tool.execute.after": async (input) => {
      if (input.tool !== "task") return;
      const start = pendingStarts.get(input.sessionID);
      if (start?.callID === input.callID) pendingStarts.delete(input.sessionID);
    }
  };
};

export default CortexSubagentTransportPlugin;

