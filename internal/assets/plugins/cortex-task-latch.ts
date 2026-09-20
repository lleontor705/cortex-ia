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
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

function resolveExecutable(cmd: string): string | null {
  const isWin = process.platform === "win32";
  const locator = isWin ? "where.exe" : "which";
  try {
    const out = execFileSync(locator, [cmd], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
    }).trim();
    if (out) {
      const first = out.split(/\r?\n/)[0].trim();
      if (path.isAbsolute(first) && fs.existsSync(first)) {
        return first;
      }
    }
  } catch {}
  return null;
}

function resolveCortexExecutable(directory?: string): string | null {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(home, "go", "bin", "cortex-ia.exe"),
    path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
    path.join(home, ".local", "bin", "cortex-ia"),
    "/usr/local/bin/cortex-ia",
    "/usr/bin/cortex-ia",
  ];
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  const resolved = resolveExecutable("cortex-ia");
  if (resolved) {
    if (directory && path.resolve(resolved).startsWith(path.resolve(directory) + path.sep)) {
      return null;
    }
    return resolved;
  }
  return null;
}

const ROLES = new Set(["discovery", "investigate", "planner", "implement", "reviewer"]);
const RECOVERY = new Set(["cortex_recover", "cortex_ia_recover", "cortex_ia_work_recover", "cortex_ia_work_retry"]);
const UNLATCH = new Set(["unlatch", "cortex_unlatch", "cortex_ia_unlatch", "cortex_work_unlatch"]);
interface Failure {
  role: string;
  taskId?: string;
  reason?: string;
  objective?: string;
  attempts?: number;
  tripped?: boolean;
}
interface Pending extends Failure { parent: string }

