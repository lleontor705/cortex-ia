import { type Plugin, tool } from "@opencode-ai/plugin";
import { execFileSync, execSync, spawn, type ChildProcess } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

const receiptSchema = {
  type: "object",
  additionalProperties: false,
  required: ["phase_status", "execution_status", "verification_verdict", "summary"],
  properties: {
    phase_status: { type: "string" },
    execution_status: { type: "string", enum: ["completed", "partial", "failed", "blocked", "unverified"] },
    verification_verdict: { type: "string" },
    summary: { type: "string" },
    changed_files: { type: "array", items: { type: "string" } },
    checks: { type: "array", items: { type: "string" } }
  }
};

interface SubagentTrack {
  id: string;
  role?: string;
  task_id?: string;
  startedAt: number;
}

const activeSubagents = new Map<string, SubagentTrack>();
const subagentStartTimes = new Map<string, number>();
interface WorkAuthority {
  claimToken: string;
  leases: Map<string, string>;
  sessionID: string;
}

const workAuthority = new Map<string, WorkAuthority>();
const controllerIdentity = (sessionID: string) => `opencode-session:${sessionID}`;
const logDelegation = logLifecycle;

function configRoot(): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  return path.resolve(home, ".config", "opencode");
}

const activeJobPanes = new Map<string, string>();
const activeJobTabs = new Map<string, string>();

function rememberJobPane(jobID: string, paneID: string) {
  activeJobPanes.set(jobID, paneID);
  try {
    cortex(["delegate", "set-pane", jobID, paneID]);
  } catch {}
}

function forgetJobPane(jobID: string) {
  activeJobPanes.delete(jobID);
}

function rememberJobTab(jobID: string, tabID: string) {
  activeJobTabs.set(jobID, tabID);
}

function forgetJobTab(jobID: string) {
  activeJobTabs.delete(jobID);
}

function closeHerdrJobResources(jobID: string) {
  let herdr = "";
  try { herdr = firstExecutable("herdr"); } catch {}

  const tabID = activeJobTabs.get(jobID);
  if (tabID) {
    if (herdr) {
      try {
        execFileSync(herdr, ["tab", "close", tabID], { stdio: "ignore", windowsHide: true });
      } catch {}
    }
    forgetJobTab(jobID);
    forgetJobPane(jobID);
    return;
  }

  const paneID = activeJobPanes.get(jobID);
  if (paneID) {
    if (herdr) {
      try {
        execFileSync(herdr, ["pane", "close", paneID], { stdio: "ignore", windowsHide: true });
      } catch {}
    }
    forgetJobPane(jobID);
  }
}

function saveAuthorityState() {
  // Pure in-memory authority tracking; SQLite delegation.db is authoritative
}

function emitDelegationEvent(_event: Record<string, unknown>) {
  // SQLite delegation_events is authoritative; no flat file needed
}

function withoutToken(value: any, tokenField: string): any {
  if (!value || typeof value !== "object") return value;
  const copy = { ...value };
  delete copy[tokenField];
  return copy;
}

function workPath(value: string): string {
  let normalized = path.posix.normalize(value.replaceAll("\\", "/")).replace(/^\.\//, "");
  if (process.platform === "win32" || process.platform === "darwin") normalized = normalized.toLowerCase();
  return normalized;
}

function durableWorkStatus(taskID: string): any {
  return parseJSON(cortex(["work", "status", taskID]));
}

function bridgeAuthorityView(taskID: string, durable: any, sessionID: string) {
  const authority = workAuthority.get(taskID);
  const claimExpiresAt = typeof durable?.claim?.expires_at === "string" ? durable.claim.expires_at : "";
  const expiresAt = Date.parse(claimExpiresAt);
  const durableClaimLive = Number.isFinite(expiresAt) && expiresAt > Date.now();
  const durableOwner = typeof durable?.claim?.owner === "string" ? durable.claim.owner : "";
  const handlePresent = Boolean(authority);
  const ownedByCurrentSession = authority?.sessionID === sessionID;
  const durableOwnerMatchesBridge = durableOwner !== "" && durableOwner === controllerIdentity(sessionID);
  const usable = Boolean(
    handlePresent
    && ownedByCurrentSession
    && durableOwnerMatchesBridge
    && durableClaimLive
  );
  const writeUsable = usable && durable?.status === "in_progress";
  return {
    handle_present: handlePresent,
    owned_by_current_session: ownedByCurrentSession,
    durable_owner_matches_bridge: durableOwnerMatchesBridge,
    durable_claim_live: durableClaimLive,
    usable,
    write_usable: writeUsable,
    retained_lease_paths: authority ? [...authority.leases.keys()].sort() : [],
    action: writeUsable
      ? "CONTINUE_WITH_HEARTBEAT"
      : usable
        ? "NON_WRITING_AUTHORITY_ONLY"
        : "STOP_WRITING_AND_RECONCILE"
  };
}

function authorityFailure(code: string, taskID: string, sessionID: string, leasePath?: string): Error {
  let durable: any = {};
  try { durable = durableWorkStatus(taskID); } catch {}
  return new Error(JSON.stringify({
    code,
    task_id: taskID,
    ...(leasePath ? { path: leasePath } : {}),
    durable_status: durable?.status || "unknown",
    revision: durable?.revision,
    claim_expires_at: durable?.claim?.expires_at,
    bridge_authority: bridgeAuthorityView(taskID, durable, sessionID),
    action: "RECONCILE_WORK_THEN_RETRY_WITH_FRESH_AUTHORITY"
  }));
}

function authorityForSession(taskID: string, sessionID: string, requireWriting = false): WorkAuthority {
  let durable: any;
  try {
    durable = durableWorkStatus(taskID);
  } catch {
    throw authorityFailure("WORK_STATUS_UNAVAILABLE", taskID, sessionID);
  }
  const view = bridgeAuthorityView(taskID, durable, sessionID);
  if (!view.usable || (requireWriting && !view.write_usable)) {
    throw authorityFailure(requireWriting ? "BRIDGE_WRITE_AUTHORITY_UNUSABLE" : "BRIDGE_AUTHORITY_UNUSABLE", taskID, sessionID);
  }
  return workAuthority.get(taskID)!;
}

function logLifecycle(msg: string) {
  const line = `[${new Date().toISOString()}] ${msg}\n`;
  try {
    const home = process.env.USERPROFILE || process.env.HOME || "";
    const logPath = path.resolve(home, ".config", "opencode", "cortex-delegation.log");
    fs.appendFileSync(logPath, line, "utf-8");
  } catch {}
}

function firstExecutable(name: "cortex-ia" | "herdr"): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = name === "herdr"
    ? ["herdr", path.join(local, "Programs", "Herdr", "bin", "herdr.exe"), path.join(home, ".cargo", "bin", "herdr.exe"), "/usr/local/bin/herdr", "/usr/bin/herdr"]
    : [
        path.join(home, "go", "bin", "cortex-ia.exe"),
        "cortex-ia",
        path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
        path.join(home, ".local", "bin", "cortex-ia"),
        "/usr/local/bin/cortex-ia",
        "/usr/bin/cortex-ia"
      ];
  for (const candidate of candidates) {
    if (candidate !== name && fs.existsSync(candidate)) return candidate;
    if (candidate === name) {
      try {
        execFileSync(candidate, name === "herdr" ? ["--version"] : ["version"], { stdio: "ignore", windowsHide: true });
        return candidate;
      } catch {}
    }
  }
  throw new Error(`${name} executable not found`);
}

function cortex(args: string[], cwd?: string): string {
  try {
    return execFileSync(firstExecutable("cortex-ia"), args, {
      encoding: "utf-8",
      cwd,
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"]
    });
  } catch (err: any) {
    const msg = String(err?.message || err);
    if (msg.includes("uv_spawn") || msg.includes("EUNKNOWN")) {
      try {
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 200);
        return execFileSync(firstExecutable("cortex-ia"), args, {
          encoding: "utf-8",
          cwd,
          windowsHide: true,
          stdio: ["ignore", "pipe", "pipe"]
        });
      } catch {}
    }
    throw err;
  }
}

function cortexAuthorized(args: string[], token: string): string {
  return execFileSync(firstExecutable("cortex-ia"), args, {
    encoding: "utf-8",
    input: token,
    windowsHide: true,
    stdio: ["pipe", "pipe", "pipe"]
  });
}

function cortexInput(args: string[], input: string): string {
  return execFileSync(firstExecutable("cortex-ia"), args, {
    encoding: "utf-8",
    input,
    windowsHide: true,
    stdio: ["pipe", "pipe", "pipe"]
  });
}

function parseJSON(text: string): any {
  return JSON.parse(text.trim());
}

function transientRequest(value: object): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cortex-delegation-"));
  const requestPath = path.join(dir, "request.json");
  fs.writeFileSync(requestPath, JSON.stringify(value), { encoding: "utf-8", mode: 0o600, flag: "wx" });
  return requestPath;
}

