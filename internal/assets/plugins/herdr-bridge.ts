import { Plugin } from "@opencode/plugin";
import { Buffer } from "node:buffer";

function makeSchemaNode(type: string, extra: Record<string, any> = {}): any {
  const node: any = { type, ...extra };
  node.describe = (desc: string) => {
    node.description = desc;
    return node;
  };
  node.optional = () => {
    node._optional = true;
    return node;
  };
  return node;
}

function itemsToJSONSchema(item: any): any {
  if (!item) return { type: "string" };
  const { _optional, _rawProps, ...rest } = item;
  if (item._rawProps) {
    return argsToJSONSchema(item._rawProps);
  }
  return rest;
}

function argsToJSONSchema(argsObj: Record<string, any>): any {
  const properties: Record<string, any> = {};
  const required: string[] = [];

  for (const [key, val] of Object.entries(argsObj || {})) {
    if (!val) continue;
    const { _optional, _rawProps, ...rest } = val;
    if (_rawProps) {
      properties[key] = argsToJSONSchema(_rawProps);
    } else {
      properties[key] = rest;
    }
    if (!_optional) {
      required.push(key);
    }
  }

  const schema: any = {
    type: "object",
    properties,
  };
  if (required.length > 0) {
    schema.required = required;
  }
  return schema;
}

function tool(def: { description: string; args?: Record<string, any>; execute: (args: any, context: any) => Promise<any> }) {
  return {
    description: def.description,
    args: def.args || {},
    execute: def.execute,
  };
}

tool.schema = {
  string: () => makeSchemaNode("string"),
  number: () => makeSchemaNode("number"),
  boolean: () => makeSchemaNode("boolean"),
  enum: (values: string[]) => makeSchemaNode("string", { enum: values }),
  array: (items: any) => makeSchemaNode("array", { items: itemsToJSONSchema(items) }),
  object: (props: Record<string, any>) => makeSchemaNode("object", { _rawProps: props }),
};
import { execFileSync, execSync, spawn, type ChildProcess } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

