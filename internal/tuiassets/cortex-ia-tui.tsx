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

const EVENT_POLL_INTERVAL_MS = 500;
const SNAPSHOT_POLL_INTERVAL_MS = 2500;
const SNAPSHOT_STALE_MS = 10_000;
const MAX_INITIAL_BYTES = 256 * 1024;
const MAX_RETAINED_JOBS = 100;
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
  summary: {
    sessions: number;
    active_tasks: number;
    active_agents: number;
    active_delegations: number;
    blocked: number;
  };
  tasks: DashboardTask[];
  delegations: DashboardDelegation[];
};

type AttentionItem = { id: string; title: string; detail: string };
type OperationalCounts = { active: number; review: number; attention: number };

const EMPTY_SNAPSHOT: UISnapshot = {
  schema_version: 1,
  generated_at: "",
  project_root: "",
  summary: {
    sessions: 0,
    active_tasks: 0,
    active_agents: 0,
    active_delegations: 0,
    blocked: 0,
  },
  tasks: [],
  delegations: [],
};

function eventsPath(): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  return path.resolve(home, ".config", "opencode", "cortex-delegation-events.jsonl");
}

function cortexExecutable(): string {
  return process.env.CORTEX_IA_BIN || "cortex-ia";
}

function projectKey(value: string): string {
  const resolved = path.resolve(value || ".").replaceAll("\\", "/");
  return process.platform === "win32" || process.platform === "darwin" ? resolved.toLowerCase() : resolved;
}

function isRunning(status: string): boolean {
  return ["accepted", "starting", "running", "queued"].includes(status);
}

function isDone(status: string): boolean {
  return ["succeeded", "done", "superseded"].includes(status);
}

function isError(status: string): boolean {
  return ["failed", "timed_out", "lost", "cancelled"].includes(status);
}

function needsAttention(status: string): boolean {
  return ["failed", "timed_out", "lost"].includes(status);
}

