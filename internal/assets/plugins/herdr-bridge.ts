import { type Plugin, tool } from "@opencode-ai/plugin";
import { execFileSync, spawn } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

const receiptSchema = {
  type: "object",
  additionalProperties: false,
  required: ["phase_status", "task_status", "verification_verdict", "summary"],
  properties: {
    phase_status: { type: "string" },
    task_status: { type: "string" },
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
const activeJobPanes = new Map<string, string>();
const logDelegation = logLifecycle;

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

function cortex(args: string[]): string {
  return execFileSync(firstExecutable("cortex-ia"), args, {
    encoding: "utf-8",
    windowsHide: true,
    stdio: ["ignore", "pipe", "pipe"]
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
  event: async ({ event }: { event?: any }) => {
    if (!event) return;
    const type = event.type || event.event || "";
    const sessionID = event.sessionID || event.sessionId || event.properties?.sessionID || event.properties?.info?.id || "";

    // 1. Detectar inicio de subagente o subtask
    if (type === "session.created" && event.properties?.info?.parentID) {
      const role = event.properties?.info?.role || event.properties?.info?.agent || "subagent";
      subagentStartTimes.set(sessionID, Date.now());
      logDelegation(`🚀 [CORTEX-IA] Subagente iniciado: '${role}' (Session: ${sessionID})`);
    } else if (type === "subtask.created") {
      const taskID = event.properties?.id || "subtask";
      subagentStartTimes.set(taskID, Date.now());
      logDelegation(`🚀 [CORTEX-IA] Tarea en background iniciada: '${taskID}'`);
    }

    // 2. Detectar finalización de subagente o subtask
    if (type === "session.idle" || type === "subtask.completed" || type === "session.error") {
      const startTime = subagentStartTimes.get(sessionID);
      const durationSec = startTime ? Math.round((Date.now() - startTime) / 1000) : 0;
      const status = type === "session.error" ? "ERROR" : "COMPLETED";
      const role = event.properties?.info?.role || event.properties?.info?.agent || "subagent";
      
      if (startTime) {
        logDelegation(`✅ [CORTEX-IA] Subagente '${role}' finalizado en ${durationSec}s (Estado: ${status})`);
        subagentStartTimes.delete(sessionID);
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
    cortex_delegate_start: tool({
      description: "Ask cortex-ia to supervise one external AGY leaf. An implement controller must claim the cortex-ia work item and reserve file leases before calling this tool. The returned execution_mode is authoritative. Poll status, then read the receipt; execute natively only when delegated is false and no external job was accepted.",
      args: {
        role: tool.schema.enum(["implement", "investigate", "reviewer", "planner"]),
        task_id: tool.schema.string().optional(),
        objective: tool.schema.string(),
        worktree: tool.schema.string().optional().describe("Absolute isolated worktree path; required for implement"),
        allowed_files: tool.schema.array(tool.schema.string()).optional(),
        acceptance_checks: tool.schema.array(tool.schema.string()).optional(),
        context_data: tool.schema.string().optional(),
        claim_confirmed: tool.schema.boolean().optional(),
        lease_confirmed: tool.schema.boolean().optional()
      },
      async execute(args, context) {
        let requestPath = "";
        let acceptedJob: any = null;
        let acceptedTransport: "direct" | "herdr" = "direct";
        try {
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
            worktree: args.worktree ? path.resolve(args.worktree) : path.resolve(context.directory),
            allowed_files: args.allowed_files || [],
            authority: {
              claim_confirmed: args.claim_confirmed !== false,
              lease_confirmed: args.lease_confirmed !== false
            },
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
            try {
              const split = execFileSync(herdr, ["pane", "split", "--direction", config.direction, "--cwd", context.directory, "--no-focus"], { encoding: "utf-8", windowsHide: true });
              const pane = paneID(split);
              if (!pane) throw new Error("Herdr did not return a pane ID");
              activeJobPanes.set(job.job_id, pane);
              const worker = ["delegate", "worker", "--job", job.job_id, "--request-file", requestPath];
              execFileSync(herdr, ["pane", "run", pane, firstExecutable("cortex-ia"), ...worker], { encoding: "utf-8", windowsHide: true });
              return JSON.stringify({ delegated: true, execution_mode: executionMode(transport), job_id: job.job_id, status: job.status, transport, pane_id: pane });
            } catch {
              cortex(["delegate", "cancel", job.job_id]);
              acceptedJob = null;
              transport = "direct";
              job = parseJSON(cortex(["delegate", "create", "--request-file", requestPath, "--transport", transport]));
              acceptedJob = job;
              acceptedTransport = transport;
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
            try {
              cortex(["delegate", "cancel", job.job_id]);
              acceptedJob = null;
            } catch (cancelError: any) {
              return JSON.stringify({
                delegated: true,
                execution_mode: executionMode(acceptedTransport),
                job_id: job.job_id,
                status: "cancellation_unknown",
                transport: acceptedTransport,
                reason: cancelError?.message || "launch failed and cancellation could not be confirmed",
                action: "RECONCILE_EXTERNAL_JOB"
              });
            }
            throw error;
          }
          child.unref();
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

    cortex_delegation_result: tool({
      description: "Read the durable structured receipt of a completed cortex-ia delegation job.",
      args: { job_id: tool.schema.string() },
      async execute(args) {
        const result = cortex(["delegate", "result", args.job_id]);
        const pane = activeJobPanes.get(args.job_id);
        if (pane) {
          try {
            const herdr = firstExecutable("herdr");
            execFileSync(herdr, ["pane", "close", pane], { encoding: "utf-8", windowsHide: true });
            activeJobPanes.delete(args.job_id);
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
        const pane = activeJobPanes.get(args.job_id);
        if (pane) {
          try {
            const herdr = firstExecutable("herdr");
            execFileSync(herdr, ["pane", "close", pane], { encoding: "utf-8", windowsHide: true });
            activeJobPanes.delete(args.job_id);
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
