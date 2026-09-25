// cortex-ia-tui.tsx
import { use as _$use } from "@opentui/solid";
import { createTextNode as _$createTextNode } from "@opentui/solid";
import { memo as _$memo } from "@opentui/solid";
import { createComponent as _$createComponent } from "@opentui/solid";
import { effect as _$effect } from "@opentui/solid";
import { insertNode as _$insertNode } from "@opentui/solid";
import { insert as _$insert } from "@opentui/solid";
import { setProp as _$setProp } from "@opentui/solid";
import { createElement as _$createElement } from "@opentui/solid";
import { execFile, spawn } from "child_process";
import fs from "fs";
import path from "path";
import { For, Show, createEffect, createMemo, createRoot, createSignal, untrack } from "solid-js";
import { useTerminalDimensions } from "@opentui/solid";
import { TextAttributes } from "@opentui/core";
var SNAPSHOT_POLL_INTERVAL_MS = 2500;
var SNAPSHOT_STALE_MS = 1e4;
var MAX_VISIBLE_ROWS = 4;
var SPINNER_FRAMES = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"];
var NEURAL_PULSE_FRAMES = ["\u25C6", "\u25C7", "\u25CB", "\u25C7"];
var NAN_USAGE_POLL_INTERVAL_MS = 6e4;
var NAN_CATALOG_TTL_MS = 10 * 6e4;
var NAN_EXEC_MAX_BUFFER = 4 * 1024 * 1024;
var NAN_EXEC_TIMEOUT_MS = 8e3;
var NAN_DETAIL_DAY_POINTS = 21;
var GLYPH = {
  active: "\u25C6",
  working: "\u25CF",
  idle: "\u25CB",
  done: "\u2713",
  fail: "\u2715",
  warn: "\u25B2",
  row: "\u25B8",
  web: "\u25C8",
  brand: "\u{1F9E0}"
};
var MARK_EXPANDED = "\u25BE";
var MARK_COLLAPSED = "\u25B8";
var NARROW_TERMINAL_COLS = 96;
var KANBAN_TWO_COLUMN_COLS = 64;
var KANBAN_COLUMN_SEPARATOR = 2;
var KANBAN_MIN_COLUMN_WIDTH = 12;
var KANBAN_READY_LIMIT = 3;
var KANBAN_COLUMN_BORDER = ["left"];
var TASKS_EXPANDED_KEY = "cortex.sidebar.tasks.expanded";
var DELEGATIONS_EXPANDED_KEY = "cortex.sidebar.delegations.expanded";
var ATTENTION_EXPANDED_KEY = "cortex.sidebar.attention.expanded";
var DENSITY_KEY = "cortex.sidebar.density";
var ETA_HISTORY_KEY = "cortex.dashboard.eta.completions";
var ETA_HISTORY_LIMIT = 12;
var ETA_INTERVAL_WINDOW = 8;
var ETA_MIN_SAMPLES = 2;
var ETA_SPARK_MIN_SAMPLES = 3;
var ETA_SPARK_POINTS = 12;
var ETA_SPARK_LEVELS = ["\u2840", "\u28C0", "\u28C4", "\u28C6", "\u28C7", "\u28E7", "\u28FF"];
var AGENTS_ROUTE = "cortex.agents";
var AGENTS_COMMAND = ":cortex-agents";
var AGENTS_MIN_COLUMNS = 68;
var AGENTS_NARROW_COLUMNS = 96;
var AGENTS_TITLE_MIN = 14;
var AGENTS_STATUS_WIDTH = 2;
var AGENTS_AGENT_WIDTH = 12;
var AGENTS_NARROW_AGENT_WIDTH = 9;
var AGENTS_ELAPSED_WIDTH = 8;
var AGENTS_ACTIVITY_WIDTH = 16;
var AGENTS_TOKENS_WIDTH = 8;
var AGENTS_COST_WIDTH = 8;
var AGENTS_TASK_WIDTH = 20;
var OPENCODE_SESSION_OWNER_PREFIX = "opencode-session:";
var PALETTE_FALLBACK = {
  text: "#f8fafc",
  textMuted: "#94a3b8",
  accent: "#a855f7",
  accentAlt: "#22d3ee",
  primary: "#6366f1",
  sky: "#38bdf8",
  success: "#34d399",
  warning: "#fbbf24",
  error: "#fb7185",
  info: "#22d3ee",
  border: "#334155",
  panel: "#111927",
  element: "#1e293b",
  background: "#0a0e17"
};
var SOFT_TEXT_WEIGHT = 0.65;
function readColor(value) {
  if (typeof value === "string" && value.trim() !== "") return value;
  if (typeof value === "object" && value !== null) {
    const candidate = value;
    if (typeof candidate.r === "number" && typeof candidate.g === "number" && typeof candidate.b === "number") {
      return value;
    }
  }
  return void 0;
}
function hexToBytes(value) {
  const hex = value.trim().replace(/^#/, "");
  if (!/^[0-9a-f]{6}$/i.test(hex)) return void 0;
  return {
    r: parseInt(hex.slice(0, 2), 16),
    g: parseInt(hex.slice(2, 4), 16),
    b: parseInt(hex.slice(4, 6), 16)
  };
}
function channelToByte(value) {
  const scaled = Math.abs(value) <= 1 ? value * 255 : value;
  return Math.round(Math.min(255, Math.max(0, scaled)));
}
function colorToBytes(color) {
  if (typeof color === "string") return hexToBytes(color);
  const candidate = color;
  if (typeof candidate.r !== "number" || typeof candidate.g !== "number" || typeof candidate.b !== "number") {
    return void 0;
  }
  return {
    r: channelToByte(candidate.r),
    g: channelToByte(candidate.g),
    b: channelToByte(candidate.b)
  };
}
function blendColor(foreground, background, weight) {
  const fg = colorToBytes(foreground);
  const bg = colorToBytes(background);
  if (!fg || !bg) return foreground;
  const mix = (a, b) => Math.round(a * weight + b * (1 - weight));
  const toHex = (value) => value.toString(16).padStart(2, "0");
  return `#${toHex(mix(fg.r, bg.r))}${toHex(mix(fg.g, bg.g))}${toHex(mix(fg.b, bg.b))}`;
}
function buildPalette(theme) {
  const source = theme ?? {};
  const read = (key, fallback) => readColor(source[key]) ?? fallback;
  const text = read("text", PALETTE_FALLBACK.text);
  const background = read("background", PALETTE_FALLBACK.background);
  const border = read("border", PALETTE_FALLBACK.border);
  return {
    text,
    textMuted: read("textMuted", PALETTE_FALLBACK.textMuted),
    textSoft: blendColor(text, background, SOFT_TEXT_WEIGHT),
    accent: read("accent", PALETTE_FALLBACK.accent),
    accentAlt: read("secondary", PALETTE_FALLBACK.accentAlt),
    primary: read("primary", PALETTE_FALLBACK.primary),
    sky: read("info", PALETTE_FALLBACK.sky),
    success: read("success", PALETTE_FALLBACK.success),
    warning: read("warning", PALETTE_FALLBACK.warning),
    error: read("error", PALETTE_FALLBACK.error),
    info: read("info", PALETTE_FALLBACK.info),
    border,
    borderSubtle: readColor(source["borderSubtle"]) ?? border,
    borderActive: readColor(source["borderActive"]) ?? border,
    panel: read("backgroundPanel", PALETTE_FALLBACK.panel),
    element: read("backgroundElement", PALETTE_FALLBACK.element),
    background
  };
}
var PALETTE_CACHE = /* @__PURE__ */ new WeakMap();
var themelessPalette;
function resolvePalette(theme) {
  if (theme && typeof theme === "object") {
    const cached = PALETTE_CACHE.get(theme);
    if (cached) return cached;
    const built = buildPalette(theme);
    PALETTE_CACHE.set(theme, built);
    return built;
  }
  return themelessPalette ??= buildPalette();
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
function nanExecutable() {
  if (process.env.NAN_BIN) return process.env.NAN_BIN;
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [path.join(local, "Programs", "nan", process.platform === "win32" ? "nan.exe" : "nan"), path.join(home, ".local", "bin", "nan"), "/usr/local/bin/nan", "/usr/bin/nan"];
  for (const candidate of candidates) {
    try {
      if (fs.existsSync(candidate)) return candidate;
    } catch {
    }
  }
  return process.platform === "win32" ? "nan.exe" : "nan";
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
function measuredColumns(width) {
  if (typeof width === "number" && Number.isFinite(width) && width > 0) return width;
  const columns = process.stdout?.columns;
  return typeof columns === "number" && Number.isFinite(columns) && columns > 0 ? columns : 0;
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
function completionIntervals(stamps) {
  const intervals = [];
  for (let i = 1; i < stamps.length; i += 1) {
    const delta = stamps[i] - stamps[i - 1];
    if (delta > 0) intervals.push(delta);
  }
  return intervals;
}
function meanCompletionInterval(stamps) {
  if (stamps.length < ETA_MIN_SAMPLES) return void 0;
  const intervals = completionIntervals(stamps);
  if (intervals.length === 0) return void 0;
  const recent = intervals.slice(-ETA_INTERVAL_WINDOW);
  return recent.reduce((sum, value) => sum + value, 0) / recent.length;
}
function etaSparkline(stamps) {
  if (stamps.length < ETA_SPARK_MIN_SAMPLES) return void 0;
  const intervals = completionIntervals(stamps).slice(-ETA_SPARK_POINTS);
  if (intervals.length === 0) return void 0;
  const min = Math.min(...intervals);
  const span = Math.max(...intervals) - min;
  const top = ETA_SPARK_LEVELS.length - 1;
  return intervals.map((value) => ETA_SPARK_LEVELS[span === 0 ? Math.floor(ETA_SPARK_LEVELS.length / 2) : Math.round((value - min) / span * top)]).join("");
}
function roleChip(role, palette) {
  const r = (role || "").toLowerCase();
  if (r.includes("orch")) return {
    color: palette.primary,
    tag: "ORCH"
  };
  if (r.includes("impl")) return {
    color: palette.warning,
    tag: "IMPL"
  };
  if (r.includes("rev")) return {
    color: palette.accent,
    tag: "REVW"
  };
  if (r.includes("inv")) return {
    color: palette.sky,
    tag: "INVS"
  };
  if (r.includes("plan")) return {
    color: palette.info,
    tag: "PLAN"
  };
  if (r.includes("disc")) return {
    color: palette.success,
    tag: "DISC"
  };
  return {
    color: palette.textMuted,
    tag: r.slice(0, 4).toUpperCase() || "WORK"
  };
}
function taskStatusChip(status, palette) {
  switch (status) {
    case "done":
      return {
        icon: GLYPH.done,
        color: palette.success,
        tag: "DONE"
      };
    case "in_progress":
      return {
        icon: GLYPH.working,
        color: palette.warning,
        tag: "PROG"
      };
    case "in_review":
      return {
        icon: GLYPH.active,
        color: palette.accent,
        tag: "REVW"
      };
    case "ready":
      return {
        icon: GLYPH.row,
        color: palette.sky,
        tag: "RDY "
      };
    case "blocked":
      return {
        icon: GLYPH.fail,
        color: palette.error,
        tag: "BLCK"
      };
    case "superseded":
      return {
        icon: ">>",
        color: palette.textMuted,
        tag: "SPRS"
      };
    case "backlog":
    default:
      return {
        icon: GLYPH.idle,
        color: palette.textMuted,
        tag: "WAIT"
      };
  }
}
function subagentStatus(status) {
  const type = typeof status === "string" ? status : status?.type;
  return type === "busy" || type === "idle" || type === "retry" ? type : "unknown";
}
function subagentRetryAttempt(status) {
  const attempt = status?.attempt;
  return typeof attempt === "number" && Number.isFinite(attempt) && attempt > 0 ? Math.floor(attempt) : void 0;
}
function sessionStartMillis(session) {
  const created = session?.time?.created;
  if (typeof created !== "number" || !Number.isFinite(created) || created <= 0) return void 0;
  return created < 1e12 ? created * 1e3 : created;
}
function sessionTokenTotal(session) {
  const tokens = session?.tokens;
  if (!tokens) return void 0;
  const used = (tokens.input || 0) + (tokens.output || 0) + (tokens.reasoning || 0) + (tokens.cache?.read || 0) + (tokens.cache?.write || 0);
  return used > 0 ? used : void 0;
}
function formatElapsedClock(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return "--:--";
  const totalSecs = Math.floor(ms / 1e3);
  const secs = totalSecs % 60;
  const mins = Math.floor(totalSecs / 60) % 60;
  const hours = Math.floor(totalSecs / 3600);
  const clock = `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
  return hours > 0 ? `${hours}:${clock}` : clock;
}
function fitWidth(value, width) {
  if (width <= 0) return "";
  if (value.length > width) return `${value.slice(0, Math.max(0, width - 1))}\u2026`;
  return value.padEnd(width, " ");
}
function taskForSession(sessionID, tasks) {
  for (const task of tasks || []) {
    if (!task) continue;
    const owner = typeof task.owner === "string" ? task.owner : "";
    const ownerSession = owner.startsWith(OPENCODE_SESSION_OWNER_PREFIX) ? owner.slice(OPENCODE_SESSION_OWNER_PREFIX.length) : "";
    if (task.opencode_session_id === sessionID || task.opencode_parent_session_id === sessionID || ownerSession === sessionID) {
      return {
        taskID: task.task_id,
        status: task.status
      };
    }
  }
  return void 0;
}
function partActivityLabel(part) {
  if (!part || typeof part.type !== "string") return void 0;
  if (part.type === "tool") return typeof part.tool === "string" && part.tool ? {
    tool: part.tool
  } : void 0;
  if (part.type === "reasoning") return {
    step: "reasoning"
  };
  if (part.type === "text") return {
    step: "writing"
  };
  if (part.type === "step-start") return {
    step: "step"
  };
  return void 0;
}
function lastActivityLabel(api, sessionID) {
  const messages = sessionMessages(api, sessionID);
  const partFn = api?.state?.part;
  if (!messages || typeof partFn !== "function") return {};
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const info = messages[i]?.info || messages[i];
    if (info?.role !== "assistant" || typeof info?.id !== "string") continue;
    let parts;
    try {
      parts = partFn(info.id);
    } catch {
      return {};
    }
    if (!Array.isArray(parts)) return {};
    for (let j = parts.length - 1; j >= 0; j -= 1) {
      const label = partActivityLabel(parts[j]);
      if (label) return label;
    }
    return {};
  }
  return {};
}
function subagentRank(status) {
  if (status === "busy") return 0;
  if (status === "retry") return 1;
  return 2;
}
function subagentStatusColor(status, palette) {
  if (status === "busy") return palette.accent;
  if (status === "retry") return palette.warning;
  if (status === "idle") return palette.textMuted;
  return palette.textSoft;
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
      title: "Snapshot unavailable",
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
  const freshnessColor = () => {
    if (props.freshness() === "live") return palette.success;
    if (props.freshness() === "stale") return palette.warning;
    return palette.textMuted;
  };
  const freshnessGlyph = () => {
    if (props.freshness() === "live") return GLYPH.working;
    if (props.freshness() === "stale") return GLYPH.warn;
    return GLYPH.idle;
  };
  const resolvedActivity = () => {
    const activity = props.nativeActivity();
    return activity && activity !== "unknown" ? activity : void 0;
  };
  return (() => {
    var _el$ = _$createElement("box"), _el$5 = _$createElement("box"), _el$6 = _$createElement("text"), _el$7 = _$createElement("box"), _el$8 = _$createElement("text"), _el$9 = _$createElement("text");
    _$insertNode(_el$, _el$5);
    _$insertNode(_el$, _el$7);
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
        var _el$2 = _$createElement("box"), _el$3 = _$createElement("text"), _el$4 = _$createElement("text");
        _$insertNode(_el$2, _el$3);
        _$insertNode(_el$2, _el$4);
        _$setProp(_el$2, "flexDirection", "row");
        _$setProp(_el$2, "marginTop", 0);
        _$insert(_el$3, () => `${GLYPH.row} `);
        _$insert(_el$4, () => clipped(projectName(), Math.max(6, props.textLimit - 4)));
        _$effect((_p$) => {
          var _v$ = palette.sky, _v$2 = palette.sky;
          _v$ !== _p$.e && (_p$.e = _$setProp(_el$3, "fg", _v$, _p$.e));
          _v$2 !== _p$.t && (_p$.t = _$setProp(_el$4, "fg", _v$2, _p$.t));
          return _p$;
        }, {
          e: void 0,
          t: void 0
        });
        return _el$2;
      }
    }), _el$5);
    _$insertNode(_el$5, _el$6);
    _$setProp(_el$5, "flexDirection", "row");
    _$setProp(_el$5, "marginTop", 0);
    _$insert(_el$6, () => `${freshnessGlyph()} ${props.freshness()}`);
    _$insert(_el$5, _$createComponent(Show, {
      get when() {
        return resolvedActivity();
      },
      children: (activity) => (() => {
        var _el$0 = _$createElement("text");
        _$insert(_el$0, () => ` \xB7 OpenCode: ${activity()}`);
        _$effect((_$p) => _$setProp(_el$0, "fg", palette.textMuted, _$p));
        return _el$0;
      })()
    }), null);
    _$insertNode(_el$7, _el$8);
    _$insertNode(_el$7, _el$9);
    _$setProp(_el$7, "flexDirection", "row");
    _$setProp(_el$7, "marginTop", 0);
    _$insert(_el$7, _$createComponent(Show, {
      get when() {
        return props.sessionElapsed?.();
      },
      children: (elapsed) => (() => {
        var _el$1 = _$createElement("text");
        _$insert(_el$1, () => clipped(`T+${elapsed()} `, Math.max(8, props.textLimit)));
        _$effect((_$p) => _$setProp(_el$1, "fg", palette.sky, _$p));
        return _el$1;
      })()
    }), _el$8);
    _$setProp(_el$8, "onMouseDown", () => openWebConsole());
    _$setProp(_el$8, "selectable", false);
    _$insert(_el$8, () => `[${GLYPH.web} Web] `);
    _$setProp(_el$9, "selectable", false);
    _$insert(_el$9, () => props.density() === "compact" ? `[${MARK_COLLAPSED} CMP]` : `[${MARK_EXPANDED} EXP]`);
    _$effect((_p$) => {
      var _v$3 = props.isExecuting() ? palette.warning : palette.primary, _v$4 = props.isExecuting() ? `${GLYPH.brand} CORTEX\xB7IA v2.0 [${props.spinner()} ACTIVE]` : `${GLYPH.brand} CORTEX\xB7IA v2.0 [${GLYPH.working} STANDBY]`, _v$5 = palette.accent, _v$6 = palette.panel, _v$7 = freshnessColor(), _v$8 = palette.info, _v$9 = props.density() === "compact" ? palette.success : palette.textMuted, _v$0 = props.onToggleDensity;
      _v$3 !== _p$.e && (_p$.e = _$setProp(_el$, "borderColor", _v$3, _p$.e));
      _v$4 !== _p$.t && (_p$.t = _$setProp(_el$, "title", _v$4, _p$.t));
      _v$5 !== _p$.a && (_p$.a = _$setProp(_el$, "titleColor", _v$5, _p$.a));
      _v$6 !== _p$.o && (_p$.o = _$setProp(_el$, "backgroundColor", _v$6, _p$.o));
      _v$7 !== _p$.i && (_p$.i = _$setProp(_el$6, "fg", _v$7, _p$.i));
      _v$8 !== _p$.n && (_p$.n = _$setProp(_el$8, "fg", _v$8, _p$.n));
      _v$9 !== _p$.s && (_p$.s = _$setProp(_el$9, "fg", _v$9, _p$.s));
      _v$0 !== _p$.h && (_p$.h = _$setProp(_el$9, "onMouseDown", _v$0, _p$.h));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0,
      n: void 0,
      s: void 0,
      h: void 0
    });
    return _el$;
  })();
}
function StatusCell(props) {
  return (() => {
    var _el$10 = _$createElement("box"), _el$11 = _$createElement("text"), _el$12 = _$createElement("text");
    _$insertNode(_el$10, _el$11);
    _$insertNode(_el$10, _el$12);
    _$setProp(_el$10, "flexDirection", "row");
    _$setProp(_el$10, "gap", 1);
    _$insert(_el$11, () => props.label);
    _$insert(_el$12, () => props.value());
    _$effect((_p$) => {
      var _v$1 = props.palette.textMuted, _v$10 = props.color ? props.color() : props.palette.text;
      _v$1 !== _p$.e && (_p$.e = _$setProp(_el$11, "fg", _v$1, _p$.e));
      _v$10 !== _p$.t && (_p$.t = _$setProp(_el$12, "fg", _v$10, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$10;
  })();
}
function padMetric(value) {
  return String(Math.max(0, Math.min(999, Math.round(value)))).padStart(2, "0");
}
function EmptyState(props) {
  return (() => {
    var _el$13 = _$createElement("box"), _el$14 = _$createElement("text");
    _$insertNode(_el$13, _el$14);
    _$setProp(_el$13, "flexDirection", "row");
    _$setProp(_el$13, "paddingLeft", 2);
    _$insert(_el$14, () => `${GLYPH.idle} ${props.label}`);
    _$effect((_$p) => _$setProp(_el$14, "fg", props.palette.textMuted, _$p));
    return _el$13;
  })();
}
function Section(props) {
  const displayTitle = createMemo(() => props.compact && props.shortTitle ? props.shortTitle : props.title);
  const displayBadge = createMemo(() => {
    if (props.densityCompact && !props.expanded() && props.summary) return props.summary;
    return props.compact && props.shortBadge ? props.shortBadge : props.badge;
  });
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$15 = _$createElement("box"), _el$16 = _$createElement("box"), _el$17 = _$createElement("text"), _el$18 = _$createElement("text");
    _$insertNode(_el$15, _el$16);
    _$setProp(_el$15, "flexDirection", "column");
    _$setProp(_el$15, "marginTop", 1);
    _$insertNode(_el$16, _el$17);
    _$insertNode(_el$16, _el$18);
    _$setProp(_el$16, "flexDirection", "row");
    _$setProp(_el$17, "selectable", false);
    _$insert(_el$17, () => props.expanded() ? `${MARK_EXPANDED} ` : `${MARK_COLLAPSED} `);
    _$setProp(_el$18, "selectable", false);
    _$insert(_el$18, displayTitle);
    _$insert(_el$16, _$createComponent(Show, {
      get when() {
        return displayBadge();
      },
      get children() {
        var _el$19 = _$createElement("text");
        _$insert(_el$19, () => ` [${displayBadge()}]`);
        _$effect((_$p) => _$setProp(_el$19, "fg", palette.sky, _$p));
        return _el$19;
      }
    }), null);
    _$insert(_el$15, _$createComponent(Show, {
      get when() {
        return props.expanded();
      },
      get children() {
        return props.children;
      }
    }), null);
    _$effect((_p$) => {
      var _v$11 = props.onToggle, _v$12 = props.expanded() ? palette.info : palette.textMuted, _v$13 = palette.text;
      _v$11 !== _p$.e && (_p$.e = _$setProp(_el$16, "onMouseDown", _v$11, _p$.e));
      _v$12 !== _p$.t && (_p$.t = _$setProp(_el$17, "fg", _v$12, _p$.t));
      _v$13 !== _p$.a && (_p$.a = _$setProp(_el$18, "fg", _v$13, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$15;
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
  const showLegend = createMemo(() => !props.compact && props.textLimit >= 34);
  const palette = resolvePalette(props.theme);
  const block = "\u2588";
  return (() => {
    var _el$20 = _$createElement("box"), _el$21 = _$createElement("box"), _el$22 = _$createElement("text"), _el$23 = _$createElement("text"), _el$24 = _$createElement("text"), _el$25 = _$createElement("text"), _el$27 = _$createElement("text"), _el$28 = _$createElement("text"), _el$29 = _$createElement("text");
    _$insertNode(_el$20, _el$21);
    _$setProp(_el$20, "flexDirection", "column");
    _$setProp(_el$20, "marginTop", 1);
    _$insertNode(_el$21, _el$22);
    _$insertNode(_el$21, _el$23);
    _$insertNode(_el$21, _el$24);
    _$insertNode(_el$21, _el$25);
    _$insertNode(_el$21, _el$27);
    _$insertNode(_el$21, _el$28);
    _$insertNode(_el$21, _el$29);
    _$setProp(_el$21, "flexDirection", "row");
    _$insert(_el$22, () => props.label ?? (props.compact ? "D:" : "DAG: "));
    _$insert(_el$23, () => block.repeat(doneW()));
    _$insert(_el$24, () => block.repeat(revW()));
    _$insert(_el$25, () => block.repeat(progW()));
    _$insert(_el$21, _$createComponent(Show, {
      get when() {
        return blckW() > 0;
      },
      get children() {
        var _el$26 = _$createElement("text");
        _$insert(_el$26, () => block.repeat(blckW()));
        _$effect((_$p) => _$setProp(_el$26, "fg", palette.error, _$p));
        return _el$26;
      }
    }), _el$27);
    _$insert(_el$27, () => block.repeat(emptyW()));
    _$insert(_el$28, () => ` ${pct()}%`);
    _$insert(_el$29, (() => {
      var _c$ = _$memo(() => !!props.compact);
      return () => _c$() ? "" : ` (${props.done}/${props.total})`;
    })());
    _$insert(_el$21, _$createComponent(Show, {
      get when() {
        return showLegend();
      },
      get children() {
        return [(() => {
          var _el$30 = _$createElement("text");
          _$insert(_el$30, () => ` \u2713${props.done}`);
          _$effect((_$p) => _$setProp(_el$30, "fg", palette.success, _$p));
          return _el$30;
        })(), (() => {
          var _el$31 = _$createElement("text");
          _$insert(_el$31, () => ` \u25C6${props.inReview}`);
          _$effect((_$p) => _$setProp(_el$31, "fg", palette.primary, _$p));
          return _el$31;
        })(), (() => {
          var _el$32 = _$createElement("text");
          _$insert(_el$32, () => ` \u25CF${props.inProgress}`);
          _$effect((_$p) => _$setProp(_el$32, "fg", palette.warning, _$p));
          return _el$32;
        })(), _$createComponent(Show, {
          get when() {
            return (props.blocked || 0) > 0;
          },
          get children() {
            var _el$33 = _$createElement("text");
            _$insert(_el$33, () => ` \u2715${props.blocked}`);
            _$effect((_$p) => _$setProp(_el$33, "fg", palette.error, _$p));
            return _el$33;
          }
        }), (() => {
          var _el$34 = _$createElement("text");
          _$insert(_el$34, () => ` \u25CB${waiting()}`);
          _$effect((_$p) => _$setProp(_el$34, "fg", palette.textMuted, _$p));
          return _el$34;
        })()];
      }
    }), null);
    _$effect((_p$) => {
      var _v$14 = palette.info, _v$15 = palette.success, _v$16 = palette.primary, _v$17 = palette.warning, _v$18 = palette.borderSubtle, _v$19 = palette.text, _v$20 = palette.textMuted;
      _v$14 !== _p$.e && (_p$.e = _$setProp(_el$22, "fg", _v$14, _p$.e));
      _v$15 !== _p$.t && (_p$.t = _$setProp(_el$23, "fg", _v$15, _p$.t));
      _v$16 !== _p$.a && (_p$.a = _$setProp(_el$24, "fg", _v$16, _p$.a));
      _v$17 !== _p$.o && (_p$.o = _$setProp(_el$25, "fg", _v$17, _p$.o));
      _v$18 !== _p$.i && (_p$.i = _$setProp(_el$27, "fg", _v$18, _p$.i));
      _v$19 !== _p$.n && (_p$.n = _$setProp(_el$28, "fg", _v$19, _p$.n));
      _v$20 !== _p$.s && (_p$.s = _$setProp(_el$29, "fg", _v$20, _p$.s));
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
    return _el$20;
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
    return diff > 0 ? formatDuration(diff) : "expired";
  });
  const palette = resolvePalette(props.theme);
  return (() => {
    var _el$35 = _$createElement("box"), _el$36 = _$createElement("box"), _el$37 = _$createElement("text"), _el$38 = _$createElement("text"), _el$40 = _$createElement("box"), _el$41 = _$createElement("text");
    _$insertNode(_el$35, _el$36);
    _$insertNode(_el$35, _el$40);
    _$setProp(_el$35, "flexDirection", "column");
    _$setProp(_el$35, "marginTop", 1);
    _$setProp(_el$35, "paddingLeft", 1);
    _$setProp(_el$35, "paddingRight", 1);
    _$setProp(_el$35, "borderStyle", "rounded");
    _$setProp(_el$35, "titleAlignment", "left");
    _$insertNode(_el$36, _el$37);
    _$insertNode(_el$36, _el$38);
    _$setProp(_el$36, "flexDirection", "row");
    _$insert(_el$37, () => `${GLYPH.active} `);
    _$insert(_el$38, () => clipped(`${props.task.task_id} \xB7 ${props.task.title}`, props.textLimit - 3));
    _$insert(_el$36, _$createComponent(Show, {
      get when() {
        return props.densityCompact;
      },
      get children() {
        var _el$39 = _$createElement("text");
        _$setProp(_el$39, "onMouseDown", () => openWebConsole(props.task.board_id, props.task.task_id));
        _$setProp(_el$39, "selectable", false);
        _$insert(_el$39, () => ` [${GLYPH.web}]`);
        _$effect((_$p) => _$setProp(_el$39, "fg", palette.info, _$p));
        return _el$39;
      }
    }), null);
    _$insertNode(_el$40, _el$41);
    _$setProp(_el$40, "flexDirection", "row");
    _$insert(_el$41, () => `T+${elapsed()} `);
    _$insert(_el$40, _$createComponent(Show, {
      get when() {
        return ttlRemaining();
      },
      children: (ttl) => (() => {
        var _el$45 = _$createElement("text");
        _$insert(_el$45, () => `\u2502 ${GLYPH.warn} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`);
        _$effect((_$p) => _$setProp(_el$45, "fg", ttl() === "expired" ? palette.error : palette.sky, _$p));
        return _el$45;
      })()
    }), null);
    _$insert(_el$35, _$createComponent(Show, {
      get when() {
        return !props.densityCompact;
      },
      get children() {
        return [(() => {
          var _el$42 = _$createElement("box");
          _$setProp(_el$42, "flexDirection", "row");
          _$insert(_el$42, _$createComponent(Show, {
            get when() {
              return props.activeDelegation;
            },
            get fallback() {
              return (() => {
                var _el$46 = _$createElement("text");
                _$insert(_el$46, () => clipped(`${GLYPH.row} Durable task${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit));
                _$effect((_$p) => _$setProp(_el$46, "fg", palette.accent, _$p));
                return _el$46;
              })();
            },
            children: (del) => (() => {
              var _el$47 = _$createElement("text");
              _$insert(_el$47, () => `${GLYPH.working} ${del().transport || "direct"}${del().pane_id ? ` \xB7 ${del().pane_id}` : ""}${del().attempt ? ` \xB7 int #${del().attempt}` : ""}`);
              _$effect((_$p) => _$setProp(_el$47, "fg", palette.info, _$p));
              return _el$47;
            })()
          }));
          return _el$42;
        })(), (() => {
          var _el$43 = _$createElement("box"), _el$44 = _$createElement("text");
          _$insertNode(_el$43, _el$44);
          _$setProp(_el$43, "flexDirection", "row");
          _$setProp(_el$43, "marginTop", 0);
          _$setProp(_el$43, "onMouseDown", () => openWebConsole(props.task.board_id, props.task.task_id));
          _$setProp(_el$44, "selectable", false);
          _$insert(_el$44, () => `[${GLYPH.web} View in Web]`);
          _$effect((_$p) => _$setProp(_el$44, "fg", palette.info, _$p));
          return _el$43;
        })()];
      }
    }), null);
    _$effect((_p$) => {
      var _v$21 = props.activeDelegation ? palette.warning : palette.borderSubtle, _v$22 = clipped(`${props.spinner()} TASK IN PROGRESS`, props.textLimit), _v$23 = palette.warning, _v$24 = palette.panel, _v$25 = palette.sky, _v$26 = palette.text, _v$27 = palette.warning;
      _v$21 !== _p$.e && (_p$.e = _$setProp(_el$35, "borderColor", _v$21, _p$.e));
      _v$22 !== _p$.t && (_p$.t = _$setProp(_el$35, "title", _v$22, _p$.t));
      _v$23 !== _p$.a && (_p$.a = _$setProp(_el$35, "titleColor", _v$23, _p$.a));
      _v$24 !== _p$.o && (_p$.o = _$setProp(_el$35, "backgroundColor", _v$24, _p$.o));
      _v$25 !== _p$.i && (_p$.i = _$setProp(_el$37, "fg", _v$25, _p$.i));
      _v$26 !== _p$.n && (_p$.n = _$setProp(_el$38, "fg", _v$26, _p$.n));
      _v$27 !== _p$.s && (_p$.s = _$setProp(_el$41, "fg", _v$27, _p$.s));
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
    return _el$35;
  })();
}
function TaskRows(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.tasks.length > 0;
    },
    get fallback() {
      return _$createComponent(EmptyState, {
        palette,
        label: "No queued tasks"
      });
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
            var _el$48 = _$createElement("box"), _el$49 = _$createElement("box"), _el$50 = _$createElement("text"), _el$51 = _$createElement("text"), _el$52 = _$createElement("text"), _el$53 = _$createElement("text"), _el$54 = _$createElement("box"), _el$55 = _$createElement("text");
            _$insertNode(_el$48, _el$49);
            _$insertNode(_el$48, _el$54);
            _$setProp(_el$48, "flexDirection", "column");
            _$setProp(_el$48, "marginTop", 0);
            _$insertNode(_el$49, _el$50);
            _$insertNode(_el$49, _el$51);
            _$insertNode(_el$49, _el$52);
            _$insertNode(_el$49, _el$53);
            _$setProp(_el$49, "flexDirection", "row");
            _$setProp(_el$49, "paddingLeft", 2);
            _$insert(_el$50, () => `${isProg ? props.spinner() : chip.icon} `);
            _$insert(_el$51, () => `[${chip.tag}] `);
            _$insert(_el$52, () => clipped(task.task_id, Math.max(8, props.textLimit - 14)));
            _$setProp(_el$53, "onMouseDown", () => openWebConsole(task.board_id, task.task_id));
            _$setProp(_el$53, "selectable", false);
            _$insert(_el$53, () => ` [${GLYPH.web}]`);
            _$insertNode(_el$54, _el$55);
            _$setProp(_el$54, "flexDirection", "row");
            _$setProp(_el$54, "paddingLeft", 5);
            _$insert(_el$55, () => `${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` \xB7 ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` \xB7 ${GLYPH.warn} ${task.lease_count}lk` : ""}`);
            _$effect((_p$) => {
              var _v$28 = chip.color, _v$29 = chip.color, _v$30 = isProg ? palette.text : palette.textSoft, _v$31 = palette.info, _v$32 = palette.textMuted;
              _v$28 !== _p$.e && (_p$.e = _$setProp(_el$50, "fg", _v$28, _p$.e));
              _v$29 !== _p$.t && (_p$.t = _$setProp(_el$51, "fg", _v$29, _p$.t));
              _v$30 !== _p$.a && (_p$.a = _$setProp(_el$52, "fg", _v$30, _p$.a));
              _v$31 !== _p$.o && (_p$.o = _$setProp(_el$53, "fg", _v$31, _p$.o));
              _v$32 !== _p$.i && (_p$.i = _$setProp(_el$55, "fg", _v$32, _p$.i));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0,
              i: void 0
            });
            return _el$48;
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
      return _$createComponent(EmptyState, {
        palette,
        label: "No active workers"
      });
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
            var _el$56 = _$createElement("box"), _el$57 = _$createElement("box"), _el$58 = _$createElement("text"), _el$59 = _$createElement("text"), _el$60 = _$createElement("text"), _el$61 = _$createElement("text"), _el$62 = _$createElement("box"), _el$63 = _$createElement("text");
            _$insertNode(_el$56, _el$57);
            _$insertNode(_el$56, _el$62);
            _$setProp(_el$56, "flexDirection", "column");
            _$setProp(_el$56, "marginTop", 0);
            _$insertNode(_el$57, _el$58);
            _$insertNode(_el$57, _el$59);
            _$insertNode(_el$57, _el$60);
            _$insertNode(_el$57, _el$61);
            _$setProp(_el$57, "flexDirection", "row");
            _$setProp(_el$57, "paddingLeft", 2);
            _$setProp(_el$58, "fg", statusCol);
            _$insert(_el$58, () => `${isRunning ? props.spinner() : job.status === "succeeded" ? GLYPH.done : GLYPH.fail} `);
            _$insert(_el$59, () => `[${chip.tag}] `);
            _$insert(_el$60, () => clipped(job.role || "worker", Math.max(6, props.textLimit - 12)));
            _$setProp(_el$61, "fg", statusCol);
            _$insert(_el$61, elapsed);
            _$insertNode(_el$62, _el$63);
            _$setProp(_el$62, "flexDirection", "row");
            _$setProp(_el$62, "paddingLeft", 5);
            _$insert(_el$63, () => clipped(`${shortID(job.job_id)} \xB7 ${job.transport || "direct"}${job.pane_id ? ` \xB7 ${job.pane_id}` : ""}${job.attempt ? ` \xB7 int #${job.attempt}` : ""}`, props.textLimit));
            _$effect((_p$) => {
              var _v$33 = chip.color, _v$34 = isRunning ? palette.text : palette.textSoft, _v$35 = palette.textMuted;
              _v$33 !== _p$.e && (_p$.e = _$setProp(_el$59, "fg", _v$33, _p$.e));
              _v$34 !== _p$.t && (_p$.t = _$setProp(_el$60, "fg", _v$34, _p$.t));
              _v$35 !== _p$.a && (_p$.a = _$setProp(_el$63, "fg", _v$35, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$56;
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
      return _$createComponent(EmptyState, {
        palette,
        label: "No pending alerts"
      });
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.items.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (item) => (() => {
          var _el$64 = _$createElement("box"), _el$65 = _$createElement("box"), _el$66 = _$createElement("text"), _el$67 = _$createElement("text"), _el$68 = _$createElement("box"), _el$69 = _$createElement("text");
          _$insertNode(_el$64, _el$65);
          _$insertNode(_el$64, _el$68);
          _$setProp(_el$64, "flexDirection", "column");
          _$insertNode(_el$65, _el$66);
          _$insertNode(_el$65, _el$67);
          _$setProp(_el$65, "flexDirection", "row");
          _$setProp(_el$65, "paddingLeft", 2);
          _$insert(_el$66, () => `${GLYPH.fail} `);
          _$insert(_el$67, () => clipped(item.title, Math.max(8, props.textLimit - 4)));
          _$insertNode(_el$68, _el$69);
          _$setProp(_el$68, "flexDirection", "row");
          _$setProp(_el$68, "paddingLeft", 5);
          _$insert(_el$69, () => clipped(item.detail, Math.max(8, props.textLimit - 5)));
          _$effect((_p$) => {
            var _v$36 = palette.error, _v$37 = palette.text, _v$38 = palette.textMuted;
            _v$36 !== _p$.e && (_p$.e = _$setProp(_el$66, "fg", _v$36, _p$.e));
            _v$37 !== _p$.t && (_p$.t = _$setProp(_el$67, "fg", _v$37, _p$.t));
            _v$38 !== _p$.a && (_p$.a = _$setProp(_el$69, "fg", _v$38, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$64;
        })()
      });
    }
  });
}
function OperationalStatusBlock(props) {
  const palette = resolvePalette(props.theme);
  const succeededJobs = createMemo(() => props.jobs.filter((j) => j.status === "succeeded").length);
  const failedJobs = createMemo(() => props.jobs.filter((j) => ["failed", "timed_out", "lost", "cancelled"].includes(j.status)).length);
  const activeJobs = createMemo(() => props.jobs.filter((j) => ["running", "starting", "accepted"].includes(j.status)).length);
  const doneTasks = createMemo(() => props.snapshot.summary.done || 0);
  const totalTasks = createMemo(() => props.snapshot.summary.total_tasks || props.snapshot.tasks.length);
  const totalLeases = createMemo(() => props.snapshot.tasks.reduce((sum, t) => sum + (t.lease_count || 0), 0));
  const blockedTasks = createMemo(() => props.snapshot.summary.blocked || 0);
  const metrics = createMemo(() => [{
    label: "act",
    value: props.activeExecutions
  }, {
    label: "rev",
    value: props.inReview
  }, {
    label: "done",
    value: doneTasks()
  }, {
    label: "alrt",
    value: props.attentionCount
  }]);
  const hasSignal = createMemo(() => props.activeExecutions > 0 || props.inReview > 0 || props.attentionCount > 0 || doneTasks() > 0 || totalTasks() > 0 || totalLeases() > 0 || blockedTasks() > 0 || succeededJobs() > 0 || failedJobs() > 0 || activeJobs() > 0);
  const etaLabel = createMemo(() => {
    const eta = props.eta();
    const suffix = eta.backlog > 0 ? ` \xB7 ${eta.backlog} backlog` : "";
    if (props.layout.compact) {
      const shortSuffix = eta.backlog > 0 ? ` +${eta.backlog}b` : "";
      if (eta.remaining === 0) return `ETA: ready${shortSuffix}`;
      if (eta.estimateMs === void 0) return `ETA: est\u2026 (${eta.remaining})${shortSuffix}`;
      return `ETA: ${formatDuration(eta.estimateMs)} (${eta.remaining})${shortSuffix}`;
    }
    if (eta.remaining === 0) return `ETA: board complete${suffix}`;
    if (eta.estimateMs === void 0) return `ETA: estimating (${eta.remaining} left)${suffix}`;
    return `ETA: ${formatDuration(eta.estimateMs)} \xB7 ${eta.remaining} left${suffix}`;
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
  const freshnessLabel = createMemo(() => {
    if (props.stale) {
      return props.layout.compact ? `${GLYPH.warn} stale` : `${GLYPH.warn} stale (+${syncAgeSec()}s)`;
    }
    return props.layout.compact ? `${GLYPH.working} live` : `${GLYPH.working} live \xB7 ${syncAgeSec()}s ago`;
  });
  const synapseValue = createMemo(() => activeJobs() > 0 ? `${props.spinner()} active \xB7 ${activeJobs()} runs` : `${props.pulse()} synced`);
  const authorityValue = createMemo(() => totalLeases() > 0 ? `sqlite \xB7 ${totalLeases()} lk` : "sqlite");
  const healthCell = () => (() => {
    var _el$70 = _$createElement("box"), _el$71 = _$createElement("text");
    _$insertNode(_el$70, _el$71);
    _$setProp(_el$70, "flexDirection", "row");
    _$setProp(_el$70, "gap", 1);
    _$insertNode(_el$71, _$createTextNode(`health`));
    _$insert(_el$70, _$createComponent(Show, {
      get when() {
        return successRate() !== void 0;
      },
      get fallback() {
        return (() => {
          var _el$76 = _$createElement("text");
          _$insert(_el$76, () => `${GLYPH.idle} standby`);
          _$effect((_$p) => _$setProp(_el$76, "fg", palette.textMuted, _$p));
          return _el$76;
        })();
      },
      get children() {
        return [(() => {
          var _el$73 = _$createElement("text");
          _$insert(_el$73, () => healthBars().filled);
          _$effect((_$p) => _$setProp(_el$73, "fg", healthColor(), _$p));
          return _el$73;
        })(), (() => {
          var _el$74 = _$createElement("text");
          _$insert(_el$74, () => healthBars().empty);
          _$effect((_$p) => _$setProp(_el$74, "fg", palette.borderSubtle, _$p));
          return _el$74;
        })(), (() => {
          var _el$75 = _$createElement("text");
          _$insert(_el$75, () => `${successRate()}%`);
          _$effect((_$p) => _$setProp(_el$75, "fg", healthColor(), _$p));
          return _el$75;
        })()];
      }
    }), null);
    _$effect((_$p) => _$setProp(_el$71, "fg", palette.textMuted, _$p));
    return _el$70;
  })();
  const signalCells = () => [_$createComponent(StatusCell, {
    palette,
    label: "synapse",
    value: synapseValue,
    color: () => activeJobs() > 0 ? palette.warning : palette.text
  }), _$memo(healthCell), _$createComponent(StatusCell, {
    palette,
    label: "authority",
    value: authorityValue
  }), _$createComponent(StatusCell, {
    palette,
    label: "web",
    value: () => "loopback",
    color: () => palette.info
  })];
  return (() => {
    var _el$77 = _$createElement("box");
    _$setProp(_el$77, "flexDirection", "column");
    _$setProp(_el$77, "marginTop", 1);
    _$setProp(_el$77, "paddingLeft", 1);
    _$setProp(_el$77, "paddingRight", 1);
    _$setProp(_el$77, "borderStyle", "rounded");
    _$setProp(_el$77, "titleAlignment", "left");
    _$insert(_el$77, _$createComponent(Show, {
      get when() {
        return hasSignal();
      },
      get fallback() {
        return (() => {
          var _el$85 = _$createElement("text");
          _$insert(_el$85, () => `${GLYPH.idle} standby \xB7 0 active \xB7 authority sqlite \xB7 ${props.stale ? "stale" : "live"}`);
          _$effect((_$p) => _$setProp(_el$85, "fg", palette.textMuted, _$p));
          return _el$85;
        })();
      },
      get children() {
        return [(() => {
          var _el$78 = _$createElement("box");
          _$setProp(_el$78, "flexDirection", "row");
          _$setProp(_el$78, "gap", 2);
          _$insert(_el$78, _$createComponent(For, {
            get each() {
              return metrics();
            },
            children: (metric) => (() => {
              var _el$86 = _$createElement("box"), _el$87 = _$createElement("text"), _el$88 = _$createElement("text");
              _$insertNode(_el$86, _el$87);
              _$insertNode(_el$86, _el$88);
              _$setProp(_el$86, "flexDirection", "row");
              _$setProp(_el$86, "gap", 1);
              _$insert(_el$87, () => metric.label);
              _$insert(_el$88, () => padMetric(metric.value));
              _$effect((_p$) => {
                var _v$43 = palette.textMuted, _v$44 = palette.accent;
                _v$43 !== _p$.e && (_p$.e = _$setProp(_el$87, "fg", _v$43, _p$.e));
                _v$44 !== _p$.t && (_p$.t = _$setProp(_el$88, "fg", _v$44, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$86;
            })()
          }));
          return _el$78;
        })(), _$createComponent(Show, {
          get when() {
            return !props.layout.compact;
          },
          get fallback() {
            return (() => {
              var _el$89 = _$createElement("box");
              _$setProp(_el$89, "flexDirection", "column");
              _$insert(_el$89, signalCells);
              return _el$89;
            })();
          },
          get children() {
            var _el$79 = _$createElement("box"), _el$80 = _$createElement("box"), _el$81 = _$createElement("box");
            _$insertNode(_el$79, _el$80);
            _$insertNode(_el$79, _el$81);
            _$setProp(_el$79, "flexDirection", "row");
            _$setProp(_el$79, "gap", 2);
            _$setProp(_el$80, "flexDirection", "column");
            _$setProp(_el$80, "flexGrow", 1);
            _$insert(_el$80, _$createComponent(StatusCell, {
              palette,
              label: "synapse",
              value: synapseValue,
              color: () => activeJobs() > 0 ? palette.warning : palette.text
            }), null);
            _$insert(_el$80, healthCell, null);
            _$setProp(_el$81, "flexDirection", "column");
            _$setProp(_el$81, "flexGrow", 1);
            _$insert(_el$81, _$createComponent(StatusCell, {
              palette,
              label: "authority",
              value: authorityValue
            }), null);
            _$insert(_el$81, _$createComponent(StatusCell, {
              palette,
              label: "web",
              value: () => "loopback",
              color: () => palette.info
            }), null);
            return _el$79;
          }
        }), (() => {
          var _el$82 = _$createElement("box"), _el$83 = _$createElement("text");
          _$insertNode(_el$82, _el$83);
          _$setProp(_el$82, "flexDirection", "row");
          _$setProp(_el$82, "gap", 2);
          _$insert(_el$83, freshnessLabel);
          _$insert(_el$82, _$createComponent(Show, {
            get when() {
              return totalTasks() > 0;
            },
            get children() {
              return [(() => {
                var _el$84 = _$createElement("text");
                _$insert(_el$84, etaLabel);
                _$effect((_$p) => _$setProp(_el$84, "fg", etaColor(), _$p));
                return _el$84;
              })(), _$createComponent(Show, {
                get when() {
                  return props.eta().sparkline;
                },
                children: (spark) => (() => {
                  var _el$90 = _$createElement("text");
                  _$insert(_el$90, spark);
                  _$effect((_$p) => _$setProp(_el$90, "fg", palette.textMuted, _$p));
                  return _el$90;
                })()
              })];
            }
          }), null);
          _$effect((_$p) => _$setProp(_el$83, "fg", props.stale ? palette.warning : palette.success, _$p));
          return _el$82;
        })()];
      }
    }));
    _$effect((_p$) => {
      var _v$39 = failedJobs() > 0 ? palette.error : props.stale ? palette.borderSubtle : palette.borderActive, _v$40 = props.layout.compact ? `${GLYPH.brand} CONTROL` : `${GLYPH.brand} CONTROL MATRIX`, _v$41 = palette.accent, _v$42 = palette.element;
      _v$39 !== _p$.e && (_p$.e = _$setProp(_el$77, "borderColor", _v$39, _p$.e));
      _v$40 !== _p$.t && (_p$.t = _$setProp(_el$77, "title", _v$40, _p$.t));
      _v$41 !== _p$.a && (_p$.a = _$setProp(_el$77, "titleColor", _v$41, _p$.a));
      _v$42 !== _p$.o && (_p$.o = _$setProp(_el$77, "backgroundColor", _v$42, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$77;
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
  const freshness = createMemo(() => {
    if (!props.scopeReady() || !props.snapshot().generated_at) return "standby";
    return stale() ? "stale" : "live";
  });
  const snapshotFailureLabel = createMemo(() => {
    const generated = Date.parse(props.snapshot().generated_at);
    if (!Number.isFinite(generated)) return "Snapshot refresh failed";
    return `Snapshot refresh failed \xB7 data from ${Math.round((props.now() - generated) / 1e3)}s ago`;
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
    var _el$91 = _$createElement("box");
    _$use((node) => setRootWidth(Math.max(0, node.width || 0)), _el$91);
    _$setProp(_el$91, "flexDirection", "column");
    _$setProp(_el$91, "onSizeChange", function() {
      setRootWidth(Math.max(0, this.width || 0));
    });
    _$insert(_el$91, _$createComponent(CortexCockpitHeader, {
      isExecuting,
      get nativeActivity() {
        return props.nativeActivity;
      },
      freshness,
      get projectRoot() {
        return props.snapshot().project_root;
      },
      get sessionElapsed() {
        return props.sessionElapsed;
      },
      get density() {
        return props.density;
      },
      get onToggleDensity() {
        return props.toggleDensity;
      },
      get textLimit() {
        return layout().textLimit;
      },
      get spinner() {
        return props.spinner;
      },
      get theme() {
        return props.theme;
      }
    }), null);
    _$insert(_el$91, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$92 = _$createElement("text");
        _$insertNode(_el$92, _$createTextNode(`Conversation unavailable \xB7 awaiting metadata`));
        _$setProp(_el$92, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$92, "fg", palette.warning, _$p));
        return _el$92;
      }
    }), null);
    _$insert(_el$91, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$94 = _$createElement("text");
        _$insertNode(_el$94, _$createTextNode(`Loading conversation state\u2026`));
        _$setProp(_el$94, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$94, "fg", palette.textMuted, _$p));
        return _el$94;
      }
    }), null);
    _$insert(_el$91, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$96 = _$createElement("text");
        _$setProp(_el$96, "marginTop", 1);
        _$insert(_el$96, snapshotFailureLabel);
        _$effect((_$p) => _$setProp(_el$96, "fg", palette.error, _$p));
        return _el$96;
      }
    }), null);
    _$insert(_el$91, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!props.scopeReady())() && Boolean(props.snapshot().generated_at);
      },
      get children() {
        return [_$createComponent(OperationalStatusBlock, {
          get snapshot() {
            return props.snapshot();
          },
          get jobs() {
            return props.jobs();
          },
          get activeExecutions() {
            return activeExecutionsCount();
          },
          get inReview() {
            return counts().review;
          },
          get attentionCount() {
            return counts().attention;
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
            get densityCompact() {
              return props.density() === "compact";
            },
            get textLimit() {
              return layout().textLimit;
            },
            get theme() {
              return props.theme;
            }
          })
        }), _$createComponent(Section, {
          title: "Task Board",
          shortTitle: "Tasks",
          get badge() {
            return _$memo(() => totalTasks() > 0)() ? `${props.snapshot().summary.active_tasks || 0} act / ${totalTasks()} tot` : void 0;
          },
          get shortBadge() {
            return _$memo(() => totalTasks() > 0)() ? `${props.snapshot().summary.active_tasks || 0}/${totalTasks()}` : void 0;
          },
          get summary() {
            return `${counts().active} act \xB7 ${inReviewTasks()} rev \xB7 ${totalTasks()} tot`;
          },
          get compact() {
            return layout().compact;
          },
          get densityCompact() {
            return props.density() === "compact";
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
          title: "Workers & Delegation",
          shortTitle: "Workers",
          get badge() {
            return _$memo(() => totalDelegationsCount() > 0)() ? `${activeDelegationsCount()} act / ${totalDelegationsCount()} tot` : void 0;
          },
          get shortBadge() {
            return _$memo(() => totalDelegationsCount() > 0)() ? `${activeDelegationsCount()}/${totalDelegationsCount()}` : void 0;
          },
          get summary() {
            return `${activeDelegationsCount()} act \xB7 ${totalDelegationsCount()} tot`;
          },
          get compact() {
            return layout().compact;
          },
          get densityCompact() {
            return props.density() === "compact";
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
          title: "Attention Center",
          shortTitle: "Alerts",
          get badge() {
            return _$memo(() => counts().attention > 0)() ? `${counts().attention} alerts` : void 0;
          },
          get shortBadge() {
            return _$memo(() => counts().attention > 0)() ? `${counts().attention}` : void 0;
          },
          get summary() {
            return _$memo(() => counts().attention > 0)() ? `${counts().attention} alerts` : "no alerts";
          },
          get compact() {
            return layout().compact;
          },
          get densityCompact() {
            return props.density() === "compact";
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
        })];
      }
    }), null);
    return _el$91;
  })();
}
function SidebarFooterMetrics(props) {
  const dimensions = useTerminalDimensions();
  const contextPct = createMemo(() => {
    const metrics = props.metrics();
    if (metrics.tokensUsed === void 0 || metrics.tokenLimit === void 0) return void 0;
    return Math.min(100, Math.max(0, Math.round(metrics.tokensUsed / metrics.tokenLimit * 100)));
  });
  const gaugeSegments = createMemo(() => dimensions().width < NARROW_TERMINAL_COLS ? 6 : 10);
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
    return metrics.tokensUsed !== void 0 || metrics.cost !== void 0;
  });
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$97 = _$createElement("box");
      _$setProp(_el$97, "flexDirection", "row");
      _$setProp(_el$97, "paddingLeft", 1);
      _$setProp(_el$97, "paddingRight", 1);
      _$insert(_el$97, _$createComponent(Show, {
        get when() {
          return props.metrics().tokensUsed !== void 0;
        },
        get children() {
          return [(() => {
            var _el$98 = _$createElement("text");
            _$insertNode(_el$98, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$98, "fg", palette.border, _$p));
            return _el$98;
          })(), _$createComponent(Show, {
            get when() {
              return contextGauge();
            },
            get fallback() {
              return (() => {
                var _el$103 = _$createElement("text");
                _$insert(_el$103, () => `\u25C6 ${formatTokens(props.metrics().tokensUsed)} tok`);
                _$effect((_$p) => _$setProp(_el$103, "fg", palette.sky, _$p));
                return _el$103;
              })();
            },
            children: (gauge) => (() => {
              var _el$104 = _$createElement("box"), _el$105 = _$createElement("text"), _el$107 = _$createElement("text"), _el$108 = _$createElement("text"), _el$109 = _$createElement("text"), _el$110 = _$createElement("text"), _el$112 = _$createElement("text");
              _$insertNode(_el$104, _el$105);
              _$insertNode(_el$104, _el$107);
              _$insertNode(_el$104, _el$108);
              _$insertNode(_el$104, _el$109);
              _$insertNode(_el$104, _el$110);
              _$insertNode(_el$104, _el$112);
              _$setProp(_el$104, "flexDirection", "row");
              _$insertNode(_el$105, _$createTextNode(`\u25C6 `));
              _$insert(_el$107, () => gauge().filled);
              _$insert(_el$108, () => gauge().empty);
              _$insert(_el$109, () => ` ${contextPct()}%`);
              _$insertNode(_el$110, _$createTextNode(` \xB7 `));
              _$insert(_el$112, () => formatTokens(props.metrics().tokensUsed));
              _$effect((_p$) => {
                var _v$45 = contextColor(), _v$46 = contextColor(), _v$47 = palette.border, _v$48 = contextColor(), _v$49 = palette.border, _v$50 = palette.sky;
                _v$45 !== _p$.e && (_p$.e = _$setProp(_el$105, "fg", _v$45, _p$.e));
                _v$46 !== _p$.t && (_p$.t = _$setProp(_el$107, "fg", _v$46, _p$.t));
                _v$47 !== _p$.a && (_p$.a = _$setProp(_el$108, "fg", _v$47, _p$.a));
                _v$48 !== _p$.o && (_p$.o = _$setProp(_el$109, "fg", _v$48, _p$.o));
                _v$49 !== _p$.i && (_p$.i = _$setProp(_el$110, "fg", _v$49, _p$.i));
                _v$50 !== _p$.n && (_p$.n = _$setProp(_el$112, "fg", _v$50, _p$.n));
                return _p$;
              }, {
                e: void 0,
                t: void 0,
                a: void 0,
                o: void 0,
                i: void 0,
                n: void 0
              });
              return _el$104;
            })()
          })];
        }
      }), null);
      _$insert(_el$97, _$createComponent(Show, {
        get when() {
          return props.metrics().cost !== void 0;
        },
        get children() {
          return [(() => {
            var _el$100 = _$createElement("text");
            _$insertNode(_el$100, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$100, "fg", palette.border, _$p));
            return _el$100;
          })(), (() => {
            var _el$102 = _$createElement("text");
            _$insert(_el$102, () => `$ ${formatCost(props.metrics().cost)}`);
            _$effect((_$p) => _$setProp(_el$102, "fg", palette.warning, _$p));
            return _el$102;
          })()];
        }
      }), null);
      return _el$97;
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
      var _el$113 = _$createElement("box"), _el$114 = _$createElement("text"), _el$115 = _$createElement("text"), _el$117 = _$createElement("text"), _el$119 = _$createElement("text"), _el$121 = _$createElement("text");
      _$insertNode(_el$113, _el$114);
      _$insertNode(_el$113, _el$115);
      _$insertNode(_el$113, _el$117);
      _$insertNode(_el$113, _el$119);
      _$insertNode(_el$113, _el$121);
      _$setProp(_el$113, "paddingLeft", 1);
      _$setProp(_el$113, "paddingRight", 1);
      _$setProp(_el$113, "flexDirection", "row");
      _$insert(_el$114, () => `${GLYPH.brand} `);
      _$insertNode(_el$115, _$createTextNode(`CORTEX`));
      _$insertNode(_el$117, _$createTextNode(`\xB7`));
      _$insertNode(_el$119, _$createTextNode(`IA `));
      _$insertNode(_el$121, _$createTextNode(`\u2502 `));
      _$insert(_el$113, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$123 = _$createElement("box"), _el$124 = _$createElement("text"), _el$125 = _$createElement("text"), _el$127 = _$createElement("text");
            _$insertNode(_el$123, _el$124);
            _$insertNode(_el$123, _el$125);
            _$insertNode(_el$123, _el$127);
            _$setProp(_el$123, "flexDirection", "row");
            _$insert(_el$124, () => `\u25CF ${counts().active} active`);
            _$insertNode(_el$125, _$createTextNode(` \xB7 `));
            _$insert(_el$127, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$123, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$128 = _$createElement("text");
                  _$insertNode(_el$128, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$128, "fg", palette.border, _$p));
                  return _el$128;
                })(), (() => {
                  var _el$130 = _$createElement("text");
                  _$insert(_el$130, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$130, "fg", palette.error, _$p));
                  return _el$130;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$56 = palette.warning, _v$57 = palette.border, _v$58 = palette.accent;
              _v$56 !== _p$.e && (_p$.e = _$setProp(_el$124, "fg", _v$56, _p$.e));
              _v$57 !== _p$.t && (_p$.t = _$setProp(_el$125, "fg", _v$57, _p$.t));
              _v$58 !== _p$.a && (_p$.a = _$setProp(_el$127, "fg", _v$58, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$123;
          })();
        },
        children: (task) => (() => {
          var _el$131 = _$createElement("box"), _el$132 = _$createElement("text"), _el$133 = _$createElement("text"), _el$134 = _$createElement("text"), _el$135 = _$createElement("text");
          _$insertNode(_el$131, _el$132);
          _$insertNode(_el$131, _el$133);
          _$insertNode(_el$131, _el$134);
          _$insertNode(_el$131, _el$135);
          _$setProp(_el$131, "flexDirection", "row");
          _$setProp(_el$131, "onMouseDown", () => openWebConsole(task().board_id, task().task_id));
          _$insert(_el$132, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$133, () => clipped(task().title, 20));
          _$insert(_el$134, () => ` \xB7 ${counts().active} active`);
          _$insert(_el$135, () => ` [${GLYPH.web}]`);
          _$effect((_p$) => {
            var _v$59 = palette.warning, _v$60 = palette.text, _v$61 = palette.textMuted, _v$62 = palette.info;
            _v$59 !== _p$.e && (_p$.e = _$setProp(_el$132, "fg", _v$59, _p$.e));
            _v$60 !== _p$.t && (_p$.t = _$setProp(_el$133, "fg", _v$60, _p$.t));
            _v$61 !== _p$.a && (_p$.a = _$setProp(_el$134, "fg", _v$61, _p$.a));
            _v$62 !== _p$.o && (_p$.o = _$setProp(_el$135, "fg", _v$62, _p$.o));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0,
            o: void 0
          });
          return _el$131;
        })()
      }), null);
      _$effect((_p$) => {
        var _v$51 = palette.accentAlt, _v$52 = palette.text, _v$53 = palette.info, _v$54 = palette.sky, _v$55 = palette.border;
        _v$51 !== _p$.e && (_p$.e = _$setProp(_el$114, "fg", _v$51, _p$.e));
        _v$52 !== _p$.t && (_p$.t = _$setProp(_el$115, "fg", _v$52, _p$.t));
        _v$53 !== _p$.a && (_p$.a = _$setProp(_el$117, "fg", _v$53, _p$.a));
        _v$54 !== _p$.o && (_p$.o = _$setProp(_el$119, "fg", _v$54, _p$.o));
        _v$55 !== _p$.i && (_p$.i = _$setProp(_el$121, "fg", _v$55, _p$.i));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0
      });
      return _el$113;
    }
  });
}
var KANBAN_GROUP_META = {
  in_progress: {
    label: "IN PROGRESS",
    glyph: GLYPH.working,
    header: "warning",
    id: "text",
    body: "textSoft"
  },
  in_review: {
    label: "IN REVIEW",
    glyph: GLYPH.active,
    header: "accent",
    id: "text",
    body: "textSoft"
  },
  done: {
    label: "DONE",
    glyph: GLYPH.done,
    header: "success",
    id: "sky",
    body: "textMuted"
  },
  blocked: {
    label: "BLK",
    glyph: GLYPH.fail,
    header: "error",
    id: "error",
    body: "error"
  }
};
var KANBAN_COLUMNS_WIDE = [["in_progress"], ["in_review"], ["done", "blocked"]];
var KANBAN_COLUMNS_MEDIUM = [["in_progress", "in_review"], ["done", "blocked"]];
var KANBAN_COLUMNS_STACKED = [["in_progress", "in_review", "done", "blocked"]];
function KanbanCard(props) {
  const meta = KANBAN_GROUP_META[props.kind];
  return (() => {
    var _el$136 = _$createElement("box"), _el$137 = _$createElement("text"), _el$138 = _$createElement("text");
    _$insertNode(_el$136, _el$137);
    _$insertNode(_el$136, _el$138);
    _$setProp(_el$136, "flexDirection", "column");
    _$setProp(_el$136, "marginTop", 1);
    _$setProp(_el$137, "wrapMode", "none");
    _$setProp(_el$137, "truncate", true);
    _$insert(_el$137, () => `${meta.glyph} ${props.task.task_id}`);
    _$setProp(_el$138, "wrapMode", "none");
    _$setProp(_el$138, "truncate", true);
    _$insert(_el$138, () => props.task.title);
    _$insert(_el$136, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.kind === "in_progress")() ? props.task.owner : void 0;
      },
      children: (owner) => (() => {
        var _el$141 = _$createElement("text");
        _$setProp(_el$141, "wrapMode", "none");
        _$setProp(_el$141, "truncate", true);
        _$insert(_el$141, () => `Claim: ${owner()}`);
        _$effect((_$p) => _$setProp(_el$141, "fg", props.palette.info, _$p));
        return _el$141;
      })()
    }), null);
    _$insert(_el$136, _$createComponent(Show, {
      get when() {
        return props.kind === "in_review";
      },
      get children() {
        var _el$139 = _$createElement("text");
        _$insertNode(_el$139, _$createTextNode(`awaiting reviewer`));
        _$setProp(_el$139, "wrapMode", "none");
        _$setProp(_el$139, "truncate", true);
        _$effect((_$p) => _$setProp(_el$139, "fg", props.palette.warning, _$p));
        return _el$139;
      }
    }), null);
    _$effect((_p$) => {
      var _v$63 = props.palette[meta.id], _v$64 = TextAttributes.BOLD, _v$65 = props.palette[meta.body];
      _v$63 !== _p$.e && (_p$.e = _$setProp(_el$137, "fg", _v$63, _p$.e));
      _v$64 !== _p$.t && (_p$.t = _$setProp(_el$137, "attributes", _v$64, _p$.t));
      _v$65 !== _p$.a && (_p$.a = _$setProp(_el$138, "fg", _v$65, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$136;
  })();
}
function KanbanGroup(props) {
  const meta = KANBAN_GROUP_META[props.kind];
  return (() => {
    var _el$142 = _$createElement("box"), _el$143 = _$createElement("text");
    _$insertNode(_el$142, _el$143);
    _$setProp(_el$142, "flexDirection", "column");
    _$setProp(_el$142, "marginBottom", 1);
    _$setProp(_el$143, "wrapMode", "none");
    _$setProp(_el$143, "truncate", true);
    _$insert(_el$143, () => `${meta.glyph} ${meta.label} (${props.tasks().length})`);
    _$insert(_el$142, _$createComponent(For, {
      get each() {
        return props.tasks();
      },
      children: (task) => _$createComponent(KanbanCard, {
        task,
        get kind() {
          return props.kind;
        },
        get palette() {
          return props.palette;
        }
      })
    }), null);
    _$effect((_p$) => {
      var _v$66 = props.palette[meta.header], _v$67 = TextAttributes.BOLD;
      _v$66 !== _p$.e && (_p$.e = _$setProp(_el$143, "fg", _v$66, _p$.e));
      _v$67 !== _p$.t && (_p$.t = _$setProp(_el$143, "attributes", _v$67, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$142;
  })();
}
function SessionKanbanPanel(props) {
  const tasks = createMemo(() => props.snapshot().tasks);
  const palette = resolvePalette(props.theme);
  const dimensions = useTerminalDimensions();
  const grouped = createMemo(() => {
    const all = tasks();
    return {
      in_progress: all.filter((t) => t.status === "in_progress"),
      in_review: all.filter((t) => t.status === "in_review"),
      done: all.filter((t) => t.status === "ready").slice(0, KANBAN_READY_LIMIT),
      blocked: all.filter((t) => t.status === "blocked")
    };
  });
  const columns = createMemo(() => {
    const width = measuredColumns(dimensions().width);
    if (width >= NARROW_TERMINAL_COLS) return KANBAN_COLUMNS_WIDE;
    if (width >= KANBAN_TWO_COLUMN_COLS) return KANBAN_COLUMNS_MEDIUM;
    return KANBAN_COLUMNS_STACKED;
  });
  const columnWidth = createMemo(() => {
    const count = columns().length;
    const available = Math.max(0, measuredColumns(dimensions().width) - 2);
    const usable = available - (count - 1) * KANBAN_COLUMN_SEPARATOR;
    return Math.max(KANBAN_MIN_COLUMN_WIDTH, Math.floor(usable / count));
  });
  return (() => {
    var _el$144 = _$createElement("box"), _el$145 = _$createElement("box"), _el$146 = _$createElement("text"), _el$147 = _$createElement("text"), _el$148 = _$createElement("box");
    _$insertNode(_el$144, _el$145);
    _$insertNode(_el$144, _el$148);
    _$setProp(_el$144, "flexDirection", "column");
    _$setProp(_el$144, "padding", 1);
    _$insertNode(_el$145, _el$146);
    _$insertNode(_el$145, _el$147);
    _$setProp(_el$145, "flexDirection", "row");
    _$setProp(_el$145, "marginBottom", 1);
    _$setProp(_el$146, "wrapMode", "none");
    _$setProp(_el$146, "truncate", true);
    _$insert(_el$146, () => `${GLYPH.web} CORTEX \xB7 IA KANBAN DECK [${props.pulse()}] `);
    _$insert(_el$147, () => `(${tasks().length} tasks \xB7 ${props.jobs().length} workers)`);
    _$setProp(_el$148, "flexDirection", "row");
    _$insert(_el$148, _$createComponent(For, {
      get each() {
        return columns();
      },
      children: (column, index) => (() => {
        var _el$149 = _$createElement("box");
        _$setProp(_el$149, "flexDirection", "column");
        _$insert(_el$149, _$createComponent(For, {
          each: column,
          children: (kind) => _$createComponent(KanbanGroup, {
            kind,
            tasks: () => grouped()[kind],
            palette
          })
        }));
        _$effect((_p$) => {
          var _v$71 = columnWidth(), _v$72 = index() > 0 ? KANBAN_COLUMN_BORDER : false, _v$73 = palette.border;
          _v$71 !== _p$.e && (_p$.e = _$setProp(_el$149, "width", _v$71, _p$.e));
          _v$72 !== _p$.t && (_p$.t = _$setProp(_el$149, "border", _v$72, _p$.t));
          _v$73 !== _p$.a && (_p$.a = _$setProp(_el$149, "borderColor", _v$73, _p$.a));
          return _p$;
        }, {
          e: void 0,
          t: void 0,
          a: void 0
        });
        return _el$149;
      })()
    }));
    _$effect((_p$) => {
      var _v$68 = palette.info, _v$69 = TextAttributes.BOLD, _v$70 = palette.textMuted;
      _v$68 !== _p$.e && (_p$.e = _$setProp(_el$146, "fg", _v$68, _p$.e));
      _v$69 !== _p$.t && (_p$.t = _$setProp(_el$146, "attributes", _v$69, _p$.t));
      _v$70 !== _p$.a && (_p$.a = _$setProp(_el$147, "fg", _v$70, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$144;
  })();
}
var CORTEX_LOGO_BRAILLE = ["       \u28E0\u28F6\u28FF\u28FF\u28FF\u28FF\u28F6\u28E4\u2840       \u2880\u28E4\u28F6\u28FF\u28FF\u28FF\u28FF\u28F6\u28C4", "    \u28B0\u28FF\u28FF\u281F\u2809   \u2819\u28BF\u28FF\u28F7\u2840   \u28A0\u28FE\u28FF\u287F\u280B   \u2808\u283B\u28FF\u28FF\u2846", "   \u28A0\u28FF\u28FF\u280B  \u2880\u28E4\u28E4\u28C0  \u2839\u28FF\u28FF\u28C4\u28E0\u28FF\u28FF\u280F  \u28C0\u28E4\u28E4\u2840  \u2819\u28FF\u28FF\u2844", "   \u28FE\u28FF\u2803  \u28B0\u28FF\u28FF\u28FF\u28FF\u28F7\u2840 \u2839\u28FF\u28FF\u28FF\u28FF\u280F \u28A0\u28FE\u28FF\u28FF\u28FF\u28FF\u2846  \u2818\u28FF\u28F7", "  \u28B8\u28FF\u285F   \u2838\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u28C6 \u2839\u28FF\u28FF\u280F \u28F0\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u2807   \u28BB\u28FF\u2847", "  \u2818\u28FF\u28E7    \u2808\u281B\u283F\u28FF\u28FF\u28FF\u28FF\u28F7\u28C4\u2819\u280B\u28E0\u28FE\u28FF\u28FF\u28FF\u28FF\u283F\u281B\u2801    \u28FC\u28FF\u2803", "   \u2839\u28FF\u28E7\u2840     \u2808\u2819\u283F\u28FF\u28FF\u28FF\u2846\u28B0\u28FF\u28FF\u28FF\u283F\u280B\u2801     \u2880\u28FC\u28FF\u280F", "    \u2819\u28BF\u28FF\u28E6\u2840   \u2880\u28E0\u28F4\u28FF\u28FF\u28FF\u2847\u28B8\u28FF\u28FF\u28FF\u28E6\u28C4\u2840   \u2880\u28F4\u28FF\u287F\u280B", "      \u2809\u281B\u283F\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u287F\u281B\u2801 \u2808\u281B\u28BF\u28FF\u28FF\u28FF\u28FF\u28FF\u28FF\u283F\u281B\u2809", "  \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2557  \u2588\u2588\u2557     \u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2557 ", " \u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255D\u2588\u2588\u2554\u2550\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u255A\u2550\u2550\u2588\u2588\u2554\u2550\u2550\u255D\u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255D\u255A\u2588\u2588\u2557\u2588\u2588\u2554\u255D     \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557", " \u2588\u2588\u2551     \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255D   \u2588\u2588\u2551   \u2588\u2588\u2588\u2588\u2588\u2557   \u255A\u2588\u2588\u2588\u2554\u255D\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2551", " \u2588\u2588\u2551     \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557   \u2588\u2588\u2551   \u2588\u2588\u2554\u2550\u2550\u255D   \u2588\u2588\u2554\u2588\u2588\u2557\u255A\u2550\u2550\u2550\u2550\u255D\u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2551", " \u255A\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u255A\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255D\u2588\u2588\u2551  \u2588\u2588\u2551   \u2588\u2588\u2551   \u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2554\u255D \u2588\u2588\u2557     \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551", "  \u255A\u2550\u2550\u2550\u2550\u2550\u255D \u255A\u2550\u2550\u2550\u2550\u2550\u255D \u255A\u2550\u255D  \u255A\u2550\u255D   \u255A\u2550\u255D   \u255A\u2550\u2550\u2550\u2550\u2550\u2550\u255D\u255A\u2550\u255D  \u255A\u2550\u255D     \u255A\u2550\u255D\u255A\u2550\u255D  \u255A\u2550\u255D"];
var CORTEX_LOGO_GRADIENT = ["accent", "accentAlt", "primary", "sky"];
var CORTEX_LOGO_GRADIENT_STOPS = [4, 9, 12];
function logoLineColor(index, palette) {
  const stop = CORTEX_LOGO_GRADIENT_STOPS.findIndex((limit) => index < limit);
  const step = stop === -1 ? CORTEX_LOGO_GRADIENT_STOPS.length : stop;
  return palette[CORTEX_LOGO_GRADIENT[step]];
}
function HomeLogo(props) {
  const dim = useTerminalDimensions();
  const palette = resolvePalette(props.theme);
  const isLarge = createMemo(() => {
    const d = dim();
    return d.height >= CORTEX_LOGO_BRAILLE.length + 5 && d.width >= 72;
  });
  return (() => {
    var _el$150 = _$createElement("box");
    _$setProp(_el$150, "flexDirection", "column");
    _$setProp(_el$150, "alignItems", "center");
    _$setProp(_el$150, "marginBottom", 1);
    _$insert(_el$150, _$createComponent(Show, {
      get when() {
        return isLarge();
      },
      get fallback() {
        return (() => {
          var _el$152 = _$createElement("box"), _el$153 = _$createElement("text"), _el$154 = _$createElement("text");
          _$insertNode(_el$152, _el$153);
          _$insertNode(_el$152, _el$154);
          _$setProp(_el$152, "flexDirection", "column");
          _$setProp(_el$152, "alignItems", "center");
          _$insert(_el$153, () => `${GLYPH.active} CORTEX \xB7 IA ${GLYPH.active}`);
          _$insertNode(_el$154, _$createTextNode(`[Adaptive Cognitive Control Plane]`));
          _$effect((_p$) => {
            var _v$74 = palette.info, _v$75 = TextAttributes.BOLD, _v$76 = palette.textMuted;
            _v$74 !== _p$.e && (_p$.e = _$setProp(_el$153, "fg", _v$74, _p$.e));
            _v$75 !== _p$.t && (_p$.t = _$setProp(_el$153, "attributes", _v$75, _p$.t));
            _v$76 !== _p$.a && (_p$.a = _$setProp(_el$154, "fg", _v$76, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$152;
        })();
      },
      get children() {
        return [_$createComponent(For, {
          each: CORTEX_LOGO_BRAILLE,
          children: (line, index) => {
            return (() => {
              var _el$156 = _$createElement("text");
              _$insert(_el$156, line);
              _$effect((_$p) => _$setProp(_el$156, "fg", logoLineColor(index(), palette), _$p));
              return _el$156;
            })();
          }
        }), (() => {
          var _el$151 = _$createElement("text");
          _$setProp(_el$151, "marginTop", 1);
          _$insert(_el$151, () => `${GLYPH.active} OpenCode Multi-Agent Control Plane & Task DAG ${GLYPH.active}`);
          _$effect((_$p) => _$setProp(_el$151, "fg", palette.textMuted, _$p));
          return _el$151;
        })()];
      }
    }));
    return _el$150;
  })();
}
function formatLargeTokens(value) {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`;
  if (value >= 1e6) return `${(value / 1e6).toFixed(1)}M`;
  if (value >= 1e3) return `${(value / 1e3).toFixed(1)}k`;
  return String(Math.round(value));
}
function formatInteger(n) {
  return String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}
