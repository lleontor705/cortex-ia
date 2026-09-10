import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
  TuiThemeCurrent,
} from "@opencode-ai/plugin/tui";
import { execFile } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import {
  For,
  Show,
  createEffect,
  createMemo,
  createRoot,
  createSignal,
} from "solid-js";
import type { BoxRenderable } from "@opentui/core";

const SNAPSHOT_POLL_INTERVAL_MS = 2500;
const SNAPSHOT_STALE_MS = 10_000;
const MAX_VISIBLE_ROWS = 4;
const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const NEURAL_PULSE_FRAMES = ["◈", "◇", "◆", "◇"];

const TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
const DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
const ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";

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

function sidebarLayout(width: number): SidebarLayout {
  const measured = Number.isFinite(width) && width > 0 ? Math.floor(width) : 0;
  return {
    compact: measured === 0 || measured < 32,
    textLimit: Math.max(8, measured ? measured - 6 : 18),
    gaugeWidth: Math.max(3, Math.min(14, measured ? measured - 14 : 6)),
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

function roleChip(role: string): { icon: string; color: string; tag: string } {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return { icon: "🧠", color: CORTEX_THEME.brandIndigo, tag: "ORCH" };
  if (r.includes("impl")) return { icon: "⚡", color: CORTEX_THEME.amberGold, tag: "IMPL" };
  if (r.includes("rev")) return { icon: "⚖️", color: CORTEX_THEME.brandPurple, tag: "REVW" };
  if (r.includes("inv")) return { icon: "🔍", color: CORTEX_THEME.skyBlue, tag: "INVS" };
  if (r.includes("plan")) return { icon: "📋", color: CORTEX_THEME.neonCyan, tag: "PLAN" };
  if (r.includes("disc")) return { icon: "🧭", color: CORTEX_THEME.emeraldGreen, tag: "DISC" };
  return { icon: "🤖", color: CORTEX_THEME.slateMuted, tag: r.slice(0, 4).toUpperCase() || "WORK" };
}

function taskStatusChip(status: string): { icon: string; color: string; tag: string } {
  switch (status) {
    case "done":
      return { icon: "✓", color: CORTEX_THEME.emeraldGreen, tag: "DONE" };
    case "in_progress":
      return { icon: "⚡", color: CORTEX_THEME.amberGold, tag: "PROG" };
    case "in_review":
      return { icon: "◆", color: CORTEX_THEME.brandPurple, tag: "REVW" };
    case "ready":
      return { icon: "▶", color: CORTEX_THEME.skyBlue, tag: "RDY " };
    case "blocked":
      return { icon: "✕", color: CORTEX_THEME.roseRed, tag: "BLCK" };
    case "superseded":
      return { icon: "↷", color: CORTEX_THEME.slateMuted, tag: "SPRS" };
    case "backlog":
    default:
      return { icon: "○", color: CORTEX_THEME.slateMuted, tag: "WAIT" };
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

function currentSessionID(api: TuiPluginApi): string | undefined {
  const route = api.route.current;
  const id = route.name === "session" ? route.params?.sessionID : undefined;
  return typeof id === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(id) ? id : undefined;
}

function nativeSessionActivity(api: TuiPluginApi): NativeActivity | undefined {
  const id = currentSessionID(api);
  if (!id) return undefined;
  if (api.state.session.get(id)?.id !== id) return "unknown";
  const status = api.state.session.status(id)?.type;
  return status === "busy" || status === "idle" || status === "retry" ? status : "unknown";
}

function conversationScope(api: TuiPluginApi) {
  const sessionID = currentSessionID(api);
  if (!sessionID) return undefined;
  let current = sessionID;
  const seen = new Set<string>();
  while (seen.size < 64 && /^[A-Za-z0-9_-]{1,256}$/.test(current) && !seen.has(current)) {
    seen.add(current);
    const session = api.state.session.get(current);
    if (!session || session.id !== current) return undefined;
    if (!session.parentID) return { sessionID, rootSessionID: current, project: api.state.path.directory };
    current = session.parentID;
  }
  return undefined;
}

function CortexCockpitHeader(props: {
  isExecuting: () => boolean;
  nativeActivity: () => NativeActivity | undefined;
  projectRoot?: string;
  spinner: () => string;
  pulse: () => string;
  theme: TuiThemeCurrent;
}) {
  return (
    <box
      flexDirection="column"
      borderStyle="rounded"
      borderColor={props.isExecuting() ? CORTEX_THEME.amberGold : CORTEX_THEME.brandIndigo}
      paddingLeft={1}
      paddingRight={1}
    >
      {/* Brand Title + Pulse Badge */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.brandViolet}>🧠 </text>
        <text fg={CORTEX_THEME.pureWhite}>CORTEX</text>
        <text fg={CORTEX_THEME.neonCyan}>·</text>
        <text fg={CORTEX_THEME.skyBlue}>IA </text>
        <text fg={CORTEX_THEME.brandPurple}>v2.0 </text>
        <Show
          when={props.isExecuting()}
          fallback={<text fg={CORTEX_THEME.emeraldGreen}>[● STANDBY]</text>}
        >
          <text fg={CORTEX_THEME.amberGold}>{`[${props.spinner()} NEURAL ACT]`}</text>
        </Show>
      </box>

      {/* Subtitle & Project Root */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.slateMuted}>Neural Control Bridge </text>
        <Show when={props.projectRoot}>
          <text fg={CORTEX_THEME.slateBorder}>│ </text>
          <text fg={CORTEX_THEME.skyBlue}>{clipped(path.basename(props.projectRoot!), 13)}</text>
        </Show>
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
  theme: TuiThemeCurrent;
}) {
  return (
    <box flexDirection="column" marginTop={1}>
      {/* Row 1 */}
      <box flexDirection="row">
        <text fg={props.activeExecutions > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted}>
          {`[ ${props.activeExecutions > 0 ? props.spinner() : "●"} ${props.activeExecutions} CURSO ] `}
        </text>
        <text fg={props.inReview > 0 ? CORTEX_THEME.brandPurple : CORTEX_THEME.slateMuted}>
          {`[ ◆ ${props.inReview} REVW ]`}
        </text>
      </box>
      {/* Row 2 */}
      <box flexDirection="row" marginTop={0}>
        <text fg={props.doneTasks > 0 ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.slateMuted}>
          {`[ ✓ ${props.doneTasks} DONE ] `}
        </text>
        <text fg={props.attentionCount > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted}>
          {`[ ${props.attentionCount > 0 ? "✕" : "○"} ${props.attentionCount} ALRT ]`}
        </text>
      </box>
    </box>
  );
}

function Section(props: {
  title: string;
  icon?: string;
  badge?: string;
  expanded: () => boolean;
  onToggle: () => void;
  theme: TuiThemeCurrent;
  children: unknown;
}) {
  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row" onMouseDown={props.onToggle}>
        <text fg={props.expanded() ? CORTEX_THEME.neonCyan : CORTEX_THEME.slateMuted} selectable={false}>
          {props.expanded() ? "▼ " : "▶ "}
        </text>
        <Show when={props.icon}>
          <text fg={CORTEX_THEME.brandViolet}>{`${props.icon} `}</text>
        </Show>
        <text fg={CORTEX_THEME.pureWhite} selectable={false}>
          {props.title}
        </text>
        <Show when={props.badge}>
          <text fg={CORTEX_THEME.skyBlue}>{` [ ${props.badge} ]`}</text>
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
  total: number;
  width?: number;
  theme: TuiThemeCurrent;
}) {
  const w = props.width || 14;
  const total = Math.max(props.total, 1);
  const doneW = Math.round((props.done / total) * w);
  const revW = Math.round((props.inReview / total) * w);
  const progW = Math.round((props.inProgress / total) * w);
  const emptyW = Math.max(0, w - doneW - revW - progW);
  const pct = Math.round((props.done / total) * 100);

  return (
    <box flexDirection="column" marginTop={1}>
      <box flexDirection="row">
        <text fg={CORTEX_THEME.neonCyan}>DAG: </text>
        <text fg={CORTEX_THEME.emeraldGreen}>{"█".repeat(doneW)}</text>
        <text fg={CORTEX_THEME.brandPurple}>{"▓".repeat(revW)}</text>
        <text fg={CORTEX_THEME.amberGold}>{"▒".repeat(progW)}</text>
        <text fg={CORTEX_THEME.slateBorder}>{"░".repeat(emptyW)}</text>
        <text fg={CORTEX_THEME.pureWhite}>{` ${pct}%`}</text>
        <text fg={CORTEX_THEME.slateMuted}>{` (${props.done}/${props.total})`}</text>
      </box>
      <box flexDirection="row">
        <text fg={CORTEX_THEME.emeraldGreen}>{`✓${props.done} `}</text>
        <text fg={CORTEX_THEME.brandPurple}>{`◆${props.inReview} `}</text>
        <text fg={CORTEX_THEME.amberGold}>{`●${props.inProgress} `}</text>
        <text fg={CORTEX_THEME.slateMuted}>
          {`○${Math.max(0, props.total - props.done - props.inReview - props.inProgress)}`}
        </text>
      </box>
    </box>
  );
}

function ActiveTaskHero(props: {
  task: DashboardTask;
  activeDelegation?: DelegationJob;
  now: () => number;
  spinner: () => string;
  theme: TuiThemeCurrent;
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

  return (
    <box
      flexDirection="column"
      marginTop={1}
      paddingLeft={1}
      paddingRight={1}
      borderStyle="rounded"
      borderColor={CORTEX_THEME.amberGold}
    >
      {/* Header Tag */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.amberGold}>{`${props.spinner()} ⚡ TAREA EN EJECUCIÓN`}</text>
      </box>

      {/* Task ID & Title */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.skyBlue}>🎯 </text>
        <text fg={CORTEX_THEME.pureWhite}>{clipped(`${props.task.task_id} · ${props.task.title}`, 26)}</text>
      </box>

      {/* Metrics Row: Elapsed + Lease */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.slateMuted}>  ⏱ </text>
        <text fg={CORTEX_THEME.amberGold}>{`+${elapsed()} `}</text>
        <Show when={ttlRemaining()}>
          {(ttl) => (
            <text fg={ttl() === "expirado" ? CORTEX_THEME.roseRed : CORTEX_THEME.skyBlue}>
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
            <text fg={CORTEX_THEME.brandPurple}>
              {`  🧑‍💻 OpenCode Nativo${props.task.owner ? ` (${clipped(props.task.owner, 10)})` : ""}`}
            </text>
          }
        >
          {(del) => (
            <text fg={CORTEX_THEME.neonCyan}>
              {`  🤖 AGY ${del().transport || "direct"}${del().pane_id ? ` · ${del().pane_id}` : ""}${del().attempt ? ` · int #${del().attempt}` : ""}`}
            </text>
          )}
        </Show>
      </box>
    </box>
  );
}

function TaskRows(props: {
  tasks: DashboardTask[];
  spinner: () => string;
  theme: TuiThemeCurrent;
}) {
  return (
    <Show when={props.tasks.length > 0} fallback={<text fg={CORTEX_THEME.slateMuted}>  Sin tareas durables activas</text>}>
      <For each={props.tasks.slice(0, MAX_VISIBLE_ROWS)}>
        {(task) => {
          const chip = taskStatusChip(task.status);
          const isProg = task.status === "in_progress";
          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row">
                <text fg={chip.color}>
                  {`  ${isProg ? props.spinner() : chip.icon} `}
                </text>
                <text fg={chip.color}>{`[${chip.tag}] `}</text>
                <text fg={isProg ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight}>
                  {clipped(`${task.task_id} · ${task.title}`, 24)}
                </text>
              </box>
              <box flexDirection="row">
                <text fg={CORTEX_THEME.slateMuted}>
                  {`     ${clipped(task.board_id, 10)}${task.owner ? ` · ${clipped(task.owner, 8)}` : ""}${task.lease_count ? ` · 🛡 ${task.lease_count}lk` : ""}`}
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
  theme: TuiThemeCurrent;
}) {
  return (
    <Show when={props.jobs.length > 0} fallback={<text fg={CORTEX_THEME.slateMuted}>  Sin ejecuciones delegadas</text>}>
      <For each={props.jobs.slice(0, MAX_VISIBLE_ROWS)}>
        {(job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const chip = roleChip(job.role);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` +${formatDuration(props.now() - t)}` : "";
          });
          const statusCol = isRunning
            ? CORTEX_THEME.amberGold
            : job.status === "succeeded"
            ? CORTEX_THEME.emeraldGreen
            : CORTEX_THEME.roseRed;

          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row">
                <text fg={statusCol}>
                  {`  ${isRunning ? props.spinner() : job.status === "succeeded" ? "✓" : "✕"} `}
                </text>
                <text fg={chip.color}>{`${chip.icon} [${chip.tag}] `}</text>
                <text fg={isRunning ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight}>
                  {job.role || "worker"}
                </text>
                <text fg={statusCol}>{elapsed()}</text>
              </box>
              <box flexDirection="row">
                <text fg={CORTEX_THEME.slateMuted}>
                  {`     ${shortID(job.job_id)} · ${job.transport || "direct"}${job.pane_id ? ` · ${job.pane_id}` : ""}${job.attempt ? ` · int #${job.attempt}` : ""}`}
                </text>
              </box>
            </box>
          );
        }}
      </For>
    </Show>
  );
}

function AttentionRows(props: { items: AttentionItem[]; theme: TuiThemeCurrent }) {
  return (
    <Show when={props.items.length > 0} fallback={<text fg={CORTEX_THEME.slateMuted}>  Sin alertas</text>}>
      <For each={props.items.slice(0, MAX_VISIBLE_ROWS)}>
        {(item) => (
          <box flexDirection="column">
            <box flexDirection="row">
              <text fg={CORTEX_THEME.roseRed}>{`  ✕ `}</text>
              <text fg={CORTEX_THEME.pureWhite}>{clipped(item.title, 26)}</text>
            </box>
            <text fg={CORTEX_THEME.slateMuted}>{`     ${item.detail}`}</text>
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
  now: () => number;
  spinner: () => string;
  pulse: () => string;
  theme: TuiThemeCurrent;
  layout: SidebarLayout;
}) {
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
    const rate = successRate() ?? 0;
    if (rate >= 90) return CORTEX_THEME.emeraldGreen;
    if (rate >= 70) return CORTEX_THEME.neonCyan;
    if (rate >= 50) return CORTEX_THEME.amberGold;
    return CORTEX_THEME.roseRed;
  });

  return (
    <box
      flexDirection="column"
      marginTop={1}
      paddingLeft={1}
      paddingRight={1}
      borderStyle="rounded"
      borderColor={failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.brandIndigo}
    >
      {/* Panel Header */}
      <box flexDirection="row">
        <text fg={CORTEX_THEME.brandViolet}>🧠 </text>
        <text fg={CORTEX_THEME.pureWhite}>{props.layout.compact ? "CONTROL" : "CONTROL MATRIX "}</text>
        <text fg={CORTEX_THEME.neonCyan}>◈</text>
      </box>

      {/* Synapse Pulse / Status Line */}
      <Show when={!props.layout.compact}><box flexDirection="row">
        <Show
          when={activeJobs() > 0}
          fallback={
            <text fg={CORTEX_THEME.emeraldGreen}>
              {`  ${props.pulse()} SYNAPSE: SINCRONIZADO`}
            </text>
          }
        >
          <text fg={CORTEX_THEME.amberGold}>
            {`  ${props.spinner()} SYNAPSE: MOTOR ACTIVO`}
          </text>
        </Show>
      </box></Show>

      {/* Grid Fila 1: Píldoras de Éxito y Fallos */}
      <Show when={!props.layout.compact}><box flexDirection="row" marginTop={0}>
        <text fg={CORTEX_THEME.emeraldGreen}>{`[ ✓ ${succeededJobs()} ÉXITO ] `}</text>
        <text fg={failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted}>
          {`[ ✕ ${failedJobs()} FALLO ]`}
        </text>
      </box></Show>

      {/* Grid Fila 2: Píldoras de En Curso y Locks */}
      <box flexDirection="row" marginTop={0}>
        <text fg={activeJobs() > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted}>
          {props.layout.compact ? `${activeJobs()} externos · ${totalLeases()} bloqueos` : `[ ${activeJobs() > 0 ? props.spinner() : "●"} ${activeJobs()} CURSO ] `}
        </text>
        <Show when={!props.layout.compact}>
          <text fg={totalLeases() > 0 ? CORTEX_THEME.skyBlue : CORTEX_THEME.slateMuted}>{`[ 🛡 ${totalLeases()} LOCKS ]`}</text>
        </Show>
      </box>

      {/* Micro Medidor de Salud Operativa */}
      <box flexDirection="row" marginTop={0}>
        <text fg={CORTEX_THEME.slateMuted}>Salud: </text>
        <text fg={healthColor()}>{healthBars().filled}</text>
        <text fg={CORTEX_THEME.slateBorder}>{healthBars().empty}</text>
        <text fg={healthColor()}>{successRate() === undefined ? "N/A · sin historial" : ` ${successRate()}%`}</text>
      </box>

      {/* Autoridad SQLite y DAG */}
      <box flexDirection="row" marginTop={0}>
        <text fg={CORTEX_THEME.slateMuted}>
          {props.layout.compact ? `DAG ${doneTasks()}/${totalTasks()}` : `📋 DAG: ${doneTasks()}/${totalTasks()} · Autoridad: SQLite`}
        </text>
      </box>

      {/* Frescura de Datos / Telemetría */}
      <box flexDirection="row" marginTop={0}>
        <Show
          when={props.stale}
          fallback={
            <text fg={CORTEX_THEME.emeraldGreen}>
              {props.layout.compact ? "En vivo" : `🟢 En vivo · Sync hace ${syncAgeSec()}s`}
            </text>
          }
        >
          <text fg={CORTEX_THEME.amberGold}>
              {props.layout.compact ? "Datos no confirmados" : `🟡 Snapshot desfasado (+${syncAgeSec()}s)`}
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
  now: () => number;
  spinner: () => string;
  pulse: () => string;
  tasksExpanded: () => boolean;
  delegationsExpanded: () => boolean;
  attentionExpanded: () => boolean;
  toggleTasks: () => void;
  toggleDelegations: () => void;
  toggleAttention: () => void;
  theme: TuiThemeCurrent;
}) {
  const [rootWidth, setRootWidth] = createSignal(0);
  const layout = createMemo(() => sidebarLayout(rootWidth()));
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

  const isExecuting = createMemo(
    () => props.nativeActivity() === "busy" || activeExecutionsCount() > 0 || Boolean(activeDelegation())
  );

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
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);

  return (
    <box
      flexDirection="column"
      ref={(node: BoxRenderable) => setRootWidth(Math.max(0, node.width || 0))}
      onSizeChange={(width: number) => setRootWidth(Math.max(0, width || 0))}
    >
      {/* 1. Header Cockpit con Brand Logo & Estilo Cortex */}
      <CortexCockpitHeader
        isExecuting={isExecuting}
        nativeActivity={props.nativeActivity}
        projectRoot={props.snapshot().project_root}
        spinner={props.spinner}
        pulse={props.pulse}
        theme={props.theme}
      />

      <Show when={!props.scopeReady()}>
        <text fg={CORTEX_THEME.amberGold} marginTop={1}>Conversación no disponible · esperando metadatos</text>
      </Show>

      <Show when={props.scopeReady() && !props.snapshot().generated_at && !props.snapshotError()}>
        <text fg={CORTEX_THEME.slateMuted} marginTop={1}>Cargando estado de la conversación…</text>
      </Show>

      <Show when={props.snapshotError()}>
        <text fg={CORTEX_THEME.roseRed} marginTop={1}>No se pudo actualizar · datos no confirmados</text>
      </Show>

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
              total={totalTasks()}
              width={layout().gaugeWidth}
              compact={layout().compact}
            theme={props.theme}
          />
        </Show>

        {/* 4. Tarjeta Hero de Tarea Activa (si hay tarea en curso) */}
        <Show when={activeTask()}>
          {(task) => (
            <ActiveTaskHero
              task={task()}
              activeDelegation={activeDelegation()}
              now={props.now}
              spinner={props.spinner}
              theme={props.theme}
            />
          )}
        </Show>

        {/* 5. Sección: Tablero de Tareas */}
        <Section
          title="Tablero de Tareas"
          icon="📋"
          badge={`${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot`}
          expanded={props.tasksExpanded}
          onToggle={props.toggleTasks}
          theme={props.theme}
        >
          <TaskRows tasks={props.snapshot().tasks} spinner={props.spinner} theme={props.theme} />
        </Section>

        {/* 6. Sección: Ejecuciones & Workers */}
        <Section
          title="Workers & Delegación"
          icon="🤖"
          badge={`${activeDelegationsCount()} act / ${totalDelegationsCount()} tot`}
          expanded={props.delegationsExpanded}
          onToggle={props.toggleDelegations}
          theme={props.theme}
        >
          <DelegationRows jobs={props.jobs()} spinner={props.spinner} now={props.now} theme={props.theme} />
        </Section>

        {/* 7. Sección: Centro de Atención & Alertas */}
        <Section
          title="Centro de Atención"
          icon="⚠️"
          badge={`${counts().attention} alertas`}
          expanded={props.attentionExpanded}
          onToggle={props.toggleAttention}
          theme={props.theme}
        >
          <AttentionRows items={attention()} theme={props.theme} />
        </Section>

        {/* 8. Panel de Control Matrix y Contadores Inferiores */}
        <OperationalBottomDashboard
          snapshot={props.snapshot()}
          jobs={props.jobs()}
          stale={stale()}
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

function HomeBottomStatus(props: {
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  spinner: () => string;
  snapshotError: () => string;
  theme: TuiThemeCurrent;
}) {
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);

  return (
    <Show when={visible()}>
      <box paddingLeft={1} paddingRight={1} flexDirection="row">
        <text fg={CORTEX_THEME.brandViolet}>🧠 </text>
        <text fg={CORTEX_THEME.pureWhite}>CORTEX</text>
        <text fg={CORTEX_THEME.neonCyan}>·</text>
        <text fg={CORTEX_THEME.skyBlue}>IA </text>
        <text fg={CORTEX_THEME.slateBorder}>│ </text>
        <Show
          when={activeTask()}
          fallback={
            <box flexDirection="row">
              <text fg={CORTEX_THEME.amberGold}>{`● ${counts().active} en curso`}</text>
              <text fg={CORTEX_THEME.slateBorder}> · </text>
              <text fg={CORTEX_THEME.brandPurple}>{`◆ ${counts().review} rev`}</text>
              <Show when={counts().attention > 0}>
                <text fg={CORTEX_THEME.slateBorder}> · </text>
                <text fg={CORTEX_THEME.roseRed}>{`✕ ${counts().attention} alert`}</text>
              </Show>
            </box>
          }
        >
          {(task) => (
            <box flexDirection="row">
              <text fg={CORTEX_THEME.amberGold}>{`[${props.spinner()} ${task().task_id}] `}</text>
              <text fg={CORTEX_THEME.pureWhite}>{clipped(task().title, 20)}</text>
              <text fg={CORTEX_THEME.slateMuted}>{` · ${counts().active} activos`}</text>
            </box>
          )}
        </Show>
      </box>
    </Show>
  );
}

function initialize(api: TuiPluginApi, disposeRoot: () => void): void {
  const nativeActivity = createMemo(() => nativeSessionActivity(api));
  const scopeReady = createMemo(() => Boolean(conversationScope(api)?.project));
  const [snapshot, setSnapshot] = createSignal<UISnapshot>(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [pulseFrame, setPulseFrame] = createSignal(0);

  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get<boolean>(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(
    api.kv.get<boolean>(DELEGATIONS_EXPANDED_KEY, true) !== false
  );
  const [attentionExpanded, setAttentionExpanded] = createSignal(
    api.kv.get<boolean>(ATTENTION_EXPANDED_KEY, true) !== false
  );

  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const pulse = createMemo(() => NEURAL_PULSE_FRAMES[pulseFrame() % NEURAL_PULSE_FRAMES.length]);
  const jobs = createMemo(() => snapshot().delegations.map((job, sequence) => ({ ...job, sequence })));

  let disposed = false;
  let generation = 0;
  let activeKey = "";
  let pendingGeneration: number | undefined;
  let previousAttentionCount = 0;

  const togglePreference = (key: string, value: () => boolean, setter: (next: boolean) => void): void => {
    const next = !value();
    setter(next);
    api.kv.set(key, next);
  };

  const readSnapshot = (): void => {
    if (disposed) return;
    const scope = conversationScope(api);
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
        if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api)) !== key) return;
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
      api.kv.set(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });

  createEffect(readSnapshot);
  const snapshotPoll = setInterval(readSnapshot, SNAPSHOT_POLL_INTERVAL_MS);
  const clock = setInterval(() => setNow(Date.now()), 1000);
  const spinnerTimer = setInterval(() => setFrame((f) => (f + 1) % SPINNER_FRAMES.length), 90);
  const pulseTimer = setInterval(() => setPulseFrame((p) => (p + 1) % NEURAL_PULSE_FRAMES.length), 350);

  api.slots.register({
    order: 85,
    slots: {
      sidebar_content(ctx) {
        return (
          <SidebarStatus
            nativeActivity={nativeActivity}
            scopeReady={scopeReady}
            snapshot={snapshot}
            jobs={jobs}
            snapshotError={snapshotError}
            now={now}
            spinner={spinner}
            pulse={pulse}
            tasksExpanded={tasksExpanded}
            delegationsExpanded={delegationsExpanded}
            attentionExpanded={attentionExpanded}
            toggleTasks={() => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded)}
            toggleDelegations={() => togglePreference(DELEGATIONS_EXPANDED_KEY, delegationsExpanded, setDelegationsExpanded)}
            toggleAttention={() => togglePreference(ATTENTION_EXPANDED_KEY, attentionExpanded, setAttentionExpanded)}
            theme={ctx.theme.current}
          />
        );
      },
      home_bottom(ctx) {
        return (
          <HomeBottomStatus
            snapshot={snapshot}
            jobs={jobs}
            spinner={spinner}
            snapshotError={snapshotError}
            theme={ctx.theme.current}
          />
        );
      },
    },
  });

  api.lifecycle.onDispose(() => {
    disposed = true;
    clearInterval(snapshotPoll);
    clearInterval(clock);
    clearInterval(spinnerTimer);
    clearInterval(pulseTimer);
    disposeRoot();
  });
}

const tui: TuiPlugin = async (api) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};

const plugin: TuiPluginModule = { id: "cortex-ia.delegation-status", tui };
export default plugin;
