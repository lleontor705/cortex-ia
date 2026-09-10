// cortex-ia-tui.tsx
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
  return (() => {
    var _el$ = _$createElement("box"), _el$2 = _$createElement("box"), _el$3 = _$createElement("text"), _el$5 = _$createElement("text"), _el$7 = _$createElement("text"), _el$9 = _$createElement("text"), _el$1 = _$createElement("text"), _el$12 = _$createElement("box"), _el$13 = _$createElement("text");
    _$insertNode(_el$, _el$2);
    _$insertNode(_el$, _el$12);
    _$setProp(_el$, "flexDirection", "column");
    _$setProp(_el$, "borderStyle", "rounded");
    _$setProp(_el$, "paddingLeft", 1);
    _$setProp(_el$, "paddingRight", 1);
    _$insertNode(_el$2, _el$3);
    _$insertNode(_el$2, _el$5);
    _$insertNode(_el$2, _el$7);
    _$insertNode(_el$2, _el$9);
    _$insertNode(_el$2, _el$1);
    _$setProp(_el$2, "flexDirection", "row");
    _$insertNode(_el$3, _$createTextNode(`\u{1F9E0} `));
    _$insertNode(_el$5, _$createTextNode(`CORTEX`));
    _$insertNode(_el$7, _$createTextNode(`\xB7`));
    _$insertNode(_el$9, _$createTextNode(`IA `));
    _$insertNode(_el$1, _$createTextNode(`v2.0 `));
    _$insert(_el$2, _$createComponent(Show, {
      get when() {
        return props.isExecuting();
      },
      get fallback() {
        return (() => {
          var _el$18 = _$createElement("text");
          _$insertNode(_el$18, _$createTextNode(`[\u25CF STANDBY]`));
          _$effect((_$p) => _$setProp(_el$18, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$18;
        })();
      },
      get children() {
        var _el$11 = _$createElement("text");
        _$insert(_el$11, () => `[${props.spinner()} NEURAL ACT]`);
        _$effect((_$p) => _$setProp(_el$11, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$11;
      }
    }), null);
    _$insertNode(_el$12, _el$13);
    _$setProp(_el$12, "flexDirection", "row");
    _$insertNode(_el$13, _$createTextNode(`Neural Control Bridge `));
    _$insert(_el$12, _$createComponent(Show, {
      get when() {
        return props.projectRoot;
      },
      get children() {
        return [(() => {
          var _el$15 = _$createElement("text");
          _$insertNode(_el$15, _$createTextNode(`\u2502 `));
          _$effect((_$p) => _$setProp(_el$15, "fg", CORTEX_THEME.slateBorder, _$p));
          return _el$15;
        })(), (() => {
          var _el$17 = _$createElement("text");
          _$insert(_el$17, () => clipped(path.basename(props.projectRoot), 13));
          _$effect((_$p) => _$setProp(_el$17, "fg", CORTEX_THEME.skyBlue, _$p));
          return _el$17;
        })()];
      }
    }), null);
    _$effect((_p$) => {
      var _v$ = props.isExecuting() ? CORTEX_THEME.amberGold : CORTEX_THEME.brandIndigo, _v$2 = CORTEX_THEME.brandViolet, _v$3 = CORTEX_THEME.pureWhite, _v$4 = CORTEX_THEME.neonCyan, _v$5 = CORTEX_THEME.skyBlue, _v$6 = CORTEX_THEME.brandPurple, _v$7 = CORTEX_THEME.slateMuted;
      _v$ !== _p$.e && (_p$.e = _$setProp(_el$, "borderColor", _v$, _p$.e));
      _v$2 !== _p$.t && (_p$.t = _$setProp(_el$3, "fg", _v$2, _p$.t));
      _v$3 !== _p$.a && (_p$.a = _$setProp(_el$5, "fg", _v$3, _p$.a));
      _v$4 !== _p$.o && (_p$.o = _$setProp(_el$7, "fg", _v$4, _p$.o));
      _v$5 !== _p$.i && (_p$.i = _$setProp(_el$9, "fg", _v$5, _p$.i));
      _v$6 !== _p$.n && (_p$.n = _$setProp(_el$1, "fg", _v$6, _p$.n));
      _v$7 !== _p$.s && (_p$.s = _$setProp(_el$13, "fg", _v$7, _p$.s));
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
    return _el$;
  })();
}
function OperationalKPIHud(props) {
  return (() => {
    var _el$20 = _$createElement("box"), _el$21 = _$createElement("box"), _el$22 = _$createElement("text"), _el$23 = _$createElement("text"), _el$24 = _$createElement("box"), _el$25 = _$createElement("text"), _el$26 = _$createElement("text");
    _$insertNode(_el$20, _el$21);
    _$insertNode(_el$20, _el$24);
    _$setProp(_el$20, "flexDirection", "column");
    _$setProp(_el$20, "marginTop", 1);
    _$insertNode(_el$21, _el$22);
    _$insertNode(_el$21, _el$23);
    _$setProp(_el$21, "flexDirection", "row");
    _$insert(_el$22, () => `[ ${props.activeExecutions > 0 ? props.spinner() : "\u25CF"} ${props.activeExecutions} CURSO ] `);
    _$insert(_el$23, () => `[ \u25C6 ${props.inReview} REVW ]`);
    _$insertNode(_el$24, _el$25);
    _$insertNode(_el$24, _el$26);
    _$setProp(_el$24, "flexDirection", "row");
    _$setProp(_el$24, "marginTop", 0);
    _$insert(_el$25, () => `[ \u2713 ${props.doneTasks} DONE ] `);
    _$insert(_el$26, () => `[ ${props.attentionCount > 0 ? "\u2715" : "\u25CB"} ${props.attentionCount} ALRT ]`);
    _$effect((_p$) => {
      var _v$8 = props.activeExecutions > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted, _v$9 = props.inReview > 0 ? CORTEX_THEME.brandPurple : CORTEX_THEME.slateMuted, _v$0 = props.doneTasks > 0 ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.slateMuted, _v$1 = props.attentionCount > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted;
      _v$8 !== _p$.e && (_p$.e = _$setProp(_el$22, "fg", _v$8, _p$.e));
      _v$9 !== _p$.t && (_p$.t = _$setProp(_el$23, "fg", _v$9, _p$.t));
      _v$0 !== _p$.a && (_p$.a = _$setProp(_el$25, "fg", _v$0, _p$.a));
      _v$1 !== _p$.o && (_p$.o = _$setProp(_el$26, "fg", _v$1, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$20;
  })();
}
function Section(props) {
  return (() => {
    var _el$27 = _$createElement("box"), _el$28 = _$createElement("box"), _el$29 = _$createElement("text"), _el$31 = _$createElement("text");
    _$insertNode(_el$27, _el$28);
    _$setProp(_el$27, "flexDirection", "column");
    _$setProp(_el$27, "marginTop", 1);
    _$insertNode(_el$28, _el$29);
    _$insertNode(_el$28, _el$31);
    _$setProp(_el$28, "flexDirection", "row");
    _$setProp(_el$29, "selectable", false);
    _$insert(_el$29, () => props.expanded() ? "\u25BC " : "\u25B6 ");
    _$insert(_el$28, _$createComponent(Show, {
      get when() {
        return props.icon;
      },
      get children() {
        var _el$30 = _$createElement("text");
        _$insert(_el$30, () => `${props.icon} `);
        _$effect((_$p) => _$setProp(_el$30, "fg", CORTEX_THEME.brandViolet, _$p));
        return _el$30;
      }
    }), _el$31);
    _$setProp(_el$31, "selectable", false);
    _$insert(_el$31, () => props.title);
    _$insert(_el$28, _$createComponent(Show, {
      get when() {
        return props.badge;
      },
      get children() {
        var _el$32 = _$createElement("text");
        _$insert(_el$32, () => ` [ ${props.badge} ]`);
        _$effect((_$p) => _$setProp(_el$32, "fg", CORTEX_THEME.skyBlue, _$p));
        return _el$32;
      }
    }), null);
    _$insert(_el$27, _$createComponent(Show, {
      get when() {
        return props.expanded();
      },
      get children() {
        return props.children;
      }
    }), null);
    _$effect((_p$) => {
      var _v$10 = props.onToggle, _v$11 = props.expanded() ? CORTEX_THEME.neonCyan : CORTEX_THEME.slateMuted, _v$12 = CORTEX_THEME.pureWhite;
      _v$10 !== _p$.e && (_p$.e = _$setProp(_el$28, "onMouseDown", _v$10, _p$.e));
      _v$11 !== _p$.t && (_p$.t = _$setProp(_el$29, "fg", _v$11, _p$.t));
      _v$12 !== _p$.a && (_p$.a = _$setProp(_el$31, "fg", _v$12, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$27;
  })();
}
function MultiColorProgressBar(props) {
  const w = props.width || 14;
  const total = Math.max(props.total, 1);
  const doneW = Math.round(props.done / total * w);
  const revW = Math.round(props.inReview / total * w);
  const progW = Math.round(props.inProgress / total * w);
  const emptyW = Math.max(0, w - doneW - revW - progW);
  const pct = Math.round(props.done / total * 100);
  return (() => {
    var _el$33 = _$createElement("box"), _el$34 = _$createElement("box"), _el$35 = _$createElement("text"), _el$37 = _$createElement("text"), _el$38 = _$createElement("text"), _el$39 = _$createElement("text"), _el$40 = _$createElement("text"), _el$41 = _$createElement("text"), _el$42 = _$createElement("text"), _el$43 = _$createElement("box"), _el$44 = _$createElement("text"), _el$45 = _$createElement("text"), _el$46 = _$createElement("text"), _el$47 = _$createElement("text");
    _$insertNode(_el$33, _el$34);
    _$insertNode(_el$33, _el$43);
    _$setProp(_el$33, "flexDirection", "column");
    _$setProp(_el$33, "marginTop", 1);
    _$insertNode(_el$34, _el$35);
    _$insertNode(_el$34, _el$37);
    _$insertNode(_el$34, _el$38);
    _$insertNode(_el$34, _el$39);
    _$insertNode(_el$34, _el$40);
    _$insertNode(_el$34, _el$41);
    _$insertNode(_el$34, _el$42);
    _$setProp(_el$34, "flexDirection", "row");
    _$insertNode(_el$35, _$createTextNode(`DAG: `));
    _$insert(_el$37, () => "\u2588".repeat(doneW));
    _$insert(_el$38, () => "\u2593".repeat(revW));
    _$insert(_el$39, () => "\u2592".repeat(progW));
    _$insert(_el$40, () => "\u2591".repeat(emptyW));
    _$insert(_el$41, ` ${pct}%`);
    _$insert(_el$42, () => ` (${props.done}/${props.total})`);
    _$insertNode(_el$43, _el$44);
    _$insertNode(_el$43, _el$45);
    _$insertNode(_el$43, _el$46);
    _$insertNode(_el$43, _el$47);
    _$setProp(_el$43, "flexDirection", "row");
    _$insert(_el$44, () => `\u2713${props.done} `);
    _$insert(_el$45, () => `\u25C6${props.inReview} `);
    _$insert(_el$46, () => `\u25CF${props.inProgress} `);
    _$insert(_el$47, () => `\u25CB${Math.max(0, props.total - props.done - props.inReview - props.inProgress)}`);
    _$effect((_p$) => {
      var _v$13 = CORTEX_THEME.neonCyan, _v$14 = CORTEX_THEME.emeraldGreen, _v$15 = CORTEX_THEME.brandPurple, _v$16 = CORTEX_THEME.amberGold, _v$17 = CORTEX_THEME.slateBorder, _v$18 = CORTEX_THEME.pureWhite, _v$19 = CORTEX_THEME.slateMuted, _v$20 = CORTEX_THEME.emeraldGreen, _v$21 = CORTEX_THEME.brandPurple, _v$22 = CORTEX_THEME.amberGold, _v$23 = CORTEX_THEME.slateMuted;
      _v$13 !== _p$.e && (_p$.e = _$setProp(_el$35, "fg", _v$13, _p$.e));
      _v$14 !== _p$.t && (_p$.t = _$setProp(_el$37, "fg", _v$14, _p$.t));
      _v$15 !== _p$.a && (_p$.a = _$setProp(_el$38, "fg", _v$15, _p$.a));
      _v$16 !== _p$.o && (_p$.o = _$setProp(_el$39, "fg", _v$16, _p$.o));
      _v$17 !== _p$.i && (_p$.i = _$setProp(_el$40, "fg", _v$17, _p$.i));
      _v$18 !== _p$.n && (_p$.n = _$setProp(_el$41, "fg", _v$18, _p$.n));
      _v$19 !== _p$.s && (_p$.s = _$setProp(_el$42, "fg", _v$19, _p$.s));
      _v$20 !== _p$.h && (_p$.h = _$setProp(_el$44, "fg", _v$20, _p$.h));
      _v$21 !== _p$.r && (_p$.r = _$setProp(_el$45, "fg", _v$21, _p$.r));
      _v$22 !== _p$.d && (_p$.d = _$setProp(_el$46, "fg", _v$22, _p$.d));
      _v$23 !== _p$.l && (_p$.l = _$setProp(_el$47, "fg", _v$23, _p$.l));
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
    return _el$33;
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
    var _el$48 = _$createElement("box"), _el$49 = _$createElement("box"), _el$50 = _$createElement("text"), _el$51 = _$createElement("box"), _el$52 = _$createElement("text"), _el$54 = _$createElement("text"), _el$55 = _$createElement("box"), _el$56 = _$createElement("text"), _el$58 = _$createElement("text"), _el$59 = _$createElement("box");
    _$insertNode(_el$48, _el$49);
    _$insertNode(_el$48, _el$51);
    _$insertNode(_el$48, _el$55);
    _$insertNode(_el$48, _el$59);
    _$setProp(_el$48, "flexDirection", "column");
    _$setProp(_el$48, "marginTop", 1);
    _$setProp(_el$48, "paddingLeft", 1);
    _$setProp(_el$48, "paddingRight", 1);
    _$setProp(_el$48, "borderStyle", "rounded");
    _$insertNode(_el$49, _el$50);
    _$setProp(_el$49, "flexDirection", "row");
    _$insert(_el$50, () => `${props.spinner()} \u26A1 TAREA EN EJECUCI\xD3N`);
    _$insertNode(_el$51, _el$52);
    _$insertNode(_el$51, _el$54);
    _$setProp(_el$51, "flexDirection", "row");
    _$insertNode(_el$52, _$createTextNode(`\u{1F3AF} `));
    _$insert(_el$54, () => clipped(`${props.task.task_id} \xB7 ${props.task.title}`, 26));
    _$insertNode(_el$55, _el$56);
    _$insertNode(_el$55, _el$58);
    _$setProp(_el$55, "flexDirection", "row");
    _$insertNode(_el$56, _$createTextNode(` \u23F1 `));
    _$insert(_el$58, () => `+${elapsed()} `);
    _$insert(_el$55, _$createComponent(Show, {
      get when() {
        return ttlRemaining();
      },
      children: (ttl) => (() => {
        var _el$60 = _$createElement("text");
        _$insert(_el$60, () => `\u2502 \u{1F6E1} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`);
        _$effect((_$p) => _$setProp(_el$60, "fg", ttl() === "expirado" ? CORTEX_THEME.roseRed : CORTEX_THEME.skyBlue, _$p));
        return _el$60;
      })()
    }), null);
    _$setProp(_el$59, "flexDirection", "row");
    _$insert(_el$59, _$createComponent(Show, {
      get when() {
        return props.activeDelegation;
      },
      get fallback() {
        return (() => {
          var _el$61 = _$createElement("text");
          _$insert(_el$61, () => `  \u{1F9D1}\u200D\u{1F4BB} OpenCode Nativo${props.task.owner ? ` (${clipped(props.task.owner, 10)})` : ""}`);
          _$effect((_$p) => _$setProp(_el$61, "fg", CORTEX_THEME.brandPurple, _$p));
          return _el$61;
        })();
      },
      children: (del) => (() => {
        var _el$62 = _$createElement("text");
        _$insert(_el$62, () => `  \u{1F916} AGY ${del().transport || "direct"}${del().pane_id ? ` \xB7 ${del().pane_id}` : ""}${del().attempt ? ` \xB7 int #${del().attempt}` : ""}`);
        _$effect((_$p) => _$setProp(_el$62, "fg", CORTEX_THEME.neonCyan, _$p));
        return _el$62;
      })()
    }));
    _$effect((_p$) => {
      var _v$24 = CORTEX_THEME.amberGold, _v$25 = CORTEX_THEME.amberGold, _v$26 = CORTEX_THEME.skyBlue, _v$27 = CORTEX_THEME.pureWhite, _v$28 = CORTEX_THEME.slateMuted, _v$29 = CORTEX_THEME.amberGold;
      _v$24 !== _p$.e && (_p$.e = _$setProp(_el$48, "borderColor", _v$24, _p$.e));
      _v$25 !== _p$.t && (_p$.t = _$setProp(_el$50, "fg", _v$25, _p$.t));
      _v$26 !== _p$.a && (_p$.a = _$setProp(_el$52, "fg", _v$26, _p$.a));
      _v$27 !== _p$.o && (_p$.o = _$setProp(_el$54, "fg", _v$27, _p$.o));
      _v$28 !== _p$.i && (_p$.i = _$setProp(_el$56, "fg", _v$28, _p$.i));
      _v$29 !== _p$.n && (_p$.n = _$setProp(_el$58, "fg", _v$29, _p$.n));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0
    });
    return _el$48;
  })();
}
function TaskRows(props) {
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$63 = _$createElement("text");
        _$insertNode(_el$63, _$createTextNode(` Sin tareas durables activas`));
        _$effect((_$p) => _$setProp(_el$63, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$63;
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
            var _el$65 = _$createElement("box"), _el$66 = _$createElement("box"), _el$67 = _$createElement("text"), _el$68 = _$createElement("text"), _el$69 = _$createElement("text"), _el$70 = _$createElement("box"), _el$71 = _$createElement("text");
            _$insertNode(_el$65, _el$66);
            _$insertNode(_el$65, _el$70);
            _$setProp(_el$65, "flexDirection", "column");
            _$setProp(_el$65, "marginTop", 0);
            _$insertNode(_el$66, _el$67);
            _$insertNode(_el$66, _el$68);
            _$insertNode(_el$66, _el$69);
            _$setProp(_el$66, "flexDirection", "row");
            _$insert(_el$67, () => `  ${isProg ? props.spinner() : chip.icon} `);
            _$insert(_el$68, () => `[${chip.tag}] `);
            _$insert(_el$69, () => clipped(`${task.task_id} \xB7 ${task.title}`, 24));
            _$insertNode(_el$70, _el$71);
            _$setProp(_el$70, "flexDirection", "row");
            _$insert(_el$71, () => `     ${clipped(task.board_id, 10)}${task.owner ? ` \xB7 ${clipped(task.owner, 8)}` : ""}${task.lease_count ? ` \xB7 \u{1F6E1} ${task.lease_count}lk` : ""}`);
            _$effect((_p$) => {
              var _v$30 = chip.color, _v$31 = chip.color, _v$32 = isProg ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight, _v$33 = CORTEX_THEME.slateMuted;
              _v$30 !== _p$.e && (_p$.e = _$setProp(_el$67, "fg", _v$30, _p$.e));
              _v$31 !== _p$.t && (_p$.t = _$setProp(_el$68, "fg", _v$31, _p$.t));
              _v$32 !== _p$.a && (_p$.a = _$setProp(_el$69, "fg", _v$32, _p$.a));
              _v$33 !== _p$.o && (_p$.o = _$setProp(_el$71, "fg", _v$33, _p$.o));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0
            });
            return _el$65;
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
        var _el$72 = _$createElement("text");
        _$insertNode(_el$72, _$createTextNode(` Sin ejecuciones delegadas`));
        _$effect((_$p) => _$setProp(_el$72, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$72;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.jobs.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const chip = roleChip(job.role);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` +${formatDuration(props.now() - t)}` : "";
          });
          const statusCol = isRunning ? CORTEX_THEME.amberGold : job.status === "succeeded" ? CORTEX_THEME.emeraldGreen : CORTEX_THEME.roseRed;
          return (() => {
            var _el$74 = _$createElement("box"), _el$75 = _$createElement("box"), _el$76 = _$createElement("text"), _el$77 = _$createElement("text"), _el$78 = _$createElement("text"), _el$79 = _$createElement("text"), _el$80 = _$createElement("box"), _el$81 = _$createElement("text");
            _$insertNode(_el$74, _el$75);
            _$insertNode(_el$74, _el$80);
            _$setProp(_el$74, "flexDirection", "column");
            _$setProp(_el$74, "marginTop", 0);
            _$insertNode(_el$75, _el$76);
            _$insertNode(_el$75, _el$77);
            _$insertNode(_el$75, _el$78);
            _$insertNode(_el$75, _el$79);
            _$setProp(_el$75, "flexDirection", "row");
            _$setProp(_el$76, "fg", statusCol);
            _$insert(_el$76, () => `  ${isRunning ? props.spinner() : job.status === "succeeded" ? "\u2713" : "\u2715"} `);
            _$insert(_el$77, () => `${chip.icon} [${chip.tag}] `);
            _$insert(_el$78, () => job.role || "worker");
            _$setProp(_el$79, "fg", statusCol);
            _$insert(_el$79, elapsed);
            _$insertNode(_el$80, _el$81);
            _$setProp(_el$80, "flexDirection", "row");
            _$insert(_el$81, () => `     ${shortID(job.job_id)} \xB7 ${job.transport || "direct"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}${job.attempt ? ` \xB7 int #${job.attempt}` : ""}`);
            _$effect((_p$) => {
              var _v$34 = chip.color, _v$35 = isRunning ? CORTEX_THEME.pureWhite : CORTEX_THEME.slateLight, _v$36 = CORTEX_THEME.slateMuted;
              _v$34 !== _p$.e && (_p$.e = _$setProp(_el$77, "fg", _v$34, _p$.e));
              _v$35 !== _p$.t && (_p$.t = _$setProp(_el$78, "fg", _v$35, _p$.t));
              _v$36 !== _p$.a && (_p$.a = _$setProp(_el$81, "fg", _v$36, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$74;
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
        var _el$82 = _$createElement("text");
        _$insertNode(_el$82, _$createTextNode(` Sin alertas`));
        _$effect((_$p) => _$setProp(_el$82, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$82;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$84 = _$createElement("box"), _el$85 = _$createElement("box"), _el$86 = _$createElement("text"), _el$88 = _$createElement("text"), _el$89 = _$createElement("text");
          _$insertNode(_el$84, _el$85);
          _$insertNode(_el$84, _el$89);
          _$setProp(_el$84, "flexDirection", "column");
          _$insertNode(_el$85, _el$86);
          _$insertNode(_el$85, _el$88);
          _$setProp(_el$85, "flexDirection", "row");
          _$insertNode(_el$86, _$createTextNode(`  \u2715 `));
          _$insert(_el$88, () => clipped(item.title, 26));
          _$insert(_el$89, () => `     ${item.detail}`);
          _$effect((_p$) => {
            var _v$37 = CORTEX_THEME.roseRed, _v$38 = CORTEX_THEME.pureWhite, _v$39 = CORTEX_THEME.slateMuted;
            _v$37 !== _p$.e && (_p$.e = _$setProp(_el$86, "fg", _v$37, _p$.e));
            _v$38 !== _p$.t && (_p$.t = _$setProp(_el$88, "fg", _v$38, _p$.t));
            _v$39 !== _p$.a && (_p$.a = _$setProp(_el$89, "fg", _v$39, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$84;
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
  const successRate = createMemo(() => {
    const closed = succeededJobs() + failedJobs();
    if (closed === 0) return 100;
    return Math.round(succeededJobs() / closed * 100);
  });
  const syncAgeSec = createMemo(() => {
    const t = Date.parse(props.snapshot.generated_at);
    if (!Number.isFinite(t)) return 0;
    return Math.max(0, Math.floor((props.now() - t) / 1e3));
  });
  const gaugeWidth = 14;
  const healthBars = createMemo(() => {
    const rate = successRate() / 100;
    const filled = Math.round(rate * gaugeWidth);
    const empty = Math.max(0, gaugeWidth - filled);
    return {
      filled: "\u25A0".repeat(filled),
      empty: "\u25A1".repeat(empty)
    };
  });
  const healthColor = createMemo(() => {
    const rate = successRate();
    if (rate >= 90) return CORTEX_THEME.emeraldGreen;
    if (rate >= 70) return CORTEX_THEME.neonCyan;
    if (rate >= 50) return CORTEX_THEME.amberGold;
    return CORTEX_THEME.roseRed;
  });
  return (() => {
    var _el$90 = _$createElement("box"), _el$91 = _$createElement("box"), _el$92 = _$createElement("text"), _el$94 = _$createElement("text"), _el$96 = _$createElement("text"), _el$98 = _$createElement("box"), _el$100 = _$createElement("box"), _el$101 = _$createElement("text"), _el$102 = _$createElement("text"), _el$103 = _$createElement("box"), _el$104 = _$createElement("text"), _el$105 = _$createElement("text"), _el$106 = _$createElement("box"), _el$107 = _$createElement("text"), _el$109 = _$createElement("text"), _el$110 = _$createElement("text"), _el$111 = _$createElement("text"), _el$112 = _$createElement("box"), _el$113 = _$createElement("text"), _el$114 = _$createElement("box");
    _$insertNode(_el$90, _el$91);
    _$insertNode(_el$90, _el$98);
    _$insertNode(_el$90, _el$100);
    _$insertNode(_el$90, _el$103);
    _$insertNode(_el$90, _el$106);
    _$insertNode(_el$90, _el$112);
    _$insertNode(_el$90, _el$114);
    _$setProp(_el$90, "flexDirection", "column");
    _$setProp(_el$90, "marginTop", 1);
    _$setProp(_el$90, "paddingLeft", 1);
    _$setProp(_el$90, "paddingRight", 1);
    _$setProp(_el$90, "borderStyle", "rounded");
    _$insertNode(_el$91, _el$92);
    _$insertNode(_el$91, _el$94);
    _$insertNode(_el$91, _el$96);
    _$setProp(_el$91, "flexDirection", "row");
    _$insertNode(_el$92, _$createTextNode(`\u{1F9E0} `));
    _$insertNode(_el$94, _$createTextNode(`CONTROL MATRIX `));
    _$insertNode(_el$96, _$createTextNode(`\u25C8`));
    _$setProp(_el$98, "flexDirection", "row");
    _$insert(_el$98, _$createComponent(Show, {
      get when() {
        return activeJobs() > 0;
      },
      get fallback() {
        return (() => {
          var _el$116 = _$createElement("text");
          _$insert(_el$116, () => `  ${props.pulse()} SYNAPSE: SINCRONIZADO`);
          _$effect((_$p) => _$setProp(_el$116, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$116;
        })();
      },
      get children() {
        var _el$99 = _$createElement("text");
        _$insert(_el$99, () => `  ${props.spinner()} SYNAPSE: MOTOR ACTIVO`);
        _$effect((_$p) => _$setProp(_el$99, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$99;
      }
    }));
    _$insertNode(_el$100, _el$101);
    _$insertNode(_el$100, _el$102);
    _$setProp(_el$100, "flexDirection", "row");
    _$setProp(_el$100, "marginTop", 0);
    _$insert(_el$101, () => `[ \u2713 ${succeededJobs()} \xC9XITO ] `);
    _$insert(_el$102, () => `[ \u2715 ${failedJobs()} FALLO ]`);
    _$insertNode(_el$103, _el$104);
    _$insertNode(_el$103, _el$105);
    _$setProp(_el$103, "flexDirection", "row");
    _$setProp(_el$103, "marginTop", 0);
    _$insert(_el$104, () => `[ ${activeJobs() > 0 ? props.spinner() : "\u25CF"} ${activeJobs()} CURSO ] `);
    _$insert(_el$105, () => `[ \u{1F6E1} ${totalLeases()} LOCKS ]`);
    _$insertNode(_el$106, _el$107);
    _$insertNode(_el$106, _el$109);
    _$insertNode(_el$106, _el$110);
    _$insertNode(_el$106, _el$111);
    _$setProp(_el$106, "flexDirection", "row");
    _$setProp(_el$106, "marginTop", 0);
    _$insertNode(_el$107, _$createTextNode(`Salud: `));
    _$insert(_el$109, () => healthBars().filled);
    _$insert(_el$110, () => healthBars().empty);
    _$insert(_el$111, () => ` ${successRate()}%`);
    _$insertNode(_el$112, _el$113);
    _$setProp(_el$112, "flexDirection", "row");
    _$setProp(_el$112, "marginTop", 0);
    _$insert(_el$113, () => `\u{1F4CB} DAG: ${doneTasks()}/${totalTasks()} \xB7 \u{1F4C1} SQLite WAL`);
    _$setProp(_el$114, "flexDirection", "row");
    _$setProp(_el$114, "marginTop", 0);
    _$insert(_el$114, _$createComponent(Show, {
      get when() {
        return props.stale;
      },
      get fallback() {
        return (() => {
          var _el$117 = _$createElement("text");
          _$insert(_el$117, () => `\u{1F7E2} En vivo \xB7 Sync hace ${syncAgeSec()}s`);
          _$effect((_$p) => _$setProp(_el$117, "fg", CORTEX_THEME.emeraldGreen, _$p));
          return _el$117;
        })();
      },
      get children() {
        var _el$115 = _$createElement("text");
        _$insert(_el$115, () => `\u{1F7E1} Snapshot desfasado (+${syncAgeSec()}s)`);
        _$effect((_$p) => _$setProp(_el$115, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$115;
      }
    }));
    _$effect((_p$) => {
      var _v$40 = failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.brandIndigo, _v$41 = CORTEX_THEME.brandViolet, _v$42 = CORTEX_THEME.pureWhite, _v$43 = CORTEX_THEME.neonCyan, _v$44 = CORTEX_THEME.emeraldGreen, _v$45 = failedJobs() > 0 ? CORTEX_THEME.roseRed : CORTEX_THEME.slateMuted, _v$46 = activeJobs() > 0 ? CORTEX_THEME.amberGold : CORTEX_THEME.slateMuted, _v$47 = totalLeases() > 0 ? CORTEX_THEME.skyBlue : CORTEX_THEME.slateMuted, _v$48 = CORTEX_THEME.slateMuted, _v$49 = healthColor(), _v$50 = CORTEX_THEME.slateBorder, _v$51 = healthColor(), _v$52 = CORTEX_THEME.slateMuted;
      _v$40 !== _p$.e && (_p$.e = _$setProp(_el$90, "borderColor", _v$40, _p$.e));
      _v$41 !== _p$.t && (_p$.t = _$setProp(_el$92, "fg", _v$41, _p$.t));
      _v$42 !== _p$.a && (_p$.a = _$setProp(_el$94, "fg", _v$42, _p$.a));
      _v$43 !== _p$.o && (_p$.o = _$setProp(_el$96, "fg", _v$43, _p$.o));
      _v$44 !== _p$.i && (_p$.i = _$setProp(_el$101, "fg", _v$44, _p$.i));
      _v$45 !== _p$.n && (_p$.n = _$setProp(_el$102, "fg", _v$45, _p$.n));
      _v$46 !== _p$.s && (_p$.s = _$setProp(_el$104, "fg", _v$46, _p$.s));
      _v$47 !== _p$.h && (_p$.h = _$setProp(_el$105, "fg", _v$47, _p$.h));
      _v$48 !== _p$.r && (_p$.r = _$setProp(_el$107, "fg", _v$48, _p$.r));
      _v$49 !== _p$.d && (_p$.d = _$setProp(_el$109, "fg", _v$49, _p$.d));
      _v$50 !== _p$.l && (_p$.l = _$setProp(_el$110, "fg", _v$50, _p$.l));
      _v$51 !== _p$.u && (_p$.u = _$setProp(_el$111, "fg", _v$51, _p$.u));
      _v$52 !== _p$.c && (_p$.c = _$setProp(_el$113, "fg", _v$52, _p$.c));
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
      l: void 0,
      u: void 0,
      c: void 0
    });
    return _el$90;
  })();
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
  const inReviewTasks = createMemo(() => props.snapshot().summary.in_review || 0);
  const inProgressTasks = createMemo(() => props.snapshot().summary.in_progress || 0);
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);
  return (() => {
    var _el$118 = _$createElement("box");
    _$setProp(_el$118, "flexDirection", "column");
    _$insert(_el$118, _$createComponent(CortexCockpitHeader, {
      isExecuting,
      get nativeActivity() {
        return props.nativeActivity;
      },
      get projectRoot() {
        return props.snapshot().project_root;
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
    }), null);
    _$insert(_el$118, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$119 = _$createElement("text");
        _$insertNode(_el$119, _$createTextNode(`Conversaci\xF3n no disponible \xB7 esperando metadatos`));
        _$setProp(_el$119, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$119, "fg", CORTEX_THEME.amberGold, _$p));
        return _el$119;
      }
    }), null);
    _$insert(_el$118, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$121 = _$createElement("text");
        _$insertNode(_el$121, _$createTextNode(`Cargando estado de la conversaci\xF3n\u2026`));
        _$setProp(_el$121, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$121, "fg", CORTEX_THEME.slateMuted, _$p));
        return _el$121;
      }
    }), null);
    _$insert(_el$118, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$123 = _$createElement("text");
        _$insertNode(_el$123, _$createTextNode(`No se pudo actualizar \xB7 datos no confirmados`));
        _$setProp(_el$123, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$123, "fg", CORTEX_THEME.roseRed, _$p));
        return _el$123;
      }
    }), null);
    _$insert(_el$118, _$createComponent(Show, {
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
              get total() {
                return totalTasks();
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
            get theme() {
              return props.theme;
            }
          })
        }), _$createComponent(Section, {
          title: "Tablero de Tareas",
          icon: "\u{1F4CB}",
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
          title: "Workers & Delegaci\xF3n",
          icon: "\u{1F916}",
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
          title: "Centro de Atenci\xF3n",
          icon: "\u26A0\uFE0F",
          get badge() {
            return `${counts().attention} alertas`;
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
          }
        })];
      }
    }), null);
    return _el$118;
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
      var _el$125 = _$createElement("box"), _el$126 = _$createElement("text"), _el$128 = _$createElement("text"), _el$130 = _$createElement("text"), _el$132 = _$createElement("text"), _el$134 = _$createElement("text");
      _$insertNode(_el$125, _el$126);
      _$insertNode(_el$125, _el$128);
      _$insertNode(_el$125, _el$130);
      _$insertNode(_el$125, _el$132);
      _$insertNode(_el$125, _el$134);
      _$setProp(_el$125, "paddingLeft", 1);
      _$setProp(_el$125, "paddingRight", 1);
      _$setProp(_el$125, "flexDirection", "row");
      _$insertNode(_el$126, _$createTextNode(`\u{1F9E0} `));
      _$insertNode(_el$128, _$createTextNode(`CORTEX`));
      _$insertNode(_el$130, _$createTextNode(`\xB7`));
      _$insertNode(_el$132, _$createTextNode(`IA `));
      _$insertNode(_el$134, _$createTextNode(`\u2502 `));
      _$insert(_el$125, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$136 = _$createElement("box"), _el$137 = _$createElement("text"), _el$138 = _$createElement("text"), _el$140 = _$createElement("text");
            _$insertNode(_el$136, _el$137);
            _$insertNode(_el$136, _el$138);
            _$insertNode(_el$136, _el$140);
            _$setProp(_el$136, "flexDirection", "row");
            _$insert(_el$137, () => `\u25CF ${counts().active} en curso`);
            _$insertNode(_el$138, _$createTextNode(` \xB7 `));
            _$insert(_el$140, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$136, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$141 = _$createElement("text");
                  _$insertNode(_el$141, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$141, "fg", CORTEX_THEME.slateBorder, _$p));
                  return _el$141;
                })(), (() => {
                  var _el$143 = _$createElement("text");
                  _$insert(_el$143, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$143, "fg", CORTEX_THEME.roseRed, _$p));
                  return _el$143;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$58 = CORTEX_THEME.amberGold, _v$59 = CORTEX_THEME.slateBorder, _v$60 = CORTEX_THEME.brandPurple;
              _v$58 !== _p$.e && (_p$.e = _$setProp(_el$137, "fg", _v$58, _p$.e));
              _v$59 !== _p$.t && (_p$.t = _$setProp(_el$138, "fg", _v$59, _p$.t));
              _v$60 !== _p$.a && (_p$.a = _$setProp(_el$140, "fg", _v$60, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$136;
          })();
        },
        children: (task) => (() => {
          var _el$144 = _$createElement("box"), _el$145 = _$createElement("text"), _el$146 = _$createElement("text"), _el$147 = _$createElement("text");
          _$insertNode(_el$144, _el$145);
          _$insertNode(_el$144, _el$146);
          _$insertNode(_el$144, _el$147);
          _$setProp(_el$144, "flexDirection", "row");
          _$insert(_el$145, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$146, () => clipped(task().title, 20));
          _$insert(_el$147, () => ` \xB7 ${counts().active} activos`);
          _$effect((_p$) => {
            var _v$61 = CORTEX_THEME.amberGold, _v$62 = CORTEX_THEME.pureWhite, _v$63 = CORTEX_THEME.slateMuted;
            _v$61 !== _p$.e && (_p$.e = _$setProp(_el$145, "fg", _v$61, _p$.e));
            _v$62 !== _p$.t && (_p$.t = _$setProp(_el$146, "fg", _v$62, _p$.t));
            _v$63 !== _p$.a && (_p$.a = _$setProp(_el$147, "fg", _v$63, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$144;
        })()
      }), null);
      _$effect((_p$) => {
        var _v$53 = CORTEX_THEME.brandViolet, _v$54 = CORTEX_THEME.pureWhite, _v$55 = CORTEX_THEME.neonCyan, _v$56 = CORTEX_THEME.skyBlue, _v$57 = CORTEX_THEME.slateBorder;
        _v$53 !== _p$.e && (_p$.e = _$setProp(_el$126, "fg", _v$53, _p$.e));
        _v$54 !== _p$.t && (_p$.t = _$setProp(_el$128, "fg", _v$54, _p$.t));
        _v$55 !== _p$.a && (_p$.a = _$setProp(_el$130, "fg", _v$55, _p$.a));
        _v$56 !== _p$.o && (_p$.o = _$setProp(_el$132, "fg", _v$56, _p$.o));
        _v$57 !== _p$.i && (_p$.i = _$setProp(_el$134, "fg", _v$57, _p$.i));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0
      });
      return _el$125;
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
  cortex_ia_tui_default as default
};
