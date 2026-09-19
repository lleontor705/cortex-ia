// OpenCode Plugin helper ensuring default export is a valid plugin definition object for v1 and v2
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): T => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  },
};

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

function dispatchInfo(args: Record<string, any>, prompt: string): { limit: number; operational: boolean; prompt: string } {
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
  const hostTarget = args.subagent_type ?? args.agent ?? args.subagent;
  if (args.subagent_type !== undefined && args.agent !== undefined && args.agent !== args.subagent_type) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch agent does not match host task target");
  }
  if (args.subagent_type !== undefined && args.subagent !== undefined && args.subagent !== args.subagent_type) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch agent does not match host task target");
  }
  if (args.agent !== undefined && args.subagent !== undefined && args.agent !== args.subagent) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch agent does not match host task target");
  }

  if (envelope.role !== undefined && hostTarget !== undefined && envelope.role !== hostTarget) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: dispatch role does not match host task target");
  }

  const role = envelope.role ?? hostTarget;
  if (role === "implement" && Array.isArray(envelope.allowed_files) && envelope.allowed_files.length === 0) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: implementation requires a non-empty file scope; external-effect authority is unavailable, route read-only operations to investigate or reviewer");
  }

  if (envelope.contract_version !== undefined) {
    if (typeof envelope.contract_version === "number") {
      envelope.contract_version = (envelope.contract_version as number).toFixed(1);
    } else if (typeof envelope.contract_version === "string") {
      const clean = envelope.contract_version.trim().toLowerCase().replace(/^v/, "");
      if (clean === "1" || clean.startsWith("1.")) envelope.contract_version = "1.0";
      else if (clean === "2" || clean.startsWith("2.")) envelope.contract_version = "2.0";
    }
    if (typeof envelope.role === "string") {
      envelope.role = envelope.role.trim().toLowerCase();
    }
    if (envelope.role === undefined && hostTarget !== undefined) {
      envelope.role = typeof hostTarget === "string" ? hostTarget.trim().toLowerCase() : hostTarget;
    }
    if (
      envelope.task_id === undefined ||
      envelope.task_id === "none" ||
      envelope.task_id === "null" ||
      envelope.task_id === "N/A" ||
      (typeof envelope.task_id === "string" && !envelope.task_id.trim())
    ) {
      envelope.task_id = null;
    } else if (typeof envelope.task_id === "number") {
      envelope.task_id = String(envelope.task_id);
    } else if (typeof envelope.task_id === "string") {
      envelope.task_id = envelope.task_id.trim();
    }
    if (
      envelope.spec_plane === undefined ||
      envelope.spec_plane === "" ||
      envelope.spec_plane === "none" ||
      envelope.spec_plane === "null" ||
      envelope.spec_plane === "undefined" ||
      envelope.spec_plane === false
    ) {
      envelope.spec_plane = null;
    } else if (typeof envelope.spec_plane === "string") {
      envelope.spec_plane = envelope.spec_plane.trim().toLowerCase();
    }
    if (!envelope.workflow || typeof envelope.workflow !== "string" || !envelope.workflow.trim()) {
      envelope.workflow = envelope.role === "planner" ? "sdd-lite" : (envelope.role || hostTarget || "investigate");
    } else {
      envelope.workflow = envelope.workflow.trim().toLowerCase();
    }
    if (!envelope.phase || typeof envelope.phase !== "string" || !envelope.phase.trim()) {
      envelope.phase = envelope.role === "investigate" ? "diagnose"
        : envelope.role === "reviewer" ? "verify"
        : envelope.role === "discovery" ? "profile"
        : envelope.role === "planner" ? "integrated"
        : "execute";
    } else {
      envelope.phase = envelope.phase.trim().toLowerCase();
    }

    if (!envelope.objective || typeof envelope.objective !== "string" || !envelope.objective.trim()) {
      const candidate = envelope.description ?? envelope.task ?? envelope.goal ?? args.description ?? args.objective ?? args.task ?? args.goal;
      if (typeof candidate === "string" && candidate.trim()) {
        envelope.objective = candidate.trim();
      } else {
        const promptWithoutEnvelope = prompt.replace(/<minion-(?:dispatch|contract)>[\s\S]*?<\/minion-(?:dispatch|contract)>/g, "").trim();
        envelope.objective = promptWithoutEnvelope ? promptWithoutEnvelope.slice(0, 200).trim() : `Execute ${envelope.role || hostTarget || "unspecified"} objective`;
      }
    } else {
      envelope.objective = envelope.objective.trim();
    }

    const toStringArray = (val: unknown): string[] => {
      if (Array.isArray(val)) {
        return val
          .map(item => (typeof item === "string" ? item.trim() : String(item ?? "").trim()))
          .filter(item => item.length > 0);
      }
      if (typeof val === "string" && val.trim()) {
        return [val.trim()];
      }
      return [];
    };

    envelope.allowed_files = toStringArray(envelope.allowed_files);
    envelope.acceptance_checks = toStringArray(envelope.acceptance_checks);
    envelope.artifact_refs = toStringArray(envelope.artifact_refs);
    if (envelope.non_goals !== undefined) {
      envelope.non_goals = toStringArray(envelope.non_goals);
    }

    const contractErrors: string[] = [];
    if (envelope.contract_version !== "1.0" && envelope.contract_version !== "2.0") {
      contractErrors.push(`unsupported contract_version '${envelope.contract_version}' (must be '1.0' or '2.0')`);
    }
    if (!["discovery", "investigate", "planner", "implement", "reviewer"].includes(envelope.role)) {
      contractErrors.push(`unrecognized role '${envelope.role}' (allowed: discovery, investigate, planner, implement, reviewer)`);
    }
    for (const key of ["workflow", "phase", "objective"]) {
      if (typeof envelope[key] !== "string" || !envelope[key].trim()) {
        contractErrors.push(`field '${key}' must be a non-empty string (got ${JSON.stringify(envelope[key])})`);
      }
    }
    if (!(envelope.task_id === null || (typeof envelope.task_id === "string" && envelope.task_id.trim()))) {
      contractErrors.push(`task_id must be null or non-empty string (got ${JSON.stringify(envelope.task_id)})`);
    }
    if (![null, "openspec", "cortex", "hybrid"].includes(envelope.spec_plane)) {
      contractErrors.push(`spec_plane '${envelope.spec_plane}' is invalid (allowed: null, openspec, cortex, hybrid)`);
    }
    for (const key of ["allowed_files", "acceptance_checks", "artifact_refs"]) {
      if (!Array.isArray(envelope[key]) || !envelope[key].every((item: unknown) => typeof item === "string")) {
        contractErrors.push(`field '${key}' must be an array of strings`);
      }
    }
    if (envelope.non_goals !== undefined && (!Array.isArray(envelope.non_goals) || !envelope.non_goals.every((item: unknown) => typeof item === "string"))) {
      contractErrors.push(`field 'non_goals' must be an array of strings`);
    }

    if (contractErrors.length > 0) {
      throw new Error(`SUBAGENT_TRANSPORT_ERROR: invalid common dispatch contract: ${contractErrors.join("; ")}`);
    }

    if (envelope.role === "planner") {
      const phases: Record<string, string[]> = {
        "decision-map": ["chart", "resolve"],
        "sdd-lite": ["integrated", "propose", "plan", "tasks", "spec", "design", "archive", "decompose"],
        "sdd-full": ["propose", "spec", "design", "tasks", "archive", "decompose"],
      };
      if (!phases[envelope.workflow]) {
        throw new Error(`SUBAGENT_TRANSPORT_ERROR: invalid planning workflow (${envelope.workflow}; allowed: decision-map, sdd-lite, sdd-full)`);
      }
      if (!phases[envelope.workflow].includes(envelope.phase)) {
        throw new Error(`SUBAGENT_TRANSPORT_ERROR: invalid planning phase for ${envelope.workflow} (${envelope.phase}; allowed: [${phases[envelope.workflow].join(", ")}])`);
      }
      if (envelope.spec_plane === null) {
        throw new Error("SUBAGENT_TRANSPORT_ERROR: specification plane cannot be null for planner");
      }
    }
  }

  const explicitBudget = extractBudget(args, envelope);
  const workload = envelope.workload_policy ?? "flexible";
  if (!["strict", "flexible", "unbounded"].includes(workload) || envelope.workload_policy === null) {
    throw new Error("SUBAGENT_TRANSPORT_ERROR: workload_policy must be strict, flexible or unbounded");
  }
  // Normalize only after all aliases and routing fields pass validation. Keep
  // caller input immutable; the dispatch hook replaces its outgoing prompt.
  const normalized = { ...envelope, workload_policy: workload };
  if (normalized.role === "planner" && normalized.workflow === "sdd-lite" && ["propose", "plan", "tasks", "spec", "design"].includes(normalized.phase)) {
    normalized.phase = "integrated";
  }
  delete normalized.steps;
  delete normalized.budget;
  if (explicitBudget !== undefined) normalized.max_steps = explicitBudget;
  const normalizedPrompt = envelopes.length
    ? prompt.replace(envelopes[0][0], () => `<minion-dispatch>${JSON.stringify(normalized)}</minion-dispatch>`)
    : explicitBudget !== undefined ? `${prompt}\n<minion-dispatch>${JSON.stringify(normalized)}</minion-dispatch>` : prompt;
  const hostRole = hostTarget;
  if (hostRole === "planner") return { limit: Number.POSITIVE_INFINITY, operational: false, prompt: normalizedPrompt };
  const fallback = typeof hostRole === "string" && Object.prototype.hasOwnProperty.call(ROLE_DEFAULT_STEPS, hostRole)
    ? ROLE_DEFAULT_STEPS[hostRole] : DEFAULT_MAX_STEPS;
  return { limit: explicitBudget ?? fallback, operational: false, prompt: normalizedPrompt };
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
export const CortexSubagentTransportPlugin = async (ctx: any) => {
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
    const toolName = part?.tool || part?.name;
    if (part?.type !== "tool" || toolName !== "task") return;
    const dispatches = starts.get(part.sessionID);
    if (!dispatches) return;
    // v1.18.29: normal session/tools uses callID; handleSubtask uses part.id.
    // Both are exact identities, never a parent-only fallback. Collisions fail closed.
    const partCallID = part.callID || part.id;
    const matches = [...dispatches.values()].filter(s => s.callID === partCallID || s.callID === part.id);
    if (!matches.length) return;
    const data = part.state?.metadata;
    if (!data?.sessionId) return;
    const identity = JSON.stringify([part.messageID, part.id, partCallID]);
    const conflict = matches.length !== 1 || !["running", "completed", "error"].includes(part.state?.status) || typeof data.sessionId !== "string" ||
      data.parentSessionId !== part.sessionID || !part.id || !part.messageID || !partCallID;
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
      const data = result?.data !== undefined ? result.data : result;
      if (result?.error || data === undefined || data === null) return fail();
      return data;
    } catch { return fail(); } finally { clearTimeout(timer); }
  };
  const resumeFailure = () => {
    throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: cannot verify complete resume identity and budget; explicitly dispatch a fresh bounded task without task_id/session_id after reconciling prior work");
  };

  const readSessionInfo = async (id: string, signal: AbortSignal) => {
    if (ctx.session?.get) {
      return await ctx.session.get({ sessionID: id, path: { id } } as any, { signal } as any);
    }
    if ((ctx as any).client?.session?.get) {
      return await (ctx as any).client.session.get({ path: { id }, sessionID: id, signal });
    }
    throw new Error("No session get API available");
  };

  const readSessionMessages = async (id: string, signal: AbortSignal, limit = 1000) => {
    if (ctx.session?.messages) {
      return await (ctx.session as any).messages({ sessionID: id, path: { id }, query: { limit }, signal });
    }
    if (ctx.session?.context) {
      return await ctx.session.context({ sessionID: id } as any, { signal } as any);
    }
    if ((ctx as any).client?.session?.messages) {
      return await (ctx as any).client.session.messages({ path: { id }, query: { limit }, signal });
    }
    if ((ctx as any).client?.session?.context) {
      return await (ctx as any).client.session.context({ sessionID: id, signal });
    }
    throw new Error("No session messages API available");
  };

  const normalizeMessages = (raw: any, sessionID: string) => {
    const list = Array.isArray(raw) ? raw : (Array.isArray(raw?.data) ? raw.data : []);
    return list.map((msg: any, mIdx: number) => {
      const msgId = msg.info?.id || msg.id || `msg_${mIdx}`;
      const msgSessionId = msg.info?.sessionID || msg.sessionID || sessionID;
      const role = msg.info?.role || msg.type || msg.role || "";
      const rawParts = Array.isArray(msg.parts)
        ? msg.parts
        : (Array.isArray(msg.content) ? msg.content : []);
      const parts = rawParts.map((part: any, pIdx: number) => {
        const partId = part.id || `${msgId}_part_${pIdx}`;
        const callID = part.callID || part.id || partId;
        const toolName = part.tool || part.name || "";
        return {
          id: partId,
          callID,
          tool: toolName,
          name: toolName,
          type: part.type,
          sessionID: part.sessionID || msgSessionId,
          messageID: part.messageID || msgId,
          state: part.state
        };
      });
      return {
        info: {
          id: msgId,
          sessionID: msgSessionId,
          role
        },
        parts
      };
    });
  };

  // The host returns at most limit messages, with no implicit lower cap. Asking
  // for one extra proves completeness without relying on SDK cursor support.
  const history = async (id: string) => {
    const raw = await lookup(signal => readSessionMessages(id, signal, 1001));
    const messages = normalizeMessages(raw, id);
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
      const info = await lookup(signal => readSessionInfo(id, signal));
      if (info.id !== id || info.parentID !== parent || info.revert) return resumeFailure();
      const [parentHistory, childHistory] = await Promise.all([history(parent), history(id)]);
      const links = parentHistory.flatMap(message => message.info.role === "assistant" ? message.parts : []).filter(part =>
        part.type === "tool" && (part.tool === "task" || part.name === "task") && part.state?.metadata?.sessionId === id);
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
      const info = await lookup(signal => readSessionInfo(id, signal));
      if (disposed || info.id !== id || deleted.has(id)) return fail();
      register(info);
      if (!info.parentID && !childSessions.has(id)) return; // SDK-proven root, not an unknown child.
    }
    const parent = parents.get(id);
    if (!parent || deleted.has(id) || deleted.has(parent)) return fail();
    if (!owners.has(id)) {
      const raw = await lookup(signal => readSessionMessages(parent, signal, 100));
      const messages = normalizeMessages(raw, parent);
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

    const abortController = new AbortController();
    const { signal } = abortController;

    const processEvent = async (event: any) => {
      if (disposed || !event) return;
      const type = event.type || (event as any).event || "";
      const sessionData = (event as any).data || (event as any).properties?.info || (event as any).properties || {};
      const sessionID = sessionData.sessionID || sessionData.id;
      const parentID = sessionData.parentID;

      // Register child subagent sessions spawned with a parentID
      if (type === "session.created" && parentID && sessionID) {
        register({ id: sessionID, parentID }); // Creation proves ancestry only; it grants no allowance.
      } else if (type === "message.part.updated") {
        const part = (event.properties as any)?.part;
        if (part) {
          metadata(part);
          if (part.type === "tool") terminal(part.sessionID, part.callID || part.id, part.tool || part.name,
            part.state?.input, part.state?.status, part.state?.status === "error" ? part.state.error : part.state?.output);
        }
      } else if (type === "session.message.content.updated" || type === "message.updated") {
        const content = (event as any).data?.content || (event as any).data?.part || (event as any).properties?.part;
        if (content) {
          const parts = Array.isArray(content) ? content : [content];
          for (const part of parts) {
            const partCallID = part.callID || part.id;
            const toolName = part.tool || part.name;
            const sid = (event as any).data?.sessionID || part.sessionID;
            const normalizedPart = {
              id: part.id || partCallID,
              callID: partCallID,
              tool: toolName,
              name: toolName,
              type: part.type,
              sessionID: sid,
              messageID: (event as any).data?.messageID || part.messageID,
              state: part.state
            };
            metadata(normalizedPart);
            if (part.type === "tool" && sid && partCallID) {
              terminal(sid, partCallID, toolName,
                part.state?.input, part.state?.status, part.state?.status === "error" ? part.state.error : (part.state?.content || part.state?.output));
            }
          }
        }
      } else if (type === "session.deleted") {
        const id = sessionData.sessionID || sessionData.id;
        if (id) {
          deleted.add(id);
        }
      }
    };

    if (ctx.event?.subscribe) {
      ;(async () => {
        try {
          for await (const event of ctx.event.subscribe({ signal })) {
            await processEvent(event);
          }
        } catch (err: any) {
          if (err?.name !== "AbortError" && !signal.aborted) {}
        }
      })();
    }

    const contextHook = async (contextOrEvent: any, maybeEvent?: any) => {
      const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
      const sessionID = event?.sessionID || event?.sessionId || event?.message?.sessionID;
      if (!sessionID) return;

      // Preserve host policy, role instructions, and other plugins' system content.
      if (childSessions.has(sessionID)) {
        if (Array.isArray(event.system)) {
          for (let index = event.system.length - 1; index >= 0; index--) {
            const item = event.system[index];
            const str = typeof item === "string" ? item : (item && typeof item === "object" && typeof item.text === "string" ? item.text : "");
            if (str.startsWith("CORTEX_BUDGET_WARNING:") || str.startsWith("CORTEX_REPETITION_WARNING:")) {
              event.system.splice(index, 1);
            }
          }
          const hasText = (t: string) => event.system.some((item: any) => typeof item === "string" ? item.includes(t) : (item && typeof item === "object" && typeof item.text === "string" && item.text.includes(t)));
          const appendSystem = (t: string) => {
            if (event.system.length > 0 && typeof event.system[0] === "object" && event.system[0] !== null) {
              event.system.push({ type: "text", text: t });
            } else {
              event.system.push(t);
            }
          };
          if (!hasText(TRANSPORT_ISOLATION_SYSTEM)) appendSystem(TRANSPORT_ISOLATION_SYSTEM);
          const count = sessionStepCounts.get(sessionID) ?? 0;
          const budget = sessionStepLimits.get(sessionID) ?? Infinity;
          if (count >= budget) appendSystem(`CORTEX_BUDGET_WARNING: advisory tool budget ${budget} reached (${count} attempts). Continue when useful; assess remaining scope and report partial progress if needed. This warning does not change task authority.`);
          const streak = progress.get(sessionID)?.streak ?? 0;
          if (streak >= repetitionLimit) appendSystem(`CORTEX_REPETITION_WARNING: ${streak} consecutive terminal tools had identical tool, arguments, status and result. This detects observable repetition, not semantic lack of progress; polling can be legitimate. Reassess the approach and report progress.`);
        } else if (typeof event.system === "string") {
          if (!event.system.includes(TRANSPORT_ISOLATION_SYSTEM)) {
            event.system += "\n\n" + TRANSPORT_ISOLATION_SYSTEM;
          }
        }

        if (maybeEvent && maybeEvent.system && event.system) {
          maybeEvent.system = event.system;
        }

        // OpenCode v2: Proactive Role-based Tool Pruning
        if (event?.tools && typeof event.tools === "object") {
          const role = event?.agent || event?.role || (owners.get(sessionID)?.role);
          if (role) {
            const roleLower = String(role).toLowerCase();
            if (roleLower === "orchestrator" || roleLower === "investigate" || roleLower === "reviewer") {
              delete event.tools["edit"];
              delete event.tools["write_to_file"];
              delete event.tools["write"];
              delete event.tools["apply_patch"];
              delete event.tools["cortex_ia_work_claim"];
              delete event.tools["cortex_ia_file_reserve"];
            }
            if (roleLower === "implement") {
              delete event.tools["cortex_ia_work_create"];
              delete event.tools["cortex_ia_work_approve"];
              delete event.tools["cortex_ia_board_create"];
            }
            if (roleLower === "planner") {
              delete event.tools["cortex_ia_work_claim"];
              delete event.tools["cortex_ia_file_reserve"];
              delete event.tools["cortex_ia_work_approve"];
            }
          }
          if (operationalSessions.has(sessionID)) {
            delete event.tools["edit"];
            delete event.tools["write_to_file"];
            delete event.tools["write"];
            delete event.tools["apply_patch"];
          }
        }
      }
    };

    if (ctx.session?.hook) {
      ctx.session.hook("context", contextHook);
    }

    const executeAfter = async (contextOrEvent: any, maybeEvent?: any) => {
      if (!disposed) {
        const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
        const sessionID = event?.sessionID || event?.sessionId;
        const callID = event?.callID || event?.callId || event?.id || event?.toolCallId || event?.toolCallID || event?.call_id || (contextOrEvent as any)?.callID || (contextOrEvent as any)?.callId || (contextOrEvent as any)?.id || "";
        const tool = event?.tool || event?.name;
        const args = event?.args || event?.input;
        const output = event?.output ?? event?.result;
        terminal(sessionID, callID, tool, args, "completed", output);
      }
    };

    const executeBefore = async (contextOrEvent: any, maybeEvent?: any) => {
      const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
      const rawTool = (event?.tool || event?.name || "").toLowerCase();
      const toolName = rawTool;
      const sessionID = event?.sessionID || event?.sessionId || "";
      let callID = event?.callID || event?.callId || event?.id || event?.toolCallId || event?.toolCallID || event?.call_id || event?.tool_call_id ||
        (contextOrEvent as any)?.callID || (contextOrEvent as any)?.callId || (contextOrEvent as any)?.id ||
        (maybeEvent as any)?.callID || (maybeEvent as any)?.callId || (maybeEvent as any)?.id || "";
      if (!callID && sessionID) {
        callID = `call_synth_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
      }
      if (event) {
        event.callID = callID;
        event.callId = callID;
      }
      if (contextOrEvent && typeof contextOrEvent === "object") {
        contextOrEvent.callID = callID;
        contextOrEvent.callId = callID;
      }
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
        if (!callID || activeCalls.get(sessionID)?.has(callID)) return fail();
        const historicalPending = restoredPending.get(sessionID)?.has(callID) ?? false;
        const callKey = createHash("sha256").update(callID).digest("hex");
        if (progress.get(sessionID)?.seen.has(callKey)) return fail();
        const count = sessionStepCounts.get(sessionID) ?? 0;
        const limit = sessionStepLimits.get(sessionID) ?? 0;
        const ceiling = Number.isFinite(limit) ? emergencySteps : Infinity;
        const inputArgs = event?.args || event?.input;
        if (count - (historicalPending ? 1 : 0) >= ceiling) {
          const cleanup = cleanupCounts.get(sessionID) ?? 0;
          const cleanupReserved = restoredCleanupPending.get(sessionID)?.has(callID) ?? false;
          if (cleanup - (cleanupReserved ? 1 : 0) < 5 && isCleanup(toolName, inputArgs)) {
            cleanupCounts.set(sessionID, cleanup + (cleanupReserved ? 0 : 1));
            restoredCleanupPending.get(sessionID)?.delete(callID);
            restoredPending.get(sessionID)?.delete(callID);
            if (!activeCalls.has(sessionID)) activeCalls.set(sessionID, new Set());
            activeCalls.get(sessionID)!.add(callID);
            sessionStepCounts.set(sessionID, count + (historicalPending ? 0 : 1));
            return;
          }
          throw new Error(`AGENT_EMERGENCY_LIMIT: reached ${ceiling} tool attempts. Return a partial receipt and reconcile prior work before an explicitly authorized continuation; task status is unchanged. Up to five cleanup tools remain available.`);
        }
        restoredPending.get(sessionID)?.delete(callID);
        if (!activeCalls.has(sessionID)) activeCalls.set(sessionID, new Set());
        activeCalls.get(sessionID)!.add(callID);
        sessionStepCounts.set(sessionID, count + (historicalPending ? 0 : 1));
      }

      // 1. Task dispatch gate: enforce non-empty prompt and detect contract limits
      if (toolName === "task" || toolName === "subagent") {
        const args = ((event?.args || event?.input) || {}) as Record<string, any>;
        const prompt = args?.prompt || args?.description || "";
        if (typeof prompt !== "string" || prompt.trim().length === 0) {
          throw new Error("SUBAGENT_TRANSPORT_ERROR: task dispatch requires a non-empty prompt or envelope");
        }

        const { limit: stepBudget, operational: isOperational, prompt: normalizedPrompt } = dispatchInfo(args, prompt);
        const existingStart = starts.get(sessionID)?.get(callID);
        if (existingStart) {
          const normalizedArgs = { ...args, prompt: normalizedPrompt };
          delete normalizedArgs.steps;
          delete normalizedArgs.max_steps;
          delete normalizedArgs.budget;
          if (event.args) event.args = normalizedArgs;
          if (event.input) event.input = normalizedArgs;
          if (maybeEvent?.args) maybeEvent.args = normalizedArgs;
          if (maybeEvent?.input) maybeEvent.input = normalizedArgs;
          return;
        }
        if (!sessionID || !callID) {
          throw new Error("SUBAGENT_TRANSPORT_AMBIGUOUS: task requires a unique parent and callID pair");
        }
        if (args.task_id && args.session_id && args.task_id !== args.session_id) return resumeFailure();
        const resume = args.task_id || args.session_id;
        if (resume) {
          if (typeof resume !== "string" || resume === sessionID) return resumeFailure();
          await identify(resume);
          if (!owners.has(resume) || parents.get(resume) !== sessionID || deleted.has(resume) ||
              deleted.has(sessionID) || owners.get(resume)?.invalid) return resumeFailure();
          if (starts.get(sessionID)?.has(callID)) return fail();
        }
        const start: Start = {
          callID: callID,
          limit: resume ? sessionStepLimits.get(resume)! : stepBudget,
          childID: resume || undefined,
          resume: !!resume,
          operational: resume ? operationalSessions.has(resume) : isOperational,
        };
        if (!starts.has(sessionID)) starts.set(sessionID, new Map());
        starts.get(sessionID)!.set(callID, start);
        const normalizedArgs = { ...args, prompt: normalizedPrompt };
        delete normalizedArgs.steps;
        delete normalizedArgs.max_steps;
        delete normalizedArgs.budget;
        if (event.args) event.args = normalizedArgs;
        if (event.input) event.input = normalizedArgs;
        if (maybeEvent?.args) maybeEvent.args = normalizedArgs;
        if (maybeEvent?.input) maybeEvent.input = normalizedArgs;
      }
    };

    if (ctx.tool?.hook) {
      ctx.tool.hook("execute.after", executeAfter);
      ctx.tool.hook("execute.before", executeBefore);
    }

    const cleanup = async () => {
      disposed = true;
      abortController.abort();
      childSessions.clear();
      sessionStepCounts.clear();
      sessionStepLimits.clear();
      operationalSessions.clear();
      starts.clear(); parents.clear(); deleted.clear(); owners.clear();
      cleanupCounts.clear();
      restorations.clear();
      restoredPending.clear(); restoredCleanupPending.clear(); activeCalls.clear(); progress.clear();
    };
    (cleanup as any).dispose = cleanup;
    (cleanup as any)["tool.execute.before"] = executeBefore;
    (cleanup as any)["tool.execute.after"] = executeAfter;
    (cleanup as any)["experimental.chat.system.transform"] = contextHook;
    (cleanup as any).event = async (raw: any) => {
      await processEvent(raw?.event || raw);
    };
    return cleanup;
};

export { dispatchInfo };

Object.assign(CortexSubagentTransportPlugin, {
  dispatchInfo,
});

export const CortexSubagentTransportPluginDefinition = {
  id: "cortex-subagent-transport",
  setup: CortexSubagentTransportPlugin,
  server: CortexSubagentTransportPlugin,
  dispatchInfo,
};

export default CortexSubagentTransportPluginDefinition;