function openStatsView() {
  try {
    if (process.platform === "win32") {
      spawn("cmd.exe", ["/c", "start", "Cortex Stats", cortexExecutable(), "stats"], {
        detached: true,
        stdio: "ignore"
      }).unref();
    } else {
      spawn(cortexExecutable(), ["stats"], {
        detached: true,
        stdio: "ignore"
      }).unref();
    }
  } catch {
  }
}
function HomeStatsWidget(props) {
  const palette = resolvePalette(props.theme);
  const data = props.stats;
  const dim = useTerminalDimensions();
  return _$createComponent(Show, {
    get when() {
      return data();
    },
    children: (s) => {
      const isNarrow = () => dim().width < 80;
      return (() => {
        var _el$157 = _$createElement("box"), _el$158 = _$createElement("box"), _el$159 = _$createElement("box"), _el$160 = _$createElement("text"), _el$161 = _$createElement("text"), _el$164 = _$createElement("box"), _el$165 = _$createElement("text"), _el$166 = _$createElement("text"), _el$168 = _$createElement("text"), _el$169 = _$createElement("text"), _el$171 = _$createElement("text"), _el$175 = _$createElement("box"), _el$176 = _$createElement("text"), _el$177 = _$createElement("text"), _el$179 = _$createElement("text"), _el$183 = _$createElement("text");
        _$insertNode(_el$157, _el$158);
        _$insertNode(_el$157, _el$164);
        _$insertNode(_el$157, _el$175);
        _$setProp(_el$157, "flexDirection", "column");
        _$setProp(_el$157, "borderStyle", "rounded");
        _$setProp(_el$157, "paddingLeft", 1);
        _$setProp(_el$157, "paddingRight", 1);
        _$setProp(_el$157, "marginTop", 1);
        _$setProp(_el$157, "marginBottom", 1);
        _$setProp(_el$157, "width", "100%");
        _$setProp(_el$157, "onMouseDown", () => openStatsView());
        _$insertNode(_el$158, _el$159);
        _$setProp(_el$158, "flexDirection", "row");
        _$setProp(_el$158, "justifyContent", "space-between");
        _$insertNode(_el$159, _el$160);
        _$insertNode(_el$159, _el$161);
        _$setProp(_el$159, "flexDirection", "row");
        _$insert(_el$160, () => `${GLYPH.brand} CORTEX \xB7 IA `);
        _$insertNode(_el$161, _$createTextNode(`Estad\xEDsticas de Uso`));
        _$insert(_el$158, _$createComponent(Show, {
          get when() {
            return _$memo(() => !!s().first_day)() && s().last_day;
          },
          get children() {
            var _el$163 = _$createElement("text");
            _$insert(_el$163, () => `${s().first_day} \u2192 ${s().last_day}`);
            _$effect((_$p) => _$setProp(_el$163, "fg", palette.textMuted, _$p));
            return _el$163;
          }
        }), null);
        _$insertNode(_el$164, _el$165);
        _$insertNode(_el$164, _el$166);
        _$insertNode(_el$164, _el$168);
        _$insertNode(_el$164, _el$169);
        _$insertNode(_el$164, _el$171);
        _$setProp(_el$164, "flexDirection", "row");
        _$setProp(_el$164, "marginTop", 0);
        _$insert(_el$165, () => `\u25CF ${formatInteger(s().sessions)} sesiones`);
        _$insertNode(_el$166, _$createTextNode(`\u2502`));
        _$insert(_el$168, () => `\u2709 ${formatInteger(s().messages)} mensajes`);
        _$insertNode(_el$169, _$createTextNode(`\u2502`));
        _$insert(_el$171, () => `\u25C6 ${formatLargeTokens(s().tokens)} tokens`);
        _$insert(_el$164, _$createComponent(Show, {
          get when() {
            return !isNarrow();
          },
          get children() {
            return [(() => {
              var _el$172 = _$createElement("text");
              _$insertNode(_el$172, _$createTextNode(`\u2502`));
              _$effect((_$p) => _$setProp(_el$172, "fg", palette.border, _$p));
              return _el$172;
            })(), (() => {
              var _el$174 = _$createElement("text");
              _$insert(_el$174, () => `\u{1F4C5} ${s().active_days} d\xEDas activos`);
              _$effect((_$p) => _$setProp(_el$174, "fg", palette.warning, _$p));
              return _el$174;
            })()];
          }
        }), null);
        _$insertNode(_el$175, _el$176);
        _$insertNode(_el$175, _el$177);
        _$insertNode(_el$175, _el$179);
        _$insertNode(_el$175, _el$183);
        _$setProp(_el$175, "flexDirection", "row");
        _$setProp(_el$175, "marginTop", 0);
        _$insert(_el$176, () => `\u2605 Top: ${s().favorite_model || "-"} (${(s().favorite_model_share ?? 0).toFixed(1)}%)`);
        _$insertNode(_el$177, _$createTextNode(`\u2502`));
        _$insert(_el$179, () => `\u26A1 Pico: ${String(s().peak_hour).padStart(2, "0")}:00`);
        _$insert(_el$175, _$createComponent(Show, {
          get when() {
            return isNarrow();
          },
          get children() {
            return [(() => {
              var _el$180 = _$createElement("text");
              _$insertNode(_el$180, _$createTextNode(`\u2502`));
              _$effect((_$p) => _$setProp(_el$180, "fg", palette.border, _$p));
              return _el$180;
            })(), (() => {
              var _el$182 = _$createElement("text");
              _$insert(_el$182, () => `\u{1F4C5} ${s().active_days}d`);
              _$effect((_$p) => _$setProp(_el$182, "fg", palette.warning, _$p));
              return _el$182;
            })()];
          }
        }), _el$183);
        _$insertNode(_el$183, _$createTextNode(` \xB7 [:cortex-stats para panel interactivo]`));
        _$effect((_p$) => {
          var _v$77 = palette.border, _v$78 = palette.accent, _v$79 = TextAttributes.BOLD, _v$80 = palette.text, _v$81 = TextAttributes.BOLD, _v$82 = isNarrow() ? 1 : 2, _v$83 = palette.sky, _v$84 = palette.border, _v$85 = palette.info, _v$86 = palette.border, _v$87 = palette.success, _v$88 = TextAttributes.BOLD, _v$89 = isNarrow() ? 1 : 2, _v$90 = palette.accentAlt, _v$91 = palette.border, _v$92 = palette.sky, _v$93 = palette.textMuted;
          _v$77 !== _p$.e && (_p$.e = _$setProp(_el$157, "borderColor", _v$77, _p$.e));
          _v$78 !== _p$.t && (_p$.t = _$setProp(_el$160, "fg", _v$78, _p$.t));
          _v$79 !== _p$.a && (_p$.a = _$setProp(_el$160, "attributes", _v$79, _p$.a));
          _v$80 !== _p$.o && (_p$.o = _$setProp(_el$161, "fg", _v$80, _p$.o));
          _v$81 !== _p$.i && (_p$.i = _$setProp(_el$161, "attributes", _v$81, _p$.i));
          _v$82 !== _p$.n && (_p$.n = _$setProp(_el$164, "gap", _v$82, _p$.n));
          _v$83 !== _p$.s && (_p$.s = _$setProp(_el$165, "fg", _v$83, _p$.s));
          _v$84 !== _p$.h && (_p$.h = _$setProp(_el$166, "fg", _v$84, _p$.h));
          _v$85 !== _p$.r && (_p$.r = _$setProp(_el$168, "fg", _v$85, _p$.r));
          _v$86 !== _p$.d && (_p$.d = _$setProp(_el$169, "fg", _v$86, _p$.d));
          _v$87 !== _p$.l && (_p$.l = _$setProp(_el$171, "fg", _v$87, _p$.l));
          _v$88 !== _p$.u && (_p$.u = _$setProp(_el$171, "attributes", _v$88, _p$.u));
          _v$89 !== _p$.c && (_p$.c = _$setProp(_el$175, "gap", _v$89, _p$.c));
          _v$90 !== _p$.w && (_p$.w = _$setProp(_el$176, "fg", _v$90, _p$.w));
          _v$91 !== _p$.m && (_p$.m = _$setProp(_el$177, "fg", _v$91, _p$.m));
          _v$92 !== _p$.f && (_p$.f = _$setProp(_el$179, "fg", _v$92, _p$.f));
          _v$93 !== _p$.y && (_p$.y = _$setProp(_el$183, "fg", _v$93, _p$.y));
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
          c: void 0,
          w: void 0,
          m: void 0,
          f: void 0,
          y: void 0
        });
        return _el$157;
      })();
    }
  });
}
function nanModelTokens(window, model) {
  if (!window || !Array.isArray(window.byModel)) return 0;
  for (const entry of window.byModel) {
    if (entry?.model !== model) continue;
    const input = typeof entry.inputTokens === "number" ? entry.inputTokens : 0;
    const output = typeof entry.outputTokens === "number" ? entry.outputTokens : 0;
    return input + output;
  }
  return 0;
}
function nanDailySeries(timeSeries, limit) {
  if (!Array.isArray(timeSeries)) return [];
  const totals = /* @__PURE__ */ new Map();
  for (const point of timeSeries) {
    if (typeof point?.date !== "string" || point.date === "") continue;
    const input = typeof point.inputTokens === "number" ? point.inputTokens : 0;
    const output = typeof point.outputTokens === "number" ? point.outputTokens : 0;
    totals.set(point.date, (totals.get(point.date) ?? 0) + input + output);
  }
  return [...totals.entries()].sort((a, b) => a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0).slice(-limit).map(([date, tokens]) => ({
    date,
    tokens
  }));
}
function burnSparkline(values, points) {
  const window = values.slice(-points);
  const peak = Math.max(0, ...window);
  const top = ETA_SPARK_LEVELS.length - 1;
  return window.map((value) => ETA_SPARK_LEVELS[peak > 0 ? Math.max(0, Math.min(top, Math.round(value / peak * top))) : 0]).join("");
}
var NAN_REASONING_OFF_LEVELS = ["none", "minimal"];
function nanCatalogModel(entry) {
  const meta = entry.meta;
  const quota = typeof meta.monthlyQuotaTokens === "number" && Number.isFinite(meta.monthlyQuotaTokens) ? meta.monthlyQuotaTokens : 0;
  return {
    model: entry.model,
    tier: typeof meta.tier === "string" ? meta.tier : "",
    quota,
    rolling4hRef: typeof meta.rolling4hRefTokens === "number" ? meta.rolling4hRefTokens : 0,
    vocabulary: Array.isArray(meta.effortVocabulary) ? meta.effortVocabulary.filter((level) => typeof level === "string") : [],
    effortMode: typeof meta.effortMode === "string" ? meta.effortMode : "",
    effortAdjustable: meta.effortAdjustable === true,
    preferredOver: typeof meta.preferredOver === "string" ? meta.preferredOver : ""
  };
}
function agentModelOption(entry) {
  if (typeof entry?.agent !== "string" || entry.agent === "") return void 0;
  const model = typeof entry.model === "string" && entry.model !== "" ? entry.model : "";
  const variant = typeof entry.variant === "string" && entry.variant !== "" ? `#${entry.variant}` : "";
  return {
    agent: entry.agent,
    value: model ? `${model}${variant}` : "(unset)"
  };
}
function nanEffortModeBadge(mode) {
  if (mode === "adaptive") return `${GLYPH.active} adaptive \xB7 depth auto`;
  if (mode === "accepted-not-adjustable") return `${GLYPH.warn} accepted \xB7 depth fixed`;
  if (mode === "adjustable") return `${GLYPH.done} adjustable`;
  return `${GLYPH.idle} posture unknown`;
}
function nanSetReceiptLines(stdout) {
  try {
    const receipt = JSON.parse(stdout);
    const lines = [`agent    ${receipt.agent || "-"}`, `action   ${receipt.action || "-"}${receipt.dry_run ? " (dry-run)" : ""}`, `previous ${receipt.previous || "(unset)"}`, `value    ${receipt.value || "(unset)"}`, `changed  ${receipt.changed === true}`, `managed  ${receipt.managed === true}`];
    if (receipt.restored === true) lines.push("restored true");
    if (Array.isArray(receipt.warnings)) for (const warning of receipt.warnings) lines.push(`warning  ${warning}`);
    return lines;
  } catch {
    return [];
  }
}
function nanCommandFailure(error, stderr) {
  const text = (stderr || error.message || "").trim();
  return text ? text.split(/\r?\n/)[0] : "command failed";
}
function NanUsageStrip(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.view();
    },
    children: (view) => (() => {
      var _el$185 = _$createElement("box"), _el$186 = _$createElement("text"), _el$187 = _$createElement("text"), _el$189 = _$createElement("text"), _el$191 = _$createElement("text");
      _$insertNode(_el$185, _el$186);
      _$insertNode(_el$185, _el$187);
      _$insertNode(_el$185, _el$189);
      _$insertNode(_el$185, _el$191);
      _$setProp(_el$185, "flexDirection", "row");
      _$setProp(_el$185, "paddingLeft", 1);
      _$setProp(_el$185, "paddingRight", 1);
      _$insert(_el$186, () => `${GLYPH.active} nan `);
      _$insert(_el$185, _$createComponent(Show, {
        get when() {
          return view().percent !== void 0;
        },
        get fallback() {
          return (() => {
            var _el$192 = _$createElement("text");
            _$insertNode(_el$192, _$createTextNode(`quota n/a `));
            _$effect((_$p) => _$setProp(_el$192, "fg", palette.textMuted, _$p));
            return _el$192;
          })();
        },
        get children() {
          return _$createComponent(MultiColorProgressBar, {
            get done() {
              return view().percent ?? 0;
            },
            inReview: 0,
            inProgress: 0,
            total: 100,
            width: 10,
            compact: true,
            get textLimit() {
              return props.textLimit;
            },
            label: "MTD: ",
            get theme() {
              return props.theme;
            }
          });
        }
      }), _el$187);
      _$insertNode(_el$187, _$createTextNode(` \xB7 `));
      _$insertNode(_el$189, _$createTextNode(`24h `));
      _$insert(_el$191, () => formatLargeTokens(view().burn));
      _$effect((_p$) => {
        var _v$94 = palette.accentAlt, _v$95 = palette.border, _v$96 = palette.textMuted, _v$97 = palette.sky;
        _v$94 !== _p$.e && (_p$.e = _$setProp(_el$186, "fg", _v$94, _p$.e));
        _v$95 !== _p$.t && (_p$.t = _$setProp(_el$187, "fg", _v$95, _p$.t));
        _v$96 !== _p$.a && (_p$.a = _$setProp(_el$189, "fg", _v$96, _p$.a));
        _v$97 !== _p$.o && (_p$.o = _$setProp(_el$191, "fg", _v$97, _p$.o));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0
      });
      return _el$185;
    })()
  });
}
function NanModelPickerPanel(props) {
  const palette = resolvePalette(props.theme);
  const [modelID, setModelID] = createSignal("");
  const [level, setLevel] = createSignal("");
  const [agentID, setAgentID] = createSignal("");
  const agents = () => props.agents() ?? [];
  const activeModel = createMemo(() => props.models().find((m) => m.model === modelID()) || props.models()[0]);
  const activeAgent = createMemo(() => agents().find((a) => a.agent === agentID()) || agents()[0]);
  const needsEffort = createMemo(() => Boolean(activeModel()?.effortAdjustable && (activeModel()?.vocabulary.length ?? 0) > 0));
  const ready = createMemo(() => Boolean(activeModel() && activeAgent() && (!needsEffort() || level() !== "")));
  const dispatch = (apply) => {
    const model = activeModel();
    const agent = activeAgent();
    if (!ready() || !model || !agent) return;
    const chosen = needsEffort() ? level() : "";
    if (apply) props.onApply(agent.agent, model.model, chosen);
    else props.onPreview(agent.agent, model.model, chosen);
  };
  return (() => {
    var _el$194 = _$createElement("box"), _el$195 = _$createElement("text"), _el$196 = _$createElement("text");
    _$insertNode(_el$194, _el$195);
    _$insertNode(_el$194, _el$196);
    _$setProp(_el$194, "flexDirection", "column");
    _$setProp(_el$194, "padding", 1);
    _$insert(_el$195, () => `${GLYPH.web} nan model picker \xB7 browse only, preview then apply`);
    _$insertNode(_el$196, _$createTextNode(`pick a model, an effort, and an agent \xB7 the ref form is nan/&lt;model>`));
    _$insert(_el$194, _$createComponent(Show, {
      get when() {
        return props.loadError() !== "";
      },
      get children() {
        var _el$198 = _$createElement("text");
        _$insert(_el$198, () => `catalog unavailable (${props.loadError()}) \xB7 no assignment attempted`);
        _$effect((_$p) => _$setProp(_el$198, "fg", palette.error, _$p));
        return _el$198;
      }
    }), null);
    _$insert(_el$194, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.loading();
      },
      get children() {
        var _el$199 = _$createElement("text");
        _$insertNode(_el$199, _$createTextNode(`reading nan catalog\u2026`));
        _$effect((_$p) => _$setProp(_el$199, "fg", palette.textMuted, _$p));
        return _el$199;
      }
    }), null);
    _$insert(_el$194, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.loadError() === "" && !props.loading()))() && props.models().length === 0;
      },
      get children() {
        var _el$201 = _$createElement("text");
        _$insertNode(_el$201, _$createTextNode(`no picker-eligible nan chat models in the catalog`));
        _$effect((_$p) => _$setProp(_el$201, "fg", palette.textMuted, _$p));
        return _el$201;
      }
    }), null);
    _$insert(_el$194, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.models().length > 0;
      },
      get children() {
        return [(() => {
          var _el$203 = _$createElement("text");
          _$insertNode(_el$203, _$createTextNode(`model`));
          _$effect((_p$) => {
            var _v$98 = palette.textSoft, _v$99 = TextAttributes.BOLD;
            _v$98 !== _p$.e && (_p$.e = _$setProp(_el$203, "fg", _v$98, _p$.e));
            _v$99 !== _p$.t && (_p$.t = _$setProp(_el$203, "attributes", _v$99, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$203;
        })(), _$createComponent(For, {
          get each() {
            return props.models();
          },
          children: (model) => (() => {
            var _el$224 = _$createElement("box"), _el$225 = _$createElement("text"), _el$226 = _$createElement("text"), _el$227 = _$createElement("text");
            _$insertNode(_el$224, _el$225);
            _$insertNode(_el$224, _el$226);
            _$insertNode(_el$224, _el$227);
            _$setProp(_el$224, "flexDirection", "row");
            _$setProp(_el$224, "onMouseDown", () => {
              setModelID(model.model);
              setLevel("");
              props.onSelectionChange();
            });
            _$insert(_el$225, () => activeModel()?.model === model.model ? "\u25B8 " : "  ");
            _$insert(_el$226, () => `nan/${model.model}`);
            _$insert(_el$227, () => ` \xB7 ${nanEffortModeBadge(model.effortMode)}`);
            _$insert(_el$224, _$createComponent(Show, {
              get when() {
                return model.preferredOver !== "";
              },
              get children() {
                var _el$228 = _$createElement("text");
                _$insert(_el$228, () => ` \xB7 preferred over ${model.preferredOver}`);
                _$effect((_$p) => _$setProp(_el$228, "fg", palette.success, _$p));
                return _el$228;
              }
            }), null);
            _$insert(_el$224, _$createComponent(Show, {
              get when() {
                return model.tier === "legacy";
              },
              get children() {
                var _el$229 = _$createElement("text");
                _$insertNode(_el$229, _$createTextNode(` \xB7 legacy \xB7 deprioritized`));
                _$effect((_$p) => _$setProp(_el$229, "fg", palette.warning, _$p));
                return _el$229;
              }
            }), null);
            _$effect((_p$) => {
              var _v$113 = palette.border, _v$114 = model.tier === "legacy" ? palette.textMuted : palette.text, _v$115 = palette.textMuted;
              _v$113 !== _p$.e && (_p$.e = _$setProp(_el$225, "fg", _v$113, _p$.e));
              _v$114 !== _p$.t && (_p$.t = _$setProp(_el$226, "fg", _v$114, _p$.t));
              _v$115 !== _p$.a && (_p$.a = _$setProp(_el$227, "fg", _v$115, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$224;
          })()
        }), (() => {
          var _el$205 = _$createElement("text");
          _$insertNode(_el$205, _$createTextNode(`effort`));
          _$effect((_p$) => {
            var _v$100 = palette.textSoft, _v$101 = TextAttributes.BOLD;
            _v$100 !== _p$.e && (_p$.e = _$setProp(_el$205, "fg", _v$100, _p$.e));
            _v$101 !== _p$.t && (_p$.t = _$setProp(_el$205, "attributes", _v$101, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$205;
        })(), _$createComponent(Show, {
          get when() {
            return needsEffort();
          },
          get fallback() {
            return (() => {
              var _el$231 = _$createElement("text");
              _$insert(_el$231, () => `no adjustable depth for nan/${activeModel()?.model ?? ""} \xB7 the reference is written without --effort`);
              _$effect((_$p) => _$setProp(_el$231, "fg", palette.textMuted, _$p));
              return _el$231;
            })();
          },
          get children() {
            var _el$207 = _$createElement("box");
            _$setProp(_el$207, "flexDirection", "row");
            _$insert(_el$207, _$createComponent(For, {
              get each() {
                return activeModel()?.vocabulary ?? [];
              },
              children: (option) => (() => {
                var _el$232 = _$createElement("text");
                _$setProp(_el$232, "onMouseDown", () => {
                  setLevel(option);
                  props.onSelectionChange();
                });
                _$insert(_el$232, () => `[${level() === option ? "\u25B8" : " "} ${option}${NAN_REASONING_OFF_LEVELS.includes(option) ? " \xB7skips reasoning" : ""}] `);
                _$effect((_$p) => _$setProp(_el$232, "fg", level() === option ? palette.success : palette.text, _$p));
                return _el$232;
              })()
            }));
            return _el$207;
          }
        }), (() => {
          var _el$208 = _$createElement("text");
          _$insert(_el$208, () => `agent \xB7 effective mapping (${agents().length})`);
          _$effect((_p$) => {
            var _v$102 = palette.textSoft, _v$103 = TextAttributes.BOLD;
            _v$102 !== _p$.e && (_p$.e = _$setProp(_el$208, "fg", _v$102, _p$.e));
            _v$103 !== _p$.t && (_p$.t = _$setProp(_el$208, "attributes", _v$103, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$208;
        })(), _$createComponent(For, {
          get each() {
            return agents();
          },
          children: (agent) => (() => {
            var _el$233 = _$createElement("text");
            _$setProp(_el$233, "onMouseDown", () => {
              setAgentID(agent.agent);
              props.onSelectionChange();
            });
            _$insert(_el$233, () => `[${activeAgent()?.agent === agent.agent ? "\u25B8" : " "} ${agent.agent}: ${agent.value}] `);
            _$effect((_$p) => _$setProp(_el$233, "fg", activeAgent()?.agent === agent.agent ? palette.info : palette.textMuted, _$p));
            return _el$233;
          })()
        }), _$createComponent(Show, {
          get when() {
            return _$memo(() => !!!props.loading())() && agents().length === 0;
          },
          get children() {
            var _el$209 = _$createElement("text");
            _$insertNode(_el$209, _$createTextNode(`agent list unavailable \xB7 confirmation is disabled`));
            _$effect((_$p) => _$setProp(_el$209, "fg", palette.warning, _$p));
            return _el$209;
          }
        }), (() => {
          var _el$211 = _$createElement("box"), _el$212 = _$createElement("text"), _el$216 = _$createElement("text");
          _$insertNode(_el$211, _el$212);
          _$insertNode(_el$211, _el$216);
          _$setProp(_el$211, "flexDirection", "row");
          _$setProp(_el$211, "marginTop", 1);
          _$insertNode(_el$212, _$createTextNode(`[ preview dry-run ]`));
          _$setProp(_el$212, "onMouseDown", () => dispatch(false));
          _$insert(_el$211, _$createComponent(Show, {
            get when() {
              return props.preview() !== "";
            },
            get children() {
              var _el$214 = _$createElement("text");
              _$insertNode(_el$214, _$createTextNode(`  [ apply ]`));
              _$setProp(_el$214, "onMouseDown", () => dispatch(true));
              _$effect((_$p) => _$setProp(_el$214, "fg", palette.success, _$p));
              return _el$214;
            }
          }), _el$216);
          _$insert(_el$216, (() => {
            var _c$2 = _$memo(() => props.phase() === "previewing");
            return () => _c$2() ? "  running dry-run\u2026" : props.phase() === "applying" ? "  applying\u2026" : "";
          })());
          _$effect((_p$) => {
            var _v$104 = ready() ? palette.info : palette.textMuted, _v$105 = palette.textMuted;
            _v$104 !== _p$.e && (_p$.e = _$setProp(_el$212, "fg", _v$104, _p$.e));
            _v$105 !== _p$.t && (_p$.t = _$setProp(_el$216, "fg", _v$105, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$211;
        })(), _$createComponent(Show, {
          get when() {
            return props.runError() !== "";
          },
          get children() {
            var _el$217 = _$createElement("text");
            _$insert(_el$217, () => `model set failed \xB7 ${props.runError()} \u2014 the effective mapping above is unchanged`);
            _$effect((_$p) => _$setProp(_el$217, "fg", palette.error, _$p));
            return _el$217;
          }
        }), _$createComponent(Show, {
          get when() {
            return props.preview() !== "";
          },
          get children() {
            return [(() => {
              var _el$218 = _$createElement("text");
              _$insertNode(_el$218, _$createTextNode(`dry-run receipt (nothing written)`));
              _$effect((_p$) => {
                var _v$106 = palette.textSoft, _v$107 = TextAttributes.BOLD;
                _v$106 !== _p$.e && (_p$.e = _$setProp(_el$218, "fg", _v$106, _p$.e));
                _v$107 !== _p$.t && (_p$.t = _$setProp(_el$218, "attributes", _v$107, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$218;
            })(), (() => {
              var _el$220 = _$createElement("text");
              _$insert(_el$220, () => props.preview());
              _$effect((_$p) => _$setProp(_el$220, "fg", palette.textMuted, _$p));
              return _el$220;
            })()];
          }
        }), _$createComponent(Show, {
          get when() {
            return props.result() !== "";
          },
          get children() {
            return [(() => {
              var _el$221 = _$createElement("text");
              _$insertNode(_el$221, _$createTextNode(`applied receipt`));
              _$effect((_p$) => {
                var _v$108 = palette.success, _v$109 = TextAttributes.BOLD;
                _v$108 !== _p$.e && (_p$.e = _$setProp(_el$221, "fg", _v$108, _p$.e));
                _v$109 !== _p$.t && (_p$.t = _$setProp(_el$221, "attributes", _v$109, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$221;
            })(), (() => {
              var _el$223 = _$createElement("text");
              _$insert(_el$223, () => props.result());
              _$effect((_$p) => _$setProp(_el$223, "fg", palette.text, _$p));
              return _el$223;
            })()];
          }
        })];
      }
    }), null);
    _$effect((_p$) => {
      var _v$110 = palette.accent, _v$111 = TextAttributes.BOLD, _v$112 = palette.textMuted;
      _v$110 !== _p$.e && (_p$.e = _$setProp(_el$195, "fg", _v$110, _p$.e));
      _v$111 !== _p$.t && (_p$.t = _$setProp(_el$195, "attributes", _v$111, _p$.t));
      _v$112 !== _p$.a && (_p$.a = _$setProp(_el$196, "fg", _v$112, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$194;
  })();
}
function NanDetailPanel(props) {
  const palette = resolvePalette(props.theme);
  const dim = useTerminalDimensions();
  const sparkline = createMemo(() => burnSparkline(props.days().map((point) => point.tokens), NAN_DETAIL_DAY_POINTS));
  const rollingRefs = createMemo(() => props.rows().filter((row) => row.rolling4hRef > 0));
  const textLimit = createMemo(() => Math.max(24, measuredColumns(dim().width) - 8));
  const days = () => props.days();
  return (() => {
    var _el$234 = _$createElement("box"), _el$235 = _$createElement("text");
    _$insertNode(_el$234, _el$235);
    _$setProp(_el$234, "flexDirection", "column");
    _$setProp(_el$234, "padding", 1);
    _$insert(_el$235, () => `${GLYPH.active} nan usage detail \xB7 daily granularity`);
    _$insert(_el$234, _$createComponent(Show, {
      get when() {
        return props.loadError() !== "";
      },
      get children() {
        var _el$236 = _$createElement("text");
        _$insert(_el$236, () => props.loadError());
        _$effect((_$p) => _$setProp(_el$236, "fg", palette.error, _$p));
        return _el$236;
      }
    }), null);
    _$insert(_el$234, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.loading();
      },
      get children() {
        var _el$237 = _$createElement("text");
        _$insertNode(_el$237, _$createTextNode(`reading nan metrics\u2026`));
        _$effect((_$p) => _$setProp(_el$237, "fg", palette.textMuted, _$p));
        return _el$237;
      }
    }), null);
    _$insert(_el$234, _$createComponent(Show, {
      get when() {
        return days().length > 0;
      },
      get children() {
        var _el$239 = _$createElement("box"), _el$240 = _$createElement("text"), _el$242 = _$createElement("text"), _el$243 = _$createElement("text");
        _$insertNode(_el$239, _el$240);
        _$insertNode(_el$239, _el$242);
        _$insertNode(_el$239, _el$243);
        _$setProp(_el$239, "flexDirection", "row");
        _$setProp(_el$239, "marginTop", 1);
        _$insertNode(_el$240, _$createTextNode(`daily tokens `));
        _$insert(_el$242, sparkline);
        _$insert(_el$243, () => `  ${days()[0].date} \u2192 ${days()[days().length - 1].date}`);
        _$effect((_p$) => {
          var _v$116 = palette.textSoft, _v$117 = palette.accentAlt, _v$118 = palette.textMuted;
          _v$116 !== _p$.e && (_p$.e = _$setProp(_el$240, "fg", _v$116, _p$.e));
          _v$117 !== _p$.t && (_p$.t = _$setProp(_el$242, "fg", _v$117, _p$.t));
          _v$118 !== _p$.a && (_p$.a = _$setProp(_el$243, "fg", _v$118, _p$.a));
          return _p$;
        }, {
          e: void 0,
          t: void 0,
          a: void 0
        });
        return _el$239;
      }
    }), null);
    _$insert(_el$234, _$createComponent(Show, {
      get when() {
        return props.rows().length > 0;
      },
      get children() {
        return [(() => {
          var _el$244 = _$createElement("text");
          _$insertNode(_el$244, _$createTextNode(`month-to-date against monthly quota`));
          _$setProp(_el$244, "marginTop", 1);
          _$effect((_p$) => {
            var _v$119 = palette.textSoft, _v$120 = TextAttributes.BOLD;
            _v$119 !== _p$.e && (_p$.e = _$setProp(_el$244, "fg", _v$119, _p$.e));
            _v$120 !== _p$.t && (_p$.t = _$setProp(_el$244, "attributes", _v$120, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$244;
        })(), _$createComponent(For, {
          get each() {
            return props.rows();
          },
          children: (row) => (() => {
            var _el$248 = _$createElement("box"), _el$249 = _$createElement("text"), _el$250 = _$createElement("text");
            _$insertNode(_el$248, _el$249);
            _$insertNode(_el$248, _el$250);
            _$setProp(_el$248, "flexDirection", "row");
            _$insert(_el$249, () => `${(row.model + "                    ").slice(0, 20)}`);
            _$insert(_el$250, () => `${formatLargeTokens(row.monthToDate).padStart(8, " ")}`);
            _$insert(_el$248, _$createComponent(Show, {
              get when() {
                return row.quota > 0;
              },
              get fallback() {
                return (() => {
                  var _el$252 = _$createElement("text");
                  _$insertNode(_el$252, _$createTextNode(`  quota unknown`));
                  _$effect((_$p) => _$setProp(_el$252, "fg", palette.textMuted, _$p));
                  return _el$252;
                })();
              },
              get children() {
                return [(() => {
                  var _el$251 = _$createElement("text");
                  _$insert(_el$251, () => ` / ${formatLargeTokens(row.quota)} `);
                  _$effect((_$p) => _$setProp(_el$251, "fg", palette.textMuted, _$p));
                  return _el$251;
                })(), _$createComponent(MultiColorProgressBar, {
                  get done() {
                    return Math.round(row.monthToDate / row.quota * 100);
                  },
                  inReview: 0,
                  inProgress: 0,
                  total: 100,
                  width: 12,
                  compact: true,
                  get textLimit() {
                    return textLimit();
                  },
                  get theme() {
                    return props.theme;
                  }
                })];
              }
            }), null);
            _$effect((_p$) => {
              var _v$123 = palette.text, _v$124 = palette.sky;
              _v$123 !== _p$.e && (_p$.e = _$setProp(_el$249, "fg", _v$123, _p$.e));
              _v$124 !== _p$.t && (_p$.t = _$setProp(_el$250, "fg", _v$124, _p$.t));
              return _p$;
            }, {
              e: void 0,
              t: void 0
            });
            return _el$248;
          })()
        })];
      }
    }), null);
    _$insert(_el$234, _$createComponent(For, {
      get each() {
        return rollingRefs();
      },
      children: (row) => (() => {
        var _el$254 = _$createElement("text");
        _$setProp(_el$254, "marginTop", 1);
        _$insert(_el$254, () => `reference only: ${row.model} rolling-4h allowance ${formatLargeTokens(row.rolling4hRef)} tokens \u2014 daily-granularity data cannot express a rolling window`);
        _$effect((_$p) => _$setProp(_el$254, "fg", palette.warning, _$p));
        return _el$254;
      })()
    }), null);
    _$insert(_el$234, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.loadError() === "" && !props.loading() && days().length === 0))() && props.rows().length === 0;
      },
      get children() {
        var _el$246 = _$createElement("text");
        _$insertNode(_el$246, _$createTextNode(`no nan usage data available`));
        _$effect((_$p) => _$setProp(_el$246, "fg", palette.textMuted, _$p));
        return _el$246;
      }
    }), null);
    _$effect((_p$) => {
      var _v$121 = palette.info, _v$122 = TextAttributes.BOLD;
      _v$121 !== _p$.e && (_p$.e = _$setProp(_el$235, "fg", _v$121, _p$.e));
      _v$122 !== _p$.t && (_p$.t = _$setProp(_el$235, "attributes", _v$122, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$234;
  })();
}
function CortexAgentsRow(props) {
  const row = props.row;
  const palette = props.palette;
  const statusCore = () => {
    if (row.retryAttempt) return `${GLYPH.warn}${row.retryAttempt}`;
    if (row.status === "busy") return props.pulse();
    if (row.status === "idle") return GLYPH.idle;
    if (row.status === "unknown") return GLYPH.active;
    return GLYPH.working;
  };
  const activity = () => row.tool || row.step || (row.status === "busy" ? "working" : "");
  const elapsed = () => row.startedAt ? formatElapsedClock(props.now() - row.startedAt) : "--:--";
  const taskBadge = createMemo(() => row.taskID ? `[${shortID(row.taskID)}\xB7${row.taskStatus || "unknown"}]` : "");
  return (() => {
    var _el$255 = _$createElement("box"), _el$256 = _$createElement("text"), _el$257 = _$createElement("text"), _el$258 = _$createElement("text"), _el$259 = _$createElement("text"), _el$260 = _$createElement("text"), _el$262 = _$createElement("text"), _el$263 = _$createElement("text");
    _$insertNode(_el$255, _el$256);
    _$insertNode(_el$255, _el$257);
    _$insertNode(_el$255, _el$258);
    _$insertNode(_el$255, _el$259);
    _$insertNode(_el$255, _el$260);
    _$insertNode(_el$255, _el$262);
    _$insertNode(_el$255, _el$263);
    _$setProp(_el$255, "flexDirection", "row");
    _$setProp(_el$256, "selectable", false);
    _$insert(_el$256, () => `${props.active ? GLYPH.row : " "} `);
    _$setProp(_el$257, "selectable", false);
    _$insert(_el$257, () => `${fitWidth(statusCore(), AGENTS_STATUS_WIDTH)} `);
    _$setProp(_el$258, "selectable", false);
    _$insert(_el$258, () => `${fitWidth(row.agent || "agent", props.layout().agent)} `);
    _$insert(_el$259, () => fitWidth(row.title, props.layout().title));
    _$setProp(_el$260, "selectable", false);
    _$insert(_el$260, () => ` ${fitWidth(elapsed(), AGENTS_ELAPSED_WIDTH)}`);
    _$insert(_el$255, _$createComponent(Show, {
      get when() {
        return props.layout().activity > 0;
      },
      get children() {
        var _el$261 = _$createElement("text");
        _$setProp(_el$261, "selectable", false);
        _$insert(_el$261, () => ` ${fitWidth(activity(), props.layout().activity)}`);
        _$effect((_$p) => _$setProp(_el$261, "fg", palette.info, _$p));
        return _el$261;
      }
    }), _el$262);
    _$setProp(_el$262, "selectable", false);
    _$insert(_el$262, () => ` ${fitWidth(row.tokens === void 0 ? "-" : formatTokens(row.tokens), AGENTS_TOKENS_WIDTH)}`);
    _$setProp(_el$263, "selectable", false);
    _$insert(_el$263, () => ` ${fitWidth(row.cost === void 0 ? "-" : `$${formatCost(row.cost)}`, AGENTS_COST_WIDTH)}`);
    _$insert(_el$255, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.layout().task > 0)() && taskBadge() !== "";
      },
      get children() {
        var _el$264 = _$createElement("text");
        _$setProp(_el$264, "selectable", false);
        _$insert(_el$264, () => ` ${fitWidth(taskBadge(), props.layout().task)}`);
        _$effect((_$p) => _$setProp(_el$264, "fg", taskStatusChip(row.taskStatus || "backlog", palette).color, _$p));
        return _el$264;
      }
    }), null);
    _$effect((_p$) => {
      var _v$125 = props.active ? palette.element : palette.background, _v$126 = props.onActivate, _v$127 = props.active ? palette.accent : palette.textMuted, _v$128 = subagentStatusColor(row.status, palette), _v$129 = palette.accentAlt, _v$130 = props.active ? palette.text : palette.textSoft, _v$131 = palette.textSoft, _v$132 = palette.sky, _v$133 = palette.warning;
      _v$125 !== _p$.e && (_p$.e = _$setProp(_el$255, "backgroundColor", _v$125, _p$.e));
      _v$126 !== _p$.t && (_p$.t = _$setProp(_el$255, "onMouseDown", _v$126, _p$.t));
      _v$127 !== _p$.a && (_p$.a = _$setProp(_el$256, "fg", _v$127, _p$.a));
      _v$128 !== _p$.o && (_p$.o = _$setProp(_el$257, "fg", _v$128, _p$.o));
      _v$129 !== _p$.i && (_p$.i = _$setProp(_el$258, "fg", _v$129, _p$.i));
      _v$130 !== _p$.n && (_p$.n = _$setProp(_el$259, "fg", _v$130, _p$.n));
      _v$131 !== _p$.s && (_p$.s = _$setProp(_el$260, "fg", _v$131, _p$.s));
      _v$132 !== _p$.h && (_p$.h = _$setProp(_el$262, "fg", _v$132, _p$.h));
      _v$133 !== _p$.r && (_p$.r = _$setProp(_el$263, "fg", _v$133, _p$.r));
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
    return _el$255;
  })();
}
function CortexAgentsPanel(props) {
  const palette = resolvePalette(props.theme);
  const dim = useTerminalDimensions();
  const layout = createMemo(() => {
    const measured = Math.floor(Number(dim().width) || 0) || measuredColumns();
    const total = Math.max(AGENTS_MIN_COLUMNS, measured);
    const narrow = total < AGENTS_NARROW_COLUMNS;
    const agent = narrow ? AGENTS_NARROW_AGENT_WIDTH : AGENTS_AGENT_WIDTH;
    const activity = narrow ? 0 : AGENTS_ACTIVITY_WIDTH;
    const task = narrow ? 0 : AGENTS_TASK_WIDTH;
    const fixed = AGENTS_STATUS_WIDTH + 1 + agent + 1 + AGENTS_ELAPSED_WIDTH + 1 + AGENTS_TOKENS_WIDTH + 1 + AGENTS_COST_WIDTH + (activity > 0 ? activity + 1 : 0) + (task > 0 ? task + 1 : 0);
    return {
      agent,
      activity,
      task,
      title: Math.max(AGENTS_TITLE_MIN, total - fixed - 6)
    };
  });
  const running = createMemo(() => props.rows().filter((row) => row.status === "busy" || row.status === "retry").length);
  const header = () => (() => {
    var _el$265 = _$createElement("box"), _el$266 = _$createElement("text"), _el$267 = _$createElement("text"), _el$268 = _$createElement("text"), _el$269 = _$createElement("text"), _el$270 = _$createElement("text"), _el$272 = _$createElement("text"), _el$273 = _$createElement("text");
    _$insertNode(_el$265, _el$266);
    _$insertNode(_el$265, _el$267);
    _$insertNode(_el$265, _el$268);
    _$insertNode(_el$265, _el$269);
    _$insertNode(_el$265, _el$270);
    _$insertNode(_el$265, _el$272);
    _$insertNode(_el$265, _el$273);
    _$setProp(_el$265, "flexDirection", "row");
    _$setProp(_el$266, "selectable", false);
    _$insert(_el$266, () => `${fitWidth("", 1)} `);
    _$setProp(_el$267, "selectable", false);
    _$insert(_el$267, () => `${fitWidth("st", AGENTS_STATUS_WIDTH)} `);
    _$setProp(_el$268, "selectable", false);
    _$insert(_el$268, () => `${fitWidth("agent", layout().agent)} `);
    _$setProp(_el$269, "selectable", false);
    _$insert(_el$269, () => fitWidth("objective", layout().title));
    _$setProp(_el$270, "selectable", false);
    _$insert(_el$270, () => ` ${fitWidth("elapsed", AGENTS_ELAPSED_WIDTH)}`);
    _$insert(_el$265, _$createComponent(Show, {
      get when() {
        return layout().activity > 0;
      },
      get children() {
        var _el$271 = _$createElement("text");
        _$setProp(_el$271, "selectable", false);
        _$insert(_el$271, () => ` ${fitWidth("activity", layout().activity)}`);
        _$effect((_$p) => _$setProp(_el$271, "fg", palette.textMuted, _$p));
        return _el$271;
      }
    }), _el$272);
    _$setProp(_el$272, "selectable", false);
    _$insert(_el$272, () => ` ${fitWidth("tokens", AGENTS_TOKENS_WIDTH)}`);
    _$setProp(_el$273, "selectable", false);
    _$insert(_el$273, () => ` ${fitWidth("cost", AGENTS_COST_WIDTH)}`);
    _$insert(_el$265, _$createComponent(Show, {
      get when() {
        return layout().task > 0;
      },
      get children() {
        var _el$274 = _$createElement("text");
        _$setProp(_el$274, "selectable", false);
        _$insert(_el$274, () => ` ${fitWidth("task", layout().task)}`);
        _$effect((_$p) => _$setProp(_el$274, "fg", palette.textMuted, _$p));
        return _el$274;
      }
    }), null);
    _$effect((_p$) => {
      var _v$134 = palette.textMuted, _v$135 = palette.textMuted, _v$136 = palette.textMuted, _v$137 = palette.textMuted, _v$138 = palette.textMuted, _v$139 = palette.textMuted, _v$140 = palette.textMuted;
      _v$134 !== _p$.e && (_p$.e = _$setProp(_el$266, "fg", _v$134, _p$.e));
      _v$135 !== _p$.t && (_p$.t = _$setProp(_el$267, "fg", _v$135, _p$.t));
      _v$136 !== _p$.a && (_p$.a = _$setProp(_el$268, "fg", _v$136, _p$.a));
      _v$137 !== _p$.o && (_p$.o = _$setProp(_el$269, "fg", _v$137, _p$.o));
      _v$138 !== _p$.i && (_p$.i = _$setProp(_el$270, "fg", _v$138, _p$.i));
      _v$139 !== _p$.n && (_p$.n = _$setProp(_el$272, "fg", _v$139, _p$.n));
      _v$140 !== _p$.s && (_p$.s = _$setProp(_el$273, "fg", _v$140, _p$.s));
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
    return _el$265;
  })();
  return (() => {
    var _el$275 = _$createElement("box"), _el$277 = _$createElement("box"), _el$278 = _$createElement("text");
    _$insertNode(_el$275, _el$277);
    _$setProp(_el$275, "flexDirection", "column");
    _$setProp(_el$275, "borderStyle", "rounded");
    _$setProp(_el$275, "titleAlignment", "left");
    _$setProp(_el$275, "paddingLeft", 1);
    _$setProp(_el$275, "paddingRight", 1);
    _$insert(_el$275, header, _el$277);
    _$insert(_el$275, _$createComponent(Show, {
      get when() {
        return props.error() !== "";
      },
      get children() {
        var _el$276 = _$createElement("text");
        _$insert(_el$276, () => clipped(props.error(), 72));
        _$effect((_$p) => _$setProp(_el$276, "fg", palette.warning, _$p));
        return _el$276;
      }
    }), _el$277);
    _$insert(_el$275, _$createComponent(Show, {
      get when() {
        return props.rows().length > 0;
      },
      get fallback() {
        return _$createComponent(EmptyState, {
          palette,
          get label() {
            return props.loading() ? "loading subagent sessions" : "no subagent sessions";
          }
        });
      },
      get children() {
        return _$createComponent(For, {
          get each() {
            return props.rows();
          },
          children: (row, index) => _$createComponent(CortexAgentsRow, {
            row,
            get active() {
              return props.selected() === index();
            },
            get now() {
              return props.now;
            },
            get pulse() {
              return props.pulse;
            },
            layout,
            palette,
            onActivate: () => props.onActivate(index())
          })
        });
      }
    }), _el$277);
    _$insertNode(_el$277, _el$278);
    _$setProp(_el$277, "flexDirection", "row");
    _$setProp(_el$277, "marginTop", 1);
    _$insertNode(_el$278, _$createTextNode(`open enter \xB7 close esc \xB7 parent up`));
    _$setProp(_el$278, "selectable", false);
    _$effect((_p$) => {
      var _v$141 = palette.primary, _v$142 = `${GLYPH.brand} SUBAGENTS [${running()} running \xB7 ${props.rows().length} total]`, _v$143 = palette.accent, _v$144 = palette.background, _v$145 = palette.textMuted;
      _v$141 !== _p$.e && (_p$.e = _$setProp(_el$275, "borderColor", _v$141, _p$.e));
      _v$142 !== _p$.t && (_p$.t = _$setProp(_el$275, "title", _v$142, _p$.t));
      _v$143 !== _p$.a && (_p$.a = _$setProp(_el$275, "titleColor", _v$143, _p$.a));
      _v$144 !== _p$.o && (_p$.o = _$setProp(_el$275, "backgroundColor", _v$144, _p$.o));
      _v$145 !== _p$.i && (_p$.i = _$setProp(_el$278, "fg", _v$145, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
    });
    return _el$275;
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
  let consecutiveSnapshotFailures = 0;
  const [now, setNow] = createSignal(Date.now());
  const [frame, setFrame] = createSignal(0);
  const [pulseFrame, setPulseFrame] = createSignal(0);
  const getPref = (key, fallback) => {
    if (api.kv && typeof api.kv.get === "function") {
      return api.kv.get(key, fallback) !== false;
    }
    return fallback;
  };
  const setPref = (key, value) => {
    if (api.kv && typeof api.kv.set === "function") {
      api.kv.set(key, value);
    }
  };
  const showToast = (title, message, variant) => {
    const toastApi = api.ui?.toast || api.toast;
    if (!toastApi) return;
    try {
      if (typeof toastApi.show === "function") {
        toastApi.show({
          title,
          message,
          variant
        });
      } else if (typeof toastApi === "function") {
        toastApi({
          title,
          message,
          variant
        });
      }
    } catch {
    }
  };
  const terminalColumns = () => {
    const width = api.renderer?.width;
    if (typeof width === "number" && Number.isFinite(width) && width > 0) return width;
    const columns = process.stdout?.columns;
    return typeof columns === "number" && Number.isFinite(columns) && columns > 0 ? columns : 0;
  };
  const storedDensity = () => {
    if (!api.kv || typeof api.kv.get !== "function") return void 0;
    const raw = api.kv.get(DENSITY_KEY, void 0);
    return raw === "compact" || raw === "expanded" ? raw : void 0;
  };
  const initialDensity = () => {
    const stored = storedDensity();
    if (stored) return stored;
    const columns = terminalColumns();
    return columns > 0 && columns < NARROW_TERMINAL_COLS ? "compact" : "expanded";
  };
  const startingDensity = initialDensity();
  const [density, setDensity] = createSignal(startingDensity);
  const [tasksExpanded, setTasksExpanded] = createSignal(getPref(TASKS_EXPANDED_KEY, startingDensity === "expanded"));
  const [delegationsExpanded, setDelegationsExpanded] = createSignal(getPref(DELEGATIONS_EXPANDED_KEY, startingDensity === "expanded"));
  const [attentionExpanded, setAttentionExpanded] = createSignal(getPref(ATTENTION_EXPANDED_KEY, startingDensity === "expanded"));
  const setSectionExpanded = (key, setter, next) => {
    setter(next);
    setPref(key, next);
  };
  const applyDensity = (next) => {
    setDensity(next);
    setPref(DENSITY_KEY, next);
    const open = next === "expanded";
    setSectionExpanded(TASKS_EXPANDED_KEY, setTasksExpanded, open);
    setSectionExpanded(DELEGATIONS_EXPANDED_KEY, setDelegationsExpanded, open);
    setSectionExpanded(ATTENTION_EXPANDED_KEY, setAttentionExpanded, open);
  };
  const toggleDensity = () => {
    const next = density() === "compact" ? "expanded" : "compact";
    applyDensity(next);
    showToast(next === "compact" ? `${MARK_COLLAPSED} Compact density` : `${MARK_EXPANDED} Expanded density`, next === "compact" ? "Sections collapsed to one line" : "Sections expanded with full detail", "info");
  };
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
      estimateMs: mean === void 0 ? void 0 : mean * remaining,
      sparkline: etaSparkline(history)
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
      consecutiveSnapshotFailures = 0;
    }
    if (!scope || !scope.project || pendingGeneration === generation) return;
    const requestGeneration = generation;
    pendingGeneration = requestGeneration;
    const recordFailure = (message) => {
      consecutiveSnapshotFailures += 1;
      if (consecutiveSnapshotFailures >= 2) setSnapshotError(message);
    };
    const runAttempt = (retryAllowed) => {
      execFile(cortexExecutable(), ["ui", "snapshot", "--project", scope.project, "--session-id", scope.sessionID, "--root-session-id", scope.rootSessionID], {
        encoding: "utf8",
        maxBuffer: 512 * 1024,
        timeout: 7500,
        windowsHide: true
      }, (error, stdout) => {
        if (pendingGeneration === requestGeneration) pendingGeneration = void 0;
        if (disposed || requestGeneration !== generation || JSON.stringify(conversationScope(api, activeSessionOverride())) !== key) return;
        if (error) {
          if (error.killed === true && retryAllowed) {
            pendingGeneration = requestGeneration;
            runAttempt(false);
            return;
          }
          recordFailure(error.message);
          return;
        }
        try {
          const next = JSON.parse(stdout);
          if (next.schema_version !== 2 || next.requested_session_id !== scope.sessionID || next.root_session_id !== scope.rootSessionID || !Array.isArray(next.tasks) || !Array.isArray(next.delegations) || !Array.isArray(next.attention) || !next.counts || !next.summary) {
            throw new Error("snapshot schema or conversation is incompatible");
          }
          consecutiveSnapshotFailures = 0;
          setSnapshot(next);
          setSnapshotError("");
        } catch (parseError) {
          recordFailure(parseError instanceof Error ? parseError.message : "invalid snapshot JSON");
        }
      });
    };
    runAttempt(true);
  };
  createEffect(() => {
    const count = operationalCounts(snapshot(), snapshotError()).attention;
    if (count > previousAttentionCount && density() === "expanded") {
      setAttentionExpanded(true);
      setPref(ATTENTION_EXPANDED_KEY, true);
    }
    previousAttentionCount = count;
  });
  let previousTaskStatuses = /* @__PURE__ */ new Map();
  createEffect(() => {
    const tasks = snapshot().tasks;
    if (previousTaskStatuses.size > 0) {
      for (const t of tasks) {
        const prev = previousTaskStatuses.get(t.task_id);
        if (prev && prev !== t.status) {
          let variant = "info";
          let title = `Cortex-IA: Task ${t.task_id}`;
          if (t.status === "done") {
            variant = "success";
            title = `${GLYPH.done} ${t.task_id} Approved (PASS)`;
          } else if (t.status === "blocked") {
            variant = "error";
            title = `${GLYPH.fail} ${t.task_id} Blocked`;
          } else if (t.status === "in_review") {
            variant = "warning";
            title = `${GLYPH.active} ${t.task_id} In Review`;
          } else if (t.status === "in_progress") {
            variant = "info";
            title = `${GLYPH.working} ${t.task_id} Claimed`;
          }
          showToast(title, clipped(t.title, 40), variant);
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
  const [usageStats, setUsageStats] = createSignal(null);
  const [statsLoading, setStatsLoading] = createSignal(false);
  let pendingStats = false;
  const readUsageStats = () => {
    if (disposed || pendingStats) return;
    pendingStats = true;
    setStatsLoading(true);
    execFile(cortexExecutable(), ["stats", "--json"], {
      encoding: "utf8",
      maxBuffer: 512 * 1024,
      timeout: 5e3,
      windowsHide: true
    }, (error, stdout) => {
      pendingStats = false;
      setStatsLoading(false);
      if (disposed || error) return;
      try {
        const parsed = JSON.parse(stdout);
        if (typeof parsed?.sessions === "number" && typeof parsed?.messages === "number") {
          setUsageStats(parsed);
        }
      } catch {
      }
    });
  };
  readUsageStats();
  const statsPoll = setInterval(readUsageStats, 6e4);
  const [nanUsage, setNanUsage] = createSignal(null);
  const [nanCatalog, setNanCatalog] = createSignal(void 0);
  let pendingNanUsage = false;
  let pendingNanCatalog = false;
  let nanScopeKey = "";
  let nanGeneration = 0;
  const currentNanModel = () => {
    const record = sessionRecord(api, currentSessionID(api, activeSessionOverride()));
    const model = record?.model;
    if (!model || model.providerID !== "nan" || typeof model.id !== "string") return void 0;
    return model.id.split("#")[0];
  };
  const readNanCatalog = () => {
    if (disposed || pendingNanCatalog) return;
    const cached = nanCatalog();
    if (cached && Date.now() - cached.cachedAt < NAN_CATALOG_TTL_MS) return;
    pendingNanCatalog = true;
    execFile(cortexExecutable(), ["model", "catalog", "--json", "--provider", "nan"], {
      encoding: "utf8",
      maxBuffer: NAN_EXEC_MAX_BUFFER,
      timeout: NAN_EXEC_TIMEOUT_MS,
      windowsHide: true
    }, (error, stdout) => {
      pendingNanCatalog = false;
      if (disposed) return;
      if (error) {
        setNanCatalogFailed(true);
        return;
      }
      try {
        const parsed = JSON.parse(stdout);
        if (!Array.isArray(parsed?.entries)) throw new Error("catalog entries");
        const index = /* @__PURE__ */ new Map();
        const models = [];
        for (const entry of parsed.entries) {
          const meta = entry?.meta;
          if (entry?.provider !== "nan" || !meta || meta.pickerEligible !== true) continue;
          if (typeof entry.model !== "string" || entry.model === "") continue;
          if (typeof meta.monthlyQuotaTokens === "number" && Number.isFinite(meta.monthlyQuotaTokens)) index.set(entry.model, meta.monthlyQuotaTokens);
          models.push(nanCatalogModel({
            model: entry.model,
            meta
          }));
        }
        models.sort((left, right) => (left.tier === "legacy" ? 1 : 0) - (right.tier === "legacy" ? 1 : 0));
        setNanCatalog({
          cachedAt: Date.now(),
          index,
          models
        });
        setNanCatalogFailed(false);
      } catch {
        setNanCatalogFailed(true);
      }
    });
  };
  const readNanUsage = () => {
    if (disposed) return;
    const sessionID = currentSessionID(api, activeSessionOverride()) || "";
    if (sessionID !== nanScopeKey) {
      nanScopeKey = sessionID;
      nanGeneration += 1;
      setNanUsage(null);
    }
    if (pendingNanUsage) return;
    const model = currentNanModel();
    if (!model) return;
    pendingNanUsage = true;
    const requestGeneration = nanGeneration;
    readNanCatalog();
    execFile(nanExecutable(), ["metrics", "usage"], {
      encoding: "utf8",
      maxBuffer: NAN_EXEC_MAX_BUFFER,
      timeout: NAN_EXEC_TIMEOUT_MS,
      windowsHide: true
    }, (error, stdout) => {
      pendingNanUsage = false;
      if (disposed || requestGeneration !== nanGeneration) return;
      if (error) {
        setNanUsage(null);
        return;
      }
      try {
        const metrics = JSON.parse(stdout);
        if (!Array.isArray(metrics?.monthToDate?.byModel) || !Array.isArray(metrics?.last24h?.byModel)) {
          setNanUsage(null);
          return;
        }
        setNanUsage({
          model,
          monthToDate: nanModelTokens(metrics.monthToDate, model),
          last24h: nanModelTokens(metrics.last24h, model)
        });
      } catch {
        setNanUsage(null);
      }
    });
  };
  createEffect(() => {
    activeSessionOverride();
    untrack(readNanUsage);
  });
  const nanPoll = setInterval(readNanUsage, NAN_USAGE_POLL_INTERVAL_MS);
  const nanStrip = createMemo(() => {
    activeSessionOverride();
    const usage = nanUsage();
    if (!usage) return void 0;
    const model = currentNanModel();
    if (!model || model !== usage.model) return void 0;
    const quota = nanCatalog()?.index.get(model);
    if (quota === void 0) return void 0;
    return {
      percent: quota > 0 ? Math.min(100, Math.max(0, Math.round(usage.monthToDate / quota * 100))) : void 0,
      burn: usage.last24h
    };
  });
  const [nanCatalogFailed, setNanCatalogFailed] = createSignal(false);
  const [modelAgents, setModelAgents] = createSignal(void 0);
  const [pickerLoading, setPickerLoading] = createSignal(false);
  const [pickerPhase, setPickerPhase] = createSignal("idle");
  const [pickerPreview, setPickerPreview] = createSignal("");
  const [pickerResult, setPickerResult] = createSignal("");
  const [pickerRunError, setPickerRunError] = createSignal("");
  let pendingModelSet = false;
  const [nanDetailMetrics, setNanDetailMetrics] = createSignal(void 0);
  const [nanDetailLoading, setNanDetailLoading] = createSignal(false);
  const [nanDetailError, setNanDetailError] = createSignal("");
  let pendingNanDetail = false;
  const pickerLoadError = createMemo(() => nanCatalogFailed() ? "catalog JSON failed" : "");
  const readModelAgents = (onSettled) => {
    if (disposed) {
      onSettled();
      return;
    }
    execFile(cortexExecutable(), ["model", "list", "--json"], {
      encoding: "utf8",
      maxBuffer: NAN_EXEC_MAX_BUFFER,
      timeout: NAN_EXEC_TIMEOUT_MS,
      windowsHide: true
    }, (error, stdout) => {
      if (disposed) return;
      let options;
      if (!error) {
        try {
          const parsed = JSON.parse(stdout);
          if (Array.isArray(parsed?.agents)) {
            const seen = /* @__PURE__ */ new Set();
            options = [];
            for (const entry of parsed.agents) {
              const option = agentModelOption(entry);
              if (!option || seen.has(option.agent)) continue;
              seen.add(option.agent);
              options.push(option);
            }
          }
        } catch {
          options = void 0;
        }
      }
      setModelAgents(options);
      onSettled();
    });
  };
  const loadModelPicker = () => {
    if (disposed) return;
    setNanCatalogFailed(false);
    setModelAgents(void 0);
    setPickerPreview("");
    setPickerResult("");
    setPickerRunError("");
    setPickerPhase("idle");
    setPickerLoading(true);
    readNanCatalog();
    readModelAgents(() => {
      if (!disposed) setPickerLoading(false);
    });
  };
  const runModelSet = (agentName, modelName, level, dryRun) => {
    if (disposed || pendingModelSet) return;
    const args = ["model", "set", agentName, `nan/${modelName}`];
    if (level) args.push("--effort", level);
    if (dryRun) args.push("--dry-run");
    args.push("--json");
    pendingModelSet = true;
    if (dryRun) setPickerPhase("previewing");
    else {
      setPickerPhase("applying");
      setPickerRunError("");
    }
    execFile(cortexExecutable(), args, {
      encoding: "utf8",
      maxBuffer: NAN_EXEC_MAX_BUFFER,
      timeout: NAN_EXEC_TIMEOUT_MS,
      windowsHide: true
    }, (error, stdout, stderr) => {
      pendingModelSet = false;
      if (disposed) return;
      if (error) {
        setPickerPhase(dryRun ? "idle" : "ready");
        setPickerRunError(nanCommandFailure(error, stderr));
        return;
      }
      const receipt = nanSetReceiptLines(stdout).join("\n");
      if (dryRun) {
        setPickerPreview(receipt);
        setPickerPhase("ready");
        return;
      }
      setPickerResult(receipt);
      setPickerPhase("applied");
      readModelAgents(() => {
      });
    });
  };
  const loadNanDetails = () => {
    if (disposed || pendingNanDetail) return;
    pendingNanDetail = true;
    setNanDetailLoading(true);
    setNanDetailError("");
    readNanCatalog();
    execFile(nanExecutable(), ["metrics", "usage"], {
      encoding: "utf8",
      maxBuffer: NAN_EXEC_MAX_BUFFER,
      timeout: NAN_EXEC_TIMEOUT_MS,
      windowsHide: true
    }, (error, stdout) => {
      pendingNanDetail = false;
      if (disposed) return;
      setNanDetailLoading(false);
      if (error) {
        setNanDetailError("nan metrics unavailable");
        return;
      }
      try {
        const metrics = JSON.parse(stdout);
        if (!Array.isArray(metrics?.timeSeries) || !Array.isArray(metrics?.monthToDate?.byModel)) {
          setNanDetailError("nan metrics payload was not understood");
          return;
        }
        setNanDetailMetrics(metrics);
      } catch {
        setNanDetailError("nan metrics payload was not understood");
      }
    });
  };
  const nanDetailDays = createMemo(() => nanDailySeries(nanDetailMetrics()?.timeSeries, NAN_DETAIL_DAY_POINTS));
  const nanDetailRows = createMemo(() => {
    const window = nanDetailMetrics()?.monthToDate;
    const rows = [];
    for (const entry of window?.byModel ?? []) {
      if (typeof entry?.model !== "string" || entry.model === "") continue;
      const meta = nanCatalog()?.models.find((candidate) => candidate.model === entry.model);
      rows.push({
        model: entry.model,
        monthToDate: nanModelTokens(window, entry.model),
        quota: meta?.quota ?? 0,
        rolling4hRef: meta?.rolling4hRef ?? 0
      });
    }
    rows.sort((left, right) => right.monthToDate - left.monthToDate);
    return rows;
  });
  const [agentsOpen, setAgentsOpen] = createSignal(false);
  const [agentsChildren, setAgentsChildren] = createSignal([]);
  const [agentsLive, setAgentsLive] = createSignal(/* @__PURE__ */ new Map());
  const [agentsSelected, setAgentsSelected] = createSignal(0);
  const [agentsLoading, setAgentsLoading] = createSignal(false);
  const [agentsError, setAgentsError] = createSignal("");
  let agentsReturnRoute;
  let agentsSurface = "route";
  let agentsOpenedFrom;
  let agentsPending = false;
  let agentsKnownChildren = /* @__PURE__ */ new Set();
  let agentsKeyHandler;
  const agentsEventDisposers = [];
  const subagentRootSessionID = () => {
    const root = snapshot().root_session_id;
    if (typeof root === "string" && root !== "" && root !== "global") return root;
    return conversationScope(api, activeSessionOverride())?.rootSessionID || "";
  };
  const readSubagentChildren = () => {
    if (disposed || agentsPending) return;
    const root = subagentRootSessionID();
    if (!root) {
      setAgentsChildren([]);
      agentsKnownChildren = /* @__PURE__ */ new Set();
      setAgentsError("");
      return;
    }
    const listFn = api.client?.session?.children;
    if (typeof listFn !== "function") {
      setAgentsError("subagent sessions are unavailable in this host");
      return;
    }
    agentsPending = true;
    setAgentsLoading(true);
    let request;
    try {
      request = listFn.call(api.client.session, {
        sessionID: root
      });
    } catch {
      agentsPending = false;
      setAgentsLoading(false);
      setAgentsError("subagent sessions are unavailable in this host");
      return;
    }
    Promise.resolve(request).then((response) => {
      if (disposed) return;
      const list = Array.isArray(response) ? response : response?.data;
      if (!Array.isArray(list)) {
        setAgentsError("subagent sessions are unavailable in this host");
        return;
      }
      const children = list.filter((child) => child && typeof child.id === "string" && child.id !== root);
      agentsKnownChildren = new Set(children.map((child) => child.id));
      setAgentsChildren(children);
      setAgentsError("");
    }).catch(() => {
      if (!disposed) setAgentsError("subagent sessions are unavailable in this host");
    }).finally(() => {
      agentsPending = false;
      if (!disposed) setAgentsLoading(false);
    });
  };
  const patchSubagentLive = (sessionID, patch) => {
    if (typeof sessionID !== "string" || sessionID === "" || !agentsOpen()) return;
    if (!agentsKnownChildren.has(sessionID)) return;
    setAgentsLive((previous) => {
      const next = new Map(previous);
      next.set(sessionID, {
        ...next.get(sessionID),
        ...patch
      });
      return next;
    });
  };
  const partActivityPatch = (part) => {
    const label = partActivityLabel(part);
    return {
      tool: label?.tool,
      step: label?.step,
      updatedAt: Date.now()
    };
  };
  const agentRows = createMemo(() => {
    const tasks = snapshot().tasks;
    const live = agentsLive();
    const rows = agentsChildren().map((child) => {
      const patch = live.get(child.id);
      const stateStatus = api.state?.session?.status?.(child.id) || api.data?.session?.status?.(child.id);
      const patchActivity = patch && (patch.tool || patch.step) ? {
        tool: patch.tool,
        step: patch.step
      } : lastActivityLabel(api, child.id);
      const task = taskForSession(child.id, tasks);
      const created = sessionStartMillis(child);
      return {
        sessionID: child.id,
        title: patch?.title || (typeof child.title === "string" && child.title ? child.title : child.id),
        agent: patch?.agent || (typeof child.agent === "string" ? child.agent : ""),
        status: patch?.status ?? subagentStatus(stateStatus),
        retryAttempt: patch?.retryAttempt ?? subagentRetryAttempt(stateStatus),
        startedAt: created,
        tool: patchActivity.tool,
        step: patchActivity.step,
        tokens: patch?.tokens ?? sessionTokenTotal(child),
        cost: patch?.cost ?? (typeof child.cost === "number" && child.cost > 0 ? child.cost : void 0),
        updatedAt: patch?.updatedAt ?? created ?? 0,
        taskID: task?.taskID,
        taskStatus: task?.status
      };
    });
    rows.sort((left, right) => {
      const rank = subagentRank(left.status) - subagentRank(right.status);
      return rank !== 0 ? rank : right.updatedAt - left.updatedAt;
    });
    return rows;
  });
  createEffect(() => {
    const count = agentRows().length;
    if (count === 0) {
      if (agentsSelected() !== 0) setAgentsSelected(0);
      return;
    }
    if (agentsSelected() >= count) setAgentsSelected(count - 1);
  });
  const moveAgentsSelection = (delta) => {
    const count = agentRows().length;
    if (count === 0) return;
    const next = Math.min(count - 1, Math.max(0, agentsSelected() + delta));
    if (next !== agentsSelected()) setAgentsSelected(next);
  };
  const agentsKeyInput = () => {
    const keyInput = api.renderer?.keyInput;
    return keyInput && typeof keyInput.on === "function" ? keyInput : void 0;
  };
  const detachAgentsKeys = () => {
    const keyInput = agentsKeyInput();
    if (agentsKeyHandler && keyInput && typeof keyInput.off === "function") {
      try {
        keyInput.off("keypress", agentsKeyHandler);
      } catch {
      }
    }
    agentsKeyHandler = void 0;
  };
  const agentsPanelOwnsKeyboard = () => {
    if (agentsSurface === "dialog") return true;
    const name = api.route?.current?.name;
    if (typeof name !== "string" || name === "" || name === AGENTS_ROUTE) return true;
    return name === agentsOpenedFrom;
  };
  const handleAgentsKey = (event) => {
    if (!agentsOpen() || !agentsPanelOwnsKeyboard()) return;
    const name = typeof event?.name === "string" ? event.name.toLowerCase() : "";
    const sequence = typeof event?.sequence === "string" ? event.sequence : "";
    const modifiers = Boolean(event?.ctrl || event?.meta);
    let handled = true;
    if (name === "up" || sequence === "\x1B[A" || !modifiers && name === "k") {
      moveAgentsSelection(-1);
    } else if (name === "down" || sequence === "\x1B[B" || !modifiers && name === "j") {
      moveAgentsSelection(1);
    } else if (name === "return" || name === "enter" || name === "linefeed" || sequence === "\r") {
      activateSubagentRow(agentsSelected());
    } else if (name === "escape" || name === "esc" || sequence === "\x1B") {
      closeAgentsPanel();
    } else {
      handled = false;
    }
    if (!handled) return;
    if (typeof event?.preventDefault === "function") event.preventDefault();
    if (typeof event?.stopPropagation === "function") event.stopPropagation();
  };
  const attachAgentsKeys = () => {
    if (agentsKeyHandler) return;
    const keyInput = agentsKeyInput();
    if (!keyInput) return;
    const handler = (event) => handleAgentsKey(event);
    try {
      keyInput.on("keypress", handler);
      agentsKeyHandler = handler;
    } catch {
      agentsKeyHandler = void 0;
    }
  };
  const resetAgentsPanelState = () => {
    detachAgentsKeys();
    setAgentsOpen(false);
    setAgentsLive(/* @__PURE__ */ new Map());
    agentsKnownChildren = /* @__PURE__ */ new Set();
    setAgentsChildren([]);
    setAgentsError("");
  };
  const closeAgentsPanel = (restoreRoute = true) => {
    if (!agentsOpen()) return;
    resetAgentsPanelState();
    const dialogStack = api.ui?.dialog;
    if (dialogStack && typeof dialogStack.clear === "function") {
      try {
        dialogStack.clear();
      } catch {
      }
    }
    const route = api.route;
    const target = agentsReturnRoute;
    agentsReturnRoute = void 0;
    if (restoreRoute && target && route && typeof route.navigate === "function") {
      try {
        route.navigate(target.name, target.params);
      } catch {
      }
    }
  };
  const openSubagentSession = (sessionID) => {
    if (!sessionID) return;
    closeAgentsPanel(false);
    const route = api.route;
    if (route && typeof route.navigate === "function") {
      try {
        route.navigate("session", {
          sessionID
        });
        return;
      } catch {
      }
    }
    const selectSession = api.client?.tui?.selectSession;
    if (typeof selectSession === "function") {
      try {
        void Promise.resolve(selectSession.call(api.client.tui, {
          sessionID
        })).catch(() => {
        });
        return;
      } catch {
      }
    }
    showToast("Subagent", "this host cannot navigate to a child session", "info");
  };
  const activateSubagentRow = (index) => {
    const row = agentRows()[index];
    if (!row) return;
    setAgentsSelected(index);
    openSubagentSession(row.sessionID);
  };
  const agentsPanelView = () => _$createComponent(CortexAgentsPanel, {
    rows: agentRows,
    selected: agentsSelected,
    loading: agentsLoading,
    error: agentsError,
    now,
    pulse,
    onActivate: activateSubagentRow,
    get theme() {
      return api.theme?.current || api.theme;
    }
  });
  const openAgentsPanel = () => {
    const route = api.route;
    const dialogStack = api.ui?.dialog;
    const hasRoute = Boolean(route && typeof route.register === "function" && typeof route.navigate === "function");
    const hasDialog = Boolean(dialogStack && typeof dialogStack.replace === "function");
    if (!hasRoute && !hasDialog) {
      showToast("Cortex Subagents", "this host exposes no route or dialog surface", "warning");
      return false;
    }
    if (hasRoute) {
      agentsSurface = "route";
      agentsOpenedFrom = void 0;
      const current = route.current;
      if (current && typeof current.name === "string" && current.name !== AGENTS_ROUTE) {
        agentsOpenedFrom = current.name;
        agentsReturnRoute = {
          name: current.name,
          params: current.params
        };
      }
      try {
        route.navigate(AGENTS_ROUTE, {});
      } catch {
      }
    } else {
      agentsSurface = "dialog";
      agentsOpenedFrom = void 0;
      try {
        if (typeof dialogStack.setSize === "function") dialogStack.setSize("large");
        dialogStack.replace(() => agentsPanelView(), () => resetAgentsPanelState());
      } catch {
      }
    }
    setAgentsOpen(true);
    setAgentsSelected(0);
    attachAgentsKeys();
    readSubagentChildren();
    return true;
  };
  if (api.event && typeof api.event.on === "function") {
    const patch = (event, live) => patchSubagentLive(event?.properties?.sessionID, live);
    const busy = (tool) => ({
      tool,
      step: void 0,
      status: "busy",
      updatedAt: Date.now()
    });
    const subscriptions = [["session.status", (event) => patch(event, {
      status: subagentStatus(event?.properties?.status),
      retryAttempt: subagentRetryAttempt(event?.properties?.status),
      updatedAt: Date.now()
    })], ["session.idle", (event) => patch(event, {
      status: "idle",
      tool: void 0,
      step: void 0,
      updatedAt: Date.now()
    })], ["session.updated", (event) => {
      const info = event?.properties?.info;
      patchSubagentLive(info?.id ?? event?.properties?.sessionID, {
        title: typeof info?.title === "string" && info.title ? info.title : void 0,
        agent: typeof info?.agent === "string" && info.agent ? info.agent : void 0,
        tokens: sessionTokenTotal(info),
        cost: typeof info?.cost === "number" && info.cost > 0 ? info.cost : void 0,
        updatedAt: Date.now()
      });
    }], ["session.next.tool.input.started", (event) => patch(event, busy(typeof event?.properties?.name === "string" ? event.properties.name : void 0))], ["session.next.tool.called", (event) => patch(event, busy(typeof event?.properties?.tool === "string" ? event.properties.tool : void 0))], ["session.next.tool.success", (event) => patch(event, {
      tool: void 0,
      updatedAt: Date.now()
    })], ["session.next.tool.failed", (event) => patch(event, {
      tool: void 0,
      updatedAt: Date.now()
    })], ["session.next.step.started", (event) => patch(event, {
      step: "step",
      tool: void 0,
      status: "busy",
      updatedAt: Date.now()
    })], ["session.next.step.ended", (event) => {
      const properties = event?.properties;
      patch(event, {
        step: void 0,
        tool: void 0,
        tokens: sessionTokenTotal({
          tokens: properties?.tokens
        }),
        cost: typeof properties?.cost === "number" && properties.cost > 0 ? properties.cost : void 0,
        updatedAt: Date.now()
      });
    }], ["message.part.updated", (event) => patch(event, partActivityPatch(event?.properties?.part))]];
    for (const [type, handler] of subscriptions) {
      try {
        const off = api.event.on(type, handler);
        if (typeof off === "function") agentsEventDisposers.push(off);
      } catch {
      }
    }
  }
  createEffect(() => {
    const open = agentsOpen();
    const stamp = snapshot().generated_at;
    if (!open || !stamp) return;
    untrack(() => readSubagentChildren());
  });
  const cleanup = () => {
    if (disposed) return;
    disposed = true;
    detachAgentsKeys();
    for (const off of agentsEventDisposers) {
      try {
        off();
      } catch {
      }
    }
    agentsEventDisposers.length = 0;
    clearInterval(snapshotPoll);
    clearInterval(statsPoll);
    clearInterval(nanPoll);
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
          density,
          toggleDensity,
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
      render: (ctx) => (() => {
        var _el$280 = _$createElement("box");
        _$setProp(_el$280, "flexDirection", "column");
        _$setProp(_el$280, "width", "100%");
        _$insert(_el$280, _$createComponent(HomeLogo, {
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        _$insert(_el$280, _$createComponent(HomeStatsWidget, {
          stats: usageStats,
          loading: statsLoading,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        return _el$280;
      })()
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
        const panelTheme = panel?.theme?.current || panel?.theme || api.theme;
        return _$createComponent(Show, {
          get when() {
            return panel?.name === "cortex.model";
          },
          get fallback() {
            return _$createComponent(Show, {
              get when() {
                return panel?.name === "cortex.nan";
              },
              get fallback() {
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
                      theme: panelTheme
                    });
                  }
                });
              },
              get children() {
                return _$createComponent(NanDetailPanel, {
                  days: nanDetailDays,
                  rows: nanDetailRows,
                  loading: nanDetailLoading,
                  loadError: nanDetailError,
                  theme: panelTheme
                });
              }
            });
          },
          get children() {
            return _$createComponent(NanModelPickerPanel, {
              models: () => nanCatalog()?.models ?? [],
              agents: modelAgents,
              loading: pickerLoading,
              loadError: pickerLoadError,
              phase: pickerPhase,
              preview: pickerPreview,
              result: pickerResult,
              runError: pickerRunError,
              onSelectionChange: () => {
                setPickerPreview("");
                setPickerResult("");
                setPickerRunError("");
              },
              onPreview: (agent, model, level) => runModelSet(agent, model, level, true),
              onApply: (agent, model, level) => runModelSet(agent, model, level, false),
              theme: panelTheme
            });
          }
        });
      }
    });
    api.ui.slot({
      append: "session.composer.top",
      render: (ctx) => _$createComponent(NanUsageStrip, {
        view: nanStrip,
        get textLimit() {
          return Math.max(24, terminalColumns() - 8);
        },
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      })
    });
  }
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeBoardLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [{
        name: ":cortex",
        title: "Cortex Board",
        desc: "Open the Cortex board and delegation panel",
        category: "Cortex",
        nargs: "0",
        run: () => {
          if (!api.ui?.panel?.open) return false;
          api.ui.panel.open("cortex.board");
          return true;
        }
      }, {
        name: ":cortex-stats",
        title: "Cortex Usage Stats",
        desc: "Open Cortex usage statistics dashboard",
        category: "Cortex",
        nargs: "0",
        run: () => {
          openStatsView();
          return true;
        }
      }, {
        name: ":stats",
        title: "Usage Stats",
        desc: "Open Cortex usage statistics dashboard",
        category: "Cortex",
        nargs: "0",
        run: () => {
          openStatsView();
          return true;
        }
      }]
    });
    if (typeof disposeBoardLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeBoardLayer);
    }
  }
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeDensityLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [{
        name: ":cortex-density",
        title: "Cortex Density",
        desc: "Toggle sidebar compact/expanded density",
        category: "Cortex",
        nargs: "0",
        run: () => {
          toggleDensity();
          return true;
        }
      }],
      bindings: [{
        key: "alt+d",
        cmd: ":cortex-density"
      }]
    });
    if (typeof disposeDensityLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeDensityLayer);
    }
  }
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const nanPanels = [{
      name: ":model",
      title: "Nan Model Picker",
      desc: "Pick a nan model and effort for an agent from the catalog",
      panel: "cortex.model",
      load: loadModelPicker
    }, {
      name: ":nan",
      title: "Nan Usage Detail",
      desc: "Open the nan daily usage detail panel",
      panel: "cortex.nan",
      load: loadNanDetails
    }];
    const disposeNanLayers = api.keymap.registerLayer({
      priority: 40,
      commands: nanPanels.map((entry) => ({
        name: entry.name,
        title: entry.title,
        desc: entry.desc,
        category: "Cortex",
        nargs: "0",
        run: () => {
          if (!api.ui?.panel?.open) return false;
          api.ui.panel.open(entry.panel);
          entry.load();
          return true;
        }
      }))
    });
    if (typeof disposeNanLayers === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeNanLayers);
    }
  }
  if (api.route && typeof api.route.register === "function") {
    let disposeAgentsRoute;
    try {
      disposeAgentsRoute = api.route.register([{
        name: AGENTS_ROUTE,
        render: () => agentsPanelView()
      }]);
    } catch {
    }
    if (typeof disposeAgentsRoute === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeAgentsRoute);
    }
  }
  if (api.keymap && typeof api.keymap.registerLayer === "function") {
    const disposeAgentsLayer = api.keymap.registerLayer({
      priority: 40,
      commands: [{
        name: AGENTS_COMMAND,
        title: "Cortex Subagents",
        desc: "Open the live subagent monitor panel",
        category: "Cortex",
        nargs: "0",
        run: () => openAgentsPanel()
      }]
    });
    if (typeof disposeAgentsLayer === "function" && api.lifecycle && typeof api.lifecycle.onDispose === "function") {
      api.lifecycle.onDispose(disposeAgentsLayer);
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
        density,
        toggleDensity,
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
      return (() => {
        var _el$281 = _$createElement("box");
        _$setProp(_el$281, "flexDirection", "column");
        _$setProp(_el$281, "width", "100%");
        _$insert(_el$281, _$createComponent(HomeStatsWidget, {
          stats: usageStats,
          loading: statsLoading,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        _$insert(_el$281, _$createComponent(HomeBottomStatus, {
          snapshot,
          jobs,
          spinner,
          snapshotError,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        return _el$281;
      })();
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
    "session.composer.top"(ctx) {
      return _$createComponent(NanUsageStrip, {
        view: nanStrip,
        get textLimit() {
          return Math.max(24, terminalColumns() - 8);
        },
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
