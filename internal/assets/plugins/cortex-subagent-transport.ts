import { type Plugin } from "@opencode-ai/plugin";
import { createHash } from "node:crypto";

/**
 * Supplemental child scope instructions. Host system and role instructions retain
 * their authority; this hook cannot remove inherited conversation history.
 */
const TRANSPORT_ISOLATION_SYSTEM = [
  "You are an ephemeral OpenCode subagent operating under Cortex-IA work authority and Bounded Agent Contract.",
  "Host policy and role authority govern all operations; no subagent gains orchestrator delegation authority.",
  "Live claims and path leases are required before modifying files; stop writing on conflict or expiry. Never persist tokens or secrets.",
  "Follow the assigned dispatch envelope and emit the canonical completion receipt. Direct routing avoids unassigned duties; reconcile prior jobs before reattempting."
].join("\n");

const ROLE_DEFAULT_STEPS: Record<string, number> = {
  implement: 70,
  discovery: 60,
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

function policyInteger(name: string, fallback: number, minimum: number, maximum: number): number {
  const raw = process.env[name];
  if (raw === undefined) return fallback;
  if (!/^[1-9][0-9]*$/.test(raw) || !Number.isSafeInteger(Number(raw)) || Number(raw) < minimum || Number(raw) > maximum) {
    throw new Error(`SUBAGENT_TRANSPORT_CONFIG: ${name} must be an integer from ${minimum} to ${maximum}`);
  }
  return Number(raw);
}

// Canonical, bounded transient serialization. Retain only the digest, never tool data.
function outcomeFingerprint(tool: string, args: unknown, status: string, result: unknown): string | undefined {
  let remaining = 1024 * 1024;
  const canonical = (value: any, depth = 0): string => {
    if (depth > 32 || --remaining < 0) throw new Error();
    if (typeof value === "string" && value.length > remaining) throw new Error();
    if (value === null || typeof value !== "object") {
      const text = JSON.stringify(value) ?? "null";
      remaining -= text.length;
      if (remaining < 0) throw new Error();
      return text;
    }
    if (Array.isArray(value)) return `[${value.map(item => canonical(item, depth + 1)).join(",")}]`;
    return `{${Object.keys(value).sort().map(key => `${canonical(key, depth + 1)}:${canonical(value[key], depth + 1)}`).join(",")}}`;
  };
  try { return createHash("sha256").update(canonical([tool, args, status, result])).digest("hex"); }
  catch { return undefined; } // Oversized/unserializable outcomes cannot prove repetition.
}

function validateNoDuplicateKeys(jsonString: string): void {
  const stack: Set<string>[] = [];
  let inString = false;
  let escape = false;
  let stringStart = -1;
  let lastString = "";
  let expectColon = false;

  for (let i = 0; i < jsonString.length; i++) {
    const char = jsonString[i];
    if (inString) {
      if (escape) {
        escape = false;
      } else if (char === "\\") {
        escape = true;
      } else if (char === '"') {
        inString = false;
        const rawString = jsonString.slice(stringStart, i + 1);
        try {
          lastString = JSON.parse(rawString);
        } catch {
          lastString = rawString;
        }
        expectColon = true;
      }
      continue;
    }

    if (char === '"') {
      inString = true;
      escape = false;
      stringStart = i;
      continue;
    }

    if (char === "{") {
      stack.push(new Set());
      expectColon = false;
    } else if (char === "}") {
      stack.pop();
      expectColon = false;
    } else if (char === ":") {
      if (expectColon && stack.length > 0) {
        const currentSet = stack[stack.length - 1];
        if (currentSet.has(lastString)) {
          throw new Error(`SUBAGENT_TRANSPORT_ERROR: duplicate decoded key in dispatch envelope: ${lastString}`);
        }
        currentSet.add(lastString);
        expectColon = false;
      }
    } else if (!/\s/.test(char)) {
      expectColon = false;
    }
  }
}

function parseBudgetInteger(raw: unknown, source: string): number {
  if (typeof raw !== "number" || !Number.isSafeInteger(raw) || raw <= 0 || raw > 1000) {
    throw new Error(`SUBAGENT_TRANSPORT_ERROR: invalid step budget integer in ${source}`);
  }
  return raw;
}

function parseBudgetObject(obj: unknown, source: string): number {
  if (typeof obj !== "object" || obj === null || Array.isArray(obj)) {
    throw new Error(`SUBAGENT_TRANSPORT_ERROR: malformed budget object in ${source}`);
  }
  const rec = obj as Record<string, any>;
  const hasMaxTurns = Object.prototype.hasOwnProperty.call(rec, "max_turns");
  const hasMaxSteps = Object.prototype.hasOwnProperty.call(rec, "max_steps");
  if (!hasMaxTurns && !hasMaxSteps) {
    throw new Error(`SUBAGENT_TRANSPORT_ERROR: missing budget count in ${source}`);
  }
  if (hasMaxTurns && hasMaxSteps) {
    throw new Error(`SUBAGENT_TRANSPORT_ERROR: ambiguous budget object in ${source}; multiple forms even equal are rejected`);
  }
  const val = hasMaxTurns ? rec.max_turns : rec.max_steps;
  return parseBudgetInteger(val, `${source}.${hasMaxTurns ? "max_turns" : "max_steps"}`);
}

function extractBudget(args: Record<string, any>, envelope: Record<string, any>): number | undefined {
  const budgetList: { source: string; value: number }[] = [];

  if (Object.prototype.hasOwnProperty.call(args, "steps")) {
    budgetList.push({ source: "args.steps", value: parseBudgetInteger(args.steps, "args.steps") });
  }
  if (Object.prototype.hasOwnProperty.call(args, "max_steps")) {
    budgetList.push({ source: "args.max_steps", value: parseBudgetInteger(args.max_steps, "args.max_steps") });
  }
  if (Object.prototype.hasOwnProperty.call(args, "budget")) {
    budgetList.push({ source: "args.budget", value: parseBudgetObject(args.budget, "args.budget") });
  }
  if (Object.prototype.hasOwnProperty.call(envelope, "max_steps")) {
    budgetList.push({ source: "envelope.max_steps", value: parseBudgetInteger(envelope.max_steps, "envelope.max_steps") });
  }
  if (Object.prototype.hasOwnProperty.call(envelope, "steps")) {
    budgetList.push({ source: "envelope.steps", value: parseBudgetInteger(envelope.steps, "envelope.steps") });
  }
  if (Object.prototype.hasOwnProperty.call(envelope, "budget")) {
    budgetList.push({ source: "envelope.budget", value: parseBudgetObject(envelope.budget, "envelope.budget") });
  }

  if (budgetList.length > 1) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: ambiguous step budgets; multiple forms even equal are rejected");
  }

  return budgetList[0]?.value;
}

