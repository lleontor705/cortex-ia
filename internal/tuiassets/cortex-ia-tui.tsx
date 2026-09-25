import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
  TuiThemeCurrent,
} from "@opencode-ai/plugin/tui";
import { execFile, spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import {
  For,
  Show,
  createEffect,
  createMemo,
  createRoot,
  createSignal,
  untrack,
} from "solid-js";
import { useTerminalDimensions } from "@opentui/solid";
import { TextAttributes } from "@opentui/core";
import type { BoxRenderable, ColorInput } from "@opentui/core";

const SNAPSHOT_POLL_INTERVAL_MS = 2500;
const SNAPSHOT_STALE_MS = 10_000;
const MAX_VISIBLE_ROWS = 4;
const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const NEURAL_PULSE_FRAMES = ["◆", "◇", "○", "◇"];

const NAN_USAGE_POLL_INTERVAL_MS = 60_000;
const NAN_CATALOG_TTL_MS = 10 * 60_000;
const NAN_EXEC_MAX_BUFFER = 4 * 1024 * 1024;
const NAN_EXEC_TIMEOUT_MS = 8000;
const NAN_DETAIL_DAY_POINTS = 21;

// Windows Terminal renders every glyph below as a single-width cell with no VS16
// emoji presentation, so fixed-width pills and column math stay exact.
const GLYPH = {
  active: "◆",
  working: "●",
  idle: "○",
  done: "✓",
  fail: "✕",
  warn: "▲",
  row: "▸",
  web: "◈",
  brand: "🧠",
} as const;
const MARK_EXPANDED = "▾";
const MARK_COLLAPSED = "▸";

const NARROW_TERMINAL_COLS = 96;

// Kanban deck ladder: >=96 cells fits three columns, >=64 fits two, below that the
// groups stack. Each separator reserves one "│" border cell plus one spacer cell so a
// bordered column can never overflow the measured width, and a column narrower than
// 12 cells cannot render a task id.
const KANBAN_TWO_COLUMN_COLS = 64;
const KANBAN_COLUMN_SEPARATOR = 2;
const KANBAN_MIN_COLUMN_WIDTH = 12;
const KANBAN_READY_LIMIT = 3;
const KANBAN_COLUMN_BORDER = ["left"] as ("left")[];

const TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
const DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
const ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
const DENSITY_KEY = "cortex.sidebar.density";
const ETA_HISTORY_KEY = "cortex.dashboard.eta.completions";
const ETA_HISTORY_LIMIT = 12;
const ETA_INTERVAL_WINDOW = 8;
const ETA_MIN_SAMPLES = 2;
const ETA_SPARK_MIN_SAMPLES = 3;
const ETA_SPARK_POINTS = 12;

// Braille Patterns (U+2800 block) are unambiguously single-width, so each
// sparkline column costs exactly one cell in every terminal font.
const ETA_SPARK_LEVELS = ["⡀", "⣀", "⣄", "⣆", "⣇", "⣧", "⣿"];

const AGENTS_ROUTE = "cortex.agents";
const AGENTS_COMMAND = ":cortex-agents";
const AGENTS_MIN_COLUMNS = 68;
const AGENTS_NARROW_COLUMNS = 96;
const AGENTS_TITLE_MIN = 14;
const AGENTS_STATUS_WIDTH = 2;
const AGENTS_AGENT_WIDTH = 12;
const AGENTS_NARROW_AGENT_WIDTH = 9;
const AGENTS_ELAPSED_WIDTH = 8;
const AGENTS_ACTIVITY_WIDTH = 16;
const AGENTS_TOKENS_WIDTH = 8;
const AGENTS_COST_WIDTH = 8;
const AGENTS_TASK_WIDTH = 20;
const OPENCODE_SESSION_OWNER_PREFIX = "opencode-session:";

type CortexPalette = {
  text: ColorInput;
  textMuted: ColorInput;
  textSoft: ColorInput;
  accent: ColorInput;
  accentAlt: ColorInput;
  primary: ColorInput;
  sky: ColorInput;
  success: ColorInput;
  warning: ColorInput;
  error: ColorInput;
  info: ColorInput;
  border: ColorInput;
  borderSubtle: ColorInput;
  borderActive: ColorInput;
  panel: ColorInput;
  element: ColorInput;
  background: ColorInput;
};

// Defensive flat hex fallbacks aligned to the cortex identity. They are consulted
// only when the active theme omits a role, so an unknown theme still renders.
const PALETTE_FALLBACK = {
  text: "#f8fafc",
  textMuted: "#94a3b8",
  accent: "#a855f7",
  accentAlt: "#22d3ee",
  primary: "#6366f1",
  sky: "#38bdf8",
  success: "#34d399",
  warning: "#fbbf24",
  error: "#fb7185",
  info: "#22d3ee",
  border: "#334155",
  panel: "#111927",
  element: "#1e293b",
  background: "#0a0e17",
} as const;

// Share of the primary text color in the derived soft-text blend; the remainder
// comes from the theme background, so secondary copy follows any theme without a
// hardcoded literal.
const SOFT_TEXT_WEIGHT = 0.65;

type RgbBytes = { r: number; g: number; b: number };

function readColor(value: unknown): ColorInput | undefined {
  if (typeof value === "string" && value.trim() !== "") return value;
  if (typeof value === "object" && value !== null) {
    const candidate = value as { r?: unknown; g?: unknown; b?: unknown };
    if (typeof candidate.r === "number" && typeof candidate.g === "number" && typeof candidate.b === "number") {
      return value as ColorInput;
    }
  }
  return undefined;
}

function hexToBytes(value: string): RgbBytes | undefined {
  const hex = value.trim().replace(/^#/, "");
  if (!/^[0-9a-f]{6}$/i.test(hex)) return undefined;
  return {
    r: parseInt(hex.slice(0, 2), 16),
    g: parseInt(hex.slice(2, 4), 16),
    b: parseInt(hex.slice(4, 6), 16),
  };
}

// OpenTUI RGBA getters report normalized 0..1 floats; raw integer channels are
// accepted defensively so a byte-oriented theme object still blends correctly.
function channelToByte(value: number): number {
  const scaled = Math.abs(value) <= 1 ? value * 255 : value;
  return Math.round(Math.min(255, Math.max(0, scaled)));
}

function colorToBytes(color: ColorInput): RgbBytes | undefined {
  if (typeof color === "string") return hexToBytes(color);
  const candidate = color as { r?: unknown; g?: unknown; b?: unknown };
  if (typeof candidate.r !== "number" || typeof candidate.g !== "number" || typeof candidate.b !== "number") {
    return undefined;
  }
  return { r: channelToByte(candidate.r), g: channelToByte(candidate.g), b: channelToByte(candidate.b) };
}

function blendColor(foreground: ColorInput, background: ColorInput, weight: number): ColorInput {
  const fg = colorToBytes(foreground);
  const bg = colorToBytes(background);
  if (!fg || !bg) return foreground;
  const mix = (a: number, b: number) => Math.round(a * weight + b * (1 - weight));
  const toHex = (value: number) => value.toString(16).padStart(2, "0");
  return `#${toHex(mix(fg.r, bg.r))}${toHex(mix(fg.g, bg.g))}${toHex(mix(fg.b, bg.b))}`;
}

function buildPalette(theme?: TuiThemeCurrent): CortexPalette {
  const source = (theme ?? {}) as unknown as Record<string, unknown>;
  const read = (key: string, fallback: string): ColorInput => readColor(source[key]) ?? fallback;
  const text = read("text", PALETTE_FALLBACK.text);
  const background = read("background", PALETTE_FALLBACK.background);
  const border = read("border", PALETTE_FALLBACK.border);
  return {
    text,
    textMuted: read("textMuted", PALETTE_FALLBACK.textMuted),
    textSoft: blendColor(text, background, SOFT_TEXT_WEIGHT),
    accent: read("accent", PALETTE_FALLBACK.accent),
    accentAlt: read("secondary", PALETTE_FALLBACK.accentAlt),
    primary: read("primary", PALETTE_FALLBACK.primary),
    sky: read("info", PALETTE_FALLBACK.sky),
    success: read("success", PALETTE_FALLBACK.success),
    warning: read("warning", PALETTE_FALLBACK.warning),
    error: read("error", PALETTE_FALLBACK.error),
    info: read("info", PALETTE_FALLBACK.info),
    border,
    borderSubtle: readColor(source["borderSubtle"]) ?? border,
    borderActive: readColor(source["borderActive"]) ?? border,
    panel: read("backgroundPanel", PALETTE_FALLBACK.panel),
    element: read("backgroundElement", PALETTE_FALLBACK.element),
    background,
  };
}

// The palette is a pure function of the theme object identity, so one cache entry
// per theme reference keeps every widget on the same resolved colors at O(1).
const PALETTE_CACHE = new WeakMap<object, CortexPalette>();
let themelessPalette: CortexPalette | undefined;

function resolvePalette(theme?: TuiThemeCurrent): CortexPalette {
  if (theme && typeof theme === "object") {
    const cached = PALETTE_CACHE.get(theme);
    if (cached) return cached;
    const built = buildPalette(theme);
    PALETTE_CACHE.set(theme, built);
    return built;
  }
  return (themelessPalette ??= buildPalette());
}

type DelegationEvent = {
  timestamp?: string;
  kind?: string;
  job_id?: string;
  role?: string;
  status?: string;
  transport?: string;
  pane_id?: string;
  workspace?: string;
};

type DelegationJob = Required<Pick<DelegationEvent, "job_id" | "status">> &
  DelegationEvent & {
    sequence: number;
    task_id?: string;
    error_code?: string;
    error_message?: string;
    updated_at?: string;
    attempt?: number;
  };

type DashboardTask = {
  task_id: string;
  board_id: string;
  title: string;
  status: string;
  revision: number;
  owner?: string;
  claim_expires_at?: string;
  lease_count: number;
  updated_at: string;
  // Host-proven conversation provenance published by the snapshot payload.
  opencode_session_id?: string;
  opencode_root_session_id?: string;
  opencode_parent_session_id?: string;
};

type DashboardDelegation = {
  job_id: string;
  role: string;
  task_id?: string;
  status: string;
  transport: string;
  attempt: number;
  error_code?: string;
  error_message?: string;
  updated_at: string;
};

type UISnapshotSummary = {
  active_tasks: number;
  total_tasks: number;
  backlog?: number;
  ready?: number;
  in_progress?: number;
  in_review?: number;
  blocked?: number;
  done?: number;
  superseded?: number;
  total_delegations: number;
  active_delegations?: number;
  completed_delegations?: number;
  failed_delegations?: number;
  active_executions?: number;
  total_attention: number;
};

export type UISnapshot = {
  schema_version: number;
  generated_at: string;
  project_root: string;
  requested_session_id: string;
  root_session_id: string;
  summary: UISnapshotSummary;
  counts: OperationalCounts;
  attention: { id: string; entity_id: string; kind: string; title: string }[];
  tasks: DashboardTask[];
  delegations: DashboardDelegation[];
};

type AttentionItem = { id: string; title: string; detail: string };
type OperationalCounts = { active: number; review: number; attention: number };
type NativeActivity = "busy" | "idle" | "retry" | "unknown";
type SidebarLayout = { compact: boolean; textLimit: number; gaugeWidth: number };
type DensityMode = "compact" | "expanded";
type SidebarMetrics = {
  tokensUsed?: number;
  tokenLimit?: number;
  cost?: number;
};
type Tier1Eta = {
  remaining: number;
  backlog: number;
  samples: number;
  estimateMs?: number;
  sparkline?: string;
};

function sidebarLayout(width: number): SidebarLayout {
  const measured = Number.isFinite(width) && width > 0 ? Math.floor(width) : 0;
  return {
    compact: measured === 0 || measured < 40,
    textLimit: Math.max(8, measured ? measured - 6 : 26),
    gaugeWidth: Math.max(3, Math.min(14, measured ? measured - 14 : 8)),
  };
}

const EMPTY_SNAPSHOT: UISnapshot = {
  schema_version: 2,
  generated_at: "",
  project_root: "",
  requested_session_id: "",
  root_session_id: "",
  summary: {
    active_tasks: 0,
    total_tasks: 0,
    backlog: 0,
    ready: 0,
    in_progress: 0,
    in_review: 0,
    blocked: 0,
    done: 0,
    superseded: 0,
    total_delegations: 0,
    active_delegations: 0,
    completed_delegations: 0,
    failed_delegations: 0,
    active_executions: 0,
    total_attention: 0,
  },
  counts: { active: 0, review: 0, attention: 0 },
  attention: [],
  tasks: [],
  delegations: [],
};

function cortexExecutable(): string {
  if (process.env.CORTEX_IA_BIN) return process.env.CORTEX_IA_BIN;
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(home, "go", "bin", process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia"),
    process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia",
    path.join(local, "Programs", "cortex-ia", "bin", process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia"),
    path.join(home, ".local", "bin", "cortex-ia"),
    "/usr/local/bin/cortex-ia",
    "/usr/bin/cortex-ia",
  ];
  for (const candidate of candidates) {
    if (candidate !== "cortex-ia" && candidate !== "cortex-ia.exe") {
      try {
        if (fs.existsSync(candidate)) return candidate;
      } catch {}
    }
  }
  return process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia";
}

// The nan CLI ships outside PATH (per-user Programs install), so the strip must
// probe explicit candidates before falling back to a bare PATH lookup.
function nanExecutable(): string {
  if (process.env.NAN_BIN) return process.env.NAN_BIN;
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(local, "Programs", "nan", process.platform === "win32" ? "nan.exe" : "nan"),
    path.join(home, ".local", "bin", "nan"),
    "/usr/local/bin/nan",
    "/usr/bin/nan",
  ];
  for (const candidate of candidates) {
    try {
      if (fs.existsSync(candidate)) return candidate;
    } catch {}
  }
  return process.platform === "win32" ? "nan.exe" : "nan";
}

function openWebConsole(boardId?: string, taskId?: string): void {
  const args = ["web", "--open", "--daemon"];
  if (boardId) {
    args.push("--board", boardId);
  }
  if (taskId) {
    args.push("--task", taskId);
  }
  try {
    const child = spawn(cortexExecutable(), args, {
      detached: true,
      stdio: "ignore",
      windowsHide: true,
    });
    child.unref();
  } catch {
    // ignore launch errors in TUI UI click
  }
}

function shortID(id: string): string {
  return id.length > 13 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function clipped(value: string, limit = 28): string {
  const text = value.trim();
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function measuredColumns(width?: number): number {
  if (typeof width === "number" && Number.isFinite(width) && width > 0) return width;
  const columns = (process.stdout as any)?.columns;
  return typeof columns === "number" && Number.isFinite(columns) && columns > 0 ? columns : 0;
}

function formatDuration(ms: number): string {
  if (ms <= 0 || !Number.isFinite(ms)) return "00:00";
  const totalSecs = Math.floor(ms / 1000);
  const mins = Math.floor(totalSecs / 60);
  const secs = totalSecs % 60;
  if (mins >= 60) {
    const hours = Math.floor(mins / 60);
    const remMins = mins % 60;
    return `${hours}h ${remMins}m`;
  }
  return `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}

function formatTokens(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(Math.round(value));
}

function formatCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0.00";
  return value >= 0.01 ? value.toFixed(2) : value.toFixed(4);
}

function sessionRecord(api: any, sessionID?: string): any {
  if (!sessionID) return undefined;
  return api.state?.session?.get?.(sessionID) || api.data?.session?.get?.(sessionID);
}

function sessionStartTime(api: any, sessionID?: string): number | undefined {
  const created = sessionRecord(api, sessionID)?.time?.created;
  if (typeof created !== "number" || !Number.isFinite(created) || created <= 0) return undefined;
  // Older payloads report epoch seconds; the TUI clock works in milliseconds.
  return created < 1_000_000_000_000 ? created * 1000 : created;
}

function sessionCostUsd(api: any, sessionID?: string): number | undefined {
  const cost = sessionRecord(api, sessionID)?.cost;
  return typeof cost === "number" && Number.isFinite(cost) && cost > 0 ? cost : undefined;
}

function sessionContextLimit(api: any, sessionID?: string): number | undefined {
  const model = sessionRecord(api, sessionID)?.model;
  if (!model?.id || !model?.providerID) return undefined;
  const providers = api.state?.provider || api.data?.provider;
  if (!Array.isArray(providers)) return undefined;
  const limit = providers.find((p: any) => p?.id === model.providerID)?.models?.[model.id]?.limit?.context;
  return typeof limit === "number" && Number.isFinite(limit) && limit > 0 ? limit : undefined;
}

function sessionMessages(api: any, sessionID?: string): readonly any[] | undefined {
  if (!sessionID) return undefined;
  for (const source of [api.state?.session, api.data?.session, api.session]) {
    if (!source || typeof source.messages !== "function") continue;
    try {
      const messages = source.messages(sessionID);
      if (Array.isArray(messages)) return messages;
    } catch {}
  }
  return undefined;
}

// Context window fill comes from the newest assistant message usage, never from the
// cumulative session totals, which grow with every turn of the conversation.
function contextTokensUsed(messages: readonly any[] | undefined): number | undefined {
  if (!messages) return undefined;
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const info = messages[i]?.info || messages[i];
    if (info?.role !== "assistant") continue;
    const tokens = info?.tokens;
    if (!tokens) return undefined;
    const used =
      (tokens.input || 0) +
      (tokens.cache?.read || 0) +
      (tokens.cache?.write || 0) +
      (tokens.output || 0) +
      (tokens.reasoning || 0);
    return used > 0 ? used : undefined;
  }
  return undefined;
}

function loadEtaHistory(raw: unknown): number[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((value): value is number => typeof value === "number" && Number.isFinite(value) && value > 0)
    .slice(-ETA_HISTORY_LIMIT);
}

function completionIntervals(stamps: readonly number[]): number[] {
  const intervals: number[] = [];
  for (let i = 1; i < stamps.length; i += 1) {
    const delta = stamps[i] - stamps[i - 1];
    if (delta > 0) intervals.push(delta);
  }
  return intervals;
}

function meanCompletionInterval(stamps: readonly number[]): number | undefined {
  if (stamps.length < ETA_MIN_SAMPLES) return undefined;
  const intervals = completionIntervals(stamps);
  if (intervals.length === 0) return undefined;
  const recent = intervals.slice(-ETA_INTERVAL_WINDOW);
  return recent.reduce((sum, value) => sum + value, 0) / recent.length;
}

// Purely derived decoration over the persisted completion stamps: a flat series
// lands on the middle level instead of collapsing to a zero-height row.
function etaSparkline(stamps: readonly number[]): string | undefined {
  if (stamps.length < ETA_SPARK_MIN_SAMPLES) return undefined;
  const intervals = completionIntervals(stamps).slice(-ETA_SPARK_POINTS);
  if (intervals.length === 0) return undefined;
  const min = Math.min(...intervals);
  const span = Math.max(...intervals) - min;
  const top = ETA_SPARK_LEVELS.length - 1;
  return intervals
    .map((value) => ETA_SPARK_LEVELS[span === 0 ? Math.floor(ETA_SPARK_LEVELS.length / 2) : Math.round(((value - min) / span) * top)])
    .join("");
}

type RoleChip = { color: ColorInput; tag: string };

function roleChip(role: string, palette: CortexPalette): RoleChip {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return { color: palette.primary, tag: "ORCH" };
  if (r.includes("impl")) return { color: palette.warning, tag: "IMPL" };
  if (r.includes("rev")) return { color: palette.accent, tag: "REVW" };
  if (r.includes("inv")) return { color: palette.sky, tag: "INVS" };
  if (r.includes("plan")) return { color: palette.info, tag: "PLAN" };
  if (r.includes("disc")) return { color: palette.success, tag: "DISC" };
  return { color: palette.textMuted, tag: r.slice(0, 4).toUpperCase() || "WORK" };
}

function taskStatusChip(status: string, palette: CortexPalette): { icon: string; color: ColorInput; tag: string } {
  switch (status) {
    case "done":
      return { icon: GLYPH.done, color: palette.success, tag: "DONE" };
    case "in_progress":
      return { icon: GLYPH.working, color: palette.warning, tag: "PROG" };
    case "in_review":
      return { icon: GLYPH.active, color: palette.accent, tag: "REVW" };
    case "ready":
      return { icon: GLYPH.row, color: palette.sky, tag: "RDY " };
    case "blocked":
      return { icon: GLYPH.fail, color: palette.error, tag: "BLCK" };
    case "superseded":
      return { icon: ">>", color: palette.textMuted, tag: "SPRS" };
    case "backlog":
    default:
      return { icon: GLYPH.idle, color: palette.textMuted, tag: "WAIT" };
  }
}

type SubagentStatus = "busy" | "idle" | "retry" | "unknown";

type SubagentLive = {
  status?: SubagentStatus;
  retryAttempt?: number;
  title?: string;
  agent?: string;
  tool?: string;
  step?: string;
  tokens?: number;
  cost?: number;
  updatedAt?: number;
};

type SubagentRow = {
  sessionID: string;
  title: string;
  agent: string;
  status: SubagentStatus;
  retryAttempt?: number;
  startedAt?: number;
  tool?: string;
  step?: string;
  tokens?: number;
  cost?: number;
  updatedAt: number;
  taskID?: string;
  taskStatus?: string;
};

type AgentsLayout = {
  agent: number;
  activity: number;
  task: number;
  title: number;
};

function subagentStatus(status: unknown): SubagentStatus {
  const type = typeof status === "string" ? status : (status as { type?: unknown } | undefined)?.type;
  return type === "busy" || type === "idle" || type === "retry" ? type : "unknown";
}

function subagentRetryAttempt(status: unknown): number | undefined {
  const attempt = (status as { attempt?: unknown } | undefined)?.attempt;
  return typeof attempt === "number" && Number.isFinite(attempt) && attempt > 0 ? Math.floor(attempt) : undefined;
}

// Host sessions report epoch seconds on older payloads and milliseconds after the
// time.created migration; both shapes must resolve to one comparable instant.
function sessionStartMillis(session: { time?: { created?: unknown } } | undefined): number | undefined {
  const created = session?.time?.created;
  if (typeof created !== "number" || !Number.isFinite(created) || created <= 0) return undefined;
  return created < 1_000_000_000_000 ? created * 1000 : created;
}

function sessionTokenTotal(session: { tokens?: unknown } | undefined): number | undefined {
  const tokens = session?.tokens as { input?: number; output?: number; reasoning?: number; cache?: { read?: number; write?: number } } | undefined;
  if (!tokens) return undefined;
  const used =
    (tokens.input || 0) + (tokens.output || 0) + (tokens.reasoning || 0) + (tokens.cache?.read || 0) + (tokens.cache?.write || 0);
  return used > 0 ? used : undefined;
}

// The status column is a fixed two cells, so elapsed time renders as mm:ss and
// h:mm:ss instead of the sidebar's variable-width "1h 5m" form.
function formatElapsedClock(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "--:--";
  const totalSecs = Math.floor(ms / 1000);
  const secs = totalSecs % 60;
  const mins = Math.floor(totalSecs / 60) % 60;
  const hours = Math.floor(totalSecs / 3600);
  const clock = `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
  return hours > 0 ? `${hours}:${clock}` : clock;
}

function fitWidth(value: string, width: number): string {
  if (width <= 0) return "";
  if (value.length > width) return `${value.slice(0, Math.max(0, width - 1))}…`;
  return value.padEnd(width, " ");
}

function taskForSession(sessionID: string, tasks: readonly DashboardTask[]): { taskID: string; status: string } | undefined {
  for (const task of tasks || []) {
    if (!task) continue;
    const owner = typeof task.owner === "string" ? task.owner : "";
    const ownerSession = owner.startsWith(OPENCODE_SESSION_OWNER_PREFIX) ? owner.slice(OPENCODE_SESSION_OWNER_PREFIX.length) : "";
    if (
      task.opencode_session_id === sessionID ||
      task.opencode_parent_session_id === sessionID ||
      ownerSession === sessionID
    ) {
      return { taskID: task.task_id, status: task.status };
    }
  }
  return undefined;
}

function partActivityLabel(part: any): { tool?: string; step?: string } | undefined {
  if (!part || typeof part.type !== "string") return undefined;
  if (part.type === "tool") return typeof part.tool === "string" && part.tool ? { tool: part.tool } : undefined;
  if (part.type === "reasoning") return { step: "reasoning" };
  if (part.type === "text") return { step: "writing" };
  if (part.type === "step-start") return { step: "step" };
  return undefined;
}

// Only the newest assistant message can describe what the subagent is doing now;
// older messages would report a tool call that already finished.
function lastActivityLabel(api: any, sessionID: string): { tool?: string; step?: string } {
  const messages = sessionMessages(api, sessionID);
  const partFn = api?.state?.part;
  if (!messages || typeof partFn !== "function") return {};
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const info = (messages[i] as any)?.info || messages[i];
    if (info?.role !== "assistant" || typeof info?.id !== "string") continue;
    let parts: unknown;
    try {
      parts = partFn(info.id);
    } catch {
      return {};
    }
    if (!Array.isArray(parts)) return {};
    for (let j = parts.length - 1; j >= 0; j -= 1) {
      const label = partActivityLabel(parts[j]);
      if (label) return label;
    }
    return {};
  }
  return {};
}

function subagentRank(status: SubagentStatus): number {
  if (status === "busy") return 0;
  if (status === "retry") return 1;
  return 2;
}

function subagentStatusColor(status: SubagentStatus, palette: CortexPalette): ColorInput {
  if (status === "busy") return palette.accent;
  if (status === "retry") return palette.warning;
  if (status === "idle") return palette.textMuted;
  return palette.textSoft;
}

function attentionItems(snapshot: UISnapshot, snapshotError: string): AttentionItem[] {
  const items = snapshot.attention.map((item) => ({
    id: item.id,
    title: item.title,
    detail: `${item.kind} · ${shortID(item.entity_id)}`,
  }));
  if (snapshotError) {
    items.push({
      id: "snapshot-error",
      title: "Snapshot unavailable",
      detail: clipped(snapshotError, 35),
    });
  }
  return items;
}

function operationalCounts(snapshot: UISnapshot, snapshotError: string): OperationalCounts {
  return {
    ...snapshot.counts,
    attention: snapshot.counts.attention + (snapshotError ? 1 : 0),
  };
}

let lastKnownSessionID: string | undefined;

function extractSessionID(val: any): string | undefined {
  if (!val) return undefined;
  if (typeof val === "function") {
    try {
      const res = val();
      const extracted = extractSessionID(res);
      if (extracted) return extracted;
    } catch {}
  }
  if (typeof val === "string") {
    const trimmed = val.trim();
    if (/^[A-Za-z0-9_-]{1,256}$/.test(trimmed)) return trimmed;
    const match = trimmed.match(/(?:^|\/|#)session(?:s)?\/([A-Za-z0-9_-]{1,256})/);
    if (match) return match[1];
  }
  if (typeof val === "object") {
    const candidates = [
      val.sessionID,
      val.sessionId,
      val.session_id,
      val.params?.sessionID,
      val.params?.sessionId,
      val.params?.session_id,
      val.params?.id,
      val.type === "session" ? val.id : undefined,
      val.name === "session" ? val.id : undefined,
      val.session?.id,
      val.session?.sessionID,
    ];
    for (const c of candidates) {
      const extracted = extractSessionID(c);
      if (extracted) return extracted;
    }
    if (typeof val.path === "string") {
      const extracted = extractSessionID(val.path);
      if (extracted) return extracted;
    }
  }
  return undefined;
}

function currentSessionID(api: any, explicit?: any): string | undefined {
  const fromExplicit = extractSessionID(explicit);
  if (fromExplicit) {
    lastKnownSessionID = fromExplicit;
    return fromExplicit;
  }
  const route = api.route?.current || (typeof api.ui?.router?.current === "function" ? api.ui.router.current() : undefined);
  const routeId = extractSessionID(route);
  if (routeId) {
    lastKnownSessionID = routeId;
    return routeId;
  }
  if (api.ui?.tabs) {
    try {
      if (typeof api.ui.tabs.current === "function") {
        const curTabId = extractSessionID(api.ui.tabs.current());
        if (curTabId) {
          lastKnownSessionID = curTabId;
          return curTabId;
        }
      }
      if (typeof api.ui.tabs.active === "function") {
        const activeTabId = extractSessionID(api.ui.tabs.active());
        if (activeTabId) {
          lastKnownSessionID = activeTabId;
          return activeTabId;
        }
      }
      if (typeof api.ui.tabs.list === "function") {
        const tabs = api.ui.tabs.list();
        if (Array.isArray(tabs) && tabs.length > 0) {
          const active = tabs.find((t: any) => t && (t.active === true || t.selected === true || t.current === true));
          const activeId = extractSessionID(active);
          if (activeId) {
            lastKnownSessionID = activeId;
            return activeId;
          }
          const firstId = extractSessionID(tabs[0]);
          if (firstId) {
            lastKnownSessionID = firstId;
            return firstId;
          }
        }
      }
    } catch {}
  }
  const sessionApiId = extractSessionID(api.session) || extractSessionID(api.state?.session) || extractSessionID(api.data?.session);
  if (sessionApiId) {
    lastKnownSessionID = sessionApiId;
    return sessionApiId;
  }
  return lastKnownSessionID;
}

function nativeSessionActivity(api: any, explicit?: any): NativeActivity | undefined {
  const id = currentSessionID(api, explicit);
  if (!id) return undefined;
  const session = api.state?.session?.get?.(id) || api.data?.session?.get?.(id);
  if (session && session.id !== id) return "unknown";
  const statusObj = api.state?.session?.status?.(id) || api.data?.session?.status?.(id);
  const status = typeof statusObj === "string" ? statusObj : statusObj?.type;
  return status === "busy" || status === "idle" || status === "retry" ? status : "unknown";
}

function conversationScope(api: any, explicit?: any) {
  const sessionID = currentSessionID(api, explicit);
  const session = sessionID ? (api.state?.session?.get?.(sessionID) || api.data?.session?.get?.(sessionID)) : undefined;
  const project =
    api.state?.path?.directory ||
    session?.directory ||
    api.location?.directory ||
    (typeof api.data?.location?.default === "function" ? api.data.location.default()?.directory : undefined) ||
    process.cwd();

  if (!sessionID) {
    return { sessionID: "global", rootSessionID: "global", project };
  }
  let current = sessionID;
  const seen = new Set<string>();
  while (seen.size < 64 && /^[A-Za-z0-9_-]{1,256}$/.test(current) && !seen.has(current)) {
    seen.add(current);
    const currSession = api.state?.session?.get?.(current) || api.data?.session?.get?.(current);
    if (!currSession || currSession.id !== current) {
      if (api.data?.session?.root && typeof api.data.session.root === "function") {
        try {
          const root = api.data.session.root(sessionID);
          if (root) return { sessionID, rootSessionID: root, project };
        } catch {}
      }
      return { sessionID, rootSessionID: current, project };
    }
    const parent = currSession.parentID || currSession.parentId;
    if (!parent) return { sessionID, rootSessionID: current, project };
    current = parent;
  }
  return { sessionID, rootSessionID: sessionID, project };
}

type SidebarFreshness = "live" | "stale" | "standby";

function CortexCockpitHeader(props: {
  isExecuting: () => boolean;
  nativeActivity: () => NativeActivity | undefined;
  freshness: () => SidebarFreshness;
  projectRoot?: string;
  sessionElapsed?: () => string | undefined;
  density: () => DensityMode;
  onToggleDensity: () => void;
  textLimit: number;
  spinner: () => string;
  theme?: TuiThemeCurrent;
}) {
  const projectName = createMemo(() => {
    if (!props.projectRoot) return "";
    return path.basename(props.projectRoot);
  });
  const palette = resolvePalette(props.theme);

  const freshnessColor = () => {
    if (props.freshness() === "live") return palette.success;
    if (props.freshness() === "stale") return palette.warning;
    return palette.textMuted;
  };
  const freshnessGlyph = () => {
    if (props.freshness() === "live") return GLYPH.working;
    if (props.freshness() === "stale") return GLYPH.warn;
    return GLYPH.idle;
  };
  // "unknown" means the host never resolved an activity, so the line is omitted.
  const resolvedActivity = () => {
    const activity = props.nativeActivity();
    return activity && activity !== "unknown" ? activity : undefined;
  };

  return (
    <box
      flexDirection="column"
      borderStyle="rounded"
      borderColor={props.isExecuting() ? palette.warning : palette.primary}
      title={
        props.isExecuting()
          ? `${GLYPH.brand} CORTEX·IA v2.0 [${props.spinner()} ACTIVE]`
          : `${GLYPH.brand} CORTEX·IA v2.0 [${GLYPH.working} STANDBY]`
      }
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.panel}
      paddingLeft={1}
      paddingRight={1}
    >
      {/* Project root keeps its own line so a long path cannot wrap the controls. */}
      <Show when={projectName()}>
        <box flexDirection="row" marginTop={0}>
          <text fg={palette.sky}>{`${GLYPH.row} `}</text>
          <text fg={palette.sky}>{clipped(projectName(), Math.max(6, props.textLimit - 4))}</text>
        </box>
      </Show>

      <box flexDirection="row" marginTop={0}>
        <text fg={freshnessColor()}>{`${freshnessGlyph()} ${props.freshness()}`}</text>
        <Show when={resolvedActivity()}>
          {(activity: () => NativeActivity) => (
            <text fg={palette.textMuted}>{` · OpenCode: ${activity()}`}</text>
          )}
        </Show>
      </box>

      <box flexDirection="row" marginTop={0}>
        <Show when={props.sessionElapsed?.()}>
          {(elapsed: () => string) => (
            <text fg={palette.sky}>{clipped(`T+${elapsed()} `, Math.max(8, props.textLimit))}</text>
          )}
        </Show>
        <text fg={palette.info} onMouseDown={() => openWebConsole()} selectable={false}>
          {`[${GLYPH.web} Web] `}
        </text>
        <text
          fg={props.density() === "compact" ? palette.success : palette.textMuted}
          onMouseDown={props.onToggleDensity}
          selectable={false}
        >
          {props.density() === "compact" ? `[${MARK_COLLAPSED} CMP]` : `[${MARK_EXPANDED} EXP]`}
        </text>
      </box>
    </box>
  );
}

function StatusCell(props: { palette: CortexPalette; label: string; value: () => string; color?: () => ColorInput }) {
  return (
    <box flexDirection="row" gap={1}>
      <text fg={props.palette.textMuted}>{props.label}</text>
      <text fg={props.color ? props.color() : props.palette.text}>{props.value()}</text>
    </box>
  );
}

function padMetric(value: number): string {
  return String(Math.max(0, Math.min(999, Math.round(value)))).padStart(2, "0");
}

function EmptyState(props: { palette: CortexPalette; label: string }) {
  return (
    <box flexDirection="row" paddingLeft={2}>
      <text fg={props.palette.textMuted}>{`${GLYPH.idle} ${props.label}`}</text>
    </box>
  );
}

function Section(props: {
  title: string;
  shortTitle?: string;
  badge?: string;
  shortBadge?: string;
  summary?: string;
  compact?: boolean;
  densityCompact?: boolean;
  expanded: () => boolean;
  onToggle: () => void;
  theme?: TuiThemeCurrent;
  children: unknown;
}) {
  const displayTitle = createMemo(() => (props.compact && props.shortTitle ? props.shortTitle : props.title));
  // Collapsed compact sections collapse to a single counts line in place of the badge.
  const displayBadge = createMemo(() => {
    if (props.densityCompact && !props.expanded() && props.summary) return props.summary;
    return props.compact && props.shortBadge ? props.shortBadge : props.badge;
  });
  const palette = resolvePalette(props.theme);

  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row" onMouseDown={props.onToggle}>
        <text fg={props.expanded() ? palette.info : palette.textMuted} selectable={false}>
          {props.expanded() ? `${MARK_EXPANDED} ` : `${MARK_COLLAPSED} `}
        </text>
        <text fg={palette.text} selectable={false}>
          {displayTitle()}
        </text>
        <Show when={displayBadge()}>
          <text fg={palette.sky}>{` [${displayBadge()}]`}</text>
        </Show>
      </box>
      <Show when={props.expanded()}>{props.children}</Show>
    </box>
  );
}

function MultiColorProgressBar(props: {
  done: number;
  inReview: number;
  inProgress: number;
  blocked?: number;
  total: number;
  width?: number;
  compact: boolean;
  textLimit: number;
  // Callers that reuse this bar for a non-DAG metric (e.g. quota) override the label.
  label?: string;
  theme?: TuiThemeCurrent;
}) {
  const w = createMemo(() => Math.max(3, Math.min(props.width || 14, props.textLimit - (props.compact ? 8 : 14))));
  const total = createMemo(() => Math.max(props.total, 1));
  const doneW = createMemo(() => Math.min(w(), Math.round((props.done / total()) * w())));
  const revW = createMemo(() => Math.min(Math.max(0, w() - doneW()), Math.round((props.inReview / total()) * w())));
  const progW = createMemo(() => Math.min(Math.max(0, w() - doneW() - revW()), Math.round((props.inProgress / total()) * w())));
  const blckW = createMemo(() => Math.min(Math.max(0, w() - doneW() - revW() - progW()), Math.round(((props.blocked || 0) / total()) * w())));
  const emptyW = createMemo(() => Math.max(0, w() - doneW() - revW() - progW() - blckW()));
  const pct = createMemo(() => Math.min(100, Math.max(0, Math.round((props.done / total()) * 100))));
  const waiting = createMemo(() =>
    Math.max(0, props.total - props.done - props.inReview - props.inProgress - (props.blocked || 0))
  );
  const showLegend = createMemo(() => !props.compact && props.textLimit >= 34);
  const palette = resolvePalette(props.theme);
  const block = "█";

  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row">
        <text fg={palette.info}>{props.label ?? (props.compact ? "D:" : "DAG: ")}</text>
        <text fg={palette.success}>{block.repeat(doneW())}</text>
        <text fg={palette.primary}>{block.repeat(revW())}</text>
        <text fg={palette.warning}>{block.repeat(progW())}</text>
        <Show when={blckW() > 0}>
          <text fg={palette.error}>{block.repeat(blckW())}</text>
        </Show>
        <text fg={palette.borderSubtle}>{block.repeat(emptyW())}</text>
        <text fg={palette.text}>{` ${pct()}%`}</text>
        <text fg={palette.textMuted}>{props.compact ? "" : ` (${props.done}/${props.total})`}</text>
        <Show when={showLegend()}>
          <text fg={palette.success}>{` ✓${props.done}`}</text>
          <text fg={palette.primary}>{` ◆${props.inReview}`}</text>
          <text fg={palette.warning}>{` ●${props.inProgress}`}</text>
          <Show when={(props.blocked || 0) > 0}>
            <text fg={palette.error}>{` ✕${props.blocked}`}</text>
          </Show>
          <text fg={palette.textMuted}>{` ○${waiting()}`}</text>
        </Show>
      </box>
    </box>
  );
}

function ActiveTaskHero(props: {
  task: DashboardTask;
  activeDelegation?: DelegationJob;
  now: () => number;
  spinner: () => string;
  densityCompact: boolean;
  textLimit: number;
  theme?: TuiThemeCurrent;
}) {
  const elapsed = createMemo(() => {
    const updated = Date.parse(props.task.updated_at);
    if (!Number.isFinite(updated)) return "00:00";
    return formatDuration(props.now() - updated);
  });

  const ttlRemaining = createMemo(() => {
    if (!props.task.claim_expires_at) return undefined;
    const exp = Date.parse(props.task.claim_expires_at);
    if (!Number.isFinite(exp)) return undefined;
    const diff = exp - props.now();
    return diff > 0 ? formatDuration(diff) : "expired";
  });
  const palette = resolvePalette(props.theme);

  return (
    <box
      flexDirection="column"
      marginTop={1}
      paddingLeft={1}
      paddingRight={1}
      borderStyle="rounded"
      borderColor={props.activeDelegation ? palette.warning : palette.borderSubtle}
      title={clipped(`${props.spinner()} TASK IN PROGRESS`, props.textLimit)}
      titleColor={palette.warning}
      titleAlignment="left"
      backgroundColor={palette.panel}
    >
      <box flexDirection="row">
        <text fg={palette.sky}>{`${GLYPH.active} `}</text>
        <text fg={palette.text}>{clipped(`${props.task.task_id} · ${props.task.title}`, props.textLimit - 3)}</text>
        <Show when={props.densityCompact}>
          <text
            fg={palette.info}
            onMouseDown={() => openWebConsole(props.task.board_id, props.task.task_id)}
            selectable={false}
          >
            {` [${GLYPH.web}]`}
          </text>
        </Show>
      </box>

      <box flexDirection="row">
        <text fg={palette.warning}>{`T+${elapsed()} `}</text>
        <Show when={ttlRemaining()}>
          {(ttl: () => string) => (
            <text fg={ttl() === "expired" ? palette.error : palette.sky}>
              {`│ ${GLYPH.warn} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`}
            </text>
          )}
        </Show>
      </box>

      <Show when={!props.densityCompact}>
        <box flexDirection="row">
          <Show
            when={props.activeDelegation}
            fallback={
              <text fg={palette.accent}>
                {clipped(`${GLYPH.row} Durable task${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit)}
              </text>
            }
          >
            {(del: () => DelegationJob) => (
              <text fg={palette.info}>
                {`${GLYPH.working} ${del().transport || "direct"}${del().pane_id ? ` · ${del().pane_id}` : ""}${del().attempt ? ` · int #${del().attempt}` : ""}`}
              </text>
            )}
          </Show>
        </box>

        <box
          flexDirection="row"
          marginTop={0}
          onMouseDown={() => openWebConsole(props.task.board_id, props.task.task_id)}
        >
          <text fg={palette.info} selectable={false}>
            {`[${GLYPH.web} View in Web]`}
          </text>
        </box>
      </Show>
    </box>
  );
}

function TaskRows(props: {
  tasks: DashboardTask[];
  spinner: () => string;
  theme?: TuiThemeCurrent;
  compact?: boolean;
  textLimit: number;
}) {
  const palette = resolvePalette(props.theme);
  return (
    <Show when={props.tasks.length > 0} fallback={<EmptyState palette={palette} label="No queued tasks" />}>
      <For each={props.tasks.slice(0, MAX_VISIBLE_ROWS)}>
        {(task) => {
          const chip = taskStatusChip(task.status, palette);
          const isProg = task.status === "in_progress";
          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row" paddingLeft={2}>
                <text fg={chip.color}>{`${isProg ? props.spinner() : chip.icon} `}</text>
                <text fg={chip.color}>{`[${chip.tag}] `}</text>
                <text fg={isProg ? palette.text : palette.textSoft}>
                  {clipped(task.task_id, Math.max(8, props.textLimit - 14))}
                </text>
                <text
                  fg={palette.info}
                  onMouseDown={() => openWebConsole(task.board_id, task.task_id)}
                  selectable={false}
                >
                  {` [${GLYPH.web}]`}
                </text>
              </box>
              <box flexDirection="row" paddingLeft={5}>
                <text fg={palette.textMuted}>
                  {`${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` · ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` · ${GLYPH.warn} ${task.lease_count}lk` : ""}`}
                </text>
              </box>
            </box>
          );
        }}
      </For>
    </Show>
  );
}

function DelegationRows(props: {
  jobs: DelegationJob[];
  spinner: () => string;
  now: () => number;
  theme?: TuiThemeCurrent;
  compact?: boolean;
  textLimit: number;
}) {
  const palette = resolvePalette(props.theme);
  return (
    <Show when={props.jobs.length > 0} fallback={<EmptyState palette={palette} label="No active workers" />}>
      <For each={props.jobs.slice(0, MAX_VISIBLE_ROWS)}>
        {(job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const chip = roleChip(job.role || "", palette);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` +${formatDuration(props.now() - t)}` : "";
          });
          const statusCol = isRunning
            ? palette.warning
            : job.status === "succeeded"
            ? palette.success
            : palette.error;

          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row" paddingLeft={2}>
                <text fg={statusCol}>
                  {`${isRunning ? props.spinner() : job.status === "succeeded" ? GLYPH.done : GLYPH.fail} `}
                </text>
                <text fg={chip.color}>{`[${chip.tag}] `}</text>
                <text fg={isRunning ? palette.text : palette.textSoft}>
                  {clipped(job.role || "worker", Math.max(6, props.textLimit - 12))}
                </text>
                <text fg={statusCol}>{elapsed()}</text>
              </box>
              <box flexDirection="row" paddingLeft={5}>
                <text fg={palette.textMuted}>
                  {clipped(`${shortID(job.job_id)} · ${job.transport || "direct"}${job.pane_id ? ` · ${job.pane_id}` : ""}${job.attempt ? ` · int #${job.attempt}` : ""}`, props.textLimit)}
                </text>
              </box>
            </box>
          );
        }}
      </For>
    </Show>
  );
}

function AttentionRows(props: { items: AttentionItem[]; theme?: TuiThemeCurrent; compact?: boolean; textLimit: number }) {
  const palette = resolvePalette(props.theme);
  return (
    <Show when={props.items.length > 0} fallback={<EmptyState palette={palette} label="No pending alerts" />}>
      <For each={props.items.slice(0, MAX_VISIBLE_ROWS)}>
        {(item) => (
          <box flexDirection="column">
            <box flexDirection="row" paddingLeft={2}>
              <text fg={palette.error}>{`${GLYPH.fail} `}</text>
              <text fg={palette.text}>{clipped(item.title, Math.max(8, props.textLimit - 4))}</text>
            </box>
            <box flexDirection="row" paddingLeft={5}>
              <text fg={palette.textMuted}>{clipped(item.detail, Math.max(8, props.textLimit - 5))}</text>
            </box>
          </box>
        )}
      </For>
    </Show>
  );
}

function OperationalStatusBlock(props: {
  snapshot: UISnapshot;
  jobs: DelegationJob[];
  activeExecutions: number;
  inReview: number;
  attentionCount: number;
  stale: boolean;
  eta: () => Tier1Eta;
  now: () => number;
  spinner: () => string;
  pulse: () => string;
  theme?: TuiThemeCurrent;
  layout: SidebarLayout;
}) {
  const palette = resolvePalette(props.theme);
  const succeededJobs = createMemo(
    () => props.jobs.filter((j) => j.status === "succeeded").length
  );
  const failedJobs = createMemo(
    () => props.jobs.filter((j) => ["failed", "timed_out", "lost", "cancelled"].includes(j.status)).length
  );
  const activeJobs = createMemo(
    () => props.jobs.filter((j) => ["running", "starting", "accepted"].includes(j.status)).length
  );

  const doneTasks = createMemo(() => props.snapshot.summary.done || 0);
  const totalTasks = createMemo(() => props.snapshot.summary.total_tasks || props.snapshot.tasks.length);
  const totalLeases = createMemo(() =>
    props.snapshot.tasks.reduce((sum, t) => sum + (t.lease_count || 0), 0)
  );
  const blockedTasks = createMemo(() => props.snapshot.summary.blocked || 0);

  const metrics = createMemo(() => [
    { label: "act", value: props.activeExecutions },
    { label: "rev", value: props.inReview },
    { label: "done", value: doneTasks() },
    { label: "alrt", value: props.attentionCount },
  ]);

  const hasSignal = createMemo(
    () =>
      props.activeExecutions > 0 ||
      props.inReview > 0 ||
      props.attentionCount > 0 ||
      doneTasks() > 0 ||
      totalTasks() > 0 ||
      totalLeases() > 0 ||
      blockedTasks() > 0 ||
      succeededJobs() > 0 ||
      failedJobs() > 0 ||
      activeJobs() > 0
  );

  const etaLabel = createMemo(() => {
    const eta = props.eta();
    const suffix = eta.backlog > 0 ? ` · ${eta.backlog} backlog` : "";
    if (props.layout.compact) {
      const shortSuffix = eta.backlog > 0 ? ` +${eta.backlog}b` : "";
      if (eta.remaining === 0) return `ETA: ready${shortSuffix}`;
      if (eta.estimateMs === undefined) return `ETA: est… (${eta.remaining})${shortSuffix}`;
      return `ETA: ${formatDuration(eta.estimateMs)} (${eta.remaining})${shortSuffix}`;
    }
    if (eta.remaining === 0) return `ETA: board complete${suffix}`;
    if (eta.estimateMs === undefined) return `ETA: estimating (${eta.remaining} left)${suffix}`;
    return `ETA: ${formatDuration(eta.estimateMs)} · ${eta.remaining} left${suffix}`;
  });

  const etaColor = createMemo(() =>
    props.eta().remaining === 0 ? palette.success : palette.sky
  );

  const successRate = createMemo(() => {
    const closed = succeededJobs() + failedJobs();
    if (closed === 0) return undefined;
    return Math.round((succeededJobs() / closed) * 100);
  });

  const syncAgeSec = createMemo(() => {
    const t = Date.parse(props.snapshot.generated_at);
    if (!Number.isFinite(t)) return 0;
    return Math.max(0, Math.floor((props.now() - t) / 1000));
  });

  const healthBars = createMemo(() => {
    const rate = successRate();
    const filled = rate === undefined ? 0 : Math.round((rate / 100) * props.layout.gaugeWidth);
    const empty = rate === undefined ? 0 : Math.max(0, props.layout.gaugeWidth - filled);
    return {
      filled: "■".repeat(filled),
      empty: "□".repeat(empty),
    };
  });

  const healthColor = createMemo(() => {
    const rate = successRate();
    if (rate === undefined) return palette.textMuted;
    if (rate >= 90) return palette.success;
    if (rate >= 70) return palette.info;
    if (rate >= 50) return palette.warning;
    return palette.error;
  });

  const freshnessLabel = createMemo(() => {
    if (props.stale) {
      return props.layout.compact ? `${GLYPH.warn} stale` : `${GLYPH.warn} stale (+${syncAgeSec()}s)`;
    }
    return props.layout.compact ? `${GLYPH.working} live` : `${GLYPH.working} live · ${syncAgeSec()}s ago`;
  });

  const synapseValue = createMemo(() =>
    activeJobs() > 0 ? `${props.spinner()} active · ${activeJobs()} runs` : `${props.pulse()} synced`
  );

  const authorityValue = createMemo(() => (totalLeases() > 0 ? `sqlite · ${totalLeases()} lk` : "sqlite"));

  const healthCell = () => (
    <box flexDirection="row" gap={1}>
      <text fg={palette.textMuted}>health</text>
      <Show
        when={successRate() !== undefined}
        fallback={<text fg={palette.textMuted}>{`${GLYPH.idle} standby`}</text>}
      >
        <text fg={healthColor()}>{healthBars().filled}</text>
        <text fg={palette.borderSubtle}>{healthBars().empty}</text>
        <text fg={healthColor()}>{`${successRate()}%`}</text>
      </Show>
    </box>
  );

  const signalCells = () => (
    <>
      <StatusCell
        palette={palette}
        label="synapse"
        value={synapseValue}
        color={() => (activeJobs() > 0 ? palette.warning : palette.text)}
      />
      {healthCell()}
      <StatusCell palette={palette} label="authority" value={authorityValue} />
      <StatusCell palette={palette} label="web" value={() => "loopback"} color={() => palette.info} />
    </>
  );

  return (
    <box
      flexDirection="column"
      marginTop={1}
      paddingLeft={1}
      paddingRight={1}
      borderStyle="rounded"
      borderColor={failedJobs() > 0 ? palette.error : props.stale ? palette.borderSubtle : palette.borderActive}
      title={props.layout.compact ? `${GLYPH.brand} CONTROL` : `${GLYPH.brand} CONTROL MATRIX`}
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.element}
    >
      <Show
        when={hasSignal()}
        fallback={
          <text fg={palette.textMuted}>
            {`${GLYPH.idle} standby · 0 active · authority sqlite · ${props.stale ? "stale" : "live"}`}
          </text>
        }
      >
        <box flexDirection="row" gap={2}>
          <For each={metrics()}>
            {(metric) => (
              <box flexDirection="row" gap={1}>
                <text fg={palette.textMuted}>{metric.label}</text>
                <text fg={palette.accent}>{padMetric(metric.value)}</text>
              </box>
            )}
          </For>
        </box>

        <Show
          when={!props.layout.compact}
          fallback={<box flexDirection="column">{signalCells()}</box>}
        >
          <box flexDirection="row" gap={2}>
            <box flexDirection="column" flexGrow={1}>
              <StatusCell
                palette={palette}
                label="synapse"
                value={synapseValue}
                color={() => (activeJobs() > 0 ? palette.warning : palette.text)}
              />
              {healthCell()}
            </box>
            <box flexDirection="column" flexGrow={1}>
              <StatusCell palette={palette} label="authority" value={authorityValue} />
              <StatusCell palette={palette} label="web" value={() => "loopback"} color={() => palette.info} />
            </box>
          </box>
        </Show>

        <box flexDirection="row" gap={2}>
          <text fg={props.stale ? palette.warning : palette.success}>{freshnessLabel()}</text>
          <Show when={totalTasks() > 0}>
            <text fg={etaColor()}>{etaLabel()}</text>
            <Show when={props.eta().sparkline}>
              {(spark: () => string) => <text fg={palette.textMuted}>{spark()}</text>}
            </Show>
          </Show>
        </box>
      </Show>
    </box>
  );
}

