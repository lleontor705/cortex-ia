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

// Modern Cyberpunk & Neural Color Palette for Cortex-IA
const CORTEX_THEME = {
  brandViolet: "#8b5cf6",
  brandIndigo: "#6366f1",
  brandPurple: "#a855f7",
  neonCyan: "#06b6d4",
  skyBlue: "#38bdf8",
  amberGold: "#f59e0b",
  emeraldGreen: "#10b981",
  roseRed: "#f43f5e",
  slateBorder: "#334155",
  slateDark: "#0f172a",
  slateCard: "#1e293b",
  slateMuted: "#64748b",
  slateLight: "#cbd5e1",
  pureWhite: "#ffffff",
};

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
  panel: ColorInput;
};

type PalettePath = readonly (string | number)[];

// Hue steps mirror the shipped cortex scale exactly (emeraldGreen=green.500,
// amberGold=yellow.500, roseRed=red.500, neonCyan=cyan.500, border=neutral.700),
// so a hue-aware theme reproduces today's dark palette while light themes get the
// matching light steps. Semantic and legacy flat tokens cover hue-less themes.
const PALETTE_SOURCES: Record<keyof CortexPalette, readonly PalettePath[]> = {
  text: [["text", "base"], ["text"]],
  textMuted: [["hue", "neutral", 500], ["textMuted"], ["text", "muted"]],
  textSoft: [["hue", "neutral", 300]],
  accent: [["hue", "purple", 400], ["accent"], ["text", "action", "primary", "base"]],
  accentAlt: [["hue", "purple", 500], ["secondary"]],
  primary: [["hue", "blue", 500], ["primary"]],
  sky: [["hue", "blue", 400], ["info"]],
  success: [["hue", "green", 500], ["success"], ["text", "feedback", "success", "base"]],
  warning: [["hue", "yellow", 500], ["warning"], ["text", "feedback", "warning", "base"]],
  error: [["hue", "red", 500], ["error"], ["text", "feedback", "error", "base"]],
  info: [["hue", "cyan", 500], ["info"], ["text", "feedback", "info", "base"]],
  border: [["hue", "neutral", 700], ["border", "base"], ["border"], ["borderSubtle"]],
  panel: [["hue", "neutral", 800], ["background", "raised", "high"], ["backgroundPanel"]],
};

const PALETTE_FALLBACK: Record<keyof CortexPalette, string> = {
  text: CORTEX_THEME.pureWhite,
  textMuted: CORTEX_THEME.slateMuted,
  textSoft: CORTEX_THEME.slateLight,
  accent: CORTEX_THEME.brandPurple,
  accentAlt: CORTEX_THEME.brandViolet,
  primary: CORTEX_THEME.brandIndigo,
  sky: CORTEX_THEME.skyBlue,
  success: CORTEX_THEME.emeraldGreen,
  warning: CORTEX_THEME.amberGold,
  error: CORTEX_THEME.roseRed,
  info: CORTEX_THEME.neonCyan,
  border: CORTEX_THEME.slateBorder,
  panel: CORTEX_THEME.slateCard,
};

function readColor(source: unknown, path: PalettePath): ColorInput | undefined {
  let node: unknown = source;
  for (const key of path) {
    if (typeof node !== "object" || node === null) return undefined;
    node = (node as Record<string | number, unknown>)[key];
  }
  if (typeof node === "string") return node;
  if (typeof node === "object" && node !== null) {
    const candidate = node as { r?: unknown; g?: unknown; b?: unknown };
    if (typeof candidate.r === "number" && typeof candidate.g === "number" && typeof candidate.b === "number") {
      return node as ColorInput;
    }
  }
  return undefined;
}

