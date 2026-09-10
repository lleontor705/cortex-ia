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
import path from "path";
import { For, Show, createEffect, createMemo, createRoot, createSignal } from "solid-js";
var SNAPSHOT_POLL_INTERVAL_MS = 2500;
var SNAPSHOT_STALE_MS = 1e4;
var MAX_VISIBLE_ROWS = 4;
var SPINNER_FRAMES = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"];
var TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
var DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
var ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
var EMPTY_SNAPSHOT = {
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
    total_attention: 0
  },
  counts: {
    active: 0,
    review: 0,
    attention: 0
  },
  attention: [],
  tasks: [],
  delegations: []
};
function cortexExecutable() {
  return process.env.CORTEX_IA_BIN || "cortex-ia";
}
function shortID(id) {
  return id.length > 13 ? `${id.slice(0, 8)}\u2026${id.slice(-4)}` : id;
}
function clipped(value, limit = 28) {
  const text = value.trim();
  return text.length > limit ? `${text.slice(0, limit - 1)}\u2026` : text;
}
function formatDuration(ms) {
  if (ms <= 0 || !Number.isFinite(ms)) return "00:00";
  const totalSecs = Math.floor(ms / 1e3);
  const mins = Math.floor(totalSecs / 60);
  const secs = totalSecs % 60;
  if (mins >= 60) {
    const hours = Math.floor(mins / 60);
    const remMins = mins % 60;
    return `${hours}h ${remMins}m`;
  }
  return `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}
function statusColor(status, theme) {
  if (["succeeded", "done", "superseded"].includes(status)) return theme.success;
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return theme.error;
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return theme.warning;
  if (status === "in_review") return theme.accent;
  return theme.textMuted;
}
function statusIcon(status, spinner) {
  if (["succeeded", "done", "superseded"].includes(status)) return "\u2713";
  if (["failed", "timed_out", "lost", "cancelled", "blocked"].includes(status)) return "\u2715";
  if (status === "in_review") return "\u25C6";
  if (["accepted", "starting", "running", "queued", "in_progress"].includes(status)) return spinner;
  return "\u25CB";
}
function attentionItems(snapshot, snapshotError) {
  const items = snapshot.attention.map((item) => ({
    id: item.id,
    title: item.title,
    detail: `${item.kind} \xB7 ${shortID(item.entity_id)}`
  }));
  if (snapshotError) {
    items.push({
      id: "snapshot-error",
      title: "Snapshot no disponible",
      detail: clipped(snapshotError, 35)
    });
  }
  return items;
}
function operationalCounts(snapshot, snapshotError) {
  return {
    ...snapshot.counts,
    attention: snapshot.counts.attention + (snapshotError ? 1 : 0)
  };
}
function currentSessionID(api) {
  const route = api.route.current;
  const id = route.name === "session" ? route.params?.sessionID : void 0;
  return typeof id === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(id) ? id : void 0;
}
function nativeSessionActivity(api) {
  const id = currentSessionID(api);
  if (!id) return void 0;
  if (api.state.session.get(id)?.id !== id) return "unknown";
  const status = api.state.session.status(id)?.type;
  return status === "busy" || status === "idle" || status === "retry" ? status : "unknown";
}
function conversationScope(api) {
  const sessionID = currentSessionID(api);
  if (!sessionID) return void 0;
  let current = sessionID;
  const seen = /* @__PURE__ */ new Set();
  while (seen.size < 64 && /^[A-Za-z0-9_-]{1,256}$/.test(current) && !seen.has(current)) {
    seen.add(current);
    const session = api.state.session.get(current);
    if (!session || session.id !== current) return void 0;
    if (!session.parentID) return {
      sessionID,
      rootSessionID: current,
      project: api.state.path.directory
    };
    current = session.parentID;
  }
  return void 0;
}
function Section(props) {
  return (() => {
    var _el$ = _$createElement("box"), _el$2 = _$createElement("text");
    _$insertNode(_el$, _el$2);
    _$setProp(_el$, "flexDirection", "column");
    _$setProp(_el$, "marginTop", 1);
    _$setProp(_el$2, "selectable", false);
    _$insert(_el$2, () => `${props.expanded() ? "\u25BC" : "\u25B6"} ${props.title}${props.badge ? ` \xB7 ${props.badge}` : ""}`);
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
function ProgressBar(props) {
  const w = props.width || 14;
  const ratio = props.total > 0 ? Math.min(1, Math.max(0, props.done / props.total)) : 0;
  const filled = Math.round(ratio * w);
  const empty = Math.max(0, w - filled);
  const pct = Math.round(ratio * 100);
  return (() => {
    var _el$3 = _$createElement("box"), _el$4 = _$createElement("text"), _el$5 = _$createElement("text"), _el$6 = _$createElement("text");
    _$insertNode(_el$3, _el$4);
    _$insertNode(_el$3, _el$5);
    _$insertNode(_el$3, _el$6);
    _$setProp(_el$3, "flexDirection", "row");
    _$insert(_el$4, () => "\u2588".repeat(filled));
    _$insert(_el$5, () => "\u2591".repeat(empty));
    _$insert(_el$6, () => ` ${pct}% (${props.done}/${props.total})`);
    _$effect((_p$) => {
      var _v$3 = props.theme.success, _v$4 = props.theme.textMuted, _v$5 = props.theme.textMuted;
      _v$3 !== _p$.e && (_p$.e = _$setProp(_el$4, "fg", _v$3, _p$.e));
      _v$4 !== _p$.t && (_p$.t = _$setProp(_el$5, "fg", _v$4, _p$.t));
      _v$5 !== _p$.a && (_p$.a = _$setProp(_el$6, "fg", _v$5, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$3;
  })();
}
function ActiveTaskHero(props) {
  const elapsed = createMemo(() => {
    const updated = Date.parse(props.task.updated_at);
    if (!Number.isFinite(updated)) return "00:00";
    return formatDuration(props.now() - updated);
  });
  const ttlRemaining = createMemo(() => {
    if (!props.task.claim_expires_at) return void 0;
    const exp = Date.parse(props.task.claim_expires_at);
    if (!Number.isFinite(exp)) return void 0;
    const diff = exp - props.now();
    return diff > 0 ? formatDuration(diff) : "expirado";
  });
  return (() => {
    var _el$7 = _$createElement("box"), _el$8 = _$createElement("box"), _el$9 = _$createElement("text"), _el$0 = _$createElement("text"), _el$1 = _$createElement("box"), _el$10 = _$createElement("text"), _el$12 = _$createElement("text"), _el$13 = _$createElement("box");
    _$insertNode(_el$7, _el$8);
    _$insertNode(_el$7, _el$1);
    _$insertNode(_el$7, _el$13);
    _$setProp(_el$7, "flexDirection", "column");
    _$setProp(_el$7, "marginTop", 1);
    _$setProp(_el$7, "paddingLeft", 1);
    _$setProp(_el$7, "borderStyle", "rounded");
    _$insertNode(_el$8, _el$9);
    _$insertNode(_el$8, _el$0);
    _$setProp(_el$8, "flexDirection", "row");
    _$insert(_el$9, () => `${props.spinner()} `);
    _$insert(_el$0, () => clipped(`${props.task.task_id} \xB7 ${props.task.title}`, 26));
    _$insertNode(_el$1, _el$10);
    _$insertNode(_el$1, _el$12);
    _$setProp(_el$1, "flexDirection", "row");
    _$insertNode(_el$10, _$createTextNode(` EN CURSO `));
    _$insert(_el$12, () => `\xB7 \u23F1 ${elapsed()}`);
    _$setProp(_el$13, "flexDirection", "row");
    _$insert(_el$13, _$createComponent(Show, {
      get when() {
        return props.activeDelegation;
      },
      get fallback() {
        return (() => {
          var _el$14 = _$createElement("text");
          _$insert(_el$14, () => `  \u{1F9D1}\u200D\u{1F4BB} Nativo OpenCode${props.task.owner ? ` (${clipped(props.task.owner, 10)})` : ""}`);
          _$effect((_$p) => _$setProp(_el$14, "fg", props.theme.accent, _$p));
          return _el$14;
        })();
      },
      children: (del) => (() => {
        var _el$15 = _$createElement("text");
        _$insert(_el$15, () => `  \u26A1 AGY ${del().transport || "direct"}${del().pane_id ? ` \xB7 ${del().pane_id}` : ""}${del().attempt ? ` \xB7 int #${del().attempt}` : ""}`);
        _$effect((_$p) => _$setProp(_el$15, "fg", props.theme.accent, _$p));
        return _el$15;
      })()
    }));
    _$insert(_el$7, _$createComponent(Show, {
      get when() {
        return ttlRemaining();
      },
      children: (ttl) => (() => {
        var _el$16 = _$createElement("text");
        _$insert(_el$16, () => `  \u{1F6E1} Lease: ${ttl()}${props.task.lease_count ? ` \xB7 ${props.task.lease_count} lock` : ""}`);
        _$effect((_$p) => _$setProp(_el$16, "fg", ttl() === "expirado" ? props.theme.error : props.theme.textMuted, _$p));
        return _el$16;
      })()
    }), null);
    _$effect((_p$) => {
      var _v$6 = props.theme.warning, _v$7 = props.theme.warning, _v$8 = props.theme.text, _v$9 = props.theme.warning, _v$0 = props.theme.textMuted;
      _v$6 !== _p$.e && (_p$.e = _$setProp(_el$7, "borderColor", _v$6, _p$.e));
      _v$7 !== _p$.t && (_p$.t = _$setProp(_el$9, "fg", _v$7, _p$.t));
      _v$8 !== _p$.a && (_p$.a = _$setProp(_el$0, "fg", _v$8, _p$.a));
      _v$9 !== _p$.o && (_p$.o = _$setProp(_el$10, "fg", _v$9, _p$.o));
      _v$0 !== _p$.i && (_p$.i = _$setProp(_el$12, "fg", _v$0, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
    });
    return _el$7;
  })();
}
function TaskRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$17 = _$createElement("text");
        _$insertNode(_el$17, _$createTextNode(` Sin tareas durables activas`));
        _$effect((_$p) => _$setProp(_el$17, "fg", props.theme.textMuted, _$p));
        return _el$17;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.tasks.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (task) => (() => {
          var _el$19 = _$createElement("box"), _el$20 = _$createElement("box"), _el$21 = _$createElement("text"), _el$22 = _$createElement("text"), _el$23 = _$createElement("text");
          _$insertNode(_el$19, _el$20);
          _$insertNode(_el$19, _el$23);
          _$setProp(_el$19, "flexDirection", "column");
          _$setProp(_el$19, "marginTop", 0);
          _$insertNode(_el$20, _el$21);
          _$insertNode(_el$20, _el$22);
          _$setProp(_el$20, "flexDirection", "row");
          _$insert(_el$21, () => `${statusIcon(task.status, props.spinner())} `);
          _$insert(_el$22, () => clipped(`${task.task_id} \xB7 ${task.title}`));
          _$insert(_el$23, () => `  ${clipped(task.board_id, 12)} \xB7 ${task.status}${task.owner ? ` \xB7 ${clipped(task.owner, 8)}` : ""}${task.lease_count ? ` \xB7 ${task.lease_count} lk` : ""}`);
          _$effect((_p$) => {
            var _v$1 = statusColor(task.status, props.theme), _v$10 = task.status === "in_progress" ? props.theme.warning : props.theme.text, _v$11 = props.theme.textMuted;
            _v$1 !== _p$.e && (_p$.e = _$setProp(_el$21, "fg", _v$1, _p$.e));
            _v$10 !== _p$.t && (_p$.t = _$setProp(_el$22, "fg", _v$10, _p$.t));
            _v$11 !== _p$.a && (_p$.a = _$setProp(_el$23, "fg", _v$11, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$19;
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
        var _el$24 = _$createElement("text");
        _$insertNode(_el$24, _$createTextNode(` Sin ejecuciones delegadas`));
        _$effect((_$p) => _$setProp(_el$24, "fg", props.theme.textMuted, _$p));
        return _el$24;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.jobs.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` \xB7 ${formatDuration(props.now() - t)}` : "";
          });
          return (() => {
            var _el$26 = _$createElement("box"), _el$27 = _$createElement("box"), _el$28 = _$createElement("text"), _el$29 = _$createElement("text"), _el$30 = _$createElement("text"), _el$31 = _$createElement("text");
            _$insertNode(_el$26, _el$27);
            _$insertNode(_el$26, _el$31);
            _$setProp(_el$26, "flexDirection", "column");
            _$setProp(_el$26, "marginTop", 0);
            _$insertNode(_el$27, _el$28);
            _$insertNode(_el$27, _el$29);
            _$insertNode(_el$27, _el$30);
            _$setProp(_el$27, "flexDirection", "row");
            _$insert(_el$28, () => `${statusIcon(job.status, props.spinner())} `);
            _$insert(_el$29, () => job.role || "worker");
            _$insert(_el$30, () => ` \xB7 ${job.status}${elapsed()}`);
            _$insert(_el$31, () => `  ${shortID(job.job_id)} \xB7 ${job.transport || "direct"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}${job.attempt ? ` \xB7 int #${job.attempt}` : ""}`);
            _$effect((_p$) => {
              var _v$12 = statusColor(job.status, props.theme), _v$13 = isRunning ? props.theme.warning : props.theme.text, _v$14 = props.theme.textMuted, _v$15 = props.theme.textMuted;
              _v$12 !== _p$.e && (_p$.e = _$setProp(_el$28, "fg", _v$12, _p$.e));
              _v$13 !== _p$.t && (_p$.t = _$setProp(_el$29, "fg", _v$13, _p$.t));
              _v$14 !== _p$.a && (_p$.a = _$setProp(_el$30, "fg", _v$14, _p$.a));
              _v$15 !== _p$.o && (_p$.o = _$setProp(_el$31, "fg", _v$15, _p$.o));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0
            });
            return _el$26;
          })();
        }
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
        var _el$32 = _$createElement("text");
        _$insertNode(_el$32, _$createTextNode(` Sin alertas`));
        _$effect((_$p) => _$setProp(_el$32, "fg", props.theme.textMuted, _$p));
        return _el$32;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$34 = _$createElement("box"), _el$35 = _$createElement("text"), _el$36 = _$createElement("text");
          _$insertNode(_el$34, _el$35);
          _$insertNode(_el$34, _el$36);
          _$setProp(_el$34, "flexDirection", "column");
          _$insert(_el$35, () => `! ${clipped(item.title)}`);
          _$insert(_el$36, () => `  ${item.detail}`);
          _$effect((_p$) => {
            var _v$16 = props.theme.error, _v$17 = props.theme.textMuted;
            _v$16 !== _p$.e && (_p$.e = _$setProp(_el$35, "fg", _v$16, _p$.e));
            _v$17 !== _p$.t && (_p$.t = _$setProp(_el$36, "fg", _v$17, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$34;
        })()
      });
    }
  });
}
function SidebarStatus(props) {
  const attention = createMemo(() => attentionItems(props.snapshot(), props.snapshotError()));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const stale = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    return Boolean(props.snapshotError()) || !Number.isFinite(generated) || props.now() - generated > SNAPSHOT_STALE_MS;
  });
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const activeDelegation = createMemo(() => props.jobs().find((j) => ["running", "starting", "accepted"].includes(j.status)));
  const activeExecutionsCount = createMemo(() => {
    const s = props.snapshot().summary;
    if (typeof s.active_executions === "number") return s.active_executions;
    const inProg = s.in_progress || 0;
    const actDel = s.active_delegations || 0;
    return Math.max(inProg, actDel);
  });
  const isExecuting = createMemo(() => props.nativeActivity() === "busy" || activeExecutionsCount() > 0 || Boolean(activeDelegation()));
  const activeDelegationsCount = createMemo(() => {
    if (typeof props.snapshot().summary.active_delegations === "number") {
      return props.snapshot().summary.active_delegations;
    }
    return props.jobs().filter((j) => ["running", "starting", "accepted"].includes(j.status)).length;
  });
  const totalDelegationsCount = createMemo(() => props.snapshot().summary.total_delegations || props.jobs().length);
  const doneTasks = createMemo(() => props.snapshot().summary.done || 0);
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);
  return (() => {
    var _el$37 = _$createElement("box"), _el$38 = _$createElement("box"), _el$39 = _$createElement("text");
    _$insertNode(_el$37, _el$38);
    _$setProp(_el$37, "flexDirection", "column");
    _$insertNode(_el$38, _el$39);
    _$setProp(_el$38, "flexDirection", "row");
    _$insertNode(_el$39, _$createTextNode(`\u26A1 CORTEX-IA `));
    _$insert(_el$38, _$createComponent(Show, {
      get when() {
        return isExecuting();
      },
      get fallback() {
        return (() => {
          var _el$60 = _$createElement("text");
          _$insertNode(_el$60, _$createTextNode(`[EN ESPERA]`));
          _$effect((_$p) => _$setProp(_el$60, "fg", props.theme.textMuted, _$p));
          return _el$60;
        })();
      },
      get children() {
        var _el$41 = _$createElement("text");
        _$insert(_el$41, () => `[${props.spinner()} ACTIVO]`);
        _$effect((_$p) => _$setProp(_el$41, "fg", props.theme.warning, _$p));
        return _el$41;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return props.nativeActivity();
      },
      get children() {
        var _el$42 = _$createElement("text");
        _$insert(_el$42, () => `Sesi\xF3n \xB7 ${{
          busy: "trabajando\u2026",
          idle: "en espera",
          retry: "reintentando",
          unknown: "no disponible"
        }[props.nativeActivity() ?? "unknown"]}`);
        _$effect((_$p) => _$setProp(_el$42, "fg", props.nativeActivity() === "busy" || props.nativeActivity() === "retry" ? props.theme.warning : props.theme.textMuted, _$p));
        return _el$42;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return props.snapshot().project_root;
      },
      get children() {
        var _el$43 = _$createElement("text");
        _$insert(_el$43, () => `Proyecto \xB7 ${path.basename(props.snapshot().project_root)}`);
        _$effect((_$p) => _$setProp(_el$43, "fg", props.theme.textMuted, _$p));
        return _el$43;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$44 = _$createElement("text");
        _$insertNode(_el$44, _$createTextNode(`Conversaci\xF3n no disponible \xB7 esperando metadatos`));
        _$effect((_$p) => _$setProp(_el$44, "fg", props.theme.warning, _$p));
        return _el$44;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$46 = _$createElement("text");
        _$insertNode(_el$46, _$createTextNode(`Cargando estado de la conversaci\xF3n\u2026`));
        _$effect((_$p) => _$setProp(_el$46, "fg", props.theme.textMuted, _$p));
        return _el$46;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$48 = _$createElement("text");
        _$insertNode(_el$48, _$createTextNode(`No se pudo actualizar \xB7 datos no confirmados`));
        _$effect((_$p) => _$setProp(_el$48, "fg", props.theme.error, _$p));
        return _el$48;
      }
    }), null);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!props.scopeReady())() && Boolean(props.snapshot().generated_at);
      },
      get children() {
        return [(() => {
          var _el$50 = _$createElement("box"), _el$51 = _$createElement("text"), _el$52 = _$createElement("text"), _el$54 = _$createElement("text"), _el$55 = _$createElement("text"), _el$57 = _$createElement("text");
          _$insertNode(_el$50, _el$51);
          _$insertNode(_el$50, _el$52);
          _$insertNode(_el$50, _el$54);
          _$insertNode(_el$50, _el$55);
          _$insertNode(_el$50, _el$57);
          _$setProp(_el$50, "flexDirection", "row");
          _$setProp(_el$50, "marginTop", 1);
          _$insert(_el$51, () => `${activeExecutionsCount() > 0 ? props.spinner() : "\u25CF"} ${activeExecutionsCount()} en curso`);
          _$insertNode(_el$52, _$createTextNode(` \u2502 `));
          _$insert(_el$54, () => `\u25C6 ${counts().review} rev`);
          _$insertNode(_el$55, _$createTextNode(` \u2502 `));
          _$insert(_el$57, () => `\u2715 ${counts().attention} alert`);
          _$effect((_p$) => {
            var _v$18 = activeExecutionsCount() > 0 ? props.theme.warning : props.theme.textMuted, _v$19 = props.theme.textMuted, _v$20 = counts().review > 0 ? props.theme.accent : props.theme.textMuted, _v$21 = props.theme.textMuted, _v$22 = counts().attention > 0 ? props.theme.error : props.theme.textMuted;
            _v$18 !== _p$.e && (_p$.e = _$setProp(_el$51, "fg", _v$18, _p$.e));
            _v$19 !== _p$.t && (_p$.t = _$setProp(_el$52, "fg", _v$19, _p$.t));
            _v$20 !== _p$.a && (_p$.a = _$setProp(_el$54, "fg", _v$20, _p$.a));
            _v$21 !== _p$.o && (_p$.o = _$setProp(_el$55, "fg", _v$21, _p$.o));
            _v$22 !== _p$.i && (_p$.i = _$setProp(_el$57, "fg", _v$22, _p$.i));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0,
            o: void 0,
            i: void 0
          });
          return _el$50;
        })(), _$createComponent(Show, {
          get when() {
            return totalTasks() > 0;
          },
          get children() {
            var _el$58 = _$createElement("box");
            _$setProp(_el$58, "flexDirection", "column");
            _$setProp(_el$58, "marginTop", 1);
            _$insert(_el$58, _$createComponent(ProgressBar, {
              get done() {
                return doneTasks();
              },
              get total() {
                return totalTasks();
              },
              get theme() {
                return props.theme;
              }
            }));
            return _el$58;
          }
        }), _$createComponent(Show, {
          get when() {
            return activeTask();
          },
          children: (task) => _$createComponent(ActiveTaskHero, {
            get task() {
              return task();
            },
            get activeDelegation() {
              return activeDelegation();
            },
            get now() {
              return props.now;
            },
            get spinner() {
              return props.spinner;
            },
            get theme() {
              return props.theme;
            }
          })
        }), _$createComponent(Section, {
          title: "Tablero de Tareas",
          get badge() {
            return `${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot`;
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
              get spinner() {
                return props.spinner;
              },
              get theme() {
                return props.theme;
              }
            });
          }
        }), _$createComponent(Section, {
          title: "Ejecuciones & Workers",
          get badge() {
            return `${activeDelegationsCount()} act / ${totalDelegationsCount()} tot`;
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
              get spinner() {
                return props.spinner;
              },
              get now() {
                return props.now;
              },
              get theme() {
                return props.theme;
              }
            });
          }
        }), _$createComponent(Section, {
          title: "Atenci\xF3n requerida",
          get badge() {
            return `${counts().attention}`;
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
        }), (() => {
          var _el$59 = _$createElement("text");
          _$insert(_el$59, () => stale() ? "snapshot obsoleto" : "snapshot actualizado");
          _$effect((_$p) => _$setProp(_el$59, "fg", stale() ? props.theme.warning : props.theme.textMuted, _$p));
          return _el$59;
        })()];
      }
    }), null);
    _$effect((_$p) => _$setProp(_el$39, "fg", props.theme.accent, _$p));
    return _el$37;
  })();
}
function HomeBottomStatus(props) {
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$62 = _$createElement("box"), _el$63 = _$createElement("text");
      _$insertNode(_el$62, _el$63);
      _$setProp(_el$62, "paddingLeft", 1);
      _$setProp(_el$62, "paddingRight", 1);
      _$setProp(_el$62, "flexDirection", "row");
      _$insertNode(_el$63, _$createTextNode(`\u26A1 Cortex `));
      _$insert(_el$62, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$65 = _$createElement("box"), _el$66 = _$createElement("text"), _el$67 = _$createElement("text"), _el$69 = _$createElement("text");
            _$insertNode(_el$65, _el$66);
            _$insertNode(_el$65, _el$67);
            _$insertNode(_el$65, _el$69);
            _$setProp(_el$65, "flexDirection", "row");
            _$insert(_el$66, () => `\u25CF ${counts().active} en curso`);
            _$insertNode(_el$67, _$createTextNode(` \xB7 `));
            _$insert(_el$69, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$65, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$70 = _$createElement("text");
                  _$insertNode(_el$70, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$70, "fg", props.theme.textMuted, _$p));
                  return _el$70;
                })(), (() => {
                  var _el$72 = _$createElement("text");
                  _$insert(_el$72, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$72, "fg", props.theme.error, _$p));
                  return _el$72;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$23 = props.theme.warning, _v$24 = props.theme.textMuted, _v$25 = props.theme.accent;
              _v$23 !== _p$.e && (_p$.e = _$setProp(_el$66, "fg", _v$23, _p$.e));
              _v$24 !== _p$.t && (_p$.t = _$setProp(_el$67, "fg", _v$24, _p$.t));
              _v$25 !== _p$.a && (_p$.a = _$setProp(_el$69, "fg", _v$25, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$65;
          })();
        },
        children: (task) => (() => {
          var _el$73 = _$createElement("box"), _el$74 = _$createElement("text"), _el$75 = _$createElement("text"), _el$76 = _$createElement("text");
          _$insertNode(_el$73, _el$74);
          _$insertNode(_el$73, _el$75);
          _$insertNode(_el$73, _el$76);
          _$setProp(_el$73, "flexDirection", "row");
          _$insert(_el$74, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$75, () => clipped(task().title, 20));
          _$insert(_el$76, () => ` \xB7 ${counts().active} activos`);
          _$effect((_p$) => {
            var _v$26 = props.theme.warning, _v$27 = props.theme.text, _v$28 = props.theme.textMuted;
            _v$26 !== _p$.e && (_p$.e = _$setProp(_el$74, "fg", _v$26, _p$.e));
            _v$27 !== _p$.t && (_p$.t = _$setProp(_el$75, "fg", _v$27, _p$.t));
            _v$28 !== _p$.a && (_p$.a = _$setProp(_el$76, "fg", _v$28, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$73;
        })()
      }), null);
      _$effect((_$p) => _$setProp(_el$63, "fg", props.theme.accent, _$p));
      return _el$62;
    }
  });
}
function initialize(api, disposeRoot) {
  const nativeActivity = createMemo(() => nativeSessionActivity(api));
  const scopeReady = createMemo(() => Boolean(conversationScope(api)?.project));
  const [snapshot, setSnapshot] = createSignal(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(api.kv.get(DELEGATIONS_EXPANDED_KEY, true) !== false);
  const [attentionExpanded, setAttentionExpanded] = createSignal(api.kv.get(ATTENTION_EXPANDED_KEY, true) !== false);
  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const jobs = createMemo(() => snapshot().delegations.map((job, sequence) => ({
    ...job,
    sequence
  })));
  let disposed = false;
  let generation = 0;
  let activeKey = "";
  let pendingGeneration;
  let previousAttentionCount = 0;
  const togglePreference = (key, value, setter) => {
    const next = !value();
    setter(next);
    api.kv.set(key, next);
  };
  const readSnapshot = () => {
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
    execFile(cortexExecutable(), ["ui", "snapshot", "--project", scope.project, "--session-id", scope.sessionID, "--root-session-id", scope.rootSessionID], {
      encoding: "utf8",
      maxBuffer: 512 * 1024,
      timeout: 7500,
      windowsHide: true
    }, (error, stdout) => {
      if (pendingGeneration === requestGeneration) pendingGeneration = void 0;
      if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api)) !== key) return;
      if (error) {
        setSnapshotError(error.message);
        return;
      }
      try {
        const next = JSON.parse(stdout);
        if (next.schema_version !== 2 || next.requested_session_id !== scope.sessionID || next.root_session_id !== scope.rootSessionID || !Array.isArray(next.tasks) || !Array.isArray(next.delegations) || !Array.isArray(next.attention) || !next.counts || !next.summary) {
          throw new Error("snapshot schema or conversation is incompatible");
        }
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
  const clock = setInterval(() => setNow(Date.now()), 1e3);
  const spinnerTimer = setInterval(() => setFrame((f) => (f + 1) % SPINNER_FRAMES.length), 90);
  api.slots.register({
    order: 85,
    slots: {
      sidebar_content(ctx) {
        return _$createComponent(SidebarStatus, {
          nativeActivity,
          scopeReady,
          snapshot,
          jobs,
          snapshotError,
          now,
          spinner,
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
          spinner,
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
    clearInterval(snapshotPoll);
    clearInterval(clock);
    clearInterval(spinnerTimer);
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