export function SidebarStatus(props: {
  nativeActivity: () => NativeActivity | undefined;
  scopeReady: () => boolean;
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  snapshotError: () => string;
  sessionElapsed: () => string | undefined;
  eta: () => Tier1Eta;
  now: () => number;
  spinner: () => string;
  pulse: () => string;
  density: () => DensityMode;
  toggleDensity: () => void;
  tasksExpanded: () => boolean;
  delegationsExpanded: () => boolean;
  attentionExpanded: () => boolean;
  toggleTasks: () => void;
  toggleDelegations: () => void;
  toggleAttention: () => void;
  theme?: TuiThemeCurrent;
}) {
  const [rootWidth, setRootWidth] = createSignal(0);
  const layout = createMemo(() => sidebarLayout(rootWidth()));
  const palette = resolvePalette(props.theme);
  const attention = createMemo(() => attentionItems(props.snapshot(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const stale = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    return Boolean(props.snapshotError()) || !Number.isFinite(generated) || props.now() - generated > SNAPSHOT_STALE_MS;
  });
  const freshness = createMemo<SidebarFreshness>(() => {
    if (!props.scopeReady() || !props.snapshot().generated_at) return "standby";
    return stale() ? "stale" : "live";
  });
  const snapshotFailureLabel = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    if (!Number.isFinite(generated)) return "Snapshot refresh failed";
    return `Snapshot refresh failed · data from ${Math.round((props.now() - generated) / 1000)}s ago`;
  });

  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const activeDelegation = createMemo(() =>
    props.jobs().find((j) => ["running", "starting", "accepted"].includes(j.status))
  );

  const activeExecutionsCount = createMemo(() => {
    const s = props.snapshot().summary;
    if (typeof s.active_executions === "number") return s.active_executions;
    const inProg = s.in_progress || 0;
    const actDel = s.active_delegations || 0;
    return Math.max(inProg, actDel);
  });

  const isExecuting = createMemo(() => props.nativeActivity() === "busy" || Boolean(activeDelegation()));

  const activeDelegationsCount = createMemo(() => {
    if (typeof props.snapshot().summary.active_delegations === "number") {
      return props.snapshot().summary.active_delegations;
    }
    return props.jobs().filter((j) => ["running", "starting", "accepted"].includes(j.status)).length;
  });

  const totalDelegationsCount = createMemo(() => props.snapshot().summary.total_delegations || props.jobs().length);

  const doneTasks = createMemo(() => props.snapshot().summary.done || 0);
  const inReviewTasks = createMemo(() => props.snapshot().summary.in_review || 0);
  const inProgressTasks = createMemo(() => props.snapshot().summary.in_progress || 0);
  const blockedTasks = createMemo(() => props.snapshot().summary.blocked || 0);
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);

  return (
    <box
      flexDirection="column"
      ref={(node: BoxRenderable) => setRootWidth(Math.max(0, node.width || 0))}
      onSizeChange={function (this: BoxRenderable) {
        setRootWidth(Math.max(0, this.width || 0));
      }}
    >
      <CortexCockpitHeader
        isExecuting={isExecuting}
        nativeActivity={props.nativeActivity}
        freshness={freshness}
        projectRoot={props.snapshot().project_root}
        sessionElapsed={props.sessionElapsed}
        density={props.density}
        onToggleDensity={props.toggleDensity}
        textLimit={layout().textLimit}
        spinner={props.spinner}
        theme={props.theme}
      />

      <Show when={!props.scopeReady()}>
        <text fg={palette.warning} marginTop={1}>Conversation unavailable · awaiting metadata</text>
      </Show>

      <Show when={props.scopeReady() && !props.snapshot().generated_at && !props.snapshotError()}>
        <text fg={palette.textMuted} marginTop={1}>Loading conversation state…</text>
      </Show>

      <Show when={props.snapshotError()}>
        <text fg={palette.error} marginTop={1}>{snapshotFailureLabel()}</text>
      </Show>

      <Show when={props.scopeReady() && Boolean(props.snapshot().generated_at)}>
        <OperationalStatusBlock
          snapshot={props.snapshot()}
          jobs={props.jobs()}
          activeExecutions={activeExecutionsCount()}
          inReview={counts().review}
          attentionCount={counts().attention}
          stale={stale()}
          eta={props.eta}
          now={props.now}
          spinner={props.spinner}
          pulse={props.pulse}
          theme={props.theme}
          layout={layout()}
        />

        <Show when={totalTasks() > 0}>
          <MultiColorProgressBar
            done={doneTasks()}
            inReview={inReviewTasks()}
            inProgress={inProgressTasks()}
            blocked={blockedTasks()}
            total={totalTasks()}
            width={layout().gaugeWidth}
            compact={layout().compact}
            textLimit={layout().textLimit}
            theme={props.theme}
          />
        </Show>

        <Show when={activeTask()}>
          {(task: () => DashboardTask) => (
            <ActiveTaskHero
              task={task()}
              activeDelegation={activeDelegation()}
              now={props.now}
              spinner={props.spinner}
              densityCompact={props.density() === "compact"}
              textLimit={layout().textLimit}
              theme={props.theme}
            />
          )}
        </Show>

        <Section
          title="Task Board"
          shortTitle="Tasks"
          badge={totalTasks() > 0 ? `${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot` : undefined}
          shortBadge={totalTasks() > 0 ? `${props.snapshot().summary.active_tasks || 0}/${totalTasks()}` : undefined}
          summary={`${counts().active} act · ${inReviewTasks()} rev · ${totalTasks()} tot`}
          compact={layout().compact}
          densityCompact={props.density() === "compact"}
          expanded={props.tasksExpanded}
          onToggle={props.toggleTasks}
          theme={props.theme}
        >
          <TaskRows tasks={props.snapshot().tasks} spinner={props.spinner} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>

        <Section
          title="Workers & Delegation"
          shortTitle="Workers"
          badge={totalDelegationsCount() > 0 ? `${activeDelegationsCount()} act / ${totalDelegationsCount()} tot` : undefined}
          shortBadge={totalDelegationsCount() > 0 ? `${activeDelegationsCount()}/${totalDelegationsCount()}` : undefined}
          summary={`${activeDelegationsCount()} act · ${totalDelegationsCount()} tot`}
          compact={layout().compact}
          densityCompact={props.density() === "compact"}
          expanded={props.delegationsExpanded}
          onToggle={props.toggleDelegations}
          theme={props.theme}
        >
          <DelegationRows jobs={props.jobs()} spinner={props.spinner} now={props.now} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>

        <Section
          title="Attention Center"
          shortTitle="Alerts"
          badge={counts().attention > 0 ? `${counts().attention} alerts` : undefined}
          shortBadge={counts().attention > 0 ? `${counts().attention}` : undefined}
          summary={counts().attention > 0 ? `${counts().attention} alerts` : "no alerts"}
          compact={layout().compact}
          densityCompact={props.density() === "compact"}
          expanded={props.attentionExpanded}
          onToggle={props.toggleAttention}
          theme={props.theme}
        >
          <AttentionRows items={attention()} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>
      </Show>
    </box>
  );
}

