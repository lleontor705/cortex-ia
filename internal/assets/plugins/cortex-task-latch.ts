import { type Plugin } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

const CORTEX_ROLES = new Set(["discovery", "investigate", "planner", "implement", "reviewer"]);

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

      // Support unlatch tools directly to unblock sessions
      if (["unlatch", "cortex_unlatch", "cortex_ia_unlatch", "cortex_work_unlatch"].includes(toolName)) {
        failedSessions.delete(input.sessionID);
        return;
      }

      if (toolName !== "task") return;

      const args = (output?.args || {}) as Record<string, any>;
      const subagent = args.subagent || args.subagent_type;
      if (typeof subagent !== "string" || !CORTEX_ROLES.has(subagent.toLowerCase())) return;

      // Auto-heal latch on subsequent dispatch attempt to prevent deadlock
      const previousFailure = failedSessions.get(input.sessionID);
      if (previousFailure) {
        failedSessions.delete(input.sessionID);
      }
    },

    "tool.execute.after": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      // Clear latch when orchestrator reconciles via recovery, retry, or decomposition
      if (
        toolName === "cortex_ia_work_recover" ||
        toolName === "cortex_ia_work_retry" ||
        toolName === "cortex_ia_work_decompose"
      ) {
        failedSessions.delete(input.sessionID);
        return;
      }
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
        const cortexBin = resolveCortexExecutable(ctx.directory);
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

        // In SpaceX step 2 (delete the part) & step 3 (simplify):
        // Log the incident to SQLite if possible, but do NOT throw an unhandled exception
        // that crashes the turn or creates a session deadlock.
        if (output && typeof output === "object" && !raw) {
          output.output = `[NOTICE: Subagent '${subagent}' produced empty output. Check task state via cortex_ia_work_status.]`;
        }
        return;
      }
    },
  };
};

export default CortexTaskLatchPlugin;
