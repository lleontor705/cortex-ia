// OpenCode Plugin helper ensuring default export is a valid plugin definition object for v1 and v2
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; v1?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): T => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  },
};

import { execFile, execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import { createHash } from "node:crypto";

const EXECUTABLE_LOOKUP_TIMEOUT_MS = 3000;
const TOOL_REPORT_TIMEOUT_MS = 4000;
const DEBOUNCE_WINDOW_MS = 15000;
const CACHE_CAPACITY = 256;

function resolveExecutable(cmd: string): string | null {
  const isWin = process.platform === "win32";
  const locator = isWin ? "where.exe" : "which";
  try {
    const out = execFileSync(locator, [cmd], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
      timeout: EXECUTABLE_LOOKUP_TIMEOUT_MS,
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

const recentReports = new Map<string, number>();

function shouldEmitReport(key: string, now: number): boolean {
  const prev = recentReports.get(key);
  if (prev && now - prev < DEBOUNCE_WINDOW_MS) {
    return false;
  }
  if (recentReports.size >= CACHE_CAPACITY) {
    const oldestKey = recentReports.keys().next().value;
    if (oldestKey) recentReports.delete(oldestKey);
  }
  recentReports.set(key, now);
  return true;
}

function sanitizeValue(val: unknown, maxLen = 2048): string {
  if (val === undefined || val === null) return "";
  let str = typeof val === "string" ? val : JSON.stringify(val);
  str = str.replace(/(Bearer\s+)[^\s,;"']+/gi, "$1[REDACTED]");
  str = str.replace(/(["']?(?:token|password|secret|api_key|apikey)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;]+)/gi, "$1[REDACTED]");
  if (str.length > maxLen) {
    str = str.slice(0, maxLen) + "... (truncated)";
  }
  return str;
}

export function classifyToolErrorCode(toolName: string, message: string): string {
  const lowerMsg = (message || "").toLowerCase();
  const lowerTool = (toolName || "").toLowerCase();

  if (
    lowerMsg.includes("lease_required") ||
    lowerMsg.includes("lease_check_failed") ||
    lowerMsg.includes("all native mutation targets require a live session-owned claim") ||
    lowerMsg.includes("active claim and lease")
  ) {
    return "ERR_TOOL_LEASE_REQUIRED";
  }

  if (
    (lowerTool.startsWith("cortex_") && !lowerTool.startsWith("cortex_ia_")) ||
    lowerTool.startsWith("mcp_") ||
    lowerMsg.includes("mcp error") ||
    lowerMsg.includes("storage write error") ||
    lowerMsg.includes("write could not be persisted")
  ) {
    return "ERR_TOOL_MCP_FAILED";
  }

  if (
    lowerMsg.includes("invalid argument") ||
    lowerMsg.includes("missing required") ||
    lowerMsg.includes("validation error") ||
    lowerMsg.includes("schema error") ||
    lowerMsg.includes("unexpected parameter")
  ) {
    return "ERR_TOOL_INVALID_ARGS";
  }

  return "ERR_TOOL_EXECUTION_FAILED";
}

export function extractError(event: any, output: any, rawError: any): { isError: boolean; message: string } {
  if (rawError) {
    const msg = rawError instanceof Error ? rawError.message : String(rawError);
    return { isError: true, message: msg };
  }

  if (output instanceof Error) {
    return { isError: true, message: output.message };
  }

  if (output && typeof output === "object") {
    if (output.isError === true) {
      let msg = "MCP tool execution returned error";
      if (Array.isArray(output.content)) {
        msg = output.content.map((c: any) => c.text || JSON.stringify(c)).join("\n");
      } else if (typeof output.content === "string") {
        msg = output.content;
      }
      return { isError: true, message: msg };
    }
    if (output.error) {
      const msg = typeof output.error === "string" ? output.error : JSON.stringify(output.error);
      return { isError: true, message: msg };
    }
    if (output.status === "failed" || output.status === "error") {
      const msg = output.message || output.reason || "Tool reported failed status";
      return { isError: true, message: String(msg) };
    }
  }

  if (typeof output === "string") {
    const trimmed = output.trim();
    if (trimmed.startsWith("{") && trimmed.endsWith("}")) {
      try {
        const parsed = JSON.parse(trimmed);
        if (parsed.error) {
          const msg = typeof parsed.error === "string" ? parsed.error : JSON.stringify(parsed.error);
          return { isError: true, message: msg };
        }
        if (parsed.isError === true) {
          return { isError: true, message: parsed.message || "MCP tool error" };
        }
      } catch {}
    }
    if (
      trimmed.startsWith("Error:") ||
      trimmed.startsWith("LEASE_REQUIRED") ||
      trimmed.startsWith("LEASE_CHECK_FAILED") ||
      trimmed.includes("ErrWorkConflict") ||
      trimmed.includes("target content not found")
    ) {
      return { isError: true, message: trimmed };
    }
  }

  return { isError: false, message: "" };
}

function extractTaskID(args: Record<string, any>): string | undefined {
  if (typeof args?.task_id === "string" && args.task_id.trim()) return args.task_id.trim();
  if (typeof args?.taskId === "string" && args.taskId.trim()) return args.taskId.trim();
  if (typeof args?.task === "string" && args.task.trim()) return args.task.trim();
  return undefined;
}

export function emitToolErrorReport(
  directory: string,
  toolName: string,
  errorMessage: string,
  sessionID: string,
  callID: string,
  role: string,
  args: Record<string, any>
): void {
  const executable = resolveCortexExecutable(directory);
  if (!executable) return;

  const code = classifyToolErrorCode(toolName, errorMessage);
  const now = Date.now();
  const signature = createHash("sha256")
    .update(`${sessionID}:${toolName}:${code}:${errorMessage}`)
    .digest("hex");

  if (!shouldEmitReport(signature, now)) {
    return;
  }

  const taskID = extractTaskID(args);
  const details = {
    tool: toolName,
    code,
    error_message: errorMessage.slice(0, 2048),
    session_id: sessionID,
    call_id: callID,
    role: role || "unknown",
    args_sanitized: sanitizeValue(args, 1024),
    timestamp: new Date(now).toISOString(),
  };

  const cliArgs = [
    "report", "error",
    "--code", code,
    "--message", `Tool '${toolName}' failed: ${errorMessage.slice(0, 512)}`,
    "--details", JSON.stringify(details),
    "--source", "tool-telemetry-plugin",
    "--workspace", directory,
  ];
  if (sessionID) cliArgs.push("--session-id", sessionID);
  if (role) cliArgs.push("--role", role);
  if (taskID) cliArgs.push("--task", taskID);

  try {
    execFile(
      executable,
      cliArgs,
      {
        cwd: directory,
        timeout: TOOL_REPORT_TIMEOUT_MS,
        windowsHide: true,
        stdio: ["ignore", "ignore", "ignore"],
      },
      () => {}
    );
  } catch {}
}

export const CortexToolTelemetryPlugin = async (ctx: any) => {
  const directory = (ctx as any)?.location?.directory || (ctx as any)?.directory || process.cwd();

  const handleToolError = (
    toolName: string,
    rawError: any,
    sessionID: string,
    callID: string,
    role: string,
    args: Record<string, any>
  ) => {
    if (!toolName || toolName === "cortex_ia_report_error" || toolName === "report_error") return;
    const errorMsg = rawError instanceof Error ? rawError.message : String(rawError);
    emitToolErrorReport(directory, toolName, errorMsg, sessionID, callID, role, args);
  };

  const executeAfter = async (contextOrEvent: any, maybeEvent?: any) => {
    try {
      const event = maybeEvent !== undefined ? { ...contextOrEvent, ...maybeEvent } : contextOrEvent;
      const toolName = String(event?.tool || event?.name || "").trim();
      if (!toolName || toolName === "cortex_ia_report_error" || toolName === "report_error") return;

      const sessionID = String(event?.sessionID || event?.sessionId || (ctx as any)?.session?.id || "").trim();
      const callID = String(event?.callID || event?.callId || "").trim();
      const role = String(event?.session?.role || (ctx as any)?.session?.role || process.env.CORTEX_ROLE || "").trim();
      const args = (event?.args || event?.input || {}) as Record<string, any>;
      const output = event?.output ?? event?.result;
      const rawError = event?.error || output?.error || output?.metadata?.error;

      const err = extractError(event, output, rawError);
      if (err.isError) {
        emitToolErrorReport(directory, toolName, err.message, sessionID, callID, role, args);
      }
    } catch {}
  };

  if (ctx?.tool?.hook) {
    if (typeof ctx.tool.hook === "function") {
      try {
        await ctx.tool.hook("execute.after", executeAfter);
      } catch {}

      try {
        await ctx.tool.hook(async (event: any, next: any) => {
          if (typeof next !== "function") return;
          try {
            const result = await next();
            const toolName = String(event?.tool || event?.name || "").trim();
            const sessionID = String(event?.sessionID || event?.sessionId || "").trim();
            const callID = String(event?.callID || event?.callId || "").trim();
            const role = String(event?.session?.role || "").trim();
            const args = (event?.input || event?.args || {}) as Record<string, any>;
            const err = extractError(event, result, undefined);
            if (err.isError) {
              emitToolErrorReport(directory, toolName, err.message, sessionID, callID, role, args);
            }
            return result;
          } catch (err: any) {
            const toolName = String(event?.tool || event?.name || "").trim();
            const sessionID = String(event?.sessionID || event?.sessionId || "").trim();
            const callID = String(event?.callID || event?.callId || "").trim();
            const role = String(event?.session?.role || "").trim();
            const args = (event?.input || event?.args || {}) as Record<string, any>;
            handleToolError(toolName, err, sessionID, callID, role, args);
            throw err;
          }
        });
      } catch {}
    }
  }

  const cleanup = async () => {
    recentReports.clear();
  };
  (cleanup as any).dispose = cleanup;
  (cleanup as any)["tool.execute.after"] = executeAfter;
  return cleanup;
};

export const CortexToolTelemetryPluginDefinition = {
  id: "cortex-tool-telemetry",
  setup: CortexToolTelemetryPlugin,
  server: CortexToolTelemetryPlugin,
};

export default CortexToolTelemetryPluginDefinition;