function SidebarFooterMetrics(props: { metrics: () => SidebarMetrics; theme?: TuiThemeCurrent }) {
  const dimensions = useTerminalDimensions();
  const contextPct = createMemo(() => {
    const metrics = props.metrics();
    if (metrics.tokensUsed === undefined || metrics.tokenLimit === undefined) return undefined;
    return Math.min(100, Math.max(0, Math.round((metrics.tokensUsed / metrics.tokenLimit) * 100)));
  });

  // The sidebar strip is narrower than the terminal, so narrow terminals get fewer gauge segments.
  const gaugeSegments = createMemo(() => (dimensions().width < NARROW_TERMINAL_COLS ? 6 : 10));
  const contextGauge = createMemo(() => {
    const pct = contextPct();
    if (pct === undefined) return undefined;
    const segments = gaugeSegments();
    const filled = Math.max(0, Math.min(segments, Math.round((pct / 100) * segments)));
    return { filled: "█".repeat(filled), empty: "░".repeat(segments - filled) };
  });

  const palette = resolvePalette(props.theme);

  const contextColor = createMemo(() => {
    const pct = contextPct();
    if (pct === undefined) return palette.textMuted;
    if (pct >= 85) return palette.error;
    if (pct >= 60) return palette.warning;
    return palette.success;
  });

  const visible = createMemo(() => {
    const metrics = props.metrics();
    return metrics.tokensUsed !== undefined || metrics.cost !== undefined;
  });

  return (
    <Show when={visible()}>
      <box flexDirection="row" paddingLeft={1} paddingRight={1}>
        <Show when={props.metrics().tokensUsed !== undefined}>
          <text fg={palette.border}>{" │ "}</text>
          <Show
            when={contextGauge()}
            fallback={<text fg={palette.sky}>{`◆ ${formatTokens(props.metrics().tokensUsed!)} tok`}</text>}
          >
            {(gauge: () => { filled: string; empty: string }) => (
              <box flexDirection="row">
                <text fg={contextColor()}>{"◆ "}</text>
                <text fg={contextColor()}>{gauge().filled}</text>
                <text fg={palette.border}>{gauge().empty}</text>
                <text fg={contextColor()}>{` ${contextPct()}%`}</text>
                <text fg={palette.border}>{" · "}</text>
                <text fg={palette.sky}>{formatTokens(props.metrics().tokensUsed!)}</text>
              </box>
            )}
          </Show>
        </Show>
        <Show when={props.metrics().cost !== undefined}>
          <text fg={palette.border}>{" │ "}</text>
          <text fg={palette.warning}>{`$ ${formatCost(props.metrics().cost!)}`}</text>
        </Show>
      </box>
    </Show>
  );
}

