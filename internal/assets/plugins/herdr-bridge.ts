import { type Plugin, tool } from "@opencode-ai/plugin";
import { execFileSync, spawn } from "node:child_process";
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
const workOwner = `opencode-${process.pid}-${randomUUID()}`;
const logDelegation = logLifecycle;

function configRoot(): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  return path.resolve(home, ".config", "opencode");
}

const activeJobPanes = new Map<string, string>();

function rememberJobPane(jobID: string, paneID: string) {
  activeJobPanes.set(jobID, paneID);
  try {
    cortex(["delegate", "set-pane", jobID, paneID]);
  } catch {}
}

function forgetJobPane(jobID: string) {
  activeJobPanes.delete(jobID);
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
  const durableOwnerMatchesBridge = durableOwner !== "" && durableOwner === workOwner;
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
        execFileSync(candidate, ["version"], { stdio: "ignore", windowsHide: true });
        return candidate;
      } catch {}
    }
  }
  throw new Error(`${name} executable not found`);
}

function cortex(args: string[], cwd?: string): string {
  return execFileSync(firstExecutable("cortex-ia"), args, {
    encoding: "utf-8",
    cwd,
    windowsHide: true,
    stdio: ["ignore", "pipe", "pipe"]
  });
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

function bridgeConfig(): { useHerdr: boolean; direction: "right" | "down" } {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  try {
    const value = JSON.parse(fs.readFileSync(path.join(home, ".config", "opencode", "cortex-delegation.json"), "utf-8"));
    const direction = value?.herdr_settings?.split_direction === "down" ? "down" : "right";
    return { useHerdr: value?.use_herdr === true && value?.herdr_settings?.auto_split === true, direction };
  } catch {
    return { useHerdr: false, direction: "right" };
  }
}

function isHerdrAvailable(herdrPath: string): boolean {
  if (!herdrPath) return false;
  if (process.env.HERDR_ENV === "1" || process.env.HERDR_WORKSPACE_ID) return true;
  try {
    const out = execFileSync(herdrPath, ["pane", "list"], { encoding: "utf-8", windowsHide: true, timeout: 2000 });
    return out.includes("pane_list") || out.includes("panes");
  } catch {
    return false;
  }
}

function paneID(output: string): string {
  const parsed = parseJSON(output);
  return parsed?.result?.pane?.pane_id || parsed?.result?.pane_id || parsed?.pane_id || "";
}

type ExecutionMode = "native" | "direct_cli" | "herdr_multiplexed";

function executionMode(transport: "direct" | "herdr"): ExecutionMode {
  return transport === "herdr" ? "herdr_multiplexed" : "direct_cli";
}

export const CortexDelegationBridge: Plugin = async () => ({
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

  tool: {
    cortex_openspec_validate: tool({
      description: "Validate OpenSpec planning artifacts from the active workspace without exposing a shell.",
      args: { relative_directory: tool.schema.string().optional() },
      async execute(args, context) {
        const command = ["openspec", "validate"];
        if (args.relative_directory) command.push(args.relative_directory);
        return cortex(command, context.directory);
      }
    }),

    cortex_openspec_write: tool({
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

    cortex_discovery_write: tool({
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

    cortex_board_create: tool({
      description: "Create one durable Cortex-IA initiative board.",
      args: { board_id: tool.schema.string(), title: tool.schema.string(), description: tool.schema.string().optional() },
      async execute(args, context) {
        const command = ["board", "create", "--id", args.board_id, "--title", args.title];
        if (args.description) command.push("--description", args.description);
        return cortex(command);
      }
    }),

    cortex_board_list: tool({
      description: "List durable Cortex-IA initiative boards.",
      args: {},
      async execute() { return cortex(["board", "list"]); }
    }),

    cortex_board_status: tool({
      description: "Read one board and its authoritative task snapshot.",
      args: { board_id: tool.schema.string() },
      async execute(args) { return cortex(["board", "status", args.board_id]); }
    }),

    cortex_work_create: tool({
      description: "Create one work item in a durable same-board DAG.",
      args: {
        board_id: tool.schema.string(),
        task_id: tool.schema.string(),
        title: tool.schema.string(),
        objective: tool.schema.string().optional(),
        acceptance_criteria: tool.schema.string().optional(),
        verification: tool.schema.string().optional(),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        dependencies: tool.schema.array(tool.schema.string()).optional()
      },
      async execute(args) {
        const command = ["work", "create", "--board", args.board_id, "--id", args.task_id, "--title", args.title];
        if (args.objective) command.push("--objective", args.objective);
        if (args.acceptance_criteria) command.push("--acceptance", args.acceptance_criteria);
        if (args.verification) command.push("--verify", args.verification);
        for (const file of args.allowed_files || []) command.push("--file", file);
        for (const dependency of args.dependencies || []) command.push("--depends", dependency);
        return cortex(command);
      }
    }),

    cortex_work_list: tool({
      description: "List durable work items, optionally restricted to one board.",
      args: { board_id: tool.schema.string().optional() },
      async execute(args) {
        const command = ["work", "list"];
        if (args.board_id) command.push("--board", args.board_id);
        return cortex(command);
      }
    }),

    cortex_work_status: tool({
      description: "Read one durable Cortex-IA work item plus token-free bridge authority usability for the current session.",
      args: { task_id: tool.schema.string() },
      async execute(args, context) {
        const durable = durableWorkStatus(args.task_id);
        return JSON.stringify({ ...durable, bridge_authority: bridgeAuthorityView(args.task_id, durable, context.sessionID) });
      }
    }),

    cortex_work_recover: tool({
      description: "Recover expired work claims and leases. This never restores old authority tokens.",
      args: {},
      async execute() { return cortex(["work", "recover"]); }
    }),

    cortex_work_retry: tool({
      description: "Retry one reconciled blocked work item using revision CAS. Fails closed at the durable attempt limit.",
      args: { task_id: tool.schema.string(), revision: tool.schema.number() },
      async execute(args) { return cortex(["work", "retry", args.task_id, "--revision", String(args.revision)]); }
    }),

    cortex_work_decompose: tool({
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

    cortex_work_claim: tool({
      description: "Claim one ready work item. The bridge retains the claim token in memory and never returns it to the model.",
      args: {
        task_id: tool.schema.string(),
        ttl: tool.schema.string().optional().describe("Duration such as 15m; defaults to Cortex-IA policy")
      },
      async execute(args, context) {
        if (workAuthority.has(args.task_id)) throw new Error(`authority for ${args.task_id} is already held by this controller`);
        if ([...workAuthority.values()].some((authority) => authority.sessionID === context.sessionID)) {
          throw new Error("this implement session already owns one work task; dispatch a separate controller for additional work");
        }
        const command = ["work", "claim", args.task_id, "--owner", workOwner];
        if (args.ttl) command.push("--ttl", args.ttl);
        const claim = parseJSON(cortex(command));
        if (!claim?.claim_token) throw new Error("cortex-ia returned no claim token");
        workAuthority.set(args.task_id, { claimToken: claim.claim_token, leases: new Map(), sessionID: context.sessionID });
        saveAuthorityState();
        return JSON.stringify(withoutToken(claim, "claim_token"));
      }
    }),

    cortex_work_renew: tool({
      description: "Renew the live claim retained by this bridge.",
      args: { task_id: tool.schema.string(), ttl: tool.schema.string().optional() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const command = ["work", "renew", args.task_id, "--claim-token", "@stdin"];
        if (args.ttl) command.push("--ttl", args.ttl);
        return JSON.stringify(withoutToken(parseJSON(cortexAuthorized(command, authority.claimToken)), "claim_token"));
      }
    }),

    cortex_work_lease: tool({
      description: "Reserve one workspace-relative path using the claim token retained by this bridge.",
      args: { task_id: tool.schema.string(), path: tool.schema.string(), ttl: tool.schema.string().optional() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID, true);
        const command = ["work", "lease", args.task_id, "--claim-token", "@stdin", "--path", args.path];
        if (args.ttl) command.push("--ttl", args.ttl);
        const lease = parseJSON(cortexAuthorized(command, authority.claimToken));
        if (!lease?.lease_token || !lease?.path) throw new Error("cortex-ia returned no lease authority");
        authority.leases.set(lease.path, lease.lease_token);
        saveAuthorityState();
        return JSON.stringify(withoutToken(lease, "lease_token"));
      }
    }),

    cortex_file_reserve: tool({
      description: "Reserve exactly one workspace-relative file for one claimed task. A live reservation by another task fails closed.",
      args: {
        task_id: tool.schema.string(),
        path: tool.schema.string(),
        ttl: tool.schema.string().optional()
      },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID, true);
        const leasePath = workPath(args.path);
        const command = ["work", "reserve", args.task_id, "--claim-token", "@stdin", "--path", leasePath];
        if (args.ttl) command.push("--ttl", args.ttl);
        const lease = parseJSON(cortexAuthorized(command, authority.claimToken));
        if (!lease?.lease_token || !lease?.path) throw new Error("cortex-ia returned no lease authority");
        authority.leases.set(lease.path, lease.lease_token);
        saveAuthorityState();
        return JSON.stringify(withoutToken(lease, "lease_token"));
      }
    }),

    cortex_work_lease_renew: tool({
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

    cortex_work_release: tool({
      description: "Release one file lease retained by this bridge.",
      args: { task_id: tool.schema.string(), path: tool.schema.string() },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const normalizedPath = workPath(args.path);
        const leaseToken = authority.leases.get(normalizedPath);
        if (!leaseToken) throw authorityFailure("BRIDGE_LEASE_MISSING", args.task_id, context.sessionID, normalizedPath);
        const result = cortexAuthorized(["work", "release", "--path", normalizedPath, "--lease-token", "@stdin"], leaseToken);
        authority!.leases.delete(normalizedPath);
        saveAuthorityState();
        return result;
      }
    }),

    cortex_work_release_all: tool({
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

    cortex_file_release: tool({
      description: "Release exactly one file reservation using the token retained by this bridge.",
      args: {
        task_id: tool.schema.string(),
        path: tool.schema.string()
      },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const leasePath = workPath(args.path);
        const leaseToken = authority.leases.get(leasePath);
        if (!leaseToken) throw authorityFailure("BRIDGE_LEASE_MISSING", args.task_id, context.sessionID, leasePath);
        const result = cortexAuthorized(["work", "release", "--path", leasePath, "--lease-token", "@stdin"], leaseToken);
        authority.leases.delete(leasePath);
        saveAuthorityState();
        return result;
      }
    }),

    cortex_work_transition: tool({
      description: "Transition a claimed task using authority retained by the bridge. Only in_review, in_progress, and blocked are accepted.",
      args: {
        task_id: tool.schema.string(),
        to: tool.schema.enum(["in_review", "in_progress", "blocked"]),
        revision: tool.schema.number().optional()
      },
      async execute(args, context) {
        const authority = authorityForSession(args.task_id, context.sessionID);
        const command = ["work", "transition", args.task_id, "--claim-token", "@stdin", "--to", args.to];
        if (args.revision) command.push("--revision", String(args.revision));
        const result = cortexAuthorized(command, authority.claimToken);
        if (args.to === "blocked") {
          workAuthority.delete(args.task_id);
          saveAuthorityState();
        }
        return result;
      }
    }),

    cortex_work_approve: tool({
      description: "Record an independent work verdict. PASS is the only verdict that can produce done.",
      args: {
        task_id: tool.schema.string(),
        reviewer: tool.schema.string(),
        verdict: tool.schema.enum(["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"]),
        evidence: tool.schema.string().optional(),
        revision: tool.schema.number().optional()
      },
      async execute(args) {
        const command = ["work", "approve", args.task_id, "--reviewer", args.reviewer, "--verdict", args.verdict];
        if (args.evidence) command.push("--evidence", args.evidence);
        if (args.revision) command.push("--revision", String(args.revision));
        const result = cortex(command);
        workAuthority.delete(args.task_id);
        saveAuthorityState();
        return result;
      }
    }),

    cortex_delegate_start: tool({
      description: "Ask cortex-ia to supervise one external AGY leaf. Implement requires an explicit user-aligned workspace_strategy: isolated_worktree or current_workspace. The returned execution_mode is authoritative. Call cortex_delegation_wait once, then read the receipt; execute natively only when delegated is false and no external job was accepted.",
      args: {
        role: tool.schema.enum(["implement", "investigate", "reviewer", "planner"]),
        task_id: tool.schema.string().optional(),
        objective: tool.schema.string(),
        workspace_strategy: tool.schema.enum(["isolated_worktree", "current_workspace"]).optional(),
        worktree: tool.schema.string().optional().describe("Absolute isolated worktree path; required only for isolated_worktree"),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        acceptance_checks: tool.schema.array(tool.schema.string()).optional(),
        context_data: tool.schema.string().optional()
      },
      async execute(args, context) {
        let requestPath = "";
        let acceptedJob: any = null;
        let acceptedTransport: "direct" | "herdr" = "direct";
        try {
          if (args.role === "implement") {
            if (!args.task_id) throw new Error("implement delegation requires task_id");
            if (!args.allowed_files?.length) throw new Error("implement delegation requires leased allowed_files");
            if (!args.workspace_strategy) {
              return JSON.stringify({
                delegated: false,
                execution_mode: "native",
                reason: "workspace strategy is not aligned with the user",
                action: "ASK_USER_FOR_WORKSPACE_STRATEGY"
              });
            }
            if (args.workspace_strategy === "isolated_worktree" && !args.worktree) {
              return JSON.stringify({
                delegated: false,
                execution_mode: "native",
                reason: "isolated_worktree strategy requires an existing worktree path",
                action: "REQUEST_OR_CREATE_APPROVED_WORKTREE"
              });
            }
          }
          const objective = [
            args.objective,
            args.acceptance_checks?.length ? `Acceptance checks:\n${args.acceptance_checks.map((v) => `- ${v}`).join("\n")}` : "",
            args.context_data ? `Context:\n${args.context_data}` : ""
          ].filter(Boolean).join("\n\n");
          requestPath = transientRequest({
            role: args.role,
            task_id: args.task_id || "",
            objective,
            workspace: path.resolve(context.directory),
            workspace_strategy: args.workspace_strategy || "",
            worktree: args.workspace_strategy === "isolated_worktree" && args.worktree ? path.resolve(args.worktree) : "",
            allowed_files: args.allowed_files || [],
            output_schema: receiptSchema
          });

          const config = bridgeConfig();
          let transport: "direct" | "herdr" = "direct";
          let herdr = "";
          try { herdr = firstExecutable("herdr"); } catch {}
          if (config.useHerdr && herdr && isHerdrAvailable(herdr)) transport = "herdr";

          let job = parseJSON(cortex(["delegate", "create", "--request-file", requestPath, "--transport", transport]));
          acceptedJob = job;
          acceptedTransport = transport;

          if (transport === "herdr") {
            let openedPane = "";
            try {
              const executionDirectory = args.role === "implement" && args.workspace_strategy === "isolated_worktree" && args.worktree
                ? path.resolve(args.worktree)
                : context.directory;
              const split = execFileSync(herdr, ["pane", "split", "--direction", config.direction, "--cwd", executionDirectory, "--no-focus"], { encoding: "utf-8", windowsHide: true });
              const pane = paneID(split);
              if (!pane) throw new Error("Herdr did not return a pane ID");
              openedPane = pane;
              rememberJobPane(job.job_id, pane);
              const worker = ["delegate", "worker", "--job", job.job_id, "--request-file", requestPath];
              execFileSync(herdr, ["pane", "run", pane, firstExecutable("cortex-ia"), ...worker], { encoding: "utf-8", windowsHide: true });
              emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: job.status, transport, pane_id: pane, workspace: path.resolve(context.directory) });
              return JSON.stringify({ delegated: true, execution_mode: executionMode(transport), job_id: job.job_id, status: job.status, transport, pane_id: pane });
            } catch {
              let cancellationStatus = "cancellation_unknown";
              try {
                const cancelled = parseJSON(cortex(["delegate", "cancel", job.job_id]));
                cancellationStatus = cancelled?.status || "cancelled";
              } catch {}
              if (openedPane) {
                try { execFileSync(herdr, ["pane", "close", openedPane], { encoding: "utf-8", windowsHide: true }); } catch {}
                forgetJobPane(job.job_id);
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
          const child = spawn(firstExecutable("cortex-ia"), worker, {
            cwd: context.directory,
            detached: true,
            shell: false,
            stdio: "ignore",
            windowsHide: true
          });
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
          emitDelegationEvent({ kind: "delegation", job_id: job.job_id, role: args.role, status: job.status, transport, workspace: path.resolve(context.directory) });
          return JSON.stringify({ delegated: true, execution_mode: executionMode(transport), job_id: job.job_id, status: job.status, transport });
        } catch (error: any) {
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
          return JSON.stringify({ delegated: false, execution_mode: "native", reason: error?.message || "delegation failed", action: "USE_NATIVE_SUBAGENT" });
        }
      }
    }),

    cortex_delegation_status: tool({
      description: "Read the durable status of a cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) { return cortex(["delegate", "status", args.job_id]); }
    }),

    cortex_delegation_wait: tool({
      description: "Wait for one accepted delegation to reach a terminal durable status without model-side polling.",
      args: {
        job_id: tool.schema.string(),
        timeout_seconds: tool.schema.number().optional().describe("Maximum wait, default 300 and capped at 600 seconds")
      },
      async execute(args) {
        const timeoutSeconds = Math.max(1, Math.min(600, Math.floor(args.timeout_seconds || 300)));
        const deadline = Date.now() + timeoutSeconds * 1000;
        const terminal = new Set(["succeeded", "failed", "cancelled", "timed_out", "lost"]);
        let job: any;
        do {
          job = parseJSON(cortex(["delegate", "status", args.job_id]));
          if (terminal.has(job?.status)) {
            emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status, transport: job.transport });
            return JSON.stringify(job);
          }
          await new Promise((resolve) => setTimeout(resolve, 750));
        } while (Date.now() < deadline);
        return JSON.stringify({ ...job, wait_timed_out: true });
      }
    }),

    cortex_delegation_result: tool({
      description: "Read the durable structured receipt of a completed cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        const result = cortex(["delegate", "result", args.job_id]);
        let job: any = {};
        try { job = parseJSON(cortex(["delegate", "status", args.job_id])); } catch {}
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, role: job.role, status: job.status || "result_read", transport: job.transport });
        const pane = activeJobPanes.get(args.job_id);
        if (pane) {
          try {
            const herdr = firstExecutable("herdr");
            execFileSync(herdr, ["pane", "close", pane], { encoding: "utf-8", windowsHide: true });
            forgetJobPane(args.job_id);
          } catch {}
        }
        return result;
      }
    }),

    cortex_delegation_cancel: tool({
      description: "Request cancellation of a cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        const result = cortex(["delegate", "cancel", args.job_id]);
        emitDelegationEvent({ kind: "delegation", job_id: args.job_id, status: "cancelled" });
        const pane = activeJobPanes.get(args.job_id);
        if (pane) {
          try {
            const herdr = firstExecutable("herdr");
            execFileSync(herdr, ["pane", "close", pane], { encoding: "utf-8", windowsHide: true });
            forgetJobPane(args.job_id);
          } catch {}
        }
        return result;
      }
    }),

    cortex_delegation_recover: tool({
      description: "Mark delegation workers with expired leases as lost.",
      args: {},
      async execute() { return cortex(["delegate", "recover"]); }
    })
  }
});

export default CortexDelegationBridge;