function dispatchInfo(args: Record<string, any>, prompt: string): { limit: number; operational: boolean } {
  const envelopes = [...prompt.matchAll(/<minion-(dispatch|contract)>([\s\S]*?)<\/minion-\1>/g)];
  let envelope: Record<string, any> = {};
  if (envelopes.length > 1 ||
      (prompt.match(/<minion-(?:dispatch|contract)/g) || []).length !== envelopes.length ||
      (prompt.match(/<\/minion-(?:dispatch|contract)>/g) || []).length !== envelopes.length) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: ambiguous dispatch envelope");
  }
  if (envelopes.length) {
    try {
      validateNoDuplicateKeys(envelopes[0][2]);
      envelope = JSON.parse(envelopes[0][2]);
      if (!envelope || typeof envelope !== "object" || Array.isArray(envelope)) throw new Error();
    } catch (e: any) {
      if (e.message?.startsWith("SUBAGENT_TRANSPORT_ERROR:")) throw e;
      throw new Error("SUBAGENT_TRANSPORT_ERROR: malformed dispatch envelope");
    }
  }
  // Versioned dispatches carry the common routing contract. Old persisted task
  // inputs remain resumable; supplied routing fields must still be consistent.
  if (envelope.role !== undefined && envelope.role !== args.subagent_type) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch role does not match host task target");
  }
  if (args.agent !== undefined && args.agent !== args.subagent_type) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch agent does not match host task target");
  }

  let isOperational = false;
  const role = envelope.role ?? args.agent ?? args.subagent_type;
  if (role === "implement" && Array.isArray(envelope.allowed_files) && envelope.allowed_files.length === 0) {
    const hasTarget = Boolean(envelope.operational_target || envelope.target);
    const hasEffects = Boolean(
      (Array.isArray(envelope.operational_effects) && envelope.operational_effects.length > 0) ||
      (typeof envelope.operational_effects === "string" && envelope.operational_effects.trim()) ||
      (Array.isArray(envelope.target_effects) && envelope.target_effects.length > 0) ||
      (typeof envelope.target_effects === "string" && envelope.target_effects.trim())
    );
    const isOps = (envelope.workflow === "ops-task" || envelope.operation_type === "database" || envelope.operation_type === "ops" || envelope.is_operational === true) &&
      hasTarget && hasEffects;

    if (isOps) {
      if (envelope.execution_mode === "external" || envelope.workspace_strategy === "isolated_worktree" || args.delegated === true) {
        throw new Error("SUBAGENT_TRANSPORT_ERROR: external operational execution is unavailable; requires fresh orchestrator-native dispatch");
      }
      if (envelope.task_id === null || envelope.authority === false || envelope.authority_available === false || args.authority === false || args.authority_available === false) {
        throw new Error("SUBAGENT_TRANSPORT_ERROR: operational implementation requires valid task authority");
      }
      isOperational = true;
    } else {
      throw new Error("SUBAGENT_TRANSPORT_ERROR: implementation requires an explicit file scope or authorized operational target and effects");
    }
  }

  if (envelope.contract_version !== undefined) {
    if (envelope.contract_version !== "1.0" ||
        !["discovery", "investigate", "planner", "implement", "reviewer"].includes(envelope.role) ||
        !["workflow", "phase", "objective"].every(key => typeof envelope[key] === "string" && envelope[key].trim()) ||
        !(envelope.task_id === null || (typeof envelope.task_id === "string" && envelope.task_id.trim())) ||
        ![null, "openspec", "cortex", "hybrid"].includes(envelope.spec_plane) ||
        !["allowed_files", "acceptance_checks", "artifact_refs"].every(key => Array.isArray(envelope[key]) && envelope[key].every((item: unknown) => typeof item === "string"))) {
      throw new Error("SUBAGENT_TRANSPORT_ERROR: invalid common dispatch contract");
    }
    if (envelope.role === "planner") {
      const phases: Record<string, string[]> = {
        "decision-map": ["chart", "resolve"],
        "sdd-lite": ["integrated", "archive", "decompose"],
        "sdd-full": ["propose", "spec", "design", "tasks", "archive", "decompose"],
      };
      if (!phases[envelope.workflow]?.includes(envelope.phase) || envelope.spec_plane === null) {
        const allowed = phases[envelope.workflow]?.join(", ") ?? "known: decision-map, sdd-lite, sdd-full";
        throw new Error(`SUBAGENT_TRANSPORT_ERROR: invalid planning workflow (${envelope.workflow}), phase (${envelope.phase}; allowed: [${allowed}]), or specification plane (${envelope.spec_plane}; cannot be null)`);
      }
    }
  }

  const explicitBudget = extractBudget(args, envelope);
  const hostRole = args.subagent_type;
  if (hostRole === "planner") return { limit: Number.POSITIVE_INFINITY, operational: isOperational };
  const fallback = typeof hostRole === "string" && Object.prototype.hasOwnProperty.call(ROLE_DEFAULT_STEPS, hostRole)
    ? ROLE_DEFAULT_STEPS[hostRole] : DEFAULT_MAX_STEPS;
  return { limit: explicitBudget ?? fallback, operational: isOperational };
}