function HomeBottomStatus(props: {
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  spinner: () => string;
  snapshotError: () => string;
  theme?: TuiThemeCurrent;
}) {
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);
  const palette = resolvePalette(props.theme);

  return (
    <Show when={visible()}>
      <box paddingLeft={1} paddingRight={1} flexDirection="row">
        <text fg={palette.accentAlt}>{`${GLYPH.brand} `}</text>
        <text fg={palette.text}>CORTEX</text>
        <text fg={palette.info}>·</text>
        <text fg={palette.sky}>IA </text>
        <text fg={palette.border}>│ </text>
        <Show
          when={activeTask()}
          fallback={
            <box flexDirection="row">
              <text fg={palette.warning}>{`● ${counts().active} active`}</text>
              <text fg={palette.border}> · </text>
              <text fg={palette.accent}>{`◆ ${counts().review} rev`}</text>
              <Show when={counts().attention > 0}>
                <text fg={palette.border}> · </text>
                <text fg={palette.error}>{`✕ ${counts().attention} alert`}</text>
              </Show>
            </box>
          }
        >
          {(task: () => DashboardTask) => (
            <box
              flexDirection="row"
              onMouseDown={() => openWebConsole(task().board_id, task().task_id)}
            >
              <text fg={palette.warning}>{`[${props.spinner()} ${task().task_id}] `}</text>
              <text fg={palette.text}>{clipped(task().title, 20)}</text>
              <text fg={palette.textMuted}>{` · ${counts().active} active`}</text>
              <text fg={palette.info}>{` [${GLYPH.web}]`}</text>
            </box>
          )}
        </Show>
      </box>
    </Show>
  );
}

type KanbanGroupKind = "in_progress" | "in_review" | "done" | "blocked";

const KANBAN_GROUP_META: Record<
  KanbanGroupKind,
  {
    label: string;
    glyph: string;
    header: keyof CortexPalette;
    id: keyof CortexPalette;
    body: keyof CortexPalette;
  }
> = {
  in_progress: { label: "IN PROGRESS", glyph: GLYPH.working, header: "warning", id: "text", body: "textSoft" },
  in_review: { label: "IN REVIEW", glyph: GLYPH.active, header: "accent", id: "text", body: "textSoft" },
  done: { label: "DONE", glyph: GLYPH.done, header: "success", id: "sky", body: "textMuted" },
  blocked: { label: "BLK", glyph: GLYPH.fail, header: "error", id: "error", body: "error" },
};

const KANBAN_COLUMNS_WIDE: KanbanGroupKind[][] = [["in_progress"], ["in_review"], ["done", "blocked"]];
const KANBAN_COLUMNS_MEDIUM: KanbanGroupKind[][] = [["in_progress", "in_review"], ["done", "blocked"]];
const KANBAN_COLUMNS_STACKED: KanbanGroupKind[][] = [["in_progress", "in_review", "done", "blocked"]];

function KanbanCard(props: { task: DashboardTask; kind: KanbanGroupKind; palette: CortexPalette }) {
  const meta = KANBAN_GROUP_META[props.kind];
  return (
    <box flexDirection="column" marginTop={1}>
      <text fg={props.palette[meta.id]} attributes={TextAttributes.BOLD} wrapMode="none" truncate={true}>
        {`${meta.glyph} ${props.task.task_id}`}
      </text>
      <text fg={props.palette[meta.body]} wrapMode="none" truncate={true}>
        {props.task.title}
      </text>
      <Show when={props.kind === "in_progress" ? props.task.owner : undefined}>
        {(owner: () => string) => (
          <text fg={props.palette.info} wrapMode="none" truncate={true}>{`Claim: ${owner()}`}</text>
        )}
      </Show>
      <Show when={props.kind === "in_review"}>
        <text fg={props.palette.warning} wrapMode="none" truncate={true}>
          awaiting reviewer
        </text>
      </Show>
    </box>
  );
}

function KanbanGroup(props: { kind: KanbanGroupKind; tasks: () => DashboardTask[]; palette: CortexPalette }) {
  const meta = KANBAN_GROUP_META[props.kind];
  return (
    <box flexDirection="column" marginBottom={1}>
      <text fg={props.palette[meta.header]} attributes={TextAttributes.BOLD} wrapMode="none" truncate={true}>
        {`${meta.glyph} ${meta.label} (${props.tasks().length})`}
      </text>
      <For each={props.tasks()}>
        {(task) => <KanbanCard task={task} kind={props.kind} palette={props.palette} />}
      </For>
    </box>
  );
}

function SessionKanbanPanel(props: {
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  now: () => number;
  spinner: () => string;
  pulse: () => string;
  theme?: TuiThemeCurrent;
}) {
  const tasks = createMemo(() => props.snapshot().tasks);
  const palette = resolvePalette(props.theme);
  const dimensions = useTerminalDimensions();

  const grouped = createMemo<Record<KanbanGroupKind, DashboardTask[]>>(() => {
    const all = tasks();
    return {
      in_progress: all.filter((t) => t.status === "in_progress"),
      in_review: all.filter((t) => t.status === "in_review"),
      done: all.filter((t) => t.status === "ready").slice(0, KANBAN_READY_LIMIT),
      blocked: all.filter((t) => t.status === "blocked"),
    };
  });

  const columns = createMemo(() => {
    const width = measuredColumns(dimensions().width);
    if (width >= NARROW_TERMINAL_COLS) return KANBAN_COLUMNS_WIDE;
    if (width >= KANBAN_TWO_COLUMN_COLS) return KANBAN_COLUMNS_MEDIUM;
    return KANBAN_COLUMNS_STACKED;
  });

  const columnWidth = createMemo(() => {
    const count = columns().length;
    const available = Math.max(0, measuredColumns(dimensions().width) - 2);
    const usable = available - (count - 1) * KANBAN_COLUMN_SEPARATOR;
    return Math.max(KANBAN_MIN_COLUMN_WIDTH, Math.floor(usable / count));
  });

  return (
    <box flexDirection="column" padding={1}>
      <box flexDirection="row" marginBottom={1}>
        <text fg={palette.info} attributes={TextAttributes.BOLD} wrapMode="none" truncate={true}>
          {`${GLYPH.web} CORTEX · IA KANBAN DECK [${props.pulse()}] `}
        </text>
        <text fg={palette.textMuted}>
          {`(${tasks().length} tasks · ${props.jobs().length} workers)`}
        </text>
      </box>

      <box flexDirection="row">
        <For each={columns()}>
          {(column, index) => (
            <box
              flexDirection="column"
              width={columnWidth()}
              border={index() > 0 ? KANBAN_COLUMN_BORDER : false}
              borderColor={palette.border}
            >
              <For each={column}>
                {(kind) => <KanbanGroup kind={kind} tasks={() => grouped()[kind]} palette={palette} />}
              </For>
            </box>
          )}
        </For>
      </box>
    </box>
  );
}

const CORTEX_LOGO_BRAILLE = [
  "       ⣠⣶⣿⣿⣿⣿⣶⣤⡀       ⢀⣤⣶⣿⣿⣿⣿⣶⣄",
  "    ⢰⣿⣿⠟⠉   ⠙⢿⣿⣷⡀   ⢠⣾⣿⡿⠋   ⠈⠻⣿⣿⡆",
  "   ⢠⣿⣿⠋  ⢀⣤⣤⣀  ⠹⣿⣿⣄⣠⣿⣿⠏  ⣀⣤⣤⡀  ⠙⣿⣿⡄",
  "   ⣾⣿⠃  ⢰⣿⣿⣿⣿⣷⡀ ⠹⣿⣿⣿⣿⠏ ⢠⣾⣿⣿⣿⣿⡆  ⠘⣿⣷",
  "  ⢸⣿⡟   ⠸⣿⣿⣿⣿⣿⣿⣆ ⠹⣿⣿⠏ ⣰⣿⣿⣿⣿⣿⣿⠇   ⢻⣿⡇",
  "  ⠘⣿⣧    ⠈⠛⠿⣿⣿⣿⣿⣷⣄⠙⠋⣠⣾⣿⣿⣿⣿⠿⠛⠁    ⣼⣿⠃",
  "   ⠹⣿⣧⡀     ⠈⠙⠿⣿⣿⣿⡆⢰⣿⣿⣿⠿⠋⠁     ⢀⣼⣿⠏",
  "    ⠙⢿⣿⣦⡀   ⢀⣠⣴⣿⣿⣿⡇⢸⣿⣿⣿⣦⣄⡀   ⢀⣴⣿⡿⠋",
  "      ⠉⠛⠿⣿⣿⣿⣿⣿⣿⡿⠛⠁ ⠈⠛⢿⣿⣿⣿⣿⣿⣿⠿⠛⠉",
  "  ██████╗ ██████╗ ██████╗ ████████╗███████╗██╗  ██╗     ██╗ █████╗ ",
  " ██╔════╝██╔═══██╗██╔══██╗╚══██╔══╝██╔════╝╚██╗██╔╝     ██║██╔══██╗",
  " ██║     ██║   ██║██████╔╝   ██║   █████╗   ╚███╔╝█████╗██║███████║",
  " ██║     ██║   ██║██╔══██╗   ██║   ██╔══╝   ██╔██╗╚════╝██║██╔══██║",
  " ╚██████╗╚██████╔╝██║  ██║   ██║   ███████╗██╔╝ ██╗     ██║██║  ██║",
  "  ╚═════╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝╚═╝  ╚═╝     ╚═╝╚═╝  ╚═╝",
];

// Top-to-bottom banner gradient: the braille brain shifts across the accent pair
// while the ASCII shadow resolves into primary/sky, keeping one purple-to-blue
// family that stays legible under both dark and light resolved palettes.
const CORTEX_LOGO_GRADIENT = ["accent", "accentAlt", "primary", "sky"] as const;
const CORTEX_LOGO_GRADIENT_STOPS = [4, 9, 12] as const;

function logoLineColor(index: number, palette: CortexPalette): ColorInput {
  const stop = CORTEX_LOGO_GRADIENT_STOPS.findIndex((limit) => index < limit);
  const step = stop === -1 ? CORTEX_LOGO_GRADIENT_STOPS.length : stop;
  return palette[CORTEX_LOGO_GRADIENT[step]];
}

function HomeLogo(props: { theme?: TuiThemeCurrent }) {
  const dim = useTerminalDimensions();
  const palette = resolvePalette(props.theme);
  const isLarge = createMemo(() => {
    const d = dim();
    return d.height >= CORTEX_LOGO_BRAILLE.length + 5 && d.width >= 72;
  });

  return (
    <box flexDirection="column" alignItems="center" marginBottom={1}>
      <Show
        when={isLarge()}
        fallback={
          <box flexDirection="column" alignItems="center">
            <text fg={palette.info} attributes={TextAttributes.BOLD}>
              {`${GLYPH.active} CORTEX · IA ${GLYPH.active}`}
            </text>
            <text fg={palette.textMuted}>
              {"[Adaptive Cognitive Control Plane]"}
            </text>
          </box>
        }
      >
        <For each={CORTEX_LOGO_BRAILLE}>
          {(line, index) => {
            return <text fg={logoLineColor(index(), palette)}>{line}</text>;
          }}
        </For>
        <text fg={palette.textMuted} marginTop={1}>
          {`${GLYPH.active} OpenCode Multi-Agent Control Plane & Task DAG ${GLYPH.active}`}
        </text>
      </Show>
    </box>
  );
}

