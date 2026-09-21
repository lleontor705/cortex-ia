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
import type { BoxRenderable, ColorInput } from "@opentui/core";

const SNAPSHOT_POLL_INTERVAL_MS = 2500;
const SNAPSHOT_STALE_MS = 10_000;
const MAX_VISIBLE_ROWS = 4;
const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const NEURAL_PULSE_FRAMES = ["◈", "◇", "◆", "◇"];

const TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
const DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
const ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
const ETA_HISTORY_KEY = "cortex.dashboard.eta.completions";
const ETA_HISTORY_LIMIT = 12;
const ETA_INTERVAL_WINDOW = 8;
const ETA_MIN_SAMPLES = 2;

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

function meanCompletionInterval(stamps: readonly number[]): number | undefined {
  if (stamps.length < ETA_MIN_SAMPLES) return undefined;
  const intervals: number[] = [];
  for (let i = 1; i < stamps.length; i += 1) {
    const delta = stamps[i] - stamps[i - 1];
    if (delta > 0) intervals.push(delta);
  }
  if (intervals.length === 0) return undefined;
  const recent = intervals.slice(-ETA_INTERVAL_WINDOW);
  return recent.reduce((sum, value) => sum + value, 0) / recent.length;
}

function roleChip(role: string, palette: CortexPalette): { icon: string; color: ColorInput; tag: string } {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return { icon: "🧠", color: palette.primary, tag: "ORCH" };
  if (r.includes("impl")) return { icon: "⚡", color: palette.warning, tag: "IMPL" };
  if (r.includes("rev")) return { icon: "⚖️", color: palette.accent, tag: "REVW" };
  if (r.includes("inv")) return { icon: "🔍", color: palette.sky, tag: "INVS" };
  if (r.includes("plan")) return { icon: "📋", color: palette.info, tag: "PLAN" };
  if (r.includes("disc")) return { icon: "🧭", color: palette.success, tag: "DISC" };
  return { icon: "🤖", color: palette.textMuted, tag: r.slice(0, 4).toUpperCase() || "WORK" };
}