function shortID(id: string): string {
  return id.length > 13 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function clipped(value: string, limit = 29): string {
  const text = value.trim();
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text;
}

function statusColor(status: string, theme: TuiThemeCurrent) {
  if (isDone(status)) return theme.success;
  if (isError(status) || status === "blocked") return theme.error;
  if (isRunning(status) || status === "in_progress") return theme.warning;
  if (status === "in_review") return theme.accent;
  return theme.textMuted;
}

function statusIcon(status: string): string {
  if (isDone(status)) return "✓";
  if (isError(status) || status === "blocked") return "✕";
  if (status === "in_review") return "◆";
  if (isRunning(status) || status === "in_progress") return "●";
  return "○";
}

function showToast(api: TuiPluginApi, payload: object): void {
  const toast = api.ui.toast as
    | ((input: object) => void)
    | { show?: (input: object) => void };
  if (typeof toast === "function") {
    toast(payload);
    return;
  }
  toast?.show?.(payload);
}

function toastFor(event: DelegationEvent) {
  const status = event.status || "unknown";
  const role = event.role ? `${event.role} · ` : "";
  const job = event.job_id ? ` · ${shortID(event.job_id)}` : "";
  if (isDone(status)) {
    return { variant: "success" as const, title: "Cortex-IA", message: `${role}delegación completada${job}`, duration: 5000 };
  }
  if (isError(status)) {
    return {
      variant: status === "cancelled" ? ("warning" as const) : ("error" as const),
      title: "Cortex-IA",
      message: `${role}delegación ${status}${job}`,
      duration: 7000,
    };
  }
  return { variant: "info" as const, title: "Cortex-IA", message: `${role}delegación ${status}${job}`, duration: 4000 };
}

function mergedDelegations(snapshot: UISnapshot, eventJobs: DelegationJob[]): DelegationJob[] {
  const durable = snapshot.delegations.map((job, index) => ({
    ...job,
    sequence: eventJobs.length + snapshot.delegations.length - index,
  }));
  const durableIDs = new Set(durable.map((job) => job.job_id));
  const projectEvents = eventJobs.filter((job) => {
    if (durableIDs.has(job.job_id)) return false;
    return Boolean(job.workspace && snapshot.project_root && projectKey(job.workspace) === projectKey(snapshot.project_root));
  });
  return [...projectEvents, ...durable].slice(0, MAX_RETAINED_JOBS);
}

function attentionItems(snapshot: UISnapshot, jobs: DelegationJob[], snapshotError: string): AttentionItem[] {
  const items: AttentionItem[] = [];
  let visibleBlocked = 0;
  for (const task of snapshot.tasks) {
    if (task.status === "blocked") {
      visibleBlocked += 1;
      items.push({ id: `task-${task.task_id}`, title: `Tarea bloqueada · ${task.task_id}`, detail: clipped(task.title) });
    }
    const claimExpiry = Date.parse(task.claim_expires_at || "");
    if (task.owner && Number.isFinite(claimExpiry) && claimExpiry < Date.now()) {
      items.push({
        id: `claim-${task.task_id}`,
        title: `Claim vencido · ${task.task_id}`,
        detail: clipped(task.owner),
      });
    }
  }
  if (snapshot.summary.blocked > visibleBlocked) {
    items.push({
      id: "blocked-summary",
      title: `${snapshot.summary.blocked - visibleBlocked} tareas bloqueadas adicionales`,
      detail: "revisar task boards",
    });
  }
  for (const job of jobs.slice(0, MAX_VISIBLE_ROWS)) {
    if (needsAttention(job.status) || job.error_code) {
      items.push({
        id: `job-${job.job_id}`,
        title: `${job.error_code || job.status} · ${job.role || "delegate"}`,
        detail: `${shortID(job.job_id)} · ${job.transport || "unknown"}`,
      });
    }
  }
  if (snapshotError) {
    items.push({ id: "snapshot-error", title: "Snapshot no disponible", detail: clipped(snapshotError, 35) });
  }
  return items;
}

function operationalCounts(snapshot: UISnapshot, jobs: DelegationJob[], attention: AttentionItem[]): OperationalCounts {
  return {
    active: snapshot.tasks.filter((task) => task.status === "in_progress").length + jobs.filter((job) => isRunning(job.status)).length,
    review: snapshot.tasks.filter((task) => task.status === "in_review").length,
    attention: attention.length,
  };
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
  const attention = createMemo(() => attentionItems(props.snapshot(), props.jobs(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.jobs(), attention()));
  const stale = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    return Boolean(props.snapshotError()) || !Number.isFinite(generated) || props.now() - generated > SNAPSHOT_STALE_MS;
  });
  return (
    <box flexDirection="column">
      <text fg={props.theme.text}>Cortex-IA</text>
      <Show when={props.snapshot().project_root}>
        <text fg={props.theme.textMuted}>{`Proyecto · ${path.basename(props.snapshot().project_root)}`}</text>
      </Show>
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
      <Section title="Delegaciones" count={props.jobs().length} expanded={props.delegationsExpanded} onToggle={props.toggleDelegations} theme={props.theme}>
        <DelegationRows jobs={props.jobs()} theme={props.theme} />
      </Section>
      <Section title="Atención" count={attention().length} expanded={props.attentionExpanded} onToggle={props.toggleAttention} theme={props.theme}>
        <AttentionRows items={attention()} theme={props.theme} />
      </Section>
      <text fg={stale() ? props.theme.warning : props.theme.textMuted}>
        {stale() ? "snapshot obsoleto" : "snapshot actualizado"}
      </text>
    </box>
  );
}