function resolvePalette(theme?: TuiThemeCurrent): CortexPalette {
  const source = theme as unknown as Record<string, unknown> | undefined;
  const palette = {} as CortexPalette;
  for (const role of Object.keys(PALETTE_SOURCES) as (keyof CortexPalette)[]) {
    let resolved: ColorInput | undefined;
    for (const path of PALETTE_SOURCES[role]) {
      resolved = readColor(source, path);
      if (resolved !== undefined) break;
    }
    palette[role] = resolved ?? PALETTE_FALLBACK[role];
  }
  return palette;
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
  elapsed?: string;
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

function CortexCockpitHeader(props: {
  isExecuting: () => boolean;
  nativeActivity: () => NativeActivity | undefined;
  projectRoot?: string;
  sessionElapsed?: () => string | undefined;
  density: () => DensityMode;
  onToggleDensity: () => void;
  textLimit: number;
  spinner: () => string;
  pulse: () => string;
  theme?: TuiThemeCurrent;
}) {
  const projectName = createMemo(() => {
    if (!props.projectRoot) return "";
    return path.basename(props.projectRoot);
  });
  const palette = resolvePalette(props.theme);

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
      {/* Project Root on its own dedicated line to prevent word wrapping */}
      <Show when={projectName()}>
        <box flexDirection="row" marginTop={0}>
          <text fg={palette.sky}>{`${GLYPH.row} `}</text>
          <text fg={palette.sky}>{clipped(projectName(), Math.max(6, props.textLimit - 4))}</text>
        </box>
      </Show>

      <Show when={props.sessionElapsed?.()}>
        {(elapsed: () => string) => (
          <box flexDirection="row" marginTop={0}>
            <text fg={palette.sky}>{clipped(`T+${elapsed()}`, Math.max(8, props.textLimit))}</text>
          </box>
        )}
      </Show>

      <box flexDirection="row" marginTop={0}>
        <text fg={palette.info} onMouseDown={() => openWebConsole()} selectable={false}>
          {`[${GLYPH.web} Web]`}
        </text>
        <text
          fg={props.density() === "compact" ? palette.success : palette.textMuted}
          onMouseDown={props.onToggleDensity}
          selectable={false}
        >
          {props.density() === "compact"
            ? ` [${MARK_COLLAPSED} ${props.textLimit < 26 ? "COMP" : "COMPACT"}]`
            : ` [${MARK_EXPANDED} ${props.textLimit < 26 ? "EXPD" : "EXPANDED"}]`}
        </text>
      </box>
    </box>
  );
}

