// cortex-ia-tui.tsx
import { memo as _$memo } from "@opentui/solid";
import { createTextNode as _$createTextNode } from "@opentui/solid";
import { effect as _$effect } from "@opentui/solid";
import { insertNode as _$insertNode } from "@opentui/solid";
import { createComponent as _$createComponent } from "@opentui/solid";
import { insert as _$insert } from "@opentui/solid";
import { setProp as _$setProp } from "@opentui/solid";
import { createElement as _$createElement } from "@opentui/solid";
import { execFile } from "child_process";
import fs from "fs";
import path from "path";
import { For, Show, createEffect, createMemo, createRoot, createSignal } from "solid-js";
var EVENT_POLL_INTERVAL_MS = 500;
var SNAPSHOT_POLL_INTERVAL_MS = 2500;
var SNAPSHOT_STALE_MS = 1e4;
var MAX_INITIAL_BYTES = 256 * 1024;
var MAX_RETAINED_JOBS = 100;
var MAX_VISIBLE_ROWS = 4;
var TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
var DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
var ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
var EMPTY_SNAPSHOT = {
  schema_version: 1,
  generated_at: "",
  project_root: "",
  summary: {
    sessions: 0,
    active_tasks: 0,
    active_agents: 0,
    active_delegations: 0,
    blocked: 0
  },
  tasks: [],
  delegations: []
};
function eventsPath() {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  return path.resolve(home, ".config", "opencode", "cortex-delegation-events.jsonl");
}
function cortexExecutable() {
  return process.env.CORTEX_IA_BIN || "cortex-ia";
}
function projectKey(value) {
  const resolved = path.resolve(value || ".").replaceAll("\\", "/");
  return process.platform === "win32" || process.platform === "darwin" ? resolved.toLowerCase() : resolved;
}
function isRunning(status) {
  return ["accepted", "starting", "running", "queued"].includes(status);
}
function isDone(status) {
  return ["succeeded", "done", "superseded"].includes(status);
}
function isError(status) {
  return ["failed", "timed_out", "lost", "cancelled"].includes(status);
}
function needsAttention(status) {
  return ["failed", "timed_out", "lost"].includes(status);
}
function shortID(id) {
  return id.length > 13 ? `${id.slice(0, 8)}\u2026${id.slice(-4)}` : id;
}
function clipped(value, limit = 29) {
  const text = value.trim();
  return text.length > limit ? `${text.slice(0, limit - 1)}\u2026` : text;
}
function statusColor(status, theme) {
  if (isDone(status)) return theme.success;
  if (isError(status) || status === "blocked") return theme.error;
  if (isRunning(status) || status === "in_progress") return theme.warning;
  if (status === "in_review") return theme.accent;
  return theme.textMuted;
}
function statusIcon(status) {
  if (isDone(status)) return "\u2713";
  if (isError(status) || status === "blocked") return "\u2715";
  if (status === "in_review") return "\u25C6";
  if (isRunning(status) || status === "in_progress") return "\u25CF";
  return "\u25CB";
}
function showToast(api, payload) {
  const toast = api.ui.toast;
  if (typeof toast === "function") {
    toast(payload);
    return;
  }
  toast?.show?.(payload);
}
function toastFor(event) {
  const status = event.status || "unknown";
  const role = event.role ? `${event.role} \xB7 ` : "";
  const job = event.job_id ? ` \xB7 ${shortID(event.job_id)}` : "";
  if (isDone(status)) {
    return {
      variant: "success",
      title: "Cortex-IA",
      message: `${role}delegaci\xF3n completada${job}`,
      duration: 5e3
    };
  }
  if (isError(status)) {
    return {
      variant: status === "cancelled" ? "warning" : "error",
      title: "Cortex-IA",
      message: `${role}delegaci\xF3n ${status}${job}`,
      duration: 7e3
    };
  }
  return {
    variant: "info",
    title: "Cortex-IA",
    message: `${role}delegaci\xF3n ${status}${job}`,
    duration: 4e3
  };
}
function mergedDelegations(snapshot, eventJobs) {
  const durable = snapshot.delegations.map((job, index) => ({
    ...job,
    sequence: eventJobs.length + snapshot.delegations.length - index
  }));
  const durableIDs = new Set(durable.map((job) => job.job_id));
  const projectEvents = eventJobs.filter((job) => {
    if (durableIDs.has(job.job_id)) return false;
    return Boolean(job.workspace && snapshot.project_root && projectKey(job.workspace) === projectKey(snapshot.project_root));
  });
  return [...projectEvents, ...durable].slice(0, MAX_RETAINED_JOBS);
}
function attentionItems(snapshot, jobs, snapshotError) {
  const items = [];
  let visibleBlocked = 0;
  for (const task of snapshot.tasks) {
    if (task.status === "blocked") {
      visibleBlocked += 1;
      items.push({
        id: `task-${task.task_id}`,
        title: `Tarea bloqueada \xB7 ${task.task_id}`,
        detail: clipped(task.title)
      });
    }
    const claimExpiry = Date.parse(task.claim_expires_at || "");
    if (task.owner && Number.isFinite(claimExpiry) && claimExpiry < Date.now()) {
      items.push({
        id: `claim-${task.task_id}`,
        title: `Claim vencido \xB7 ${task.task_id}`,
        detail: clipped(task.owner)
      });
    }
  }
  if (snapshot.summary.blocked > visibleBlocked) {
    items.push({
      id: "blocked-summary",
      title: `${snapshot.summary.blocked - visibleBlocked} tareas bloqueadas adicionales`,
      detail: "revisar task boards"
    });
  }
  for (const job of jobs.slice(0, MAX_VISIBLE_ROWS)) {
    if (needsAttention(job.status) || job.error_code) {
      items.push({
        id: `job-${job.job_id}`,
        title: `${job.error_code || job.status} \xB7 ${job.role || "delegate"}`,
        detail: `${shortID(job.job_id)} \xB7 ${job.transport || "unknown"}`
      });
    }
  }
  if (snapshotError) {
    items.push({
      id: "snapshot-error",
      title: "Snapshot no disponible",
      detail: clipped(snapshotError, 35)
    });
  }
  return items;
}
function operationalCounts(snapshot, jobs, attention) {
  return {
    active: snapshot.tasks.filter((task) => task.status === "in_progress").length + jobs.filter((job) => isRunning(job.status)).length,
    review: snapshot.tasks.filter((task) => task.status === "in_review").length,
    attention: attention.length
  };
}
function Section(props) {
  return (() => {
    var _el$ = _$createElement("box"), _el$2 = _$createElement("text");
    _$insertNode(_el$, _el$2);
    _$setProp(_el$, "flexDirection", "column");
    _$setProp(_el$2, "selectable", false);
    _$insert(_el$2, () => `${props.expanded() ? "\u25BC" : "\u25B6"} ${props.title} \xB7 ${props.count}`);
    _$insert(_el$, _$createComponent(Show, {
      get when() {
        return props.expanded();
      },
      get children() {
        return props.children;
      }
    }), null);
    _$effect((_p$) => {
      var _v$ = props.theme.text, _v$2 = props.onToggle;
      _v$ !== _p$.e && (_p$.e = _$setProp(_el$2, "fg", _v$, _p$.e));
      _v$2 !== _p$.t && (_p$.t = _$setProp(_el$2, "onMouseDown", _v$2, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$;
  })();
}
function TaskRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$3 = _$createElement("text");
        _$insertNode(_el$3, _$createTextNode(` Sin tareas durables activas`));
        _$effect((_$p) => _$setProp(_el$3, "fg", props.theme.textMuted, _$p));
        return _el$3;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.tasks.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (task) => (() => {
          var _el$5 = _$createElement("box"), _el$6 = _$createElement("box"), _el$7 = _$createElement("text"), _el$8 = _$createElement("text"), _el$9 = _$createElement("text");
          _$insertNode(_el$5, _el$6);
          _$insertNode(_el$5, _el$9);
          _$setProp(_el$5, "flexDirection", "column");
          _$insertNode(_el$6, _el$7);
          _$insertNode(_el$6, _el$8);
          _$setProp(_el$6, "flexDirection", "row");
          _$insert(_el$7, () => `${statusIcon(task.status)} `);
          _$insert(_el$8, () => clipped(`${task.task_id} \xB7 ${task.title}`));
          _$insert(_el$9, () => `  ${clipped(task.board_id, 14)} \xB7 ${task.status}${task.owner ? ` \xB7 ${clipped(task.owner, 10)}` : ""}${task.lease_count ? ` \xB7 ${task.lease_count} lease` : ""}`);
          _$effect((_p$) => {
            var _v$3 = statusColor(task.status, props.theme), _v$4 = props.theme.text, _v$5 = props.theme.textMuted;
            _v$3 !== _p$.e && (_p$.e = _$setProp(_el$7, "fg", _v$3, _p$.e));
            _v$4 !== _p$.t && (_p$.t = _$setProp(_el$8, "fg", _v$4, _p$.t));
            _v$5 !== _p$.a && (_p$.a = _$setProp(_el$9, "fg", _v$5, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$5;
        })()
      });
    }
  });
}
function DelegationRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.jobs.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$0 = _$createElement("text");
        _$insertNode(_el$0, _$createTextNode(` Sin delegaciones registradas`));
        _$effect((_$p) => _$setProp(_el$0, "fg", props.theme.textMuted, _$p));
        return _el$0;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.jobs.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (job) => (() => {
          var _el$10 = _$createElement("box"), _el$11 = _$createElement("box"), _el$12 = _$createElement("text"), _el$13 = _$createElement("text"), _el$14 = _$createElement("text"), _el$15 = _$createElement("text");
          _$insertNode(_el$10, _el$11);
          _$insertNode(_el$10, _el$15);
          _$setProp(_el$10, "flexDirection", "column");
          _$insertNode(_el$11, _el$12);
          _$insertNode(_el$11, _el$13);
          _$insertNode(_el$11, _el$14);
          _$setProp(_el$11, "flexDirection", "row");
          _$insert(_el$12, () => `${statusIcon(job.status)} `);
          _$insert(_el$13, () => job.role || "delegate");
          _$insert(_el$14, () => ` \xB7 ${job.status}`);
          _$insert(_el$15, () => `  ${shortID(job.job_id)} \xB7 ${job.transport || "unknown"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}`);
          _$effect((_p$) => {
            var _v$6 = statusColor(job.status, props.theme), _v$7 = props.theme.text, _v$8 = props.theme.textMuted, _v$9 = props.theme.textMuted;
            _v$6 !== _p$.e && (_p$.e = _$setProp(_el$12, "fg", _v$6, _p$.e));
            _v$7 !== _p$.t && (_p$.t = _$setProp(_el$13, "fg", _v$7, _p$.t));
            _v$8 !== _p$.a && (_p$.a = _$setProp(_el$14, "fg", _v$8, _p$.a));
            _v$9 !== _p$.o && (_p$.o = _$setProp(_el$15, "fg", _v$9, _p$.o));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0,
            o: void 0
          });
          return _el$10;
        })()
      });
    }
  });
}
function AttentionRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.items.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$16 = _$createElement("text");
        _$insertNode(_el$16, _$createTextNode(` Sin alertas`));
        _$effect((_$p) => _$setProp(_el$16, "fg", props.theme.textMuted, _$p));
        return _el$16;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$18 = _$createElement("box"), _el$19 = _$createElement("text"), _el$20 = _$createElement("text");
          _$insertNode(_el$18, _el$19);
          _$insertNode(_el$18, _el$20);
          _$setProp(_el$18, "flexDirection", "column");
          _$insert(_el$19, () => `! ${clipped(item.title)}`);
          _$insert(_el$20, () => `  ${item.detail}`);
          _$effect((_p$) => {
            var _v$0 = props.theme.error, _v$1 = props.theme.textMuted;
            _v$0 !== _p$.e && (_p$.e = _$setProp(_el$19, "fg", _v$0, _p$.e));
            _v$1 !== _p$.t && (_p$.t = _$setProp(_el$20, "fg", _v$1, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$18;
        })()
      });
    }
  });
}
function SidebarStatus(props) {
  const attention = createMemo(() => attentionItems(props.snapshot(), props.jobs(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.jobs(), attention()));
  const stale = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    return Boolean(props.snapshotError()) || !Number.isFinite(generated) || props.now() - generated > SNAPSHOT_STALE_MS;
  });
  return (() => {
    var _el$21 = _$createElement("box"), _el$22 = _$createElement("text"), _el$25 = _$createElement("box"), _el$26 = _$createElement("text"), _el$27 = _$createElement("text"), _el$29 = _$createElement("text"), _el$30 = _$createElement("text"), _el$32 = _$createElement("text"), _el$33 = _$createElement("text");
    _$insertNode(_el$21, _el$22);
    _$insertNode(_el$21, _el$25);
    _$insertNode(_el$21, _el$33);
    _$setProp(_el$21, "flexDirection", "column");
    _$insertNode(_el$22, _$createTextNode(`Cortex-IA`));
    _$insert(_el$21, _$createComponent(Show, {
      get when() {
        return props.snapshot().project_root;
      },
      get children() {
        var _el$24 = _$createElement("text");
        _$insert(_el$24, () => `Proyecto \xB7 ${path.basename(props.snapshot().project_root)}`);
        _$effect((_$p) => _$setProp(_el$24, "fg", props.theme.textMuted, _$p));
        return _el$24;
      }
    }), _el$25);
    _$insertNode(_el$25, _el$26);
    _$insertNode(_el$25, _el$27);
    _$insertNode(_el$25, _el$29);
    _$insertNode(_el$25, _el$30);
    _$insertNode(_el$25, _el$32);
    _$setProp(_el$25, "flexDirection", "row");
    _$insert(_el$26, () => `\u25CF ${counts().active}`);
    _$insertNode(_el$27, _$createTextNode(` \xB7 `));
    _$insert(_el$29, () => `\u25C6 ${counts().review}`);
    _$insertNode(_el$30, _$createTextNode(` \xB7 `));
    _$insert(_el$32, () => `\u2715 ${counts().attention}`);
    _$insert(_el$21, _$createComponent(Section, {
      title: "Task board",
      get count() {
        return props.snapshot().summary.active_tasks;
      },
      get expanded() {
        return props.tasksExpanded;
      },
      get onToggle() {
        return props.toggleTasks;
      },
      get theme() {
        return props.theme;
      },
      get children() {
        return _$createComponent(TaskRows, {
          get tasks() {
            return props.snapshot().tasks;
          },
          get theme() {
            return props.theme;
          }
        });
      }
    }), _el$33);
    _$insert(_el$21, _$createComponent(Section, {
      title: "Delegaciones",
      get count() {
        return props.jobs().length;
      },
      get expanded() {
        return props.delegationsExpanded;
      },
      get onToggle() {
        return props.toggleDelegations;
      },
      get theme() {
        return props.theme;
      },
      get children() {
        return _$createComponent(DelegationRows, {
          get jobs() {
            return props.jobs();
          },
          get theme() {
            return props.theme;
          }
        });
      }
    }), _el$33);
    _$insert(_el$21, _$createComponent(Section, {
      title: "Atenci\xF3n",
      get count() {
        return attention().length;
      },
      get expanded() {
        return props.attentionExpanded;
      },
      get onToggle() {
        return props.toggleAttention;
      },
      get theme() {
        return props.theme;
      },
      get children() {
        return _$createComponent(AttentionRows, {
          get items() {
            return attention();
          },
          get theme() {
            return props.theme;
          }
        });
      }
    }), _el$33);
    _$insert(_el$33, () => stale() ? "snapshot obsoleto" : "snapshot actualizado");
    _$effect((_p$) => {
      var _v$10 = props.theme.text, _v$11 = props.theme.warning, _v$12 = props.theme.textMuted, _v$13 = props.theme.accent, _v$14 = props.theme.textMuted, _v$15 = props.theme.error, _v$16 = stale() ? props.theme.warning : props.theme.textMuted;
      _v$10 !== _p$.e && (_p$.e = _$setProp(_el$22, "fg", _v$10, _p$.e));
      _v$11 !== _p$.t && (_p$.t = _$setProp(_el$26, "fg", _v$11, _p$.t));
      _v$12 !== _p$.a && (_p$.a = _$setProp(_el$27, "fg", _v$12, _p$.a));
      _v$13 !== _p$.o && (_p$.o = _$setProp(_el$29, "fg", _v$13, _p$.o));
      _v$14 !== _p$.i && (_p$.i = _$setProp(_el$30, "fg", _v$14, _p$.i));
      _v$15 !== _p$.n && (_p$.n = _$setProp(_el$32, "fg", _v$15, _p$.n));
      _v$16 !== _p$.s && (_p$.s = _$setProp(_el$33, "fg", _v$16, _p$.s));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0,
      s: void 0
    });
    return _el$21;
  })();
}
function HomeBottomStatus(props) {
  const attention = createMemo(() => attentionItems(props.snapshot(), props.jobs(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.jobs(), attention()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$34 = _$createElement("box"), _el$35 = _$createElement("text"), _el$37 = _$createElement("text"), _el$38 = _$createElement("text"), _el$40 = _$createElement("text"), _el$41 = _$createElement("text"), _el$43 = _$createElement("text");
      _$insertNode(_el$34, _el$35);
      _$insertNode(_el$34, _el$37);
      _$insertNode(_el$34, _el$38);
      _$insertNode(_el$34, _el$40);
      _$insertNode(_el$34, _el$41);
      _$insertNode(_el$34, _el$43);
      _$setProp(_el$34, "paddingLeft", 1);
      _$setProp(_el$34, "paddingRight", 1);
      _$setProp(_el$34, "flexDirection", "row");
      _$insertNode(_el$35, _$createTextNode(`Cortex `));
      _$insert(_el$37, () => `\u25CF ${counts().active}`);
      _$insertNode(_el$38, _$createTextNode(` \xB7 `));
      _$insert(_el$40, () => `\u25C6 ${counts().review}`);
      _$insertNode(_el$41, _$createTextNode(` \xB7 `));
      _$insert(_el$43, () => `\u2715 ${counts().attention}`);
      _$effect((_p$) => {
        var _v$17 = props.theme.text, _v$18 = props.theme.warning, _v$19 = props.theme.textMuted, _v$20 = props.theme.accent, _v$21 = props.theme.textMuted, _v$22 = props.theme.error;
        _v$17 !== _p$.e && (_p$.e = _$setProp(_el$35, "fg", _v$17, _p$.e));
        _v$18 !== _p$.t && (_p$.t = _$setProp(_el$37, "fg", _v$18, _p$.t));
        _v$19 !== _p$.a && (_p$.a = _$setProp(_el$38, "fg", _v$19, _p$.a));
        _v$20 !== _p$.o && (_p$.o = _$setProp(_el$40, "fg", _v$20, _p$.o));
        _v$21 !== _p$.i && (_p$.i = _$setProp(_el$41, "fg", _v$21, _p$.i));
        _v$22 !== _p$.n && (_p$.n = _$setProp(_el$43, "fg", _v$22, _p$.n));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0,
        n: void 0
      });
      return _el$34;
    }
  });
}
function initialize(api, disposeRoot) {
  const source = eventsPath();
  const projectDirectory = api.state.path.directory || process.cwd();
  const [eventJobs, setEventJobs] = createSignal([]);
  const [snapshot, setSnapshot] = createSignal(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  const [now, setNow] = createSignal(Date.now());
  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(api.kv.get(DELEGATIONS_EXPANDED_KEY, true) !== false);
  const [attentionExpanded, setAttentionExpanded] = createSignal(api.kv.get(ATTENTION_EXPANDED_KEY, true) !== false);
  const jobs = createMemo(() => mergedDelegations(snapshot(), eventJobs()));
  let disposed = false;
  let snapshotInFlight = false;
  let sequence = 0;
  let offset = 0;
  let remainder = "";
  let previousAttentionCount = 0;
  const togglePreference = (key, value, setter) => {
    const next = !value();
    setter(next);
    api.kv.set(key, next);
  };
  const applyEvent = (event, notify) => {
    if (event.kind !== "delegation" || typeof event.job_id !== "string" || !event.job_id || typeof event.status !== "string" || !event.status) return;
    if (event.workspace && snapshot().project_root && projectKey(event.workspace) !== projectKey(snapshot().project_root)) return;
    sequence += 1;
    setEventJobs((current) => {
      const next = current.filter((job) => job.job_id !== event.job_id);
      next.unshift({
        ...event,
        job_id: event.job_id,
        status: event.status,
        sequence
      });
      return next.slice(0, MAX_RETAINED_JOBS);
    });
    if (notify && event.workspace && projectKey(event.workspace) === projectKey(snapshot().project_root)) showToast(api, toastFor(event));
  };
  const applyLines = (content, notify, dropFirst) => {
    const lines = content.split("\n");
    if (dropFirst) lines.shift();
    remainder = lines.pop() || "";
    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        applyEvent(JSON.parse(line), notify);
      } catch {
      }
    }
  };
  const hydrateEvents = () => {
    let size;
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
  const readEvents = () => {
    let size;
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
  const readSnapshot = () => {
    if (snapshotInFlight || disposed) return;
    snapshotInFlight = true;
    execFile(cortexExecutable(), ["ui", "snapshot", "--project", projectDirectory], {
      encoding: "utf8",
      maxBuffer: 512 * 1024,
      timeout: 7500,
      windowsHide: true
    }, (error, stdout) => {
      snapshotInFlight = false;
      if (disposed) return;
      if (error) {
        setSnapshotError(error.message || "cortex-ia ui snapshot failed");
        return;
      }
      try {
        const next = JSON.parse(stdout);
        if (next.schema_version !== 1 || !Array.isArray(next.tasks) || !Array.isArray(next.delegations)) {
          throw new Error("snapshot schema is incompatible");
        }
        setSnapshot(next);
        setSnapshotError("");
      } catch (parseError) {
        setSnapshotError(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
      }
    });
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
  const clock = setInterval(() => setNow(Date.now()), 1e3);
  api.slots.register({
    order: 85,
    slots: {
      sidebar_content(ctx) {
        return _$createComponent(SidebarStatus, {
          snapshot,
          jobs,
          snapshotError,
          now,
          tasksExpanded,
          delegationsExpanded,
          attentionExpanded,
          toggleTasks: () => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded),
          toggleDelegations: () => togglePreference(DELEGATIONS_EXPANDED_KEY, delegationsExpanded, setDelegationsExpanded),
          toggleAttention: () => togglePreference(ATTENTION_EXPANDED_KEY, attentionExpanded, setAttentionExpanded),
          get theme() {
            return ctx.theme.current;
          }
        });
      },
      home_bottom(ctx) {
        return _$createComponent(HomeBottomStatus, {
          snapshot,
          jobs,
          snapshotError,
          get theme() {
            return ctx.theme.current;
          }
        });
      }
    }
  });
  api.lifecycle.onDispose(() => {
    disposed = true;
    clearInterval(eventPoll);
    clearInterval(snapshotPoll);
    clearInterval(clock);
    disposeRoot();
  });
}
var tui = async (api) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};
var plugin = {
  id: "cortex-ia.delegation-status",
  tui
};
var cortex_ia_tui_default = plugin;
export {
  cortex_ia_tui_default as default
};