interface UsageStatsSummary {
  sessions: number;
  messages: number;
  tokens: number;
  active_days: number;
  peak_hour: number;
  peak_hour_messages: number;
  favorite_model: string;
  favorite_model_share: number;
  first_day: string;
  last_day: string;
  top_models?: Array<{ ModelID: string; Sessions: number; Tokens: number; SharePct: number }>;
}

function formatLargeTokens(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(Math.round(value));
}

function formatInteger(n: number): string {
  return String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function openStatsView(): void {
  try {
    if (process.platform === "win32") {
      spawn("cmd.exe", ["/c", "start", "Cortex Stats", cortexExecutable(), "stats"], {
        detached: true,
        stdio: "ignore",
      }).unref();
    } else {
      spawn(cortexExecutable(), ["stats"], {
        detached: true,
        stdio: "ignore",
      }).unref();
    }
  } catch {}
}

function HomeStatsWidget(props: {
  stats: () => UsageStatsSummary | null;
  loading: () => boolean;
  theme?: TuiThemeCurrent;
}) {
  const palette = resolvePalette(props.theme);
  const data = props.stats;
  const dim = useTerminalDimensions();

  return (
    <Show when={data()}>
      {(s) => {
        const isNarrow = () => dim().width < 80;
        return (
          <box
            flexDirection="column"
            borderStyle="rounded"
            borderColor={palette.border}
            paddingLeft={1}
            paddingRight={1}
            marginTop={1}
            marginBottom={1}
            width="100%"
            onMouseDown={() => openStatsView()}
          >
            {/* Header row */}
            <box flexDirection="row" justifyContent="space-between">
              <box flexDirection="row">
                <text fg={palette.accent} attributes={TextAttributes.BOLD}>
                  {`${GLYPH.brand} CORTEX · IA `}
                </text>
                <text fg={palette.text} attributes={TextAttributes.BOLD}>
                  {"Estadísticas de Uso"}
                </text>
              </box>
              <Show when={s().first_day && s().last_day}>
                <text fg={palette.textMuted}>
                  {`${s().first_day} → ${s().last_day}`}
                </text>
              </Show>
            </box>

            {/* Metrics Row 1 */}
            <box flexDirection="row" marginTop={0} gap={isNarrow() ? 1 : 2}>
              <text fg={palette.sky}>
                {`● ${formatInteger(s().sessions)} sesiones`}
              </text>
              <text fg={palette.border}>{"│"}</text>
              <text fg={palette.info}>
                {`✉ ${formatInteger(s().messages)} mensajes`}
              </text>
              <text fg={palette.border}>{"│"}</text>
              <text fg={palette.success} attributes={TextAttributes.BOLD}>
                {`◆ ${formatLargeTokens(s().tokens)} tokens`}
              </text>
              <Show when={!isNarrow()}>
                <text fg={palette.border}>{"│"}</text>
                <text fg={palette.warning}>
                  {`📅 ${s().active_days} días activos`}
                </text>
              </Show>
            </box>

            {/* Metrics Row 2 */}
            <box flexDirection="row" marginTop={0} gap={isNarrow() ? 1 : 2}>
              <text fg={palette.accentAlt}>
                {`★ Top: ${s().favorite_model || "-"} (${(s().favorite_model_share ?? 0).toFixed(1)}%)`}
              </text>
              <text fg={palette.border}>{"│"}</text>
              <text fg={palette.sky}>
                {`⚡ Pico: ${String(s().peak_hour).padStart(2, "0")}:00`}
              </text>
              <Show when={isNarrow()}>
                <text fg={palette.border}>{"│"}</text>
                <text fg={palette.warning}>
                  {`📅 ${s().active_days}d`}
                </text>
              </Show>
              <text fg={palette.textMuted}>
                {" · [:cortex-stats para panel interactivo]"}
              </text>
            </box>
          </box>
        );
      }}
    </Show>
  );
}

type NanMetricsModelUsage = { model?: string; inputTokens?: number; outputTokens?: number };
type NanMetricsWindow = { byModel?: NanMetricsModelUsage[]; totalTokens?: number };
type NanMetricsDaily = { date?: string; model?: string; inputTokens?: number; outputTokens?: number };
type NanMetrics = { monthToDate?: NanMetricsWindow; last24h?: NanMetricsWindow; timeSeries?: NanMetricsDaily[] };
type NanCatalogMeta = { model?: string; tier?: string; monthlyQuotaTokens?: number; rolling4hRefTokens?: number; effortVocabulary?: string[]; effortAdjustable?: boolean; effortMode?: string; pickerEligible?: boolean; preferredOver?: string };
type NanCatalogEntry = { provider?: string; model?: string; meta?: NanCatalogMeta };
// Quota 0 means unknown, never unlimited: callers suppress the percent rather than divide by a synthetic baseline.
type NanCatalogModel = { model: string; tier: string; quota: number; rolling4hRef: number; vocabulary: string[]; effortMode: string; effortAdjustable: boolean; preferredOver: string };
type NanCatalogCache = { cachedAt: number; index: Map<string, number>; models: NanCatalogModel[] };
type NanUsageView = { model: string; monthToDate: number; last24h: number };
type NanStripView = { percent?: number; burn: number };
type ModelAgentOption = { agent: string; value: string };
type NanDailyPoint = { date: string; tokens: number };
type NanModelUsageRow = { model: string; monthToDate: number; quota: number; rolling4hRef: number };
type PickerPhase = "idle" | "previewing" | "ready" | "applying" | "applied";

// Metrics are untrusted JSON: a missing model row is a real zero, not a failure.
function nanModelTokens(window: NanMetricsWindow | undefined, model: string): number {
  if (!window || !Array.isArray(window.byModel)) return 0;
  for (const entry of window.byModel) {
    if (entry?.model !== model) continue;
    const input = typeof entry.inputTokens === "number" ? entry.inputTokens : 0;
    const output = typeof entry.outputTokens === "number" ? entry.outputTokens : 0;
    return input + output;
  }
  return 0;
}

// Daily totals collapse the per-model timeSeries into one burn figure per date. The
// series is daily granularity by contract, so it can never reconstruct the rolling-4h
// allowance, which the detail panel states as static reference text.
function nanDailySeries(timeSeries: NanMetricsDaily[] | undefined, limit: number): NanDailyPoint[] {
  if (!Array.isArray(timeSeries)) return [];
  const totals = new Map<string, number>();
  for (const point of timeSeries) {
    if (typeof point?.date !== "string" || point.date === "") continue;
    const input = typeof point.inputTokens === "number" ? point.inputTokens : 0;
    const output = typeof point.outputTokens === "number" ? point.outputTokens : 0;
    totals.set(point.date, (totals.get(point.date) ?? 0) + input + output);
  }
  return [...totals.entries()].sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0)).slice(-limit).map(([date, tokens]) => ({ date, tokens }));
}

// Absolute totals share one zero baseline, so a quiet day reads as a low column
// instead of being amplified by a min..max window.
function burnSparkline(values: readonly number[], points: number): string {
  const window = values.slice(-points);
  const peak = Math.max(0, ...window);
  const top = ETA_SPARK_LEVELS.length - 1;
  return window.map((value) => ETA_SPARK_LEVELS[peak > 0 ? Math.max(0, Math.min(top, Math.round((value / peak) * top))) : 0]).join("");
}

// Levels that skip the reasoning pass must not read like an ordinary depth.
const NAN_REASONING_OFF_LEVELS = ["none", "minimal"];

function nanCatalogModel(entry: { model: string; meta: NanCatalogMeta }): NanCatalogModel {
  const meta = entry.meta;
  const quota = typeof meta.monthlyQuotaTokens === "number" && Number.isFinite(meta.monthlyQuotaTokens) ? meta.monthlyQuotaTokens : 0;
  return {
    model: entry.model, tier: typeof meta.tier === "string" ? meta.tier : "", quota,
    rolling4hRef: typeof meta.rolling4hRefTokens === "number" ? meta.rolling4hRefTokens : 0,
    vocabulary: Array.isArray(meta.effortVocabulary) ? meta.effortVocabulary.filter((level) => typeof level === "string") : [],
    effortMode: typeof meta.effortMode === "string" ? meta.effortMode : "", effortAdjustable: meta.effortAdjustable === true,
    preferredOver: typeof meta.preferredOver === "string" ? meta.preferredOver : "",
  };
}

function agentModelOption(entry: { agent?: string; model?: string; variant?: string }): ModelAgentOption | undefined {
  if (typeof entry?.agent !== "string" || entry.agent === "") return undefined;
  const model = typeof entry.model === "string" && entry.model !== "" ? entry.model : "";
  const variant = typeof entry.variant === "string" && entry.variant !== "" ? `#${entry.variant}` : "";
  return { agent: entry.agent, value: model ? `${model}${variant}` : "(unset)" };
}

// The posture is what the API does with a requested level, so a non-adjustable
// model says so instead of offering depth it would ignore.
function nanEffortModeBadge(mode: string): string {
  if (mode === "adaptive") return `${GLYPH.active} adaptive · depth auto`;
  if (mode === "accepted-not-adjustable") return `${GLYPH.warn} accepted · depth fixed`;
  if (mode === "adjustable") return `${GLYPH.done} adjustable`;
  return `${GLYPH.idle} posture unknown`;
}

// Only identifiers from the machine receipt reach the panel; undecodable stdout
// degrades to an empty projection instead of being surfaced verbatim.
function nanSetReceiptLines(stdout: string): string[] {
  try {
    const receipt = JSON.parse(stdout) as { agent?: string; action?: string; dry_run?: boolean; previous?: string; value?: string; changed?: boolean; managed?: boolean; restored?: boolean; warnings?: string[] };
    const lines = [
      `agent    ${receipt.agent || "-"}`, `action   ${receipt.action || "-"}${receipt.dry_run ? " (dry-run)" : ""}`,
      `previous ${receipt.previous || "(unset)"}`, `value    ${receipt.value || "(unset)"}`,
      `changed  ${receipt.changed === true}`, `managed  ${receipt.managed === true}`,
    ];
    if (receipt.restored === true) lines.push("restored true");
    if (Array.isArray(receipt.warnings)) for (const warning of receipt.warnings) lines.push(`warning  ${warning}`);
    return lines;
  } catch {
    return [];
  }
}

function nanCommandFailure(error: Error, stderr: string): string {
  const text = (stderr || error.message || "").trim();
  return text ? text.split(/\r?\n/)[0] : "command failed";
}

function NanUsageStrip(props: {
  view: () => NanStripView | undefined;
  textLimit: number;
  theme?: TuiThemeCurrent;
}) {
  const palette = resolvePalette(props.theme);

  return (
    <Show when={props.view()}>
      {(view: () => NanStripView) => (
        <box flexDirection="row" paddingLeft={1} paddingRight={1}>
          <text fg={palette.accentAlt}>{`${GLYPH.active} nan `}</text>
          <Show
            when={view().percent !== undefined}
            fallback={<text fg={palette.textMuted}>{"quota n/a "}</text>}
          >
            <MultiColorProgressBar
              done={view().percent ?? 0}
              inReview={0}
              inProgress={0}
              total={100}
              width={10}
              compact
              textLimit={props.textLimit}
              label="MTD: "
              theme={props.theme}
            />
          </Show>
          <text fg={palette.border}>{" · "}</text>
          <text fg={palette.textMuted}>{"24h "}</text>
          <text fg={palette.sky}>{formatLargeTokens(view().burn)}</text>
        </box>
      )}
    </Show>
  );
}

type NanModelPickerProps = { models: () => NanCatalogModel[]; agents: () => ModelAgentOption[] | undefined; loading: () => boolean; loadError: () => string; phase: () => PickerPhase; preview: () => string; result: () => string; runError: () => string; onSelectionChange: () => void; onPreview: (agent: string, model: string, level: string) => void; onApply: (agent: string, model: string, level: string) => void; theme?: TuiThemeCurrent };

function NanModelPickerPanel(props: NanModelPickerProps) {
  const palette = resolvePalette(props.theme);
  const [modelID, setModelID] = createSignal("");
  const [level, setLevel] = createSignal("");
  const [agentID, setAgentID] = createSignal("");
  const agents = () => props.agents() ?? [];
  const activeModel = createMemo(() => props.models().find((m) => m.model === modelID()) || props.models()[0]);
  const activeAgent = createMemo(() => agents().find((a) => a.agent === agentID()) || agents()[0]);
  const needsEffort = createMemo(() => Boolean(activeModel()?.effortAdjustable && (activeModel()?.vocabulary.length ?? 0) > 0));
  const ready = createMemo(() => Boolean(activeModel() && activeAgent() && (!needsEffort() || level() !== "")));
  const dispatch = (apply: boolean) => {
    const model = activeModel();
    const agent = activeAgent();
    if (!ready() || !model || !agent) return;
    const chosen = needsEffort() ? level() : "";
    if (apply) props.onApply(agent.agent, model.model, chosen);
    else props.onPreview(agent.agent, model.model, chosen);
  };

  return (
    <box flexDirection="column" padding={1}>
      <text fg={palette.accent} attributes={TextAttributes.BOLD}>{`${GLYPH.web} nan model picker · browse only, preview then apply`}</text>
      <text fg={palette.textMuted}>{"pick a model, an effort, and an agent · the ref form is nan/<model>"}</text>
      <Show when={props.loadError() !== ""}>
        <text fg={palette.error}>{`catalog unavailable (${props.loadError()}) · no assignment attempted`}</text>
      </Show>
      <Show when={props.loadError() === "" && props.loading()}>
        <text fg={palette.textMuted}>{"reading nan catalog…"}</text>
      </Show>
      <Show when={props.loadError() === "" && !props.loading() && props.models().length === 0}>
        <text fg={palette.textMuted}>{"no picker-eligible nan chat models in the catalog"}</text>
      </Show>

      <Show when={props.loadError() === "" && props.models().length > 0}>
        <text fg={palette.textSoft} attributes={TextAttributes.BOLD}>{"model"}</text>
        <For each={props.models()}>
          {(model) => (
            <box flexDirection="row" onMouseDown={() => { setModelID(model.model); setLevel(""); props.onSelectionChange(); }}>
              <text fg={palette.border}>{activeModel()?.model === model.model ? "▸ " : "  "}</text>
              <text fg={model.tier === "legacy" ? palette.textMuted : palette.text}>{`nan/${model.model}`}</text>
              <text fg={palette.textMuted}>{` · ${nanEffortModeBadge(model.effortMode)}`}</text>
              <Show when={model.preferredOver !== ""}>
                <text fg={palette.success}>{` · preferred over ${model.preferredOver}`}</text>
              </Show>
              <Show when={model.tier === "legacy"}>
                <text fg={palette.warning}>{" · legacy · deprioritized"}</text>
              </Show>
            </box>
          )}
        </For>

        <text fg={palette.textSoft} attributes={TextAttributes.BOLD}>{"effort"}</text>
        <Show
          when={needsEffort()}
          fallback={<text fg={palette.textMuted}>{`no adjustable depth for nan/${activeModel()?.model ?? ""} · the reference is written without --effort`}</text>}
        >
          <box flexDirection="row">
            <For each={activeModel()?.vocabulary ?? []}>
              {(option) => (
                <text fg={level() === option ? palette.success : palette.text} onMouseDown={() => { setLevel(option); props.onSelectionChange(); }}>
                  {`[${level() === option ? "▸" : " "} ${option}${NAN_REASONING_OFF_LEVELS.includes(option) ? " ·skips reasoning" : ""}] `}
                </text>
              )}
            </For>
          </box>
        </Show>

        <text fg={palette.textSoft} attributes={TextAttributes.BOLD}>{`agent · effective mapping (${agents().length})`}</text>
        <For each={agents()}>
          {(agent) => (
            <text fg={activeAgent()?.agent === agent.agent ? palette.info : palette.textMuted} onMouseDown={() => { setAgentID(agent.agent); props.onSelectionChange(); }}>
              {`[${activeAgent()?.agent === agent.agent ? "▸" : " "} ${agent.agent}: ${agent.value}] `}
            </text>
          )}
        </For>
        <Show when={!props.loading() && agents().length === 0}>
          <text fg={palette.warning}>{"agent list unavailable · confirmation is disabled"}</text>
        </Show>

        <box flexDirection="row" marginTop={1}>
          <text fg={ready() ? palette.info : palette.textMuted} onMouseDown={() => dispatch(false)}>{"[ preview dry-run ]"}</text>
          <Show when={props.preview() !== ""}>
            <text fg={palette.success} onMouseDown={() => dispatch(true)}>{"  [ apply ]"}</text>
          </Show>
          <text fg={palette.textMuted}>
            {props.phase() === "previewing" ? "  running dry-run…" : props.phase() === "applying" ? "  applying…" : ""}
          </text>
        </box>
        <Show when={props.runError() !== ""}>
          <text fg={palette.error}>{`model set failed · ${props.runError()} — the effective mapping above is unchanged`}</text>
        </Show>
        <Show when={props.preview() !== ""}>
          <text fg={palette.textSoft} attributes={TextAttributes.BOLD}>{"dry-run receipt (nothing written)"}</text>
          <text fg={palette.textMuted}>{props.preview()}</text>
        </Show>
        <Show when={props.result() !== ""}>
          <text fg={palette.success} attributes={TextAttributes.BOLD}>{"applied receipt"}</text>
          <text fg={palette.text}>{props.result()}</text>
        </Show>
      </Show>
    </box>
  );
}

type NanDetailProps = { days: () => NanDailyPoint[]; rows: () => NanModelUsageRow[]; loading: () => boolean; loadError: () => string; theme?: TuiThemeCurrent };

