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
import { execFile, spawn } from "child_process";
import fs from "fs";
import path from "path";
import { For, Show, createEffect, createMemo, createRoot, createSignal, untrack } from "solid-js";
import { useTerminalDimensions } from "@opentui/solid";
var SNAPSHOT_POLL_INTERVAL_MS = 2500;
var SNAPSHOT_STALE_MS = 1e4;
var MAX_VISIBLE_ROWS = 4;
var SPINNER_FRAMES = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"];
var NEURAL_PULSE_FRAMES = ["\u25C8", "\u25C7", "\u25C6", "\u25C7"];
var TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
var DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
var ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
var ETA_HISTORY_KEY = "cortex.dashboard.eta.completions";
var ETA_HISTORY_LIMIT = 12;
var ETA_INTERVAL_WINDOW = 8;
var ETA_MIN_SAMPLES = 2;
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
var PALETTE_SOURCES = {
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
  panel: [["hue", "neutral", 800], ["background", "raised", "high"], ["backgroundPanel"]]
};
var PALETTE_FALLBACK = {
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
  panel: CORTEX_THEME.slateCard
};
function readColor(source, path2) {
  let node = source;
  for (const key of path2) {
    if (typeof node !== "object" || node === null) return void 0;
    node = node[key];
  }
  if (typeof node === "string") return node;
  if (typeof node === "object" && node !== null) {
    const candidate = node;
    if (typeof candidate.r === "number" && typeof candidate.g === "number" && typeof candidate.b === "number") {
      return node;
    }
  }
  return void 0;
}
function resolvePalette(theme) {
  const source = theme;
  const palette = {};
  for (const role of Object.keys(PALETTE_SOURCES)) {
    let resolved;
    for (const path2 of PALETTE_SOURCES[role]) {
      resolved = readColor(source, path2);
      if (resolved !== void 0) break;
    }
    palette[role] = resolved ?? PALETTE_FALLBACK[role];
  }
  return palette;
}
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
function openWebConsole(boardId, taskId) {
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
      windowsHide: true
    });
    child.unref();
  } catch {
  }
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
function formatTokens(value) {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1e6) return `${(value / 1e6).toFixed(1)}M`;
  if (value >= 1e3) return `${(value / 1e3).toFixed(1)}k`;
  return String(Math.round(value));
}
function formatCost(value) {
  if (!Number.isFinite(value) || value <= 0) return "0.00";
  return value >= 0.01 ? value.toFixed(2) : value.toFixed(4);
}
function sessionRecord(api, sessionID) {
  if (!sessionID) return void 0;
  return api.state?.session?.get?.(sessionID) || api.data?.session?.get?.(sessionID);
}
function sessionStartTime(api, sessionID) {
  const created = sessionRecord(api, sessionID)?.time?.created;
  if (typeof created !== "number" || !Number.isFinite(created) || created <= 0) return void 0;
  return created < 1e12 ? created * 1e3 : created;
}
function sessionCostUsd(api, sessionID) {
  const cost = sessionRecord(api, sessionID)?.cost;
  return typeof cost === "number" && Number.isFinite(cost) && cost > 0 ? cost : void 0;
}
function sessionContextLimit(api, sessionID) {
  const model = sessionRecord(api, sessionID)?.model;
  if (!model?.id || !model?.providerID) return void 0;
  const providers = api.state?.provider || api.data?.provider;
  if (!Array.isArray(providers)) return void 0;
  const limit = providers.find((p) => p?.id === model.providerID)?.models?.[model.id]?.limit?.context;
  return typeof limit === "number" && Number.isFinite(limit) && limit > 0 ? limit : void 0;
}
function sessionMessages(api, sessionID) {
  if (!sessionID) return void 0;
  for (const source of [api.state?.session, api.data?.session, api.session]) {
    if (!source || typeof source.messages !== "function") continue;
    try {
      const messages = source.messages(sessionID);
      if (Array.isArray(messages)) return messages;
    } catch {
    }
  }
  return void 0;
}
function contextTokensUsed(messages) {
  if (!messages) return void 0;
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const info = messages[i]?.info || messages[i];
    if (info?.role !== "assistant") continue;
    const tokens = info?.tokens;
    if (!tokens) return void 0;
    const used = (tokens.input || 0) + (tokens.cache?.read || 0) + (tokens.cache?.write || 0) + (tokens.output || 0) + (tokens.reasoning || 0);
    return used > 0 ? used : void 0;
  }
  return void 0;
}
function loadEtaHistory(raw) {
  if (!Array.isArray(raw)) return [];
  return raw.filter((value) => typeof value === "number" && Number.isFinite(value) && value > 0).slice(-ETA_HISTORY_LIMIT);
}
function meanCompletionInterval(stamps) {
  if (stamps.length < ETA_MIN_SAMPLES) return void 0;
  const intervals = [];
  for (let i = 1; i < stamps.length; i += 1) {
    const delta = stamps[i] - stamps[i - 1];
    if (delta > 0) intervals.push(delta);
  }
  if (intervals.length === 0) return void 0;
  const recent = intervals.slice(-ETA_INTERVAL_WINDOW);
  return recent.reduce((sum, value) => sum + value, 0) / recent.length;
}
function roleChip(role, palette) {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return {
    icon: "\u{1F9E0}",
    color: palette.primary,
    tag: "ORCH"
  };
  if (r.includes("impl")) return {
    icon: "\u26A1",
    color: palette.warning,
    tag: "IMPL"
  };
  if (r.includes("rev")) return {
    icon: "\u2696\uFE0F",
    color: palette.accent,
    tag: "REVW"
  };
  if (r.includes("inv")) return {
    icon: "\u{1F50D}",
    color: palette.sky,
    tag: "INVS"
  };
  if (r.includes("plan")) return {
    icon: "\u{1F4CB}",
    color: palette.info,
    tag: "PLAN"
  };
  if (r.includes("disc")) return {
    icon: "\u{1F9ED}",
    color: palette.success,
    tag: "DISC"
  };
  return {
    icon: "\u{1F916}",
    color: palette.textMuted,
    tag: r.slice(0, 4).toUpperCase() || "WORK"
  };
}
function taskStatusChip(status, palette) {
  switch (status) {
    case "done":
      return {
        icon: "\u2713",
        color: palette.success,
        tag: "DONE"
      };
    case "in_progress":
      return {
        icon: "\u26A1",
        color: palette.warning,
        tag: "PROG"
      };
    case "in_review":
      return {
        icon: "\u25C6",
        color: palette.accent,
        tag: "REVW"
      };
    case "ready":
      return {
        icon: "\u25B6",
        color: palette.sky,
        tag: "RDY "
      };
    case "blocked":
      return {
        icon: "\u2715",
        color: palette.error,
        tag: "BLCK"
      };
    case "superseded":
      return {
        icon: "\u21B7",
        color: palette.textMuted,
        tag: "SPRS"
      };
    case "backlog":
    default:
      return {
        icon: "\u25CB",
        color: palette.textMuted,
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
var lastKnownSessionID;
function extractSessionID(val) {
  if (!val) return void 0;
  if (typeof val === "function") {
    try {
      const res = val();
      const extracted = extractSessionID(res);
      if (extracted) return extracted;
    } catch {
    }
  }
  if (typeof val === "string") {
    const trimmed = val.trim();
    if (/^[A-Za-z0-9_-]{1,256}$/.test(trimmed)) return trimmed;
    const match = trimmed.match(/(?:^|\/|#)session(?:s)?\/([A-Za-z0-9_-]{1,256})/);
    if (match) return match[1];
  }
  if (typeof val === "object") {
    const candidates = [val.sessionID, val.sessionId, val.session_id, val.params?.sessionID, val.params?.sessionId, val.params?.session_id, val.params?.id, val.type === "session" ? val.id : void 0, val.name === "session" ? val.id : void 0, val.session?.id, val.session?.sessionID];
    for (const c of candidates) {
      const extracted = extractSessionID(c);
      if (extracted) return extracted;
    }
    if (typeof val.path === "string") {
      const extracted = extractSessionID(val.path);
      if (extracted) return extracted;
    }
  }
  return void 0;
}
function currentSessionID(api, explicit) {
  const fromExplicit = extractSessionID(explicit);
  if (fromExplicit) {
    lastKnownSessionID = fromExplicit;
    return fromExplicit;
  }
  const route = api.route?.current || (typeof api.ui?.router?.current === "function" ? api.ui.router.current() : void 0);
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
          const active = tabs.find((t) => t && (t.active === true || t.selected === true || t.current === true));
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
    } catch {
    }
  }
  const sessionApiId = extractSessionID(api.session) || extractSessionID(api.state?.session) || extractSessionID(api.data?.session);
  if (sessionApiId) {
    lastKnownSessionID = sessionApiId;
    return sessionApiId;
  }
  return lastKnownSessionID;
}
function nativeSessionActivity(api, explicit) {
  const id = currentSessionID(api, explicit);
  if (!id) return void 0;
  const session = api.state?.session?.get?.(id) || api.data?.session?.get?.(id);
  if (session && session.id !== id) return "unknown";
  const statusObj = api.state?.session?.status?.(id) || api.data?.session?.status?.(id);
  const status = typeof statusObj === "string" ? statusObj : statusObj?.type;
  return status === "busy" || status === "idle" || status === "retry" ? status : "unknown";
}
function conversationScope(api, explicit) {
  const sessionID = currentSessionID(api, explicit);
  const session = sessionID ? api.state?.session?.get?.(sessionID) || api.data?.session?.get?.(sessionID) : void 0;
  const project = api.state?.path?.directory || session?.directory || api.location?.directory || (typeof api.data?.location?.default === "function" ? api.data.location.default()?.directory : void 0) || process.cwd();
  if (!sessionID) {
    return {
      sessionID: "global",
      rootSessionID: "global",
      project
    };
  }
  let current = sessionID;
  const seen = /* @__PURE__ */ new Set();
  while (seen.size < 64 && /^[A-Za-z0-9_-]{1,256}$/.test(current) && !seen.has(current)) {
    seen.add(current);
    const currSession = api.state?.session?.get?.(current) || api.data?.session?.get?.(current);
    if (!currSession || currSession.id !== current) {
      if (api.data?.session?.root && typeof api.data.session.root === "function") {
        try {
          const root = api.data.session.root(sessionID);
          if (root) return {
            sessionID,
            rootSessionID: root,
            project
          };
        } catch {
        }
      }
      return {
        sessionID,
        rootSessionID: current,
        project
      };
    }
    const parent = currSession.parentID || currSession.parentId;
    if (!parent) return {
      sessionID,
      rootSessionID: current,
      project
    };
    current = parent;
  }
  return {
    sessionID,
    rootSessionID: sessionID,
    project
  };
}
function CortexCockpitHeader(props) {
  const projectName = createMemo(() => {
    if (!props.projectRoot) return "";
    return path.basename(props.projectRoot);
  });
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$ = _$createElement("box"), _el$6 = _$createElement("box"), _el$7 = _$createElement("text");
    _$insertNode(_el$, _el$6);
    _$setProp(_el$, "flexDirection", "column");
    _$setProp(_el$, "borderStyle", "rounded");
    _$setProp(_el$, "titleAlignment", "left");
    _$setProp(_el$, "paddingLeft", 1);
    _$setProp(_el$, "paddingRight", 1);
    _$insert(_el$, _$createComponent(Show, {
      get when() {
        return projectName();
      },
      get children() {
        var _el$2 = _$createElement("box"), _el$3 = _$createElement("text"), _el$5 = _$createElement("text");
        _$insertNode(_el$2, _el$3);
        _$insertNode(_el$2, _el$5);
        _$setProp(_el$2, "flexDirection", "row");
        _$setProp(_el$2, "marginTop", 0);
        _$insertNode(_el$3, _$createTextNode(`\u{1F4C1} `));
        _$insert(_el$5, () => clipped(projectName(), Math.max(6, props.textLimit - 4)));
        _$effect((_p$) => {
          var _v$ = palette.sky, _v$2 = palette.sky;
          _v$ !== _p$.e && (_p$.e = _$setProp(_el$3, "fg", _v$, _p$.e));
          _v$2 !== _p$.t && (_p$.t = _$setProp(_el$5, "fg", _v$2, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$2;
      }
    }), _el$6);
    _$insert(_el$, _$createComponent(Show, {
      get when() {
        return props.sessionElapsed?.();
      },
      children: (elapsed) => (() => {
        var _el$9 = _$createElement("box"), _el$0 = _$createElement("text"), _el$10 = _$createElement("text");
        _$insertNode(_el$9, _el$0);
        _$insertNode(_el$9, _el$10);
        _$setProp(_el$9, "flexDirection", "row");
        _$setProp(_el$9, "marginTop", 0);
        _$insertNode(_el$0, _$createTextNode(`\u23F1 `));
        _$insert(_el$10, () => clipped(`Sesi\xF3n ${elapsed()}`, Math.max(8, props.textLimit)));
        _$effect((_p$) => {
          var _v$8 = palette.info, _v$9 = palette.sky;
          _v$8 !== _p$.e && (_p$.e = _$setProp(_el$0, "fg", _v$8, _p$.e));
          _v$9 !== _p$.t && (_p$.t = _$setProp(_el$10, "fg", _v$9, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$9;
      })()
    }), _el$6);
    _$insertNode(_el$6, _el$7);
    _$setProp(_el$6, "flexDirection", "row");
    _$setProp(_el$6, "marginTop", 0);
    _$insertNode(_el$7, _$createTextNode(`[\u{1F310} Web]`));
    _$setProp(_el$7, "onMouseDown", () => openWebConsole());
    _$setProp(_el$7, "selectable", false);
    _$effect((_p$) => {
      var _v$3 = props.isExecuting() ? palette.warning : palette.primary, _v$4 = props.isExecuting() ? `\u{1F9E0} CORTEX\xB7IA v2.0 [${props.spinner()} ACTIVO]` : "\u{1F9E0} CORTEX\xB7IA v2.0 [\u25CF STANDBY]", _v$5 = palette.accent, _v$6 = palette.panel, _v$7 = palette.info;
      _v$3 !== _p$.e && (_p$.e = _$setProp(_el$, "borderColor", _v$3, _p$.e));
      _v$4 !== _p$.t && (_p$.t = _$setProp(_el$, "title", _v$4, _p$.t));
      _v$5 !== _p$.a && (_p$.a = _$setProp(_el$, "titleColor", _v$5, _p$.a));
      _v$6 !== _p$.o && (_p$.o = _$setProp(_el$, "backgroundColor", _v$6, _p$.o));
      _v$7 !== _p$.i && (_p$.i = _$setProp(_el$7, "fg", _v$7, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
    });
    return _el$;
  })();
}
function OperationalKPIHud(props) {
  const isAllZero = () => props.activeExecutions === 0 && props.inReview === 0 && props.doneTasks === 0 && props.attentionCount === 0;
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$11 = _$createElement("box");
    _$setProp(_el$11, "flexDirection", "column");
    _$setProp(_el$11, "marginTop", 1);
    _$insert(_el$11, _$createComponent(Show, {
      get when() {
        return !isAllZero();
      },
      get fallback() {
        return (() => {
          var _el$18 = _$createElement("box"), _el$19 = _$createElement("text");
          _$insertNode(_el$18, _el$19);
          _$setProp(_el$18, "flexDirection", "row");
          _$insertNode(_el$19, _$createTextNode(`\u25CB 0 curso \xB7 0 revw \xB7 0 done \xB7 0 alrt`));
          _$effect((_$p) => _$setProp(_el$19, "fg", palette.textMuted, _$p));
          return _el$18;
        })();
      },
      get children() {
        return [(() => {
          var _el$12 = _$createElement("box"), _el$13 = _$createElement("text"), _el$14 = _$createElement("text");
          _$insertNode(_el$12, _el$13);
          _$insertNode(_el$12, _el$14);
          _$setProp(_el$12, "flexDirection", "row");
          _$insert(_el$13, () => `[${props.activeExecutions > 0 ? props.spinner() : "\u25CF"} ${props.activeExecutions} CURSO] `);
          _$insert(_el$14, () => `[\u25C6 ${props.inReview} REVW]`);
          _$effect((_p$) => {
            var _v$0 = props.activeExecutions > 0 ? palette.warning : palette.textMuted, _v$1 = props.inReview > 0 ? palette.accent : palette.textMuted;
            _v$0 !== _p$.e && (_p$.e = _$setProp(_el$13, "fg", _v$0, _p$.e));
            _v$1 !== _p$.t && (_p$.t = _$setProp(_el$14, "fg", _v$1, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$12;
        })(), (() => {
          var _el$15 = _$createElement("box"), _el$16 = _$createElement("text"), _el$17 = _$createElement("text");
          _$insertNode(_el$15, _el$16);
          _$insertNode(_el$15, _el$17);
          _$setProp(_el$15, "flexDirection", "row");
          _$setProp(_el$15, "marginTop", 0);
          _$insert(_el$16, () => `[\u2713 ${props.doneTasks} DONE] `);
          _$insert(_el$17, () => `[${props.attentionCount > 0 ? "\u2715" : "\u25CB"} ${props.attentionCount} ALRT]`);
          _$effect((_p$) => {
            var _v$10 = props.doneTasks > 0 ? palette.success : palette.textMuted, _v$11 = props.attentionCount > 0 ? palette.error : palette.textMuted;
            _v$10 !== _p$.e && (_p$.e = _$setProp(_el$16, "fg", _v$10, _p$.e));
            _v$11 !== _p$.t && (_p$.t = _$setProp(_el$17, "fg", _v$11, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$15;
        })()];
      }
    }));
    return _el$11;
  })();
}
function Section(props) {
  const displayTitle = createMemo(() => props.compact && props.shortTitle ? props.shortTitle : props.title);
  const displayBadge = createMemo(() => props.compact && props.shortBadge ? props.shortBadge : props.badge);
  const palette = resolvePalette(props.theme);
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
        _$effect((_$p) => _$setProp(_el$24, "fg", palette.accentAlt, _$p));
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
        _$effect((_$p) => _$setProp(_el$26, "fg", palette.sky, _$p));
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
      var _v$12 = props.onToggle, _v$13 = props.expanded() ? palette.info : palette.textMuted, _v$14 = palette.text;
      _v$12 !== _p$.e && (_p$.e = _$setProp(_el$22, "onMouseDown", _v$12, _p$.e));
      _v$13 !== _p$.t && (_p$.t = _$setProp(_el$23, "fg", _v$13, _p$.t));
      _v$14 !== _p$.a && (_p$.a = _$setProp(_el$25, "fg", _v$14, _p$.a));
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
  const w = createMemo(() => Math.max(3, Math.min(props.width || 14, props.textLimit - (props.compact ? 8 : 14))));
  const total = createMemo(() => Math.max(props.total, 1));
  const doneW = createMemo(() => Math.min(w(), Math.round(props.done / total() * w())));
  const revW = createMemo(() => Math.min(Math.max(0, w() - doneW()), Math.round(props.inReview / total() * w())));
  const progW = createMemo(() => Math.min(Math.max(0, w() - doneW() - revW()), Math.round(props.inProgress / total() * w())));
  const blckW = createMemo(() => Math.min(Math.max(0, w() - doneW() - revW() - progW()), Math.round((props.blocked || 0) / total() * w())));
  const emptyW = createMemo(() => Math.max(0, w() - doneW() - revW() - progW() - blckW()));
  const pct = createMemo(() => Math.min(100, Math.max(0, Math.round(props.done / total() * 100))));
  const waiting = createMemo(() => Math.max(0, props.total - props.done - props.inReview - props.inProgress - (props.blocked || 0)));
  const palette = resolvePalette(props.theme);
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
    _$insert(_el$30, () => "\u2588".repeat(doneW()));
    _$insert(_el$31, () => "\u2593".repeat(revW()));
    _$insert(_el$32, () => "\u2592".repeat(progW()));
    _$insert(_el$28, _$createComponent(Show, {
      get when() {
        return blckW() > 0;
      },
      get children() {
        var _el$33 = _$createElement("text");
        _$insert(_el$33, () => "\u2593".repeat(blckW()));
        _$effect((_$p) => _$setProp(_el$33, "fg", palette.error, _$p));
        return _el$33;
      }
    }), _el$34);
    _$insert(_el$34, () => "\u2591".repeat(emptyW()));
    _$insert(_el$35, () => ` ${pct()}%`);
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
        _$effect((_$p) => _$setProp(_el$41, "fg", palette.error, _$p));
        return _el$41;
      }
    }), _el$42);
    _$insert(_el$42, () => `\u25CB${waiting()}`);
    _$effect((_p$) => {
      var _v$15 = palette.info, _v$16 = palette.success, _v$17 = palette.accent, _v$18 = palette.warning, _v$19 = palette.border, _v$20 = palette.text, _v$21 = palette.textMuted, _v$22 = palette.success, _v$23 = palette.accent, _v$24 = palette.warning, _v$25 = palette.textMuted;
      _v$15 !== _p$.e && (_p$.e = _$setProp(_el$29, "fg", _v$15, _p$.e));
      _v$16 !== _p$.t && (_p$.t = _$setProp(_el$30, "fg", _v$16, _p$.t));
      _v$17 !== _p$.a && (_p$.a = _$setProp(_el$31, "fg", _v$17, _p$.a));
      _v$18 !== _p$.o && (_p$.o = _$setProp(_el$32, "fg", _v$18, _p$.o));
      _v$19 !== _p$.i && (_p$.i = _$setProp(_el$34, "fg", _v$19, _p$.i));
      _v$20 !== _p$.n && (_p$.n = _$setProp(_el$35, "fg", _v$20, _p$.n));
      _v$21 !== _p$.s && (_p$.s = _$setProp(_el$36, "fg", _v$21, _p$.s));
      _v$22 !== _p$.h && (_p$.h = _$setProp(_el$38, "fg", _v$22, _p$.h));
      _v$23 !== _p$.r && (_p$.r = _$setProp(_el$39, "fg", _v$23, _p$.r));
      _v$24 !== _p$.d && (_p$.d = _$setProp(_el$40, "fg", _v$24, _p$.d));
      _v$25 !== _p$.l && (_p$.l = _$setProp(_el$42, "fg", _v$25, _p$.l));
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
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$43 = _$createElement("box"), _el$44 = _$createElement("box"), _el$45 = _$createElement("text"), _el$47 = _$createElement("text"), _el$48 = _$createElement("box"), _el$49 = _$createElement("text"), _el$51 = _$createElement("text"), _el$52 = _$createElement("box"), _el$53 = _$createElement("box"), _el$54 = _$createElement("text");
    _$insertNode(_el$43, _el$44);
    _$insertNode(_el$43, _el$48);
    _$insertNode(_el$43, _el$52);
    _$insertNode(_el$43, _el$53);
    _$setProp(_el$43, "flexDirection", "column");
    _$setProp(_el$43, "marginTop", 1);
    _$setProp(_el$43, "paddingLeft", 1);
    _$setProp(_el$43, "paddingRight", 1);
    _$setProp(_el$43, "borderStyle", "rounded");
    _$setProp(_el$43, "titleAlignment", "left");
    _$insertNode(_el$44, _el$45);
    _$insertNode(_el$44, _el$47);
    _$setProp(_el$44, "flexDirection", "row");
    _$insertNode(_el$45, _$createTextNode(`\u{1F3AF} `));
    _$insert(_el$47, () => clipped(`${props.task.task_id} \xB7 ${props.task.title}`, props.textLimit - 3));
    _$insertNode(_el$48, _el$49);
    _$insertNode(_el$48, _el$51);
    _$setProp(_el$48, "flexDirection", "row");
    _$insertNode(_el$49, _$createTextNode(` \u23F1 `));
    _$insert(_el$51, () => `+${elapsed()} `);
    _$insert(_el$48, _$createComponent(Show, {
      get when() {
        return ttlRemaining();
      },
      children: (ttl) => (() => {
        var _el$56 = _$createElement("text");
        _$insert(_el$56, () => `\u2502 \u{1F6E1} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`);
        _$effect((_$p) => _$setProp(_el$56, "fg", ttl() === "expirado" ? palette.error : palette.sky, _$p));
        return _el$56;
      })()
    }), null);
    _$setProp(_el$52, "flexDirection", "row");
    _$insert(_el$52, _$createComponent(Show, {
      get when() {
        return props.activeDelegation;
      },
      get fallback() {
        return (() => {
          var _el$57 = _$createElement("text");
          _$insert(_el$57, () => clipped(`  \u{1F4CB} Tarea durable${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit));
          _$effect((_$p) => _$setProp(_el$57, "fg", palette.accent, _$p));
          return _el$57;
        })();
      },
      children: (del) => (() => {
        var _el$58 = _$createElement("text");
        _$insert(_el$58, () => `  \u{1F916} AGY ${del().transport || "direct"}${del().pane_id ? ` \xB7 ${del().pane_id}` : ""}${del().attempt ? ` \xB7 int #${del().attempt}` : ""}`);
        _$effect((_$p) => _$setProp(_el$58, "fg", palette.info, _$p));
        return _el$58;
      })()
    }));
    _$insertNode(_el$53, _el$54);
    _$setProp(_el$53, "flexDirection", "row");
    _$setProp(_el$53, "marginTop", 0);
    _$setProp(_el$53, "onMouseDown", () => openWebConsole(props.task.board_id, props.task.task_id));
    _$insertNode(_el$54, _$createTextNode(`  [ \u{1F310} Ver detalle en Web ]`));
    _$setProp(_el$54, "selectable", false);
    _$effect((_p$) => {
      var _v$26 = palette.warning, _v$27 = clipped(`${props.spinner()} \u26A1 TAREA EN EJECUCI\xD3N`, props.textLimit), _v$28 = palette.warning, _v$29 = palette.panel, _v$30 = palette.sky, _v$31 = palette.text, _v$32 = palette.textMuted, _v$33 = palette.warning, _v$34 = palette.info;
      _v$26 !== _p$.e && (_p$.e = _$setProp(_el$43, "borderColor", _v$26, _p$.e));
      _v$27 !== _p$.t && (_p$.t = _$setProp(_el$43, "title", _v$27, _p$.t));
      _v$28 !== _p$.a && (_p$.a = _$setProp(_el$43, "titleColor", _v$28, _p$.a));
      _v$29 !== _p$.o && (_p$.o = _$setProp(_el$43, "backgroundColor", _v$29, _p$.o));
      _v$30 !== _p$.i && (_p$.i = _$setProp(_el$45, "fg", _v$30, _p$.i));
      _v$31 !== _p$.n && (_p$.n = _$setProp(_el$47, "fg", _v$31, _p$.n));
      _v$32 !== _p$.s && (_p$.s = _$setProp(_el$49, "fg", _v$32, _p$.s));
      _v$33 !== _p$.h && (_p$.h = _$setProp(_el$51, "fg", _v$33, _p$.h));
      _v$34 !== _p$.r && (_p$.r = _$setProp(_el$54, "fg", _v$34, _p$.r));
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
      r: void 0
    });
    return _el$43;
  })();
}
function TaskRows(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$59 = _$createElement("text");
        _$insertNode(_el$59, _$createTextNode(` \u25CB Sin tareas en cola`));
        _$effect((_$p) => _$setProp(_el$59, "fg", palette.textMuted, _$p));
        return _el$59;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.tasks.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (task) => {
          const chip = taskStatusChip(task.status, palette);
          const isProg = task.status === "in_progress";
          return (() => {
            var _el$61 = _$createElement("box"), _el$62 = _$createElement("box"), _el$63 = _$createElement("text"), _el$64 = _$createElement("text"), _el$65 = _$createElement("text"), _el$66 = _$createElement("text"), _el$68 = _$createElement("box"), _el$69 = _$createElement("text");
            _$insertNode(_el$61, _el$62);
            _$insertNode(_el$61, _el$68);
            _$setProp(_el$61, "flexDirection", "column");
            _$setProp(_el$61, "marginTop", 0);
            _$insertNode(_el$62, _el$63);
            _$insertNode(_el$62, _el$64);
            _$insertNode(_el$62, _el$65);
            _$insertNode(_el$62, _el$66);
            _$setProp(_el$62, "flexDirection", "row");
            _$insert(_el$63, () => `  ${isProg ? props.spinner() : chip.icon} `);
            _$insert(_el$64, () => `[${chip.tag}] `);
            _$insert(_el$65, () => clipped(task.task_id, Math.max(8, props.textLimit - 14)));
            _$insertNode(_el$66, _$createTextNode(` [\u{1F310}]`));
            _$setProp(_el$66, "onMouseDown", () => openWebConsole(task.board_id, task.task_id));
            _$setProp(_el$66, "selectable", false);
            _$insertNode(_el$68, _el$69);
            _$setProp(_el$68, "flexDirection", "row");
            _$insert(_el$69, () => `     ${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` \xB7 ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` \xB7 \u{1F6E1} ${task.lease_count}lk` : ""}`);
            _$effect((_p$) => {
              var _v$35 = chip.color, _v$36 = chip.color, _v$37 = isProg ? palette.text : palette.textSoft, _v$38 = palette.info, _v$39 = palette.textMuted;
              _v$35 !== _p$.e && (_p$.e = _$setProp(_el$63, "fg", _v$35, _p$.e));
              _v$36 !== _p$.t && (_p$.t = _$setProp(_el$64, "fg", _v$36, _p$.t));
              _v$37 !== _p$.a && (_p$.a = _$setProp(_el$65, "fg", _v$37, _p$.a));
              _v$38 !== _p$.o && (_p$.o = _$setProp(_el$66, "fg", _v$38, _p$.o));
              _v$39 !== _p$.i && (_p$.i = _$setProp(_el$69, "fg", _v$39, _p$.i));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0,
              i: void 0
            });
            return _el$61;
          })();
        }
      });
    }
  });
}
function DelegationRows(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.jobs.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$70 = _$createElement("text");
        _$insertNode(_el$70, _$createTextNode(` \u25CB Sin workers activos`));
        _$effect((_$p) => _$setProp(_el$70, "fg", palette.textMuted, _$p));
        return _el$70;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.jobs.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (job) => {
          const isRunning = ["running", "starting", "accepted"].includes(job.status);
          const chip = roleChip(job.role || "", palette);
          const elapsed = createMemo(() => {
            if (!isRunning || !job.updated_at) return "";
            const t = Date.parse(job.updated_at);
            return Number.isFinite(t) ? ` +${formatDuration(props.now() - t)}` : "";
          });
          const statusCol = isRunning ? palette.warning : job.status === "succeeded" ? palette.success : palette.error;
          return (() => {
            var _el$72 = _$createElement("box"), _el$73 = _$createElement("box"), _el$74 = _$createElement("text"), _el$75 = _$createElement("text"), _el$76 = _$createElement("text"), _el$77 = _$createElement("text"), _el$78 = _$createElement("box"), _el$79 = _$createElement("text");
            _$insertNode(_el$72, _el$73);
            _$insertNode(_el$72, _el$78);
            _$setProp(_el$72, "flexDirection", "column");
            _$setProp(_el$72, "marginTop", 0);
            _$insertNode(_el$73, _el$74);
            _$insertNode(_el$73, _el$75);
            _$insertNode(_el$73, _el$76);
            _$insertNode(_el$73, _el$77);
            _$setProp(_el$73, "flexDirection", "row");
            _$setProp(_el$74, "fg", statusCol);
            _$insert(_el$74, () => `  ${isRunning ? props.spinner() : job.status === "succeeded" ? "\u2713" : "\u2715"} `);
            _$insert(_el$75, () => `${chip.icon} [${chip.tag}] `);
            _$insert(_el$76, () => clipped(job.role || "worker", Math.max(6, props.textLimit - 12)));
            _$setProp(_el$77, "fg", statusCol);
            _$insert(_el$77, elapsed);
            _$insertNode(_el$78, _el$79);
            _$setProp(_el$78, "flexDirection", "row");
            _$insert(_el$79, () => clipped(`     ${shortID(job.job_id)} \xB7 ${job.transport || "direct"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}${job.attempt ? ` \xB7 int #${job.attempt}` : ""}`, props.textLimit));
            _$effect((_p$) => {
              var _v$40 = chip.color, _v$41 = isRunning ? palette.text : palette.textSoft, _v$42 = palette.textMuted;
              _v$40 !== _p$.e && (_p$.e = _$setProp(_el$75, "fg", _v$40, _p$.e));
              _v$41 !== _p$.t && (_p$.t = _$setProp(_el$76, "fg", _v$41, _p$.t));
              _v$42 !== _p$.a && (_p$.a = _$setProp(_el$79, "fg", _v$42, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$72;
          })();
        }
      });
    }
  });
}
function AttentionRows(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.items.length > 0;
    },
    get fallback() {
      return (() => {
        var _el$80 = _$createElement("text");
        _$insertNode(_el$80, _$createTextNode(` \u2713 Sin alertas pendientes`));
        _$effect((_$p) => _$setProp(_el$80, "fg", palette.success, _$p));
        return _el$80;
      })();
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$82 = _$createElement("box"), _el$83 = _$createElement("box"), _el$84 = _$createElement("text"), _el$86 = _$createElement("text"), _el$87 = _$createElement("text");
          _$insertNode(_el$82, _el$83);
          _$insertNode(_el$82, _el$87);
          _$setProp(_el$82, "flexDirection", "column");
          _$insertNode(_el$83, _el$84);
          _$insertNode(_el$83, _el$86);
          _$setProp(_el$83, "flexDirection", "row");
          _$insertNode(_el$84, _$createTextNode(`  \u2715 `));
          _$insert(_el$86, () => clipped(item.title, Math.max(8, props.textLimit - 4)));
          _$insert(_el$87, () => `     ${clipped(item.detail, Math.max(8, props.textLimit - 5))}`);
          _$effect((_p$) => {
            var _v$43 = palette.error, _v$44 = palette.text, _v$45 = palette.textMuted;
            _v$43 !== _p$.e && (_p$.e = _$setProp(_el$84, "fg", _v$43, _p$.e));
            _v$44 !== _p$.t && (_p$.t = _$setProp(_el$86, "fg", _v$44, _p$.t));
            _v$45 !== _p$.a && (_p$.a = _$setProp(_el$87, "fg", _v$45, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$82;
        })()
      });
    }
  });
}
function OperationalBottomDashboard(props) {
  const palette = resolvePalette(props.theme);
  const succeededJobs = createMemo(() => props.jobs.filter((j) => j.status === "succeeded").length);
  const failedJobs = createMemo(() => props.jobs.filter((j) => ["failed", "timed_out", "lost", "cancelled"].includes(j.status)).length);
  const activeJobs = createMemo(() => props.jobs.filter((j) => ["running", "starting", "accepted"].includes(j.status)).length);
  const doneTasks = createMemo(() => props.snapshot.summary.done || 0);
  const totalTasks = createMemo(() => props.snapshot.summary.total_tasks || props.snapshot.tasks.length);
  const totalLeases = createMemo(() => props.snapshot.tasks.reduce((sum, t) => sum + (t.lease_count || 0), 0));
  const blockedTasks = createMemo(() => props.snapshot.summary.blocked || 0);
  const etaLabel = createMemo(() => {
    const eta = props.eta();
    const suffix = eta.backlog > 0 ? ` \xB7 ${eta.backlog} backlog` : "";
    if (props.layout.compact) {
      const shortSuffix = eta.backlog > 0 ? ` +${eta.backlog}b` : "";
      if (eta.remaining === 0) return `\u23F3 listo${shortSuffix}`;
      if (eta.estimateMs === void 0) return `\u23F3 est\u2026 (${eta.remaining})${shortSuffix}`;
      return `\u23F3 ${formatDuration(eta.estimateMs)} (${eta.remaining})${shortSuffix}`;
    }
    if (eta.remaining === 0) return `\u23F3 ETA: tablero completo${suffix}`;
    if (eta.estimateMs === void 0) return `\u23F3 ETA: estimando (${eta.remaining} pend)${suffix}`;
    return `\u23F3 ETA: ${formatDuration(eta.estimateMs)} \xB7 ${eta.remaining} pend${suffix}`;
  });
  const etaColor = createMemo(() => props.eta().remaining === 0 ? palette.success : palette.sky);
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
    if (rate === void 0) return palette.textMuted;
    if (rate >= 90) return palette.success;
    if (rate >= 70) return palette.info;
    if (rate >= 50) return palette.warning;
    return palette.error;
  });
  const hasMetrics = createMemo(() => succeededJobs() > 0 || failedJobs() > 0 || activeJobs() > 0 || totalLeases() > 0 || blockedTasks() > 0);
  return (() => {
    var _el$88 = _$createElement("box"), _el$89 = _$createElement("box"), _el$91 = _$createElement("text"), _el$100 = _$createElement("box"), _el$101 = _$createElement("text"), _el$106 = _$createElement("box"), _el$107 = _$createElement("text"), _el$110 = _$createElement("box");
    _$insertNode(_el$88, _el$89);
    _$insertNode(_el$88, _el$100);
    _$insertNode(_el$88, _el$106);
    _$insertNode(_el$88, _el$110);
    _$setProp(_el$88, "flexDirection", "column");
    _$setProp(_el$88, "marginTop", 1);
    _$setProp(_el$88, "paddingLeft", 1);
    _$setProp(_el$88, "paddingRight", 1);
    _$setProp(_el$88, "borderStyle", "rounded");
    _$setProp(_el$88, "titleAlignment", "left");
    _$insertNode(_el$89, _el$91);
    _$setProp(_el$89, "flexDirection", "row");
    _$insert(_el$89, _$createComponent(Show, {
      get when() {
        return activeJobs() > 0;
      },
      get fallback() {
        return (() => {
          var _el$112 = _$createElement("text");
          _$insert(_el$112, () => `  ${props.pulse()} SYNAPSE: SINCRONIZADO`);
          _$effect((_$p) => _$setProp(_el$112, "fg", palette.success, _$p));
          return _el$112;
        })();
      },
      get children() {
        var _el$90 = _$createElement("text");
        _$insert(_el$90, () => `  ${props.spinner()} SYNAPSE: MOTOR ACTIVO`);
        _$effect((_$p) => _$setProp(_el$90, "fg", palette.warning, _$p));
        return _el$90;
      }
    }), _el$91);
    _$insertNode(_el$91, _$createTextNode(`  [\u{1F310} Web]`));
    _$setProp(_el$91, "onMouseDown", () => openWebConsole(props.snapshot.tasks[0]?.board_id));
    _$setProp(_el$91, "selectable", false);
    _$insert(_el$88, _$createComponent(Show, {
      get when() {
        return hasMetrics();
      },
      get fallback() {
        return (() => {
          var _el$113 = _$createElement("box"), _el$114 = _$createElement("text");
          _$insertNode(_el$113, _el$114);
          _$setProp(_el$113, "flexDirection", "row");
          _$insertNode(_el$114, _$createTextNode(` Standby \xB7 0 ejecuciones`));
          _$effect((_$p) => _$setProp(_el$114, "fg", palette.textMuted, _$p));
          return _el$113;
        })();
      },
      get children() {
        return _$createComponent(Show, {
          get when() {
            return !props.layout.compact;
          },
          get fallback() {
            return (() => {
              var _el$116 = _$createElement("box"), _el$117 = _$createElement("text"), _el$118 = _$createElement("text"), _el$119 = _$createElement("text"), _el$120 = _$createElement("text");
              _$insertNode(_el$116, _el$117);
              _$insertNode(_el$116, _el$118);
              _$insertNode(_el$116, _el$119);
              _$insertNode(_el$116, _el$120);
              _$setProp(_el$116, "flexDirection", "row");
              _$insert(_el$117, () => `\u2713${succeededJobs()} `);
              _$insert(_el$118, () => `\u2715${failedJobs()} `);
              _$insert(_el$119, () => `\u25CF${activeJobs()} `);
              _$insert(_el$120, () => `\u{1F6E1}${totalLeases()}`);
              _$insert(_el$116, _$createComponent(Show, {
                get when() {
                  return blockedTasks() > 0;
                },
                get children() {
                  var _el$121 = _$createElement("text");
                  _$insert(_el$121, () => ` \u2715${blockedTasks()}b`);
                  _$effect((_$p) => _$setProp(_el$121, "fg", palette.error, _$p));
                  return _el$121;
                }
              }), null);
              _$effect((_p$) => {
                var _v$57 = succeededJobs() > 0 ? palette.success : palette.textMuted, _v$58 = failedJobs() > 0 ? palette.error : palette.textMuted, _v$59 = activeJobs() > 0 ? palette.warning : palette.textMuted, _v$60 = totalLeases() > 0 ? palette.sky : palette.textMuted;
                _v$57 !== _p$.e && (_p$.e = _$setProp(_el$117, "fg", _v$57, _p$.e));
                _v$58 !== _p$.t && (_p$.t = _$setProp(_el$118, "fg", _v$58, _p$.t));
                _v$59 !== _p$.a && (_p$.a = _$setProp(_el$119, "fg", _v$59, _p$.a));
                _v$60 !== _p$.o && (_p$.o = _$setProp(_el$120, "fg", _v$60, _p$.o));
                return _p$;
              }, {
                e: void 0,
                t: void 0,
                a: void 0,
                o: void 0
              });
              return _el$116;
            })();
          },
          get children() {
            return [(() => {
              var _el$93 = _$createElement("box"), _el$94 = _$createElement("text"), _el$95 = _$createElement("text");
              _$insertNode(_el$93, _el$94);
              _$insertNode(_el$93, _el$95);
              _$setProp(_el$93, "flexDirection", "row");
              _$insert(_el$94, () => `[ \u2713 ${succeededJobs()} \xC9XITO ] `);
              _$insert(_el$95, () => `[ \u2715 ${failedJobs()} FALLO ]`);
              _$effect((_p$) => {
                var _v$46 = palette.success, _v$47 = failedJobs() > 0 ? palette.error : palette.textMuted;
                _v$46 !== _p$.e && (_p$.e = _$setProp(_el$94, "fg", _v$46, _p$.e));
                _v$47 !== _p$.t && (_p$.t = _$setProp(_el$95, "fg", _v$47, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$93;
            })(), (() => {
              var _el$96 = _$createElement("box"), _el$97 = _$createElement("text"), _el$98 = _$createElement("text");
              _$insertNode(_el$96, _el$97);
              _$insertNode(_el$96, _el$98);
              _$setProp(_el$96, "flexDirection", "row");
              _$insert(_el$97, () => `[ ${activeJobs() > 0 ? props.spinner() : "\u25CF"} ${activeJobs()} CURSO ] `);
              _$insert(_el$98, () => `[ \u{1F6E1} ${totalLeases()} LOCKS ] `);
              _$insert(_el$96, _$createComponent(Show, {
                get when() {
                  return blockedTasks() > 0;
                },
                get children() {
                  var _el$99 = _$createElement("text");
                  _$insert(_el$99, () => `[ \u2715 ${blockedTasks()} BLCK ]`);
                  _$effect((_$p) => _$setProp(_el$99, "fg", palette.error, _$p));
                  return _el$99;
                }
              }), null);
              _$effect((_p$) => {
                var _v$48 = activeJobs() > 0 ? palette.warning : palette.textMuted, _v$49 = totalLeases() > 0 ? palette.sky : palette.textMuted;
                _v$48 !== _p$.e && (_p$.e = _$setProp(_el$97, "fg", _v$48, _p$.e));
                _v$49 !== _p$.t && (_p$.t = _$setProp(_el$98, "fg", _v$49, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$96;
            })()];
          }
        });
      }
    }), _el$100);
    _$insertNode(_el$100, _el$101);
    _$setProp(_el$100, "flexDirection", "row");
    _$insertNode(_el$101, _$createTextNode(`Salud: `));
    _$insert(_el$100, _$createComponent(Show, {
      get when() {
        return successRate() !== void 0;
      },
      get fallback() {
        return (() => {
          var _el$122 = _$createElement("text");
          _$insertNode(_el$122, _$createTextNode(`\u25CB standby`));
          _$effect((_$p) => _$setProp(_el$122, "fg", palette.textMuted, _$p));
          return _el$122;
        })();
      },
      get children() {
        return [(() => {
          var _el$103 = _$createElement("text");
          _$insert(_el$103, () => healthBars().filled);
          _$effect((_$p) => _$setProp(_el$103, "fg", healthColor(), _$p));
          return _el$103;
        })(), (() => {
          var _el$104 = _$createElement("text");
          _$insert(_el$104, () => healthBars().empty);
          _$effect((_$p) => _$setProp(_el$104, "fg", palette.border, _$p));
          return _el$104;
        })(), (() => {
          var _el$105 = _$createElement("text");
          _$insert(_el$105, () => ` ${successRate()}%`);
          _$effect((_$p) => _$setProp(_el$105, "fg", healthColor(), _$p));
          return _el$105;
        })()];
      }
    }), null);
    _$insertNode(_el$106, _el$107);
    _$setProp(_el$106, "flexDirection", "row");
    _$insert(_el$107, (() => {
      var _c$2 = _$memo(() => !!props.layout.compact);
      return () => _c$2() ? "Autoridad: SQLite" : `\u{1F4CB} DAG: ${doneTasks()}/${totalTasks()} \xB7 Autoridad: SQLite`;
    })());
    _$insert(_el$88, _$createComponent(Show, {
      get when() {
        return totalTasks() > 0;
      },
      get children() {
        var _el$108 = _$createElement("box"), _el$109 = _$createElement("text");
        _$insertNode(_el$108, _el$109);
        _$setProp(_el$108, "flexDirection", "row");
        _$insert(_el$109, etaLabel);
        _$effect((_$p) => _$setProp(_el$109, "fg", etaColor(), _$p));
        return _el$108;
      }
    }), _el$110);
    _$setProp(_el$110, "flexDirection", "row");
    _$insert(_el$110, _$createComponent(Show, {
      get when() {
        return props.stale;
      },
      get fallback() {
        return (() => {
          var _el$124 = _$createElement("text");
          _$insert(_el$124, (() => {
            var _c$4 = _$memo(() => !!props.layout.compact);
            return () => _c$4() ? "\u{1F7E2} En vivo" : `\u{1F7E2} En vivo \xB7 Sync hace ${syncAgeSec()}s`;
          })());
          _$effect((_$p) => _$setProp(_el$124, "fg", palette.success, _$p));
          return _el$124;
        })();
      },
      get children() {
        var _el$111 = _$createElement("text");
        _$insert(_el$111, (() => {
          var _c$3 = _$memo(() => !!props.layout.compact);
          return () => _c$3() ? "\u{1F7E1} Desfasado" : `\u{1F7E1} Snapshot desfasado (+${syncAgeSec()}s)`;
        })());
        _$effect((_$p) => _$setProp(_el$111, "fg", palette.warning, _$p));
        return _el$111;
      }
    }));
    _$effect((_p$) => {
      var _v$50 = failedJobs() > 0 ? palette.error : palette.primary, _v$51 = props.layout.compact ? "\u{1F9E0} CONTROL" : "\u{1F9E0} CONTROL MATRIX \u25C8", _v$52 = palette.accent, _v$53 = palette.panel, _v$54 = palette.info, _v$55 = palette.textMuted, _v$56 = palette.textMuted;
      _v$50 !== _p$.e && (_p$.e = _$setProp(_el$88, "borderColor", _v$50, _p$.e));
      _v$51 !== _p$.t && (_p$.t = _$setProp(_el$88, "title", _v$51, _p$.t));
      _v$52 !== _p$.a && (_p$.a = _$setProp(_el$88, "titleColor", _v$52, _p$.a));
      _v$53 !== _p$.o && (_p$.o = _$setProp(_el$88, "backgroundColor", _v$53, _p$.o));
      _v$54 !== _p$.i && (_p$.i = _$setProp(_el$91, "fg", _v$54, _p$.i));
      _v$55 !== _p$.n && (_p$.n = _$setProp(_el$101, "fg", _v$55, _p$.n));
      _v$56 !== _p$.s && (_p$.s = _$setProp(_el$107, "fg", _v$56, _p$.s));
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
    return _el$88;
  })();
}
function SidebarStatus(props) {
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
    var _el$125 = _$createElement("box"), _el$132 = _$createElement("box"), _el$133 = _$createElement("text");
    _$insertNode(_el$125, _el$132);
    _$use((node) => setRootWidth(Math.max(0, node.width || 0)), _el$125);
    _$setProp(_el$125, "flexDirection", "column");
    _$setProp(_el$125, "onSizeChange", function() {
      setRootWidth(Math.max(0, this.width || 0));
    });
    _$insert(_el$125, _$createComponent(CortexCockpitHeader, {
      isExecuting,
      get nativeActivity() {
        return props.nativeActivity;
      },
      get projectRoot() {
        return props.snapshot().project_root;
      },
      get sessionElapsed() {
        return props.sessionElapsed;
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
    }), _el$132);
    _$insert(_el$125, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$126 = _$createElement("text");
        _$insertNode(_el$126, _$createTextNode(`Conversaci\xF3n no disponible \xB7 esperando metadatos`));
        _$setProp(_el$126, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$126, "fg", palette.warning, _$p));
        return _el$126;
      }
    }), _el$132);
    _$insert(_el$125, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$128 = _$createElement("text");
        _$insertNode(_el$128, _$createTextNode(`Cargando estado de la conversaci\xF3n\u2026`));
        _$setProp(_el$128, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$128, "fg", palette.textMuted, _$p));
        return _el$128;
      }
    }), _el$132);
    _$insert(_el$125, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$130 = _$createElement("text");
        _$insertNode(_el$130, _$createTextNode(`No se pudo actualizar \xB7 datos no confirmados`));
        _$setProp(_el$130, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$130, "fg", palette.error, _$p));
        return _el$130;
      }
    }), _el$132);
    _$insertNode(_el$132, _el$133);
    _$setProp(_el$132, "flexDirection", "row");
    _$setProp(_el$132, "marginTop", 0);
    _$insert(_el$133, (() => {
      var _c$5 = _$memo(() => props.nativeActivity() === "busy");
      return () => _c$5() ? `${props.spinner()} OpenCode: ocupado` : _$memo(() => props.nativeActivity() === "idle")() ? "\u25CF OpenCode: en espera" : `\u25CB OpenCode: ${props.nativeActivity() || "sin estado"}`;
    })());
    _$insert(_el$125, _$createComponent(Show, {
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
          get eta() {
            return props.eta;
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
    _$effect((_$p) => _$setProp(_el$133, "fg", props.nativeActivity() === "busy" ? palette.warning : props.nativeActivity() === "idle" ? palette.success : palette.textMuted, _$p));
    return _el$125;
  })();
}
function SidebarFooterMetrics(props) {
  const dimensions = useTerminalDimensions();
  const contextPct = createMemo(() => {
    const metrics = props.metrics();
    if (metrics.tokensUsed === void 0 || metrics.tokenLimit === void 0) return void 0;
    return Math.min(100, Math.max(0, Math.round(metrics.tokensUsed / metrics.tokenLimit * 100)));
  });
  const gaugeSegments = createMemo(() => dimensions().width < 96 ? 6 : 10);
  const contextGauge = createMemo(() => {
    const pct = contextPct();
    if (pct === void 0) return void 0;
    const segments = gaugeSegments();
    const filled = Math.max(0, Math.min(segments, Math.round(pct / 100 * segments)));
    return {
      filled: "\u2588".repeat(filled),
      empty: "\u2591".repeat(segments - filled)
    };
  });
  const palette = resolvePalette(props.theme);
  const contextColor = createMemo(() => {
    const pct = contextPct();
    if (pct === void 0) return palette.textMuted;
    if (pct >= 85) return palette.error;
    if (pct >= 60) return palette.warning;
    return palette.success;
  });
  const visible = createMemo(() => {
    const metrics = props.metrics();
    return Boolean(metrics.elapsed) || metrics.tokensUsed !== void 0 || metrics.cost !== void 0;
  });
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$134 = _$createElement("box");
      _$setProp(_el$134, "flexDirection", "row");
      _$setProp(_el$134, "paddingLeft", 1);
      _$setProp(_el$134, "paddingRight", 1);
      _$insert(_el$134, _$createComponent(Show, {
        get when() {
          return props.metrics().elapsed;
        },
        children: (elapsed) => (() => {
          var _el$140 = _$createElement("box"), _el$141 = _$createElement("text"), _el$143 = _$createElement("text");
          _$insertNode(_el$140, _el$141);
          _$insertNode(_el$140, _el$143);
          _$setProp(_el$140, "flexDirection", "row");
          _$insertNode(_el$141, _$createTextNode(`\u23F1 `));
          _$insert(_el$143, elapsed);
          _$effect((_p$) => {
            var _v$61 = palette.info, _v$62 = palette.sky;
            _v$61 !== _p$.e && (_p$.e = _$setProp(_el$141, "fg", _v$61, _p$.e));
            _v$62 !== _p$.t && (_p$.t = _$setProp(_el$143, "fg", _v$62, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$140;
        })()
      }), null);
      _$insert(_el$134, _$createComponent(Show, {
        get when() {
          return props.metrics().tokensUsed !== void 0;
        },
        get children() {
          return [(() => {
            var _el$135 = _$createElement("text");
            _$insertNode(_el$135, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$135, "fg", palette.border, _$p));
            return _el$135;
          })(), _$createComponent(Show, {
            get when() {
              return contextGauge();
            },
            get fallback() {
              return (() => {
                var _el$144 = _$createElement("text");
                _$insert(_el$144, () => `\u25C6 ${formatTokens(props.metrics().tokensUsed)} tok`);
                _$effect((_$p) => _$setProp(_el$144, "fg", palette.sky, _$p));
                return _el$144;
              })();
            },
            children: (gauge) => (() => {
              var _el$145 = _$createElement("box"), _el$146 = _$createElement("text"), _el$148 = _$createElement("text"), _el$149 = _$createElement("text"), _el$150 = _$createElement("text"), _el$151 = _$createElement("text"), _el$153 = _$createElement("text");
              _$insertNode(_el$145, _el$146);
              _$insertNode(_el$145, _el$148);
              _$insertNode(_el$145, _el$149);
              _$insertNode(_el$145, _el$150);
              _$insertNode(_el$145, _el$151);
              _$insertNode(_el$145, _el$153);
              _$setProp(_el$145, "flexDirection", "row");
              _$insertNode(_el$146, _$createTextNode(`\u25C6 `));
              _$insert(_el$148, () => gauge().filled);
              _$insert(_el$149, () => gauge().empty);
              _$insert(_el$150, () => ` ${contextPct()}%`);
              _$insertNode(_el$151, _$createTextNode(` \xB7 `));
              _$insert(_el$153, () => formatTokens(props.metrics().tokensUsed));
              _$effect((_p$) => {
                var _v$63 = contextColor(), _v$64 = contextColor(), _v$65 = palette.border, _v$66 = contextColor(), _v$67 = palette.border, _v$68 = palette.sky;
                _v$63 !== _p$.e && (_p$.e = _$setProp(_el$146, "fg", _v$63, _p$.e));
                _v$64 !== _p$.t && (_p$.t = _$setProp(_el$148, "fg", _v$64, _p$.t));
                _v$65 !== _p$.a && (_p$.a = _$setProp(_el$149, "fg", _v$65, _p$.a));
                _v$66 !== _p$.o && (_p$.o = _$setProp(_el$150, "fg", _v$66, _p$.o));
                _v$67 !== _p$.i && (_p$.i = _$setProp(_el$151, "fg", _v$67, _p$.i));
                _v$68 !== _p$.n && (_p$.n = _$setProp(_el$153, "fg", _v$68, _p$.n));
                return _p$;
              }, {
                e: void 0,
                t: void 0,
                a: void 0,
                o: void 0,
                i: void 0,
                n: void 0
              });
              return _el$145;
            })()
          })];
        }
      }), null);
      _$insert(_el$134, _$createComponent(Show, {
        get when() {
          return props.metrics().cost !== void 0;
        },
        get children() {
          return [(() => {
            var _el$137 = _$createElement("text");
            _$insertNode(_el$137, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$137, "fg", palette.border, _$p));
            return _el$137;
          })(), (() => {
            var _el$139 = _$createElement("text");
            _$insert(_el$139, () => `$ ${formatCost(props.metrics().cost)}`);
            _$effect((_$p) => _$setProp(_el$139, "fg", palette.warning, _$p));
            return _el$139;
          })()];
        }
      }), null);
      return _el$134;
    }
  });
}
function HomeBottomStatus(props) {
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0);
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$154 = _$createElement("box"), _el$155 = _$createElement("text"), _el$157 = _$createElement("text"), _el$159 = _$createElement("text"), _el$161 = _$createElement("text"), _el$163 = _$createElement("text");
      _$insertNode(_el$154, _el$155);
      _$insertNode(_el$154, _el$157);
      _$insertNode(_el$154, _el$159);
      _$insertNode(_el$154, _el$161);
      _$insertNode(_el$154, _el$163);
      _$setProp(_el$154, "paddingLeft", 1);
      _$setProp(_el$154, "paddingRight", 1);
      _$setProp(_el$154, "flexDirection", "row");
      _$insertNode(_el$155, _$createTextNode(`\u{1F9E0} `));
      _$insertNode(_el$157, _$createTextNode(`CORTEX`));
      _$insertNode(_el$159, _$createTextNode(`\xB7`));
      _$insertNode(_el$161, _$createTextNode(`IA `));
      _$insertNode(_el$163, _$createTextNode(`\u2502 `));
      _$insert(_el$154, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$165 = _$createElement("box"), _el$166 = _$createElement("text"), _el$167 = _$createElement("text"), _el$169 = _$createElement("text");
            _$insertNode(_el$165, _el$166);
            _$insertNode(_el$165, _el$167);
            _$insertNode(_el$165, _el$169);
            _$setProp(_el$165, "flexDirection", "row");
            _$insert(_el$166, () => `\u25CF ${counts().active} en curso`);
            _$insertNode(_el$167, _$createTextNode(` \xB7 `));
            _$insert(_el$169, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$165, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$170 = _$createElement("text");
                  _$insertNode(_el$170, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$170, "fg", palette.border, _$p));
                  return _el$170;
                })(), (() => {
                  var _el$172 = _$createElement("text");
                  _$insert(_el$172, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$172, "fg", palette.error, _$p));
                  return _el$172;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$74 = palette.warning, _v$75 = palette.border, _v$76 = palette.accent;
              _v$74 !== _p$.e && (_p$.e = _$setProp(_el$166, "fg", _v$74, _p$.e));
              _v$75 !== _p$.t && (_p$.t = _$setProp(_el$167, "fg", _v$75, _p$.t));
              _v$76 !== _p$.a && (_p$.a = _$setProp(_el$169, "fg", _v$76, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$165;
          })();
        },
        children: (task) => (() => {
          var _el$173 = _$createElement("box"), _el$174 = _$createElement("text"), _el$175 = _$createElement("text"), _el$176 = _$createElement("text"), _el$177 = _$createElement("text");
          _$insertNode(_el$173, _el$174);
          _$insertNode(_el$173, _el$175);
          _$insertNode(_el$173, _el$176);
          _$insertNode(_el$173, _el$177);
          _$setProp(_el$173, "flexDirection", "row");
          _$setProp(_el$173, "onMouseDown", () => openWebConsole(task().board_id, task().task_id));
          _$insert(_el$174, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$175, () => clipped(task().title, 20));
          _$insert(_el$176, () => ` \xB7 ${counts().active} activos`);
          _$insertNode(_el$177, _$createTextNode(` [\u{1F310}]`));
          _$effect((_p$) => {
            var _v$77 = palette.warning, _v$78 = palette.text, _v$79 = palette.textMuted, _v$80 = palette.info;
            _v$77 !== _p$.e && (_p$.e = _$setProp(_el$174, "fg", _v$77, _p$.e));
            _v$78 !== _p$.t && (_p$.t = _$setProp(_el$175, "fg", _v$78, _p$.t));
            _v$79 !== _p$.a && (_p$.a = _$setProp(_el$176, "fg", _v$79, _p$.a));
            _v$80 !== _p$.o && (_p$.o = _$setProp(_el$177, "fg", _v$80, _p$.o));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0,
            o: void 0
          });
          return _el$173;
        })()
      }), null);
      _$effect((_p$) => {
        var _v$69 = palette.accentAlt, _v$70 = palette.text, _v$71 = palette.info, _v$72 = palette.sky, _v$73 = palette.border;
        _v$69 !== _p$.e && (_p$.e = _$setProp(_el$155, "fg", _v$69, _p$.e));
        _v$70 !== _p$.t && (_p$.t = _$setProp(_el$157, "fg", _v$70, _p$.t));
        _v$71 !== _p$.a && (_p$.a = _$setProp(_el$159, "fg", _v$71, _p$.a));
        _v$72 !== _p$.o && (_p$.o = _$setProp(_el$161, "fg", _v$72, _p$.o));
        _v$73 !== _p$.i && (_p$.i = _$setProp(_el$163, "fg", _v$73, _p$.i));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0
      });
      return _el$154;
    }
  });
}
function SessionKanbanPanel(props) {
  const tasks = createMemo(() => props.snapshot().tasks);
  const readyTasks = createMemo(() => tasks().filter((t) => t.status === "ready"));
  const inProgressTasks = createMemo(() => tasks().filter((t) => t.status === "in_progress"));
  const inReviewTasks = createMemo(() => tasks().filter((t) => t.status === "in_review"));
  const blockedTasks = createMemo(() => tasks().filter((t) => t.status === "blocked"));
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$179 = _$createElement("box"), _el$180 = _$createElement("box"), _el$181 = _$createElement("text"), _el$182 = _$createElement("text"), _el$183 = _$createElement("box"), _el$184 = _$createElement("box"), _el$185 = _$createElement("text"), _el$186 = _$createElement("box"), _el$187 = _$createElement("text"), _el$188 = _$createElement("box"), _el$189 = _$createElement("text");
    _$insertNode(_el$179, _el$180);
    _$insertNode(_el$179, _el$183);
    _$setProp(_el$179, "flexDirection", "column");
    _$setProp(_el$179, "padding", 1);
    _$insertNode(_el$180, _el$181);
    _$insertNode(_el$180, _el$182);
    _$setProp(_el$180, "flexDirection", "row");
    _$setProp(_el$180, "marginBottom", 1);
    _$setProp(_el$181, "bold", true);
    _$insert(_el$181, () => `\u25C8 CORTEX \xB7 IA KANBAN DECK [${props.pulse()}] `);
    _$insert(_el$182, () => `(${tasks().length} tareas \xB7 ${props.jobs().length} workers)`);
    _$insertNode(_el$183, _el$184);
    _$insertNode(_el$183, _el$186);
    _$insertNode(_el$183, _el$188);
    _$setProp(_el$183, "flexDirection", "row");
    _$insertNode(_el$184, _el$185);
    _$setProp(_el$184, "flexDirection", "column");
    _$setProp(_el$184, "width", 26);
    _$setProp(_el$184, "marginRight", 1);
    _$setProp(_el$185, "bold", true);
    _$insert(_el$185, () => `\u26A1 EN CURSO (${inProgressTasks().length})`);
    _$insert(_el$184, _$createComponent(For, {
      get each() {
        return inProgressTasks();
      },
      children: (task) => (() => {
        var _el$190 = _$createElement("box"), _el$191 = _$createElement("text"), _el$192 = _$createElement("text");
        _$insertNode(_el$190, _el$191);
        _$insertNode(_el$190, _el$192);
        _$setProp(_el$190, "flexDirection", "column");
        _$setProp(_el$190, "marginTop", 1);
        _$setProp(_el$191, "bold", true);
        _$insert(_el$191, () => `\u25CF ${task.task_id}`);
        _$insert(_el$192, () => clipped(task.title, 22));
        _$insert(_el$190, _$createComponent(Show, {
          get when() {
            return task.owner;
          },
          get children() {
            var _el$193 = _$createElement("text");
            _$insert(_el$193, () => `Claim: ${clipped(task.owner, 14)}`);
            _$effect((_$p) => _$setProp(_el$193, "fg", palette.info, _$p));
            return _el$193;
          }
        }), null);
        _$effect((_p$) => {
          var _v$86 = palette.text, _v$87 = palette.textSoft;
          _v$86 !== _p$.e && (_p$.e = _$setProp(_el$191, "fg", _v$86, _p$.e));
          _v$87 !== _p$.t && (_p$.t = _$setProp(_el$192, "fg", _v$87, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$190;
      })()
    }), null);
    _$insertNode(_el$186, _el$187);
    _$setProp(_el$186, "flexDirection", "column");
    _$setProp(_el$186, "width", 26);
    _$setProp(_el$186, "marginRight", 1);
    _$setProp(_el$187, "bold", true);
    _$insert(_el$187, () => `\u2696 EN REVISI\xD3N (${inReviewTasks().length})`);
    _$insert(_el$186, _$createComponent(For, {
      get each() {
        return inReviewTasks();
      },
      children: (task) => (() => {
        var _el$194 = _$createElement("box"), _el$195 = _$createElement("text"), _el$196 = _$createElement("text"), _el$197 = _$createElement("text");
        _$insertNode(_el$194, _el$195);
        _$insertNode(_el$194, _el$196);
        _$insertNode(_el$194, _el$197);
        _$setProp(_el$194, "flexDirection", "column");
        _$setProp(_el$194, "marginTop", 1);
        _$setProp(_el$195, "bold", true);
        _$insert(_el$195, () => `\u25C6 ${task.task_id}`);
        _$insert(_el$196, () => clipped(task.title, 22));
        _$insertNode(_el$197, _$createTextNode(`esperando reviewer`));
        _$effect((_p$) => {
          var _v$88 = palette.text, _v$89 = palette.textSoft, _v$90 = palette.warning;
          _v$88 !== _p$.e && (_p$.e = _$setProp(_el$195, "fg", _v$88, _p$.e));
          _v$89 !== _p$.t && (_p$.t = _$setProp(_el$196, "fg", _v$89, _p$.t));
          _v$90 !== _p$.a && (_p$.a = _$setProp(_el$197, "fg", _v$90, _p$.a));
          return _p$;
        }, {
          e: void 0,
          t: void 0,
          a: void 0
        });
        return _el$194;
      })()
    }), null);
    _$insertNode(_el$188, _el$189);
    _$setProp(_el$188, "flexDirection", "column");
    _$setProp(_el$188, "width", 26);
    _$setProp(_el$189, "bold", true);
    _$insert(_el$189, () => `\u2713 LISTAS (${readyTasks().length}) / \u2715 BLQ (${blockedTasks().length})`);
    _$insert(_el$188, _$createComponent(For, {
      get each() {
        return blockedTasks();
      },
      children: (task) => (() => {
        var _el$199 = _$createElement("box"), _el$200 = _$createElement("text"), _el$201 = _$createElement("text");
        _$insertNode(_el$199, _el$200);
        _$insertNode(_el$199, _el$201);
        _$setProp(_el$199, "flexDirection", "column");
        _$setProp(_el$199, "marginTop", 1);
        _$setProp(_el$200, "bold", true);
        _$insert(_el$200, () => `\u2715 ${task.task_id}`);
        _$insert(_el$201, () => clipped(task.title, 22));
        _$effect((_p$) => {
          var _v$91 = palette.error, _v$92 = palette.error;
          _v$91 !== _p$.e && (_p$.e = _$setProp(_el$200, "fg", _v$91, _p$.e));
          _v$92 !== _p$.t && (_p$.t = _$setProp(_el$201, "fg", _v$92, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$199;
      })()
    }), null);
    _$insert(_el$188, _$createComponent(For, {
      get each() {
        return readyTasks().slice(0, 3);
      },
      children: (task) => (() => {
        var _el$202 = _$createElement("box"), _el$203 = _$createElement("text"), _el$204 = _$createElement("text");
        _$insertNode(_el$202, _el$203);
        _$insertNode(_el$202, _el$204);
        _$setProp(_el$202, "flexDirection", "column");
        _$setProp(_el$202, "marginTop", 1);
        _$setProp(_el$203, "bold", true);
        _$insert(_el$203, () => `\u25CB ${task.task_id}`);
        _$insert(_el$204, () => clipped(task.title, 22));
        _$effect((_p$) => {
          var _v$93 = palette.sky, _v$94 = palette.textMuted;
          _v$93 !== _p$.e && (_p$.e = _$setProp(_el$203, "fg", _v$93, _p$.e));
          _v$94 !== _p$.t && (_p$.t = _$setProp(_el$204, "fg", _v$94, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$202;
      })()
    }), null);
    _$effect((_p$) => {
      var _v$81 = palette.info, _v$82 = palette.textMuted, _v$83 = palette.warning, _v$84 = palette.accent, _v$85 = palette.success;
      _v$81 !== _p$.e && (_p$.e = _$setProp(_el$181, "fg", _v$81, _p$.e));
      _v$82 !== _p$.t && (_p$.t = _$setProp(_el$182, "fg", _v$82, _p$.t));
      _v$83 !== _p$.a && (_p$.a = _$setProp(_el$185, "fg", _v$83, _p$.a));
      _v$84 !== _p$.o && (_p$.o = _$setProp(_el$187, "fg", _v$84, _p$.o));
      _v$85 !== _p$.i && (_p$.i = _$setProp(_el$189, "fg", _v$85, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
    });
    return _el$179;
  })();
}
var CORTEX_LOGO_BRAILLE = ["       \u28E0\u28F6\u28FF\u28FF\u28FF\u28FF\u28F6\u28E4\u2840       \u2880\u28E4\u28F6\u28FF\u28FF\u28FF\u28FF\u28F6\u28C4", "    \u28B0\u28FF\u28FF\u281F\u2809   \u2819\u28BF\u28FF\u28F7\u2840   \u28A0\u28FE\u28FF\u287F\u280B   \u2808\u283B\u28FF\u28FF\u2846", "   \u28A0\u28FF\u28FF\u280B  \u2880\u28E4\u28E4\u28C0  \u2839\u28FF\u28FF\u28C4\u28E0\u28FF\u28FF\u280F  \u28C0\u28E4\u28E4\u2840  \u2819\u28FF\u28FF\u2844", "   \u28FE\u28FF\u2803  \u28B0\u28FF\u28FF\u28FF\u28FF\u28F7\u2840 \u2839\u28FF\u28FF\u28FF\u28FF\u280F \u28A0\u28FE\u28FF\u28FF\u28FF\u28FF\u2846  \u2818\u28FF\u28F7", "  \u28B8\u28FF\u285F   \u2838\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u28C6 \u2839\u28FF\u28FF\u280F \u28F0\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u2807   \u28BB\u28FF\u2847", "  \u2818\u28FF\u28E7    \u2808\u281B\u283F\u28FF\u28FF\u28FF\u28FF\u28F7\u28C4\u2819\u280B\u28E0\u28FE\u28FF\u28FF\u28FF\u28FF\u283F\u281B\u2801    \u28FC\u28FF\u2803", "   \u2839\u28FF\u28E7\u2840     \u2808\u2819\u283F\u28FF\u28FF\u28FF\u2846\u28B0\u28FF\u28FF\u28FF\u283F\u280B\u2801     \u2880\u28FC\u28FF\u280F", "    \u2819\u28BF\u28FF\u28E6\u2840   \u2880\u28E0\u28F4\u28FF\u28FF\u28FF\u2847\u28B8\u28FF\u28FF\u28FF\u28E6\u28C4\u2840   \u2880\u28F4\u28FF\u287F\u280B", "      \u2809\u281B\u283F\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u287F\u281B\u2801 \u2808\u281B\u28BF\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u283F\u281B\u2809", "  \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2557  \u2588\u2588\u2557     \u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2557 ", " \u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255D\u2588\u2588\u2554\u2550\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u255A\u2550\u2550\u2588\u2588\u2554\u2550\u2550\u255D\u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255D\u255A\u2588\u2588\u2557\u2588\u2588\u2554\u255D     \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557", " \u2588\u2588\u2551     \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255D   \u2588\u2588\u2551   \u2588\u2588\u2588\u2588\u2588\u2557   \u255A\u2588\u2588\u2588\u2554\u255D\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2551", " \u2588\u2588\u2551     \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557   \u2588\u2588\u2551   \u2588\u2588\u2554\u2550\u2550\u255D   \u2588\u2588\u2554\u2588\u2588\u2557\u255A\u2550\u2550\u2550\u2550\u255D\u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2551", " \u255A\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u255A\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255D\u2588\u2588\u2551  \u2588\u2588\u2551   \u2588\u2588\u2551   \u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2554\u255D \u2588\u2588\u2557     \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551", "  \u255A\u2550\u2550\u2550\u2550\u2550\u255D \u255A\u2550\u2550\u2550\u2550\u2550\u255D \u255A\u2550\u255D  \u255A\u2550\u255D   \u255A\u2550\u255D   \u255A\u2550\u2550\u2550\u2550\u2550\u2550\u255D\u255A\u2550\u255D  \u255A\u2550\u255D     \u255A\u2550\u255D\u255A\u2550\u255D  \u255A\u2550\u255D"];
function HomeLogo(props) {
  const dim = useTerminalDimensions();
  const palette = resolvePalette(props.theme);
  const isLarge = createMemo(() => {
    const d = dim();
    return d.height >= CORTEX_LOGO_BRAILLE.length + 5 && d.width >= 72;
  });
  return (() => {
    var _el$205 = _$createElement("box");
    _$setProp(_el$205, "flexDirection", "column");
    _$setProp(_el$205, "alignItems", "center");
    _$setProp(_el$205, "marginBottom", 1);
    _$insert(_el$205, _$createComponent(Show, {
      get when() {
        return isLarge();
      },
      get fallback() {
        return (() => {
          var _el$208 = _$createElement("box"), _el$209 = _$createElement("text"), _el$211 = _$createElement("text");
          _$insertNode(_el$208, _el$209);
          _$insertNode(_el$208, _el$211);
          _$setProp(_el$208, "flexDirection", "column");
          _$setProp(_el$208, "alignItems", "center");
          _$insertNode(_el$209, _$createTextNode(`\u25C8 CORTEX \xB7 IA \u25C8`));
          _$setProp(_el$209, "bold", true);
          _$insertNode(_el$211, _$createTextNode(`[Adaptive Cognitive Control Plane]`));
          _$effect((_p$) => {
            var _v$95 = palette.info, _v$96 = palette.textMuted;
            _v$95 !== _p$.e && (_p$.e = _$setProp(_el$209, "fg", _v$95, _p$.e));
            _v$96 !== _p$.t && (_p$.t = _$setProp(_el$211, "fg", _v$96, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$208;
        })();
      },
      get children() {
        return [_$createComponent(For, {
          each: CORTEX_LOGO_BRAILLE,
          children: (line, index) => {
            const color = index() < 4 ? palette.accentAlt : index() < 9 ? palette.info : index() < 12 ? palette.sky : palette.accent;
            return (() => {
              var _el$213 = _$createElement("text");
              _$setProp(_el$213, "fg", color);
              _$insert(_el$213, line);
              return _el$213;
            })();
          }
        }), (() => {
          var _el$206 = _$createElement("text");
          _$insertNode(_el$206, _$createTextNode(`\u26A1 OpenCode Multi-Agent Control Plane &amp; Task DAG \u26A1`));
          _$setProp(_el$206, "marginTop", 1);
          _$effect((_$p) => _$setProp(_el$206, "fg", palette.textMuted, _$p));
          return _el$206;
        })()];
      }
    }));
    return _el$205;
  })();
}
function initialize(api, disposeRoot) {
  const [activeSessionOverride, setActiveSessionOverride] = createSignal();
  const updateActiveSession = (val) => {
    const id = extractSessionID(val);
    if (id && id !== activeSessionOverride()) {
      setActiveSessionOverride(id);
    }
  };
  const nativeActivity = createMemo(() => nativeSessionActivity(api, activeSessionOverride()));
  const scopeReady = createMemo(() => Boolean(conversationScope(api, activeSessionOverride())?.project));
  const [snapshot, setSnapshot] = createSignal(EMPTY_SNAPSHOT);
  const [snapshotError, setSnapshotError] = createSignal("");
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [pulseFrame, setPulseFrame] = createSignal(0);
  const getPref = (key, fallback) => {
    if (api.kv && typeof api.kv.get === "function") {
      return api.kv.get(key, fallback) !== false;
    }
    return fallback;
  };
  const setPref = (key, val) => {
    if (api.kv && typeof api.kv.set === "function") {
      api.kv.set(key, val);
    }
  };
  const [tasksExpanded, setTasksExpanded] = createSignal(getPref(TASKS_EXPANDED_KEY, true));
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(getPref(DELEGATIONS_EXPANDED_KEY, true));
  const [attentionExpanded, setAttentionExpanded] = createSignal(getPref(ATTENTION_EXPANDED_KEY, true));
  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const pulse = createMemo(() => NEURAL_PULSE_FRAMES[pulseFrame() % NEURAL_PULSE_FRAMES.length]);
  const jobs = createMemo(() => snapshot().delegations.map((job, sequence) => ({
    ...job,
    sequence
  })));
  const sessionElapsed = createMemo(() => {
    const scope = conversationScope(api, activeSessionOverride());
    const started = sessionStartTime(api, scope?.rootSessionID) ?? sessionStartTime(api, scope?.sessionID);
    if (started === void 0) return void 0;
    const diff = now() - started;
    return diff > 0 ? formatDuration(diff) : void 0;
  });
  const sidebarMetrics = createMemo(() => {
    const scope = conversationScope(api, activeSessionOverride());
    const sessionID = scope?.sessionID;
    return {
      elapsed: sessionElapsed(),
      tokensUsed: contextTokensUsed(sessionMessages(api, sessionID)),
      tokenLimit: sessionContextLimit(api, sessionID),
      cost: sessionCostUsd(api, sessionID)
    };
  });
  const [completionHistory, setCompletionHistory] = createSignal(loadEtaHistory(api.kv && typeof api.kv.get === "function" ? api.kv.get(ETA_HISTORY_KEY, []) : []));
  let previousDoneCount;
  createEffect(() => {
    const current = snapshot();
    if (!current.generated_at) {
      previousDoneCount = void 0;
      return;
    }
    const done = current.summary?.done || 0;
    if (previousDoneCount !== void 0 && done > previousDoneCount) {
      const next = [...untrack(completionHistory), Date.now()].slice(-ETA_HISTORY_LIMIT);
      setCompletionHistory(next);
      if (api.kv && typeof api.kv.set === "function") {
        api.kv.set(ETA_HISTORY_KEY, next);
      }
    }
    previousDoneCount = done;
  });
  const boardEta = createMemo(() => {
    const summary = snapshot().summary;
    const remaining = (summary.ready || 0) + (summary.in_progress || 0) + (summary.in_review || 0);
    const history = completionHistory();
    const mean = meanCompletionInterval(history);
    return {
      remaining,
      backlog: summary.backlog || 0,
      samples: history.length,
      estimateMs: mean === void 0 ? void 0 : mean * remaining
    };
  });
  let disposed = false;
  let generation = 0;
  let activeKey = "";
  let pendingGeneration;
  let previousAttentionCount = 0;
  const togglePreference = (key, value, setter) => {
    const next = !value();
    setter(next);
    setPref(key, next);
  };
  const readSnapshot = () => {
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
    execFile(cortexExecutable(), ["ui", "snapshot", "--project", scope.project, "--session-id", scope.sessionID, "--root-session-id", scope.rootSessionID], {
      encoding: "utf8",
      maxBuffer: 512 * 1024,
      timeout: 7500,
      windowsHide: true
    }, (error, stdout) => {
      if (pendingGeneration === requestGeneration) pendingGeneration = void 0;
      if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api, activeSessionOverride())) !== key) return;
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
      setPref(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });
  let previousTaskStatuses = /* @__PURE__ */ new Map();
  createEffect(() => {
    const tasks = snapshot().tasks;
    const toastApi = api.ui?.toast || api.toast;
    if (toastApi && previousTaskStatuses.size > 0) {
      for (const t of tasks) {
        const prev = previousTaskStatuses.get(t.task_id);
        if (prev && prev !== t.status) {
          let variant = "info";
          let title = `Cortex-IA: Tarea ${t.task_id}`;
          if (t.status === "done") {
            variant = "success";
            title = `\u2713 ${t.task_id} Aprobada (PASS)`;
          } else if (t.status === "blocked") {
            variant = "error";
            title = `\u2715 ${t.task_id} Bloqueada`;
          } else if (t.status === "in_review") {
            variant = "warning";
            title = `\u2696 ${t.task_id} En Revisi\xF3n`;
          } else if (t.status === "in_progress") {
            variant = "info";
            title = `\u26A1 ${t.task_id} Reclamada`;
          }
          try {
            if (typeof toastApi.show === "function") {
              toastApi.show({
                title,
                message: clipped(t.title, 40),
                variant
              });
            } else if (typeof toastApi === "function") {
              toastApi({
                title,
                message: clipped(t.title, 40),
                variant
              });
            }
          } catch {
          }
        }
      }
    }
    const nextMap = /* @__PURE__ */ new Map();
    for (const t of tasks) nextMap.set(t.task_id, t.status);
    previousTaskStatuses = nextMap;
  });
  createEffect(() => {
    activeSessionOverride();
    readSnapshot();
  });
  const snapshotPoll = setInterval(readSnapshot, SNAPSHOT_POLL_INTERVAL_MS);
  const clock = setInterval(() => setNow(Date.now()), 1e3);
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
  if (api.ui && typeof api.ui.slot === "function") {
    api.ui.slot({
      append: "sidebar.content",
      render: (ctx) => {
        updateActiveSession(ctx);
        return _$createComponent(SidebarStatus, {
          nativeActivity,
          scopeReady,
          snapshot,
          jobs,
          snapshotError,
          sessionElapsed,
          eta: boardEta,
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
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        });
      }
    });
    api.ui.slot({
      prepend: "home.footer",
      render: (ctx) => _$createComponent(HomeLogo, {
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      })
    });
    api.ui.slot({
      append: "sidebar.footer",
      render: (ctx) => {
        updateActiveSession(ctx);
        return _$createComponent(SidebarFooterMetrics, {
          metrics: sidebarMetrics,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        });
      }
    });
    api.ui.slot({
      append: "home.footer.status",
      render: (ctx) => {
        updateActiveSession(ctx);
        return _$createComponent(HomeBottomStatus, {
          snapshot,
          jobs,
          spinner,
          snapshotError,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        });
      }
    });
    api.ui.slot({
      append: "session.panel",
      render: (panel) => {
        updateActiveSession(panel);
        return _$createComponent(Show, {
          get when() {
            return !panel?.name || panel?.name === "cortex.dashboard" || panel?.name === "session.panel" || panel?.name === "cortex.board" || panel?.name === "cortex";
          },
          get children() {
            return _$createComponent(SessionKanbanPanel, {
              snapshot,
              jobs,
              now,
              spinner,
              pulse,
              get theme() {
                return panel?.theme?.current || panel?.theme || api.theme;
              }
            });
          }
        });
      }
    });
    if (api.keymap && typeof api.keymap.layer === "function") {
      api.ui.slot({
        append: "app",
        render: () => {
          api.keymap.layer(() => ({
            mode: "global",
            commands: [{
              id: "cortex.board",
              title: "Cortex Board & Delegation",
              slash: {
                name: "cortex"
              },
              run: () => {
                if (api.ui?.panel?.open) {
                  api.ui.panel.open("cortex.board");
                }
              }
            }]
          }));
          return null;
        }
      });
    }
  }
  const registeredSlots = {
    home_logo(ctx) {
      return _$createComponent(HomeLogo, {
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    sidebar_content(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(SidebarStatus, {
        nativeActivity,
        scopeReady,
        snapshot,
        jobs,
        snapshotError,
        sessionElapsed,
        eta: boardEta,
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
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    home_bottom(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(HomeBottomStatus, {
        snapshot,
        jobs,
        spinner,
        snapshotError,
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    "home.footer.status"(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(HomeBottomStatus, {
        snapshot,
        jobs,
        spinner,
        snapshotError,
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    "session.panel"(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(SessionKanbanPanel, {
        snapshot,
        jobs,
        now,
        spinner,
        pulse,
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    session_panel(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(SessionKanbanPanel, {
        snapshot,
        jobs,
        now,
        spinner,
        pulse,
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    }
  };
  if (api.slots && typeof api.slots.register === "function") {
    api.slots.register({
      order: 85,
      slots: registeredSlots
    });
  }
  if (api.lifecycle && typeof api.lifecycle.onDispose === "function") {
    api.lifecycle.onDispose(cleanup);
  }
  return cleanup;
}
var Plugin = {
  define: (def) => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  }
};
var tui = async (api) => {
  createRoot((disposeRoot) => initialize(api, disposeRoot));
};
var setup = (context) => {
  return createRoot((disposeRoot) => initialize(context, disposeRoot));
};
var CortexTUIPluginDefinition = Plugin.define({
  id: "cortex-ia.delegation-status",
  setup,
  tui
});
var cortex_ia_tui_default = CortexTUIPluginDefinition;
export {
  CortexTUIPluginDefinition,
  Plugin,
  SidebarStatus,
  cortex_ia_tui_default as default
};