const receiptSchema = {
  type: "object",
  required: ["phase_status", "execution_status", "verification_verdict", "summary"],
  properties: {
    phase_status: { type: "string" },
    execution_status: { type: "string", enum: ["completed", "partial", "failed", "blocked", "unverified"] },
    verification_verdict: { type: "string" },
    summary: { type: "string" },
    changed_files: { type: "array", items: { type: "string" } },
    checks: { type: "array", items: { type: "string" } },
    evidence_refs: { type: "array", items: { type: "string" } },
    task_ids: { type: "array", items: { type: "string" } },
    artifact_refs: { type: "array", items: { type: "string" } },
    risks: { type: "array", items: { type: "string" } },
    next_route: { type: "string" }
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
  maintenance?: { active: boolean; lastProgress: number; timer?: ReturnType<typeof setTimeout>; abort?: AbortController; progress: Map<string, string>; reason?: string };
}

const workAuthority = new Map<string, WorkAuthority>();
const maintenancePolicy = { interval_ms: 30000, status_timeout_ms: 5000, stale_progress_ms: 900000, ttl: "15m" };
function stopMaintenance(authority: WorkAuthority, reason: string) {
  const state = authority.maintenance;
  if (!state) return;
  state.active = false;
  state.reason = reason;
  if (state.timer !== undefined) clearTimeout(state.timer);
  state.abort?.abort();
}
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

function reservationPaths(args: any): string[] {
  if (args.paths !== undefined && !Array.isArray(args.paths)) throw new Error("paths must be an array");
  const paths = [...(args.path === undefined ? [] : [args.path]), ...(args.paths || [])];
  if (paths.length > 128 || paths.some(p => typeof p !== "string" || !p.trim())) throw new Error("reservation requires 1-128 nonempty paths");
  // The authority service owns normalization, validation and duplicate rejection.
  return paths;
}

function reservationReceipt(leases: any, requested: string[]): any[] {
  if (!Array.isArray(leases) || leases.length !== requested.length) throw new Error("incomplete reservation receipt; reconcile durable authority before retry");
  const expected = new Set(requested.map(p => workPath(p.trim())));
  for (const lease of leases) {
    if (typeof lease?.lease_token !== "string" || !lease.lease_token || !expected.delete(lease.path)) throw new Error("invalid reservation receipt; reconcile durable authority before retry");
  }
  return leases;
}

function durableWorkStatus(taskID: string, presentationRole?: string): any {
  const cmd = ["work", "status", taskID];
  if (presentationRole) cmd.push("--role", presentationRole);
  return parseJSON(cortex(cmd));
}

export interface CachedProjectionEntry {
  taskID: string;
  revision: number;
  sessionID: string;
  asOf: string;
  projection: any;
}

const cachedProjections = new Map<string, CachedProjectionEntry>();

function deepFreeze<T>(obj: T): Readonly<T> {
  if (obj === null || typeof obj !== "object") return obj;
  Object.freeze(obj);
  for (const key of Object.keys(obj)) {
    const val = (obj as any)[key];
    if (val !== null && (typeof val === "object" || typeof val === "function") && !Object.isFrozen(val)) {
      deepFreeze(val);
    }
  }
  return obj;
}

function deepCopy<T>(obj: T): T {
  if (obj === null || typeof obj !== "object") return obj;
  return JSON.parse(JSON.stringify(obj));
}

function getUnknownProjection(taskID: string): any {
  return deepFreeze({
    task_id: taskID,
    revision: 0,
    status: "unknown",
    authority_available: false,
    candidate_actions: [],
    blockers: [],
    notes: [],
  });
}

function handleWorkStatusFailure(taskID: string): any {
  const unknownProj = getUnknownProjection(taskID);
  cachedProjections.set(taskID, {
    taskID,
    revision: 0,
    sessionID: "",
    asOf: new Date().toISOString(),
    projection: unknownProj,
  });
  return unknownProj;
}

function processWorkStatusResponse(
  requestedTaskID: string,
  sessionID: string,
  response: any
): { accepted: boolean; reason?: string; projection: any } {
  if (!response || typeof response !== "object") {
    handleWorkStatusFailure(requestedTaskID);
    return { accepted: false, reason: "INVALID_RESPONSE_OBJECT", projection: getUnknownProjection(requestedTaskID) };
  }

  const responseTaskID = response.task_id || response.projection?.task_id;
  if (!responseTaskID || responseTaskID !== requestedTaskID) {
    return { accepted: false, reason: "RESPONSE_TASK_MISMATCH", projection: getUnknownProjection(requestedTaskID) };
  }

  const responseSessionID = response.opencode_session_id || response.session_id;
  const rootSessionID = response.opencode_root_session_id;
  const parentSessionID = response.opencode_parent_session_id;
  const isSessionMatch = !responseSessionID ||
    responseSessionID === sessionID ||
    sessionID === "host-system" ||
    (rootSessionID && rootSessionID === sessionID) ||
    (parentSessionID && parentSessionID === sessionID) ||
    (typeof sessionID === "string" && (sessionID.startsWith("ses_") || sessionID.startsWith("subagent-")));
  if (!isSessionMatch) {
    return { accepted: false, reason: "RESPONSE_SESSION_MISMATCH", projection: getUnknownProjection(requestedTaskID) };
  }

  const rev = Number(response.revision || response.projection?.revision || 0);
  const cached = cachedProjections.get(requestedTaskID);
  if (cached && rev < cached.revision) {
    return { accepted: false, reason: "STALE_REVISION_DISCARDED", projection: cached.projection };
  }

  const rawProj = response.projection || {
    task_id: requestedTaskID,
    revision: rev,
    status: response.status || "unknown",
    authority_available: Boolean(response.found !== false),
  };

  const frozenProj = deepFreeze(deepCopy(rawProj));
  cachedProjections.set(requestedTaskID, {
    taskID: requestedTaskID,
    revision: rev,
    sessionID,
    asOf: response.projection?.as_of || new Date().toISOString(),
    projection: frozenProj,
  });

  return { accepted: true, projection: frozenProj };
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
    maintenance: authority?.maintenance ? { active: authority.maintenance.active, reason: authority.maintenance.reason, ...maintenancePolicy } : undefined,
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

const executableCache = new Map<string, { path: string; until: number; environment: string }>();
function firstExecutable(name: "cortex-ia" | "herdr"): string {
  const environment = [process.env.PATH, process.env.HOME, process.env.USERPROFILE, process.env.LOCALAPPDATA, process.cwd()].join("|");
  const cached = executableCache.get(name);
  if (cached && cached.until > Date.now() && cached.environment === environment &&
      (cached.path === name || fs.existsSync(cached.path))) return cached.path;
  executableCache.delete(name);
  const remember = (value: string) => {
    executableCache.set(name, { path: value, until: Date.now() + 60000, environment });
    return value;
  };
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = name === "herdr"
    ? [
        "herdr",
        path.join(home, ".local", "bin", "herdr"),
        path.join(home, ".cargo", "bin", "herdr"),
        "/opt/homebrew/bin/herdr",
        path.join(home, ".herdr", "packages", "standalone", "releases", "0.9.0-x86_64-pc-windows-msvc", "herdr.exe"),
        path.join(local, "Programs", "Herdr", "bin", "herdr.exe"),
        path.join(home, ".cargo", "bin", "herdr.exe"),
        "/usr/local/bin/herdr",
        "/usr/bin/herdr"
      ]
    : [
        "cortex-ia",
        path.join(home, "go", "bin", "cortex-ia"),
        path.join(home, ".local", "bin", "cortex-ia"),
        "/opt/homebrew/bin/cortex-ia",
        path.join(home, "go", "bin", "cortex-ia.exe"),
        path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
        "/usr/local/bin/cortex-ia",
        "/usr/bin/cortex-ia"
      ];
  for (const candidate of candidates) {
    if (candidate !== name && fs.existsSync(candidate)) return remember(candidate);
    if (candidate === name) {
      try {
        execFileSync(candidate, name === "herdr" ? ["--version"] : ["version"], { stdio: "ignore", windowsHide: true, timeout: 5000 });
        return remember(candidate);
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
      timeout: 120000,
      stdio: ["ignore", "pipe", "pipe"]
    });
  } catch (err: any) {
    executableCache.delete("cortex-ia");
    const msg = String(err?.message || err);
    if (msg.includes("uv_spawn") || msg.includes("EUNKNOWN")) {
      try {
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 200);
        return execFileSync(firstExecutable("cortex-ia"), args, {
          encoding: "utf-8",
          cwd,
          windowsHide: true,
          timeout: 120000,
          stdio: ["ignore", "pipe", "pipe"]
        });
      } catch {}
    }
    throw err;
  }
}

function cortexAuthorized(args: string[], token: string): string {
  return cortexInput(args, token);
}

function cortexInput(args: string[], input: string, timeout = 120000): string {
  try {
  return execFileSync(firstExecutable("cortex-ia"), args, {
    encoding: "utf-8",
    input,
    windowsHide: true,
    timeout,
    stdio: ["pipe", "pipe", "pipe"]
  });
  } catch (error) {
    executableCache.delete("cortex-ia");
    throw error;
  }
}

function artifactCommand(command: string[], outputPath: string, context: any): string {
  if (context?.agent !== "implement" || typeof context.sessionID !== "string" ||
      !/^[A-Za-z0-9_-]{1,256}$/.test(context.sessionID) || typeof context.directory !== "string") {
    throw new Error("BRIDGE_ROLE_DENIED: artifact output requires the host implement controller");
  }
  const root = path.resolve(context.directory);
  const relative = path.relative(root, path.resolve(root, outputPath));
  if (!relative || path.isAbsolute(relative) || relative === ".." || relative.startsWith(`..${path.sep}`) || relative.includes(":") || relative.includes("\0")) {
    throw new Error("LEASE_CHECK_FAILED: artifact output must be inside the workspace");
  }
  const leasePath = workPath(relative);
  const owned = [...workAuthority.entries()].filter(([, authority]) => authority.sessionID === context.sessionID);
  if (owned.length !== 1) throw new Error("LEASE_REQUIRED: artifact output requires one live task claim");
  const [taskID] = owned[0];
  const authority = authorityForSession(taskID, context.sessionID, true);
  const leaseToken = authority.leases.get(leasePath);
  if (!leaseToken) throw new Error("LEASE_REQUIRED: artifact output path is not reserved by this controller");
  // The service checks physical containment and both live tokens at admission
  // and again after conversion, immediately before output or renderer launch.
  return cortexInput([...command, "--artifact-authority=@stdin"], JSON.stringify({
    project: root, session_id: context.sessionID, role: context.agent,
    task_id: taskID, path: leasePath, claim_token: authority.claimToken, lease_token: leaseToken,
  }));
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
  if (Boolean(process.env.HERDR_PANE_ID || process.env.HERDR_WORKSPACE_ID || process.env.HERDR_ENV === "1")) {
    return true;
  }
  try {
    const herdr = firstExecutable("herdr");
    if (!herdr) return false;
    const out = execFileSync(herdr, ["status", "server"], {
      encoding: "utf-8",
      timeout: 1500,
      windowsHide: true,
      stdio: ["pipe", "pipe", "ignore"],
    });
    return out.includes("status: running") || out.includes("endpoint_compatible: yes");
  } catch {
    return false;
  }
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

function sanitizeUnicode(str: string): string {
  if (typeof str !== "string") return "";
  if (typeof (str as any).toWellFormed === "function") {
    return (str as any).toWellFormed();
  }
  return str.replace(/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/g, "\uFFFD");
}

function truncateUTF8Bytes(str: string, maxBytes: number): { text: string; truncated: boolean } {
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

function safeUnicodeSlice(str: string, maxChars: number): string {
  if (!str) return "";
  const clean = sanitizeUnicode(str);
  const chars = Array.from(clean);
  if (chars.length <= maxChars) return clean;
  return chars.slice(0, maxChars).join("") + "...";
}

function extractCompactReceipt(job: any, res: any, jobID: string): any {
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

export const CortexDelegationBridge = Plugin.define({
  id: "cortex-herdr-bridge",
  async setup(ctx) {
    const client = (ctx as any).client || {};
    const hostDirectory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();
  const startMaintenance = (taskID: string, authority: WorkAuthority, directory: string) => {
    const state: NonNullable<WorkAuthority["maintenance"]> = { active: true, lastProgress: Date.now(), progress: new Map() };
    authority.maintenance = state;
    if (typeof client.session?.status !== "function" || !directory) {
      stopMaintenance(authority, "host_status_unavailable_manual_renewal_required");
      return;
    }
    const current = () => state.active && workAuthority.get(taskID) === authority && authority.maintenance === state;
    const tick = async () => {
      if (!current()) return;
      if (Date.now() - state.lastProgress >= maintenancePolicy.stale_progress_ms) {
        stopMaintenance(authority, "stale_progress"); return;
      }
      const abort = new AbortController();
      state.abort = abort;
      let timeout: ReturnType<typeof setTimeout> | undefined;
      try {
        const response: any = await Promise.race([
          client.session.status({ query: { directory }, signal: abort.signal }),
          new Promise((_, reject) => { timeout = setTimeout(() => { abort.abort(); reject(new Error("host_status_timeout")); }, maintenancePolicy.status_timeout_ms); })
        ]);
        if (!current()) return;
        const status = response?.data?.[authority.sessionID]?.type;
        if (response?.error || !["busy", "retry"].includes(status)) {
          stopMaintenance(authority, "host_idle_or_unknown"); return;
        }
        if (Date.now() - state.lastProgress >= maintenancePolicy.stale_progress_ms) {
          stopMaintenance(authority, "stale_progress"); return;
        }
        const receipt = parseJSON(cortexInput(["work", "controller-renew", taskID, "--owner", controllerIdentity(authority.sessionID), "--authority", "@stdin", "--ttl", maintenancePolicy.ttl],
          JSON.stringify({ claim_token: authority.claimToken, leases: Object.fromEntries(authority.leases) }), 10000));
        if (receipt?.task_id !== taskID || receipt?.owner !== controllerIdentity(authority.sessionID) ||
            receipt?.lease_count !== authority.leases.size || !(Date.parse(receipt?.expires_at) > Date.now())) {
          throw new Error("controller_renewal_unconfirmed");
        }
      } catch {
        stopMaintenance(authority, "maintenance_failed_manual_reconciliation_required");
      } finally {
        if (timeout !== undefined) clearTimeout(timeout);
        state.abort = undefined;
        if (current()) schedule();
      }
    };
    const schedule = () => {
      state.timer = setTimeout(tick, maintenancePolicy.interval_ms);
      (state.timer as any)?.unref?.();
    };
    schedule();
  };
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
  const abortController = new AbortController();
  const { signal } = abortController;

  if (ctx.event?.subscribe) {
    ;(async () => {
      try {
        for await (const event of ctx.event.subscribe({ signal })) {
          if (!event) continue;
          const type = event.type || (event as any).event || "";
          const sessionID = event.sessionID || (event as any).sessionId || event.properties?.sessionID || event.properties?.info?.id || "";
          const progress = type === "message.part.updated" ? event.properties?.part : type === "message.updated" ? event.properties?.info : undefined;
          for (const authority of workAuthority.values()) {
            const state = authority.maintenance;
            if (!state?.active) continue;
            if (type === "server.instance.disposed" ||
                (["session.idle", "session.deleted", "session.error"].includes(type) && (sessionID === authority.sessionID || !sessionID)) ||
                (type === "session.status" && sessionID === authority.sessionID && event.properties?.status?.type === "idle")) {
              stopMaintenance(authority, type); continue;
            }
            if (progress?.sessionID === authority.sessionID && typeof progress.id === "string" && progress.id.length <= 256) {
              if (Date.now() - state.lastProgress >= maintenancePolicy.stale_progress_ms) { stopMaintenance(authority, "stale_progress"); continue; }
              // Only a changed host message/part advances progress; repeated busy/retry
              // status and duplicate event delivery cannot maintain an orphan forever.
              const key = `${type}:${progress.id}`;
              const encoded = JSON.stringify(progress);
              if (encoded.length > 1048576) continue;
              const digest = createHash("sha256").update(encoded).digest("hex");
              if (state.progress.get(key) !== digest) {
                state.lastProgress = Date.now();
                state.progress.set(key, digest);
                if (state.progress.size > 128) state.progress.delete(state.progress.keys().next().value!);
              }
            }
          }

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
        }
      } catch (err: any) {
        if (err?.name !== "AbortError" && !signal.aborted) {}
      }
    })();
  }

  if (ctx.session?.hook) {
    ctx.session.hook("compaction", async (event: any) => {
      try {
        const activeState = cortex(["work", "list"], hostDirectory);
        if (activeState) {
          const text = `[CORTEX-IA STATE SNAPSHOT BEFORE COMPACTION]\nActive Work DAG State:\n${activeState}`;
          if (Array.isArray(event.context)) {
            event.context.push(text);
          } else if (Array.isArray(event.messages)) {
            event.messages.push({ role: "system", content: text });
          }
          logDelegation("🧠 [CORTEX-IA] Snapshot de estado DAG inyectado previo a la compactación de sesión.");
        }
      } catch {}
    });
  }

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
        if (context.directory) {
          command.push("--project", path.resolve(context.directory));
        }
        try {
          return cortex(command, context.directory);
        } catch (err: any) {
          const stdout = err?.stdout ? String(err.stdout).trim() : "";
          if (stdout.startsWith("{") && stdout.endsWith("}")) {
            return stdout;
          }
          throw err;
        }
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
        const bytes = Buffer.from(args.content, "utf-8");
        if (bytes.toString("utf-8") !== args.content) throw new Error("OpenSpec write requires well-formed Unicode for exact UTF-8 encoding");
        fs.mkdirSync(path.dirname(target), { recursive: true });
        const temporary = `${target}.${process.pid}.${randomUUID()}.tmp`;
        try {
          fs.writeFileSync(temporary, bytes, { mode: 0o644 });
          fs.renameSync(temporary, target);
        } finally {
          try { if (fs.existsSync(temporary)) fs.unlinkSync(temporary); } catch {}
        }
        const sha256 = createHash("sha256").update(bytes).digest("hex");
        return JSON.stringify({ written: clean, bytes: bytes.length, sha256 });
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

    cortex_ia_doc_convert: tool({
      description: "Convert an office document into Markdown text and metadata. Inline output is read-only; output_path requires the host implement controller's live task and file lease.",
      args: {
        file_path: tool.schema.string(),
        output_path: tool.schema.string().optional(),
        format: tool.schema.string().optional(),
        max_lines: tool.schema.number().optional(),
        ocr: tool.schema.enum(["reject", "hosted"]).optional()
      },
      async execute(args, context) {
        const cmd = ["doc", "convert", path.resolve(context.directory, args.file_path), "--json"];
        if (args.output_path) cmd.push("-o", path.resolve(context.directory, args.output_path));
        if (args.format) cmd.push("--format", args.format);
        if (args.max_lines !== undefined) cmd.push("--max-lines", String(args.max_lines));
        if (args.ocr) cmd.push("--ocr", args.ocr);
        if (args.output_path) return artifactCommand(cmd, args.output_path, context);
        return cortex(cmd, context.directory);
      }
    }),

    cortex_ia_diagram_validate: tool({
      description: "Validate a system diagram JSON specification (architecture, workflow, sequence, dataflow, lifecycle) against topological and structural rules (inspired by archify).",
      args: {
        spec_path: tool.schema.string(),
        diagram_type: tool.schema.enum(["architecture", "workflow", "sequence", "dataflow", "lifecycle"]).optional(),
        quality: tool.schema.enum(["standard", "showcase"]).optional()
      },
      async execute(args, context) {
        const cmd = ["diagram", "validate"];
        if (args.diagram_type) cmd.push(args.diagram_type);
        cmd.push(path.resolve(context.directory, args.spec_path), "--json");
        if (args.quality) cmd.push(`--quality=${args.quality}`);
        return cortex(cmd, context.directory);
      }
    }),

    cortex_ia_diagram_render: tool({
      description: "Render a diagram specification to interactive HTML. Requires the host implement controller's live task and output file lease.",
      args: {
        spec_path: tool.schema.string(),
        output_path: tool.schema.string(),
        diagram_type: tool.schema.enum(["architecture", "workflow", "sequence", "dataflow", "lifecycle"]).optional(),
        quality: tool.schema.enum(["standard", "showcase"]).optional()
      },
      async execute(args, context) {
        const diagramType = args.diagram_type || "architecture";
        const cmd = ["diagram", "render", diagramType, path.resolve(context.directory, args.spec_path), path.resolve(context.directory, args.output_path), "--json"];
        if (args.quality) cmd.push(`--quality=${args.quality}`);
        return artifactCommand(cmd, args.output_path, context);
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
          change_id: tool.schema.string(), spec_plane: tool.schema.enum(["cortex", "openspec", "hybrid", "speckit"]),
          pins: tool.schema.array(tool.schema.object({
            transport: tool.schema.string().describe("Pin transport: 'workspace_file' for workspace files (OpenSpec/SpecKit/Hybrid), 'local_cortex_cli' or 'cortex_mcp' for Cortex observations"),
            project: tool.schema.string().describe("Workspace root directory or project path"),
            locator: tool.schema.string().describe("Relative path to specification file (e.g. openspec/changes/<name>/plan.md or specs/...)"),
            sha256: tool.schema.string().describe("64-character lowercase SHA-256 digest")
          })),
          requirement_ids: tool.schema.array(tool.schema.string())
        }).optional().describe("Required for SDD tasks: exact contract pins and requirement IDs; omitted only for direct/legacy work")
      },
      async execute(args, context) {
        const sdd = args.workflow === "sdd-lite" || args.workflow === "sdd-full";
        if (sdd ? args.sdd_contract?.workflow !== args.workflow : args.sdd_contract !== undefined) throw new Error("SDD_CONTRACT_REQUIRED: workflow and typed contract must agree; direct workflows cannot carry SDD bindings");
        if (args.sdd_contract?.pins) {
          for (const pin of args.sdd_contract.pins) {
            if (pin.transport) {
              const t = String(pin.transport).toLowerCase().trim();
              if (["workspace_file", "workspace-file", "workspace", "file", "local_file", "local-file", "openspec", "speckit", "fs", "filesystem", "workspace_path", "path"].includes(t)) {
                pin.transport = "workspace_file";
              } else if (["local_cortex_cli", "local-cortex-cli", "cortex_cli", "cortex-cli", "cortex_local", "cli"].includes(t)) {
                pin.transport = "local_cortex_cli";
              } else if (["cortex_mcp", "cortex-mcp", "cortex", "mcp"].includes(t)) {
                pin.transport = "cortex_mcp";
              }
            }
            if (pin.sha256) {
              pin.sha256 = String(pin.sha256).toLowerCase().trim().replace(/^sha256:/, "");
            }
            if (pin.locator) {
              pin.locator = String(pin.locator).trim().replace(/\\/g, "/");
            }
            if (context.directory && pin.project) {
              const p = String(pin.project).trim();
              if (p === "." || !path.isAbsolute(p) || p.toLowerCase() === path.basename(context.directory).toLowerCase()) {
                pin.project = path.resolve(context.directory);
              }
            }
          }
        }
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
      args: { task_id: tool.schema.string(), role: tool.schema.string().optional() },
      async execute(args, context) {
        const hostRole = activeSubagents.get(context.sessionID)?.role || "unknown";
        try {
          const durable = durableWorkStatus(args.task_id, hostRole);
          const processed = processWorkStatusResponse(args.task_id, context.sessionID, durable);
          const bridgeAuth = bridgeAuthorityView(args.task_id, durable, context.sessionID);
          return JSON.stringify({
            ...durable,
            found: true,
            projection: processed.projection,
            bridge_authority: bridgeAuth,
          });
        } catch (error: any) {
          handleWorkStatusFailure(args.task_id);
          const stderr = (error?.stderr?.toString?.() || error?.message || "");
          if (stderr.includes("work item not found")) {
            return JSON.stringify({
              task_id: args.task_id,
              found: false,
              status: "not_found",
              error: "work item not found",
              message: `Work item "${args.task_id}" not found in Cortex-IA SQLite work database. Note: Cortex MCP observation IDs (e.g. #84) are evidence/memories, not SQLite work task items, and early SDD planning phases (propose, spec, design) have no work tasks created yet.`,
              projection: getUnknownProjection(args.task_id),
              bridge_authority: bridgeAuthorityView(args.task_id, null, context.sessionID)
            });
          }
          throw error;
        }
      }
    }),

    cortex_ia_work_approvals: tool({
      description: "List historical approval records and review bindings for a durable task.",
      args: { task_id: tool.schema.string() },
      async execute(args) {
        return cortex(["work", "approvals", args.task_id]);
      }
    }),

    cortex_ia_work_fingerprint: tool({
      description: "Calculate authoritative file and definition SHA-256 fingerprints for a task and verify whether they match the approved binding.",
      args: { task_id: tool.schema.string() },
      async execute(args) {
        return cortex(["work", "fingerprint", args.task_id]);
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
        contract: tool.schema.object({
          version: tool.schema.number(), workflow: tool.schema.enum(["sdd-lite", "sdd-full"]),
          change_id: tool.schema.string(), spec_plane: tool.schema.enum(["cortex", "openspec", "hybrid"]),
          pins: tool.schema.array(tool.schema.object({ transport: tool.schema.string(), project: tool.schema.string(), locator: tool.schema.string(), sha256: tool.schema.string() })),
          requirement_ids: tool.schema.array(tool.schema.string())
        }).optional().describe("Optional upgraded SDD contract when decomposing a direct-change task to SDD-lite or updating the change contract"),
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
        const plan: Record<string, unknown> = { tasks: args.tasks };
        if (args.contract) plan.contract = args.contract;
        return cortexInput(
          ["work", "decompose", args.task_id, "--revision", String(args.revision), "--plan", "@stdin"],
          JSON.stringify(plan)
        );
      }
    }),

    cortex_ia_work_claim: tool({
      description: "Claim one ready work item and optionally reserve initial files atomically. Tokens remain in memory. Auto-maintenance checks host busy/retry every 30s, renews to 15m, and stops after 15m without changed host message/part progress or on idle/error/dispose/delivery. Unavailable host status requires explicit manual renewal; expired authority cannot be revived.",
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
        const targetPaths = reservationPaths(args);
        const command = ["work", "claim", args.task_id, "--owner", controllerIdentity(context.sessionID), ...targetPaths.flatMap(p => ["--path", p])];
        if (args.ttl) command.push("--ttl", args.ttl);
        const claim = parseJSON(cortex(command));
        if (!claim?.claim_token) throw new Error("cortex-ia returned no claim token");
        const reserved = reservationReceipt(claim.reserved_files || [], targetPaths);
        const leases = new Map<string, string>(reserved.map(lease => [lease.path, lease.lease_token]));
        const authority: WorkAuthority = { claimToken: claim.claim_token, leases, sessionID: context.sessionID };
        workAuthority.set(args.task_id, authority);
        startMaintenance(args.task_id, authority, context.directory);
        saveAuthorityState();
        return JSON.stringify({ ...withoutToken(claim, "claim_token"), reserved_files: reserved.map(lease => withoutToken(lease, "lease_token")), maintenance: { ...maintenancePolicy, active: authority.maintenance?.active, reason: authority.maintenance?.reason } });
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
        const targetPaths = reservationPaths(args);
        if (!targetPaths.length) throw new Error("either 'path' or 'paths' must be provided to reserve files");
        const command = ["work", "reserve", args.task_id, "--claim-token", "@stdin", ...targetPaths.flatMap(p => ["--path", p])];
        if (args.ttl) command.push("--ttl", args.ttl);
        const response = parseJSON(cortexAuthorized(command, authority.claimToken));
        const reserved = reservationReceipt(response?.reserved || [response], targetPaths);
        for (const lease of reserved) authority.leases.set(lease.path, lease.lease_token);
        const results = reserved.map(lease => withoutToken(lease, "lease_token"));
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
      description: "Release all task file leases in one transaction. Only a matching durable acknowledgement clears local reservations; failure requires explicit reconciliation.",
      args: { task_id: tool.schema.string() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const released = [...authority.leases.keys()];
        stopMaintenance(authority, "release_all");
        const receipt = parseJSON(cortexAuthorized(["work", "release-all", args.task_id, "--claim-token", "@stdin"], authority.claimToken));
        if (receipt?.task_id !== args.task_id || receipt?.released_all !== true) {
          throw new Error("WORK_RELEASE_UNCONFIRMED: reservations retained locally; query task status before retrying");
        }
        authority.leases.clear();
        saveAuthorityState();
        return JSON.stringify({ released, failures: [], ...receipt });
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
      description: "Transition a claimed task using authority retained by the bridge. Emits authoritative completion receipt without requiring raw JSON text in chat.",
      args: {
        task_id: tool.schema.string(),
        to: tool.schema.enum(["in_review", "in_progress", "blocked"]).optional(),
        status: tool.schema.enum(["in_review", "in_progress", "blocked"]).optional().describe("Alias for 'to'"),
        revision: tool.schema.number().optional(),
        summary: tool.schema.string().optional().describe("Human-readable execution summary"),
        verdict: tool.schema.enum(["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE", "pass", "fail", "blocked", "inconclusive"]).optional().describe("Verification verdict"),
        evidence_refs: tool.schema.array(tool.schema.string()).optional().describe("Pointers to verified test files, commands or observations"),
        changed_files: tool.schema.array(tool.schema.string()).optional().describe("List of modified workspace files")
      },
      async execute(args, context) {
        const targetState = args.to || args.status;
        if (!targetState) throw new Error("'to' or 'status' is required for work transition");
        const authority = authorityForSession(args.task_id, context.sessionID);
        const command = ["work", "transition", args.task_id, "--claim-token", "@stdin", "--to", targetState];
        if (targetState !== "in_progress") stopMaintenance(authority, "delivery");
        if (args.revision) command.push("--revision", String(args.revision));
        if (args.summary !== undefined) command.push("--summary", args.summary);
        if (args.verdict !== undefined) command.push("--verdict", args.verdict);
        for (const ref of args.evidence_refs || []) command.push("--evidence-ref", ref);
        for (const file of args.changed_files || []) command.push("--changed-file", file);
        const result = cortexAuthorized(command, authority.claimToken);
        const delivered = parseJSON(result);
        if (delivered?.task_id !== args.task_id || delivered?.status !== targetState ||
            !Array.isArray(delivered?.leases) && delivered?.leases !== undefined ||
            (targetState !== "in_progress" && delivered?.leases?.length)) {
          throw new Error("WORK_TRANSITION_UNCONFIRMED: durable transition acknowledgement is missing or inconsistent; query task status before retrying");
        }
        if (targetState === "in_review" || targetState === "blocked") {
          authority.leases.clear();
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
        revision: tool.schema.number().optional(),
        summary: tool.schema.string().optional().describe("Independent review summary and lens verdict"),
        findings: tool.schema.array(tool.schema.string()).optional().describe("List of review findings or blockers")
      },
      async execute(args, context) {
        if (workAuthority.get(args.task_id)?.sessionID === context.sessionID) throw new Error("implementation session cannot approve its own task");
        const rawVerdict = String(args.verdict || "").toUpperCase();
        const verdict = ["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"].includes(rawVerdict) ? rawVerdict : args.verdict;
        const command = ["work", "approve", args.task_id, "--reviewer", controllerIdentity(context.sessionID), "--verdict", verdict];
        const evidenceStr = args.evidence || (args.summary ? (args.summary + (args.findings && args.findings.length ? ` (Findings: ${args.findings.join("; ")})` : "")) : "");
        if (evidenceStr) command.push("--evidence", evidenceStr);
        if (args.revision) command.push("--revision", String(args.revision));
        const result = cortex(command);
        workAuthority.delete(args.task_id);
        saveAuthorityState();
        return result;
      }
    }),

    cortex_ia_delegate_start: tool({
      description: "Ask cortex-ia to supervise one external AGY leaf. Implement requires user-aligned current_workspace. The returned execution_mode is authoritative; native execution requires mode native with no error. Model and effort remain user-configured, never overridden. Authentication uses the existing AGY account/keyring by default. Optional Gemini API authentication uses CORTEX_IA_AGY_AUTH=gemini and GEMINI_API_KEY. See cortex-work-protocol.md section 5 for authentication and temporary-home limits. Call cortex_ia_delegation_wait, then read the receipt; acceptance is not verified completion.",
      args: {
        role: tool.schema.enum(["implement", "investigate", "reviewer", "planner"]),
        task_id: tool.schema.string().optional(),
        objective: tool.schema.string(),
        workspace_strategy: tool.schema.enum(["current_workspace", "isolated_worktree"]).optional(),
        worktree: tool.schema.string().optional().describe("Legacy parameter; rejected for new work"),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        acceptance_checks: tool.schema.array(tool.schema.string()).optional(),
        context_data: tool.schema.string().optional(),
        workload_policy: tool.schema.enum(["strict", "flexible", "unbounded"]).optional().describe("Workload guidance forwarded to the leaf; omitted defaults to flexible"),
        prefer_native: tool.schema.boolean().optional().describe("Legacy preference; cannot override configured delegation policy")
      },
      async execute(args, context) {
        const workloadPolicy = args.workload_policy === undefined ? "flexible" : args.workload_policy;
        if (!["strict", "flexible", "unbounded"].includes(workloadPolicy)) {
          return JSON.stringify({ delegated: false, status: "blocked", error: {
            code: "DELEGATION_REQUEST_INVALID", message: "workload_policy must be strict, flexible or unbounded"
          }, action: "CORRECT_REQUEST_BEFORE_ACCEPTANCE" });
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
            logDelegation(`ℹ️ [CORTEX-IA] Rol '${args.role}' configurado nativo en cortex-delegation.json (reason: ${policy.reason}).`);
            return JSON.stringify({ delegated: false, execution_mode: "native", reason: policy.reason, action: "USE_NATIVE_SUBAGENT" });
          }
          if (args.prefer_native) {
            return JSON.stringify({ delegated: false, status: "blocked", error: { code: "DELEGATION_POLICY_CONFLICT", message: "prefer_native cannot override configured external delegation" }, action: "FOLLOW_CONFIGURED_POLICY" });
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
          let taskContext = "";
          if (args.task_id) {
            try {
              const statusRaw = cortex(["work", "status", args.task_id]);
              if (statusRaw) {
                taskContext = `Authoritative Task State (from Cortex-IA Work Authority):\n${statusRaw}`;
              }
            } catch {}
          }
          const objective = [
            args.objective,
            args.acceptance_checks?.length ? `Acceptance checks:\n${args.acceptance_checks.map((v) => `- ${v}`).join("\n")}` : "",
            taskContext,
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
            workload_policy: workloadPolicy,
            output_schema: receiptSchema
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
            } catch (err: any) {
              logDelegation(`❌ [CORTEX-IA] Error al crear pestaña/panel en Herdr: ${err?.message || String(err)}`);
              let cancellationStatus = "cancellation_unknown";
              try {
                const cancelled = parseJSON(cortex(["delegate", "cancel", job.job_id]));
                cancellationStatus = cancelled?.cancellation_requested ? "cancellation_requested" : cancelled?.status || "cancellation_unknown";
              } catch {}
              if (cancellationStatus === "cancelled") {
                if (openedTab || openedPane) closeHerdrJobResources(job.job_id);
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
              cancellationStatus = cancelled?.cancellation_requested ? "cancellation_requested" : cancelled?.status || "cancellation_unknown";
            } catch {}
            if (cancellationStatus === "cancelled") cleanupRequest(requestPath);
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
          const isWorkspaceBlocked = /workspace blocked by external job/i.test(diagnostic);
          const reason = error?.code === "ENOENT" || /executable not found/i.test(diagnostic) ? "BINARY_UNAVAILABLE" :
            isWorkspaceBlocked ? "WORKSPACE_BLOCKED" :
            /database|sqlite|\bbusy\b|\blocked\b|permission denied|access is denied|acceso denegado/i.test(diagnostic) ? "STATE_UNAVAILABLE" :
            stage === "policy" ? "POLICY_INVALID_OR_UNAVAILABLE" : stage === "request" ? "REQUEST_INVALID_OR_IDENTITY_UNAVAILABLE" : "JOB_CREATION_REJECTED";
          const rawMessage = diagnostic.trim().replace(/^Command failed:[^\n]*\n?/i, "").trim();
          const message = isWorkspaceBlocked
            ? (rawMessage || "Workspace is currently exclusive to another external delegation job; wait for it or execute natively")
            : (rawMessage || "Delegation was not accepted; resolve the reported stage and reason before retrying");
          const action = isWorkspaceBlocked
            ? "WAIT_FOR_ACTIVE_JOB_OR_USE_NATIVE_SUBAGENT"
            : "DIAGNOSE_DELEGATION_ERROR";
          return JSON.stringify({ delegated: false, status: "blocked", error: {
            code: "DELEGATION_PRE_ACCEPTANCE_ERROR", stage, reason_code: reason,
            ...(Number.isSafeInteger(error?.status) ? { exit_code: error.status } : {}),
            message,
            diagnostic: diagnostic.trim()
          }, action });
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
        const timeoutSeconds = args.timeout_seconds !== undefined && args.timeout_seconds > 0
          ? Math.floor(args.timeout_seconds)
          : 1800; // default 30-minute upper bound to prevent infinite polling deadlock
        const deadline = Date.now() + timeoutSeconds * 1000;
        const isCompact = args.compact === true || args.compact === "true";
        const terminal = new Set(["succeeded", "failed", "cancelled", "timed_out", "lost"]);
        let job: any;
        let consecutiveErrors = 0;
        do {
          try {
            job = parseJSON(cortex(["delegate", "query", args.job_id]));
            consecutiveErrors = 0;
          } catch (err: any) {
            try {
              job = parseJSON(cortex(["delegate", "status", args.job_id]));
              consecutiveErrors = 0;
            } catch (err2: any) {
              consecutiveErrors++;
              if (consecutiveErrors >= 5) {
                throw err;
              }
              await new Promise((resolve) => setTimeout(resolve, 1500));
              continue;
            }
          }

          if (job?.status === "lost" && job?.termination_reconciled === true && !job?.reconciliation_required) {
            return JSON.stringify({ ...job, completed: false, action: "EXPLICIT_RETRY", message: "Prior execution was lost; termination is reconciled. Any retry requires fresh authority." });
          }
          if (job?.reconciliation_required || job?.status === "lost") {
            return JSON.stringify({ ...job, completed: false, action: "RECONCILE_TERMINATION", message: "Process termination is unconfirmed; workspace remains fenced." });
          }
          if (terminal.has(job?.status)) {
            logDelegation(`🏁 [CORTEX-IA] Job de delegación ${args.job_id} finalizado: ${job.status} (Rol: ${job.role || "unknown"})`);
            emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status, transport: job.transport });
            const config = bridgeConfig();
            if (config.autoClose) {
              closeHerdrJobResources(args.job_id);
            }
            if (isCompact) {
              let res: any = job?.receipt || null;
              if (!res && job?.status === "succeeded") {
                try {
                  res = parseJSON(cortex(["delegate", "result", args.job_id]));
                } catch {}
              }
              const compactResult = extractCompactReceipt(job, res, args.job_id);
              return JSON.stringify({
                ...job,
                compact: true,
                result: compactResult
              });
            }
            if (job.status === "succeeded") {
              if (job?.receipt) {
                return JSON.stringify({ ...job, result: job.receipt });
              }
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
        try {
          job = parseJSON(cortex(["delegate", "query", args.job_id]));
        } catch {
          try { job = parseJSON(cortex(["delegate", "status", args.job_id])); } catch {}
        }
        if (job?.status === "lost" && job?.termination_reconciled === true && !job?.reconciliation_required) {
          return JSON.stringify({ ...job, completed: false, action: "EXPLICIT_RETRY", message: "Prior execution was lost; termination is reconciled. Any retry requires fresh authority." });
        }
        if (job?.reconciliation_required || job?.status === "lost") {
          return JSON.stringify({ ...job, completed: false, action: "RECONCILE_TERMINATION" });
        }
        if (job?.status && !["succeeded", "failed", "cancelled", "timed_out"].includes(job.status)) {
          return JSON.stringify({
            job_id: args.job_id,
            status: job.status,
            cancellation_requested: job.cancellation_requested === true,
            completed: false,
            message: `Delegation job is still in progress (status: ${job.status}). Use cortex_ia_delegation_wait to await completion.`
          });
        }
        let result = "";
        if (job?.receipt_available && job?.receipt) {
          result = JSON.stringify(job.receipt);
        } else {
          try {
            result = cortex(["delegate", "result", args.job_id]);
          } catch (err: any) {
            result = JSON.stringify({
              job_id: args.job_id,
              status: job?.status || "unknown",
              error: job?.receipt_missing ? "receipt_missing: delegation job completed without recording a receipt" : (err?.message || String(err))
            });
          }
        }
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status || "result_read", transport: job.transport });
        if (["succeeded", "failed", "cancelled", "timed_out"].includes(job?.status)) closeHerdrJobResources(args.job_id);
        return result;
      }
    }),

    cortex_ia_delegation_cancel: tool({
      description: "Request cancellation of a cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        const result = cortex(["delegate", "cancel", args.job_id]);
        const job = parseJSON(result);
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, status: job?.cancellation_requested ? "cancellation_requested" : job?.status || "unknown" });
        if (job?.status === "cancelled" && !job?.reconciliation_required) closeHerdrJobResources(args.job_id);
        return result;
      }
    }),

    cortex_ia_delegation_recover: tool({
      description: "Mark delegation workers with expired leases as lost.",
      args: {},
      async execute() { return cortex(["delegate", "recover"]); }
    }),

    cortex_ia_delegation_reconcile: tool({
      description: "Audit prior-boot termination of a lost job using local OS evidence. Preserves failure history and requires fresh authority for any retry.",
      args: { job_id: tool.schema.string(), reason: tool.schema.string() },
      async execute(args, context) {
        if (typeof args.reason !== "string" || !args.reason.trim() || Buffer.byteLength(args.reason.trim(), "utf8") > 1024 || args.reason.includes("\0")) {
          throw new Error("A reconciliation reason of 1..1024 UTF-8 bytes is required");
        }
        return cortex(["delegate", "reconcile", args.job_id, "--reason", args.reason.trim(), "--session-id", context.sessionID]);
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
      async execute(args, context) {
        const cmd = ["report", "error", "--code", args.code, "--message", args.message, "--session-id", context.sessionID, "--role", context.agent, "--source", context.agent, "--workspace", context.directory];
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

  // Gate canonical definitions before exposing tools. New tools must be classified explicitly.
  const roles = ["orchestrator", "discovery", "planner", "investigate", "implement", "reviewer"];
  const controllers = ["planner", "investigate", "implement", "reviewer"];
  const readers = new Set([
    "content_hash", "openspec_validate", "board_list", "board_status", "work_list", "work_status", "delegation_status", "delegation_wait", "delegation_result", "delegation_models",
    "diagram_validate", "work_approvals", "work_fingerprint"
  ]);
  const mutations: Record<string, string[]> = {
    openspec_write: ["planner"], change_archive: ["planner"], discovery_write: ["discovery"],
    board_create: ["planner", "orchestrator"], work_create: ["planner", "orchestrator"],
    work_recover: ["orchestrator"], work_retry: ["orchestrator"], work_review_refresh: ["orchestrator"], work_decompose: ["planner"],
    work_claim: ["implement"], work_renew: ["implement"],
    file_reserve: ["implement"], work_lease_renew: ["implement"],
    work_release_all: ["implement"], file_release: ["implement"], work_transition: ["implement"],
    work_approve: ["reviewer", "orchestrator"], delegate_start: controllers,
    delegation_cancel: [...controllers, "orchestrator"], delegation_recover: ["orchestrator"], delegation_reconcile: ["orchestrator"],
    doc_convert: roles,
    diagram_render: ["implement"],
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

    if (ctx.tool?.transform) {
      await ctx.tool.transform((editor: any) => {
        for (const [name, def] of Object.entries(bridgeTools) as [string, any][]) {
          const inputSchema = argsToJSONSchema(def.args || {});
          editor.add({
            name,
            description: def.description,
            input: inputSchema,
            execute: async (input: any, toolCtx: any) => {
              const context = {
                sessionID: toolCtx?.sessionID || toolCtx?.sessionId || toolCtx?.session?.id || (ctx as any).sessionId || "",
                agent: toolCtx?.agent || toolCtx?.role || toolCtx?.session?.agent || "",
                directory: toolCtx?.directory || hostDirectory,
              };
              const result = await def.execute(input, context);
              return typeof result === "string" ? { content: result } : result;
            },
          });
        }
      });
    }

    const cleanup = async () => {
      abortController.abort();
      for (const authority of workAuthority.values()) stopMaintenance(authority, "disposed");
      workAuthority.clear();
      saveAuthorityState();
    };
    (cleanup as any).dispose = cleanup;
    return cleanup;
  },
});

Object.assign(CortexDelegationBridge, {
  cachedProjections,
  deepFreeze,
  deepCopy,
  getUnknownProjection,
  handleWorkStatusFailure,
  processWorkStatusResponse,
  sanitizeUnicode,
  truncateUTF8Bytes,
  safeUnicodeSlice,
  extractCompactReceipt,
});

export default CortexDelegationBridge;