function record(value: unknown): Record<string, any> | undefined {
  if (typeof value === "string") { try { value = JSON.parse(value); } catch { return; } }
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, any> : undefined;
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

function dispatchMeta(args: Record<string, any>): { taskId?: string; role?: string } {
  const prompt = args.prompt;
  if (typeof prompt !== "string") return {};
  const matches = [...prompt.matchAll(/<minion-(dispatch|contract)>([\s\S]*?)<\/minion-\1>/g)];
  if (!matches.length) return {};
  try {
    const raw = matches[0][2];
    validateNoDuplicateKeys(raw);
    const envelope = record(raw);
    if (!envelope) return {};
    const declaredRole = typeof envelope.role === "string" ? envelope.role.toLowerCase() : undefined;
    const directRole = (args.subagent || args.subagent_type || args.agent)?.toString().toLowerCase();
    if (declaredRole && directRole && declaredRole !== directRole) {
      throw new Error("role mismatch");
    }
    const taskId = typeof envelope.task_id === "string" && envelope.task_id.trim() ? envelope.task_id.trim() : undefined;
    return { taskId, role: declaredRole || directRole };
  } catch (err: any) {
    if (err?.message === "role mismatch") {
      throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: conflicting envelope role vs argument");
    }
    return {};
  }
}

// Host task_id identifies a resumed session, never a durable Cortex work item.
function workIdentity(args: Record<string, any>): string | undefined {
  return dispatchMeta(args).taskId;
}

function failureReason(output: any): string | undefined {
  const value = output?.result ?? output?.output, receipt = record(value);
  if (output?.error || output?.metadata?.error || receipt?.error ||
      ["failed", "aborted", "cancelled", "timed_out"].includes(receipt?.status) ||
      ["failed"].includes(receipt?.phase_status)) return "FAILED_TERMINAL_RESULT";
  if (value == null || (typeof value === "string" && !value.trim())) return "EMPTY_RESULT";
  // Nonempty textual and markdown responses are valid outputs, not malformed receipts.
  return undefined;
}

/** Guards repeated dispatch after observed failure. It does not grant work authority. */
export const CortexTaskLatchPlugin = async (ctx: any) => {
  const directory = (ctx as any)?.location?.directory || (ctx as any)?.directory || process.cwd();
    const failed = new Map<string, Map<string, Failure>>(), pending = new Map<string, Pending>();
    const retries = new Map<string, string>();
    const capacity = 256;
    let saturated = false;
    const objective = (role: string, prompt: unknown) => createHash("sha256").update(JSON.stringify([role, prompt ?? null])).digest("hex");
    const failureKey = (failure: Failure) => failure.taskId ? "task:" + failure.taskId : "objective:" + failure.objective;

    const MAX_CIRCUIT_ATTEMPTS = 4;

    function latch(parent: string, failure: Failure): { failure: Failure; tripped: boolean } {
      const key = failureKey(failure);
      let existingSession = failed.get(parent);
      if (!existingSession) {
        existingSession = new Map();
        failed.set(parent, existingSession);
      }
      const prev = existingSession.get(key);
      const attempts = (prev?.attempts || 0) + 1;
      const tripped = attempts >= MAX_CIRCUIT_ATTEMPTS;

      failure.attempts = attempts;
      failure.tripped = tripped;

      const count = [...failed.values()].reduce((total, items) => total + items.size, 0);
      if (!existingSession.has(key) && count >= capacity) {
        saturated = true;
        return { failure, tripped: true };
      }
      existingSession.set(key, failure);
      return { failure, tripped };
    }

    function boundedSet<T>(map: Map<string, T>, key: string, value: T): void {
      if (key.length > 512 || (!map.has(key) && map.size >= capacity)) {
        saturated = true;
        throw new Error("CORTEX_LATCH_CAPACITY: reconciliation tracking capacity exhausted; no failures were released");
      }
      map.set(key, value);
    }

    async function root(id: string): Promise<boolean> {
      try {
        const response = await (ctx.session?.get ? ctx.session.get({ path: { id }, sessionID: id } as any) : (ctx as any).client?.session?.get({ path: { id } }));
        const data = response?.data ?? response;
        return data?.id === id && !data?.parentID;
      } catch { return false; }
    }

    function guidance(f: Failure): string {
      return f.taskId
        ? `Task ${f.taskId}. Supported continuation: Orchestrator must reconcile durable state and retry without reusing expired tokens.`
        : `Diagnostic objective (${f.role || "subagent"}): subagent encountered ${f.reason || "failure"}. Orchestrator can retry or continue natively.`;
    }

    const controller = new AbortController();
    const processEvent = async (event: any) => {
      const type = (event as any).type || (event as any).event || "";
      const props = (event as any).properties as any, id = props?.sessionID || props?.info?.id || (event as any).sessionID;
      if (type === "session.error" || type === "session.deleted") {
        const item = pending.get(id);
        if (item) latch(item.parent, { ...item, reason: "BACKGROUND_SESSION_FAILED" });
        pending.delete(id);
      }
      if (type === "session.deleted") {
        failed.delete(id);
        for (const [child, item] of pending) if (item.parent === id) pending.delete(child);
        for (const key of retries.keys()) if (key.startsWith(id + ":")) retries.delete(key);
      }
    };

    if (ctx.event?.subscribe) {
      void (async () => {
        try {
          for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
            await processEvent(event);
          }
        } catch {}
      })();
    }

    const executeBefore = async (contextOrEvent: any, maybeEvent?: any) => {
      const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
      const tool = (event?.tool || "").toLowerCase();
      const sessionID = event?.sessionID || event?.sessionId || "";
      const callID = event?.callID || event?.callId || "";
      const args = (event?.args || event?.input || {}) as Record<string, any>;

      if (UNLATCH.has(tool)) throw new Error("CORTEX_UNLATCH_UNAVAILABLE: Unlatch tools do not exist");
      if (RECOVERY.has(tool)) {
        if (!(await root(sessionID))) throw new Error("CORTEX_RECOVERY_UNAUTHORIZED: Leaf subagents cannot perform recovery or retry");
        const taskId = args?.task_id;
        if (tool === "cortex_ia_work_retry" && typeof taskId === "string" && taskId.length > 128) throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: oversized work identity");
        if (tool === "cortex_ia_work_retry" && callID && typeof taskId === "string") boundedSet(retries, sessionID + ":" + callID, taskId);
      }
      if (tool !== "task" && tool !== "subagent") return;
      const meta = dispatchMeta(args);
      const rawRole = meta.role || args.subagent || args.subagent_type || args.agent;
      if (typeof rawRole !== "string") return;
      const role = rawRole.toLowerCase();
      if (!ROLES.has(role)) return;
      const taskId = meta.taskId;
      const digest = objective(role, args.prompt);
      const failures = failed.get(sessionID);
      const readonly = ["investigate", "reviewer", "discovery", "planner"].includes(role);
      if (readonly) {
        // Read-only inspection and diagnostic roles NEVER hard-latch
        return;
      }
      if (saturated && !readonly) throw new Error("CORTEX_LATCH_CAPACITY: explicit reconciliation required; no failures were released");
      const key = failureKey({ role, taskId, objective: digest });
      const previous = failures?.get(key);
      if (!previous || !previous.tripped) return;
      throw new Error("CORTEX_CIRCUIT_OPEN: " + guidance(previous));
    };

    const executeAfter = async (contextOrEvent: any, maybeEvent?: any) => {
      const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
      const tool = (event?.tool || "").toLowerCase();
      const sessionID = event?.sessionID || event?.sessionId || "";
      const callID = event?.callID || event?.callId || "";
      const key = sessionID + ":" + callID;
      const args = (event?.args || event?.input || {}) as Record<string, any>;
      const output = event?.result ?? event?.output;

      if (tool === "cortex_ia_work_retry") {
        const taskId = retries.get(key), result = record(output);
        retries.delete(key);
        if (taskId && failed.get(sessionID)?.has("task:" + taskId) && !event?.error &&
            result?.task_id === taskId && result.status === "ready" &&
            Number.isInteger(result.revision) && result.revision > 0 && await root(sessionID)) {
          const failures = failed.get(sessionID)!;
          failures.delete("task:" + taskId);
          if (!failures.size) failed.delete(sessionID);
        }
        return;
      }
      // Recovery counts alone cannot establish that this failed objective is ready.
      if (tool !== "task" && tool !== "subagent" && tool !== "background_output") return;
      const background = tool === "background_output" ? pending.get(args.task_id) : undefined;
      const meta = dispatchMeta(args);
      const rawRole = background?.role || meta.role || args.subagent || args.subagent_type || args.agent;
      if (typeof rawRole !== "string") return;
      const role = rawRole.toLowerCase();
      if (!ROLES.has(role)) return;
      const parent = background?.parent || sessionID, taskId = background ? background.taskId : meta.taskId;
      const receipt = record(output), reason = failureReason(event);
      const identity = receipt?.session_id || receipt?.sessionID || receipt?.task_id || event?.metadata?.sessionId;
      if (!reason && (["accepted", "starting", "running", "pending"].includes(receipt?.status) ||
          (args.background === true && typeof identity === "string" && !["succeeded", "done"].includes(receipt?.status)))) {
        if (typeof identity === "string") boundedSet(pending, identity, { parent, role, taskId, objective: objective(role, args.prompt) });
        return;
      }
      if (background) pending.delete(args.task_id);
      if (!reason) {
        const successKey = failureKey({ role, taskId, objective: background?.objective || objective(role, args.prompt) });
        failed.get(parent)?.delete(successKey);
        return;
      }
      const readonly = ["investigate", "reviewer", "discovery", "planner"].includes(role.toLowerCase());
      if (readonly) {
        // Read-only subagents never throw latch exceptions in executeAfter.
        console.warn(`[CORTEX_TASK_LATCH] Read-only subagent '${role}' ended with reason '${reason}'. Latch exception suppressed.`);
        return;
      }
      const failure = { role, taskId: typeof taskId === "string" ? taskId : undefined, reason, objective: background?.objective || objective(role, args.prompt) };
      const { tripped, failure: recorded } = latch(parent, failure);
      if (tripped) {
        const executable = recorded.taskId && resolveCortexExecutable(directory);
        if (executable) {
          try {
            execFileSync(executable, ["report", "error", "--code", "ERR_SUBAGENT_CIRCUIT_OPEN",
              "--task", recorded.taskId!, "--message", `Subagent circuit tripped after ${recorded.attempts} failures: ${reason}`,
              "--details", JSON.stringify({ role, task_id: recorded.taskId, reason, attempts: recorded.attempts }), "--source", "task-latch-plugin"],
              { cwd: directory, stdio: "ignore", windowsHide: true, timeout: 3000 });
          } catch { /* Reporting availability cannot release the failed-attempt latch. */ }
        }
        throw new Error(`CORTEX_CIRCUIT_OPEN: Repeated terminal failure (${recorded.attempts} attempts: ${reason}); circuit breaker tripped. Supported continuation requires orchestrator reconciliation. ${guidance(recorded)}`);
      }
      throw new Error(`SUBAGENT_ATTEMPT_FAILED: Attempt ${recorded.attempts} for role '${role}' encountered ${reason}. Circuit breaker allows self-healing retry (${MAX_CIRCUIT_ATTEMPTS - recorded.attempts} remaining) before latching.`);
    };

    if (ctx.tool?.hook) {
      await ctx.tool.hook("execute.before", executeBefore);
      await ctx.tool.hook("execute.after", executeAfter);
    }

    const cleanup = () => {
      controller.abort();
      failed.clear();
      pending.clear();
      retries.clear();
    };
    (cleanup as any).dispose = cleanup;
    (cleanup as any)["tool.execute.before"] = executeBefore;
    (cleanup as any)["tool.execute.after"] = executeAfter;
    (cleanup as any).event = async (raw: any) => {
      await processEvent(raw?.event || raw);
    };
    return cleanup;
};

export const CortexTaskLatchPluginDefinition = {
  id: "cortex-task-latch",
  setup: CortexTaskLatchPlugin,
  server: CortexTaskLatchPlugin,
};

export default CortexTaskLatchPluginDefinition;
