import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
  TuiThemeCurrent,
} from "@opencode-ai/plugin/tui";
import { execFile } from "node:child_process";
import path from "node:path";
import {
  For,
  Show,
  createEffect,
  createMemo,
  createRoot,
  createSignal,
} from "solid-js";

const SNAPSHOT_POLL_INTERVAL_MS = 2500;
const SNAPSHOT_STALE_MS = 10_000;
const MAX_VISIBLE_ROWS = 4;
const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

const TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
const DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
const ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";

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

type UISnapshot = {
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
  return process.env.CORTEX_IA_BIN || "cortex-ia";
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

function statusColor(status: string, theme: TuiThemeCurrent) {
  if (["succeeded", "done", "superseded"].includes(status)) return theme.success;
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return theme.error;
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return theme.warning;
  if (status === "in_review") return theme.accent;
  return theme.textMuted;
}

function statusIcon(status: string, spinner: string): string {
  if (["succeeded", "done", "superseded"].includes(status)) return "✓";
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return "✕";
  if (status === "in_review") return "◆";
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return spinner;
  return "○";
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

// Route and session metadata are reactive host state. Missing/cyclic ancestry
// deliberately produces no scope, including the home route.
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

function Section(props: {
  title: string;
  badge?: string;
  expanded: () => boolean;
  onToggle: () => void;
  theme: TuiThemeCurrent;
  children: unknown;
}) {
  return (
    <box flexDirection="column" marginTop={1}>
      <text fg={props.theme.text} selectable={false} onMouseDown={props.onToggle}>
        {`${props.expanded() ? "▼" : "▶"} ${props.title}${props.badge ? ` · ${props.badge}` : ""}`}
      </text>
      <Show when={props.expanded()}>{props.children}</Show>
    </box>
  );
}

function ProgressBar(props: {
  done: number;
  total: number;
  width?: number;
  theme: TuiThemeCurrent;
}) {
  const w = props.width || 14;
  const ratio = props.total > 0 ? Math.min(1, Math.max(0, props.done / props.total)) : 0;
  const filled = Math.round(ratio * w);
  const empty = Math.max(0, w - filled);
  const pct = Math.round(ratio * 100);
  return (
    <box flexDirection="row">
      <text fg={props.theme.success}>{"█".repeat(filled)}</text>
      <text fg={props.theme.textMuted}>{"░".repeat(empty)}</text>
      <text fg={props.theme.textMuted}>{` ${pct}% (${props.done}/${props.total})`}</text>
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
    <box flexDirection="column" marginTop={1} paddingLeft={1} borderStyle="rounded" borderColor={props.theme.warning}>
      <box flexDirection="row">
        <text fg={props.theme.warning}>{`${props.spinner()} `}</text>
        <text fg={props.theme.text}>{clipped(`${props.task.task_id} · ${props.task.title}`, 26)}</text>
      </box>
      <box flexDirection="row">
        <text fg={props.theme.warning}>  EN CURSO </text>
        <text fg={props.theme.textMuted}>{`· ⏱ ${elapsed()}`}</text>
      </box>
      <box flexDirection="row">
        <Show
          when={props.activeDelegation}
          fallback={
            <text fg={props.theme.accent}>
              {`  🧑‍💻 Nativo OpenCode${props.task.owner ? ` (${clipped(props.task.owner, 10)})` : ""}`}
            </text>
          }
        >
          {(del) => (
            <text fg={props.theme.accent}>
              {`  ⚡ AGY ${del().transport || "direct"}${del().pane_id ? ` · ${del().pane_id}` : ""}${del().attempt ? ` · int #${del().attempt}` : ""}`}
            </text>
          )}
        </Show>
      </box>
      <Show when={ttlRemaining()}>
        {(ttl) => (
          <text fg={ttl() === "expirado" ? props.theme.error : props.theme.textMuted}>
            {`  🛡 Lease: ${ttl()}${props.task.lease_count ? ` · ${props.task.lease_count} lock` : ""}`}
          </text>
        )}
      </Show>
    </box>
  );
}

function TaskRows(props: {
  tasks: DashboardTask[];
  spinner: () => string;
  theme: TuiThemeCurrent;
}) {
  return (
    <Show when={props.tasks.length > 0} fallback={<text fg={props.theme.textMuted}>  Sin tareas durables activas</text>}>
      <For each={props.tasks.slice(0, MAX_VISIBLE_ROWS)}>
        {(task) => (
          <box flexDirection="column" marginTop={0}>
            <box flexDirection="row">
              <text fg={statusColor(task.status, props.theme)}>
                {`${statusIcon(task.status, props.spinner())} `}
              </text>
              <text fg={task.status === "in_progress" ? props.theme.warning : props.theme.text}>
                {clipped(`${task.task_id} · ${task.title}`)}
              </text>
            </box>
            <text fg={props.theme.textMuted}>
              {`  ${clipped(task.board_id, 12)} · ${task.status}${task.owner ? ` · ${clipped(task.owner, 8)}` : ""}${task.lease_count ? ` · ${task.lease_count} lk` : ""}`}
            </text>
          </box>
        )}
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
    <Show when={props.jobs.length > 0} fallback={<text fg={props.theme.textMuted}>  Sin ejecuciones delegadas</text>}>
      <For each={props.jobs.slice(0, MAX_VISIBLE_ROWS)}>
        {(job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` · ${formatDuration(props.now() - t)}` : "";
          });
          return (
            <box flexDirection="column" marginTop={0}>
              <box flexDirection="row">
                <text fg={statusColor(job.status, props.theme)}>
                  {`${statusIcon(job.status, props.spinner())} `}
                </text>
                <text fg={isRunning ? props.theme.warning : props.theme.text}>
                  {job.role || "worker"}
                </text>
                <text fg={props.theme.textMuted}>{` · ${job.status}${elapsed()}`}</text>
              </box>
              <text fg={props.theme.textMuted}>
                {`  ${shortID(job.job_id)} · ${job.transport || "direct"}${job.pane_id ? ` · ${job.pane_id}` : ""}${job.attempt ? ` · int #${job.attempt}` : ""}`}
              </text>
            </box>
          );
        }}
      </For>
    </Show>
  );
}

function AttentionRows(props: { items: AttentionItem[]; theme: TuiThemeCurrent }) {
  return (
    <Show when={props.items.length > 0} fallback={<text fg={props.theme.textMuted}>  Sin alertas</text>}>
      <For each={props.items.slice(0, MAX_VISIBLE_ROWS)}>
        {(item) => (
          <box flexDirection="column">
            <text fg={props.theme.error}>{`! ${clipped(item.title)}`}</text>
            <text fg={props.theme.textMuted}>{`  ${item.detail}`}</text>
          </box>
        )}
      </For>
    </Show>
  );
}

function SidebarStatus(props: {
  nativeActivity: () => NativeActivity | undefined;
  scopeReady: () => boolean;
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  snapshotError: () => string;
  now: () => number;
  spinner: () => string;
  tasksExpanded: () => boolean;
  delegationsExpanded: () => boolean;
  attentionExpanded: () => boolean;
  toggleTasks: () => void;
  toggleDelegations: () => void;
  toggleAttention: () => void;
  theme: TuiThemeCurrent;
}) {
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
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);

  return (
    <box flexDirection="column">
      {/* 1. Header Cockpit */}
      <box flexDirection="row">
        <text fg={props.theme.accent}>⚡ CORTEX-IA </text>
        <Show
          when={isExecuting()}
          fallback={<text fg={props.theme.textMuted}>[EN ESPERA]</text>}
        >
          <text fg={props.theme.warning}>{`[${props.spinner()} ACTIVO]`}</text>
        </Show>
      </box>

      {/* 2. Subtítulo / Contexto de Sesión */}
      <Show when={props.nativeActivity()}>
        <text
          fg={
            props.nativeActivity() === "busy" || props.nativeActivity() === "retry"
              ? props.theme.warning
              : props.theme.textMuted
          }
        >
          {`Sesión · ${{ busy: "trabajando…", idle: "en espera", retry: "reintentando", unknown: "no disponible" }[props.nativeActivity() ?? "unknown"]}`}
        </text>
      </Show>

      <Show when={props.snapshot().project_root}>
        <text fg={props.theme.textMuted}>{`Proyecto · ${path.basename(props.snapshot().project_root)}`}</text>
      </Show>

      <Show when={!props.scopeReady()}>
        <text fg={props.theme.warning}>Conversación no disponible · esperando metadatos</text>
      </Show>

      <Show when={props.scopeReady() && !props.snapshot().generated_at && !props.snapshotError()}>
        <text fg={props.theme.textMuted}>Cargando estado de la conversación…</text>
      </Show>

      <Show when={props.snapshotError()}>
        <text fg={props.theme.error}>No se pudo actualizar · datos no confirmados</text>
      </Show>

      <Show when={props.scopeReady() && Boolean(props.snapshot().generated_at)}>
        {/* 3. Métricas Principales (Pills) */}
        <box flexDirection="row" marginTop={1}>
          <text fg={activeExecutionsCount() > 0 ? props.theme.warning : props.theme.textMuted}>
            {`${activeExecutionsCount() > 0 ? props.spinner() : "●"} ${activeExecutionsCount()} en curso`}
          </text>
          <text fg={props.theme.textMuted}> │ </text>
          <text fg={counts().review > 0 ? props.theme.accent : props.theme.textMuted}>
            {`◆ ${counts().review} rev`}
          </text>
          <text fg={props.theme.textMuted}> │ </text>
          <text fg={counts().attention > 0 ? props.theme.error : props.theme.textMuted}>
            {`✕ ${counts().attention} alert`}
          </text>
        </box>

        {/* 4. Mini Barra de Progreso del Tablero DAG */}
        <Show when={totalTasks() > 0}>
          <box flexDirection="column" marginTop={1}>
            <ProgressBar done={doneTasks()} total={totalTasks()} theme={props.theme} />
          </box>
        </Show>

        {/* 5. Tarjeta Hero de Tarea Activa (si hay tarea en curso) */}
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

        {/* 6. Sección: Tablero de Tareas */}
        <Section
          title="Tablero de Tareas"
          badge={`${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot`}
          expanded={props.tasksExpanded}
          onToggle={props.toggleTasks}
          theme={props.theme}
        >
          <TaskRows tasks={props.snapshot().tasks} spinner={props.spinner} theme={props.theme} />
        </Section>

        {/* 7. Sección: Ejecuciones & Workers */}
        <Section
          title="Ejecuciones & Workers"
          badge={`${activeDelegationsCount()} act / ${totalDelegationsCount()} tot`}
          expanded={props.delegationsExpanded}
          onToggle={props.toggleDelegations}
          theme={props.theme}
        >
          <DelegationRows jobs={props.jobs()} spinner={props.spinner} now={props.now} theme={props.theme} />
        </Section>

        {/* 8. Sección: Atención & Alertas */}
        <Section
          title="Atención requerida"
          badge={`${counts().attention}`}
          expanded={props.attentionExpanded}
          onToggle={props.toggleAttention}
          theme={props.theme}
        >
          <AttentionRows items={attention()} theme={props.theme} />
        </Section>

        {/* 9. Footer de estado del snapshot */}
        <text fg={stale() ? props.theme.warning : props.theme.textMuted}>
          {stale() ? "snapshot obsoleto" : "snapshot actualizado"}
        </text>
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
        <text fg={props.theme.accent}>⚡ Cortex </text>
        <Show
          when={activeTask()}
          fallback={
            <box flexDirection="row">
              <text fg={props.theme.warning}>{`● ${counts().active} en curso`}</text>
              <text fg={props.theme.textMuted}> · </text>
              <text fg={props.theme.accent}>{`◆ ${counts().review} rev`}</text>
              <Show when={counts().attention > 0}>
                <text fg={props.theme.textMuted}> · </text>
                <text fg={props.theme.error}>{`✕ ${counts().attention} alert`}</text>
              </Show>
            </box>
          }
        >
          {(task) => (
            <box flexDirection="row">
              <text fg={props.theme.warning}>{`[${props.spinner()} ${task().task_id}] `}</text>
              <text fg={props.theme.text}>{clipped(task().title, 20)}</text>
              <text fg={props.theme.textMuted}>{` · ${counts().active} activos`}</text>
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

  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get<boolean>(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(
    api.kv.get<boolean>(DELEGATIONS_EXPANDED_KEY, true) !== false
  );
  const [attentionExpanded, setAttentionExpanded] = createSignal(
    api.kv.get<boolean>(ATTENTION_EXPANDED_KEY, true) !== false
  );

  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
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
    disposeRoot();
  });
}

const tui: TuiPlugin = async (api) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};

const plugin: TuiPluginModule = { id: "cortex-ia.delegation-status", tui };
export default plugin;
