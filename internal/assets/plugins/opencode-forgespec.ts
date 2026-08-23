/** Official OpenCode plugin boundary for the local ForgeSpec identity broker. */
import { spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

export interface PluginClient {
  session?: {
    get: (params: { path: { id: string } }) => Promise<{
      data?: {
        parentID?: string;
        parentId?: string;
        parent_id?: string;
        [key: string]: unknown;
      };
    }>;
  };
  app?: {
    log?: (params: {
      body: {
        service: string;
        level: "debug" | "info" | "warn" | "error";
        message: string;
        extra?: Record<string, unknown>;
      };
    }) => void;
  };
}

export interface PluginContext {
  client?: PluginClient;
  nodePath?: string;
  mcpPath?: string;
  brokerPath?: string;
  broker?: {
    ready?: Promise<any>;
    request?: (payload: unknown) => Promise<any>;
    before?: (payload: unknown) => Promise<any>;
    close?: () => void;
  };
  options?: {
    nodePath?: string;
    mcpPath?: string;
    brokerPath?: string;
    broker?: PluginContext["broker"];
  };
}

export interface ToolExecuteBeforeInput {
  tool?: string;
  sessionID?: string;
  sessionId?: string;
  callID?: string;
}

export interface ToolExecuteBeforeOutput {
  args: Record<string, unknown>;
}

export interface PluginHooks {
  config: (config?: Record<string, any>) => Promise<void>;
  "tool.execute.before": (input: ToolExecuteBeforeInput, output: ToolExecuteBeforeOutput) => Promise<void>;
  "tool.execute.after": (input: unknown, output: unknown) => Promise<void>;
  event: (params?: { event?: { type?: string; properties?: any; sessionID?: string; sessionId?: string } }) => Promise<void>;
  dispose: () => Promise<void>;
}

const TOOL_NAMES = new Set([
  "attempt_claim", "attempt_recover", "attempt_renew", "authority_manage",
  "approval_record", "board_create", "contract_commit", "contract_query",
  "contract_validate", "event_query", "forge_health", "forge_negotiate",
  "lease_release", "lease_renew", "lease_reserve", "task_define",
  "task_query", "task_transition"
]);

const TOOL = /^forgespec_([A-Za-z0-9_]+)$/;
const ALIAS = /^(?:caller|caller_id|callerId|actor|actor_id|actorId|identity|_identity)$/i;
const READY_MS = 2_000;
const MAX_DEPTH = 64;

const strip = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(strip);
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value)
      .filter(([key]) => !ALIAS.test(key))
      .map(([key, item]) => [key, strip(item)])
  );
};

const frame = (id: string, payload: unknown): string => `${JSON.stringify({ id, payload })}\n`;
const logOf = (client?: PluginClient) => (level: "debug" | "info" | "warn" | "error", message: string, extra: Record<string, unknown> = {}) =>
  client?.app?.log?.({ body: { service: "opencode-forgespec", level, message, extra } });

function lineage(client: PluginClient | undefined, sessionId: string | undefined, cache: Map<string, string[]>) {
  const walk = async (id: string, seen = new Set<string>(), chain: string[] = []): Promise<string[]> => {
    if (typeof id !== "string" || !id || seen.has(id) || chain.length > MAX_DEPTH) throw new Error("SESSION_LINEAGE_INVALID");
    if (cache.has(id)) return cache.get(id)!;
    seen.add(id);
    const response = await client?.session?.get({ path: { id } }).catch(() => undefined);
    const session = response?.data;
    if (!session) {
      cache.set(id, [id]);
      return [id];
    }
    const parent = session?.parentID ?? session?.parentId ?? session?.parent_id;
    const next = parent ? [...await walk(parent, seen, chain), id] : [id];
    if (next.length > MAX_DEPTH + 1) throw new Error("SESSION_LINEAGE_INVALID");
    cache.set(id, next);
    return next;
  };
  const effectiveId = (typeof sessionId === "string" && sessionId.trim()) ? sessionId.trim() : "default-session";
  return walk(effectiveId).catch(() => [effectiveId]).then((ids) => ({
    root: ids[0],
    ...(ids.length > 1 ? { parent: ids[ids.length - 2] } : {}),
    worker: ids.at(-1)!,
    lineage: ids,
  }));
}