function cleanupRequest(requestPath: string) {
  try { fs.rmSync(path.dirname(requestPath), { recursive: true, force: true }); } catch {}
}

function parseTabCreateResult(output: string): { paneID: string; tabID: string } {
  const parsed = parseJSON(output);
  const paneID = parsed?.result?.root_pane?.pane_id || parsed?.result?.pane?.pane_id || parsed?.result?.pane_id || "";
  const tabID = parsed?.result?.tab?.tab_id || parsed?.result?.root_pane?.tab_id || parsed?.tab_id || "";
  return { paneID, tabID };
}

function bridgeConfig(): {
  useHerdr: boolean;
  direction: "right" | "down";
  presentation: "tab" | "split";
  autoClose: boolean;
} {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  try {
    const value = JSON.parse(fs.readFileSync(path.join(home, ".config", "opencode", "cortex-delegation.json"), "utf-8"));
    const direction = value?.herdr_settings?.split_direction === "down" ? "down" : "right";
    const rawPresentation = value?.herdr_settings?.presentation;
    const presentation = rawPresentation === "split" ? "split" : "tab";
    const autoClose = value?.herdr_settings?.auto_close !== false;
    return {
      useHerdr: value?.use_herdr === true,
      direction,
      presentation,
      autoClose
    };
  } catch {
    return { useHerdr: false, direction: "right", presentation: "tab", autoClose: true };
  }
}

function isHerdrInUse(): boolean {
  return Boolean(
    process.env.HERDR_PANE_ID ||
    process.env.HERDR_WORKSPACE_ID ||
    process.env.HERDR_ENV === "1"
  );
}