function NanDetailPanel(props: NanDetailProps) {
  const palette = resolvePalette(props.theme);
  const dim = useTerminalDimensions();
  const sparkline = createMemo(() => burnSparkline(props.days().map((point) => point.tokens), NAN_DETAIL_DAY_POINTS));
  const rollingRefs = createMemo(() => props.rows().filter((row) => row.rolling4hRef > 0));
  const textLimit = createMemo(() => Math.max(24, measuredColumns(dim().width) - 8));
  const days = () => props.days();

  return (
    <box flexDirection="column" padding={1}>
      <text fg={palette.info} attributes={TextAttributes.BOLD}>{`${GLYPH.active} nan usage detail · daily granularity`}</text>
      <Show when={props.loadError() !== ""}>
        <text fg={palette.error}>{props.loadError()}</text>
      </Show>
      <Show when={props.loadError() === "" && props.loading()}>
        <text fg={palette.textMuted}>{"reading nan metrics…"}</text>
      </Show>
      <Show when={days().length > 0}>
        <box flexDirection="row" marginTop={1}>
          <text fg={palette.textSoft}>{"daily tokens "}</text>
          <text fg={palette.accentAlt}>{sparkline()}</text>
          <text fg={palette.textMuted}>{`  ${days()[0].date} → ${days()[days().length - 1].date}`}</text>
        </box>
      </Show>
      <Show when={props.rows().length > 0}>
        <text fg={palette.textSoft} attributes={TextAttributes.BOLD} marginTop={1}>{"month-to-date against monthly quota"}</text>
        <For each={props.rows()}>
          {(row) => (
            <box flexDirection="row">
              <text fg={palette.text}>{`${(row.model + "                    ").slice(0, 20)}`}</text>
              <text fg={palette.sky}>{`${formatLargeTokens(row.monthToDate).padStart(8, " ")}`}</text>
              <Show when={row.quota > 0} fallback={<text fg={palette.textMuted}>{"  quota unknown"}</text>}>
                <text fg={palette.textMuted}>{` / ${formatLargeTokens(row.quota)} `}</text>
                <MultiColorProgressBar done={Math.round((row.monthToDate / row.quota) * 100)} inReview={0} inProgress={0} total={100} width={12} compact textLimit={textLimit()} theme={props.theme} />
              </Show>
            </box>
          )}
        </For>
      </Show>
      <For each={rollingRefs()}>
        {(row) => <text fg={palette.warning} marginTop={1}>{`reference only: ${row.model} rolling-4h allowance ${formatLargeTokens(row.rolling4hRef)} tokens — daily-granularity data cannot express a rolling window`}</text>}
      </For>
      <Show when={props.loadError() === "" && !props.loading() && days().length === 0 && props.rows().length === 0}>
        <text fg={palette.textMuted}>{"no nan usage data available"}</text>
      </Show>
    </box>
  );
}

function CortexAgentsRow(props: {
  row: SubagentRow;
  active: boolean;
  now: () => number;
  pulse: () => string;
  layout: () => AgentsLayout;
  palette: CortexPalette;
  onActivate: () => void;
}) {
  const row = props.row;
  const palette = props.palette;
  const statusCore = () => {
    if (row.retryAttempt) return `${GLYPH.warn}${row.retryAttempt}`;
    if (row.status === "busy") return props.pulse();
    if (row.status === "idle") return GLYPH.idle;
    if (row.status === "unknown") return GLYPH.active;
    return GLYPH.working;
  };
  const activity = () => row.tool || row.step || (row.status === "busy" ? "working" : "");
  const elapsed = () => (row.startedAt ? formatElapsedClock(props.now() - row.startedAt) : "--:--");
  const taskBadge = createMemo(() => (row.taskID ? `[${shortID(row.taskID)}·${row.taskStatus || "unknown"}]` : ""));

  return (
    <box
      flexDirection="row"
      backgroundColor={props.active ? palette.element : palette.background}
      onMouseDown={props.onActivate}
    >
      <text fg={props.active ? palette.accent : palette.textMuted} selectable={false}>
        {`${props.active ? GLYPH.row : " "} `}
      </text>
      <text fg={subagentStatusColor(row.status, palette)} selectable={false}>
        {`${fitWidth(statusCore(), AGENTS_STATUS_WIDTH)} `}
      </text>
      <text fg={palette.accentAlt} selectable={false}>
        {`${fitWidth(row.agent || "agent", props.layout().agent)} `}
      </text>
      <text fg={props.active ? palette.text : palette.textSoft}>{fitWidth(row.title, props.layout().title)}</text>
      <text fg={palette.textSoft} selectable={false}>
        {` ${fitWidth(elapsed(), AGENTS_ELAPSED_WIDTH)}`}
      </text>
      <Show when={props.layout().activity > 0}>
        <text fg={palette.info} selectable={false}>
          {` ${fitWidth(activity(), props.layout().activity)}`}
        </text>
      </Show>
      <text fg={palette.sky} selectable={false}>
        {` ${fitWidth(row.tokens === undefined ? "-" : formatTokens(row.tokens), AGENTS_TOKENS_WIDTH)}`}
      </text>
      <text fg={palette.warning} selectable={false}>
        {` ${fitWidth(row.cost === undefined ? "-" : `$${formatCost(row.cost)}`, AGENTS_COST_WIDTH)}`}
      </text>
      <Show when={props.layout().task > 0 && taskBadge() !== ""}>
        <text fg={taskStatusChip(row.taskStatus || "backlog", palette).color} selectable={false}>
          {` ${fitWidth(taskBadge(), props.layout().task)}`}
        </text>
      </Show>
    </box>
  );
}

function CortexAgentsPanel(props: {
  rows: () => SubagentRow[];
  selected: () => number;
  loading: () => boolean;
  error: () => string;
  now: () => number;
  pulse: () => string;
  onActivate: (index: number) => void;
  theme?: TuiThemeCurrent;
}) {
  const palette = resolvePalette(props.theme);
  const dim = useTerminalDimensions();

  const layout = createMemo<AgentsLayout>(() => {
    const measured = Math.floor(Number(dim().width) || 0) || measuredColumns();
    const total = Math.max(AGENTS_MIN_COLUMNS, measured);
    const narrow = total < AGENTS_NARROW_COLUMNS;
    const agent = narrow ? AGENTS_NARROW_AGENT_WIDTH : AGENTS_AGENT_WIDTH;
    const activity = narrow ? 0 : AGENTS_ACTIVITY_WIDTH;
    const task = narrow ? 0 : AGENTS_TASK_WIDTH;
    const fixed =
      AGENTS_STATUS_WIDTH +
      1 +
      agent +
      1 +
      AGENTS_ELAPSED_WIDTH +
      1 +
      AGENTS_TOKENS_WIDTH +
      1 +
      AGENTS_COST_WIDTH +
      (activity > 0 ? activity + 1 : 0) +
      (task > 0 ? task + 1 : 0);
    return { agent, activity, task, title: Math.max(AGENTS_TITLE_MIN, total - fixed - 6) };
  });

  const running = createMemo(() => props.rows().filter((row) => row.status === "busy" || row.status === "retry").length);

  const header = () => (
    <box flexDirection="row">
      <text fg={palette.textMuted} selectable={false}>{`${fitWidth("", 1)} `}</text>
      <text fg={palette.textMuted} selectable={false}>{`${fitWidth("st", AGENTS_STATUS_WIDTH)} `}</text>
      <text fg={palette.textMuted} selectable={false}>{`${fitWidth("agent", layout().agent)} `}</text>
      <text fg={palette.textMuted} selectable={false}>{fitWidth("objective", layout().title)}</text>
      <text fg={palette.textMuted} selectable={false}>{` ${fitWidth("elapsed", AGENTS_ELAPSED_WIDTH)}`}</text>
      <Show when={layout().activity > 0}>
        <text fg={palette.textMuted} selectable={false}>{` ${fitWidth("activity", layout().activity)}`}</text>
      </Show>
      <text fg={palette.textMuted} selectable={false}>{` ${fitWidth("tokens", AGENTS_TOKENS_WIDTH)}`}</text>
      <text fg={palette.textMuted} selectable={false}>{` ${fitWidth("cost", AGENTS_COST_WIDTH)}`}</text>
      <Show when={layout().task > 0}>
        <text fg={palette.textMuted} selectable={false}>{` ${fitWidth("task", layout().task)}`}</text>
      </Show>
    </box>
  );

  return (
    <box
      flexDirection="column"
      borderStyle="rounded"
      borderColor={palette.primary}
      title={`${GLYPH.brand} SUBAGENTS [${running()} running · ${props.rows().length} total]`}
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.background}
      paddingLeft={1}
      paddingRight={1}
    >
      {header()}
      <Show when={props.error() !== ""}>
        <text fg={palette.warning}>{clipped(props.error(), 72)}</text>
      </Show>
      <Show
        when={props.rows().length > 0}
        fallback={
          <EmptyState
            palette={palette}
            label={props.loading() ? "loading subagent sessions" : "no subagent sessions"}
          />
        }
      >
        <For each={props.rows()}>
          {(row, index) => (
            <CortexAgentsRow
              row={row}
              active={props.selected() === index()}
              now={props.now}
              pulse={props.pulse}
              layout={layout}
              palette={palette}
              onActivate={() => props.onActivate(index())}
            />
          )}
        </For>
      </Show>
      <box flexDirection="row" marginTop={1}>
        <text fg={palette.textMuted} selectable={false}>{"open enter · close esc · parent up"}</text>
      </box>
    </box>
  );
}