function HomeBottomStatus(props: {
  snapshot: () => UISnapshot;
  jobs: () => DelegationJob[];
  snapshotError: () => string;
  theme: TuiThemeCurrent;
}) {
  const attention = createMemo(() => attentionItems(props.snapshot(), props.jobs(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.jobs(), attention()));
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
  const source = eventsPath();
  const projectDirectory = api.state.path.directory || process.cwd();
  const [eventJobs, setEventJobs] = createSignal<DelegationJob[]>([]);
  const [snapshot, setSnapshot] = createSignal<UISnapshot>(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  const [now, setNow] = createSignal(Date.now());
  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get<boolean>(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(api.kv.get<boolean>(DELEGATIONS_EXPANDED_KEY, true) !== false);
  const [attentionExpanded, setAttentionExpanded] = createSignal(api.kv.get<boolean>(ATTENTION_EXPANDED_KEY, true) !== false);
  const jobs = createMemo(() => mergedDelegations(snapshot(), eventJobs()));
  let disposed = false;
  let snapshotInFlight = false;
  let sequence = 0;
  let offset = 0;
  let remainder = "";
  let previousAttentionCount = 0;

  const togglePreference = (key: string, value: () => boolean, setter: (next: boolean) => void): void => {
    const next = !value();
    setter(next);
    api.kv.set(key, next);
  };

  const applyEvent = (event: DelegationEvent, notify: boolean): void => {
    if (event.kind !== "delegation" || typeof event.job_id !== "string" || !event.job_id || typeof event.status !== "string" || !event.status) return;
    if (event.workspace && snapshot().project_root && projectKey(event.workspace) !== projectKey(snapshot().project_root)) return;
    sequence += 1;
    setEventJobs((current) => {
      const next = current.filter((job) => job.job_id !== event.job_id);
      next.unshift({ ...event, job_id: event.job_id!, status: event.status!, sequence });
      return next.slice(0, MAX_RETAINED_JOBS);
    });
    if (notify && event.workspace && projectKey(event.workspace) === projectKey(snapshot().project_root)) showToast(api, toastFor(event));
  };

  const applyLines = (content: string, notify: boolean, dropFirst: boolean): void => {
    const lines = content.split("\n");
    if (dropFirst) lines.shift();
    remainder = lines.pop() || "";
    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        applyEvent(JSON.parse(line) as DelegationEvent, notify);
      } catch {
        // Ignore a malformed external line; later valid events stay observable.
      }
    }
  };

  const hydrateEvents = (): void => {
    let size: number;
    try {
      size = fs.statSync(source).size;
    } catch {
      offset = 0;
      return;
    }
    const start = Math.max(0, size - MAX_INITIAL_BYTES);
    const descriptor = fs.openSync(source, "r");
    try {
      const buffer = Buffer.alloc(size - start);
      fs.readSync(descriptor, buffer, 0, buffer.length, start);
      offset = size;
      applyLines(buffer.toString("utf-8"), false, start > 0);
    } finally {
      fs.closeSync(descriptor);
    }
  };

  const readEvents = (): void => {
    let size: number;
    try {
      size = fs.statSync(source).size;
    } catch {
      return;
    }
    if (size < offset) {
      setEventJobs([]);
      remainder = "";
      hydrateEvents();
      return;
    }
    if (size === offset) return;
    const descriptor = fs.openSync(source, "r");
    try {
      const buffer = Buffer.alloc(size - offset);
      fs.readSync(descriptor, buffer, 0, buffer.length, offset);
      offset = size;
      const prefix = remainder;
      remainder = "";
      applyLines(`${prefix}${buffer.toString("utf-8")}`, true, false);
    } finally {
      fs.closeSync(descriptor);
    }
  };

  const readSnapshot = (): void => {
    if (snapshotInFlight || disposed) return;
    snapshotInFlight = true;
    execFile(
      cortexExecutable(),
      ["ui", "snapshot", "--project", projectDirectory],
      { encoding: "utf8", maxBuffer: 512 * 1024, timeout: 7500, windowsHide: true },
      (error, stdout) => {
        snapshotInFlight = false;
        if (disposed) return;
        if (error) {
          setSnapshotError(error.message || "cortex-ia ui snapshot failed");
          return;
        }
        try {
          const next = JSON.parse(stdout) as UISnapshot;
          if (next.schema_version !== 1 || !Array.isArray(next.tasks) || !Array.isArray(next.delegations)) {
            throw new Error("snapshot schema is incompatible");
          }
          setSnapshot(next);
          setSnapshotError("");
        } catch (parseError) {
          setSnapshotError(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
        }
      },
    );
  };

  createEffect(() => {
    const count = attentionItems(snapshot(), jobs(), snapshotError()).length;
    if (count > previousAttentionCount) {
      setAttentionExpanded(true);
      api.kv.set(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });

  hydrateEvents();
  readSnapshot();
  const eventPoll = setInterval(readEvents, EVENT_POLL_INTERVAL_MS);
  const snapshotPoll = setInterval(readSnapshot, SNAPSHOT_POLL_INTERVAL_MS);
  const clock = setInterval(() => setNow(Date.now()), 1000);
  api.slots.register({
    order: 85,
    slots: {
      sidebar_content(ctx) {
        return (
          <SidebarStatus
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
    clearInterval(eventPoll);
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