function taskStatusChip(status: string, palette: CortexPalette): { icon: string; color: ColorInput; tag: string } {
  switch (status) {
    case "done":
      return { icon: "✓", color: palette.success, tag: "DONE" };
    case "in_progress":
      return { icon: "⚡", color: palette.warning, tag: "PROG" };
    case "in_review":
      return { icon: "◆", color: palette.accent, tag: "REVW" };
    case "ready":
      return { icon: "▶", color: palette.sky, tag: "RDY " };
    case "blocked":
      return { icon: "✕", color: palette.error, tag: "BLCK" };
    case "superseded":
      return { icon: "↷", color: palette.textMuted, tag: "SPRS" };
    case "backlog":
    default:
      return { icon: "○", color: palette.textMuted, tag: "WAIT" };
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
      title: "Snapshot no disponible",
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
      title={props.isExecuting() ? `🧠 CORTEX·IA v2.0 [${props.spinner()} ACTIVO]` : "🧠 CORTEX·IA v2.0 [● STANDBY]"}
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.panel}
      paddingLeft={1}
      paddingRight={1}
    >
      {/* Project Root on its own dedicated line to prevent word wrapping */}
      <Show when={projectName()}>
        <box flexDirection="row" marginTop={0}>
          <text fg={palette.sky}>📁 </text>
          <text fg={palette.sky}>{clipped(projectName(), Math.max(6, props.textLimit - 4))}</text>
        </box>
      </Show>

      <Show when={props.sessionElapsed?.()}>
        {(elapsed: () => string) => (
          <box flexDirection="row" marginTop={0}>
            <text fg={palette.info}>{"⏱ "}</text>
            <text fg={palette.sky}>{clipped(`Sesión ${elapsed()}`, Math.max(8, props.textLimit))}</text>
          </box>
        )}
      </Show>

      <box flexDirection="row" marginTop={0}>
        <text fg={palette.info} onMouseDown={() => openWebConsole()} selectable={false}>
          {"[🌐 Web]"}
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
            <text fg={palette.textMuted}>○ 0 curso · 0 revw · 0 done · 0 alrt</text>
          </box>
        }
      >
        {/* Row 1 */}
        <box flexDirection="row">
          <text fg={props.activeExecutions > 0 ? palette.warning : palette.textMuted}>
            {`[${props.activeExecutions > 0 ? props.spinner() : "●"} ${props.activeExecutions} CURSO] `}
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
  icon?: string;
  badge?: string;
  shortBadge?: string;
  compact?: boolean;
  expanded: () => boolean;
  onToggle: () => void;
  theme?: TuiThemeCurrent;
  children: unknown;
}) {
  const displayTitle = createMemo(() => (props.compact && props.shortTitle ? props.shortTitle : props.title));
  const displayBadge = createMemo(() => (props.compact && props.shortBadge ? props.shortBadge : props.badge));
  const palette = resolvePalette(props.theme);

  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row" onMouseDown={props.onToggle}>
        <text fg={props.expanded() ? palette.info : palette.textMuted} selectable={false}>
          {props.expanded() ? "▼ " : "▶ "}
        </text>
        <Show when={props.icon}>
          <text fg={palette.accentAlt}>{`${props.icon} `}</text>
        </Show>
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
    return diff > 0 ? formatDuration(diff) : "expirado";
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
      title={clipped(`${props.spinner()} ⚡ TAREA EN EJECUCIÓN`, props.textLimit)}
      titleColor={palette.warning}
      titleAlignment="left"
      backgroundColor={palette.panel}
    >
      {/* Task ID & Title */}
      <box flexDirection="row">
        <text fg={palette.sky}>🎯 </text>
        <text fg={palette.text}>{clipped(`${props.task.task_id} · ${props.task.title}`, props.textLimit - 3)}</text>
      </box>

      {/* Metrics Row: Elapsed + Lease */}
      <box flexDirection="row">
        <text fg={palette.textMuted}>  ⏱ </text>
        <text fg={palette.warning}>{`+${elapsed()} `}</text>
        <Show when={ttlRemaining()}>
          {(ttl: () => string) => (
            <text fg={ttl() === "expirado" ? palette.error : palette.sky}>
              {`│ 🛡 TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`}
            </text>
          )}
        </Show>
      </box>

      {/* Worker / Delegation Info */}
      <box flexDirection="row">
        <Show
          when={props.activeDelegation}
          fallback={
            <text fg={palette.accent}>
              {clipped(`  📋 Tarea durable${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit)}
            </text>
          }
        >
          {(del: () => DelegationJob) => (
            <text fg={palette.info}>
              {`  🤖 AGY ${del().transport || "direct"}${del().pane_id ? ` · ${del().pane_id}` : ""}${del().attempt ? ` · int #${del().attempt}` : ""}`}
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
          {"  [ 🌐 Ver detalle en Web ]"}
        </text>
      </box>
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
    <Show when={props.tasks.length > 0} fallback={<text fg={palette.textMuted}>  ○ Sin tareas en cola</text>}>
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
                  {" [🌐]"}
                </text>
              </box>
              <box flexDirection="row">
                <text fg={palette.textMuted}>
                  {`     ${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` · ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` · 🛡 ${task.lease_count}lk` : ""}`}
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
    <Show when={props.jobs.length > 0} fallback={<text fg={palette.textMuted}>  ○ Sin workers activos</text>}>
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
                <text fg={chip.color}>{`${chip.icon} [${chip.tag}] `}</text>
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
    <Show when={props.items.length > 0} fallback={<text fg={palette.success}>  ✓ Sin alertas pendientes</text>}>
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
      if (eta.remaining === 0) return `⏳ listo${shortSuffix}`;
      if (eta.estimateMs === undefined) return `⏳ est… (${eta.remaining})${shortSuffix}`;
      return `⏳ ${formatDuration(eta.estimateMs)} (${eta.remaining})${shortSuffix}`;
    }
    if (eta.remaining === 0) return `⏳ ETA: tablero completo${suffix}`;
    if (eta.estimateMs === undefined) return `⏳ ETA: estimando (${eta.remaining} pend)${suffix}`;
    return `⏳ ETA: ${formatDuration(eta.estimateMs)} · ${eta.remaining} pend${suffix}`;
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
      title={props.layout.compact ? "🧠 CONTROL" : "🧠 CONTROL MATRIX ◈"}
      titleColor={palette.accent}
      titleAlignment="left"
      backgroundColor={palette.panel}
    >
      {/* Synapse Pulse / Status Line */}
      <box flexDirection="row">
        <Show
          when={activeJobs() > 0}
          fallback={
            <text fg={palette.success}>
              {`  ${props.pulse()} SYNAPSE: SINCRONIZADO`}
            </text>
          }
        >
          <text fg={palette.warning}>
            {`  ${props.spinner()} SYNAPSE: MOTOR ACTIVO`}
          </text>
        </Show>
        <text
          fg={palette.info}
          onMouseDown={() => openWebConsole(props.snapshot.tasks[0]?.board_id)}
          selectable={false}
        >
          {"  [🌐 Web]"}
        </text>
      </box>

      {/* Grid: Pills only when metrics exist, otherwise clean standby indicator */}
      <Show
        when={hasMetrics()}
        fallback={
          <box flexDirection="row">
            <text fg={palette.textMuted}>  Standby · 0 ejecuciones</text>
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
              <text fg={totalLeases() > 0 ? palette.sky : palette.textMuted}>{`🛡${totalLeases()}`}</text>
              <Show when={blockedTasks() > 0}>
                <text fg={palette.error}>{` ✕${blockedTasks()}b`}</text>
              </Show>
            </box>
          }
        >
          <box flexDirection="row">
            <text fg={palette.success}>{`[ ✓ ${succeededJobs()} ÉXITO ] `}</text>
            <text fg={failedJobs() > 0 ? palette.error : palette.textMuted}>
              {`[ ✕ ${failedJobs()} FALLO ]`}
            </text>
          </box>
          <box flexDirection="row">
            <text fg={activeJobs() > 0 ? palette.warning : palette.textMuted}>
              {`[ ${activeJobs() > 0 ? props.spinner() : "●"} ${activeJobs()} CURSO ] `}
            </text>
            <text fg={totalLeases() > 0 ? palette.sky : palette.textMuted}>
              {`[ 🛡 ${totalLeases()} LOCKS ] `}
            </text>
            <Show when={blockedTasks() > 0}>
              <text fg={palette.error}>{`[ ✕ ${blockedTasks()} BLCK ]`}</text>
            </Show>
          </box>
        </Show>
      </Show>

      {/* Micro Medidor de Salud Operativa */}
      <box flexDirection="row">
        <text fg={palette.textMuted}>Salud: </text>
        <Show
          when={successRate() !== undefined}
          fallback={<text fg={palette.textMuted}>○ standby</text>}
        >
          <text fg={healthColor()}>{healthBars().filled}</text>
          <text fg={palette.border}>{healthBars().empty}</text>
          <text fg={healthColor()}>{` ${successRate()}%`}</text>
        </Show>
      </box>

      {/* Autoridad SQLite y DAG */}
      <box flexDirection="row">
        <text fg={palette.textMuted}>
          {props.layout.compact ? "Autoridad: SQLite" : `📋 DAG: ${doneTasks()}/${totalTasks()} · Autoridad: SQLite`}
        </text>
      </box>

      <Show when={totalTasks() > 0}>
        <box flexDirection="row">
          <text fg={etaColor()}>{etaLabel()}</text>
        </box>
      </Show>

      {/* Frescura de Datos / Telemetría */}
      <box flexDirection="row">
        <Show
          when={props.stale}
          fallback={
            <text fg={palette.success}>
              {props.layout.compact ? "🟢 En vivo" : `🟢 En vivo · Sync hace ${syncAgeSec()}s`}
            </text>
          }
        >
          <text fg={palette.warning}>
            {props.layout.compact ? "🟡 Desfasado" : `🟡 Snapshot desfasado (+${syncAgeSec()}s)`}
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
      {/* 1. Header Cockpit con Brand Logo & Estilo Cortex */}
      <CortexCockpitHeader
        isExecuting={isExecuting}
        nativeActivity={props.nativeActivity}
        projectRoot={props.snapshot().project_root}
        sessionElapsed={props.sessionElapsed}
        textLimit={layout().textLimit}
        spinner={props.spinner}
        pulse={props.pulse}
        theme={props.theme}
      />

      <Show when={!props.scopeReady()}>
        <text fg={palette.warning} marginTop={1}>Conversación no disponible · esperando metadatos</text>
      </Show>

      <Show when={props.scopeReady() && !props.snapshot().generated_at && !props.snapshotError()}>
        <text fg={palette.textMuted} marginTop={1}>Cargando estado de la conversación…</text>
      </Show>

      <Show when={props.snapshotError()}>
        <text fg={palette.error} marginTop={1}>No se pudo actualizar · datos no confirmados</text>
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
            ? `${props.spinner()} OpenCode: ocupado`
            : props.nativeActivity() === "idle"
            ? "● OpenCode: en espera"
            : `○ OpenCode: ${props.nativeActivity() || "sin estado"}`}
        </text>
      </box>

      <Show when={props.scopeReady() && Boolean(props.snapshot().generated_at)}>
        {/* 2. Micro-HUD de Métricas Operacionales */}
        <OperationalKPIHud
          activeExecutions={activeExecutionsCount()}
          inReview={counts().review}
          doneTasks={doneTasks()}
          attentionCount={counts().attention}
          spinner={props.spinner}
          theme={props.theme}
        />

        {/* 3. Barra de Progreso Multi-Color del Tablero DAG */}
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

        {/* 4. Tarjeta Hero de Tarea Activa (si hay tarea en curso) */}
        <Show when={activeTask()}>
          {(task: () => DashboardTask) => (
            <ActiveTaskHero
              task={task()}
              activeDelegation={activeDelegation()}
              now={props.now}
              spinner={props.spinner}
              textLimit={layout().textLimit}
              theme={props.theme}
            />
          )}
        </Show>

        {/* 5. Sección: Tablero de Tareas */}
        <Section
          title="Tablero de Tareas"
          shortTitle="Tareas"
          icon="📋"
          badge={totalTasks() > 0 ? `${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot` : undefined}
          shortBadge={totalTasks() > 0 ? `${props.snapshot().summary.active_tasks || 0}/${totalTasks()}` : undefined}
          compact={layout().compact}
          expanded={props.tasksExpanded}
          onToggle={props.toggleTasks}
          theme={props.theme}
        >
          <TaskRows tasks={props.snapshot().tasks} spinner={props.spinner} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>

        {/* 6. Sección: Ejecuciones & Workers */}
        <Section
          title="Workers & Delegación"
          shortTitle="Workers"
          icon="🤖"
          badge={totalDelegationsCount() > 0 ? `${activeDelegationsCount()} act / ${totalDelegationsCount()} tot` : undefined}
          shortBadge={totalDelegationsCount() > 0 ? `${activeDelegationsCount()}/${totalDelegationsCount()}` : undefined}
          compact={layout().compact}
          expanded={props.delegationsExpanded}
          onToggle={props.toggleDelegations}
          theme={props.theme}
        >
          <DelegationRows jobs={props.jobs()} spinner={props.spinner} now={props.now} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>

        {/* 7. Sección: Centro de Atención & Alertas */}
        <Section
          title="Centro de Atención"
          shortTitle="Alertas"
          icon="⚠️"
          badge={counts().attention > 0 ? `${counts().attention} alertas` : undefined}
          shortBadge={counts().attention > 0 ? `${counts().attention}` : undefined}
          compact={layout().compact}
          expanded={props.attentionExpanded}
          onToggle={props.toggleAttention}
          theme={props.theme}
        >
          <AttentionRows items={attention()} theme={props.theme} compact={layout().compact} textLimit={layout().textLimit} />
        </Section>

        {/* 8. Panel de Control Matrix y Contadores Inferiores */}
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
  const gaugeSegments = createMemo(() => (dimensions().width < 96 ? 6 : 10));
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
              <text fg={palette.info}>{"⏱ "}</text>
              <text fg={palette.sky}>{elapsed()}</text>
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
        <text fg={palette.accentAlt}>🧠 </text>
        <text fg={palette.text}>CORTEX</text>
        <text fg={palette.info}>·</text>
        <text fg={palette.sky}>IA </text>
        <text fg={palette.border}>│ </text>
        <Show
          when={activeTask()}
          fallback={
            <box flexDirection="row">
              <text fg={palette.warning}>{`● ${counts().active} en curso`}</text>
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
              <text fg={palette.textMuted}>{` · ${counts().active} activos`}</text>
              <text fg={palette.info}>{" [🌐]"}</text>
            </box>
          )}
        </Show>
      </box>
    </Show>
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
  const readyTasks = createMemo(() => tasks().filter((t) => t.status === "ready"));
  const inProgressTasks = createMemo(() => tasks().filter((t) => t.status === "in_progress"));
  const inReviewTasks = createMemo(() => tasks().filter((t) => t.status === "in_review"));
  const blockedTasks = createMemo(() => tasks().filter((t) => t.status === "blocked"));
  const palette = resolvePalette(props.theme);

  return (
    <box flexDirection="column" padding={1}>
      <box flexDirection="row" marginBottom={1}>
        <text fg={palette.info} bold={true}>
          {`◈ CORTEX · IA KANBAN DECK [${props.pulse()}] `}
        </text>
        <text fg={palette.textMuted}>
          {`(${tasks().length} tareas · ${props.jobs().length} workers)`}
        </text>
      </box>

      {/* Columnas Kanban */}
      <box flexDirection="row">
        {/* Columna: En Curso */}
        <box flexDirection="column" width={26} marginRight={1}>
          <text fg={palette.warning} bold={true}>
            {`⚡ EN CURSO (${inProgressTasks().length})`}
          </text>
          <For each={inProgressTasks()}>
            {(task) => (
              <box flexDirection="column" marginTop={1}>
                <text fg={palette.text} bold={true}>{`● ${task.task_id}`}</text>
                <text fg={palette.textSoft}>{clipped(task.title, 22)}</text>
                <Show when={task.owner}>
                  <text fg={palette.info}>{`Claim: ${clipped(task.owner!, 14)}`}</text>
                </Show>
              </box>
            )}
          </For>
        </box>

        {/* Columna: En Revisión */}
        <box flexDirection="column" width={26} marginRight={1}>
          <text fg={palette.accent} bold={true}>
            {`⚖ EN REVISIÓN (${inReviewTasks().length})`}
          </text>
          <For each={inReviewTasks()}>
            {(task) => (
              <box flexDirection="column" marginTop={1}>
                <text fg={palette.text} bold={true}>{`◆ ${task.task_id}`}</text>
                <text fg={palette.textSoft}>{clipped(task.title, 22)}</text>
                <text fg={palette.warning}>esperando reviewer</text>
              </box>
            )}
          </For>
        </box>

        {/* Columna: Bloqueadas & Listas */}
        <box flexDirection="column" width={26}>
          <text fg={palette.success} bold={true}>
            {`✓ LISTAS (${readyTasks().length}) / ✕ BLQ (${blockedTasks().length})`}
          </text>
          <For each={blockedTasks()}>
            {(task) => (
              <box flexDirection="column" marginTop={1}>
                <text fg={palette.error} bold={true}>{`✕ ${task.task_id}`}</text>
                <text fg={palette.error}>{clipped(task.title, 22)}</text>
              </box>
            )}
          </For>
          <For each={readyTasks().slice(0, 3)}>
            {(task) => (
              <box flexDirection="column" marginTop={1}>
                <text fg={palette.sky} bold={true}>{`○ ${task.task_id}`}</text>
                <text fg={palette.textMuted}>{clipped(task.title, 22)}</text>
              </box>
            )}
          </For>
        </box>
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
            <text fg={palette.info} bold={true}>
              {"◈ CORTEX · IA ◈"}
            </text>
            <text fg={palette.textMuted}>
              {"[Adaptive Cognitive Control Plane]"}
            </text>
          </box>
        }
      >
        <For each={CORTEX_LOGO_BRAILLE}>
          {(line, index) => {
            const color =
              index() < 4
                ? palette.accentAlt
                : index() < 9
                ? palette.info
                : index() < 12
                ? palette.sky
                : palette.accent;
            return <text fg={color}>{line}</text>;
          }}
        </For>
        <text fg={palette.textMuted} marginTop={1}>
          {"⚡ OpenCode Multi-Agent Control Plane & Task DAG ⚡"}
        </text>
      </Show>
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
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [pulseFrame, setPulseFrame] = createSignal(0);

  const getPref = (key: string, fallback: boolean): boolean => {
    if (api.kv && typeof api.kv.get === "function") {
      return api.kv.get(key, fallback) !== false;
    }
    return fallback;
  };

  const setPref = (key: string, val: boolean): void => {
    if (api.kv && typeof api.kv.set === "function") {
      api.kv.set(key, val);
    }
  };

  const [tasksExpanded, setTasksExpanded] = createSignal(getPref(TASKS_EXPANDED_KEY, true));
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(
    getPref(DELEGATIONS_EXPANDED_KEY, true)
  );
  const [attentionExpanded, setAttentionExpanded] = createSignal(
    getPref(ATTENTION_EXPANDED_KEY, true)
  );

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
    if (count > previousAttentionCount) {
      setAttentionExpanded(true);
      setPref(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });

  let previousTaskStatuses = new Map<string, string>();
  createEffect(() => {
    const tasks = snapshot().tasks;
    const toastApi = (api as any).ui?.toast || (api as any).toast;
    if (toastApi && previousTaskStatuses.size > 0) {
      for (const t of tasks) {
        const prev = previousTaskStatuses.get(t.task_id);
        if (prev && prev !== t.status) {
          let variant = "info";
          let title = `Cortex-IA: Tarea ${t.task_id}`;
          if (t.status === "done") {
            variant = "success";
            title = `✓ ${t.task_id} Aprobada (PASS)`;
          } else if (t.status === "blocked") {
            variant = "error";
            title = `✕ ${t.task_id} Bloqueada`;
          } else if (t.status === "in_review") {
            variant = "warning";
            title = `⚖ ${t.task_id} En Revisión`;
          } else if (t.status === "in_progress") {
            variant = "info";
            title = `⚡ ${t.task_id} Reclamada`;
          }
          try {
            if (typeof toastApi.show === "function") {
              toastApi.show({
                title,
                message: clipped(t.title, 40),
                variant,
              });
            } else if (typeof toastApi === "function") {
              toastApi({
                title,
                message: clipped(t.title, 40),
                variant,
              });
            }
          } catch {}
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

  const cleanup = () => {
    if (disposed) return;
    disposed = true;
    clearInterval(snapshotPoll);
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
      render: (ctx: any) => <HomeLogo theme={ctx?.theme?.current || ctx?.theme || api.theme} />,
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

    if (api.keymap && typeof api.keymap.layer === "function") {
      api.ui.slot({
        append: "app",
        render: () => {
          api.keymap.layer(() => ({
            mode: "global",
            commands: [
              {
                id: "cortex.board",
                title: "Cortex Board & Delegation",
                slash: { name: "cortex" },
                run: () => {
                  if (api.ui?.panel?.open) {
                    api.ui.panel.open("cortex.board");
                  }
                },
              },
            ],
          }));
          return null;
        },
      });
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
        <HomeBottomStatus
          snapshot={snapshot}
          jobs={jobs}
          spinner={spinner}
          snapshotError={snapshotError}
          theme={ctx?.theme?.current || ctx?.theme || api.theme}
        />
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