// Admission only: the bridge still validates the retained claim/lease authority.
function isCleanup(tool: string, args: Record<string, any> | undefined): boolean {
  if (/^cortex_(?:ia_)?(?:file_release|work_release|work_release_all)$/.test(tool)) return true;
  return /^cortex_(?:ia_)?work_transition$/.test(tool) &&
    (args?.to === "in_review" || args?.to === "blocked");
}

/**
 * CortexSubagentTransportPlugin provides runtime transport isolation and contract SLA bounding
 * for OpenCode subagents. It supplements host system instructions with child scope
 * guidance, advisory budgets, and a configurable emergency ceiling.
 */
export const CortexSubagentTransportPlugin: Plugin = async (ctx) => {
  const emergencySteps = policyInteger("CORTEX_IA_EMERGENCY_STEPS", 500, 1, 100000);
  const repetitionLimit = policyInteger("CORTEX_IA_REPETITION_LIMIT", 5, 2, 100);
  let disposed = false;
  const childSessions = new Set<string>();
  const sessionStepCounts = new Map<string, number>();
  const sessionStepLimits = new Map<string, number>();
  const operationalSessions = new Set<string>();
  type Start = { callID: string; limit: number; childID?: string; part?: string; invalid?: boolean; resume?: boolean; operational?: boolean };
  // Retain dispatch identities/counters as tombstones until disposal: after is not child completion.
  const starts = new Map<string, Map<string, Start>>();
  const parents = new Map<string, string>();
  const deleted = new Set<string>();
  const owners = new Map<string, Start>();
  const cleanupCounts = new Map<string, number>();
  const restorations = new Map<string, Promise<void>>();
  const restoredPending = new Map<string, Set<string>>();
  const restoredCleanupPending = new Map<string, Set<string>>();
  const activeCalls = new Map<string, Set<string>>();
  type Progress = { seen: Set<string>; last?: string; streak: number };
  const progress = new Map<string, Progress>();
  const terminal = (id: string, callID: string, tool: string, args: unknown, status: string, result: unknown) => {
    if (!childSessions.has(id) || !callID || !["completed", "error"].includes(status)) return;
    let state = progress.get(id);
    if (!state) { state = { seen: new Set(), streak: 0 }; progress.set(id, state); }
    const callKey = createHash("sha256").update(callID).digest("hex");
    if (state.seen.has(callKey)) return;
    state.seen.add(callKey);
    // Bound recent-event deduplication memory even for unlimited planner sessions.
    if (state.seen.size > 10000) state.seen.delete(state.seen.values().next().value!);
    activeCalls.get(id)?.delete(callID);
    restoredPending.get(id)?.delete(callID);
    restoredCleanupPending.get(id)?.delete(callID);
    const fingerprint = outcomeFingerprint(tool, args, status, result);
    state.streak = fingerprint && fingerprint === state.last ? state.streak + 1 : fingerprint ? 1 : 0;
    state.last = fingerprint;
  };
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
    const conflict = matches.length !== 1 || !["running", "completed", "error"].includes(part.state?.status) || typeof data.sessionId !== "string" ||
      data.parentSessionId !== part.sessionID || !part.id || !part.messageID || !part.callID;
    for (const s of matches) {
      const owner = owners.get(data.sessionId);
      if (owner && owner !== s && !s.resume) owner.invalid = true;
      if (conflict || (owner && owner !== s && !s.resume) ||
          (s.part && s.part !== identity) || (s.childID && s.childID !== data.sessionId)) s.invalid = true;
      if (!s.invalid) {
        s.part = identity;
        s.childID = data.sessionId;
        if (s.operational) operationalSessions.add(data.sessionId);
      }
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
  const resumeFailure = () => {
    throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: cannot verify complete resume identity and budget; explicitly dispatch a fresh bounded task without task_id/session_id after reconciling prior work");
  };
  // The host returns at most limit messages, with no implicit lower cap. Asking
  // for one extra proves completeness without relying on SDK cursor support.
  const history = async (id: string) => {
    const messages = await lookup(signal => ctx.client.session.messages({ path: { id }, query: { limit: 1001 }, signal }));
    if (!Array.isArray(messages) || messages.length > 1000 || JSON.stringify(messages).length > 4 * 1024 * 1024) return resumeFailure();
    let count = 0;
    const ids = new Set<string>();
    for (const message of messages) {
      if (!message.info?.id || message.info.sessionID !== id || !Array.isArray(message.parts)) return resumeFailure();
      count += message.parts.length;
      if (count > 10000) return resumeFailure();
      for (const part of message.parts) {
        if (!part.id || ids.has(part.id) || part.sessionID !== id || part.messageID !== message.info.id) return resumeFailure();
        ids.add(part.id);
      }
    }
    return messages;
  };
  const restore = (id: string, parent: string): Promise<void> => {
    const pending = restorations.get(id);
    if (pending) return pending;
    const run = (async () => {
      const info = await lookup(signal => ctx.client.session.get({ path: { id }, signal }));
      if (info.id !== id || info.parentID !== parent || info.revert) return resumeFailure();
      const [parentHistory, childHistory] = await Promise.all([history(parent), history(id)]);
      const links = parentHistory.flatMap(message => message.info.role === "assistant" ? message.parts : []).filter(part =>
        part.type === "tool" && part.tool === "task" && part.state?.metadata?.sessionId === id);
      if (links.some(part => !part.callID || part.state.metadata.parentSessionId !== parent ||
          !["running", "completed", "error"].includes(part.state.status) || !part.state.input ||
          (part.state.input.task_id && part.state.input.task_id !== id) ||
          (part.state.input.session_id && part.state.input.session_id !== id))) return resumeFailure();
      const originals = links.filter(part => !part.state.input.task_id && !part.state.input.session_id);
      if (originals.length !== 1) return resumeFailure();
      const original = originals[0];
      const prompt = original.state.input.prompt || original.state.input.description;
      if (typeof prompt !== "string" || !prompt.trim()) return resumeFailure();
      const { limit, operational: isOperational } = dispatchInfo(original.state.input, prompt);
      const identity = JSON.stringify([original.messageID, original.id, original.callID]);
      const attempts = childHistory.flatMap(message => message.parts).filter(part => part.type === "tool");
      if (attempts.some(part => !part.callID || !["pending", "running", "completed", "error"].includes(part.state?.status))) return resumeFailure();
      if (new Set(attempts.map(part => part.callID)).size !== attempts.length) return resumeFailure();
      const ceiling = Number.isFinite(limit) ? emergencySteps : Infinity;
      const usedCleanup = attempts.slice(ceiling).filter(part => isCleanup(part.tool, part.state.input)).length;
      // All awaits are finished. Never overwrite newer live authority/counters.
      if (disposed || deleted.has(id) || deleted.has(parent) || (parents.has(id) && parents.get(id) !== parent)) return resumeFailure();
      const existing = owners.get(id);
      if (existing?.invalid || (existing && existing.part !== identity) ||
          [...(starts.get(parent)?.values() ?? [])].some(start => start.childID === id && start.invalid)) return resumeFailure();
      const start = existing ?? { callID: original.callID, limit, childID: id, part: identity, operational: isOperational };
      const collision = starts.get(parent)?.get(original.callID);
      if (collision && collision !== existing && (collision.part !== identity || collision.invalid)) return resumeFailure();
      if (!starts.has(parent)) starts.set(parent, new Map());
      starts.get(parent)!.set(original.callID, start);
      parents.set(id, parent);
      childSessions.add(id);
      owners.set(id, start);
      if (start.operational || isOperational) operationalSessions.add(id);
      sessionStepLimits.set(id, existing?.limit ?? limit);
      sessionStepCounts.set(id, Math.max(sessionStepCounts.get(id) ?? 0, attempts.length));
      cleanupCounts.set(id, Math.max(cleanupCounts.get(id) ?? 0, usedCleanup));
      if (!restoredCleanupPending.has(id)) restoredCleanupPending.set(id, new Set());
      for (const part of attempts.slice(ceiling)) {
        if (part.state.status === "pending" && isCleanup(part.tool, part.state.input)) restoredCleanupPending.get(id)!.add(part.callID);
      }
      if (!restoredPending.has(id)) restoredPending.set(id, new Set());
      for (const part of attempts) {
        if (part.state.status === "pending") restoredPending.get(id)!.add(part.callID);
        terminal(id, part.callID, part.tool, part.state.input, part.state.status,
          part.state.status === "error" ? part.state.error : part.state.output);
      }
    })();
    restorations.set(id, run);
    void run.finally(() => { if (restorations.get(id) === run) restorations.delete(id); }).catch(() => {});
    return run;
  };
  const identify = async (id: string) => {
    if (disposed || deleted.has(id)) return fail();
    if (!parents.has(id)) {
      const info = await lookup(signal => ctx.client.session.get({ path: { id }, signal }));
      if (disposed || info.id !== id || deleted.has(id)) return fail();
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
    let candidates = [...(starts.get(parent)?.values() ?? [])].filter(s => s.childID === id && s.part);
    if (!candidates.length) {
      await restore(id, parent);
      candidates = [...(starts.get(parent)?.values() ?? [])].filter(s => s.childID === id && s.part);
    }
    if (disposed || !candidates.length || candidates.some(s => s.invalid) || deleted.has(id) || deleted.has(parent)) return fail();
    const owner = owners.get(id);
    if (!owner) {
      if (candidates.length !== 1) return fail();
      owners.set(id, candidates[0]);
      sessionStepLimits.set(id, candidates[0].limit);
      if (candidates[0].operational) operationalSessions.add(id);
    } else if (owner.operational) {
      operationalSessions.add(id);
    }
  };

  return {
    dispose: async () => {
      disposed = true;
      childSessions.clear();
      sessionStepCounts.clear();
      sessionStepLimits.clear();
      operationalSessions.clear();
      starts.clear(); parents.clear(); deleted.clear(); owners.clear();
      cleanupCounts.clear();
      restorations.clear();
      restoredPending.clear(); restoredCleanupPending.clear(); activeCalls.clear(); progress.clear();
    },

    event: async ({ event }) => {
      if (disposed || !event) return;
      const type = event.type || event.event || "";
      const info = (event.properties as any)?.info;

      // Register child subagent sessions spawned with a parentID
      if (type === "session.created" && info?.parentID && info?.id) {
        register(info); // Creation proves ancestry only; it grants no allowance.
      } else if (type === "message.part.updated") {
        const part = (event.properties as any)?.part;
        metadata(part);
        if (part?.type === "tool") terminal(part.sessionID, part.callID, part.tool,
          part.state?.input, part.state?.status, part.state?.status === "error" ? part.state.error : part.state?.output);
      } else if (type === "session.deleted") {
        const id = info?.id;
        if (id) {
          deleted.add(id);
        }
      }
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (!input?.sessionID || !output?.system) return;
      
      // Preserve host policy, role instructions, and other plugins' system content.
      if (childSessions.has(input.sessionID)) {
        for (let index = output.system.length - 1; index >= 0; index--) {
          if (output.system[index].startsWith("CORTEX_BUDGET_WARNING:") || output.system[index].startsWith("CORTEX_REPETITION_WARNING:")) output.system.splice(index, 1);
        }
        if (!output.system.includes(TRANSPORT_ISOLATION_SYSTEM)) output.system.push(TRANSPORT_ISOLATION_SYSTEM);
        const count = sessionStepCounts.get(input.sessionID) ?? 0;
        const budget = sessionStepLimits.get(input.sessionID) ?? Infinity;
        if (count >= budget) output.system.push(`CORTEX_BUDGET_WARNING: advisory tool budget ${budget} reached (${count} attempts). Continue when useful; assess remaining scope and report partial progress if needed. This warning does not change task authority.`);
        const streak = progress.get(input.sessionID)?.streak ?? 0;
        if (streak >= repetitionLimit) output.system.push(`CORTEX_REPETITION_WARNING: ${streak} consecutive terminal tools had identical tool, arguments, status and result. This detects observable repetition, not semantic lack of progress; polling can be legitimate. Reassess the approach and report progress.`);
      }
    },

    "tool.execute.after": async (input, output) => {
      if (!disposed) terminal(input.sessionID, input.callID, input.tool, input.args, "completed", output.output);
    },

    "tool.execute.before": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      const sessionID = input?.sessionID || "";
      if (!sessionID) return fail();
      await identify(sessionID);
      // Async event callbacks can run at the await boundary: recheck before charging.
      if (disposed || deleted.has(sessionID) || deleted.has(parents.get(sessionID) ?? "") ||
          owners.get(sessionID)?.invalid || [...(starts.get(parents.get(sessionID) ?? "")?.values() ?? [])]
            .some(s => s.childID === sessionID && s.invalid)) return fail();

      // Guard: operational subagents cannot execute repository write tools
      if (operationalSessions.has(sessionID)) {
        if (["edit", "write_to_file", "write", "apply_patch"].includes(toolName)) {
          throw new Error("SUBAGENT_TRANSPORT_ERROR: repository write tools are forbidden for operational tasks; operations execute against target systems only");
        }
      }

      // Charge all ordinary tools, including nested task dispatches, before dispatch handling.
      if (sessionID && childSessions.has(sessionID)) {
        if (!input.callID || activeCalls.get(sessionID)?.has(input.callID)) return fail();
        const historicalPending = restoredPending.get(sessionID)?.has(input.callID) ?? false;
        const callKey = createHash("sha256").update(input.callID).digest("hex");
        if (progress.get(sessionID)?.seen.has(callKey)) return fail();
        const count = sessionStepCounts.get(sessionID) ?? 0;
        const limit = sessionStepLimits.get(sessionID) ?? 0;
        const ceiling = Number.isFinite(limit) ? emergencySteps : Infinity;
        if (count - (historicalPending ? 1 : 0) >= ceiling) {
          const cleanup = cleanupCounts.get(sessionID) ?? 0;
          const cleanupReserved = restoredCleanupPending.get(sessionID)?.has(input.callID) ?? false;
          if (cleanup - (cleanupReserved ? 1 : 0) < 5 && isCleanup(toolName, output?.args)) {
            cleanupCounts.set(sessionID, cleanup + (cleanupReserved ? 0 : 1));
            restoredCleanupPending.get(sessionID)?.delete(input.callID);
            restoredPending.get(sessionID)?.delete(input.callID);
            if (!activeCalls.has(sessionID)) activeCalls.set(sessionID, new Set());
            activeCalls.get(sessionID)!.add(input.callID);
            sessionStepCounts.set(sessionID, count + (historicalPending ? 0 : 1));
            return;
          }
          throw new Error(`AGENT_EMERGENCY_LIMIT: reached ${ceiling} tool attempts. Return a partial receipt and reconcile prior work before an explicitly authorized continuation; task status is unchanged. Up to five cleanup tools remain available.`);
        }
        restoredPending.get(sessionID)?.delete(input.callID);
        if (!activeCalls.has(sessionID)) activeCalls.set(sessionID, new Set());
        activeCalls.get(sessionID)!.add(input.callID);
        sessionStepCounts.set(sessionID, count + (historicalPending ? 0 : 1));
      }

      // 1. Task dispatch gate: enforce non-empty prompt and detect contract limits
      if (toolName === "task") {
        const args = (output?.args || {}) as Record<string, any>;
        const prompt = args?.prompt || args?.description || "";
        if (typeof prompt !== "string" || prompt.trim().length === 0) {
          throw new Error("SUBAGENT_TRANSPORT_ERROR: task dispatch requires a non-empty prompt or envelope");
        }

        const { limit: stepBudget, operational: isOperational } = dispatchInfo(args, prompt);
        if (!sessionID || !input.callID || starts.get(sessionID)?.has(input.callID)) {
          throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: task requires a unique parent and callID pair");
        }
        if (args.task_id && args.session_id && args.task_id !== args.session_id) return resumeFailure();
        const resume = args.task_id || args.session_id;
        if (resume) {
          if (typeof resume !== "string" || resume === sessionID) return resumeFailure();
          await identify(resume);
          if (!owners.has(resume) || parents.get(resume) !== sessionID || deleted.has(resume) ||
              deleted.has(sessionID) || owners.get(resume)?.invalid) return resumeFailure();
          if (starts.get(sessionID)?.has(input.callID)) return fail();
        }
        const start: Start = {
          callID: input.callID,
          limit: resume ? sessionStepLimits.get(resume)! : stepBudget,
          childID: resume || undefined,
          resume: !!resume,
          operational: resume ? operationalSessions.has(resume) : isOperational,
        };
        if (!starts.has(sessionID)) starts.set(sessionID, new Map());
        starts.get(sessionID)!.set(input.callID, start);
      }
    }
  };
};

export { CortexSubagentTransportPlugin, dispatchInfo };
export default CortexSubagentTransportPlugin;
