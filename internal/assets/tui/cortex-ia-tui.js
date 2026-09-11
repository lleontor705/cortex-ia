// cortex-ia-tui.tsx
import { use as _$use } from "@opentui/solid";
import { memo as _$memo } from "@opentui/solid";
import { createComponent as _$createComponent } from "@opentui/solid";
import { effect as _$effect } from "@opentui/solid";
import { insert as _$insert } from "@opentui/solid";
import { createTextNode as _$createTextNode } from "@opentui/solid";
import { insertNode as _$insertNode } from "@opentui/solid";
import { setProp as _$setProp } from "@opentui/solid";
import { createElement as _$createElement } from "@opentui/solid";
import { execFile } from "child_process";
import fs from "fs";
import path from "path";
import { For, Show, createEffect, createMemo, createRoot, createSignal } from "solid-js";
var SNAPSHOT_POLL_INTERVAL_MS = 2500;
var SNAPSHOT_STALE_MS = 1e4;
var MAX_VISIBLE_ROWS = 4;
var SPINNER_FRAMES = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"];
var NEURAL_PULSE_FRAMES = ["\u25C8", "\u25C7", "\u25C6", "\u25C7"];
var TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
var DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
var ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
var CORTEX_THEME = {
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
  pureWhite: "#ffffff"
};
function sidebarLayout(width) {
  const measured = Number.isFinite(width) && width > 0 ? Math.floor(width) : 0;
  return {
    compact: measured === 0 || measured < 40,
    textLimit: Math.max(8, measured ? measured - 6 : 26),
    gaugeWidth: Math.max(3, Math.min(14, measured ? measured - 14 : 8))
  };
}
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
  if (process.env.CORTEX_IA_BIN) return process.env.CORTEX_IA_BIN;
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [path.join(home, "go", "bin", process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia"), process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia", path.join(local, "Programs", "cortex-ia", "bin", process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia"), path.join(home, ".local", "bin", "cortex-ia"), "/usr/local/bin/cortex-ia", "/usr/bin/cortex-ia"];
  for (const candidate of candidates) {
    if (candidate !== "cortex-ia" && candidate !== "cortex-ia.exe") {
      try {
        if (fs.existsSync(candidate)) return candidate;
      } catch {
      }
    }
  }
  return process.platform === "win32" ? "cortex-ia.exe" : "cortex-ia";
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
function roleChip(role) {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return {
    icon: "\u{1F9E0}",
    color: CORTEX_THEME.brandIndigo,
    tag: "ORCH"
  };
  if (r.includes("impl")) return {
    icon: "\u26A1",
    color: CORTEX_THEME.amberGold,
    tag: "IMPL"
  };
  if (r.includes("rev")) return {
    icon: "\u2696\uFE0F",
    color: CORTEX_THEME.brandPurple,
    tag: "REVW"
  };
  if (r.includes("inv")) return {
    icon: "\u{1F50D}",
    color: CORTEX_THEME.skyBlue,
    tag: "INVS"
  };
  if (r.includes("plan")) return {
    icon: "\u{1F4CB}",
    color: CORTEX_THEME.neonCyan,
    tag: "PLAN"
  };
  if (r.includes("disc")) return {
    icon: "\u{1F9ED}",
    color: CORTEX_THEME.emeraldGreen,
    tag: "DISC"
  };
  return {
    icon: "\u{1F916}",
    color: CORTEX_THEME.slateMuted,
    tag: r.slice(0, 4).toUpperCase() || "WORK"
  };
}
function taskStatusChip(status) {
  switch (status) {
    case "done":
      return {
        icon: "\u2713",
        color: CORTEX_THEME.emeraldGreen,
        tag: "DONE"
      };
    case "in_progress":
      return {
        icon: "\u26A1",
        color: CORTEX_THEME.amberGold,
        tag: "PROG"
      };
    case "in_review":
      return {
        icon: "\u25C6",
        color: CORTEX_THEME.brandPurple,
        tag: "REVW"
      };
    case "ready":
      return {
        icon: "\u25B6",
        color: CORTEX_THEME.skyBlue,
        tag: "RDY "
      };
    case "blocked":
      return {
        icon: "\u2715",
        color: CORTEX_THEME.roseRed,
        tag: "BLCK"
      };
    case "superseded":
      return {
        icon: "\u21B7",
        color: CORTEX_THEME.slateMuted,
        tag: "SPRS"
      };
    case "backlog":
    default:
      return {
        icon: "\u25CB",
        color: CORTEX_THEME.slateMuted,
        tag: "WAIT"
      };
  }
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
function CortexCockpitHeader(props) {
  const projectName = createMemo(() => {
    if (!props.projectRoot) return "";
    return path.basename(props.projectRoot);
  });
  return (() => {
    var _el$ = _$createElement("box"), _el$2 = _$createElement("box"), _el$3 = _$createElement("text"), _el$5 = _$createElement("text"), _el$7 = _$createElement("text");
    _$insertNode(_el$, _el$2);
    _$setProp(_el$, "flexDirection", "column");
    _$setProp(_el$, "borderStyle", "rounded");
    _$setProp(_el$, "paddingLeft", 1);
    _$setProp(_el$, "paddingRight", 1);
    _$insertNode(_el$2, _el$3);
    _$insertNode(_el$2, _el$5);
    _$insertNode(_el$2, _el$7);
    _$setProp(_el$2, "flexDirection", "row");
    _$insertNode(_el$3, _$createTextNode(`\u{1F9E0} `));
    _$insertNode(_el$5, _$createTextNode(`CORTEX\xB7IA`));
    _$insertNode(_el$7, _$createTextNode(` v2.0`));
    _$insert(_el$2, _$createComponent(Show, {
      get when() {
        return props.isExecuting();
      },
      get fallback() {
        return (() => {
          var _el$12 = _$createElement("text");
          _$insertNode(_el$12, _$createTextNode(` [\u25CF STANDBY]`));
          _$effect((_$p) => _$setProp(_el$12, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$12;
        })();
      },
      get children() {
        var _el$9 = _$createElement("text");
        _$insert(_el$9, () => ` [${props.spinner()} ACTIVO]`);
        _$effect((_$p) => _$setProp(_el$9, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$9;
      }
    }), null);
    _$insert(_el$, _$createComponent(Show, {
      get when() {
        return projectName();
      },
      get children() {
        var _el$0 = _$createElement("box"), _el$1 = _$createElement("text"), _el$11 = _$createElement("text");
        _$insertNode(_el$0, _el$1);
        _$insertNode(_el$0, _el$11);
        _$setProp(_el$0, "flexDirection", "row");
        _$setProp(_el$0, "marginTop", 0);
        _$insertNode(_el$1, _$createTextNode(`\u{1F4C1} `));
        _$insert(_el$11, () => clipped(projectName(), Math.max(6, props.textLimit - 4)));
        _$effect((_p$) => {
          var _v$ = CORTEX_THEME.skyBlue, _v$2 = CORTEX_THEME.skyBlue;
          _v$ !== _p$.e && (_p$.e = _$setProp(_el$1, "fg", _v$, _p$.e));
          _v$2 !== _p$.t && (_p$.t = _$setProp(_el$11, "fg", _v$2, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$0;
      }
    }), null);
    _$effect((_p$) => {
      var _v$3 = props.isExecuting() ? CORTEX_THEME.amberGold : CORTEX_THEME.brandIndigo, _v$4 = CORTEX_THEME.brandViolet, _v$5 = CORTEX_THEME.pureWhite, _v$6 = CORTEX_THEME.slateMuted;
      _v$3 !== _p$.e && (_p$.e = _$setProp(_el$, "borderColor", _v$3, _p$.e));
      _v$4 !== _p$.t && (_p$.t = _$setProp(_el$3, "fg", _v$4, _p$.t));
      _v$5 !== _p$.a && (_p$.a = _$setProp(_el$5, "fg", _v$5, _p$.a));
      _v$6 !== _p$.o && (_p$.o = _$setProp(_el$7, "fg", _v$6, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$;
  })();
}
function OperationalKPIHud(props) {
  return (() => {
    var _el$14 = _$createElement("box"), _el$15 = _$createElement("box"), _el$16 = _$createElement("text"), _el$17 = _$createElement("text"), _el$18 = _$createElement("box"), _el$19 = _$createElement("text"), _el$20 = _$createElement("text");
    _$insertNode(_el$14, _el$15);
    _$insertNode(_el$14, _el$18);
    _$setProp(_el$14, "flexDirection", "column");
    _$setProp(_el$14, "marginTop", 1);
    _$insertNode(_el$15, _el$16);
    _$insertNode(_el$15, _el$17);
    _$setProp(_el$15, "flexDirection", "row");
    _$insert(_el$16, () => `[${props.activeExecutions > 0 ? props.spinner() : "\u25CF"} ${props.activeExecutions} CURSO] `);
    _$insert(_el$17, () => `[\u25C6 ${props.inReview} REVW]`);
    _$insertNode(_el$18, _el$19);
    _$insertNode(_el$18, _el$20);
    _$setProp(_el$18, "flexDirection", "row");
    _$setProp(_el$18, "marginTop", 0);
    _$insert(_el$19, () => `[\u2713 ${props.doneTasks} DONE] `);
    _$insert(_el$20, () => `[${props.attentionCount > 0 ? "\u2715" : "\u25CB"} ${props.attentionCount} ALRT]`);
    _$effect((_p$) => {
      var _v$7 = props.activeExecutions > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted, _v$8 = props.inReview > 0 ? CORTEX_THEME.brandPurple : CORTEX_THEME.slateMuted, _v$9 = props.doneTasks > 0 ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.slateMuted, _v$0 = props.attentionCount > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted;
      _v$7 !== _p$.e && (_p$.e = _$setProp(_el$16, "fg", _v$7, _p$.e));
      _v$8 !== _p$.t && (_p$.t = _$setProp(_el$17, "fg", _v$8, _p$.t));
      _v$9 !== _p$.a && (_p$.a = _$setProp(_el$19, "fg", _v$9, _p$.a));
      _v$0 !== _p$.o && (_p$.o = _$setProp(_el$20, "fg", _v$0, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$14;
  })();
}
function Section(props) {
  const displayTitle = createMemo(() => props.compact && props.shortTitle ? props.shortTitle : props.title);
  const displayBadge = createMemo(() => props.compact && props.shortBadge ? props.shortBadge : props.badge);
  return (() => {
    var _el$21 = _$createElement("box"), _el$22 = _$createElement("box"), _el$23 = _$createElement("text"), _el$25 = _$createElement("text");
    _$insertNode(_el$21, _el$22);
    _$setProp(_el$21, "flexDirection", "column");
    _$setProp(_el$21, "marginTop", 1);
    _$insertNode(_el$22, _el$23);
    _$insertNode(_el$22, _el$25);
    _$setProp(_el$22, "flexDirection", "row");
    _$setProp(_el$23, "selectable", false);
    _$insert(_el$23, () => props.expanded() ? "\u25BC " : "\u25B6 ");
    _$insert(_el$22, _$createComponent(Show, {
      get when() {
        return props.icon;
      },
      get children() {
        var _el$24 = _$createElement("text");
        _$insert(_el$24, () => `${props.icon} `);
        _$effect((_$p) => _$setProp(_el$24, "fg", CORTEX_THEME.brandViolet, _$p));
        return _el$24;
      }
    }), _el$25);
    _$setProp(_el$25, "selectable", false);
    _$insert(_el$25, displayTitle);
    _$insert(_el$22, _$createComponent(Show, {
      get when() {
        return displayBadge();
      },
      get children() {
        var _el$26 = _$createElement("text");
        _$insert(_el$26, () => ` [${displayBadge()}]`);
        _$effect((_$p) => _$setProp(_el$26, "fg", CORTEX_THEME.skyBlue, _$p));
        return _el$26;
      }
    }), null);
    _$insert(_el$21, _$createComponent(Show, {
      get when() {
        return props.expanded();
      },
      get children() {
        return props.children;
      }
    }), null);
    _$effect((_p$) => {
      var _v$1 = props.onToggle, _v$10 = props.expanded() ? CORTEX_THEME.neonCyan : CORTEX_THEME.slateMuted, _v$11 = CORTEX_THEME.pureWhite;
      _v$1 !== _p$.e && (_p$.e = _$setProp(_el$22, "onMouseDown", _v$1, _p$.e));
      _v$10 !== _p$.t && (_p$.t = _$setProp(_el$23, "fg", _v$10, _p$.t));
      _v$11 !== _p$.a && (_p$.a = _$setProp(_el$25, "fg", _v$11, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$21;
  })();
}
function MultiColorProgressBar(props) {
  const w = Math.max(3, Math.min(props.width || 14, props.textLimit - (props.compact ? 8 : 14)));
  const total = Math.max(props.total, 1);
  const doneW = Math.round(props.done / total * w);
  const revW = Math.round(props.inReview / total * w);
  const progW = Math.round(props.inProgress / total * w);
  const blckW = Math.round((props.blocked || 0) / total * w);
  const emptyW = Math.max(0, w - doneW - revW - progW - blckW);
  const pct = Math.round(props.done / total * 100);
  const waiting = Math.max(0, props.total - props.done - props.inReview - props.inProgress - (props.blocked || 0));
  return (() => {
    var _el$27 = _$createElement("box"), _el$28 = _$createElement("box"), _el$29 = _$createElement("text"), _el$30 = _$createElement("text"), _el$31 = _$createElement("text"), _el$32 = _$createElement("text"), _el$34 = _$createElement("text"), _el$35 = _$createElement("text"), _el$36 = _$createElement("text"), _el$37 = _$createElement("box"), _el$38 = _$createElement("text"), _el$39 = _$createElement("text"), _el$40 = _$createElement("text"), _el$42 = _$createElement("text");
    _$insertNode(_el$27, _el$28);
    _$insertNode(_el$27, _el$37);
    _$setProp(_el$27, "flexDirection", "column");
    _$setProp(_el$27, "marginTop", 1);
    _$insertNode(_el$28, _el$29);
    _$insertNode(_el$28, _el$30);
    _$insertNode(_el$28, _el$31);
    _$insertNode(_el$28, _el$32);
    _$insertNode(_el$28, _el$34);
    _$insertNode(_el$28, _el$35);
    _$insertNode(_el$28, _el$36);
    _$setProp(_el$28, "flexDirection", "row");
    _$insert(_el$29, () => props.compact ? "D:" : "DAG: ");
    _$insert(_el$30, () => "\u2588".repeat(doneW));
    _$insert(_el$31, () => "\u2593".repeat(revW));
    _$insert(_el$32, () => "\u2592".repeat(progW));
    _$insert(_el$28, _$createComponent(Show, {
      when: blckW > 0,
      get children() {
        var _el$33 = _$createElement("text");
        _$insert(_el$33, () => "\u2593".repeat(blckW));
        _$effect((_$p) => _$setProp(_el$33, "fg", CORTEX_THEME.roseRed, _$p));
        return _el$33;
      }
    }), _el$34);
    _$insert(_el$34, () => "\u2591".repeat(emptyW));
    _$insert(_el$35, ` ${pct}%`);
    _$insert(_el$36, (() => {
      var _c$ = _$memo(() => !!props.compact);
      return () => _c$() ? "" : ` (${props.done}/${props.total})`;
    })());
    _$insertNode(_el$37, _el$38);
    _$insertNode(_el$37, _el$39);
    _$insertNode(_el$37, _el$40);
    _$insertNode(_el$37, _el$42);
    _$setProp(_el$37, "flexDirection", "row");
    _$insert(_el$38, () => `\u2713${props.done} `);
    _$insert(_el$39, () => `\u25C6${props.inReview} `);
    _$insert(_el$40, () => `\u25CF${props.inProgress} `);
    _$insert(_el$37, _$createComponent(Show, {
      get when() {
        return (props.blocked || 0) > 0;
      },
      get children() {
        var _el$41 = _$createElement("text");
        _$insert(_el$41, () => `\u2715${props.blocked} `);
        _$effect((_$p) => _$setProp(_el$41, "fg", CORTEX_THEME.roseRed, _$p));
        return _el$41;
      }
    }), _el$42);
    _$insert(_el$42, `\u25CB${waiting}`);
    _$effect((_p$) => {
      var _v$12 = CORTEX_THEME.neonCyan, _v$13 = CORTEX_THEME.emeraldGreen, _v$14 = CORTEX_THEME.brandPurple, _v$15 = CORTEX_THEME.amberGold, _v$16 = CORTEX_THEME.slateBorder, _v$17 = CORTEX_THEME.pureWhite, _v$18 = CORTEX_THEME.slateMuted, _v$19 = CORTEX_THEME.emeraldGreen, _v$20 = CORTEX_THEME.brandPurple, _v$21 = CORTEX_THEME.amberGold, _v$22 = CORTEX_THEME.slateMuted;
      _v$12 !== _p$.e && (_p$.e = _$setProp(_el$29, "fg", _v$12, _p$.e));
      _v$13 !== _p$.t && (_p$.t = _$setProp(_el$30, "fg", _v$13, _p$.t));
      _v$14 !== _p$.a && (_p$.a = _$setProp(_el$31, "fg", _v$14, _p$.a));
      _v$15 !== _p$.o && (_p$.o = _$setProp(_el$32, "fg", _v$15, _p$.o));
      _v$16 !== _p$.i && (_p$.i = _$setProp(_el$34, "fg", _v$16, _p$.i));
      _v$17 !== _p$.n && (_p$.n = _$setProp(_el$35, "fg", _v$17, _p$.n));
      _v$18 !== _p$.s && (_p$.s = _$setProp(_el$36, "fg", _v$18, _p$.s));
      _v$19 !== _p$.h && (_p$.h = _$setProp(_el$38, "fg", _v$19, _p$.h));
      _v$20 !== _p$.r && (_p$.r = _$setProp(_el$39, "fg", _v$20, _p$.r));
      _v$21 !== _p$.d && (_p$.d = _$setProp(_el$40, "fg", _v$21, _p$.d));
      _v$22 !== _p$.l && (_p$.l = _$setProp(_el$42, "fg", _v$22, _p$.l));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0,
      s: void 0,
      h: void 0,
      r: void 0,
      d: void 0,
      l: void 0
    });
    return _el$27;
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
    var _el$43 = _$createElement("box"), _el$44 = _$createElement("box"), _el$45 = _$createElement("text"), _el$46 = _$createElement("box"), _el$47 = _$createElement("text"), _el$49 = _$createElement("text"), _el$50 = _$createElement("box"), _el$51 = _$createElement("text"), _el$53 = _$createElement("text"), _el$54 = _$createElement("box");
    _$insertNode(_el$43, _el$44);
    _$insertNode(_el$43, _el$46);
    _$insertNode(_el$43, _el$50);
    _$insertNode(_el$43, _el$54);
    _$setProp(_el$43, "flexDirection", "column");
    _$setProp(_el$43, "marginTop", 1);
    _$setProp(_el$43, "paddingLeft", 1);
    _$setProp(_el$43, "paddingRight", 1);
    _$setProp(_el$43, "borderStyle", "rounded");
    _$insertNode(_el$44, _el$45);
    _$setProp(_el$44, "flexDirection", "row");
    _$insert(_el$45, () => clipped(`${props.spinner()} \u26A1 TAREA EN EJECUCI\xD3N`, props.textLimit));
    _$insertNode(_el$46, _el$47);
    _$insertNode(_el$46, _el$49);
    _$setProp(_el$46, "flexDirection", "row");
    _$insertNode(_el$47, _$createTextNode(`\u{1F3AF} `));
    _$insert(_el$49, () => clipped(`${props.task.task_id} \xB7 ${props.task.title}`, props.textLimit - 3));
    _$insertNode(_el$50, _el$51);
    _$insertNode(_el$50, _el$53);
    _$setProp(_el$50, "flexDirection", "row");
    _$insertNode(_el$51, _$createTextNode(` \u23F1 `));
    _$insert(_el$53, () => `+${elapsed()} `);
    _$insert(_el$50, _$createComponent(Show, {
      get when() {
        return ttlRemaining();
      },
      children: (ttl) => (() => {
        var _el$55 = _$createElement("text");
        _$insert(_el$55, () => `\u2502 \u{1F6E1} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`);
        _$effect((_$p) => _$setProp(_el$55, "fg", ttl() === "expirado" ? CORTEX_THEME.roseRed : CORTEX_THEME.skyBlue, _$p));
        return _el$55;
      })()
    }), null);
    _$setProp(_el$54, "flexDirection", "row");
    _$insert(_el$54, _$createComponent(Show, {
      get when() {
        return props.activeDelegation;
      },
      get fallback() {
        return (() => {
          var _el$56 = _$createElement("text");
          _$insert(_el$56, () => clipped(`  \u{1F4CB} Tarea durable${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit));
          _$effect((_$p) => _$setProp(_el$56, "fg", CORTEX_THEME.brandPurple, _$p));
          return _el$56;
        })();
      },
      children: (del) => (() => {
        var _el$57 = _$createElement("text");
        _$insert(_el$57, () => `  \u{1F916} AGY ${del().transport || "direct"}${del().pane_id ? ` \xB7 ${del().pane_id}` : ""}${del().attempt ? ` \xB7 int #${del().attempt}` : ""}`);
        _$effect((_$p) => _$setProp(_el$57, "fg", CORTEX_THEME.neonCyan, _$p));
        return _el$57;
      })()
    }));
    _$effect((_p$) => {
      var _v$23 = CORTEX_THEME.amberGold, _v$24 = CORTEX_THEME.amberGold, _v$25 = CORTEX_THEME.skyBlue, _v$26 = CORTEX_THEME.pureWhite, _v$27 = CORTEX_THEME.slateMuted, _v$28 = CORTEX_THEME.amberGold;
      _v$23 !== _p$.e && (_p$.e = _$setProp(_el$43, "borderColor", _v$23, _p$.e));
      _v$24 !== _p$.t && (_p$.t = _$setProp(_el$45, "fg", _v$24, _p$.t));
      _v$25 !== _p$.a && (_p$.a = _$setProp(_el$47, "fg", _v$25, _p$.a));
      _v$26 !== _p$.o && (_p$.o = _$setProp(_el$49, "fg", _v$26, _p$.o));
      _v$27 !== _p$.i && (_p$.i = _$setProp(_el$51, "fg", _v$27, _p$.i));
      _v$28 !== _p$.n && (_p$.n = _$setProp(_el$53, "fg", _v$28, _p$.n));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0
    });
    return _el$43;
  })();
}
function TaskRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$58 = _$createElement("text");
        _$insertNode(_el$58, _$createTextNode(` \u25CB Sin tareas en cola`));
        _$effect((_$p) => _$setProp(_el$58, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$58;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.tasks.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (task) => {
          const chip = taskStatusChip(task.status);
          const isProg = task.status === "in_progress";
          return (() => {
            var _el$60 = _$createElement("box"), _el$61 = _$createElement("box"), _el$62 = _$createElement("text"), _el$63 = _$createElement("text"), _el$64 = _$createElement("text"), _el$65 = _$createElement("box"), _el$66 = _$createElement("text");
            _$insertNode(_el$60, _el$61);
            _$insertNode(_el$60, _el$65);
            _$setProp(_el$60, "flexDirection", "column");
            _$setProp(_el$60, "marginTop", 0);
            _$insertNode(_el$61, _el$62);
            _$insertNode(_el$61, _el$63);
            _$insertNode(_el$61, _el$64);
            _$setProp(_el$61, "flexDirection", "row");
            _$insert(_el$62, () => `  ${isProg ? props.spinner() : chip.icon} `);
            _$insert(_el$63, () => `[${chip.tag}] `);
            _$insert(_el$64, () => clipped(task.task_id, Math.max(8, props.textLimit - 9)));
            _$insertNode(_el$65, _el$66);
            _$setProp(_el$65, "flexDirection", "row");
            _$insert(_el$66, () => `     ${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` \xB7 ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` \xB7 \u{1F6E1} ${task.lease_count}lk` : ""}`);
            _$effect((_p$) => {
              var _v$29 = chip.color, _v$30 = chip.color, _v$31 = isProg ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight, _v$32 = CORTEX_THEME.slateMuted;
              _v$29 !== _p$.e && (_p$.e = _$setProp(_el$62, "fg", _v$29, _p$.e));
              _v$30 !== _p$.t && (_p$.t = _$setProp(_el$63, "fg", _v$30, _p$.t));
              _v$31 !== _p$.a && (_p$.a = _$setProp(_el$64, "fg", _v$31, _p$.a));
              _v$32 !== _p$.o && (_p$.o = _$setProp(_el$66, "fg", _v$32, _p$.o));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0
            });
            return _el$60;
          })();
        }
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
        var _el$67 = _$createElement("text");
        _$insertNode(_el$67, _$createTextNode(` \u25CB Sin workers activos`));
        _$effect((_$p) => _$setProp(_el$67, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$67;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.jobs.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const chip = roleChip(job.role || "");
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` +${formatDuration(props.now() - t)}` : "";
          });
          const statusCol = isRunning ? CORTEX_THEME.amberGold : job.status === "succeeded" ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.roseRed;
          return (() => {
            var _el$69 = _$createElement("box"), _el$70 = _$createElement("box"), _el$71 = _$createElement("text"), _el$72 = _$createElement("text"), _el$73 = _$createElement("text"), _el$74 = _$createElement("text"), _el$75 = _$createElement("box"), _el$76 = _$createElement("text");
            _$insertNode(_el$69, _el$70);
            _$insertNode(_el$69, _el$75);
            _$setProp(_el$69, "flexDirection", "column");
            _$setProp(_el$69, "marginTop", 0);
            _$insertNode(_el$70, _el$71);
            _$insertNode(_el$70, _el$72);
            _$insertNode(_el$70, _el$73);
            _$insertNode(_el$70, _el$74);
            _$setProp(_el$70, "flexDirection", "row");
            _$setProp(_el$71, "fg", statusCol);
            _$insert(_el$71, () => `  ${isRunning ? props.spinner() : job.status === "succeeded" ? "\u2713" : "\u2715"} `);
            _$insert(_el$72, () => `${chip.icon} [${chip.tag}] `);
            _$insert(_el$73, () => clipped(job.role || "worker", Math.max(6, props.textLimit - 12)));
            _$setProp(_el$74, "fg", statusCol);
            _$insert(_el$74, elapsed);
            _$insertNode(_el$75, _el$76);
            _$setProp(_el$75, "flexDirection", "row");
            _$insert(_el$76, () => clipped(`     ${shortID(job.job_id)} \xB7 ${job.transport || "direct"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}${job.attempt ? ` \xB7 int #${job.attempt}` : ""}`, props.textLimit));
            _$effect((_p$) => {
              var _v$33 = chip.color, _v$34 = isRunning ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight, _v$35 = CORTEX_THEME.slateMuted;
              _v$33 !== _p$.e && (_p$.e = _$setProp(_el$72, "fg", _v$33, _p$.e));
              _v$34 !== _p$.t && (_p$.t = _$setProp(_el$73, "fg", _v$34, _p$.t));
              _v$35 !== _p$.a && (_p$.a = _$setProp(_el$76, "fg", _v$35, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$69;
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
        var _el$77 = _$createElement("text");
        _$insertNode(_el$77, _$createTextNode(` \u2713 Sin alertas pendientes`));
        _$effect((_$p) => _$setProp(_el$77, "fg", CORTEX_THEME.emeraldGreen, _$p));
        return _el$77;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$79 = _$createElement("box"), _el$80 = _$createElement("box"), _el$81 = _$createElement("text"), _el$83 = _$createElement("text"), _el$84 = _$createElement("text");
          _$insertNode(_el$79, _el$80);
          _$insertNode(_el$79, _el$84);
          _$setProp(_el$79, "flexDirection", "column");
          _$insertNode(_el$80, _el$81);
          _$insertNode(_el$80, _el$83);
          _$setProp(_el$80, "flexDirection", "row");
          _$insertNode(_el$81, _$createTextNode(`  \u2715 `));
          _$insert(_el$83, () => clipped(item.title, Math.max(8, props.textLimit - 4)));
          _$insert(_el$84, () => `     ${clipped(item.detail, Math.max(8, props.textLimit - 5))}`);
          _$effect((_p$) => {
            var _v$36 = CORTEX_THEME.roseRed, _v$37 = CORTEX_THEME.pureWhite, _v$38 = CORTEX_THEME.slateMuted;
            _v$36 !== _p$.e && (_p$.e = _$setProp(_el$81, "fg", _v$36, _p$.e));
            _v$37 !== _p$.t && (_p$.t = _$setProp(_el$83, "fg", _v$37, _p$.t));
            _v$38 !== _p$.a && (_p$.a = _$setProp(_el$84, "fg", _v$38, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$79;
        })()
      });
    }
  });
}
function OperationalBottomDashboard(props) {
  const succeededJobs = createMemo(() => props.jobs.filter((j) => j.status === "succeeded").length);
  const failedJobs = createMemo(() => props.jobs.filter((j) => ["failed", "timed_out", "lost", "cancelled"].includes(j.status)).length);
  const activeJobs = createMemo(() => props.jobs.filter((j) => ["running", "starting", "accepted"].includes(j.status)).length);
  const doneTasks = createMemo(() => props.snapshot.summary.done || 0);
  const totalTasks = createMemo(() => props.snapshot.summary.total_tasks || props.snapshot.tasks.length);
  const totalLeases = createMemo(() => props.snapshot.tasks.reduce((sum, t) => sum + (t.lease_count || 0), 0));
  const blockedTasks = createMemo(() => props.snapshot.summary.blocked || 0);
  const successRate = createMemo(() => {
    const closed = succeededJobs() + failedJobs();
    if (closed === 0) return void 0;
    return Math.round(succeededJobs() / closed * 100);
  });
  const syncAgeSec = createMemo(() => {
    const t = Date.parse(props.snapshot.generated_at);
    if (!Number.isFinite(t)) return 0;
    return Math.max(0, Math.floor((props.now() - t) / 1e3));
  });
  const healthBars = createMemo(() => {
    const rate = successRate();
    const filled = rate === void 0 ? 0 : Math.round(rate / 100 * props.layout.gaugeWidth);
    const empty = rate === void 0 ? 0 : Math.max(0, props.layout.gaugeWidth - filled);
    return {
      filled: "\u25A0".repeat(filled),
      empty: "\u25A1".repeat(empty)
    };
  });
  const healthColor = createMemo(() => {
    const rate = successRate();
    if (rate === void 0) return CORTEX_THEME.slateMuted;
    if (rate >= 90) return CORTEX_THEME.emeraldGreen;
    if (rate >= 70) return CORTEX_THEME.neonCyan;
    if (rate >= 50) return CORTEX_THEME.amberGold;
    return CORTEX_THEME.roseRed;
  });
  const hasMetrics = createMemo(() => succeededJobs() > 0 || failedJobs() > 0 || activeJobs() > 0 || totalLeases() > 0 || blockedTasks() > 0);
  return (() => {
    var _el$85 = _$createElement("box"), _el$86 = _$createElement("box"), _el$87 = _$createElement("text"), _el$89 = _$createElement("text"), _el$90 = _$createElement("text"), _el$92 = _$createElement("box"), _el$101 = _$createElement("box"), _el$102 = _$createElement("text"), _el$107 = _$createElement("box"), _el$108 = _$createElement("text"), _el$109 = _$createElement("box");
    _$insertNode(_el$85, _el$86);
    _$insertNode(_el$85, _el$92);
    _$insertNode(_el$85, _el$101);
    _$insertNode(_el$85, _el$107);
    _$insertNode(_el$85, _el$109);
    _$setProp(_el$85, "flexDirection", "column");
    _$setProp(_el$85, "marginTop", 1);
    _$setProp(_el$85, "paddingLeft", 1);
    _$setProp(_el$85, "paddingRight", 1);
    _$setProp(_el$85, "borderStyle", "rounded");
    _$insertNode(_el$86, _el$87);
    _$insertNode(_el$86, _el$89);
    _$insertNode(_el$86, _el$90);
    _$setProp(_el$86, "flexDirection", "row");
    _$insertNode(_el$87, _$createTextNode(`\u{1F9E0} `));
    _$insert(_el$89, () => props.layout.compact ? "CONTROL" : "CONTROL MATRIX ");
    _$insertNode(_el$90, _$createTextNode(`\u25C8`));
    _$setProp(_el$92, "flexDirection", "row");
    _$insert(_el$92, _$createComponent(Show, {
      get when() {
        return activeJobs() > 0;
      },
      get fallback() {
        return (() => {
          var _el$111 = _$createElement("text");
          _$insert(_el$111, () => `  ${props.pulse()} SYNAPSE: SINCRONIZADO`);
          _$effect((_$p) => _$setProp(_el$111, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$111;
        })();
      },
      get children() {
        var _el$93 = _$createElement("text");
        _$insert(_el$93, () => `  ${props.spinner()} SYNAPSE: MOTOR ACTIVO`);
        _$effect((_$p) => _$setProp(_el$93, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$93;
      }
    }));
    _$insert(_el$85, _$createComponent(Show, {
      get when() {
        return hasMetrics();
      },
      get fallback() {
        return (() => {
          var _el$112 = _$createElement("box"), _el$113 = _$createElement("text");
          _$insertNode(_el$112, _el$113);
          _$setProp(_el$112, "flexDirection", "row");
          _$insertNode(_el$113, _$createTextNode(` Standby \xB7 0 ejecuciones`));
          _$effect((_$p) => _$setProp(_el$113, "fg", CORTEX_THEME.slateMuted, _$p));
          return _el$112;
        })();
      },
      get children() {
        return _$createComponent(Show, {
          get when() {
            return !props.layout.compact;
          },
          get fallback() {
            return (() => {
              var _el$115 = _$createElement("box"), _el$116 = _$createElement("text"), _el$117 = _$createElement("text"), _el$118 = _$createElement("text"), _el$119 = _$createElement("text");
              _$insertNode(_el$115, _el$116);
              _$insertNode(_el$115, _el$117);
              _$insertNode(_el$115, _el$118);
              _$insertNode(_el$115, _el$119);
              _$setProp(_el$115, "flexDirection", "row");
              _$insert(_el$116, () => `\u2713${succeededJobs()} `);
              _$insert(_el$117, () => `\u2715${failedJobs()} `);
              _$insert(_el$118, () => `\u25CF${activeJobs()} `);
              _$insert(_el$119, () => `\u{1F6E1}${totalLeases()}`);
              _$insert(_el$115, _$createComponent(Show, {
                get when() {
                  return blockedTasks() > 0;
                },
                get children() {
                  var _el$120 = _$createElement("text");
                  _$insert(_el$120, () => ` \u2715${blockedTasks()}b`);
                  _$effect((_$p) => _$setProp(_el$120, "fg", CORTEX_THEME.roseRed, _$p));
                  return _el$120;
                }
              }), null);
              _$effect((_p$) => {
                var _v$49 = succeededJobs() > 0 ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.slateMuted, _v$50 = failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted, _v$51 = activeJobs() > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted, _v$52 = totalLeases() > 0 ? CORTEX_THEME.skyBlue : CORTEX_THEME.slateMuted;
                _v$49 !== _p$.e && (_p$.e = _$setProp(_el$116, "fg", _v$49, _p$.e));
                _v$50 !== _p$.t && (_p$.t = _$setProp(_el$117, "fg", _v$50, _p$.t));
                _v$51 !== _p$.a && (_p$.a = _$setProp(_el$118, "fg", _v$51, _p$.a));
                _v$52 !== _p$.o && (_p$.o = _$setProp(_el$119, "fg", _v$52, _p$.o));
                return _p$;
              }, {
                e: void 0,
                t: void 0,
                a: void 0,
                o: void 0
              });
              return _el$115;
            })();
          },
          get children() {
            return [(() => {
              var _el$94 = _$createElement("box"), _el$95 = _$createElement("text"), _el$96 = _$createElement("text");
              _$insertNode(_el$94, _el$95);
              _$insertNode(_el$94, _el$96);
              _$setProp(_el$94, "flexDirection", "row");
              _$insert(_el$95, () => `[ \u2713 ${succeededJobs()} \xC9XITO ] `);
              _$insert(_el$96, () => `[ \u2715 ${failedJobs()} FALLO ]`);
              _$effect((_p$) => {
                var _v$39 = CORTEX_THEME.emeraldGreen, _v$40 = failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted;
                _v$39 !== _p$.e && (_p$.e = _$setProp(_el$95, "fg", _v$39, _p$.e));
                _v$40 !== _p$.t && (_p$.t = _$setProp(_el$96, "fg", _v$40, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$94;
            })(), (() => {
              var _el$97 = _$createElement("box"), _el$98 = _$createElement("text"), _el$99 = _$createElement("text");
              _$insertNode(_el$97, _el$98);
              _$insertNode(_el$97, _el$99);
              _$setProp(_el$97, "flexDirection", "row");
              _$insert(_el$98, () => `[ ${activeJobs() > 0 ? props.spinner() : "\u25CF"} ${activeJobs()} CURSO ] `);
              _$insert(_el$99, () => `[ \u{1F6E1} ${totalLeases()} LOCKS ] `);
              _$insert(_el$97, _$createComponent(Show, {
                get when() {
                  return blockedTasks() > 0;
                },
                get children() {
                  var _el$100 = _$createElement("text");
                  _$insert(_el$100, () => `[ \u2715 ${blockedTasks()} BLCK ]`);
                  _$effect((_$p) => _$setProp(_el$100, "fg", CORTEX_THEME.roseRed, _$p));
                  return _el$100;
                }
              }), null);
              _$effect((_p$) => {
                var _v$41 = activeJobs() > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted, _v$42 = totalLeases() > 0 ? CORTEX_THEME.skyBlue : CORTEX_THEME.slateMuted;
                _v$41 !== _p$.e && (_p$.e = _$setProp(_el$98, "fg", _v$41, _p$.e));
                _v$42 !== _p$.t && (_p$.t = _$setProp(_el$99, "fg", _v$42, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$97;
            })()];
          }
        });
      }
    }), _el$101);
    _$insertNode(_el$101, _el$102);
    _$setProp(_el$101, "flexDirection", "row");
    _$insertNode(_el$102, _$createTextNode(`Salud: `));
    _$insert(_el$101, _$createComponent(Show, {
      get when() {
        return successRate() !== void 0;
      },
      get fallback() {
        return (() => {
          var _el$121 = _$createElement("text");
          _$insertNode(_el$121, _$createTextNode(`\u25CB standby`));
          _$effect((_$p) => _$setProp(_el$121, "fg", CORTEX_THEME.slateMuted, _$p));
          return _el$121;
        })();
      },
      get children() {
        return [(() => {
          var _el$104 = _$createElement("text");
          _$insert(_el$104, () => healthBars().filled);
          _$effect((_$p) => _$setProp(_el$104, "fg", healthColor(), _$p));
          return _el$104;
        })(), (() => {
          var _el$105 = _$createElement("text");
          _$insert(_el$105, () => healthBars().empty);
          _$effect((_$p) => _$setProp(_el$105, "fg", CORTEX_THEME.slateBorder, _$p));
          return _el$105;
        })(), (() => {
          var _el$106 = _$createElement("text");
          _$insert(_el$106, () => ` ${successRate()}%`);
          _$effect((_$p) => _$setProp(_el$106, "fg", healthColor(), _$p));
          return _el$106;
        })()];
      }
    }), null);
    _$insertNode(_el$107, _el$108);
    _$setProp(_el$107, "flexDirection", "row");
    _$insert(_el$108, (() => {
      var _c$2 = _$memo(() => !!props.layout.compact);
      return () => _c$2() ? "Autoridad: SQLite" : `\u{1F4CB} DAG: ${doneTasks()}/${totalTasks()} \xB7 Autoridad: SQLite`;
    })());
    _$setProp(_el$109, "flexDirection", "row");
    _$insert(_el$109, _$createComponent(Show, {
      get when() {
        return props.stale;
      },
      get fallback() {
        return (() => {
          var _el$123 = _$createElement("text");
          _$insert(_el$123, (() => {
            var _c$4 = _$memo(() => !!props.layout.compact);
            return () => _c$4() ? "\u{1F7E2} En vivo" : `\u{1F7E2} En vivo \xB7 Sync hace ${syncAgeSec()}s`;
          })());
          _$effect((_$p) => _$setProp(_el$123, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$123;
        })();
      },
      get children() {
        var _el$110 = _$createElement("text");
        _$insert(_el$110, (() => {
          var _c$3 = _$memo(() => !!props.layout.compact);
          return () => _c$3() ? "\u{1F7E1} Desfasado" : `\u{1F7E1} Snapshot desfasado (+${syncAgeSec()}s)`;
        })());
        _$effect((_$p) => _$setProp(_el$110, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$110;
      }
    }));
    _$effect((_p$) => {
      var _v$43 = failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.brandIndigo, _v$44 = CORTEX_THEME.brandViolet, _v$45 = CORTEX_THEME.pureWhite, _v$46 = CORTEX_THEME.neonCyan, _v$47 = CORTEX_THEME.slateMuted, _v$48 = CORTEX_THEME.slateMuted;
      _v$43 !== _p$.e && (_p$.e = _$setProp(_el$85, "borderColor", _v$43, _p$.e));
      _v$44 !== _p$.t && (_p$.t = _$setProp(_el$87, "fg", _v$44, _p$.t));
      _v$45 !== _p$.a && (_p$.a = _$setProp(_el$89, "fg", _v$45, _p$.a));
      _v$46 !== _p$.o && (_p$.o = _$setProp(_el$90, "fg", _v$46, _p$.o));
      _v$47 !== _p$.i && (_p$.i = _$setProp(_el$102, "fg", _v$47, _p$.i));
      _v$48 !== _p$.n && (_p$.n = _$setProp(_el$108, "fg", _v$48, _p$.n));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0
    });
    return _el$85;
  })();
}
function SidebarStatus(props) {
  const [rootWidth, setRootWidth] = createSignal(0);
  const layout = createMemo(() => sidebarLayout(rootWidth()));
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
  return (() => {
    var _el$124 = _$createElement("box"), _el$131 = _$createElement("text");
    _$insertNode(_el$124, _el$131);
    _$use((node) => setRootWidth(Math.max(0, node.width || 0)), _el$124);
    _$setProp(_el$124, "flexDirection", "column");
    _$setProp(_el$124, "onSizeChange", function() {
      setRootWidth(Math.max(0, this.width || 0));
    });
    _$insert(_el$124, _$createComponent(CortexCockpitHeader, {
      isExecuting,
      get nativeActivity() {
        return props.nativeActivity;
      },
      get projectRoot() {
        return props.snapshot().project_root;
      },
      get textLimit() {
        return layout().textLimit;
      },
      get spinner() {
        return props.spinner;
      },
      get pulse() {
        return props.pulse;
      },
      get theme() {
        return props.theme;
      }
    }), _el$131);
    _$insert(_el$124, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$125 = _$createElement("text");
        _$insertNode(_el$125, _$createTextNode(`Conversaci\xF3n no disponible \xB7 esperando metadatos`));
        _$setProp(_el$125, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$125, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$125;
      }
    }), _el$131);
    _$insert(_el$124, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$127 = _$createElement("text");
        _$insertNode(_el$127, _$createTextNode(`Cargando estado de la conversaci\xF3n\u2026`));
        _$setProp(_el$127, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$127, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$127;
      }
    }), _el$131);
    _$insert(_el$124, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$129 = _$createElement("text");
        _$insertNode(_el$129, _$createTextNode(`No se pudo actualizar \xB7 datos no confirmados`));
        _$setProp(_el$129, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$129, "fg", CORTEX_THEME.roseRed, _$p));
        return _el$129;
      }
    }), _el$131);
    _$insert(_el$131, () => clipped(`OpenCode nativo: ${props.nativeActivity() || "sin estado"}`, layout().textLimit));
    _$insert(_el$124, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!props.scopeReady())() && Boolean(props.snapshot().generated_at);
      },
      get children() {
        return [_$createComponent(OperationalKPIHud, {
          get activeExecutions() {
            return activeExecutionsCount();
          },
          get inReview() {
            return counts().review;
          },
          get doneTasks() {
            return doneTasks();
          },
          get attentionCount() {
            return counts().attention;
          },
          get spinner() {
            return props.spinner;
          },
          get theme() {
            return props.theme;
          }
        }), _$createComponent(Show, {
          get when() {
            return totalTasks() > 0;
          },
          get children() {
            return _$createComponent(MultiColorProgressBar, {
              get done() {
                return doneTasks();
              },
              get inReview() {
                return inReviewTasks();
              },
              get inProgress() {
                return inProgressTasks();
              },
              get blocked() {
                return blockedTasks();
              },
              get total() {
                return totalTasks();
              },
              get width() {
                return layout().gaugeWidth;
              },
              get compact() {
                return layout().compact;
              },
              get textLimit() {
                return layout().textLimit;
              },
              get theme() {
                return props.theme;
              }
            });
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
            get textLimit() {
              return layout().textLimit;
            },
            get theme() {
              return props.theme;
            }
          })
        }), _$createComponent(Section, {
          title: "Tablero de Tareas",
          shortTitle: "Tareas",
          icon: "\u{1F4CB}",
          get badge() {
            return _$memo(() => totalTasks() > 0)() ? `${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot` : void 0;
          },
          get shortBadge() {
            return _$memo(() => totalTasks() > 0)() ? `${props.snapshot().summary.active_tasks || 0}/${totalTasks()}` : void 0;
          },
          get compact() {
            return layout().compact;
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
              },
              get compact() {
                return layout().compact;
              },
              get textLimit() {
                return layout().textLimit;
              }
            });
          }
        }), _$createComponent(Section, {
          title: "Workers & Delegaci\xF3n",
          shortTitle: "Workers",
          icon: "\u{1F916}",
          get badge() {
            return _$memo(() => totalDelegationsCount() > 0)() ? `${activeDelegationsCount()} act / ${totalDelegationsCount()} tot` : void 0;
          },
          get shortBadge() {
            return _$memo(() => totalDelegationsCount() > 0)() ? `${activeDelegationsCount()}/${totalDelegationsCount()}` : void 0;
          },
          get compact() {
            return layout().compact;
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
              },
              get compact() {
                return layout().compact;
              },
              get textLimit() {
                return layout().textLimit;
              }
            });
          }
        }), _$createComponent(Section, {
          title: "Centro de Atenci\xF3n",
          shortTitle: "Alertas",
          icon: "\u26A0\uFE0F",
          get badge() {
            return _$memo(() => counts().attention > 0)() ? `${counts().attention} alertas` : void 0;
          },
          get shortBadge() {
            return _$memo(() => counts().attention > 0)() ? `${counts().attention}` : void 0;
          },
          get compact() {
            return layout().compact;
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
              },
              get compact() {
                return layout().compact;
              },
              get textLimit() {
                return layout().textLimit;
              }
            });
          }
        }), _$createComponent(OperationalBottomDashboard, {
          get snapshot() {
            return props.snapshot();
          },
          get jobs() {
            return props.jobs();
          },
          get stale() {
            return stale();
          },
          get now() {
            return props.now;
          },
          get spinner() {
            return props.spinner;
          },
          get pulse() {
            return props.pulse;
          },
          get theme() {
            return props.theme;
          },
          get layout() {
            return layout();
          }
        })];
      }
    }), null);
    _$effect((_$p) => _$setProp(_el$131, "fg", CORTEX_THEME.slateMuted, _$p));
    return _el$124;
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
      var _el$132 = _$createElement("box"), _el$133 = _$createElement("text"), _el$135 = _$createElement("text"), _el$137 = _$createElement("text"), _el$139 = _$createElement("text"), _el$141 = _$createElement("text");
      _$insertNode(_el$132, _el$133);
      _$insertNode(_el$132, _el$135);
      _$insertNode(_el$132, _el$137);
      _$insertNode(_el$132, _el$139);
      _$insertNode(_el$132, _el$141);
      _$setProp(_el$132, "paddingLeft", 1);
      _$setProp(_el$132, "paddingRight", 1);
      _$setProp(_el$132, "flexDirection", "row");
      _$insertNode(_el$133, _$createTextNode(`\u{1F9E0} `));
      _$insertNode(_el$135, _$createTextNode(`CORTEX`));
      _$insertNode(_el$137, _$createTextNode(`\xB7`));
      _$insertNode(_el$139, _$createTextNode(`IA `));
      _$insertNode(_el$141, _$createTextNode(`\u2502 `));
      _$insert(_el$132, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$143 = _$createElement("box"), _el$144 = _$createElement("text"), _el$145 = _$createElement("text"), _el$147 = _$createElement("text");
            _$insertNode(_el$143, _el$144);
            _$insertNode(_el$143, _el$145);
            _$insertNode(_el$143, _el$147);
            _$setProp(_el$143, "flexDirection", "row");
            _$insert(_el$144, () => `\u25CF ${counts().active} en curso`);
            _$insertNode(_el$145, _$createTextNode(` \xB7 `));
            _$insert(_el$147, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$143, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$148 = _$createElement("text");
                  _$insertNode(_el$148, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$148, "fg", CORTEX_THEME.slateBorder, _$p));
                  return _el$148;
                })(), (() => {
                  var _el$150 = _$createElement("text");
                  _$insert(_el$150, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$150, "fg", CORTEX_THEME.roseRed, _$p));
                  return _el$150;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$58 = CORTEX_THEME.amberGold, _v$59 = CORTEX_THEME.slateBorder, _v$60 = CORTEX_THEME.brandPurple;
              _v$58 !== _p$.e && (_p$.e = _$setProp(_el$144, "fg", _v$58, _p$.e));
              _v$59 !== _p$.t && (_p$.t = _$setProp(_el$145, "fg", _v$59, _p$.t));
              _v$60 !== _p$.a && (_p$.a = _$setProp(_el$147, "fg", _v$60, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$143;
          })();
        },
        children: (task) => (() => {
          var _el$151 = _$createElement("box"), _el$152 = _$createElement("text"), _el$153 = _$createElement("text"), _el$154 = _$createElement("text");
          _$insertNode(_el$151, _el$152);
          _$insertNode(_el$151, _el$153);
          _$insertNode(_el$151, _el$154);
          _$setProp(_el$151, "flexDirection", "row");
          _$insert(_el$152, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$153, () => clipped(task().title, 20));
          _$insert(_el$154, () => ` \xB7 ${counts().active} activos`);
          _$effect((_p$) => {
            var _v$61 = CORTEX_THEME.amberGold, _v$62 = CORTEX_THEME.pureWhite, _v$63 = CORTEX_THEME.slateMuted;
            _v$61 !== _p$.e && (_p$.e = _$setProp(_el$152, "fg", _v$61, _p$.e));
            _v$62 !== _p$.t && (_p$.t = _$setProp(_el$153, "fg", _v$62, _p$.t));
            _v$63 !== _p$.a && (_p$.a = _$setProp(_el$154, "fg", _v$63, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$151;
        })()
      }), null);
      _$effect((_p$) => {
        var _v$53 = CORTEX_THEME.brandViolet, _v$54 = CORTEX_THEME.pureWhite, _v$55 = CORTEX_THEME.neonCyan, _v$56 = CORTEX_THEME.skyBlue, _v$57 = CORTEX_THEME.slateBorder;
        _v$53 !== _p$.e && (_p$.e = _$setProp(_el$133, "fg", _v$53, _p$.e));
        _v$54 !== _p$.t && (_p$.t = _$setProp(_el$135, "fg", _v$54, _p$.t));
        _v$55 !== _p$.a && (_p$.a = _$setProp(_el$137, "fg", _v$55, _p$.a));
        _v$56 !== _p$.o && (_p$.o = _$setProp(_el$139, "fg", _v$56, _p$.o));
        _v$57 !== _p$.i && (_p$.i = _$setProp(_el$141, "fg", _v$57, _p$.i));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0
      });
      return _el$132;
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
  const [pulseFrame, setPulseFrame] = createSignal(0);
  const [tasksExpanded, setTasksExpanded] = createSignal(api.kv.get(TASKS_EXPANDED_KEY, true) !== false);
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(api.kv.get(DELEGATIONS_EXPANDED_KEY, true) !== false);
  const [attentionExpanded, setAttentionExpanded] = createSignal(api.kv.get(ATTENTION_EXPANDED_KEY, true) !== false);
  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const pulse = createMemo(() => NEURAL_PULSE_FRAMES[pulseFrame() % NEURAL_PULSE_FRAMES.length]);
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
        setSnapshot(EMPTY_SNAPSHOT);
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
        setSnapshot(EMPTY_SNAPSHOT);
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
  const pulseTimer = setInterval(() => setPulseFrame((p) => (p + 1) % NEURAL_PULSE_FRAMES.length), 350);
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
          pulse,
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
    clearInterval(pulseTimer);
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
  SidebarStatus,
  cortex_ia_tui_default as default
};
