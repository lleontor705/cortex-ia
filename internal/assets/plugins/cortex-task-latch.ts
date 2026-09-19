// OpenCode v2 Plugin helper ensuring default export is a valid plugin definition object
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any }>(def: T): T => {
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
interface Failure { role: string; taskId?: string; reason?: string; objective?: string }
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

// Host task_id identifies a resumed session, never a durable Cortex work item.
function workIdentity(args: Record<string, any>): string | undefined {
  const prompt = args.prompt;
  if (prompt === undefined) return undefined;
  if (typeof prompt !== "string" || prompt.length > 256 * 1024) throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: bounded prompt required");
  const matches = [...prompt.matchAll(/<minion-(dispatch|contract)>([\s\S]*?)<\/minion-\1>/g)];
  if (matches.length > 1 || (prompt.match(/<minion-(?:dispatch|contract)/g) || []).length !== matches.length ||
      (prompt.match(/<\/minion-(?:dispatch|contract)>/g) || []).length !== matches.length) {
    throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: ambiguous envelope");
  }
  if (!matches.length) return undefined;
  let envelope: Record<string, any>;
  try {
    validateNoDuplicateKeys(matches[0][2]);
    envelope = record(matches[0][2])!;
    if (!envelope || (envelope.role !== undefined && envelope.role !== (args.subagent_type || args.subagent || args.agent)) ||
        (envelope.task_id !== undefined && envelope.task_id !== null &&
        (typeof envelope.task_id !== "string" || !envelope.task_id.trim() || envelope.task_id.length > 128))) throw new Error();
  } catch { throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: malformed, duplicate or conflicting envelope"); }
  return envelope.task_id ?? undefined;
}

function failureReason(output: any): string | undefined {
  const value = output?.result ?? output?.output, receipt = record(value);
  if (output?.error || output?.metadata?.error || receipt?.error ||
      ["failed", "aborted", "cancelled", "timed_out", "blocked"].includes(receipt?.status) ||
      ["failed", "blocked"].includes(receipt?.phase_status)) return "FAILED_TERMINAL_RESULT";
  if (value == null || (typeof value === "string" && !value.trim())) return "EMPTY_RESULT";
  // Nonempty legacy text remains compatible; response length cannot prove corruption.
  if (typeof value === "string" && /^[{[]/.test(value.trim()) && !receipt) return "MALFORMED_RECEIPT";
}

/** Guards repeated dispatch after observed failure. It does not grant work authority. */
export const CortexTaskLatchPlugin = Plugin.define({
  id: "cortex-task-latch",
  async setup(ctx) {
    const directory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();
    const failed = new Map<string, Map<string, Failure>>(), pending = new Map<string, Pending>();
    const retries = new Map<string, string>();
    const capacity = 256;
    let saturated = false;
    const objective = (role: string, prompt: unknown) => createHash("sha256").update(JSON.stringify([role, prompt ?? null])).digest("hex");
    const failureKey = (failure: Failure) => failure.taskId ? "task:" + failure.taskId : "objective:" + failure.objective;

    function latch(parent: string, failure: Failure): void {
      const key = failureKey(failure);
      const existing = failed.get(parent);
      const count = [...failed.values()].reduce((total, items) => total + items.size, 0);
      if (!existing?.has(key) && count >= capacity) { saturated = true; return; }
      if (!existing) failed.set(parent, new Map());
      failed.get(parent)!.set(key, failure);
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
        : "Task identity is unknown; continuation capability is unavailable without an identified task. Reconcile before starting a fresh session.";
    }

    const controller = new AbortController();
    void (async () => {
      try {
        for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
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
        }
      } catch {}
    })();

    await ctx.tool.hook("execute.before", async (event: any) => {
      const tool = (event?.tool || "").toLowerCase();
      const sessionID = event?.sessionID || event?.sessionId || "";
      const callID = event?.callID || event?.callId || "";
      const args = (event?.input || {}) as Record<string, any>;

      if (UNLATCH.has(tool)) throw new Error("CORTEX_UNLATCH_UNAVAILABLE: Unlatch tools do not exist");
      if (RECOVERY.has(tool)) {
        if (!(await root(sessionID))) throw new Error("CORTEX_RECOVERY_UNAUTHORIZED: Leaf subagents cannot perform recovery or retry");
        const taskId = args?.task_id;
        if (tool === "cortex_ia_work_retry" && typeof taskId === "string" && taskId.length > 128) throw new Error("CORTEX_DISPATCH_IDENTITY_INVALID: oversized work identity");
        if (tool === "cortex_ia_work_retry" && callID && typeof taskId === "string") boundedSet(retries, sessionID + ":" + callID, taskId);
      }
      if (tool !== "task" && tool !== "subagent") return;
      const role = args.subagent || args.subagent_type || args.agent;
      if (typeof role !== "string" || !ROLES.has(role.toLowerCase())) return;
      const taskId = workIdentity(args);
      const digest = objective(role, args.prompt);
      const failures = failed.get(sessionID);
      const readonly = ["investigate", "reviewer"].includes(role.toLowerCase());
      if (saturated && !readonly) throw new Error("CORTEX_LATCH_CAPACITY: explicit reconciliation required; no failures were released");
      const previous = failures?.get(failureKey({ role, taskId, objective: digest })) ||
        (!taskId && !readonly ? failures?.values().next().value : undefined);
      if (!previous) return;
      const diagnosis = readonly && (role !== previous.role || digest !== previous.objective);
      if (!diagnosis) throw new Error("CORTEX_DISPATCH_LATCHED: " + guidance(previous));
    });

    await ctx.tool.hook("execute.after", async (event: any) => {
      const tool = (event?.tool || "").toLowerCase();
      const sessionID = event?.sessionID || event?.sessionId || "";
      const callID = event?.callID || event?.callId || "";
      const key = sessionID + ":" + callID;
      const args = (event?.input || {}) as Record<string, any>;
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
      const role = background?.role || args.subagent || args.subagent_type || args.agent;
      if (typeof role !== "string" || !ROLES.has(role.toLowerCase())) return;
      const parent = background?.parent || sessionID, taskId = background ? background.taskId : workIdentity(args);
      const receipt = record(output), reason = failureReason(event);
      const identity = receipt?.session_id || receipt?.sessionID || receipt?.task_id || event?.metadata?.sessionId;
      if (!reason && (["accepted", "starting", "running", "pending"].includes(receipt?.status) ||
          (args.background === true && typeof identity === "string" && !["succeeded", "done"].includes(receipt?.status)))) {
        if (typeof identity === "string") boundedSet(pending, identity, { parent, role, taskId, objective: objective(role, args.prompt) });
        return;
      }
      if (background) pending.delete(args.task_id);
      if (!reason) return;
      const failure = { role, taskId: typeof taskId === "string" ? taskId : undefined, reason, objective: background?.objective || objective(role, args.prompt) };
      latch(parent, failure);
      const executable = failure.taskId && resolveCortexExecutable(directory);
      if (executable) {
        try {
          execFileSync(executable, ["report", "error", "--code", "ERR_SUBAGENT_EMPTY_OUTPUT",
            "--task", failure.taskId!, "--message", "Subagent terminal failure: " + reason,
            "--details", JSON.stringify({ role, task_id: failure.taskId, reason }), "--source", "task-latch-plugin"],
            { cwd: directory, stdio: "ignore", windowsHide: true, timeout: 3000 });
        } catch { /* Reporting availability cannot release the failed-attempt latch. */ }
      }
      throw new Error("CORTEX_SUBAGENT_EMPTY_RESULT: " + reason + "; supported continuation requires orchestrator reconciliation. " + guidance(failure));
    });

    return () => {
      controller.abort();
      failed.clear();
      pending.clear();
      retries.clear();
    };
  }
});

export default CortexTaskLatchPlugin;
