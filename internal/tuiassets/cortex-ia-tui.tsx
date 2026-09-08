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

type UISnapshot = {
  schema_version: number;
  generated_at: string;
  project_root: string;
  requested_session_id: string;
  root_session_id: string;
  summary: { active_tasks: number; total_delegations: number; total_attention: number };
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
  generated_at: "", project_root: "", requested_session_id: "", root_session_id: "",
  summary: { active_tasks: 0, total_delegations: 0, total_attention: 0 },
  counts: { active: 0, review: 0, attention: 0 },
  attention: [], tasks: [], delegations: [],
};

function cortexExecutable(): string {
  return process.env.CORTEX_IA_BIN || "cortex-ia";
}

function shortID(id: string): string {
  return id.length > 13 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function clipped(value: string, limit = 29): string {
  const text = value.trim();
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function statusColor(status: string, theme: TuiThemeCurrent) {
  if (["succeeded", "done", "superseded"].includes(status)) return theme.success;
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return theme.error;
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return theme.warning;
  if (status === "in_review") return theme.accent;
  return theme.textMuted;
}

function statusIcon(status: string): string {
  if (["succeeded", "done", "superseded"].includes(status)) return "✓";
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return "✕";
  if (status === "in_review") return "◆";
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return "●";
  return "○";
}

function attentionItems(snapshot: UISnapshot, snapshotError: string): AttentionItem[] {
  const items = snapshot.attention.map((item) => ({
    id: item.id, title: item.title, detail: `${item.kind} · ${shortID(item.entity_id)}`,
  }));
  if (snapshotError) items.push({ id: "snapshot-error", title: "Snapshot no disponible", detail: clipped(snapshotError, 35) });
  return items;
}

function operationalCounts(snapshot: UISnapshot, snapshotError: string): OperationalCounts {
  return { ...snapshot.counts, attention: snapshot.counts.attention + (snapshotError ? 1 : 0) };
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
  count: number;
  expanded: () => boolean;
  onToggle: () => void;
  theme: TuiThemeCurrent;
  children: unknown;
}) {
  return (
    <box flexDirection="column">
      <text fg={props.theme.text} selectable={false} onMouseDown={props.onToggle}>
        {`${props.expanded() ? "▼" : "▶"} ${props.title} · ${props.count}`}
      </text>
      <Show when={props.expanded()}>{props.children}</Show>
    </box>
  );
}

function TaskRows(props: { tasks: DashboardTask[]; theme: TuiThemeCurrent }) {
  return (
    <Show when={props.tasks.length > 0} fallback={<text fg={props.theme.textMuted}>  Sin tareas durables activas</text>}>
      <For each={props.tasks.slice(0, MAX_VISIBLE_ROWS)}>
        {(task) => (
          <box flexDirection="column">
            <box flexDirection="row">
              <text fg={statusColor(task.status, props.theme)}>{`${statusIcon(task.status)} `}</text>
              <text fg={props.theme.text}>{clipped(`${task.task_id} · ${task.title}`)}</text>
            </box>
            <text fg={props.theme.textMuted}>
              {`  ${clipped(task.board_id, 14)} · ${task.status}${task.owner ? ` · ${clipped(task.owner, 10)}` : ""}${task.lease_count ? ` · ${task.lease_count} lease` : ""}`}
            </text>
          </box>
        )}
      </For>
    </Show>
  );
}

function DelegationRows(props: { jobs: DelegationJob[]; theme: TuiThemeCurrent }) {
  return (
    <Show when={props.jobs.length > 0} fallback={<text fg={props.theme.textMuted}>  Sin delegaciones registradas</text>}>
      <For each={props.jobs.slice(0, MAX_VISIBLE_ROWS)}>
        {(job) => (
          <box flexDirection="column">
            <box flexDirection="row">
              <text fg={statusColor(job.status, props.theme)}>{`${statusIcon(job.status)} `}</text>
              <text fg={props.theme.text}>{job.role || "delegate"}</text>
              <text fg={props.theme.textMuted}>{` · ${job.status}`}</text>
            </box>
            <text fg={props.theme.textMuted}>
              {`  ${shortID(job.job_id)} · ${job.transport || "unknown"}${job.pane_id ? ` · ${job.pane_id}` : ""}`}
            </text>
          </box>
        )}
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
  return (
    <box flexDirection="column">
      <text fg={props.theme.text}>Cortex-IA</text>
      <Show when={props.nativeActivity()}>
        <text fg={props.nativeActivity() === "busy" || props.nativeActivity() === "retry" ? props.theme.warning : props.theme.textMuted}>
          {`Sesión actual · ${{ busy: "trabajando", idle: "en espera", retry: "reintentando", unknown: "estado no disponible" }[props.nativeActivity() ?? "unknown"]}`}
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
      <box flexDirection="row">
        <text fg={props.theme.warning}>{`● ${counts().active}`}</text>
        <text fg={props.theme.textMuted}> · </text>
        <text fg={props.theme.accent}>{`◆ ${counts().review}`}</text>
        <text fg={props.theme.textMuted}> · </text>
        <text fg={props.theme.error}>{`✕ ${counts().attention}`}</text>
      </box>
      <Section title="Task board" count={props.snapshot().summary.active_tasks} expanded={props.tasksExpanded} onToggle={props.toggleTasks} theme={props.theme}>
        <TaskRows tasks={props.snapshot().tasks} theme={props.theme} />
      </Section>
      <Section title="Delegaciones" count={props.snapshot().summary.total_delegations} expanded={props.delegationsExpanded} onToggle={props.toggleDelegations} theme={props.theme}>
        <DelegationRows jobs={props.jobs()} theme={props.theme} />
      </Section>
      <Section title="Atención" count={counts().attention} expanded={props.attentionExpanded} onToggle={props.toggleAttention} theme={props.theme}>
        <AttentionRows items={attention()} theme={props.theme} />
      </Section>
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
  snapshotError: () => string;
  theme: TuiThemeCurrent;
}) {
  const attention = createMemo(() => attentionItems(props.snapshot(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);
  return (
    <Show when={visible()}>
      <box paddingLeft={1} paddingRight={1} flexDirection="row">
        <text fg={props.theme.text}>Cortex </text>
        <text fg={props.theme.warning}>{`● ${counts().active}`}</text>
        <text fg={props.theme.textMuted}> · </text>
        <text fg={props.theme.accent}>{`◆ ${counts().review}`}</text>
        <text fg={props.theme.textMuted}> · </text>
        <text fg={props.theme.error}>{`✕ ${counts().attention}`}</text>
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
  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get<boolean>(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(api.kv.get<boolean>(DELEGATIONS_EXPANDED_KEY, true) !== false);
  const [attentionExpanded, setAttentionExpanded] = createSignal(api.kv.get<boolean>(ATTENTION_EXPANDED_KEY, true) !== false);
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
    execFile(cortexExecutable(), ["ui", "snapshot", "--project", scope.project,
      "--session-id", scope.sessionID, "--root-session-id", scope.rootSessionID],
      { encoding: "utf8", maxBuffer: 512 * 1024, timeout: 7500, windowsHide: true },
      (error, stdout) => {
        if (pendingGeneration === requestGeneration) pendingGeneration = undefined;
        if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api)) !== key) return;
        if (error) { setSnapshotError(error.message); return; }
        try {
          const next = JSON.parse(stdout) as UISnapshot;
          if (next.schema_version !== 2 || next.requested_session_id !== scope.sessionID || next.root_session_id !== scope.rootSessionID ||
              !Array.isArray(next.tasks) || !Array.isArray(next.delegations) || !Array.isArray(next.attention) ||
              !next.counts || !next.summary) throw new Error("snapshot schema or conversation is incompatible");
          setSnapshot(next);
          setSnapshotError("");
        } catch (parseError) {
          setSnapshotError(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
        }
      });
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
        return <HomeBottomStatus snapshot={snapshot} jobs={jobs} snapshotError={snapshotError} theme={ctx.theme.current} />;
      },
    },
  });
  api.lifecycle.onDispose(() => {
    disposed = true;
    clearInterval(snapshotPoll);
    clearInterval(clock);
    disposeRoot();
  });
}

const tui: TuiPlugin = async (api) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};

const plugin: TuiPluginModule = { id: "cortex-ia.delegation-status", tui };
export default plugin;