function resolveNodeCommand(context: PluginContext = {}): string {
  const explicit = context.nodePath ?? context.options?.nodePath ?? process.env.FORGESPEC_NODE_PATH;
  if (typeof explicit === "string" && explicit.trim()) return explicit.trim();
  const base = String(process.execPath).split(/[\\/]/).pop()?.toLowerCase();
  return base === "node" || base === "node.exe" ? process.execPath : "node";
}

function resolvePackagePath(specifier: string): string {
  const resolved = typeof import.meta.resolve === "function"
    ? import.meta.resolve(specifier)
    : createRequire(import.meta.url).resolve(specifier);
  return fileURLToPath(resolved);
}

function resolveSidecarPath(): string {
  const configured = process.env.FORGESPEC_IDENTITY_SIDECAR_PATH?.trim();
  if (configured) return configured;
  const dir = process.env.FORGESPEC_DIR?.trim() || path.join(os.homedir(), ".forgespec");
  return path.join(dir, "identity.db");
}

function localBroker({ client, broker: supplied, nodeCommand, brokerPath }: {
  client?: PluginClient;
  broker?: PluginContext["broker"];
  nodeCommand?: string;
  brokerPath?: string;
} = {}) {
  if (supplied) return { request: (input: unknown) => (supplied.request ? supplied.request(input) : supplied.before!(input)), close: () => supplied.close?.() };
  const entry = brokerPath ?? resolvePackagePath("forgespec-mcp/broker");
  const sidecarPath = resolveSidecarPath();
  const child: ChildProcess = spawn(nodeCommand ?? resolveNodeCommand(), [entry], {
    shell: false,
    stdio: ["pipe", "pipe", "pipe"],
    env: { ...process.env, FORGESPEC_IDENTITY_SIDECAR_PATH: sidecarPath }
  });
  const pending = new Map<string, { resolve: (value: any) => void; reject: (reason: any) => void }>();
  let buffer = "";
  let closed = false;
  let readyResolve: (value: any) => void;
  let readyReject: (reason: any) => void;
  const ready = new Promise<any>((resolveReady, rejectReady) => {
    readyResolve = resolveReady;
    readyReject = rejectReady;
  });
  const fail = (error: Error) => {
    closed = true;
    for (const item of pending.values()) item.reject(error);
    pending.clear();
  };
  const consume = (chunk: Buffer | string) => {
    buffer += String(chunk);
    if (buffer.length > 64 * 1024) return fail(new Error("BROKER_OUTPUT_TOO_LARGE"));
    const rows = buffer.split("\n");
    buffer = rows.pop() ?? "";
    for (const row of rows) if (row) {
      try {
        const value = JSON.parse(row);
        if (value.type === "ready") { readyResolve(value); continue; }
        const item = pending.get(value.id);
        if (!item) continue;
        pending.delete(value.id);
        value.payload?.error ? item.reject(new Error(value.payload.error.code || "BROKER_ERROR")) : item.resolve(value.payload);
      } catch {
        fail(new Error("BROKER_PROTOCOL_ERROR"));
        return;
      }
    }
  };
  const timer = setTimeout(() => { readyReject(new Error("BROKER_READY_TIMEOUT")); fail(new Error("BROKER_READY_TIMEOUT")); }, READY_MS);
  ready.finally(() => clearTimeout(timer)).catch(() => undefined);
  child.stdout?.on("data", consume);
  child.stderr?.on("data", (chunk) => { if (String(chunk).length > 64 * 1024) fail(new Error("BROKER_STDERR_TOO_LARGE")); });
  child.once("error", (error) => { readyReject(error); fail(error); });
  child.once("exit", () => { const error = new Error("BROKER_UNAVAILABLE"); readyReject(error); fail(error); });
  return {
    ready,
    request(input: unknown) {
      return ready.then(() => new Promise((resolveReply, reject) => {
        if (closed) return reject(new Error("BROKER_UNAVAILABLE"));
        const id = randomUUID();
        pending.set(id, { resolve: resolveReply, reject });
        child.stdin?.write(frame(id, input));
      }));
    },
    close() {
      if (!closed) {
        fail(new Error("BROKER_DISPOSED"));
        child.stdin?.end();
        child.kill();
      }
    }
  };
}