function hasExecutable(name: string): boolean {
  try {
    const isWin = process.platform === "win32";
    const cmd = isWin ? `where.exe ${name}` : `which ${name}`;
    execSync(cmd, { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

function launchNativeTerminal(
  cortexBin: string,
  workerArgs: string[],
  cwd: string,
  title: string
): ChildProcess {
  const isWin = process.platform === "win32";
  const isMac = process.platform === "darwin";
  const isLinux = process.platform === "linux";

  if (isWin) {
    const quotedArgs = workerArgs.map((arg) => (arg.includes(" ") ? `"${arg}"` : arg)).join(" ");
    const fullCmd = `"${cortexBin}" ${quotedArgs}`;

    // 1. Try Windows Terminal (wt.exe) if available
    if (hasExecutable("wt.exe")) {
      try {
        return spawn("wt.exe", ["-d", cwd, "--title", title, "cmd.exe", "/c", `${fullCmd} & pause`], {
          detached: true,
          stdio: "ignore",
          windowsHide: false
        });
      } catch {}
    }
    // 2. Universal Windows fallback: cmd.exe /c start with quoted title
    return spawn("cmd.exe", ["/c", "start", `"${title}"`, "cmd.exe", "/c", `${fullCmd} & pause`], {
      cwd,
      detached: true,
      stdio: "ignore",
      windowsHide: false
    });
  }

  if (isMac) {
    const quotedArgs = workerArgs.map((arg) => (arg.includes(" ") ? `"${arg}"` : arg)).join(" ");
    const cmdString = `cd "${cwd}" && "${cortexBin}" ${quotedArgs}`;

    if (hasExecutable("ghostty")) {
      return spawn("ghostty", ["-e", cortexBin, ...workerArgs], { cwd, detached: true, stdio: "ignore" });
    }
    if (hasExecutable("wezterm")) {
      return spawn("wezterm", ["start", "--cwd", cwd, "--", cortexBin, ...workerArgs], { detached: true, stdio: "ignore" });
    }
    if (hasExecutable("kitty")) {
      return spawn("kitty", ["--title", title, "--directory", cwd, cortexBin, ...workerArgs], { detached: true, stdio: "ignore" });
    }
    if (hasExecutable("alacritty")) {
      return spawn("alacritty", ["--title", title, "--working-directory", cwd, "-e", cortexBin, ...workerArgs], { detached: true, stdio: "ignore" });
    }
    if (fs.existsSync("/Applications/iTerm.app")) {
      const script = `tell application "iTerm" to create window with default profile command "${cmdString.replace(/"/g, '\\"')}"`;
      return spawn("osascript", ["-e", script], { detached: true, stdio: "ignore" });
    }
    // Default macOS Terminal.app
    const script = `tell application "Terminal" to do script "${cmdString.replace(/"/g, '\\"')}"`;
    return spawn("osascript", ["-e", script], { detached: true, stdio: "ignore" });
  }

  if (isLinux) {
    const hasDisplay = Boolean(process.env.DISPLAY || process.env.WAYLAND_DISPLAY);
    if (!hasDisplay) {
      // Headless / SSH fallback: spawn background process directly
      return spawn(cortexBin, workerArgs, { cwd, detached: true, stdio: "ignore" });
    }
    // User preference via $TERMINAL
    if (process.env.TERMINAL && hasExecutable(process.env.TERMINAL)) {
      return spawn(process.env.TERMINAL, ["-e", cortexBin, ...workerArgs], { cwd, detached: true, stdio: "ignore" });
    }
    // Standard Linux terminal emulators in search priority
    const linuxTerminals = [
      { bin: "x-terminal-emulator", args: ["-e", cortexBin, ...workerArgs] },
      { bin: "ghostty", args: ["-e", cortexBin, ...workerArgs] },
      { bin: "wezterm", args: ["start", "--cwd", cwd, "--", cortexBin, ...workerArgs] },
      { bin: "kitty", args: ["--title", title, "--directory", cwd, cortexBin, ...workerArgs] },
      { bin: "alacritty", args: ["--title", title, "--working-directory", cwd, "-e", cortexBin, ...workerArgs] },
      { bin: "gnome-terminal", args: ["--working-directory=" + cwd, "--title=" + title, "--", cortexBin, ...workerArgs] },
      { bin: "konsole", args: ["--workdir", cwd, "-e", cortexBin, ...workerArgs] },
      { bin: "xfce4-terminal", args: ["--default-working-directory=" + cwd, "-e", `${cortexBin} ${workerArgs.join(" ")}`] },
      { bin: "foot", args: ["-D", cwd, cortexBin, ...workerArgs] },
      { bin: "xterm", args: ["-title", title, "-e", cortexBin, ...workerArgs] }
    ];
    for (const term of linuxTerminals) {
      if (hasExecutable(term.bin)) {
        return spawn(term.bin, term.args, { cwd, detached: true, stdio: "ignore" });
      }
    }
    return spawn(cortexBin, workerArgs, { cwd, detached: true, stdio: "ignore" });
  }

  return spawn(cortexBin, workerArgs, { cwd, detached: true, stdio: "ignore" });
}

function paneID(output: string): string {
  const parsed = parseJSON(output);
  return parsed?.result?.pane?.pane_id || parsed?.result?.pane_id || parsed?.pane_id || "";
}

type ExecutionMode = "native" | "direct_cli" | "herdr_multiplexed";

function executionMode(transport: "direct" | "herdr"): ExecutionMode {
  return transport === "herdr" ? "herdr_multiplexed" : "direct_cli";
}

export function sanitizeUnicode(str: string): string {
  if (typeof str !== "string") return "";
  if (typeof (str as any).toWellFormed === "function") {
    return (str as any).toWellFormed();
  }
  return str.replace(/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/g, "\uFFFD");
}

export function truncateUTF8Bytes(str: string, maxBytes: number): { text: string; truncated: boolean } {
  if (!str) return { text: "", truncated: false };
  const clean = sanitizeUnicode(str);
  if (Buffer.byteLength(clean, "utf-8") <= maxBytes) {
    return { text: clean, truncated: false };
  }
  let currentBytes = 0;
  let result = "";
  for (const char of clean) {
    const charBytes = Buffer.byteLength(char, "utf-8");
    if (currentBytes + charBytes > maxBytes) {
      return { text: result, truncated: true };
    }
    result += char;
    currentBytes += charBytes;
  }
  return { text: result, truncated: false };
}

export function safeUnicodeSlice(str: string, maxChars: number): string {
  if (!str) return "";
  const clean = sanitizeUnicode(str);
  const chars = Array.from(clean);
  if (chars.length <= maxChars) return clean;
  return chars.slice(0, maxChars).join("") + "...";
}

export function extractCompactReceipt(job: any, res: any, jobID: string): any {
  const authoritativeStatus = job?.status || res?.status || "unknown";
  const isSuccess = authoritativeStatus === "succeeded";
  const exitCode = typeof job?.exit_code === "number" ? job.exit_code
    : typeof res?.exit_code === "number" ? res.exit_code
    : (isSuccess ? 0 : 1);

  let rawOutput = res?.output;
  if (typeof rawOutput === "string") {
    try {
      const parsed = JSON.parse(rawOutput);
      if (parsed && typeof parsed === "object") rawOutput = parsed;
    } catch {}
  }

  const candidate = (rawOutput?.structured_output && typeof rawOutput.structured_output === "object") ? rawOutput.structured_output
    : (res?.structured_output && typeof res.structured_output === "object") ? res.structured_output
    : (rawOutput && typeof rawOutput === "object" && (rawOutput.phase_status !== undefined || rawOutput.summary !== undefined || rawOutput.execution_status !== undefined || rawOutput.verification_verdict !== undefined || rawOutput.verdict !== undefined || rawOutput.checks !== undefined || rawOutput.changed_files !== undefined)) ? rawOutput
    : (res && typeof res === "object" && (res.phase_status !== undefined || res.summary !== undefined || res.execution_status !== undefined || res.verification_verdict !== undefined || res.verdict !== undefined || res.checks !== undefined || res.changed_files !== undefined)) ? res
    : null;

  const validExecutionStatuses = new Set(["completed", "partial", "failed", "blocked", "unverified"]);
  const rawExecutionStatus = typeof candidate?.execution_status === "string" ? candidate.execution_status.trim() : "";
  const execution_status = validExecutionStatuses.has(rawExecutionStatus) ? rawExecutionStatus : undefined;

  const rawPhaseStatus = typeof candidate?.phase_status === "string" ? candidate.phase_status.trim() : "";
  const phase_status = rawPhaseStatus.length > 0 ? rawPhaseStatus : undefined;

  const rawVerdict = candidate?.verification_verdict !== undefined ? candidate.verification_verdict : candidate?.verdict;
  const verification_verdict = typeof rawVerdict === "string" && rawVerdict.trim().length > 0 ? rawVerdict.trim() : undefined;

  let rawSummary = "";
  if (typeof candidate?.summary === "string" && candidate.summary.trim().length > 0) {
    rawSummary = candidate.summary.trim();
  } else {
    const rawFallback = rawOutput?.response
      || rawOutput?.text
      || res?.response
      || res?.text
      || res?.error
      || (typeof rawOutput === "string" ? rawOutput : "")
      || job?.error_message
      || "";
    rawSummary = typeof rawFallback === "string" ? rawFallback.trim() : (rawFallback ? String(rawFallback).trim() : "");
  }

  const { text: summary, truncated: summaryTruncated } = truncateUTF8Bytes(rawSummary, 2048);

  const changed_files = Array.isArray(candidate?.changed_files) ? candidate.changed_files.map(String) : undefined;
  const checks = Array.isArray(candidate?.checks) ? candidate.checks.map(String) : undefined;

  const output: any = {
    summary,
    details_omitted: true
  };
  if (phase_status !== undefined) output.phase_status = phase_status;
  if (execution_status !== undefined) output.execution_status = execution_status;
  if (verification_verdict !== undefined) output.verification_verdict = verification_verdict;
  if (changed_files !== undefined) output.changed_files = changed_files;
  if (checks !== undefined) output.checks = checks;
  if (summaryTruncated) output.truncated = true;

  const id = job?.job_id || res?.job_id || jobID;
  const compactResult: any = {
    job_id: id,
    full_result_job_id: id,
    status: authoritativeStatus,
    exit_code: exitCode,
    compact: true,
    details_omitted: true,
    summary,
    output,
    structured_output: output
  };

  if (phase_status !== undefined) compactResult.phase_status = phase_status;
  if (execution_status !== undefined) compactResult.execution_status = execution_status;
  if (verification_verdict !== undefined) compactResult.verification_verdict = verification_verdict;
  if (changed_files !== undefined) compactResult.changed_files = changed_files;
  if (checks !== undefined) compactResult.checks = checks;
  if (summaryTruncated) compactResult.truncated = true;

  if (res?.output_hash) compactResult.output_hash = res.output_hash;
  if (res?.created_at || job?.created_at) compactResult.created_at = res?.created_at || job?.created_at;
  if (job?.error_code) compactResult.error_code = job.error_code;
  if (job?.error_message) compactResult.error_message = job.error_message;
  if (!res && isSuccess) compactResult.receipt_missing = true;

  return compactResult;
}

export const CortexDelegationBridge: Plugin = async ({ client }) => {
  const aliases = process.env.CORTEX_IA_LEGACY_TOOL_ALIASES;
  if (aliases !== undefined && aliases !== "true" && aliases !== "false") throw new Error("BRIDGE_CONFIG_INVALID: CORTEX_IA_LEGACY_TOOL_ALIASES must be true or false");
  // Only host session metadata proves ancestry; tool arguments and transcripts
  // must never supply conversation ownership.
  const conversationOwnership = async (sessionID: string, directory: string) => {
    const seen = new Set<string>();
    let current = sessionID;
    let parent = "";
    while (seen.size < 64 && /^[A-Za-z0-9_-]{1,256}$/.test(current) && !seen.has(current)) {
      seen.add(current);
      const result = await client.session.get({ path: { id: current }, query: { directory } });
      const session = result.data;
      if (!session || session.id !== current) break;
      if (current === sessionID) parent = session.parentID || "";
      if (!session.parentID) {
        return { opencode_session_id: sessionID, opencode_root_session_id: current, opencode_parent_session_id: parent };
      }
      current = session.parentID;
    }
    throw new Error("OpenCode conversation ancestry is unavailable");
  };
  return ({
  dispose: async () => {
    workAuthority.clear();
    saveAuthorityState();
  },

  event: async ({ event }: { event?: any }) => {
    if (!event) return;
    const type = event.type || event.event || "";
    const sessionID = event.sessionID || event.sessionId || event.properties?.sessionID || event.properties?.info?.id || "";

    // 1. Detectar inicio de subagente o subtask
    if (type === "session.created" && event.properties?.info?.parentID) {
      const role = event.properties?.info?.role || event.properties?.info?.agent || "subagent";
      subagentStartTimes.set(sessionID, Date.now());
      activeSubagents.set(sessionID, { id: sessionID, role, startedAt: Date.now() });
      logDelegation(`🚀 [CORTEX-IA] Subagente iniciado: '${role}' (Session: ${sessionID})`);
    } else if (type === "subtask.created") {
      const taskID = event.properties?.id || "subtask";
      subagentStartTimes.set(taskID, Date.now());
      activeSubagents.set(taskID, { id: taskID, role: event.properties?.agent || "subtask", startedAt: Date.now() });
      logDelegation(`🚀 [CORTEX-IA] Tarea en background iniciada: '${taskID}'`);
    }

    // 2. Detectar finalización de subagente o subtask
    if (type === "session.idle" || type === "subtask.completed" || type === "session.error") {
      const trackingID = type === "subtask.completed" ? event.properties?.id || "subtask" : sessionID;
      const tracked = activeSubagents.get(trackingID);
      const startTime = subagentStartTimes.get(trackingID);
      const durationSec = startTime ? Math.round((Date.now() - startTime) / 1000) : 0;
      const status = type === "session.error" ? "ERROR" : "COMPLETED";
      const role = tracked?.role || event.properties?.info?.role || event.properties?.info?.agent || "subagent";
      
      if (startTime) {
        logDelegation(`✅ [CORTEX-IA] Subagente '${role}' finalizado en ${durationSec}s (Estado: ${status})`);
        subagentStartTimes.delete(trackingID);
        activeSubagents.delete(trackingID);
      }
    }
  },

  "experimental.session.compacting": async (_input: any, output: any) => {
    try {
      // Inyectar snapshot del DAG y tareas activas antes de la compactación de contexto
      const activeState = cortex(["work", "list"]);
      if (activeState && output?.context) {
        output.context.push({
          role: "system",
          content: `[CORTEX-IA STATE SNAPSHOT BEFORE COMPACTION]\nActive Work DAG State:\n${activeState}`
        });
        logDelegation("🧠 [CORTEX-IA] Snapshot de estado DAG inyectado previo a la compactación de sesión.");
      }
    } catch {}
  },

  tool: (() => {
    const bridgeTools = {
    cortex_ia_content_hash: tool({
      description: "Compute lowercase SHA-256 of exact UTF-8 content (at most 1 MiB). No newline/Unicode normalization, persistence, or network access. Returns only sha256 and byte_length.",
      args: { content: tool.schema.string() },
      async execute(args) {
        const byteLength = Buffer.byteLength(args.content, "utf-8");
        if (byteLength > 1048576) throw new Error("content hash input exceeds 1 MiB (1048576 UTF-8 bytes)");
        const bytes = Buffer.from(args.content, "utf-8");
        if (bytes.toString("utf-8") !== args.content) throw new Error("content hash requires well-formed Unicode for exact UTF-8 encoding");
        return JSON.stringify({ sha256: createHash("sha256").update(bytes).digest("hex"), byte_length: byteLength });
      }
    }),

    cortex_ia_openspec_validate: tool({
      description: "Validate OpenSpec planning artifacts from the active workspace without exposing a shell.",
      args: {
        relative_directory: tool.schema.string(),
        workflow: tool.schema.enum(["sdd-lite", "sdd-full", "decision-map"]),
        phase: tool.schema.enum(["integrated", "propose", "spec", "design", "tasks", "chart", "resolve"])
      },
      async execute(args, context) {
        const command = ["openspec", "validate", args.relative_directory, "--workflow", args.workflow, "--phase", args.phase, "--json"];
        return cortex(command, context.directory);
      }
    }),

    cortex_ia_change_archive: tool({
      description: "Close one SDD change after durable task approval and current contract/file fingerprint checks. Cortex-only closure writes a logical receipt; OpenSpec/hybrid also archive the change directory. Planner only.",
      args: {
        board_id: tool.schema.string(), change_id: tool.schema.string(),
        workflow: tool.schema.enum(["sdd-lite", "sdd-full"]),
        spec_plane: tool.schema.enum(["cortex", "openspec", "hybrid"])
      },
      async execute(args, context) {
        return cortex(["work", "archive", "--board", args.board_id, "--project", path.resolve(context.directory),
          "--change", args.change_id, "--workflow", args.workflow, "--spec-plane", args.spec_plane], context.directory);
      }
    }),

    cortex_ia_openspec_write: tool({
      description: "Write one Markdown planning artifact under openspec/changes. This tool cannot modify product code.",
      args: { relative_path: tool.schema.string(), content: tool.schema.string() },
      async execute(args, context) {
        const relative = args.relative_path.replaceAll("\\", "/");
        const clean = path.posix.normalize(relative);
        if (!clean.startsWith("openspec/changes/") || !clean.endsWith(".md") || clean.includes("/../")) {
          throw new Error("OpenSpec writes must target a Markdown file under openspec/changes/");
        }
        const root = path.resolve(context.directory);
        const target = path.resolve(root, ...clean.split("/"));
        const allowedRoot = path.resolve(root, "openspec", "changes") + path.sep;
        const comparableTarget = process.platform === "win32" || process.platform === "darwin" ? target.toLowerCase() : target;
        const comparableRoot = process.platform === "win32" || process.platform === "darwin" ? allowedRoot.toLowerCase() : allowedRoot;
        if (!comparableTarget.startsWith(comparableRoot)) throw new Error("OpenSpec path escaped its managed root");
        fs.mkdirSync(path.dirname(target), { recursive: true });
        const temporary = `${target}.${process.pid}.tmp`;
        fs.writeFileSync(temporary, args.content, { encoding: "utf-8", mode: 0o644 });
        fs.renameSync(temporary, target);
        return JSON.stringify({ written: clean, bytes: Buffer.byteLength(args.content, "utf-8") });
      }
    }),

    cortex_ia_discovery_write: tool({
      description: "Write the complete project-local .cortex-ia/discovery.md report atomically. This tool is reserved for the discovery agent.",
      args: { content: tool.schema.string() },
      async execute(args, context) {
        if (context.agent !== "discovery") {
          throw new Error("only the discovery agent may write the project discovery report");
        }
        const content = args.content.trim();
        const output = `${content}\n`;
        const bytes = Buffer.byteLength(output, "utf-8");
        if (!content.startsWith("# Cortex-IA Project Discovery")) {
          throw new Error("discovery report must start with the canonical heading");
        }
        if (bytes === 0 || bytes > 128 * 1024) {
          throw new Error("discovery report must contain 1-131072 UTF-8 bytes");
        }
        const root = path.resolve(context.directory);
        const directory = path.join(root, ".cortex-ia");
        const target = path.join(directory, "discovery.md");
        const comparableTarget = process.platform === "win32" || process.platform === "darwin" ? target.toLowerCase() : target;
        const comparableRoot = process.platform === "win32" || process.platform === "darwin" ? `${directory.toLowerCase()}${path.sep}` : `${directory}${path.sep}`;
        if (!comparableTarget.startsWith(comparableRoot)) {
          throw new Error("discovery report escaped the project-local .cortex-ia directory");
        }
        fs.mkdirSync(directory, { recursive: true });
        const temporary = `${target}.${process.pid}.${randomUUID()}.tmp`;
        try {
          fs.writeFileSync(temporary, output, { encoding: "utf-8", mode: 0o644 });
          fs.renameSync(temporary, target);
        } finally {
          try { if (fs.existsSync(temporary)) fs.unlinkSync(temporary); } catch {}
        }
        return JSON.stringify({
          artifact: ".cortex-ia/discovery.md",
          bytes,
          sha256: createHash("sha256").update(output, "utf-8").digest("hex")
        });
      }
    }),

    cortex_ia_board_create: tool({
      description: "Create one durable Cortex-IA initiative board.",
      args: { board_id: tool.schema.string(), title: tool.schema.string(), description: tool.schema.string().optional() },
      async execute(args, context) {
        const command = ["board", "create", "--id", args.board_id, "--title", args.title];
        if (args.description) command.push("--description", args.description);
        return cortex(command);
      }
    }),

    cortex_ia_board_list: tool({
      description: "List durable Cortex-IA initiative boards.",
      args: {},
      async execute() { return cortex(["board", "list"]); }
    }),

    cortex_ia_board_status: tool({
      description: "Read one board and its authoritative task snapshot.",
      args: { board_id: tool.schema.string() },
      async execute(args) { return cortex(["board", "status", args.board_id]); }
    }),

    cortex_ia_ledger_status: tool({
      description: "Read the dual ledger report (Task Ledger facts and Progress Ledger cycle evaluations) for an initiative board.",
      args: { board_id: tool.schema.string().optional() },
      async execute(args) {
        const command = ["ledger", "status"];
        if (args.board_id) command.push("--board", args.board_id);
        return cortex(command);
      }
    }),

    cortex_ia_ledger_fact_add: tool({
      description: "Record an authoritative environmental or technical fact into the Task Ledger. Optionally syncs as a durable observation in Cortex Memory.",
      args: {
        fact: tool.schema.string(),
        board_id: tool.schema.string().optional(),
        source: tool.schema.string().optional(),
        sync_cortex: tool.schema.boolean().optional().describe("If true, also syncs this fact as a persistent discovery observation in Cortex memory")
      },
      async execute(args) {
        const command = ["ledger", "fact", "add", args.fact];
        if (args.board_id) command.push("--board", args.board_id);
        if (args.source) command.push("--source", args.source);
        if (args.sync_cortex) command.push("--sync-cortex");
        return cortex(command);
      }
    }),

    cortex_ia_ledger_progress_record: tool({
      description: "Record an orchestrator progress evaluation, drift detection, and intended action into the Progress Ledger.",
      args: { summary: tool.schema.string(), cycle: tool.schema.number().optional(), drift: tool.schema.boolean().optional(), action: tool.schema.string().optional(), board_id: tool.schema.string().optional() },
      async execute(args) {
        const command = ["ledger", "progress", "record", "--summary", args.summary];
        if (args.board_id) command.push("--board", args.board_id);
        if (args.action) command.push("--action", args.action);
        if (args.cycle) command.push("--cycle", String(args.cycle));
        if (args.drift) command.push("--drift");
        return cortex(command);
      }
    }),

    cortex_ia_work_create: tool({
      description: "Create one work item in a durable same-board DAG.",
      args: {
        workflow: tool.schema.enum(["direct-change", "fast-tdd", "hotfix", "sdd-lite", "sdd-full"]),
        board_id: tool.schema.string(),
        task_id: tool.schema.string(),
        title: tool.schema.string(),
        objective: tool.schema.string().optional(),
        acceptance_criteria: tool.schema.string().optional(),
        verification: tool.schema.string().optional(),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        dependencies: tool.schema.array(tool.schema.string()).optional(),
        sdd_contract: tool.schema.object({
          version: tool.schema.number(), workflow: tool.schema.enum(["sdd-lite", "sdd-full"]),
          change_id: tool.schema.string(), spec_plane: tool.schema.enum(["cortex", "openspec", "hybrid"]),
          pins: tool.schema.array(tool.schema.object({ transport: tool.schema.string(), project: tool.schema.string(), locator: tool.schema.string(), sha256: tool.schema.string() })),
          requirement_ids: tool.schema.array(tool.schema.string())
        }).optional().describe("Required for SDD tasks: exact contract pins and requirement IDs; omitted only for direct/legacy work")
      },
      async execute(args, context) {
        const sdd = args.workflow === "sdd-lite" || args.workflow === "sdd-full";
        if (sdd ? args.sdd_contract?.workflow !== args.workflow : args.sdd_contract !== undefined) throw new Error("SDD_CONTRACT_REQUIRED: workflow and typed contract must agree; direct workflows cannot carry SDD bindings");
        const ownership = await conversationOwnership(context.sessionID, context.directory);
        const command = ["work", "create", "--board", args.board_id, "--id", args.task_id, "--title", args.title,
          "--workflow", args.workflow,
          "--project", path.resolve(context.directory), "--opencode-session-id", ownership.opencode_session_id,
          "--opencode-root-session-id", ownership.opencode_root_session_id];
        if (ownership.opencode_parent_session_id) command.push("--opencode-parent-session-id", ownership.opencode_parent_session_id);
        if (args.objective) command.push("--objective", args.objective);
        if (args.acceptance_criteria) command.push("--acceptance", args.acceptance_criteria);
        if (args.verification) command.push("--verify", args.verification);
        for (const file of args.allowed_files || []) command.push("--file", file);
        for (const dependency of args.dependencies || []) command.push("--depends", dependency);
        let contractPath = "";
        try {
          if (args.sdd_contract) {
            contractPath = transientRequest(args.sdd_contract);
            command.push("--contract-file", contractPath);
          }
          return cortex(command);
        } finally { if (contractPath) cleanupRequest(contractPath); }
      }
    }),

    cortex_ia_work_review_refresh: tool({
      description: "Reopen a done SDD task for fresh independent review after approved files change. Requires current revision; does not authorize writes or approve anything. Orchestrator only.",
      args: { task_id: tool.schema.string(), revision: tool.schema.number() },
      async execute(args) {
        return cortex(["work", "review-refresh", args.task_id, "--revision", String(args.revision)]);
      }
    }),

    cortex_ia_work_list: tool({
      description: "List durable work items, optionally restricted to one board.",
      args: { board_id: tool.schema.string().optional() },
      async execute(args) {
        const command = ["work", "list"];
        if (args.board_id) command.push("--board", args.board_id);
        return cortex(command);
      }
    }),

    cortex_ia_work_status: tool({
      description: "Read one durable Cortex-IA work item plus token-free bridge authority usability for the current session. Returns not_found if the task does not exist in SQLite.",
      args: { task_id: tool.schema.string() },
      async execute(args, context) {
        try {
          const durable = durableWorkStatus(args.task_id);
          return JSON.stringify({ ...durable, found: true, bridge_authority: bridgeAuthorityView(args.task_id, durable, context.sessionID) });
        } catch (error: any) {
          const stderr = (error?.stderr?.toString?.() || error?.message || "");
          if (stderr.includes("work item not found")) {
            return JSON.stringify({
              task_id: args.task_id,
              found: false,
              status: "not_found",
              error: "work item not found",
              message: `Work item "${args.task_id}" not found in Cortex-IA SQLite work database. Note: Cortex MCP observation IDs (e.g. #84) are evidence/memories, not SQLite work task items, and early SDD planning phases (propose, spec, design) have no work tasks created yet.`,
              bridge_authority: bridgeAuthorityView(args.task_id, null, context.sessionID)
            });
          }
          throw error;
        }
      }
    }),

    cortex_ia_work_recover: tool({
      description: "Recover expired work claims and leases. This never restores old authority tokens.",
      args: {},
      async execute() { return cortex(["work", "recover"]); }
    }),

    cortex_ia_work_retry: tool({
      description: "Retry one reconciled blocked work item using revision CAS. Fails closed at the durable attempt limit.",
      args: { task_id: tool.schema.string(), revision: tool.schema.number() },
      async execute(args) { return cortex(["work", "retry", args.task_id, "--revision", String(args.revision)]); }
    }),

    cortex_ia_work_decompose: tool({
      description: "Planner-only: atomically supersede one orchestrator-routed blocked task with a sequential chain of 2-8 smaller tasks, preserving its board, project, upstream dependencies, and downstream DAG.",
      args: {
        task_id: tool.schema.string(),
        revision: tool.schema.number(),
        tasks: tool.schema.array(tool.schema.object({
          task_id: tool.schema.string(),
          title: tool.schema.string(),
          objective: tool.schema.string(),
          acceptance_criteria: tool.schema.string(),
          verification: tool.schema.string(),
          allowed_files: tool.schema.array(tool.schema.string()).optional()
        }))
      },
      async execute(args) {
        return cortexInput(
          ["work", "decompose", args.task_id, "--revision", String(args.revision), "--plan", "@stdin"],
          JSON.stringify({ tasks: args.tasks })
        );
      }
    }),

    cortex_ia_work_claim: tool({
      description: "Claim one ready work item. Optionally reserve initial files atomically. The bridge retains tokens in memory.",
      args: {
        task_id: tool.schema.string(),
        path: tool.schema.string().optional().describe("Optional single workspace-relative file to reserve immediately upon claiming"),
        paths: tool.schema.array(tool.schema.string()).optional().describe("Optional list of workspace-relative files to reserve immediately upon claiming"),
        ttl: tool.schema.string().optional().describe("Duration such as 15m; defaults to Cortex-IA policy")
      },
      async execute(args, context) {
        if (workAuthority.has(args.task_id)) throw new Error(`authority for ${args.task_id} is already held by this controller`);
        if ([...workAuthority.values()].some((authority) => authority.sessionID === context.sessionID)) {
          throw new Error("this implement session already owns one work task; dispatch a separate controller for additional work");
        }
        const command = ["work", "claim", args.task_id, "--owner", controllerIdentity(context.sessionID)];
        if (args.ttl) command.push("--ttl", args.ttl);
        const claim = parseJSON(cortex(command));
        if (!claim?.claim_token) throw new Error("cortex-ia returned no claim token");
        const leases = new Map<string, string>();
        const authority = { claimToken: claim.claim_token, leases, sessionID: context.sessionID };
        workAuthority.set(args.task_id, authority);
        const targetPaths: string[] = [];
        if (args.path) targetPaths.push(args.path);
        if (Array.isArray(args.paths)) {
          for (const p of args.paths) {
            if (p && !targetPaths.includes(p)) targetPaths.push(p);
          }
        }
        const reservedFiles: any[] = [];
        for (const rawPath of targetPaths) {
          const leasePath = workPath(rawPath);
          const reserveCmd = ["work", "reserve", args.task_id, "--claim-token", "@stdin", "--path", leasePath];
          if (args.ttl) reserveCmd.push("--ttl", args.ttl);
          const lease = parseJSON(cortexAuthorized(reserveCmd, authority.claimToken));
          if (lease?.lease_token && lease?.path) {
            leases.set(lease.path, lease.lease_token);
            reservedFiles.push(withoutToken(lease, "lease_token"));
          }
        }
        saveAuthorityState();
        const res = withoutToken(claim, "claim_token");
        if (reservedFiles.length > 0) {
          res.reserved_files = reservedFiles;
        }
        return JSON.stringify(res);
      }
    }),

    cortex_ia_work_renew: tool({
      description: "Renew the live claim retained by this bridge.",
      args: { task_id: tool.schema.string(), ttl: tool.schema.string().optional() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const command = ["work", "renew", args.task_id, "--claim-token", "@stdin"];
        if (args.ttl) command.push("--ttl", args.ttl);
        return JSON.stringify(withoutToken(parseJSON(cortexAuthorized(command, authority.claimToken)), "claim_token"));
      }
    }),

    cortex_ia_file_reserve: tool({
      description: "Reserve one or more workspace-relative files for one claimed task. A live reservation by another task fails closed.",
      args: {
        task_id: tool.schema.string(),
        path: tool.schema.string().optional(),
        paths: tool.schema.array(tool.schema.string()).optional(),
        ttl: tool.schema.string().optional()
      },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID, true);
        const targetPaths: string[] = [];
        if (args.path) targetPaths.push(args.path);
        if (Array.isArray(args.paths)) {
          for (const p of args.paths) {
            if (p && !targetPaths.includes(p)) targetPaths.push(p);
          }
        }
        if (targetPaths.length === 0) {
          throw new Error("either 'path' or 'paths' must be provided to reserve files");
        }
        const results = [];
        for (const rawPath of targetPaths) {
          const leasePath = workPath(rawPath);
          const command = ["work", "reserve", args.task_id, "--claim-token", "@stdin", "--path", leasePath];
          if (args.ttl) command.push("--ttl", args.ttl);
          const lease = parseJSON(cortexAuthorized(command, authority.claimToken));
          if (!lease?.lease_token || !lease?.path) throw new Error("cortex-ia returned no lease authority for path: " + rawPath);
          authority.leases.set(lease.path, lease.lease_token);
          results.push(withoutToken(lease, "lease_token"));
        }
        saveAuthorityState();
        if (results.length === 1 && args.path) {
          return JSON.stringify(results[0]);
        }
        return JSON.stringify({ reserved: results, count: results.length });
      }
    }),

    cortex_ia_work_lease_renew: tool({
      description: "Renew one live file lease retained by this bridge.",
      args: { task_id: tool.schema.string(), path: tool.schema.string(), ttl: tool.schema.string().optional() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const normalizedPath = workPath(args.path);
        const leaseToken = authority.leases.get(normalizedPath);
        if (!leaseToken) throw authorityFailure("BRIDGE_LEASE_MISSING", args.task_id, context.sessionID, normalizedPath);
        const command = ["work", "lease-renew", "--path", normalizedPath, "--lease-token", "@stdin"];
        if (args.ttl) command.push("--ttl", args.ttl);
        const lease = parseJSON(cortexAuthorized(command, leaseToken));
        if (lease?.path && lease?.lease_token) {
          authority!.leases.delete(normalizedPath);
          authority!.leases.set(lease.path, lease.lease_token);
        }
        return JSON.stringify(withoutToken(lease, "lease_token"));
      }
    }),

    cortex_ia_work_release_all: tool({
      description: "Release every file lease retained for one task. Returns partial failures for explicit reconciliation.",
      args: { task_id: tool.schema.string() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const released: string[] = [];
        const failures: Array<{ path: string; error: string }> = [];
        for (const [leasePath, leaseToken] of [...authority.leases.entries()]) {
          try {
            cortexAuthorized(["work", "release", "--path", leasePath, "--lease-token", "@stdin"], leaseToken);
            authority.leases.delete(leasePath);
            released.push(leasePath);
          } catch (error: any) {
            failures.push({ path: leasePath, error: error?.message || "release failed" });
          }
        }
        saveAuthorityState();
        return JSON.stringify({ released, failures });
      }
    }),

    cortex_ia_file_release: tool({
      description: "Release one or more file reservations using the token(s) retained by this bridge.",
      args: {
        task_id: tool.schema.string(),
        path: tool.schema.string().optional().describe("Single workspace-relative file path to release"),
        paths: tool.schema.array(tool.schema.string()).optional().describe("List of workspace-relative file paths to release")
      },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const targetPaths: string[] = [];
        if (args.path) targetPaths.push(args.path);
        if (Array.isArray(args.paths)) {
          for (const p of args.paths) {
            if (p && !targetPaths.includes(p)) targetPaths.push(p);
          }
        }
        if (targetPaths.length === 0) {
          throw new Error("either 'path' or 'paths' must be provided to release files");
        }
        const released: string[] = [];
        const failures: Array<{ path: string; error: string }> = [];
        for (const rawPath of targetPaths) {
          const leasePath = workPath(rawPath);
          const leaseToken = authority.leases.get(leasePath);
          if (!leaseToken) {
            failures.push({ path: leasePath, error: "BRIDGE_LEASE_MISSING" });
            continue;
          }
          try {
            cortexAuthorized(["work", "release", "--path", leasePath, "--lease-token", "@stdin"], leaseToken);
            authority.leases.delete(leasePath);
            released.push(leasePath);
          } catch (error: any) {
            failures.push({ path: leasePath, error: error?.message || "release failed" });
          }
        }
        saveAuthorityState();
        if (targetPaths.length === 1 && args.path && failures.length === 0) {
          return JSON.stringify({ released: released[0], status: "released" });
        }
        return JSON.stringify({ released, failures });
      }
    }),

    cortex_ia_work_transition: tool({
      description: "Transition a claimed task using authority retained by the bridge. Only in_review, in_progress, and blocked are accepted.",
      args: {
        task_id: tool.schema.string(),
        to: tool.schema.enum(["in_review", "in_progress", "blocked"]).optional(),
        status: tool.schema.enum(["in_review", "in_progress", "blocked"]).optional().describe("Alias for 'to'"),
        revision: tool.schema.number().optional()
      },
      async execute(args, context) {
        const targetState = args.to || args.status;
        if (!targetState) throw new Error("'to' or 'status' is required for work transition");
        const authority = authorityForSession(args.task_id, context.sessionID);
        const command = ["work", "transition", args.task_id, "--claim-token", "@stdin", "--to", targetState];
        if (args.revision) command.push("--revision", String(args.revision));
        const result = cortexAuthorized(command, authority.claimToken);
        if (targetState === "in_review" || targetState === "blocked") {
          try {
            cortexAuthorized(["work", "release-all", args.task_id, "--claim-token", "@stdin"], authority.claimToken);
            authority.leases.clear();
          } catch {}
        }
        if (targetState === "blocked") {
          workAuthority.delete(args.task_id);
        }
        saveAuthorityState();
        return result;
      }
    }),

    cortex_ia_work_approve: tool({
      description: "Record an independent work verdict. PASS is the only verdict that can produce done.",
      args: {
        task_id: tool.schema.string(),
        reviewer: tool.schema.string().optional().describe("Legacy display hint; reviewer identity always comes from the host session"),
        verdict: tool.schema.enum(["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE", "pass", "fail", "blocked", "inconclusive"]),
        evidence: tool.schema.string().optional(),
        revision: tool.schema.number().optional()
      },
      async execute(args, context) {
        if (workAuthority.get(args.task_id)?.sessionID === context.sessionID) throw new Error("implementation session cannot approve its own task");
        const rawVerdict = String(args.verdict || "").toUpperCase();
        const verdict = ["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"].includes(rawVerdict) ? rawVerdict : args.verdict;
        const command = ["work", "approve", args.task_id, "--reviewer", controllerIdentity(context.sessionID), "--verdict", verdict];
        if (args.evidence) command.push("--evidence", args.evidence);
        if (args.revision) command.push("--revision", String(args.revision));
        const result = cortex(command);
        workAuthority.delete(args.task_id);
        saveAuthorityState();
        return result;
      }
    }),

    cortex_ia_delegate_start: tool({
      description: "Ask cortex-ia to supervise one external AGY leaf. Implement requires an explicit user-aligned workspace_strategy: current_workspace. The returned execution_mode is authoritative. Call cortex_ia_delegation_wait once, then read the receipt; execute natively only when delegated is false and no external job was accepted.",
      args: {
        role: tool.schema.enum(["implement", "investigate", "reviewer", "planner"]),
        task_id: tool.schema.string().optional(),
        objective: tool.schema.string(),
        workspace_strategy: tool.schema.enum(["current_workspace", "isolated_worktree"]).optional(),
        worktree: tool.schema.string().optional().describe("Legacy parameter; rejected for new work"),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        acceptance_checks: tool.schema.array(tool.schema.string()).optional(),
        context_data: tool.schema.string().optional(),
        model: tool.schema.string().optional().describe("Dynamic model ID to use for delegation (query with cortex_ia_delegation_models)"),
        effort: tool.schema.enum(["low", "medium", "high"]).optional().describe("Reasoning effort level"),
        prefer_native: tool.schema.boolean().optional().describe("If true, explicitly bypass external AGY delegation and execute natively in OpenCode")
      },
      async execute(args, context) {
        if (args.prefer_native) {
          return JSON.stringify({
            delegated: false,
            execution_mode: "native",
            reason: "requested_native",
            action: "USE_NATIVE_SUBAGENT"
          });
        }
        let requestPath = "";
        let acceptedJob: any = null;
        let acceptedTransport: "direct" | "herdr" = "direct";
        let stage: "policy" | "request" | "create" = "policy";
        try {
          const policy = parseJSON(cortex(["delegate", "policy", "--role", args.role]));
          if (policy?.schema_version !== 1 || policy.role !== args.role || typeof policy.external_enabled !== "boolean" ||
              !["external_enabled", "delegation_disabled", "role_native"].includes(policy.reason) ||
              policy.external_enabled !== (policy.reason === "external_enabled")) throw new Error("invalid delegation policy receipt");
          if (!policy.external_enabled) {
            return JSON.stringify({ delegated: false, execution_mode: "native", reason: policy.reason, action: "USE_NATIVE_SUBAGENT" });
          }
          stage = "request";
          if (args.worktree || args.workspace_strategy === "isolated_worktree") {
            return JSON.stringify({
              delegated: false,
              status: "blocked",
              error: { code: "DELEGATION_REQUEST_INVALID", message: "worktree paths and isolated_worktree strategy are retired; use current_workspace without worktree" },
              action: "USE_CURRENT_WORKSPACE"
            });
          }
          if (args.role === "implement") {
            if (!args.task_id) throw new Error("implement delegation requires task_id");
            if (!args.allowed_files?.length) throw new Error("implement delegation requires leased allowed_files");
            if (!args.workspace_strategy) {
              return JSON.stringify({
                delegated: false,
                status: "blocked",
                error: { code: "DELEGATION_WORKSPACE_REQUIRED", message: "workspace strategy is not aligned with the user" },
                action: "ASK_USER_FOR_WORKSPACE_STRATEGY"
              });
            }
            if (args.workspace_strategy !== "current_workspace") {
              return JSON.stringify({
                delegated: false,
                status: "blocked",
                error: { code: "DELEGATION_REQUEST_INVALID", message: "unsupported workspace strategy" },
                action: "USE_CURRENT_WORKSPACE"
              });
            }
          }
          const objective = [
            args.objective,
            args.acceptance_checks?.length ? `Acceptance checks:\n${args.acceptance_checks.map((v) => `- ${v}`).join("\n")}` : "",
            args.context_data ? `Context:\n${args.context_data}` : ""
          ].filter(Boolean).join("\n\n");
          requestPath = transientRequest({
            ...await conversationOwnership(context.sessionID, context.directory),
            project: path.resolve(context.directory),
            role: args.role,
            task_id: args.task_id || "",
            objective,
            workspace: path.resolve(context.directory),
            workspace_strategy: args.workspace_strategy || "",
            worktree: "",
            allowed_files: args.allowed_files || [],
            output_schema: receiptSchema,
            model: args.model || undefined,
            effort: args.effort || undefined
          });

          const config = bridgeConfig();
          let transport: "direct" | "herdr" = "direct";
          let herdr = "";
          try { herdr = firstExecutable("herdr"); } catch {}
          if (config.useHerdr && herdr && isHerdrInUse()) transport = "herdr";

          stage = "create";
          logDelegation(`🚀 [CORTEX-IA] Solicitando delegación para rol '${args.role}' (Transport: ${transport})`);
          let job = parseJSON(cortex(["delegate", "create", "--request-file", requestPath, "--transport", transport]));
          acceptedJob = job;
          acceptedTransport = transport;
          logDelegation(`⚡ [CORTEX-IA] Job de delegación aceptado: ${job.job_id} (Rol: ${args.role}, Transport: ${transport})`);

          if (transport === "herdr") {
            let openedPane = "";
            let openedTab = "";
            try {
              const executionDirectory = context.directory;
              if (config.presentation === "split") {
                const split = execFileSync(herdr, ["pane", "split", "--direction", config.direction, "--cwd", executionDirectory, "--no-focus"], { encoding: "utf-8", windowsHide: true });
                const pane = paneID(split);
                if (!pane) throw new Error("Herdr did not return a pane ID");
                openedPane = pane;
              } else {
                // Background tab: executes agent without splitting the active terminal screen
                const tabLabel = `cortex-${args.role}-${job.job_id.slice(0, 8)}`;
                const tabArgs = ["tab", "create", "--no-focus", "--label", tabLabel];
                if (executionDirectory) tabArgs.push("--cwd", executionDirectory);
                const tabOutput = execFileSync(herdr, tabArgs, { encoding: "utf-8", windowsHide: true });
                const { paneID: pane, tabID: tab } = parseTabCreateResult(tabOutput);
                if (!pane) throw new Error("Herdr did not return a pane ID for background tab");
                openedPane = pane;
                openedTab = tab;
                if (tab) rememberJobTab(job.job_id, tab);
              }
              rememberJobPane(job.job_id, openedPane);
              const worker = ["delegate", "worker", "--job", job.job_id, "--request-file", requestPath];
              execFileSync(herdr, ["pane", "run", openedPane, firstExecutable("cortex-ia"), ...worker], { encoding: "utf-8", windowsHide: true });
              emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: job.status, transport, pane_id: openedPane, tab_id: openedTab || undefined, workspace: path.resolve(context.directory) });
              return JSON.stringify({ delegated: true, execution_mode: executionMode(transport), job_id: job.job_id, status: job.status, transport, pane_id: openedPane, tab_id: openedTab || undefined });
            } catch {
              let cancellationStatus = "cancellation_unknown";
              try {
                const cancelled = parseJSON(cortex(["delegate", "cancel", job.job_id]));
                cancellationStatus = cancelled?.status || "cancelled";
              } catch {}
              if (openedTab || openedPane) {
                closeHerdrJobResources(job.job_id);
              } else {
                cleanupRequest(requestPath);
              }
              emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: cancellationStatus, transport: "herdr" });
              return JSON.stringify({
                delegated: true,
                execution_mode: "herdr_multiplexed",
                job_id: job.job_id,
                status: cancellationStatus,
                transport: "herdr",
                action: "RECONCILE_EXTERNAL_JOB_AND_RETRY_WITH_FRESH_AUTHORITY"
              });
            }
          }

          const worker = ["delegate", "worker", "--job", job.job_id, "--request-file", requestPath];
          const title = `Cortex-IA Delegated ${args.role.toUpperCase()} - Job ${job.job_id.slice(0, 8)}`;
          const child = launchNativeTerminal(firstExecutable("cortex-ia"), worker, context.directory, title);
          try {
            await new Promise<void>((resolve, reject) => {
              child.once("spawn", resolve);
              child.once("error", reject);
            });
          } catch (error) {
            let cancellationStatus = "cancellation_unknown";
            try {
              const cancelled = parseJSON(cortex(["delegate", "cancel", job.job_id]));
              cancellationStatus = cancelled?.status || "cancelled";
            } catch {}
            cleanupRequest(requestPath);
              emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: cancellationStatus, transport, workspace: path.resolve(context.directory) });
            return JSON.stringify({
              delegated: true,
              execution_mode: executionMode(acceptedTransport),
              job_id: job.job_id,
              status: cancellationStatus,
              transport: acceptedTransport,
              reason: (error as any)?.message || "worker launch failed",
              action: "RECONCILE_EXTERNAL_JOB_AND_RETRY_WITH_FRESH_AUTHORITY"
            });
          }
          child.unref();
          logDelegation(`💻 [CORTEX-IA] Worker lanzado para job: ${job.job_id} (${args.role})`);
          emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: job.status, transport, workspace: path.resolve(context.directory) });
          return JSON.stringify({ delegated: true, execution_mode: executionMode(transport), job_id: job.job_id, status: job.status, transport });
        } catch (error: any) {
          logDelegation(`❌ [CORTEX-IA] Fallo en delegación (stage: ${stage}): ${error?.message || error}`);
          if (acceptedJob?.job_id) {
            return JSON.stringify({
              delegated: true,
              execution_mode: executionMode(acceptedTransport),
              job_id: acceptedJob.job_id,
              status: acceptedJob.status || "reconciliation_required",
              transport: acceptedTransport,
              reason: error?.message || "delegation state requires reconciliation",
              action: "RECONCILE_EXTERNAL_JOB"
            });
          }
          if (requestPath) cleanupRequest(requestPath);
          const diagnostic = typeof error?.stderr === "string" ? error.stderr.slice(0, 4096) : String(error?.message ?? "").slice(0, 4096);
          const reason = error?.code === "ENOENT" || /executable not found/i.test(diagnostic) ? "BINARY_UNAVAILABLE" :
            /database|sqlite|busy|locked|permission denied|access is denied|acceso denegado/i.test(diagnostic) ? "STATE_UNAVAILABLE" :
            stage === "policy" ? "POLICY_INVALID_OR_UNAVAILABLE" : stage === "request" ? "REQUEST_INVALID_OR_IDENTITY_UNAVAILABLE" : "JOB_CREATION_REJECTED";
          return JSON.stringify({ delegated: false, status: "blocked", error: {
            code: "DELEGATION_PRE_ACCEPTANCE_ERROR", stage, reason_code: reason,
            ...(Number.isSafeInteger(error?.status) ? { exit_code: error.status } : {}),
            message: "Delegation was not accepted; resolve the reported stage and reason before retrying"
          }, action: "DIAGNOSE_DELEGATION_ERROR" });
        }
      }
    }),

    cortex_ia_delegation_status: tool({
      description: "Read the durable status of a cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) { return cortex(["delegate", "status", args.job_id]); }
    }),

    cortex_ia_delegation_wait: tool({
      description: "Wait for one accepted delegation to reach a terminal durable status without model-side polling.",
      args: {
        job_id: tool.schema.string(),
        timeout_seconds: tool.schema.number().optional().describe("Maximum wait in seconds; 0 or omitted means wait until completion without hard timeout"),
        compact: tool.schema.boolean().optional().describe("If true, return a compact summary receipt preserving full durable storage")
      },
      async execute(args) {
        const timeoutSeconds = args.timeout_seconds !== undefined ? Math.max(0, Math.floor(args.timeout_seconds)) : 0;
        const deadline = timeoutSeconds > 0 ? Date.now() + timeoutSeconds * 1000 : Infinity;
        const isCompact = args.compact === true || args.compact === "true";
        const terminal = new Set(["succeeded", "failed", "cancelled", "timed_out", "lost"]);
        let job: any;
        let consecutiveErrors = 0;
        do {
          try {
            job = parseJSON(cortex(["delegate", "status", args.job_id]));
            consecutiveErrors = 0;
          } catch (err: any) {
            consecutiveErrors++;
            if (consecutiveErrors >= 5) {
              throw err;
            }
            await new Promise((resolve) => setTimeout(resolve, 1500));
            continue;
          }

          if (terminal.has(job?.status)) {
            logDelegation(`🏁 [CORTEX-IA] Job de delegación ${args.job_id} finalizado: ${job.status} (Rol: ${job.role || "unknown"})`);
            emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status, transport: job.transport });
            const config = bridgeConfig();
            if (config.autoClose) {
              closeHerdrJobResources(args.job_id);
            }
            if (isCompact) {
              let res: any = null;
              try {
                res = parseJSON(cortex(["delegate", "result", args.job_id]));
              } catch {}
              const compactResult = extractCompactReceipt(job, res, args.job_id);
              return JSON.stringify({
                ...job,
                compact: true,
                result: compactResult
              });
            }
            if (job.status === "succeeded") {
              try {
                const res = parseJSON(cortex(["delegate", "result", args.job_id]));
                return JSON.stringify({ ...job, result: res });
              } catch {}
            }
            return JSON.stringify(job);
          }
          await new Promise((resolve) => setTimeout(resolve, 1500));
        } while (Date.now() < deadline);
        const timedOutJob = job || { job_id: args.job_id, status: "unknown" };
        if (isCompact) {
          const compactResult = extractCompactReceipt(timedOutJob, null, args.job_id);
          return JSON.stringify({
            ...timedOutJob,
            wait_timed_out: true,
            compact: true,
            result: compactResult
          });
        }
        return JSON.stringify({ ...timedOutJob, wait_timed_out: true });
      }
    }),

    cortex_ia_delegation_result: tool({
      description: "Read the durable structured receipt of a completed cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        let job: any = {};
        try { job = parseJSON(cortex(["delegate", "status", args.job_id])); } catch {}
        if (job?.status && !["succeeded", "failed", "cancelled", "timed_out", "lost"].includes(job.status)) {
          return JSON.stringify({
            job_id: args.job_id,
            status: job.status,
            completed: false,
            message: `Delegation job is still in progress (status: ${job.status}). Use cortex_ia_delegation_wait to await completion.`
          });
        }
        let result = "";
        try {
          result = cortex(["delegate", "result", args.job_id]);
        } catch (err: any) {
          result = JSON.stringify({
            job_id: args.job_id,
            status: job?.status || "unknown",
            error: err?.message || String(err)
          });
        }
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status || "result_read", transport: job.transport });
        closeHerdrJobResources(args.job_id);
        return result;
      }
    }),

    cortex_ia_delegation_cancel: tool({
      description: "Request cancellation of a cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        const result = cortex(["delegate", "cancel", args.job_id]);
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, status: "cancelled" });
        closeHerdrJobResources(args.job_id);
        return result;
      }
    }),

    cortex_ia_delegation_recover: tool({
      description: "Mark delegation workers with expired leases as lost.",
      args: {},
      async execute() { return cortex(["delegate", "recover"]); }
    }),

    cortex_ia_delegation_models: tool({
      description: "Query available AGY models dynamically from the external CLI. The orchestrator and controllers use this to discover supported models and select the optimal model and effort level without hardcoding model names.",
      args: {},
      async execute() {
        try {
          return cortex(["delegate", "models", "--json"]);
        } catch (error: any) {
          return JSON.stringify({ error: error?.message || String(error) });
        }
      }
    }),

    cortex_ia_report_error: tool({
      description: "Emit an operational error and incident report to the central telemetry hub and local audit ledger.",
      args: {
        code: tool.schema.string().describe("Standard error code (ERR_TASK_BLOCKED, ERR_DELEGATION_FAILURE, ERR_VERIFICATION_FAIL, ERR_INVARIANT_VIOLATION)"),
        message: tool.schema.string().describe("Descriptive error message explaining the failure condition"),
        details: tool.schema.string().optional().describe("Extended stack trace, error logs, or failure details"),
        task_id: tool.schema.string().optional().describe("Associated work task ID"),
        job_id: tool.schema.string().optional().describe("Associated delegation job ID")
      },
      async execute(args) {
        const cmd = ["report", "error", "--code", args.code, "--message", args.message];
        if (args.details) cmd.push("--details", args.details);
        if (args.task_id) cmd.push("--task", args.task_id);
        if (args.job_id) cmd.push("--job", args.job_id);
        try {
          return cortex(cmd);
        } catch (error: any) {
          return JSON.stringify({ error: error?.message || String(error) });
        }
      }
    })
  };

  // Gate canonical definitions before exposing aliases, so both names share the
  // same trusted host-role boundary. New tools must be classified explicitly.
  const roles = ["orchestrator", "discovery", "planner", "investigate", "implement", "reviewer"];
  const controllers = ["planner", "investigate", "implement", "reviewer"];
  const readers = new Set(["content_hash", "openspec_validate", "board_list", "board_status", "ledger_status", "work_list", "work_status", "delegation_status", "delegation_wait", "delegation_result", "delegation_models"]);
  const mutations: Record<string, string[]> = {
    openspec_write: ["planner"], change_archive: ["planner"], discovery_write: ["discovery"],
    board_create: ["planner", "orchestrator"], work_create: ["planner", "orchestrator"],
    ledger_fact_add: roles, ledger_progress_record: ["orchestrator"],
    work_recover: ["orchestrator"], work_retry: ["orchestrator"], work_review_refresh: ["orchestrator"], work_decompose: ["planner"],
    work_claim: ["implement"], work_renew: ["implement"],
    file_reserve: ["implement"], work_lease_renew: ["implement"],
    work_release_all: ["implement"], file_release: ["implement"], work_transition: ["implement"],
    work_approve: ["reviewer"], delegate_start: controllers,
    delegation_cancel: [...controllers, "orchestrator"], delegation_recover: ["orchestrator"],
    report_error: roles
  };
  for (const [name, definition] of Object.entries(bridgeTools) as [string, any][]) {
    const capability = name.slice("cortex_ia_".length);
    if (readers.has(capability)) continue;
    const allowed = mutations[capability];
    if (!allowed) throw new Error(`BRIDGE_POLICY_UNCLASSIFIED: ${name}`);
    const execute = definition.execute;
    definition.execute = async (args: any, context: any) => {
      if (!allowed.includes(context?.agent) || typeof context?.sessionID !== "string" ||
          !/^[A-Za-z0-9_-]{1,256}$/.test(context.sessionID)) throw new Error(`BRIDGE_ROLE_DENIED: ${name} requires an authorized host role and session`);
      if (capability === "delegate_start" && args.role !== context.agent) throw new Error("BRIDGE_ROLE_DENIED: delegation role must match the host controller");
      return execute(args, context);
    };
  }

  if (aliases !== "true") return bridgeTools;
  return {
    ...bridgeTools,
      cortex_openspec_validate: bridgeTools.cortex_ia_openspec_validate,
      cortex_openspec_write: bridgeTools.cortex_ia_openspec_write,
      cortex_discovery_write: bridgeTools.cortex_ia_discovery_write,
      cortex_board_create: bridgeTools.cortex_ia_board_create,
      cortex_board_list: bridgeTools.cortex_ia_board_list,
      cortex_board_status: bridgeTools.cortex_ia_board_status,
      cortex_ledger_status: bridgeTools.cortex_ia_ledger_status,
      cortex_ledger_fact_add: bridgeTools.cortex_ia_ledger_fact_add,
      cortex_ledger_progress_record: bridgeTools.cortex_ia_ledger_progress_record,
      cortex_work_create: bridgeTools.cortex_ia_work_create,
      cortex_work_list: bridgeTools.cortex_ia_work_list,
      cortex_work_status: bridgeTools.cortex_ia_work_status,
      cortex_work_recover: bridgeTools.cortex_ia_work_recover,
      cortex_work_retry: bridgeTools.cortex_ia_work_retry,
      cortex_work_decompose: bridgeTools.cortex_ia_work_decompose,
      cortex_work_claim: bridgeTools.cortex_ia_work_claim,
      cortex_work_renew: bridgeTools.cortex_ia_work_renew,
      cortex_work_lease: bridgeTools.cortex_ia_file_reserve,
      cortex_file_reserve: bridgeTools.cortex_ia_file_reserve,
      cortex_work_lease_renew: bridgeTools.cortex_ia_work_lease_renew,
      cortex_work_release: bridgeTools.cortex_ia_file_release,
      cortex_work_release_all: bridgeTools.cortex_ia_work_release_all,
      cortex_file_release: bridgeTools.cortex_ia_file_release,
      cortex_work_transition: bridgeTools.cortex_ia_work_transition,
      cortex_work_approve: bridgeTools.cortex_ia_work_approve,
      cortex_delegate_start: bridgeTools.cortex_ia_delegate_start,
      cortex_delegation_status: bridgeTools.cortex_ia_delegation_status,
      cortex_delegation_wait: bridgeTools.cortex_ia_delegation_wait,
      cortex_delegation_result: bridgeTools.cortex_ia_delegation_result,
      cortex_delegation_cancel: bridgeTools.cortex_ia_delegation_cancel,
      cortex_delegation_recover: bridgeTools.cortex_ia_delegation_recover,
      cortex_delegation_models: bridgeTools.cortex_ia_delegation_models,
      cortex_report_error: bridgeTools.cortex_ia_report_error,
  };
})()

});

};

export default CortexDelegationBridge;
