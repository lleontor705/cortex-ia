import { type Plugin } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

const CORTEX_ROLES = new Set(["discovery", "investigate", "planner", "implement", "reviewer"]);

function resolveCortexExecutable(): string | null {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(home, "go", "bin", "cortex-ia.exe"),
    "cortex-ia",
    path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
    path.join(home, ".local", "bin", "cortex-ia"),
    "/usr/local/bin/cortex-ia",
    "/usr/bin/cortex-ia",
  ];
  for (const candidate of candidates) {
    if (candidate !== "cortex-ia" && fs.existsSync(candidate)) return candidate;
    if (candidate === "cortex-ia") {
      try {
        execFileSync(candidate, ["version"], { stdio: "ignore", windowsHide: true });
        return candidate;
      } catch {}
    }
  }
  return null;
}

interface TaskFailure {
  role: string;
  taskId?: string;
  reason: string;
  timestamp: number;
}

/**
 * CortexTaskLatchPlugin guards subagent dispatch in OpenCode against silent dropouts,
 * empty outputs, and provider rate-limits. When an active subagent terminates empty or corrupt,
 * this plugin records a terminal failure in SQLite and latches the session to prevent
 * ungrounded re-dispatch loops by the orchestrator.
 */
export const CortexTaskLatchPlugin: Plugin = async (ctx) => {
  const failedSessions = new Map<string, TaskFailure>();

  return {
    dispose: async () => {
      failedSessions.clear();
    },

    event: async ({ event }) => {
      if (event.type === "session.deleted") {
        const id = (event.properties as any)?.info?.id;
        if (id) failedSessions.delete(id);
      }
    },

    "tool.execute.before": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      if (toolName !== "task") return;

      const args = (output?.args || {}) as Record<string, any>;
      const subagent = args.subagent || args.subagent_type;
      if (typeof subagent !== "string" || !CORTEX_ROLES.has(subagent.toLowerCase())) return;

      const previousFailure = failedSessions.get(input.sessionID);
      if (previousFailure) {
        throw new Error(
          `CORTEX_DISPATCH_LATCHED: Earlier in this session '${previousFailure.role}' failed with '${previousFailure.reason}'. ` +
          `Task launches remain latched to prevent ungrounded re-dispatch loops. ` +
          `Reconcile authority for task '${previousFailure.taskId || "unknown"}' before continuing.`
        );
      }
    },

    "tool.execute.after": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      if (toolName !== "task") return;

      const args = (input?.args || {}) as Record<string, any>;
      const subagent = args.subagent || args.subagent_type;
      if (typeof subagent !== "string" || !CORTEX_ROLES.has(subagent.toLowerCase())) return;

      // Asynchronous background minions report their completion later through state transitions
      if (args.background === true) return;

      const raw = typeof output?.output === "string" ? output.output.trim() : JSON.stringify(output?.output || "").trim();

      // Check for empty, whitespace-only, or truncated response (< 20 characters)
      if (!raw || raw.length < 20) {
        const taskId = args.task_id || args.taskId;
        const failure: TaskFailure = {
          role: subagent,
          taskId: typeof taskId === "string" ? taskId : undefined,
          reason: "EMPTY_OR_CORRUPTED_OUTPUT",
          timestamp: Date.now(),
        };
        failedSessions.set(input.sessionID, failure);

        // Record platform operational incident in SQLite ledger
        const cortexBin = resolveCortexExecutable();
        if (cortexBin && failure.taskId) {
          try {
            execFileSync(
              cortexBin,
              [
                "report",
                "error",
                "--code",
                "ERR_SUBAGENT_EMPTY_OUTPUT",
                "--task",
                failure.taskId,
                "--message",
                `Subagent ${subagent} returned empty or corrupted output (< 20 chars)`,
                "--source",
                "task-latch-plugin",
              ],
              {
                cwd: ctx.directory,
                stdio: "ignore",
                windowsHide: true,
                timeout: 3000,
              }
            );
          } catch {}
        }

        throw new Error(
          `CORTEX_SUBAGENT_EMPTY_RESULT: Subagent '${subagent}' produced no valid response. ` +
          `The process was likely aborted due to context length, timeout, or an unhandled exception. ` +
          `Session is latched; reconcile task state in SQLite before continuing.`
        );
      }
    },
  };
};

export default CortexTaskLatchPlugin;