function OperationalKPIHud(props: {
  activeExecutions: number;
  inReview: number;
  doneTasks: number;
  attentionCount: number;
  spinner: () => string;
  theme?: TuiThemeCurrent;
}) {
  const isAllZero = () =>
    props.activeExecutions === 0 &&
    props.inReview === 0 &&
    props.doneTasks === 0 &&
    props.attentionCount === 0;
  const palette = resolvePalette(props.theme);

  return (
    <box flexDirection="column" marginTop={1}>
      <Show
        when={!isAllZero()}
        fallback={
          <box flexDirection="row">
            <text fg={palette.textMuted}>○ 0 act · 0 rev · 0 done · 0 alrt</text>
          </box>
        }
      >
        {/* Row 1 */}
        <box flexDirection="row">
          <text fg={props.activeExecutions > 0 ? palette.warning : palette.textMuted}>
            {`[${props.activeExecutions > 0 ? props.spinner() : "●"} ${props.activeExecutions} ACT] `}
          </text>
          <text fg={props.inReview > 0 ? palette.accent : palette.textMuted}>
            {`[◆ ${props.inReview} REVW]`}
          </text>
        </box>
        {/* Row 2 */}
        <box flexDirection="row" marginTop={0}>
          <text fg={props.doneTasks > 0 ? palette.success : palette.textMuted}>
            {`[✓ ${props.doneTasks} DONE] `}
          </text>
          <text fg={props.attentionCount > 0 ? palette.error : palette.textMuted}>
            {`[${props.attentionCount > 0 ? "✕" : "○"} ${props.attentionCount} ALRT]`}
          </text>
        </box>
      </Show>
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
  const palette = resolvePalette(props.theme);

  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row">
        <text fg={palette.info}>{props.compact ? "D:" : "DAG: "}</text>
        <text fg={palette.success}>{"█".repeat(doneW())}</text>
        <text fg={palette.accent}>{"▓".repeat(revW())}</text>
        <text fg={palette.warning}>{"▒".repeat(progW())}</text>
        <Show when={blckW() > 0}>
          <text fg={palette.error}>{"▓".repeat(blckW())}</text>
        </Show>
        <text fg={palette.border}>{"░".repeat(emptyW())}</text>
        <text fg={palette.text}>{` ${pct()}%`}</text>
        <text fg={palette.textMuted}>{props.compact ? "" : ` (${props.done}/${props.total})`}</text>
      </box>
      <box flexDirection="row">
        <text fg={palette.success}>{`✓${props.done} `}</text>
        <text fg={palette.accent}>{`◆${props.inReview} `}</text>
        <text fg={palette.warning}>{`●${props.inProgress} `}</text>
        <Show when={(props.blocked || 0) > 0}>
          <text fg={palette.error}>{`✕${props.blocked} `}</text>
        </Show>
        <text fg={palette.textMuted}>{`○${waiting()}`}</text>
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
      borderColor={palette.warning}
      title={clipped(`${props.spinner()} TASK IN PROGRESS`, props.textLimit)}
      titleColor={palette.warning}
      titleAlignment="left"
      backgroundColor={palette.panel}
    >
      {/* Task ID & Title */}
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

      {/* Metrics Row: Elapsed + Lease */}
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
        {/* Worker / Delegation Info */}
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
                {`${GLYPH.working} AGY ${del().transport || "direct"}${del().pane_id ? ` · ${del().pane_id}` : ""}${del().attempt ? ` · int #${del().attempt}` : ""}`}
              </text>
            )}
          </Show>
        </box>

        {/* 1-Click to Web Action Button */}
        <box
          flexDirection="row"
          marginTop={0}
          onMouseDown={() => openWebConsole(props.task.board_id, props.task.task_id)}
        >
          <text fg={palette.info} selectable={false}>
            {`  [ ${GLYPH.web} View in Web ]`}
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
    <Show when={props.tasks.length > 0} fallback={<text fg={palette.textMuted}>  ○ No queued tasks</text>}>
      <For each={props.tasks.slice(0, MAX_VISIBLE_ROWS)}>
        {(task) => {
          const chip = taskStatusChip(task.status, palette);
          const isProg = task.status === "in_progress";
          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row">
                <text fg={chip.color}>
                  {`  ${isProg ? props.spinner() : chip.icon} `}
                </text>
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
              <box flexDirection="row">
                <text fg={palette.textMuted}>
                  {`     ${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` · ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` · ${GLYPH.warn} ${task.lease_count}lk` : ""}`}
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
    <Show when={props.jobs.length > 0} fallback={<text fg={palette.textMuted}>  ○ No active workers</text>}>
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
              <box flexDirection="row">
                <text fg={statusCol}>
                  {`  ${isRunning ? props.spinner() : job.status === "succeeded" ? "✓" : "✕"} `}
                </text>
                <text fg={chip.color}>{`[${chip.tag}] `}</text>
                <text fg={isRunning ? palette.text : palette.textSoft}>
                  {clipped(job.role || "worker", Math.max(6, props.textLimit - 12))}
                </text>
                <text fg={statusCol}>{elapsed()}</text>
              </box>
              <box flexDirection="row">
                <text fg={palette.textMuted}>
                  {clipped(`     ${shortID(job.job_id)} · ${job.transport || "direct"}${job.pane_id ? ` · ${job.pane_id}` : ""}${job.attempt ? ` · int #${job.attempt}` : ""}`, props.textLimit)}
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
    <Show when={props.items.length > 0} fallback={<text fg={palette.success}>  ✓ No pending alerts</text>}>
      <For each={props.items.slice(0, MAX_VISIBLE_ROWS)}>
        {(item) => (
          <box flexDirection="column">
            <box flexDirection="row">
              <text fg={palette.error}>{`  ✕ `}</text>
              <text fg={palette.text}>{clipped(item.title, Math.max(8, props.textLimit - 4))}</text>
            </box>
              <text fg={palette.textMuted}>{`     ${clipped(item.detail, Math.max(8, props.textLimit - 5))}`}</text>
          </box>
        )}
      </For>
    </Show>
  );
}

function OperationalBottomDashboard(props: {
  snapshot: UISnapshot;
  jobs: DelegationJob[];
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

  const hasMetrics = createMemo(
    () => succeededJobs() > 0 || failedJobs() > 0 || activeJobs() > 0 || totalLeases() > 0 || blockedTasks() > 0
  );

  return (
    <box
      flexDirection="column"
      marginTop={1}
      paddingLeft={1}
      paddingRight={1}
      borderStyle="rounded"
      borderColor={failedJobs() > 0 ? palette.error : palette.primary}
      title={props.layout.compact ? `${GLYPH.brand} CONTROL` : `${GLYPH.brand} CONTROL MATRIX`}
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.panel}
    >
      {/* Synapse pulse / status line */}
      <box flexDirection="row">
        <Show
          when={activeJobs() > 0}
          fallback={
            <text fg={palette.success}>
              {`  ${props.pulse()} SYNAPSE: SYNCED`}
            </text>
          }
        >
          <text fg={palette.warning}>
            {`  ${props.spinner()} SYNAPSE: ENGINE ACTIVE`}
          </text>
        </Show>
        <text
          fg={palette.info}
          onMouseDown={() => openWebConsole(props.snapshot.tasks[0]?.board_id)}
          selectable={false}
        >
          {`  [${GLYPH.web} Web]`}
        </text>
      </box>

      {/* Grid: Pills only when metrics exist, otherwise clean standby indicator */}
      <Show
        when={hasMetrics()}
        fallback={
          <box flexDirection="row">
            <text fg={palette.textMuted}>  Standby · 0 runs</text>
          </box>
        }
      >
        <Show
          when={!props.layout.compact}
          fallback={
            <box flexDirection="row">
              <text fg={succeededJobs() > 0 ? palette.success : palette.textMuted}>{`✓${succeededJobs()} `}</text>
              <text fg={failedJobs() > 0 ? palette.error : palette.textMuted}>{`✕${failedJobs()} `}</text>
              <text fg={activeJobs() > 0 ? palette.warning : palette.textMuted}>{`●${activeJobs()} `}</text>
              <text fg={totalLeases() > 0 ? palette.sky : palette.textMuted}>{`${GLYPH.warn}${totalLeases()}`}</text>
              <Show when={blockedTasks() > 0}>
                <text fg={palette.error}>{` ✕${blockedTasks()}b`}</text>
              </Show>
            </box>
          }
        >
          <box flexDirection="row">
            <text fg={palette.success}>{`[ ✓ ${succeededJobs()} OK ] `}</text>
            <text fg={failedJobs() > 0 ? palette.error : palette.textMuted}>
              {`[ ✕ ${failedJobs()} FAIL ]`}
            </text>
          </box>
          <box flexDirection="row">
            <text fg={activeJobs() > 0 ? palette.warning : palette.textMuted}>
              {`[ ${activeJobs() > 0 ? props.spinner() : "●"} ${activeJobs()} ACT ] `}
            </text>
            <text fg={totalLeases() > 0 ? palette.sky : palette.textMuted}>
              {`[ ${GLYPH.warn} ${totalLeases()} LOCKS ] `}
            </text>
            <Show when={blockedTasks() > 0}>
              <text fg={palette.error}>{`[ ✕ ${blockedTasks()} BLCK ]`}</text>
            </Show>
          </box>
        </Show>
      </Show>

      {/* Operational health gauge */}
      <box flexDirection="row">
        <text fg={palette.textMuted}>Health: </text>
        <Show
          when={successRate() !== undefined}
          fallback={<text fg={palette.textMuted}>○ standby</text>}
        >
          <text fg={healthColor()}>{healthBars().filled}</text>
          <text fg={palette.border}>{healthBars().empty}</text>
          <text fg={healthColor()}>{` ${successRate()}%`}</text>
        </Show>
      </box>

      {/* SQLite authority and DAG */}
      <box flexDirection="row">
        <text fg={palette.textMuted}>
          {props.layout.compact ? "Authority: SQLite" : `${GLYPH.row} DAG: ${doneTasks()}/${totalTasks()} · Authority: SQLite`}
        </text>
      </box>

      <Show when={totalTasks() > 0}>
        <box flexDirection="row">
          <text fg={etaColor()}>{etaLabel()}</text>
          <Show when={props.eta().sparkline}>
            {(spark: () => string) => <text fg={palette.textMuted}>{` ${spark()}`}</text>}
          </Show>
        </box>
      </Show>

      {/* Data freshness / telemetry */}
      <box flexDirection="row">
        <Show
          when={props.stale}
          fallback={
            <text fg={palette.success}>
              {props.layout.compact ? `${GLYPH.working} Live` : `${GLYPH.working} Live · Synced ${syncAgeSec()}s ago`}
            </text>
          }
        >
          <text fg={palette.warning}>
            {props.layout.compact ? `${GLYPH.warn} Stale` : `${GLYPH.warn} Snapshot stale (+${syncAgeSec()}s)`}
          </text>
        </Show>
      </box>
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
      {/* 1. Cockpit header with brand logo and Cortex styling */}
      <CortexCockpitHeader
        isExecuting={isExecuting}
        nativeActivity={props.nativeActivity}
        projectRoot={props.snapshot().project_root}
        sessionElapsed={props.sessionElapsed}
        density={props.density}
        onToggleDensity={props.toggleDensity}
        textLimit={layout().textLimit}
        spinner={props.spinner}
        pulse={props.pulse}
        theme={props.theme}
      />

      <Show when={!props.scopeReady()}>
        <text fg={palette.warning} marginTop={1}>Conversation unavailable · awaiting metadata</text>
      </Show>

      <Show when={props.scopeReady() && !props.snapshot().generated_at && !props.snapshotError()}>
        <text fg={palette.textMuted} marginTop={1}>Loading conversation state…</text>
      </Show>

      <Show when={props.snapshotError()}>
        <text fg={palette.error} marginTop={1}>Update failed · data unconfirmed</text>
      </Show>
      <box flexDirection="row" marginTop={0}>
        <text fg={
          props.nativeActivity() === "busy"
            ? palette.warning
            : props.nativeActivity() === "idle"
            ? palette.success
            : palette.textMuted
        }>
          {props.nativeActivity() === "busy"
            ? `${props.spinner()} OpenCode: busy`
            : props.nativeActivity() === "idle"
            ? "● OpenCode: idle"
            : `○ OpenCode: ${props.nativeActivity() || "unknown"}`}
        </text>
      </box>

      <Show when={props.scopeReady() && Boolean(props.snapshot().generated_at)}>
        {/* 2. Operational metrics micro-HUD */}
        <OperationalKPIHud
          activeExecutions={activeExecutionsCount()}
          inReview={counts().review}
          doneTasks={doneTasks()}
          attentionCount={counts().attention}
          spinner={props.spinner}
          theme={props.theme}
        />

        {/* 3. Multi-color DAG board progress bar */}
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

        {/* 4. Active task hero card (when a task is in progress) */}
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

        {/* 5. Section: task board */}
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

        {/* 6. Section: runs & workers */}
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

        {/* 7. Section: attention center & alerts */}
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

        {/* 8. Control matrix panel and bottom counters */}
        <OperationalBottomDashboard
          snapshot={props.snapshot()}
          jobs={props.jobs()}
          stale={stale()}
          eta={props.eta}
          now={props.now}
          spinner={props.spinner}
          pulse={props.pulse}
          theme={props.theme}
          layout={layout()}
        />
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
    return Boolean(metrics.elapsed) || metrics.tokensUsed !== undefined || metrics.cost !== undefined;
  });

  return (
    <Show when={visible()}>
      <box flexDirection="row" paddingLeft={1} paddingRight={1}>
        <Show when={props.metrics().elapsed}>
          {(elapsed: () => string) => (
            <box flexDirection="row">
              <text fg={palette.sky}>{`T+${elapsed()}`}</text>
            </box>
          )}
        </Show>
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
      elapsed: sessionElapsed(),
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
    }
    if (!scope || !scope.project || pendingGeneration === generation) return;
    const requestGeneration = generation;
    pendingGeneration = requestGeneration;
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
          setSnapshot(EMPTY_SNAPSHOT);
          setSnapshotError(error.message);
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
          setSnapshot(next);
          setSnapshotError("");
        } catch (parseError) {
          setSnapshot(EMPTY_SNAPSHOT);
          setSnapshotError(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
        }
      }
    );
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

  const cleanup = () => {
    if (disposed) return;
    disposed = true;
    clearInterval(snapshotPoll);
    clearInterval(statsPoll);
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
        return (
          <Show when={!panel?.name || panel?.name === "cortex.dashboard" || panel?.name === "session.panel" || panel?.name === "cortex.board" || panel?.name === "cortex"}>
            <SessionKanbanPanel
              snapshot={snapshot}
              jobs={jobs}
              now={now}
              spinner={spinner}
              pulse={pulse}
              theme={panel?.theme?.current || panel?.theme || api.theme}
            />
          </Show>
        );
      },
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