function initialize(api: any, disposeRoot: () => void): () => void {
  const [activeSessionOverride, setActiveSessionOverride] = createSignal<string | undefined>();
  const updateActiveSession = (val: any) => {
    const id = extractSessionID(val);
    if (id && id !== activeSessionOverride()) {
      setActiveSessionOverride(id);
    }
  };

  const nativeActivity = createMemo(() => nativeSessionActivity(api, activeSessionOverride()));
  const scopeReady = createMemo(() => Boolean(conversationScope(api, activeSessionOverride())?.project));
  const [snapshot, setSnapshot] = createSignal<UISnapshot>(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  // A single failed poll is normal under concurrent writers, so the panel keeps
  // the last good snapshot and only reports an error once failures persist.
  let consecutiveSnapshotFailures = 0;
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [pulseFrame, setPulseFrame] = createSignal(0);

  const getPref = (key: string, fallback: boolean): boolean => {
    if (api.kv && typeof api.kv.get === "function") {
      return api.kv.get(key, fallback) !== false;
    }
    return fallback;
  };

  const setPref = (key: string, value: unknown): void => {
    if (api.kv && typeof api.kv.set === "function") {
      api.kv.set(key, value);
    }
  };

  const showToast = (title: string, message: string, variant: string): void => {
    const toastApi = (api as any).ui?.toast || (api as any).toast;
    if (!toastApi) return;
    try {
      if (typeof toastApi.show === "function") {
        toastApi.show({ title, message, variant });
      } else if (typeof toastApi === "function") {
        toastApi({ title, message, variant });
      }
    } catch {}
  };

  const terminalColumns = (): number => {
    const width = api.renderer?.width;
    if (typeof width === "number" && Number.isFinite(width) && width > 0) return width;
    const columns = (process.stdout as any)?.columns;
    return typeof columns === "number" && Number.isFinite(columns) && columns > 0 ? columns : 0;
  };

  const storedDensity = (): DensityMode | undefined => {
    if (!api.kv || typeof api.kv.get !== "function") return undefined;
    const raw = api.kv.get(DENSITY_KEY, undefined);
    return raw === "compact" || raw === "expanded" ? raw : undefined;
  };

  // Without a persisted choice, a narrow terminal starts compact; any manual toggle wins from then on.
  const initialDensity = (): DensityMode => {
    const stored = storedDensity();
    if (stored) return stored;
    const columns = terminalColumns();
    return columns > 0 && columns < NARROW_TERMINAL_COLS ? "compact" : "expanded";
  };

  const startingDensity = initialDensity();
  const [density, setDensity] = createSignal<DensityMode>(startingDensity);

  const [tasksExpanded, setTasksExpanded] = createSignal(
    getPref(TASKS_EXPANDED_KEY, startingDensity === "expanded")
  );
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(
    getPref(DELEGATIONS_EXPANDED_KEY, startingDensity === "expanded")
  );
  const [attentionExpanded, setAttentionExpanded] = createSignal(
    getPref(ATTENTION_EXPANDED_KEY, startingDensity === "expanded")
  );

  const setSectionExpanded = (key: string, setter: (next: boolean) => void, next: boolean): void => {
    setter(next);
    setPref(key, next);
  };

  const applyDensity = (next: DensityMode): void => {
    setDensity(next);
    setPref(DENSITY_KEY, next);
    const open = next === "expanded";
    setSectionExpanded(TASKS_EXPANDED_KEY, setTasksExpanded, open);
    setSectionExpanded(DELEGATIONS_EXPANDED_KEY, setDelegationsExpanded, open);
    setSectionExpanded(ATTENTION_EXPANDED_KEY, setAttentionExpanded, open);
  };

  const toggleDensity = (): void => {
    const next: DensityMode = density() === "compact" ? "expanded" : "compact";
    applyDensity(next);
    showToast(
      next === "compact" ? `${MARK_COLLAPSED} Compact density` : `${MARK_EXPANDED} Expanded density`,
      next === "compact" ? "Sections collapsed to one line" : "Sections expanded with full detail",
      "info"
    );
  };

  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const pulse = createMemo(() => NEURAL_PULSE_FRAMES[pulseFrame() % NEURAL_PULSE_FRAMES.length]);
  const jobs = createMemo(() => snapshot().delegations.map((job, sequence) => ({ ...job, sequence })));

  const sessionElapsed = createMemo(() => {
    const scope = conversationScope(api, activeSessionOverride());
    const started = sessionStartTime(api, scope?.rootSessionID) ?? sessionStartTime(api, scope?.sessionID);
    if (started === undefined) return undefined;
    const diff = now() - started;
    return diff > 0 ? formatDuration(diff) : undefined;
  });

  const sidebarMetrics = createMemo<SidebarMetrics>(() => {
    const scope = conversationScope(api, activeSessionOverride());
    const sessionID = scope?.sessionID;
    return {
      tokensUsed: contextTokensUsed(sessionMessages(api, sessionID)),
      tokenLimit: sessionContextLimit(api, sessionID),
      cost: sessionCostUsd(api, sessionID),
    };
  });

  const [completionHistory, setCompletionHistory] = createSignal<number[]>(
    loadEtaHistory(api.kv && typeof api.kv.get === "function" ? api.kv.get(ETA_HISTORY_KEY, []) : [])
  );

  let previousDoneCount: number | undefined;
  createEffect(() => {
    const current = snapshot();
    if (!current.generated_at) {
      previousDoneCount = undefined;
      return;
    }
    const done = current.summary?.done || 0;
    if (previousDoneCount !== undefined && done > previousDoneCount) {
      const next = [...untrack(completionHistory), Date.now()].slice(-ETA_HISTORY_LIMIT);
      setCompletionHistory(next);
      if (api.kv && typeof api.kv.set === "function") {
        api.kv.set(ETA_HISTORY_KEY, next);
      }
    }
    previousDoneCount = done;
  });

  const boardEta = createMemo<Tier1Eta>(() => {
    const summary = snapshot().summary;
    const remaining = (summary.ready || 0) + (summary.in_progress || 0) + (summary.in_review || 0);
    const history = completionHistory();
    const mean = meanCompletionInterval(history);
    return {
      remaining,
      backlog: summary.backlog || 0,
      samples: history.length,
      estimateMs: mean === undefined ? undefined : mean * remaining,
      sparkline: etaSparkline(history),
    };
  });

  let disposed = false;
  let generation = 0;
  let activeKey = "";
  let pendingGeneration: number | undefined;
  let previousAttentionCount = 0;

  const togglePreference = (key: string, value: () => boolean, setter: (next: boolean) => void): void => {
    const next = !value();
    setter(next);
    setPref(key, next);
  };

  const readSnapshot = (): void => {
    if (disposed) return;
    const scope = conversationScope(api, activeSessionOverride());
    const key = JSON.stringify(scope) || "";
    if (key !== activeKey) {
      activeKey = key;
      generation += 1;
      setSnapshot(EMPTY_SNAPSHOT);
      setSnapshotError("");
      consecutiveSnapshotFailures = 0;
    }
    if (!scope || !scope.project || pendingGeneration === generation) return;
    const requestGeneration = generation;
    pendingGeneration = requestGeneration;
    const recordFailure = (message: string): void => {
      consecutiveSnapshotFailures += 1;
      if (consecutiveSnapshotFailures >= 2) setSnapshotError(message);
    };
    const runAttempt = (retryAllowed: boolean): void => {
      execFile(
        cortexExecutable(),
        [
          "ui",
          "snapshot",
          "--project",
          scope.project,
          "--session-id",
          scope.sessionID,
          "--root-session-id",
          scope.rootSessionID,
        ],
        { encoding: "utf8", maxBuffer: 512 * 1024, timeout: 7500, windowsHide: true },
        (error, stdout) => {
          if (pendingGeneration === requestGeneration) pendingGeneration = undefined;
          if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api, activeSessionOverride())) !== key) return;
          if (error) {
            // A timeout kill is the one transient class worth a single immediate
            // retry; a genuine spawn failure would recur and only delay the report.
            if ((error as { killed?: boolean }).killed === true && retryAllowed) {
              pendingGeneration = requestGeneration;
              runAttempt(false);
              return;
            }
            recordFailure(error.message);
            return;
          }
          try {
            const next = JSON.parse(stdout) as UISnapshot;
            if (
              next.schema_version !== 2 ||
              next.requested_session_id !== scope.sessionID ||
              next.root_session_id !== scope.rootSessionID ||
              !Array.isArray(next.tasks) ||
              !Array.isArray(next.delegations) ||
              !Array.isArray(next.attention) ||
              !next.counts ||
              !next.summary
            ) {
              throw new Error("snapshot schema or conversation is incompatible");
            }
            consecutiveSnapshotFailures = 0;
            setSnapshot(next);
            setSnapshotError("");
          } catch (parseError) {
            recordFailure(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
          }
        }
      );
    };
    runAttempt(true);
  };

  createEffect(() => {
    const count = operationalCounts(snapshot(), snapshotError()).attention;
    if (count > previousAttentionCount && density() === "expanded") {
      setAttentionExpanded(true);
      setPref(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });

  let previousTaskStatuses = new Map<string, string>();
  createEffect(() => {
    const tasks = snapshot().tasks;
    if (previousTaskStatuses.size > 0) {
      for (const t of tasks) {
        const prev = previousTaskStatuses.get(t.task_id);
        if (prev && prev !== t.status) {
          let variant = "info";
          let title = `Cortex-IA: Task ${t.task_id}`;
          if (t.status === "done") {
            variant = "success";
            title = `${GLYPH.done} ${t.task_id} Approved (PASS)`;
          } else if (t.status === "blocked") {
            variant = "error";
            title = `${GLYPH.fail} ${t.task_id} Blocked`;
          } else if (t.status === "in_review") {
            variant = "warning";
            title = `${GLYPH.active} ${t.task_id} In Review`;
          } else if (t.status === "in_progress") {
            variant = "info";
            title = `${GLYPH.working} ${t.task_id} Claimed`;
          }
          showToast(title, clipped(t.title, 40), variant);
        }
      }
    }
    const nextMap = new Map<string, string>();
    for (const t of tasks) nextMap.set(t.task_id, t.status);
    previousTaskStatuses = nextMap;
  });

  createEffect(() => {
    activeSessionOverride();
    readSnapshot();
  });
  const snapshotPoll = setInterval(readSnapshot, SNAPSHOT_POLL_INTERVAL_MS);
  const clock = setInterval(() => setNow(Date.now()), 1000);
  const spinnerTimer = setInterval(() => setFrame((f) => (f + 1) % SPINNER_FRAMES.length), 90);
  const pulseTimer = setInterval(() => setPulseFrame((p) => (p + 1) % NEURAL_PULSE_FRAMES.length), 350);

  const [usageStats, setUsageStats] = createSignal<UsageStatsSummary | null>(null);
  const [statsLoading, setStatsLoading] = createSignal(false);
  let pendingStats = false;

  const readUsageStats = (): void => {
    if (disposed || pendingStats) return;
    pendingStats = true;
    setStatsLoading(true);
    execFile(
      cortexExecutable(),
      ["stats", "--json"],
      { encoding: "utf8", maxBuffer: 512 * 1024, timeout: 5000, windowsHide: true },
      (error, stdout) => {
        pendingStats = false;
        setStatsLoading(false);
        if (disposed || error) return;
        try {
          const parsed = JSON.parse(stdout) as UsageStatsSummary;
          if (typeof parsed?.sessions === "number" && typeof parsed?.messages === "number") {
            setUsageStats(parsed);
          }
        } catch {}
      }
    );
  };

  readUsageStats();
  const statsPoll = setInterval(readUsageStats, 60_000);

  const [nanUsage, setNanUsage] = createSignal<NanUsageView | null>(null);
  const [nanCatalog, setNanCatalog] = createSignal<NanCatalogCache | undefined>(undefined);
  let pendingNanUsage = false;
  let pendingNanCatalog = false;
  let nanScopeKey = "";
  let nanGeneration = 0;

  const currentNanModel = (): string | undefined => {
    const record = sessionRecord(api, currentSessionID(api, activeSessionOverride()));
    const model = record?.model;
    if (!model || model.providerID !== "nan" || typeof model.id !== "string") return undefined;
    return model.id.split("#")[0];
  };

  // The catalog snapshot feeds both the strip (quota index) and the picker (full
  // metadata). A failed refresh is recorded so the picker can render an explicit
  // empty state, while the strip stays silently degraded. A legacy tier sorts last
  // so its preferred replacement renders ahead of it.
  const readNanCatalog = (): void => {
    if (disposed || pendingNanCatalog) return;
    const cached = nanCatalog();
    if (cached && Date.now() - cached.cachedAt < NAN_CATALOG_TTL_MS) return;
    pendingNanCatalog = true;
    execFile(
      cortexExecutable(),
      ["model", "catalog", "--json", "--provider", "nan"],
      { encoding: "utf8", maxBuffer: NAN_EXEC_MAX_BUFFER, timeout: NAN_EXEC_TIMEOUT_MS, windowsHide: true },
      (error, stdout) => {
        pendingNanCatalog = false;
        if (disposed) return;
        if (error) {
          setNanCatalogFailed(true);
          return;
        }
        try {
          const parsed = JSON.parse(stdout) as { entries?: NanCatalogEntry[] };
          if (!Array.isArray(parsed?.entries)) throw new Error("catalog entries");
          const index = new Map<string, number>();
          const models: NanCatalogModel[] = [];
          for (const entry of parsed.entries) {
            const meta = entry?.meta;
            if (entry?.provider !== "nan" || !meta || meta.pickerEligible !== true) continue;
            if (typeof entry.model !== "string" || entry.model === "") continue;
            if (typeof meta.monthlyQuotaTokens === "number" && Number.isFinite(meta.monthlyQuotaTokens)) index.set(entry.model, meta.monthlyQuotaTokens);
            models.push(nanCatalogModel({ model: entry.model, meta }));
          }
          models.sort((left, right) => (left.tier === "legacy" ? 1 : 0) - (right.tier === "legacy" ? 1 : 0));
          setNanCatalog({ cachedAt: Date.now(), index, models });
          setNanCatalogFailed(false);
        } catch {
          setNanCatalogFailed(true);
        }
      }
    );
  };

  const readNanUsage = (): void => {
    if (disposed) return;
    const sessionID = currentSessionID(api, activeSessionOverride()) || "";
    if (sessionID !== nanScopeKey) {
      nanScopeKey = sessionID;
      nanGeneration += 1;
      setNanUsage(null);
    }
    if (pendingNanUsage) return;
    const model = currentNanModel();
    if (!model) return;
    pendingNanUsage = true;
    const requestGeneration = nanGeneration;
    readNanCatalog();
    execFile(
      nanExecutable(),
      ["metrics", "usage"],
      { encoding: "utf8", maxBuffer: NAN_EXEC_MAX_BUFFER, timeout: NAN_EXEC_TIMEOUT_MS, windowsHide: true },
      (error, stdout) => {
        pendingNanUsage = false;
        if (disposed || requestGeneration !== nanGeneration) return;
        if (error) {
          setNanUsage(null);
          return;
        }
        try {
          const metrics = JSON.parse(stdout) as NanMetrics;
          if (!Array.isArray(metrics?.monthToDate?.byModel) || !Array.isArray(metrics?.last24h?.byModel)) {
            setNanUsage(null);
            return;
          }
          setNanUsage({
            model,
            monthToDate: nanModelTokens(metrics.monthToDate, model),
            last24h: nanModelTokens(metrics.last24h, model),
          });
        } catch {
          setNanUsage(null);
        }
      }
    );
  };

  // Reading nan signals inside the effect would add spurious dependencies, so the
  // poll body runs untracked and the effect subscribes only to the active session.
  createEffect(() => {
    activeSessionOverride();
    untrack(readNanUsage);
  });
  const nanPoll = setInterval(readNanUsage, NAN_USAGE_POLL_INTERVAL_MS);

  const nanStrip = createMemo<NanStripView | undefined>(() => {
    activeSessionOverride();
    const usage = nanUsage();
    if (!usage) return undefined;
    const model = currentNanModel();
    if (!model || model !== usage.model) return undefined;
    const quota = nanCatalog()?.index.get(model);
    if (quota === undefined) return undefined;
    return {
      percent: quota > 0 ? Math.min(100, Math.max(0, Math.round((usage.monthToDate / quota) * 100))) : undefined,
      burn: usage.last24h,
    };
  });

  const [nanCatalogFailed, setNanCatalogFailed] = createSignal(false);
  const [modelAgents, setModelAgents] = createSignal<ModelAgentOption[] | undefined>(undefined);
  const [pickerLoading, setPickerLoading] = createSignal(false);
  const [pickerPhase, setPickerPhase] = createSignal<PickerPhase>("idle");
  const [pickerPreview, setPickerPreview] = createSignal("");
  const [pickerResult, setPickerResult] = createSignal("");
  const [pickerRunError, setPickerRunError] = createSignal("");
  let pendingModelSet = false;

  const [nanDetailMetrics, setNanDetailMetrics] = createSignal<NanMetrics | undefined>(undefined);
  const [nanDetailLoading, setNanDetailLoading] = createSignal(false);
  const [nanDetailError, setNanDetailError] = createSignal("");
  let pendingNanDetail = false;

  // A failed catalog refresh is the only load error the panel must surface; an
  // absent agent list renders as its own empty state.
  const pickerLoadError = createMemo(() => (nanCatalogFailed() ? "catalog JSON failed" : ""));

  const readModelAgents = (onSettled: () => void): void => {
    if (disposed) {
      onSettled();
      return;
    }
    execFile(
      cortexExecutable(),
      ["model", "list", "--json"],
      { encoding: "utf8", maxBuffer: NAN_EXEC_MAX_BUFFER, timeout: NAN_EXEC_TIMEOUT_MS, windowsHide: true },
      (error, stdout) => {
        if (disposed) return;
        let options: ModelAgentOption[] | undefined;
        if (!error) {
          try {
            const parsed = JSON.parse(stdout) as { agents?: Array<{ agent?: string; model?: string; variant?: string }> };
            if (Array.isArray(parsed?.agents)) {
              const seen = new Set<string>();
              options = [];
              for (const entry of parsed.agents) {
                const option = agentModelOption(entry);
                if (!option || seen.has(option.agent)) continue;
                seen.add(option.agent);
                options.push(option);
              }
            }
          } catch {
            options = undefined;
          }
        }
        setModelAgents(options);
        onSettled();
      }
    );
  };

  const loadModelPicker = (): void => {
    if (disposed) return;
    setNanCatalogFailed(false);
    setModelAgents(undefined);
    setPickerPreview("");
    setPickerResult("");
    setPickerRunError("");
    setPickerPhase("idle");
    setPickerLoading(true);
    readNanCatalog();
    readModelAgents(() => {
      if (!disposed) setPickerLoading(false);
    });
  };

  // The preview path is the identical argv without --dry-run, so the confirmed
  // apply can never diverge from the receipt the user approved. A failed apply
  // refreshes nothing: the displayed mapping only follows a confirmed success.
  const runModelSet = (agentName: string, modelName: string, level: string, dryRun: boolean): void => {
    if (disposed || pendingModelSet) return;
    const args = ["model", "set", agentName, `nan/${modelName}`];
    if (level) args.push("--effort", level);
    if (dryRun) args.push("--dry-run");
    args.push("--json");
    pendingModelSet = true;
    if (dryRun) setPickerPhase("previewing");
    else {
      setPickerPhase("applying");
      setPickerRunError("");
    }
    execFile(
      cortexExecutable(),
      args,
      { encoding: "utf8", maxBuffer: NAN_EXEC_MAX_BUFFER, timeout: NAN_EXEC_TIMEOUT_MS, windowsHide: true },
      (error, stdout, stderr) => {
        pendingModelSet = false;
        if (disposed) return;
        if (error) {
          setPickerPhase(dryRun ? "idle" : "ready");
          setPickerRunError(nanCommandFailure(error, stderr));
          return;
        }
        const receipt = nanSetReceiptLines(stdout).join("\n");
        if (dryRun) {
          setPickerPreview(receipt);
          setPickerPhase("ready");
          return;
        }
        setPickerResult(receipt);
        setPickerPhase("applied");
        readModelAgents(() => {});
      }
    );
  };

  const loadNanDetails = (): void => {
    if (disposed || pendingNanDetail) return;
    pendingNanDetail = true;
    setNanDetailLoading(true);
    setNanDetailError("");
    readNanCatalog();
    execFile(
      nanExecutable(),
      ["metrics", "usage"],
      { encoding: "utf8", maxBuffer: NAN_EXEC_MAX_BUFFER, timeout: NAN_EXEC_TIMEOUT_MS, windowsHide: true },
      (error, stdout) => {
        pendingNanDetail = false;
        if (disposed) return;
        setNanDetailLoading(false);
        if (error) {
          setNanDetailError("nan metrics unavailable");
          return;
        }
        try {
          const metrics = JSON.parse(stdout) as NanMetrics;
          if (!Array.isArray(metrics?.timeSeries) || !Array.isArray(metrics?.monthToDate?.byModel)) {
            setNanDetailError("nan metrics payload was not understood");
            return;
          }
          setNanDetailMetrics(metrics);
        } catch {
          setNanDetailError("nan metrics payload was not understood");
        }
      }
    );
  };

  const nanDetailDays = createMemo(() => nanDailySeries(nanDetailMetrics()?.timeSeries, NAN_DETAIL_DAY_POINTS));
  const nanDetailRows = createMemo<NanModelUsageRow[]>(() => {
    const window = nanDetailMetrics()?.monthToDate;
    const rows: NanModelUsageRow[] = [];
    for (const entry of window?.byModel ?? []) {
      if (typeof entry?.model !== "string" || entry.model === "") continue;
      const meta = nanCatalog()?.models.find((candidate) => candidate.model === entry.model);
      rows.push({
        model: entry.model,
        monthToDate: nanModelTokens(window, entry.model),
        quota: meta?.quota ?? 0,
        rolling4hRef: meta?.rolling4hRef ?? 0,
      });
    }
    rows.sort((left, right) => right.monthToDate - left.monthToDate);
    return rows;
  });

  const [agentsOpen, setAgentsOpen] = createSignal(false);
  const [agentsChildren, setAgentsChildren] = createSignal<any[]>([]);
  const [agentsLive, setAgentsLive] = createSignal<Map<string, SubagentLive>>(new Map());
  const [agentsSelected, setAgentsSelected] = createSignal(0);
  const [agentsLoading, setAgentsLoading] = createSignal(false);
  const [agentsError, setAgentsError] = createSignal("");
  let agentsReturnRoute: { name: string; params?: Record<string, unknown> } | undefined;
  let agentsSurface: "route" | "dialog" = "route";
  let agentsOpenedFrom: string | undefined;
  let agentsPending = false;
  let agentsKnownChildren = new Set<string>();
  let agentsKeyHandler: ((event: any) => void) | undefined;
  const agentsEventDisposers: Array<() => void> = [];

  // The snapshot poll already resolves the conversation root, so the panel never
  // re-derives ancestry by walking parentID chains on its own.
  const subagentRootSessionID = (): string => {
    const root = snapshot().root_session_id;
    if (typeof root === "string" && root !== "" && root !== "global") return root;
    return conversationScope(api, activeSessionOverride())?.rootSessionID || "";
  };

  // The panel never polls on its own: the child list refreshes with the existing
  // snapshot cadence so concurrent authority writers are never contended faster.
  const readSubagentChildren = (): void => {
    if (disposed || agentsPending) return;
    const root = subagentRootSessionID();
    if (!root) {
      setAgentsChildren([]);
      agentsKnownChildren = new Set();
      setAgentsError("");
      return;
    }
    const listFn = api.client?.session?.children;
    if (typeof listFn !== "function") {
      setAgentsError("subagent sessions are unavailable in this host");
      return;
    }
    agentsPending = true;
    setAgentsLoading(true);
    let request: unknown;
    try {
      request = listFn.call(api.client.session, { sessionID: root });
    } catch {
      agentsPending = false;
      setAgentsLoading(false);
      setAgentsError("subagent sessions are unavailable in this host");
      return;
    }
    Promise.resolve(request)
      .then((response: any) => {
        if (disposed) return;
        const list = Array.isArray(response) ? response : response?.data;
        if (!Array.isArray(list)) {
          setAgentsError("subagent sessions are unavailable in this host");
          return;
        }
        const children = list.filter((child: any) => child && typeof child.id === "string" && child.id !== root);
        agentsKnownChildren = new Set(children.map((child: any) => child.id));
        setAgentsChildren(children);
        setAgentsError("");
      })
      .catch(() => {
        if (!disposed) setAgentsError("subagent sessions are unavailable in this host");
      })
      .finally(() => {
        agentsPending = false;
        if (!disposed) setAgentsLoading(false);
      });
  };

  const patchSubagentLive = (sessionID: unknown, patch: SubagentLive): void => {
    if (typeof sessionID !== "string" || sessionID === "" || !agentsOpen()) return;
    if (!agentsKnownChildren.has(sessionID)) return;
    setAgentsLive((previous) => {
      const next = new Map(previous);
      next.set(sessionID, { ...next.get(sessionID), ...patch });
      return next;
    });
  };

  const partActivityPatch = (part: any): SubagentLive => {
    const label = partActivityLabel(part);
    return { tool: label?.tool, step: label?.step, updatedAt: Date.now() };
  };

  const agentRows = createMemo<SubagentRow[]>(() => {
    const tasks = snapshot().tasks;
    const live = agentsLive();
    const rows = agentsChildren().map((child: any): SubagentRow => {
      const patch = live.get(child.id);
      const stateStatus = api.state?.session?.status?.(child.id) || api.data?.session?.status?.(child.id);
      const patchActivity = patch && (patch.tool || patch.step) ? { tool: patch.tool, step: patch.step } : lastActivityLabel(api, child.id);
      const task = taskForSession(child.id, tasks);
      const created = sessionStartMillis(child);
      return {
        sessionID: child.id,
        title: patch?.title || (typeof child.title === "string" && child.title ? child.title : child.id),
        agent: patch?.agent || (typeof child.agent === "string" ? child.agent : ""),
        status: patch?.status ?? subagentStatus(stateStatus),
        retryAttempt: patch?.retryAttempt ?? subagentRetryAttempt(stateStatus),
        startedAt: created,
        tool: patchActivity.tool,
        step: patchActivity.step,
        tokens: patch?.tokens ?? sessionTokenTotal(child),
        cost: patch?.cost ?? (typeof child.cost === "number" && child.cost > 0 ? child.cost : undefined),
        updatedAt: patch?.updatedAt ?? created ?? 0,
        taskID: task?.taskID,
        taskStatus: task?.status,
      };
    });
    rows.sort((left, right) => {
      const rank = subagentRank(left.status) - subagentRank(right.status);
      return rank !== 0 ? rank : right.updatedAt - left.updatedAt;
    });
    return rows;
  });

  createEffect(() => {
    const count = agentRows().length;
    if (count === 0) {
      if (agentsSelected() !== 0) setAgentsSelected(0);
      return;
    }
    if (agentsSelected() >= count) setAgentsSelected(count - 1);
  });

  const moveAgentsSelection = (delta: number): void => {
    const count = agentRows().length;
    if (count === 0) return;
    const next = Math.min(count - 1, Math.max(0, agentsSelected() + delta));
    if (next !== agentsSelected()) setAgentsSelected(next);
  };

  const agentsKeyInput = (): any => {
    const keyInput = api.renderer?.keyInput;
    return keyInput && typeof keyInput.on === "function" ? keyInput : undefined;
  };

  const detachAgentsKeys = (): void => {
    const keyInput = agentsKeyInput();
    if (agentsKeyHandler && keyInput && typeof keyInput.off === "function") {
      try {
        keyInput.off("keypress", agentsKeyHandler);
      } catch {}
    }
    agentsKeyHandler = undefined;
  };

  // The host may or may not report the plugin route through route.current, so the
  // panel treats its own route and the route it opened from as active, and only
  // yields the keyboard once a genuinely different host route is reported.
  const agentsPanelOwnsKeyboard = (): boolean => {
    if (agentsSurface === "dialog") return true;
    const name = (api.route?.current as { name?: unknown } | undefined)?.name;
    if (typeof name !== "string" || name === "" || name === AGENTS_ROUTE) return true;
    return name === agentsOpenedFrom;
  };

  const handleAgentsKey = (event: any): void => {
    if (!agentsOpen() || !agentsPanelOwnsKeyboard()) return;
    const name = typeof event?.name === "string" ? event.name.toLowerCase() : "";
    const sequence = typeof event?.sequence === "string" ? event.sequence : "";
    const modifiers = Boolean(event?.ctrl || event?.meta);
    let handled = true;
    if (name === "up" || sequence === "\u001b[A" || (!modifiers && name === "k")) {
      moveAgentsSelection(-1);
    } else if (name === "down" || sequence === "\u001b[B" || (!modifiers && name === "j")) {
      moveAgentsSelection(1);
    } else if (name === "return" || name === "enter" || name === "linefeed" || sequence === "\r") {
      activateSubagentRow(agentsSelected());
    } else if (name === "escape" || name === "esc" || sequence === "\u001b") {
      closeAgentsPanel();
    } else {
      handled = false;
    }
    if (!handled) return;
    if (typeof event?.preventDefault === "function") event.preventDefault();
    if (typeof event?.stopPropagation === "function") event.stopPropagation();
  };

  const attachAgentsKeys = (): void => {
    if (agentsKeyHandler) return;
    const keyInput = agentsKeyInput();
    if (!keyInput) return;
    const handler = (event: any) => handleAgentsKey(event);
    try {
      keyInput.on("keypress", handler);
      agentsKeyHandler = handler;
    } catch {
      agentsKeyHandler = undefined;
    }
  };

  const resetAgentsPanelState = (): void => {
    detachAgentsKeys();
    setAgentsOpen(false);
    setAgentsLive(new Map());
    agentsKnownChildren = new Set();
    setAgentsChildren([]);
    setAgentsError("");
  };

  // Leaving through the panel's own action restores the route the user came from,
  // but entering a child session must not bounce through the parent route first.
  const closeAgentsPanel = (restoreRoute = true): void => {
    if (!agentsOpen()) return;
    resetAgentsPanelState();
    const dialogStack = api.ui?.dialog;
    if (dialogStack && typeof dialogStack.clear === "function") {
      try {
        dialogStack.clear();
      } catch {}
    }
    const route = api.route;
    const target = agentsReturnRoute;
    agentsReturnRoute = undefined;
    if (restoreRoute && target && route && typeof route.navigate === "function") {
      try {
        route.navigate(target.name, target.params);
      } catch {}
    }
  };

  const openSubagentSession = (sessionID: string): void => {
    if (!sessionID) return;
    closeAgentsPanel(false);
    const route = api.route;
    if (route && typeof route.navigate === "function") {
      try {
        route.navigate("session", { sessionID });
        return;
      } catch {}
    }
    // Hosts without a plugin route surface still expose the documented session
    // switch endpoint, so a click stays useful instead of dead-ending.
    const selectSession = api.client?.tui?.selectSession;
    if (typeof selectSession === "function") {
      try {
        void Promise.resolve(selectSession.call(api.client.tui, { sessionID })).catch(() => {});
        return;
      } catch {}
    }
    showToast("Subagent", "this host cannot navigate to a child session", "info");
  };

  const activateSubagentRow = (index: number): void => {
    const row = agentRows()[index];
    if (!row) return;
    setAgentsSelected(index);
    openSubagentSession(row.sessionID);
  };

  const agentsPanelView = () => (
    <CortexAgentsPanel
      rows={agentRows}
      selected={agentsSelected}
      loading={agentsLoading}
      error={agentsError}
      now={now}
      pulse={pulse}
      onActivate={activateSubagentRow}
      theme={api.theme?.current || api.theme}
    />
  );

  const openAgentsPanel = (): boolean => {
    const route = api.route;
    const dialogStack = api.ui?.dialog;
    const hasRoute = Boolean(route && typeof route.register === "function" && typeof route.navigate === "function");
    const hasDialog = Boolean(dialogStack && typeof dialogStack.replace === "function");
    if (!hasRoute && !hasDialog) {
      showToast("Cortex Subagents", "this host exposes no route or dialog surface", "warning");
      return false;
    }
    if (hasRoute) {
      agentsSurface = "route";
      agentsOpenedFrom = undefined;
      const current = route.current as { name?: unknown; params?: Record<string, unknown> } | undefined;
      if (current && typeof current.name === "string" && current.name !== AGENTS_ROUTE) {
        agentsOpenedFrom = current.name;
        agentsReturnRoute = { name: current.name, params: current.params };
      }
      try {
        route.navigate(AGENTS_ROUTE, {});
      } catch {}
    } else {
      agentsSurface = "dialog";
      agentsOpenedFrom = undefined;
      try {
        if (typeof dialogStack.setSize === "function") dialogStack.setSize("large");
        dialogStack.replace(() => agentsPanelView(), () => resetAgentsPanelState());
      } catch {}
    }
    setAgentsOpen(true);
    setAgentsSelected(0);
    attachAgentsKeys();
    readSubagentChildren();
    return true;
  };

  if (api.event && typeof api.event.on === "function") {
    const patch = (event: any, live: SubagentLive): void => patchSubagentLive(event?.properties?.sessionID, live);
    const busy = (tool: string | undefined): SubagentLive => ({ tool, step: undefined, status: "busy", updatedAt: Date.now() });
    const subscriptions: Array<[string, (event: any) => void]> = [
      ["session.status", (event) => patch(event, { status: subagentStatus(event?.properties?.status), retryAttempt: subagentRetryAttempt(event?.properties?.status), updatedAt: Date.now() })],
      ["session.idle", (event) => patch(event, { status: "idle", tool: undefined, step: undefined, updatedAt: Date.now() })],
      ["session.updated", (event) => {
        const info = event?.properties?.info;
        patchSubagentLive(info?.id ?? event?.properties?.sessionID, {
          title: typeof info?.title === "string" && info.title ? info.title : undefined,
          agent: typeof info?.agent === "string" && info.agent ? info.agent : undefined,
          tokens: sessionTokenTotal(info),
          cost: typeof info?.cost === "number" && info.cost > 0 ? info.cost : undefined,
          updatedAt: Date.now(),
        });
      }],
      ["session.next.tool.input.started", (event) => patch(event, busy(typeof event?.properties?.name === "string" ? event.properties.name : undefined))],
      ["session.next.tool.called", (event) => patch(event, busy(typeof event?.properties?.tool === "string" ? event.properties.tool : undefined))],
      ["session.next.tool.success", (event) => patch(event, { tool: undefined, updatedAt: Date.now() })],
      ["session.next.tool.failed", (event) => patch(event, { tool: undefined, updatedAt: Date.now() })],
      ["session.next.step.started", (event) => patch(event, { step: "step", tool: undefined, status: "busy", updatedAt: Date.now() })],
      ["session.next.step.ended", (event) => {
        const properties = event?.properties;
        patch(event, {
          step: undefined,
          tool: undefined,
          tokens: sessionTokenTotal({ tokens: properties?.tokens }),
          cost: typeof properties?.cost === "number" && properties.cost > 0 ? properties.cost : undefined,
          updatedAt: Date.now(),
        });
      }],
      ["message.part.updated", (event) => patch(event, partActivityPatch(event?.properties?.part))],
    ];
    for (const [type, handler] of subscriptions) {
      try {
        const off = api.event.on(type, handler);
        if (typeof off === "function") agentsEventDisposers.push(off);
      } catch {}
    }
  }

  createEffect(() => {
    const open = agentsOpen();
    const stamp = snapshot().generated_at;
    if (!open || !stamp) return;
    untrack(() => readSubagentChildren());
  });

  const cleanup = () => {
    if (disposed) return;
    disposed = true;
    detachAgentsKeys();
    for (const off of agentsEventDisposers) {
      try {
        off();
      } catch {}
    }
    agentsEventDisposers.length = 0;
    clearInterval(snapshotPoll);
    clearInterval(statsPoll);
    clearInterval(nanPoll);
    clearInterval(clock);
    clearInterval(spinnerTimer);
    clearInterval(pulseTimer);
    disposeRoot();
  };

  // OpenCode v2 Slot Registration (context.ui.slot)
  if (api.ui && typeof api.ui.slot === "function") {
    api.ui.slot({
      append: "sidebar.content",
      render: (ctx: any) => {
        updateActiveSession(ctx);
        return (
          <SidebarStatus
            nativeActivity={nativeActivity}
            scopeReady={scopeReady}
            snapshot={snapshot}
            jobs={jobs}
            snapshotError={snapshotError}
            sessionElapsed={sessionElapsed}
            eta={boardEta}
            now={now}
            spinner={spinner}
            pulse={pulse}
            density={density}
            toggleDensity={toggleDensity}
            tasksExpanded={tasksExpanded}
            delegationsExpanded={delegationsExpanded}
            attentionExpanded={attentionExpanded}
            toggleTasks={() => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded)}
            toggleDelegations={() => togglePreference(DELEGATIONS_EXPANDED_KEY, delegationsExpanded, setDelegationsExpanded)}
            toggleAttention={() => togglePreference(ATTENTION_EXPANDED_KEY, attentionExpanded, setAttentionExpanded)}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
        );
      },
    });

    // OpenCode v2 has no v1 home_logo slot; the banner prepends the home footer surface instead.
    api.ui.slot({
      prepend: "home.footer",
      render: (ctx: any) => (
        <box flexDirection="column" width="100%">
          <HomeLogo theme={ctx?.theme?.current || ctx?.theme || api.theme} />
          <HomeStatsWidget
            stats={usageStats}
            loading={statsLoading}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
        </box>
      ),
    });

    api.ui.slot({
      append: "sidebar.footer",
      render: (ctx: any) => {
        updateActiveSession(ctx);
        return (
          <SidebarFooterMetrics
            metrics={sidebarMetrics}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
        );
      },
    });

    api.ui.slot({
      append: "home.footer.status",
      render: (ctx: any) => {
        updateActiveSession(ctx);
        return (
          <HomeBottomStatus
            snapshot={snapshot}
            jobs={jobs}
            spinner={spinner}
            snapshotError={snapshotError}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
        );
      },
    });

    api.ui.slot({
      append: "session.panel",
      render: (panel: any) => {
        updateActiveSession(panel);
        const panelTheme = panel?.theme?.current || panel?.theme || api.theme;
        return (
          <Show
            when={panel?.name === "cortex.model"}
            fallback={
              <Show
                when={panel?.name === "cortex.nan"}
                fallback={
                  <Show when={!panel?.name || panel?.name === "cortex.dashboard" || panel?.name === "session.panel" || panel?.name === "cortex.board" || panel?.name === "cortex"}>
                    <SessionKanbanPanel
                      snapshot={snapshot}
                      jobs={jobs}
                      now={now}
                      spinner={spinner}
                      pulse={pulse}
                      theme={panelTheme}
                    />
                  </Show>
                }
              >
                <NanDetailPanel
                  days={nanDetailDays}
                  rows={nanDetailRows}
                  loading={nanDetailLoading}
                  loadError={nanDetailError}
                  theme={panelTheme}
                />
              </Show>
            }
          >
            <NanModelPickerPanel
              models={() => nanCatalog()?.models ?? []}
              agents={modelAgents}
              loading={pickerLoading}
              loadError={pickerLoadError}
              phase={pickerPhase}
              preview={pickerPreview}
              result={pickerResult}
              runError={pickerRunError}
              onSelectionChange={() => { setPickerPreview(""); setPickerResult(""); setPickerRunError(""); }}
              onPreview={(agent, model, level) => runModelSet(agent, model, level, true)}
              onApply={(agent, model, level) => runModelSet(agent, model, level, false)}
              theme={panelTheme}
            />
          </Show>
        );
      },
    });

    api.ui.slot({
      append: "session.composer.top",
      render: (ctx: any) => (
        <NanUsageStrip
          view={nanStrip}
          textLimit={Math.max(24, terminalColumns() - 8)}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      ),
    });
  }

  // The legacy `/cortex` slash command lived behind the inert `api.keymap.layer` surface;
  // it re-registers here as a guarded v2 layer that opens the board panel.
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeBoardLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [
        {
          name: ":cortex",
          title: "Cortex Board",
          desc: "Open the Cortex board and delegation panel",
          category: "Cortex",
          nargs: "0",
          run: () => {
            if (!api.ui?.panel?.open) return false;
            api.ui.panel.open("cortex.board");
            return true;
          },
        },
        {
          name: ":cortex-stats",
          title: "Cortex Usage Stats",
          desc: "Open Cortex usage statistics dashboard",
          category: "Cortex",
          nargs: "0",
          run: () => {
            openStatsView();
            return true;
          },
        },
        {
          name: ":stats",
          title: "Usage Stats",
          desc: "Open Cortex usage statistics dashboard",
          category: "Cortex",
          nargs: "0",
          run: () => {
            openStatsView();
            return true;
          },
        },
      ],
    });
    if (typeof disposeBoardLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeBoardLayer);
    }
  }

  // `api.keymap.layer` does not exist on the v2 keymap host (`registerLayer` is the
  // documented surface), so the density toggle registers its own layer.
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeDensityLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [
        {
          name: ":cortex-density",
          title: "Cortex Density",
          desc: "Toggle sidebar compact/expanded density",
          category: "Cortex",
          nargs: "0",
          run: () => {
            toggleDensity();
            return true;
          },
        },
      ],
      bindings: [{ key: "alt+d", cmd: ":cortex-density" }],
    });
    if (typeof disposeDensityLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeDensityLayer);
    }
  }

  // The picker and detail layers reuse the same v2 registerLayer host contract:
  // each command opens its panel by name, and every catalog or metrics read stays
  // lazy until the panel is actually opened.
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const nanPanels = [
      { name: ":model", title: "Nan Model Picker", desc: "Pick a nan model and effort for an agent from the catalog", panel: "cortex.model", load: loadModelPicker },
      { name: ":nan", title: "Nan Usage Detail", desc: "Open the nan daily usage detail panel", panel: "cortex.nan", load: loadNanDetails },
    ];
    const disposeNanLayers = api.keymap.registerLayer({
      priority: 40,
      commands: nanPanels.map((entry) => ({
        name: entry.name,
        title: entry.title,
        desc: entry.desc,
        category: "Cortex",
        nargs: "0",
        run: () => {
          if (!api.ui?.panel?.open) return false;
          api.ui.panel.open(entry.panel);
          entry.load();
          return true;
        },
      })),
    });
    if (typeof disposeNanLayers === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeNanLayers);
    }
  }

  // The agents panel owns a plugin route plus one Cortex command. The route is
  // registered up front so the command only has to navigate to it, and the whole
  // block stays inert on runtimes without the v2 route surface.
  if (api.route && typeof api.route.register === "function") {
    let disposeAgentsRoute: (() => void) | undefined;
    try {
      disposeAgentsRoute = api.route.register([
        { name: AGENTS_ROUTE, render: () => agentsPanelView() },
      ]);
    } catch {}
    if (typeof disposeAgentsRoute === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeAgentsRoute);
    }
  }

  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeAgentsLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [
        {
          name: AGENTS_COMMAND,
          title: "Cortex Subagents",
          desc: "Open the live subagent monitor panel",
          category: "Cortex",
          nargs: "0",
          run: () => openAgentsPanel(),
        },
      ],
    });
    if (typeof disposeAgentsLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeAgentsLayer);
    }
  }

  // OpenCode v1 Slot Registration (api.slots.register)
  const registeredSlots: Record<string, (ctx: any) => any> = {
    home_logo(ctx: any) {
      return <HomeLogo theme={ctx?.theme?.current || ctx?.theme || api.theme} />;
    },
    sidebar_content(ctx: any) {
      updateActiveSession(ctx);
      return (
        <SidebarStatus
          nativeActivity={nativeActivity}
          scopeReady={scopeReady}
          snapshot={snapshot}
          jobs={jobs}
          snapshotError={snapshotError}
          sessionElapsed={sessionElapsed}
          eta={boardEta}
          now={now}
          spinner={spinner}
          pulse={pulse}
          density={density}
          toggleDensity={toggleDensity}
          tasksExpanded={tasksExpanded}
          delegationsExpanded={delegationsExpanded}
          attentionExpanded={attentionExpanded}
          toggleTasks={() => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded)}
          toggleDelegations={() => togglePreference(DELEGATIONS_EXPANDED_KEY, delegationsExpanded, setDelegationsExpanded)}
          toggleAttention={() => togglePreference(ATTENTION_EXPANDED_KEY, attentionExpanded, setAttentionExpanded)}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      );
    },
    home_bottom(ctx: any) {
      updateActiveSession(ctx);
      return (
        <box flexDirection="column" width="100%">
          <HomeStatsWidget
            stats={usageStats}
            loading={statsLoading}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
          <HomeBottomStatus
            snapshot={snapshot}
            jobs={jobs}
            spinner={spinner}
            snapshotError={snapshotError}
            theme={ctx?.theme?.current || ctx?.theme || api.theme}
          />
        </box>
      );
    },
    "home.footer.status"(ctx: any) {
      updateActiveSession(ctx);
      return (
        <HomeBottomStatus
          snapshot={snapshot}
          jobs={jobs}
          spinner={spinner}
          snapshotError={snapshotError}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      );
    },
    "session.panel"(ctx: any) {
      updateActiveSession(ctx);
      return (
        <SessionKanbanPanel
          snapshot={snapshot}
          jobs={jobs}
          now={now}
          spinner={spinner}
          pulse={pulse}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      );
    },
    "session.composer.top"(ctx: any) {
      return (
        <NanUsageStrip
          view={nanStrip}
          textLimit={Math.max(24, terminalColumns() - 8)}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      );
    },
    session_panel(ctx: any) {
      updateActiveSession(ctx);
      return (
        <SessionKanbanPanel
          snapshot={snapshot}
          jobs={jobs}
          now={now}
          spinner={spinner}
          pulse={pulse}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
      );
    },
  };

  if (api.slots && typeof api.slots.register === "function") {
    api.slots.register({
      order: 85,
      slots: registeredSlots,
    });
  }

  if (api.lifecycle && typeof api.lifecycle.onDispose === "function") {
    api.lifecycle.onDispose(cleanup);
  }

  return cleanup;
}

export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => any; tui?: (api: any) => any; server?: (ctx: any) => any }>(def: T): T => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  },
};

const tui: TuiPlugin = async (api: any) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};

const setup = (context: any) => {
  return createRoot((disposeRoot) => initialize(context, disposeRoot));
};

export const CortexTUIPluginDefinition = Plugin.define({
  id: "cortex-ia.delegation-status",
  setup,
  tui,
});

export default CortexTUIPluginDefinition;