export default async function forgeSpecPlugin(context: PluginContext = {}): Promise<PluginHooks> {
  const client = context.client;
  const log = logOf(client);
  const cache = new Map<string, string[]>();
  const nodeCommand = resolveNodeCommand(context);
  const broker = localBroker({
    ...context,
    broker: context.broker ?? context.options?.broker,
    brokerPath: context.brokerPath ?? context.options?.brokerPath,
    nodeCommand
  });
  let bootstrap: any;
  const ensure = async () => {
    bootstrap ??= broker.ready ? await broker.ready : undefined;
    return bootstrap;
  };
  const hooks: PluginHooks = {
    config: async (config: Record<string, any> = {}) => {
      const ready = await ensure().catch((error: Error) => {
        log("warn", "ForgeSpec broker readiness unavailable", { code: error?.message });
        return undefined;
      });
      const mcpIndexPath = context.mcpPath ?? context.options?.mcpPath ?? resolvePackagePath("forgespec-mcp/mcp");
      const environment = {
        FORGESPEC_IDENTITY_ROOT_PUBLIC_KEY: ready?.root_public_key ?? "",
        FORGESPEC_IDENTITY_ISSUER: ready?.issuer ?? "",
        FORGESPEC_IDENTITY_AUDIENCE: ready?.audience ?? "broker",
        FORGESPEC_IDENTITY_SIDECAR_PATH: resolveSidecarPath()
      };
      config.mcp ??= {};
      config.mcp.forgespec = { type: "local", command: [nodeCommand, mcpIndexPath], enabled: true, environment };
    },
    "tool.execute.before": async (input, output) => {
      if (!TOOL.test(input?.tool ?? "")) return;
      const match = TOOL.exec(input.tool ?? "");
      const requested = match?.[1];
      const toolName = requested?.startsWith("forgespec_") ? `forge_${requested.slice("forgespec_".length)}` : requested;
      if (!toolName || !TOOL_NAMES.has(toolName)) return;
      try {
        const ready = await ensure();
        const full = await lineage(client, input.sessionID ?? input.sessionId, cache);
        const { lineage: ordered, ...session } = full;
        const args = strip(output?.args ?? {}) as Record<string, unknown>;
        const envelope = await broker.request({ session, lineage: ordered, tool: toolName, args, call: input.callID, audience: ready?.audience });
        const target = output.args;
        for (const key of Object.keys(target)) delete target[key];
        Object.assign(target, args, { _identity: envelope });
      } catch (error: any) {
        log("warn", "ForgeSpec identity injection failed closed", { tool: input.tool, code: error?.message });
        throw new Error("FORGESPEC_IDENTITY_UNAVAILABLE");
      }
    },
    "tool.execute.after": async () => undefined,
    event: async ({ event } = {}) => {
      if (event?.type === "session.deleted") {
        const id = event.properties?.info?.id ?? event.properties?.sessionID ?? event.sessionID ?? event.sessionId;
        if (id) {
          for (const [key, chain] of cache) {
            if (key === id || chain.includes(id)) cache.delete(key);
          }
        }
      }
    },
    dispose: async () => broker.close(),
  };
  return hooks;
}

export { forgeSpecPlugin };
