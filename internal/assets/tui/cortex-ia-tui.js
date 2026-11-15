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
  brand: "\u25C6"
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
var MINIONS_EXPANDED_KEY = "cortex.sidebar.minions.expanded";
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
var NAN_INK = {
  background: "#0B0B0C",
  panel: "#121214",
  element: "#121214",
  border: "#3A3A3D",
  borderSubtle: "#252527",
  accentBorder: "#7D39EB",
  accent: "#9B6BF0",
  text: "#FFFFFF",
  textSoft: "#C9C9CC",
  textMuted: "#9A9A9E",
  success: "#22C55E",
  warning: "#F59E0B",
  error: "#EF4444"
};
var PALETTE_FALLBACK = {
  text: NAN_INK.text,
  textMuted: NAN_INK.textMuted,
  accent: NAN_INK.accent,
  accentAlt: NAN_INK.accent,
  accentBorder: NAN_INK.accentBorder,
  primary: NAN_INK.accentBorder,
  sky: NAN_INK.textSoft,
  success: NAN_INK.success,
  warning: NAN_INK.warning,
  error: NAN_INK.error,
  info: NAN_INK.accent,
  border: NAN_INK.border,
  borderSubtle: NAN_INK.borderSubtle,
  panel: NAN_INK.panel,
  element: NAN_INK.element,
  background: NAN_INK.background
};
var SOFT_TEXT_WEIGHT = 0.78;
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
    accentAlt: read("accent", PALETTE_FALLBACK.accentAlt),
    accentBorder: readColor(source["borderActive"]) ?? PALETTE_FALLBACK.accentBorder,
    primary: read("primary", PALETTE_FALLBACK.primary),
    sky: read("info", PALETTE_FALLBACK.sky),
    success: read("success", PALETTE_FALLBACK.success),
    warning: read("warning", PALETTE_FALLBACK.warning),
    error: read("error", PALETTE_FALLBACK.error),
    info: read("info", PALETTE_FALLBACK.info),
    border,
    borderSubtle: readColor(source["borderSubtle"]) ?? PALETTE_FALLBACK.borderSubtle,
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
    color: palette.accent,
    tag: "ORCH"
  };
  if (r.includes("impl")) return {
    color: palette.warning,
    tag: "IMPL"
  };
  if (r.includes("rev")) return {
    color: palette.accentAlt,
    tag: "REVW"
  };
  if (r.includes("inv")) return {
    color: palette.textSoft,
    tag: "INVS"
  };
  if (r.includes("plan")) return {
    color: palette.textSoft,
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
    _$setProp(_el$, "borderStyle", "single");
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
      var _v$3 = props.isExecuting() ? palette.warning : palette.accentBorder, _v$4 = props.isExecuting() ? `${GLYPH.brand} CORTEX\xB7IA v2.0 [${props.spinner()} ACTIVE]` : `${GLYPH.brand} CORTEX\xB7IA v2.0 [${GLYPH.working} STANDBY]`, _v$5 = palette.accent, _v$6 = palette.panel, _v$7 = freshnessColor(), _v$8 = palette.info, _v$9 = props.density() === "compact" ? palette.success : palette.textMuted, _v$0 = props.onToggleDensity;
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
    _$setProp(_el$16, "border", ["bottom"]);
    _$setProp(_el$17, "selectable", false);
    _$insert(_el$17, () => props.expanded() ? `${MARK_EXPANDED} ` : `${MARK_COLLAPSED} `);
    _$setProp(_el$18, "selectable", false);
    _$insert(_el$18, () => displayTitle().toUpperCase());
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
      var _v$11 = palette.border, _v$12 = props.onToggle, _v$13 = props.expanded() ? palette.accent : palette.textMuted, _v$14 = palette.text, _v$15 = TextAttributes.BOLD;
      _v$11 !== _p$.e && (_p$.e = _$setProp(_el$16, "borderColor", _v$11, _p$.e));
      _v$12 !== _p$.t && (_p$.t = _$setProp(_el$16, "onMouseDown", _v$12, _p$.t));
      _v$13 !== _p$.a && (_p$.a = _$setProp(_el$17, "fg", _v$13, _p$.a));
      _v$14 !== _p$.o && (_p$.o = _$setProp(_el$18, "fg", _v$14, _p$.o));
      _v$15 !== _p$.i && (_p$.i = _$setProp(_el$18, "attributes", _v$15, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
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
  const block = "\u25B0";
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
      var _v$16 = palette.info, _v$17 = palette.success, _v$18 = palette.primary, _v$19 = palette.warning, _v$20 = palette.borderSubtle, _v$21 = palette.text, _v$22 = palette.textMuted;
      _v$16 !== _p$.e && (_p$.e = _$setProp(_el$22, "fg", _v$16, _p$.e));
      _v$17 !== _p$.t && (_p$.t = _$setProp(_el$23, "fg", _v$17, _p$.t));
      _v$18 !== _p$.a && (_p$.a = _$setProp(_el$24, "fg", _v$18, _p$.a));
      _v$19 !== _p$.o && (_p$.o = _$setProp(_el$25, "fg", _v$19, _p$.o));
      _v$20 !== _p$.i && (_p$.i = _$setProp(_el$27, "fg", _v$20, _p$.i));
      _v$21 !== _p$.n && (_p$.n = _$setProp(_el$28, "fg", _v$21, _p$.n));
      _v$22 !== _p$.s && (_p$.s = _$setProp(_el$29, "fg", _v$22, _p$.s));
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
    _$setProp(_el$35, "borderStyle", "single");
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
        var _el$46 = _$createElement("text");
        _$insert(_el$46, () => `\u2502 ${GLYPH.warn} TTL: ${ttl()}${props.task.lease_count ? ` (${props.task.lease_count} lk)` : ""}`);
        _$effect((_$p) => _$setProp(_el$46, "fg", ttl() === "expired" ? palette.error : palette.sky, _$p));
        return _el$46;
      })()
    }), null);
    _$insert(_el$35, _$createComponent(Show, {
      get when() {
        return !props.densityCompact;
      },
      get children() {
        return [(() => {
          var _el$42 = _$createElement("box"), _el$43 = _$createElement("text");
          _$insertNode(_el$42, _el$43);
          _$setProp(_el$42, "flexDirection", "row");
          _$insert(_el$43, () => clipped(`${GLYPH.row} Durable task${props.task.owner ? ` (${props.task.owner})` : ""}`, props.textLimit));
          _$effect((_$p) => _$setProp(_el$43, "fg", palette.accent, _$p));
          return _el$42;
        })(), (() => {
          var _el$44 = _$createElement("box"), _el$45 = _$createElement("text");
          _$insertNode(_el$44, _el$45);
          _$setProp(_el$44, "flexDirection", "row");
          _$setProp(_el$44, "marginTop", 0);
          _$setProp(_el$44, "onMouseDown", () => openWebConsole(props.task.board_id, props.task.task_id));
          _$setProp(_el$45, "selectable", false);
          _$insert(_el$45, () => `[${GLYPH.web} View in Web]`);
          _$effect((_$p) => _$setProp(_el$45, "fg", palette.info, _$p));
          return _el$44;
        })()];
      }
    }), null);
    _$effect((_p$) => {
      var _v$23 = palette.accentBorder, _v$24 = clipped(`${props.spinner()} TASK IN PROGRESS`, props.textLimit), _v$25 = palette.warning, _v$26 = palette.panel, _v$27 = palette.sky, _v$28 = palette.text, _v$29 = palette.warning;
      _v$23 !== _p$.e && (_p$.e = _$setProp(_el$35, "borderColor", _v$23, _p$.e));
      _v$24 !== _p$.t && (_p$.t = _$setProp(_el$35, "title", _v$24, _p$.t));
      _v$25 !== _p$.a && (_p$.a = _$setProp(_el$35, "titleColor", _v$25, _p$.a));
      _v$26 !== _p$.o && (_p$.o = _$setProp(_el$35, "backgroundColor", _v$26, _p$.o));
      _v$27 !== _p$.i && (_p$.i = _$setProp(_el$37, "fg", _v$27, _p$.i));
      _v$28 !== _p$.n && (_p$.n = _$setProp(_el$38, "fg", _v$28, _p$.n));
      _v$29 !== _p$.s && (_p$.s = _$setProp(_el$41, "fg", _v$29, _p$.s));
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
            var _el$47 = _$createElement("box"), _el$48 = _$createElement("box"), _el$49 = _$createElement("text"), _el$50 = _$createElement("text"), _el$51 = _$createElement("text"), _el$52 = _$createElement("text"), _el$53 = _$createElement("box"), _el$54 = _$createElement("text");
            _$insertNode(_el$47, _el$48);
            _$insertNode(_el$47, _el$53);
            _$setProp(_el$47, "flexDirection", "column");
            _$setProp(_el$47, "marginTop", 0);
            _$insertNode(_el$48, _el$49);
            _$insertNode(_el$48, _el$50);
            _$insertNode(_el$48, _el$51);
            _$insertNode(_el$48, _el$52);
            _$setProp(_el$48, "flexDirection", "row");
            _$setProp(_el$48, "paddingLeft", 2);
            _$insert(_el$49, () => `${isProg ? props.spinner() : chip.icon} `);
            _$insert(_el$50, () => `[${chip.tag}] `);
            _$insert(_el$51, () => clipped(task.task_id, Math.max(8, props.textLimit - 14)));
            _$setProp(_el$52, "onMouseDown", () => openWebConsole(task.board_id, task.task_id));
            _$setProp(_el$52, "selectable", false);
            _$insert(_el$52, () => ` [${GLYPH.web}]`);
            _$insertNode(_el$53, _el$54);
            _$setProp(_el$53, "flexDirection", "row");
            _$setProp(_el$53, "paddingLeft", 5);
            _$insert(_el$54, () => `${clipped(task.title, Math.max(8, props.textLimit - 5))}${task.owner ? ` \xB7 ${clipped(task.owner, 6)}` : ""}${task.lease_count ? ` \xB7 ${GLYPH.warn} ${task.lease_count}lk` : ""}`);
            _$effect((_p$) => {
              var _v$30 = chip.color, _v$31 = chip.color, _v$32 = isProg ? palette.text : palette.textSoft, _v$33 = palette.info, _v$34 = palette.textMuted;
              _v$30 !== _p$.e && (_p$.e = _$setProp(_el$49, "fg", _v$30, _p$.e));
              _v$31 !== _p$.t && (_p$.t = _$setProp(_el$50, "fg", _v$31, _p$.t));
              _v$32 !== _p$.a && (_p$.a = _$setProp(_el$51, "fg", _v$32, _p$.a));
              _v$33 !== _p$.o && (_p$.o = _$setProp(_el$52, "fg", _v$33, _p$.o));
              _v$34 !== _p$.i && (_p$.i = _$setProp(_el$54, "fg", _v$34, _p$.i));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0,
              i: void 0
            });
            return _el$47;
          })();
        }
      });
    }
  });
}
function MinionRows(props) {
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return props.rows.length > 0;
    },
    get fallback() {
      return _$createComponent(EmptyState, {
        palette,
        label: "No live minions"
      });
    },
    get children() {
      return _$createComponent(For, {
        get each() {
          return props.rows.slice(0, MAX_VISIBLE_ROWS);
        },
        children: (row, index) => {
          const glyph = () => row.retryAttempt ? `${GLYPH.warn}${row.retryAttempt}` : row.status === "busy" ? props.spinner() : row.status === "idle" ? GLYPH.idle : row.status === "unknown" ? GLYPH.active : GLYPH.working;
          const activity = row.step || row.tool || "";
          return (() => {
            var _el$55 = _$createElement("box"), _el$56 = _$createElement("box"), _el$57 = _$createElement("text"), _el$58 = _$createElement("text"), _el$59 = _$createElement("text"), _el$60 = _$createElement("box"), _el$61 = _$createElement("text");
            _$insertNode(_el$55, _el$56);
            _$insertNode(_el$55, _el$60);
            _$setProp(_el$55, "flexDirection", "column");
            _$setProp(_el$55, "marginTop", 0);
            _$insertNode(_el$56, _el$57);
            _$insertNode(_el$56, _el$58);
            _$insertNode(_el$56, _el$59);
            _$setProp(_el$56, "flexDirection", "row");
            _$setProp(_el$56, "paddingLeft", 2);
            _$setProp(_el$56, "onMouseDown", () => props.onActivate?.(index()));
            _$insert(_el$57, () => `${glyph()} `);
            _$insert(_el$58, () => `${fitWidth(row.agent || "agent", 10)} `);
            _$insert(_el$59, () => clipped(row.title, Math.max(8, props.textLimit - 16)));
            _$insertNode(_el$60, _el$61);
            _$setProp(_el$60, "flexDirection", "row");
            _$setProp(_el$60, "paddingLeft", 5);
            _$insert(_el$61, () => clipped(`${row.status}${activity ? ` \xB7 ${activity}` : ""}${row.tokens !== void 0 ? ` \xB7 ${formatTokens(row.tokens)} tok` : ""}${row.taskID ? ` \xB7 ${shortID(row.taskID)}` : ""}`, props.textLimit));
            _$effect((_p$) => {
              var _v$35 = subagentStatusColor(row.status, palette), _v$36 = roleChip(row.agent || "agent", palette).color, _v$37 = row.status === "busy" ? palette.text : palette.textSoft, _v$38 = palette.textMuted;
              _v$35 !== _p$.e && (_p$.e = _$setProp(_el$57, "fg", _v$35, _p$.e));
              _v$36 !== _p$.t && (_p$.t = _$setProp(_el$58, "fg", _v$36, _p$.t));
              _v$37 !== _p$.a && (_p$.a = _$setProp(_el$59, "fg", _v$37, _p$.a));
              _v$38 !== _p$.o && (_p$.o = _$setProp(_el$61, "fg", _v$38, _p$.o));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0,
              o: void 0
            });
            return _el$55;
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
          var _el$62 = _$createElement("box"), _el$63 = _$createElement("box"), _el$64 = _$createElement("text"), _el$65 = _$createElement("text"), _el$66 = _$createElement("box"), _el$67 = _$createElement("text");
          _$insertNode(_el$62, _el$63);
          _$insertNode(_el$62, _el$66);
          _$setProp(_el$62, "flexDirection", "column");
          _$insertNode(_el$63, _el$64);
          _$insertNode(_el$63, _el$65);
          _$setProp(_el$63, "flexDirection", "row");
          _$setProp(_el$63, "paddingLeft", 2);
          _$insert(_el$64, () => `${GLYPH.fail} `);
          _$insert(_el$65, () => clipped(item.title, Math.max(8, props.textLimit - 4)));
          _$insertNode(_el$66, _el$67);
          _$setProp(_el$66, "flexDirection", "row");
          _$setProp(_el$66, "paddingLeft", 5);
          _$insert(_el$67, () => clipped(item.detail, Math.max(8, props.textLimit - 5)));
          _$effect((_p$) => {
            var _v$39 = palette.error, _v$40 = palette.text, _v$41 = palette.textMuted;
            _v$39 !== _p$.e && (_p$.e = _$setProp(_el$64, "fg", _v$39, _p$.e));
            _v$40 !== _p$.t && (_p$.t = _$setProp(_el$65, "fg", _v$40, _p$.t));
            _v$41 !== _p$.a && (_p$.a = _$setProp(_el$67, "fg", _v$41, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$62;
        })()
      });
    }
  });
}
function OperationalStatusBlock(props) {
  const palette = resolvePalette(props.theme);
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
  const hasSignal = createMemo(() => props.activeExecutions > 0 || props.inReview > 0 || props.attentionCount > 0 || doneTasks() > 0 || totalTasks() > 0 || totalLeases() > 0 || blockedTasks() > 0);
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
    const closed = doneTasks() + blockedTasks();
    if (closed === 0) return void 0;
    return Math.round(doneTasks() / closed * 100);
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
      filled: "\u25B0".repeat(filled),
      empty: "\u25B1".repeat(empty)
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
  const synapseValue = createMemo(() => props.activeExecutions > 0 ? `${props.spinner()} active \xB7 ${props.activeExecutions} runs` : `${props.pulse()} synced`);
  const authorityValue = createMemo(() => totalLeases() > 0 ? `sqlite \xB7 ${totalLeases()} lk` : "sqlite");
  const healthCell = () => (() => {
    var _el$68 = _$createElement("box"), _el$69 = _$createElement("text");
    _$insertNode(_el$68, _el$69);
    _$setProp(_el$68, "flexDirection", "row");
    _$setProp(_el$68, "gap", 1);
    _$insertNode(_el$69, _$createTextNode(`health`));
    _$insert(_el$68, _$createComponent(Show, {
      get when() {
        return successRate() !== void 0;
      },
      get fallback() {
        return (() => {
          var _el$74 = _$createElement("text");
          _$insert(_el$74, () => `${GLYPH.idle} standby`);
          _$effect((_$p) => _$setProp(_el$74, "fg", palette.textMuted, _$p));
          return _el$74;
        })();
      },
      get children() {
        return [(() => {
          var _el$71 = _$createElement("text");
          _$insert(_el$71, () => healthBars().filled);
          _$effect((_$p) => _$setProp(_el$71, "fg", healthColor(), _$p));
          return _el$71;
        })(), (() => {
          var _el$72 = _$createElement("text");
          _$insert(_el$72, () => healthBars().empty);
          _$effect((_$p) => _$setProp(_el$72, "fg", palette.borderSubtle, _$p));
          return _el$72;
        })(), (() => {
          var _el$73 = _$createElement("text");
          _$insert(_el$73, () => `${successRate()}%`);
          _$effect((_$p) => _$setProp(_el$73, "fg", healthColor(), _$p));
          return _el$73;
        })()];
      }
    }), null);
    _$effect((_$p) => _$setProp(_el$69, "fg", palette.textMuted, _$p));
    return _el$68;
  })();
  const signalCells = () => [_$createComponent(StatusCell, {
    palette,
    label: "synapse",
    value: synapseValue,
    color: () => props.activeExecutions > 0 ? palette.warning : palette.text
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
    var _el$75 = _$createElement("box");
    _$setProp(_el$75, "flexDirection", "column");
    _$setProp(_el$75, "marginTop", 1);
    _$setProp(_el$75, "paddingLeft", 1);
    _$setProp(_el$75, "paddingRight", 1);
    _$setProp(_el$75, "borderStyle", "single");
    _$setProp(_el$75, "titleAlignment", "left");
    _$insert(_el$75, _$createComponent(Show, {
      get when() {
        return hasSignal();
      },
      get fallback() {
        return (() => {
          var _el$83 = _$createElement("text");
          _$insert(_el$83, () => `${GLYPH.idle} standby \xB7 0 active \xB7 authority sqlite \xB7 ${props.stale ? "stale" : "live"}`);
          _$effect((_$p) => _$setProp(_el$83, "fg", palette.textMuted, _$p));
          return _el$83;
        })();
      },
      get children() {
        return [(() => {
          var _el$76 = _$createElement("box");
          _$setProp(_el$76, "flexDirection", "row");
          _$setProp(_el$76, "gap", 2);
          _$insert(_el$76, _$createComponent(For, {
            get each() {
              return metrics();
            },
            children: (metric) => (() => {
              var _el$84 = _$createElement("box"), _el$85 = _$createElement("text"), _el$86 = _$createElement("text");
              _$insertNode(_el$84, _el$85);
              _$insertNode(_el$84, _el$86);
              _$setProp(_el$84, "flexDirection", "row");
              _$setProp(_el$84, "gap", 1);
              _$insert(_el$85, () => metric.label);
              _$insert(_el$86, () => padMetric(metric.value));
              _$effect((_p$) => {
                var _v$46 = palette.textMuted, _v$47 = palette.text;
                _v$46 !== _p$.e && (_p$.e = _$setProp(_el$85, "fg", _v$46, _p$.e));
                _v$47 !== _p$.t && (_p$.t = _$setProp(_el$86, "fg", _v$47, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$84;
            })()
          }));
          return _el$76;
        })(), _$createComponent(Show, {
          get when() {
            return !props.layout.compact;
          },
          get fallback() {
            return (() => {
              var _el$87 = _$createElement("box");
              _$setProp(_el$87, "flexDirection", "column");
              _$insert(_el$87, signalCells);
              return _el$87;
            })();
          },
          get children() {
            var _el$77 = _$createElement("box"), _el$78 = _$createElement("box"), _el$79 = _$createElement("box");
            _$insertNode(_el$77, _el$78);
            _$insertNode(_el$77, _el$79);
            _$setProp(_el$77, "flexDirection", "row");
            _$setProp(_el$77, "gap", 2);
            _$setProp(_el$78, "flexDirection", "column");
            _$setProp(_el$78, "flexGrow", 1);
            _$insert(_el$78, _$createComponent(StatusCell, {
              palette,
              label: "synapse",
              value: synapseValue,
              color: () => props.activeExecutions > 0 ? palette.warning : palette.text
            }), null);
            _$insert(_el$78, healthCell, null);
            _$setProp(_el$79, "flexDirection", "column");
            _$setProp(_el$79, "flexGrow", 1);
            _$insert(_el$79, _$createComponent(StatusCell, {
              palette,
              label: "authority",
              value: authorityValue
            }), null);
            _$insert(_el$79, _$createComponent(StatusCell, {
              palette,
              label: "web",
              value: () => "loopback",
              color: () => palette.info
            }), null);
            return _el$77;
          }
        }), (() => {
          var _el$80 = _$createElement("box"), _el$81 = _$createElement("text");
          _$insertNode(_el$80, _el$81);
          _$setProp(_el$80, "flexDirection", "row");
          _$setProp(_el$80, "gap", 2);
          _$insert(_el$81, freshnessLabel);
          _$insert(_el$80, _$createComponent(Show, {
            get when() {
              return totalTasks() > 0;
            },
            get children() {
              return [(() => {
                var _el$82 = _$createElement("text");
                _$insert(_el$82, etaLabel);
                _$effect((_$p) => _$setProp(_el$82, "fg", etaColor(), _$p));
                return _el$82;
              })(), _$createComponent(Show, {
                get when() {
                  return props.eta().sparkline;
                },
                children: (spark) => (() => {
                  var _el$88 = _$createElement("text");
                  _$insert(_el$88, spark);
                  _$effect((_$p) => _$setProp(_el$88, "fg", palette.textMuted, _$p));
                  return _el$88;
                })()
              })];
            }
          }), null);
          _$effect((_$p) => _$setProp(_el$81, "fg", props.stale ? palette.warning : palette.success, _$p));
          return _el$80;
        })()];
      }
    }));
    _$effect((_p$) => {
      var _v$42 = blockedTasks() > 0 ? palette.error : props.stale ? palette.borderSubtle : palette.borderActive, _v$43 = props.layout.compact ? `${GLYPH.brand} CONTROL` : `${GLYPH.brand} CONTROL MATRIX`, _v$44 = palette.accent, _v$45 = palette.element;
      _v$42 !== _p$.e && (_p$.e = _$setProp(_el$75, "borderColor", _v$42, _p$.e));
      _v$43 !== _p$.t && (_p$.t = _$setProp(_el$75, "title", _v$43, _p$.t));
      _v$44 !== _p$.a && (_p$.a = _$setProp(_el$75, "titleColor", _v$44, _p$.a));
      _v$45 !== _p$.o && (_p$.o = _$setProp(_el$75, "backgroundColor", _v$45, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$75;
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
  const runningMinions = createMemo(() => props.minions().filter((row) => row.status === "busy" || row.status === "retry").length);
  const activeExecutionsCount = createMemo(() => {
    const s = props.snapshot().summary;
    if (typeof s.active_executions === "number") return s.active_executions;
    return s.in_progress || 0;
  });
  const isExecuting = createMemo(() => props.nativeActivity() === "busy" || runningMinions() > 0);
  const doneTasks = createMemo(() => props.snapshot().summary.done || 0);
  const inReviewTasks = createMemo(() => props.snapshot().summary.in_review || 0);
  const inProgressTasks = createMemo(() => props.snapshot().summary.in_progress || 0);
  const blockedTasks = createMemo(() => props.snapshot().summary.blocked || 0);
  const totalTasks = createMemo(() => props.snapshot().summary.total_tasks || props.snapshot().tasks.length);
  return (() => {
    var _el$89 = _$createElement("box");
    _$use((node) => setRootWidth(Math.max(0, node.width || 0)), _el$89);
    _$setProp(_el$89, "flexDirection", "column");
    _$setProp(_el$89, "onSizeChange", function() {
      setRootWidth(Math.max(0, this.width || 0));
    });
    _$insert(_el$89, _$createComponent(CortexCockpitHeader, {
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
    _$insert(_el$89, _$createComponent(Show, {
      get when() {
        return !props.scopeReady();
      },
      get children() {
        var _el$90 = _$createElement("text");
        _$insertNode(_el$90, _$createTextNode(`Conversation unavailable \xB7 awaiting metadata`));
        _$setProp(_el$90, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$90, "fg", palette.warning, _$p));
        return _el$90;
      }
    }), null);
    _$insert(_el$89, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.scopeReady() && !props.snapshot().generated_at))() && !props.snapshotError();
      },
      get children() {
        var _el$92 = _$createElement("text");
        _$insertNode(_el$92, _$createTextNode(`Loading conversation state\u2026`));
        _$setProp(_el$92, "marginTop", 1);
        _$effect((_$p) => _$setProp(_el$92, "fg", palette.textMuted, _$p));
        return _el$92;
      }
    }), null);
    _$insert(_el$89, _$createComponent(Show, {
      get when() {
        return props.snapshotError();
      },
      get children() {
        var _el$94 = _$createElement("text");
        _$setProp(_el$94, "marginTop", 1);
        _$insert(_el$94, snapshotFailureLabel);
        _$effect((_$p) => _$setProp(_el$94, "fg", palette.error, _$p));
        return _el$94;
      }
    }), null);
    _$insert(_el$89, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!props.scopeReady())() && Boolean(props.snapshot().generated_at);
      },
      get children() {
        return [_$createComponent(OperationalStatusBlock, {
          get snapshot() {
            return props.snapshot();
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
          title: "Live Minions",
          shortTitle: "Minions",
          get badge() {
            return _$memo(() => props.minions().length > 0)() ? `${runningMinions()} run / ${props.minions().length} tot` : void 0;
          },
          get shortBadge() {
            return _$memo(() => props.minions().length > 0)() ? `${runningMinions()}/${props.minions().length}` : void 0;
          },
          get summary() {
            return `${runningMinions()} run \xB7 ${props.minions().length} tot`;
          },
          get compact() {
            return layout().compact;
          },
          get densityCompact() {
            return props.density() === "compact";
          },
          get expanded() {
            return props.minionsExpanded;
          },
          get onToggle() {
            return props.toggleMinions;
          },
          get theme() {
            return props.theme;
          },
          get children() {
            return _$createComponent(MinionRows, {
              get rows() {
                return props.minions();
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
              },
              get onActivate() {
                return props.onActivateMinion;
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
    return _el$89;
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
      filled: "\u25B0".repeat(filled),
      empty: "\u25B1".repeat(segments - filled)
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
      var _el$95 = _$createElement("box");
      _$setProp(_el$95, "flexDirection", "row");
      _$setProp(_el$95, "paddingLeft", 1);
      _$setProp(_el$95, "paddingRight", 1);
      _$insert(_el$95, _$createComponent(Show, {
        get when() {
          return props.metrics().tokensUsed !== void 0;
        },
        get children() {
          return [(() => {
            var _el$96 = _$createElement("text");
            _$insertNode(_el$96, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$96, "fg", palette.border, _$p));
            return _el$96;
          })(), _$createComponent(Show, {
            get when() {
              return contextGauge();
            },
            get fallback() {
              return (() => {
                var _el$101 = _$createElement("text");
                _$insert(_el$101, () => `\u25C6 ${formatTokens(props.metrics().tokensUsed)} tok`);
                _$effect((_$p) => _$setProp(_el$101, "fg", palette.sky, _$p));
                return _el$101;
              })();
            },
            children: (gauge) => (() => {
              var _el$102 = _$createElement("box"), _el$103 = _$createElement("text"), _el$105 = _$createElement("text"), _el$106 = _$createElement("text"), _el$107 = _$createElement("text"), _el$108 = _$createElement("text"), _el$110 = _$createElement("text");
              _$insertNode(_el$102, _el$103);
              _$insertNode(_el$102, _el$105);
              _$insertNode(_el$102, _el$106);
              _$insertNode(_el$102, _el$107);
              _$insertNode(_el$102, _el$108);
              _$insertNode(_el$102, _el$110);
              _$setProp(_el$102, "flexDirection", "row");
              _$insertNode(_el$103, _$createTextNode(`\u25C6 `));
              _$insert(_el$105, () => gauge().filled);
              _$insert(_el$106, () => gauge().empty);
              _$insert(_el$107, () => ` ${contextPct()}%`);
              _$insertNode(_el$108, _$createTextNode(` \xB7 `));
              _$insert(_el$110, () => formatTokens(props.metrics().tokensUsed));
              _$effect((_p$) => {
                var _v$48 = contextColor(), _v$49 = contextColor(), _v$50 = palette.border, _v$51 = contextColor(), _v$52 = palette.border, _v$53 = palette.sky;
                _v$48 !== _p$.e && (_p$.e = _$setProp(_el$103, "fg", _v$48, _p$.e));
                _v$49 !== _p$.t && (_p$.t = _$setProp(_el$105, "fg", _v$49, _p$.t));
                _v$50 !== _p$.a && (_p$.a = _$setProp(_el$106, "fg", _v$50, _p$.a));
                _v$51 !== _p$.o && (_p$.o = _$setProp(_el$107, "fg", _v$51, _p$.o));
                _v$52 !== _p$.i && (_p$.i = _$setProp(_el$108, "fg", _v$52, _p$.i));
                _v$53 !== _p$.n && (_p$.n = _$setProp(_el$110, "fg", _v$53, _p$.n));
                return _p$;
              }, {
                e: void 0,
                t: void 0,
                a: void 0,
                o: void 0,
                i: void 0,
                n: void 0
              });
              return _el$102;
            })()
          })];
        }
      }), null);
      _$insert(_el$95, _$createComponent(Show, {
        get when() {
          return props.metrics().cost !== void 0;
        },
        get children() {
          return [(() => {
            var _el$98 = _$createElement("text");
            _$insertNode(_el$98, _$createTextNode(` \u2502 `));
            _$effect((_$p) => _$setProp(_el$98, "fg", palette.border, _$p));
            return _el$98;
          })(), (() => {
            var _el$100 = _$createElement("text");
            _$insert(_el$100, () => `$ ${formatCost(props.metrics().cost)}`);
            _$effect((_$p) => _$setProp(_el$100, "fg", palette.warning, _$p));
            return _el$100;
          })()];
        }
      }), null);
      return _el$95;
    }
  });
}
function HomeBottomStatus(props) {
  const activeTask = createMemo(() => props.snapshot().tasks.find((t) => t.status === "in_progress"));
  const counts = createMemo(() => operationalCounts(props.snapshot(), props.snapshotError()));
  const visible = createMemo(() => counts().active > 0 || counts().review > 0 || counts().attention > 0 || props.minions().length > 0);
  const palette = resolvePalette(props.theme);
  return _$createComponent(Show, {
    get when() {
      return visible();
    },
    get children() {
      var _el$111 = _$createElement("box"), _el$112 = _$createElement("text"), _el$113 = _$createElement("text"), _el$115 = _$createElement("text"), _el$117 = _$createElement("text"), _el$119 = _$createElement("text");
      _$insertNode(_el$111, _el$112);
      _$insertNode(_el$111, _el$113);
      _$insertNode(_el$111, _el$115);
      _$insertNode(_el$111, _el$117);
      _$insertNode(_el$111, _el$119);
      _$setProp(_el$111, "paddingLeft", 1);
      _$setProp(_el$111, "paddingRight", 1);
      _$setProp(_el$111, "flexDirection", "row");
      _$insert(_el$112, () => `${GLYPH.brand} `);
      _$insertNode(_el$113, _$createTextNode(`CORTEX`));
      _$insertNode(_el$115, _$createTextNode(`\xB7`));
      _$insertNode(_el$117, _$createTextNode(`IA `));
      _$insertNode(_el$119, _$createTextNode(`\u2502 `));
      _$insert(_el$111, _$createComponent(Show, {
        get when() {
          return activeTask();
        },
        get fallback() {
          return (() => {
            var _el$121 = _$createElement("box"), _el$122 = _$createElement("text"), _el$123 = _$createElement("text"), _el$125 = _$createElement("text");
            _$insertNode(_el$121, _el$122);
            _$insertNode(_el$121, _el$123);
            _$insertNode(_el$121, _el$125);
            _$setProp(_el$121, "flexDirection", "row");
            _$insert(_el$122, () => `\u25CF ${counts().active} active`);
            _$insertNode(_el$123, _$createTextNode(` \xB7 `));
            _$insert(_el$125, () => `\u25C6 ${counts().review} rev`);
            _$insert(_el$121, _$createComponent(Show, {
              get when() {
                return props.minions().length > 0;
              },
              get children() {
                return [(() => {
                  var _el$126 = _$createElement("text");
                  _$insertNode(_el$126, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$126, "fg", palette.border, _$p));
                  return _el$126;
                })(), (() => {
                  var _el$128 = _$createElement("text");
                  _$insert(_el$128, () => `\u25B8 ${props.minions().length} minions`);
                  _$effect((_$p) => _$setProp(_el$128, "fg", palette.textSoft, _$p));
                  return _el$128;
                })()];
              }
            }), null);
            _$insert(_el$121, _$createComponent(Show, {
              get when() {
                return counts().attention > 0;
              },
              get children() {
                return [(() => {
                  var _el$129 = _$createElement("text");
                  _$insertNode(_el$129, _$createTextNode(` \xB7 `));
                  _$effect((_$p) => _$setProp(_el$129, "fg", palette.border, _$p));
                  return _el$129;
                })(), (() => {
                  var _el$131 = _$createElement("text");
                  _$insert(_el$131, () => `\u2715 ${counts().attention} alert`);
                  _$effect((_$p) => _$setProp(_el$131, "fg", palette.error, _$p));
                  return _el$131;
                })()];
              }
            }), null);
            _$effect((_p$) => {
              var _v$59 = palette.warning, _v$60 = palette.border, _v$61 = palette.accent;
              _v$59 !== _p$.e && (_p$.e = _$setProp(_el$122, "fg", _v$59, _p$.e));
              _v$60 !== _p$.t && (_p$.t = _$setProp(_el$123, "fg", _v$60, _p$.t));
              _v$61 !== _p$.a && (_p$.a = _$setProp(_el$125, "fg", _v$61, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$121;
          })();
        },
        children: (task) => (() => {
          var _el$132 = _$createElement("box"), _el$133 = _$createElement("text"), _el$134 = _$createElement("text"), _el$135 = _$createElement("text"), _el$136 = _$createElement("text");
          _$insertNode(_el$132, _el$133);
          _$insertNode(_el$132, _el$134);
          _$insertNode(_el$132, _el$135);
          _$insertNode(_el$132, _el$136);
          _$setProp(_el$132, "flexDirection", "row");
          _$setProp(_el$132, "onMouseDown", () => openWebConsole(task().board_id, task().task_id));
          _$insert(_el$133, () => `[${props.spinner()} ${task().task_id}] `);
          _$insert(_el$134, () => clipped(task().title, 20));
          _$insert(_el$135, () => ` \xB7 ${counts().active} active`);
          _$insert(_el$136, () => ` [${GLYPH.web}]`);
          _$effect((_p$) => {
            var _v$62 = palette.warning, _v$63 = palette.text, _v$64 = palette.textMuted, _v$65 = palette.info;
            _v$62 !== _p$.e && (_p$.e = _$setProp(_el$133, "fg", _v$62, _p$.e));
            _v$63 !== _p$.t && (_p$.t = _$setProp(_el$134, "fg", _v$63, _p$.t));
            _v$64 !== _p$.a && (_p$.a = _$setProp(_el$135, "fg", _v$64, _p$.a));
            _v$65 !== _p$.o && (_p$.o = _$setProp(_el$136, "fg", _v$65, _p$.o));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0,
            o: void 0
          });
          return _el$132;
        })()
      }), null);
      _$effect((_p$) => {
        var _v$54 = palette.accentAlt, _v$55 = palette.text, _v$56 = palette.info, _v$57 = palette.sky, _v$58 = palette.border;
        _v$54 !== _p$.e && (_p$.e = _$setProp(_el$112, "fg", _v$54, _p$.e));
        _v$55 !== _p$.t && (_p$.t = _$setProp(_el$113, "fg", _v$55, _p$.t));
        _v$56 !== _p$.a && (_p$.a = _$setProp(_el$115, "fg", _v$56, _p$.a));
        _v$57 !== _p$.o && (_p$.o = _$setProp(_el$117, "fg", _v$57, _p$.o));
        _v$58 !== _p$.i && (_p$.i = _$setProp(_el$119, "fg", _v$58, _p$.i));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0,
        i: void 0
      });
      return _el$111;
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
    var _el$137 = _$createElement("box"), _el$138 = _$createElement("text"), _el$139 = _$createElement("text");
    _$insertNode(_el$137, _el$138);
    _$insertNode(_el$137, _el$139);
    _$setProp(_el$137, "flexDirection", "column");
    _$setProp(_el$137, "marginTop", 1);
    _$setProp(_el$137, "border", ["bottom"]);
    _$setProp(_el$138, "wrapMode", "none");
    _$setProp(_el$138, "truncate", true);
    _$insert(_el$138, () => `${meta.glyph} ${props.task.task_id}`);
    _$setProp(_el$139, "wrapMode", "none");
    _$setProp(_el$139, "truncate", true);
    _$insert(_el$139, () => props.task.title);
    _$insert(_el$137, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.kind === "in_progress")() ? props.task.owner : void 0;
      },
      children: (owner) => (() => {
        var _el$142 = _$createElement("text");
        _$setProp(_el$142, "wrapMode", "none");
        _$setProp(_el$142, "truncate", true);
        _$insert(_el$142, () => `Claim: ${owner()}`);
        _$effect((_$p) => _$setProp(_el$142, "fg", props.palette.info, _$p));
        return _el$142;
      })()
    }), null);
    _$insert(_el$137, _$createComponent(Show, {
      get when() {
        return props.kind === "in_review";
      },
      get children() {
        var _el$140 = _$createElement("text");
        _$insertNode(_el$140, _$createTextNode(`awaiting reviewer`));
        _$setProp(_el$140, "wrapMode", "none");
        _$setProp(_el$140, "truncate", true);
        _$effect((_$p) => _$setProp(_el$140, "fg", props.palette.warning, _$p));
        return _el$140;
      }
    }), null);
    _$effect((_p$) => {
      var _v$66 = props.palette.borderSubtle, _v$67 = props.palette[meta.id], _v$68 = TextAttributes.BOLD, _v$69 = props.palette[meta.body];
      _v$66 !== _p$.e && (_p$.e = _$setProp(_el$137, "borderColor", _v$66, _p$.e));
      _v$67 !== _p$.t && (_p$.t = _$setProp(_el$138, "fg", _v$67, _p$.t));
      _v$68 !== _p$.a && (_p$.a = _$setProp(_el$138, "attributes", _v$68, _p$.a));
      _v$69 !== _p$.o && (_p$.o = _$setProp(_el$139, "fg", _v$69, _p$.o));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0
    });
    return _el$137;
  })();
}
function KanbanGroup(props) {
  const meta = KANBAN_GROUP_META[props.kind];
  return (() => {
    var _el$143 = _$createElement("box"), _el$144 = _$createElement("text");
    _$insertNode(_el$143, _el$144);
    _$setProp(_el$143, "flexDirection", "column");
    _$setProp(_el$143, "marginBottom", 1);
    _$setProp(_el$144, "wrapMode", "none");
    _$setProp(_el$144, "truncate", true);
    _$insert(_el$144, () => `${meta.glyph} ${meta.label} (${props.tasks().length})`);
    _$insert(_el$143, _$createComponent(For, {
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
      var _v$70 = props.palette[meta.header], _v$71 = TextAttributes.BOLD;
      _v$70 !== _p$.e && (_p$.e = _$setProp(_el$144, "fg", _v$70, _p$.e));
      _v$71 !== _p$.t && (_p$.t = _$setProp(_el$144, "attributes", _v$71, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$143;
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
    var _el$145 = _$createElement("box"), _el$146 = _$createElement("box"), _el$147 = _$createElement("text"), _el$148 = _$createElement("text"), _el$149 = _$createElement("box");
    _$insertNode(_el$145, _el$146);
    _$insertNode(_el$145, _el$149);
    _$setProp(_el$145, "flexDirection", "column");
    _$setProp(_el$145, "padding", 1);
    _$insertNode(_el$146, _el$147);
    _$insertNode(_el$146, _el$148);
    _$setProp(_el$146, "flexDirection", "row");
    _$setProp(_el$146, "marginBottom", 1);
    _$setProp(_el$147, "wrapMode", "none");
    _$setProp(_el$147, "truncate", true);
    _$insert(_el$147, () => `${GLYPH.web} CORTEX \xB7 IA KANBAN DECK [${props.pulse()}] `);
    _$insert(_el$148, () => `(${tasks().length} tasks \xB7 ${props.minions().length} minions)`);
    _$setProp(_el$149, "flexDirection", "row");
    _$insert(_el$149, _$createComponent(For, {
      get each() {
        return columns();
      },
      children: (column, index) => (() => {
        var _el$150 = _$createElement("box");
        _$setProp(_el$150, "flexDirection", "column");
        _$insert(_el$150, _$createComponent(For, {
          each: column,
          children: (kind) => _$createComponent(KanbanGroup, {
            kind,
            tasks: () => grouped()[kind],
            palette
          })
        }));
        _$effect((_p$) => {
          var _v$75 = columnWidth(), _v$76 = index() > 0 ? KANBAN_COLUMN_BORDER : false, _v$77 = palette.border;
          _v$75 !== _p$.e && (_p$.e = _$setProp(_el$150, "width", _v$75, _p$.e));
          _v$76 !== _p$.t && (_p$.t = _$setProp(_el$150, "border", _v$76, _p$.t));
          _v$77 !== _p$.a && (_p$.a = _$setProp(_el$150, "borderColor", _v$77, _p$.a));
          return _p$;
        }, {
          e: void 0,
          t: void 0,
          a: void 0
        });
        return _el$150;
      })()
    }));
    _$effect((_p$) => {
      var _v$72 = palette.info, _v$73 = TextAttributes.BOLD, _v$74 = palette.textMuted;
      _v$72 !== _p$.e && (_p$.e = _$setProp(_el$147, "fg", _v$72, _p$.e));
      _v$73 !== _p$.t && (_p$.t = _$setProp(_el$147, "attributes", _v$73, _p$.t));
      _v$74 !== _p$.a && (_p$.a = _$setProp(_el$148, "fg", _v$74, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$145;
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
    var _el$151 = _$createElement("box");
    _$setProp(_el$151, "flexDirection", "column");
    _$setProp(_el$151, "alignItems", "center");
    _$setProp(_el$151, "marginBottom", 1);
    _$insert(_el$151, _$createComponent(Show, {
      get when() {
        return isLarge();
      },
      get fallback() {
        return (() => {
          var _el$153 = _$createElement("box"), _el$154 = _$createElement("text"), _el$155 = _$createElement("text");
          _$insertNode(_el$153, _el$154);
          _$insertNode(_el$153, _el$155);
          _$setProp(_el$153, "flexDirection", "column");
          _$setProp(_el$153, "alignItems", "center");
          _$insert(_el$154, () => `${GLYPH.active} CORTEX \xB7 IA ${GLYPH.active}`);
          _$insertNode(_el$155, _$createTextNode(`[Adaptive Cognitive Control Plane]`));
          _$effect((_p$) => {
            var _v$78 = palette.info, _v$79 = TextAttributes.BOLD, _v$80 = palette.textMuted;
            _v$78 !== _p$.e && (_p$.e = _$setProp(_el$154, "fg", _v$78, _p$.e));
            _v$79 !== _p$.t && (_p$.t = _$setProp(_el$154, "attributes", _v$79, _p$.t));
            _v$80 !== _p$.a && (_p$.a = _$setProp(_el$155, "fg", _v$80, _p$.a));
            return _p$;
          }, {
            e: void 0,
            t: void 0,
            a: void 0
          });
          return _el$153;
        })();
      },
      get children() {
        return [_$createComponent(For, {
          each: CORTEX_LOGO_BRAILLE,
          children: (line, index) => {
            return (() => {
              var _el$157 = _$createElement("text");
              _$insert(_el$157, line);
              _$effect((_$p) => _$setProp(_el$157, "fg", logoLineColor(index(), palette), _$p));
              return _el$157;
            })();
          }
        }), (() => {
          var _el$152 = _$createElement("text");
          _$setProp(_el$152, "marginTop", 1);
          _$insert(_el$152, () => `${GLYPH.active} OpenCode Multi-Agent Control Plane & Task DAG ${GLYPH.active}`);
          _$effect((_$p) => _$setProp(_el$152, "fg", palette.textMuted, _$p));
          return _el$152;
        })()];
      }
    }));
    return _el$151;
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
        var _el$158 = _$createElement("box"), _el$159 = _$createElement("box"), _el$160 = _$createElement("box"), _el$161 = _$createElement("text"), _el$162 = _$createElement("text"), _el$165 = _$createElement("box"), _el$166 = _$createElement("text"), _el$167 = _$createElement("text"), _el$169 = _$createElement("text"), _el$170 = _$createElement("text"), _el$172 = _$createElement("text"), _el$176 = _$createElement("box"), _el$177 = _$createElement("text"), _el$178 = _$createElement("text"), _el$180 = _$createElement("text"), _el$184 = _$createElement("text");
        _$insertNode(_el$158, _el$159);
        _$insertNode(_el$158, _el$165);
        _$insertNode(_el$158, _el$176);
        _$setProp(_el$158, "flexDirection", "column");
        _$setProp(_el$158, "borderStyle", "single");
        _$setProp(_el$158, "paddingLeft", 1);
        _$setProp(_el$158, "paddingRight", 1);
        _$setProp(_el$158, "marginTop", 1);
        _$setProp(_el$158, "marginBottom", 1);
        _$setProp(_el$158, "width", "100%");
        _$setProp(_el$158, "onMouseDown", () => openStatsView());
        _$insertNode(_el$159, _el$160);
        _$setProp(_el$159, "flexDirection", "row");
        _$setProp(_el$159, "justifyContent", "space-between");
        _$insertNode(_el$160, _el$161);
        _$insertNode(_el$160, _el$162);
        _$setProp(_el$160, "flexDirection", "row");
        _$insert(_el$161, () => `${GLYPH.brand} CORTEX \xB7 IA `);
        _$insertNode(_el$162, _$createTextNode(`Estad\xEDsticas de Uso`));
        _$insert(_el$159, _$createComponent(Show, {
          get when() {
            return _$memo(() => !!s().first_day)() && s().last_day;
          },
          get children() {
            var _el$164 = _$createElement("text");
            _$insert(_el$164, () => `${s().first_day} \u2192 ${s().last_day}`);
            _$effect((_$p) => _$setProp(_el$164, "fg", palette.textMuted, _$p));
            return _el$164;
          }
        }), null);
        _$insertNode(_el$165, _el$166);
        _$insertNode(_el$165, _el$167);
        _$insertNode(_el$165, _el$169);
        _$insertNode(_el$165, _el$170);
        _$insertNode(_el$165, _el$172);
        _$setProp(_el$165, "flexDirection", "row");
        _$setProp(_el$165, "marginTop", 0);
        _$insert(_el$166, () => `\u25CF ${formatInteger(s().sessions)} sesiones`);
        _$insertNode(_el$167, _$createTextNode(`\u2502`));
        _$insert(_el$169, () => `\u25B8 ${formatInteger(s().messages)} mensajes`);
        _$insertNode(_el$170, _$createTextNode(`\u2502`));
        _$insert(_el$172, () => `\u25C6 ${formatLargeTokens(s().tokens)} tokens`);
        _$insert(_el$165, _$createComponent(Show, {
          get when() {
            return !isNarrow();
          },
          get children() {
            return [(() => {
              var _el$173 = _$createElement("text");
              _$insertNode(_el$173, _$createTextNode(`\u2502`));
              _$effect((_$p) => _$setProp(_el$173, "fg", palette.border, _$p));
              return _el$173;
            })(), (() => {
              var _el$175 = _$createElement("text");
              _$insert(_el$175, () => `\u25B8 ${s().active_days} d\xEDas activos`);
              _$effect((_$p) => _$setProp(_el$175, "fg", palette.warning, _$p));
              return _el$175;
            })()];
          }
        }), null);
        _$insertNode(_el$176, _el$177);
        _$insertNode(_el$176, _el$178);
        _$insertNode(_el$176, _el$180);
        _$insertNode(_el$176, _el$184);
        _$setProp(_el$176, "flexDirection", "row");
        _$setProp(_el$176, "marginTop", 0);
        _$insert(_el$177, () => `\u25C6 Top: ${s().favorite_model || "-"} (${(s().favorite_model_share ?? 0).toFixed(1)}%)`);
        _$insertNode(_el$178, _$createTextNode(`\u2502`));
        _$insert(_el$180, () => `\u25B8 Pico: ${String(s().peak_hour).padStart(2, "0")}:00`);
        _$insert(_el$176, _$createComponent(Show, {
          get when() {
            return isNarrow();
          },
          get children() {
            return [(() => {
              var _el$181 = _$createElement("text");
              _$insertNode(_el$181, _$createTextNode(`\u2502`));
              _$effect((_$p) => _$setProp(_el$181, "fg", palette.border, _$p));
              return _el$181;
            })(), (() => {
              var _el$183 = _$createElement("text");
              _$insert(_el$183, () => `\u25B8 ${s().active_days}d`);
              _$effect((_$p) => _$setProp(_el$183, "fg", palette.warning, _$p));
              return _el$183;
            })()];
          }
        }), _el$184);
        _$insertNode(_el$184, _$createTextNode(` \xB7 [:cortex-stats para panel interactivo]`));
        _$effect((_p$) => {
          var _v$81 = palette.border, _v$82 = palette.accent, _v$83 = TextAttributes.BOLD, _v$84 = palette.text, _v$85 = TextAttributes.BOLD, _v$86 = isNarrow() ? 1 : 2, _v$87 = palette.sky, _v$88 = palette.border, _v$89 = palette.info, _v$90 = palette.border, _v$91 = palette.success, _v$92 = TextAttributes.BOLD, _v$93 = isNarrow() ? 1 : 2, _v$94 = palette.accentAlt, _v$95 = palette.border, _v$96 = palette.sky, _v$97 = palette.textMuted;
          _v$81 !== _p$.e && (_p$.e = _$setProp(_el$158, "borderColor", _v$81, _p$.e));
          _v$82 !== _p$.t && (_p$.t = _$setProp(_el$161, "fg", _v$82, _p$.t));
          _v$83 !== _p$.a && (_p$.a = _$setProp(_el$161, "attributes", _v$83, _p$.a));
          _v$84 !== _p$.o && (_p$.o = _$setProp(_el$162, "fg", _v$84, _p$.o));
          _v$85 !== _p$.i && (_p$.i = _$setProp(_el$162, "attributes", _v$85, _p$.i));
          _v$86 !== _p$.n && (_p$.n = _$setProp(_el$165, "gap", _v$86, _p$.n));
          _v$87 !== _p$.s && (_p$.s = _$setProp(_el$166, "fg", _v$87, _p$.s));
          _v$88 !== _p$.h && (_p$.h = _$setProp(_el$167, "fg", _v$88, _p$.h));
          _v$89 !== _p$.r && (_p$.r = _$setProp(_el$169, "fg", _v$89, _p$.r));
          _v$90 !== _p$.d && (_p$.d = _$setProp(_el$170, "fg", _v$90, _p$.d));
          _v$91 !== _p$.l && (_p$.l = _$setProp(_el$172, "fg", _v$91, _p$.l));
          _v$92 !== _p$.u && (_p$.u = _$setProp(_el$172, "attributes", _v$92, _p$.u));
          _v$93 !== _p$.c && (_p$.c = _$setProp(_el$176, "gap", _v$93, _p$.c));
          _v$94 !== _p$.w && (_p$.w = _$setProp(_el$177, "fg", _v$94, _p$.w));
          _v$95 !== _p$.m && (_p$.m = _$setProp(_el$178, "fg", _v$95, _p$.m));
          _v$96 !== _p$.f && (_p$.f = _$setProp(_el$180, "fg", _v$96, _p$.f));
          _v$97 !== _p$.y && (_p$.y = _$setProp(_el$184, "fg", _v$97, _p$.y));
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
        return _el$158;
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
      var _el$186 = _$createElement("box"), _el$187 = _$createElement("text"), _el$188 = _$createElement("text"), _el$190 = _$createElement("text"), _el$192 = _$createElement("text");
      _$insertNode(_el$186, _el$187);
      _$insertNode(_el$186, _el$188);
      _$insertNode(_el$186, _el$190);
      _$insertNode(_el$186, _el$192);
      _$setProp(_el$186, "flexDirection", "row");
      _$setProp(_el$186, "paddingLeft", 1);
      _$setProp(_el$186, "paddingRight", 1);
      _$insert(_el$187, () => `${GLYPH.active} nan `);
      _$insert(_el$186, _$createComponent(Show, {
        get when() {
          return view().percent !== void 0;
        },
        get fallback() {
          return (() => {
            var _el$193 = _$createElement("text");
            _$insertNode(_el$193, _$createTextNode(`quota n/a `));
            _$effect((_$p) => _$setProp(_el$193, "fg", palette.textMuted, _$p));
            return _el$193;
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
      }), _el$188);
      _$insertNode(_el$188, _$createTextNode(` \xB7 `));
      _$insertNode(_el$190, _$createTextNode(`24h `));
      _$insert(_el$192, () => formatLargeTokens(view().burn));
      _$effect((_p$) => {
        var _v$98 = palette.accent, _v$99 = palette.borderSubtle, _v$100 = palette.textMuted, _v$101 = palette.sky;
        _v$98 !== _p$.e && (_p$.e = _$setProp(_el$187, "fg", _v$98, _p$.e));
        _v$99 !== _p$.t && (_p$.t = _$setProp(_el$188, "fg", _v$99, _p$.t));
        _v$100 !== _p$.a && (_p$.a = _$setProp(_el$190, "fg", _v$100, _p$.a));
        _v$101 !== _p$.o && (_p$.o = _$setProp(_el$192, "fg", _v$101, _p$.o));
        return _p$;
      }, {
        e: void 0,
        t: void 0,
        a: void 0,
        o: void 0
      });
      return _el$186;
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
    var _el$195 = _$createElement("box"), _el$196 = _$createElement("text"), _el$197 = _$createElement("text");
    _$insertNode(_el$195, _el$196);
    _$insertNode(_el$195, _el$197);
    _$setProp(_el$195, "flexDirection", "column");
    _$setProp(_el$195, "padding", 1);
    _$insert(_el$196, () => `${GLYPH.web} nan model picker \xB7 browse only, preview then apply`);
    _$insertNode(_el$197, _$createTextNode(`pick a model, an effort, and an agent \xB7 the ref form is nan/&lt;model>`));
    _$insert(_el$195, _$createComponent(Show, {
      get when() {
        return props.loadError() !== "";
      },
      get children() {
        var _el$199 = _$createElement("text");
        _$insert(_el$199, () => `catalog unavailable (${props.loadError()}) \xB7 no assignment attempted`);
        _$effect((_$p) => _$setProp(_el$199, "fg", palette.error, _$p));
        return _el$199;
      }
    }), null);
    _$insert(_el$195, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.loading();
      },
      get children() {
        var _el$200 = _$createElement("text");
        _$insertNode(_el$200, _$createTextNode(`reading nan catalog\u2026`));
        _$effect((_$p) => _$setProp(_el$200, "fg", palette.textMuted, _$p));
        return _el$200;
      }
    }), null);
    _$insert(_el$195, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.loadError() === "" && !props.loading()))() && props.models().length === 0;
      },
      get children() {
        var _el$202 = _$createElement("text");
        _$insertNode(_el$202, _$createTextNode(`no picker-eligible nan chat models in the catalog`));
        _$effect((_$p) => _$setProp(_el$202, "fg", palette.textMuted, _$p));
        return _el$202;
      }
    }), null);
    _$insert(_el$195, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.models().length > 0;
      },
      get children() {
        return [(() => {
          var _el$204 = _$createElement("text");
          _$insertNode(_el$204, _$createTextNode(`MODEL`));
          _$effect((_p$) => {
            var _v$102 = palette.textSoft, _v$103 = TextAttributes.BOLD;
            _v$102 !== _p$.e && (_p$.e = _$setProp(_el$204, "fg", _v$102, _p$.e));
            _v$103 !== _p$.t && (_p$.t = _$setProp(_el$204, "attributes", _v$103, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$204;
        })(), _$createComponent(For, {
          get each() {
            return props.models();
          },
          children: (model) => (() => {
            var _el$225 = _$createElement("box"), _el$226 = _$createElement("text"), _el$227 = _$createElement("text"), _el$228 = _$createElement("text");
            _$insertNode(_el$225, _el$226);
            _$insertNode(_el$225, _el$227);
            _$insertNode(_el$225, _el$228);
            _$setProp(_el$225, "flexDirection", "row");
            _$setProp(_el$225, "onMouseDown", () => {
              setModelID(model.model);
              setLevel("");
              props.onSelectionChange();
            });
            _$insert(_el$226, () => activeModel()?.model === model.model ? "\u25B8 " : "  ");
            _$insert(_el$227, () => `nan/${model.model}`);
            _$insert(_el$228, () => ` \xB7 ${nanEffortModeBadge(model.effortMode)}`);
            _$insert(_el$225, _$createComponent(Show, {
              get when() {
                return model.preferredOver !== "";
              },
              get children() {
                var _el$229 = _$createElement("text");
                _$insert(_el$229, () => ` \xB7 preferred over ${model.preferredOver}`);
                _$effect((_$p) => _$setProp(_el$229, "fg", palette.success, _$p));
                return _el$229;
              }
            }), null);
            _$insert(_el$225, _$createComponent(Show, {
              get when() {
                return model.tier === "legacy";
              },
              get children() {
                var _el$230 = _$createElement("text");
                _$insertNode(_el$230, _$createTextNode(` \xB7 legacy \xB7 deprioritized`));
                _$effect((_$p) => _$setProp(_el$230, "fg", palette.warning, _$p));
                return _el$230;
              }
            }), null);
            _$effect((_p$) => {
              var _v$117 = palette.border, _v$118 = model.tier === "legacy" ? palette.textMuted : palette.text, _v$119 = palette.textMuted;
              _v$117 !== _p$.e && (_p$.e = _$setProp(_el$226, "fg", _v$117, _p$.e));
              _v$118 !== _p$.t && (_p$.t = _$setProp(_el$227, "fg", _v$118, _p$.t));
              _v$119 !== _p$.a && (_p$.a = _$setProp(_el$228, "fg", _v$119, _p$.a));
              return _p$;
            }, {
              e: void 0,
              t: void 0,
              a: void 0
            });
            return _el$225;
          })()
        }), (() => {
          var _el$206 = _$createElement("text");
          _$insertNode(_el$206, _$createTextNode(`EFFORT`));
          _$effect((_p$) => {
            var _v$104 = palette.textSoft, _v$105 = TextAttributes.BOLD;
            _v$104 !== _p$.e && (_p$.e = _$setProp(_el$206, "fg", _v$104, _p$.e));
            _v$105 !== _p$.t && (_p$.t = _$setProp(_el$206, "attributes", _v$105, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$206;
        })(), _$createComponent(Show, {
          get when() {
            return needsEffort();
          },
          get fallback() {
            return (() => {
              var _el$232 = _$createElement("text");
              _$insert(_el$232, () => `no adjustable depth for nan/${activeModel()?.model ?? ""} \xB7 the reference is written without --effort`);
              _$effect((_$p) => _$setProp(_el$232, "fg", palette.textMuted, _$p));
              return _el$232;
            })();
          },
          get children() {
            var _el$208 = _$createElement("box");
            _$setProp(_el$208, "flexDirection", "row");
            _$insert(_el$208, _$createComponent(For, {
              get each() {
                return activeModel()?.vocabulary ?? [];
              },
              children: (option) => (() => {
                var _el$233 = _$createElement("text");
                _$setProp(_el$233, "onMouseDown", () => {
                  setLevel(option);
                  props.onSelectionChange();
                });
                _$insert(_el$233, () => `[${level() === option ? "\u25B8" : " "} ${option}${NAN_REASONING_OFF_LEVELS.includes(option) ? " \xB7skips reasoning" : ""}] `);
                _$effect((_$p) => _$setProp(_el$233, "fg", level() === option ? palette.success : palette.text, _$p));
                return _el$233;
              })()
            }));
            return _el$208;
          }
        }), (() => {
          var _el$209 = _$createElement("text");
          _$insert(_el$209, () => `AGENT \xB7 EFFECTIVE MAPPING (${agents().length})`);
          _$effect((_p$) => {
            var _v$106 = palette.textSoft, _v$107 = TextAttributes.BOLD;
            _v$106 !== _p$.e && (_p$.e = _$setProp(_el$209, "fg", _v$106, _p$.e));
            _v$107 !== _p$.t && (_p$.t = _$setProp(_el$209, "attributes", _v$107, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$209;
        })(), _$createComponent(For, {
          get each() {
            return agents();
          },
          children: (agent) => (() => {
            var _el$234 = _$createElement("text");
            _$setProp(_el$234, "onMouseDown", () => {
              setAgentID(agent.agent);
              props.onSelectionChange();
            });
            _$insert(_el$234, () => `[${activeAgent()?.agent === agent.agent ? "\u25B8" : " "} ${agent.agent}: ${agent.value}] `);
            _$effect((_$p) => _$setProp(_el$234, "fg", activeAgent()?.agent === agent.agent ? palette.info : palette.textMuted, _$p));
            return _el$234;
          })()
        }), _$createComponent(Show, {
          get when() {
            return _$memo(() => !!!props.loading())() && agents().length === 0;
          },
          get children() {
            var _el$210 = _$createElement("text");
            _$insertNode(_el$210, _$createTextNode(`agent list unavailable \xB7 confirmation is disabled`));
            _$effect((_$p) => _$setProp(_el$210, "fg", palette.warning, _$p));
            return _el$210;
          }
        }), (() => {
          var _el$212 = _$createElement("box"), _el$213 = _$createElement("text"), _el$217 = _$createElement("text");
          _$insertNode(_el$212, _el$213);
          _$insertNode(_el$212, _el$217);
          _$setProp(_el$212, "flexDirection", "row");
          _$setProp(_el$212, "marginTop", 1);
          _$insertNode(_el$213, _$createTextNode(`[ preview dry-run ]`));
          _$setProp(_el$213, "onMouseDown", () => dispatch(false));
          _$insert(_el$212, _$createComponent(Show, {
            get when() {
              return props.preview() !== "";
            },
            get children() {
              var _el$215 = _$createElement("text");
              _$insertNode(_el$215, _$createTextNode(`  [ apply ]`));
              _$setProp(_el$215, "onMouseDown", () => dispatch(true));
              _$effect((_$p) => _$setProp(_el$215, "fg", palette.success, _$p));
              return _el$215;
            }
          }), _el$217);
          _$insert(_el$217, (() => {
            var _c$2 = _$memo(() => props.phase() === "previewing");
            return () => _c$2() ? "  running dry-run\u2026" : props.phase() === "applying" ? "  applying\u2026" : "";
          })());
          _$effect((_p$) => {
            var _v$108 = ready() ? palette.info : palette.textMuted, _v$109 = palette.textMuted;
            _v$108 !== _p$.e && (_p$.e = _$setProp(_el$213, "fg", _v$108, _p$.e));
            _v$109 !== _p$.t && (_p$.t = _$setProp(_el$217, "fg", _v$109, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$212;
        })(), _$createComponent(Show, {
          get when() {
            return props.runError() !== "";
          },
          get children() {
            var _el$218 = _$createElement("text");
            _$insert(_el$218, () => `model set failed \xB7 ${props.runError()} \u2014 the effective mapping above is unchanged`);
            _$effect((_$p) => _$setProp(_el$218, "fg", palette.error, _$p));
            return _el$218;
          }
        }), _$createComponent(Show, {
          get when() {
            return props.preview() !== "";
          },
          get children() {
            return [(() => {
              var _el$219 = _$createElement("text");
              _$insertNode(_el$219, _$createTextNode(`dry-run receipt (nothing written)`));
              _$effect((_p$) => {
                var _v$110 = palette.textSoft, _v$111 = TextAttributes.BOLD;
                _v$110 !== _p$.e && (_p$.e = _$setProp(_el$219, "fg", _v$110, _p$.e));
                _v$111 !== _p$.t && (_p$.t = _$setProp(_el$219, "attributes", _v$111, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$219;
            })(), (() => {
              var _el$221 = _$createElement("text");
              _$insert(_el$221, () => props.preview());
              _$effect((_$p) => _$setProp(_el$221, "fg", palette.textMuted, _$p));
              return _el$221;
            })()];
          }
        }), _$createComponent(Show, {
          get when() {
            return props.result() !== "";
          },
          get children() {
            return [(() => {
              var _el$222 = _$createElement("text");
              _$insertNode(_el$222, _$createTextNode(`applied receipt`));
              _$effect((_p$) => {
                var _v$112 = palette.success, _v$113 = TextAttributes.BOLD;
                _v$112 !== _p$.e && (_p$.e = _$setProp(_el$222, "fg", _v$112, _p$.e));
                _v$113 !== _p$.t && (_p$.t = _$setProp(_el$222, "attributes", _v$113, _p$.t));
                return _p$;
              }, {
                e: void 0,
                t: void 0
              });
              return _el$222;
            })(), (() => {
              var _el$224 = _$createElement("text");
              _$insert(_el$224, () => props.result());
              _$effect((_$p) => _$setProp(_el$224, "fg", palette.text, _$p));
              return _el$224;
            })()];
          }
        })];
      }
    }), null);
    _$effect((_p$) => {
      var _v$114 = palette.accent, _v$115 = TextAttributes.BOLD, _v$116 = palette.textMuted;
      _v$114 !== _p$.e && (_p$.e = _$setProp(_el$196, "fg", _v$114, _p$.e));
      _v$115 !== _p$.t && (_p$.t = _$setProp(_el$196, "attributes", _v$115, _p$.t));
      _v$116 !== _p$.a && (_p$.a = _$setProp(_el$197, "fg", _v$116, _p$.a));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0
    });
    return _el$195;
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
    var _el$235 = _$createElement("box"), _el$236 = _$createElement("text");
    _$insertNode(_el$235, _el$236);
    _$setProp(_el$235, "flexDirection", "column");
    _$setProp(_el$235, "padding", 1);
    _$insert(_el$236, () => `${GLYPH.active} nan usage detail \xB7 daily granularity`);
    _$insert(_el$235, _$createComponent(Show, {
      get when() {
        return props.loadError() !== "";
      },
      get children() {
        var _el$237 = _$createElement("text");
        _$insert(_el$237, () => props.loadError());
        _$effect((_$p) => _$setProp(_el$237, "fg", palette.error, _$p));
        return _el$237;
      }
    }), null);
    _$insert(_el$235, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.loadError() === "")() && props.loading();
      },
      get children() {
        var _el$238 = _$createElement("text");
        _$insertNode(_el$238, _$createTextNode(`reading nan metrics\u2026`));
        _$effect((_$p) => _$setProp(_el$238, "fg", palette.textMuted, _$p));
        return _el$238;
      }
    }), null);
    _$insert(_el$235, _$createComponent(Show, {
      get when() {
        return days().length > 0;
      },
      get children() {
        var _el$240 = _$createElement("box"), _el$241 = _$createElement("text"), _el$243 = _$createElement("text"), _el$244 = _$createElement("text");
        _$insertNode(_el$240, _el$241);
        _$insertNode(_el$240, _el$243);
        _$insertNode(_el$240, _el$244);
        _$setProp(_el$240, "flexDirection", "row");
        _$setProp(_el$240, "marginTop", 1);
        _$insertNode(_el$241, _$createTextNode(`DAILY TOKENS `));
        _$insert(_el$243, sparkline);
        _$insert(_el$244, () => `  ${days()[0].date} \u2192 ${days()[days().length - 1].date}`);
        _$effect((_p$) => {
          var _v$120 = palette.textSoft, _v$121 = palette.accentAlt, _v$122 = palette.textMuted;
          _v$120 !== _p$.e && (_p$.e = _$setProp(_el$241, "fg", _v$120, _p$.e));
          _v$121 !== _p$.t && (_p$.t = _$setProp(_el$243, "fg", _v$121, _p$.t));
          _v$122 !== _p$.a && (_p$.a = _$setProp(_el$244, "fg", _v$122, _p$.a));
          return _p$;
        }, {
          e: void 0,
          t: void 0,
          a: void 0
        });
        return _el$240;
      }
    }), null);
    _$insert(_el$235, _$createComponent(Show, {
      get when() {
        return props.rows().length > 0;
      },
      get children() {
        return [(() => {
          var _el$245 = _$createElement("text");
          _$insertNode(_el$245, _$createTextNode(`MONTH-TO-DATE AGAINST MONTHLY QUOTA`));
          _$setProp(_el$245, "marginTop", 1);
          _$effect((_p$) => {
            var _v$123 = palette.textSoft, _v$124 = TextAttributes.BOLD;
            _v$123 !== _p$.e && (_p$.e = _$setProp(_el$245, "fg", _v$123, _p$.e));
            _v$124 !== _p$.t && (_p$.t = _$setProp(_el$245, "attributes", _v$124, _p$.t));
            return _p$;
          }, {
            e: void 0,
            t: void 0
          });
          return _el$245;
        })(), _$createComponent(For, {
          get each() {
            return props.rows();
          },
          children: (row) => (() => {
            var _el$249 = _$createElement("box"), _el$250 = _$createElement("text"), _el$251 = _$createElement("text");
            _$insertNode(_el$249, _el$250);
            _$insertNode(_el$249, _el$251);
            _$setProp(_el$249, "flexDirection", "row");
            _$insert(_el$250, () => `${(row.model + "                    ").slice(0, 20)}`);
            _$insert(_el$251, () => `${formatLargeTokens(row.monthToDate).padStart(8, " ")}`);
            _$insert(_el$249, _$createComponent(Show, {
              get when() {
                return row.quota > 0;
              },
              get fallback() {
                return (() => {
                  var _el$253 = _$createElement("text");
                  _$insertNode(_el$253, _$createTextNode(`  quota unknown`));
                  _$effect((_$p) => _$setProp(_el$253, "fg", palette.textMuted, _$p));
                  return _el$253;
                })();
              },
              get children() {
                return [(() => {
                  var _el$252 = _$createElement("text");
                  _$insert(_el$252, () => ` / ${formatLargeTokens(row.quota)} `);
                  _$effect((_$p) => _$setProp(_el$252, "fg", palette.textMuted, _$p));
                  return _el$252;
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
              var _v$127 = palette.text, _v$128 = palette.sky;
              _v$127 !== _p$.e && (_p$.e = _$setProp(_el$250, "fg", _v$127, _p$.e));
              _v$128 !== _p$.t && (_p$.t = _$setProp(_el$251, "fg", _v$128, _p$.t));
              return _p$;
            }, {
              e: void 0,
              t: void 0
            });
            return _el$249;
          })()
        })];
      }
    }), null);
    _$insert(_el$235, _$createComponent(For, {
      get each() {
        return rollingRefs();
      },
      children: (row) => (() => {
        var _el$255 = _$createElement("text");
        _$setProp(_el$255, "marginTop", 1);
        _$insert(_el$255, () => `reference only: ${row.model} rolling-4h allowance ${formatLargeTokens(row.rolling4hRef)} tokens \u2014 daily-granularity data cannot express a rolling window`);
        _$effect((_$p) => _$setProp(_el$255, "fg", palette.warning, _$p));
        return _el$255;
      })()
    }), null);
    _$insert(_el$235, _$createComponent(Show, {
      get when() {
        return _$memo(() => !!(props.loadError() === "" && !props.loading() && days().length === 0))() && props.rows().length === 0;
      },
      get children() {
        var _el$247 = _$createElement("text");
        _$insertNode(_el$247, _$createTextNode(`no nan usage data available`));
        _$effect((_$p) => _$setProp(_el$247, "fg", palette.textMuted, _$p));
        return _el$247;
      }
    }), null);
    _$effect((_p$) => {
      var _v$125 = palette.info, _v$126 = TextAttributes.BOLD;
      _v$125 !== _p$.e && (_p$.e = _$setProp(_el$236, "fg", _v$125, _p$.e));
      _v$126 !== _p$.t && (_p$.t = _$setProp(_el$236, "attributes", _v$126, _p$.t));
      return _p$;
    }, {
      e: void 0,
      t: void 0
    });
    return _el$235;
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
    var _el$256 = _$createElement("box"), _el$257 = _$createElement("text"), _el$258 = _$createElement("text"), _el$259 = _$createElement("text"), _el$260 = _$createElement("text"), _el$261 = _$createElement("text"), _el$263 = _$createElement("text"), _el$264 = _$createElement("text");
    _$insertNode(_el$256, _el$257);
    _$insertNode(_el$256, _el$258);
    _$insertNode(_el$256, _el$259);
    _$insertNode(_el$256, _el$260);
    _$insertNode(_el$256, _el$261);
    _$insertNode(_el$256, _el$263);
    _$insertNode(_el$256, _el$264);
    _$setProp(_el$256, "flexDirection", "row");
    _$setProp(_el$257, "selectable", false);
    _$insert(_el$257, () => `${props.active ? GLYPH.row : " "} `);
    _$setProp(_el$258, "selectable", false);
    _$insert(_el$258, () => `${fitWidth(statusCore(), AGENTS_STATUS_WIDTH)} `);
    _$setProp(_el$259, "selectable", false);
    _$insert(_el$259, () => `${fitWidth(row.agent || "agent", props.layout().agent)} `);
    _$insert(_el$260, () => fitWidth(row.title, props.layout().title));
    _$setProp(_el$261, "selectable", false);
    _$insert(_el$261, () => ` ${fitWidth(elapsed(), AGENTS_ELAPSED_WIDTH)}`);
    _$insert(_el$256, _$createComponent(Show, {
      get when() {
        return props.layout().activity > 0;
      },
      get children() {
        var _el$262 = _$createElement("text");
        _$setProp(_el$262, "selectable", false);
        _$insert(_el$262, () => ` ${fitWidth(activity(), props.layout().activity)}`);
        _$effect((_$p) => _$setProp(_el$262, "fg", palette.info, _$p));
        return _el$262;
      }
    }), _el$263);
    _$setProp(_el$263, "selectable", false);
    _$insert(_el$263, () => ` ${fitWidth(row.tokens === void 0 ? "-" : formatTokens(row.tokens), AGENTS_TOKENS_WIDTH)}`);
    _$setProp(_el$264, "selectable", false);
    _$insert(_el$264, () => ` ${fitWidth(row.cost === void 0 ? "-" : `$${formatCost(row.cost)}`, AGENTS_COST_WIDTH)}`);
    _$insert(_el$256, _$createComponent(Show, {
      get when() {
        return _$memo(() => props.layout().task > 0)() && taskBadge() !== "";
      },
      get children() {
        var _el$265 = _$createElement("text");
        _$setProp(_el$265, "selectable", false);
        _$insert(_el$265, () => ` ${fitWidth(taskBadge(), props.layout().task)}`);
        _$effect((_$p) => _$setProp(_el$265, "fg", taskStatusChip(row.taskStatus || "backlog", palette).color, _$p));
        return _el$265;
      }
    }), null);
    _$effect((_p$) => {
      var _v$129 = props.active ? palette.element : palette.background, _v$130 = props.onActivate, _v$131 = props.active ? palette.accent : palette.textMuted, _v$132 = subagentStatusColor(row.status, palette), _v$133 = palette.accentAlt, _v$134 = props.active ? palette.text : palette.textSoft, _v$135 = palette.textSoft, _v$136 = palette.sky, _v$137 = palette.warning;
      _v$129 !== _p$.e && (_p$.e = _$setProp(_el$256, "backgroundColor", _v$129, _p$.e));
      _v$130 !== _p$.t && (_p$.t = _$setProp(_el$256, "onMouseDown", _v$130, _p$.t));
      _v$131 !== _p$.a && (_p$.a = _$setProp(_el$257, "fg", _v$131, _p$.a));
      _v$132 !== _p$.o && (_p$.o = _$setProp(_el$258, "fg", _v$132, _p$.o));
      _v$133 !== _p$.i && (_p$.i = _$setProp(_el$259, "fg", _v$133, _p$.i));
      _v$134 !== _p$.n && (_p$.n = _$setProp(_el$260, "fg", _v$134, _p$.n));
      _v$135 !== _p$.s && (_p$.s = _$setProp(_el$261, "fg", _v$135, _p$.s));
      _v$136 !== _p$.h && (_p$.h = _$setProp(_el$263, "fg", _v$136, _p$.h));
      _v$137 !== _p$.r && (_p$.r = _$setProp(_el$264, "fg", _v$137, _p$.r));
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
    return _el$256;
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
    var _el$266 = _$createElement("box"), _el$267 = _$createElement("text"), _el$268 = _$createElement("text"), _el$269 = _$createElement("text"), _el$270 = _$createElement("text"), _el$271 = _$createElement("text"), _el$273 = _$createElement("text"), _el$274 = _$createElement("text");
    _$insertNode(_el$266, _el$267);
    _$insertNode(_el$266, _el$268);
    _$insertNode(_el$266, _el$269);
    _$insertNode(_el$266, _el$270);
    _$insertNode(_el$266, _el$271);
    _$insertNode(_el$266, _el$273);
    _$insertNode(_el$266, _el$274);
    _$setProp(_el$266, "flexDirection", "row");
    _$setProp(_el$267, "selectable", false);
    _$insert(_el$267, () => `${fitWidth("", 1)} `);
    _$setProp(_el$268, "selectable", false);
    _$insert(_el$268, () => `${fitWidth("ST", AGENTS_STATUS_WIDTH)} `);
    _$setProp(_el$269, "selectable", false);
    _$insert(_el$269, () => `${fitWidth("AGENT", layout().agent)} `);
    _$setProp(_el$270, "selectable", false);
    _$insert(_el$270, () => fitWidth("OBJECTIVE", layout().title));
    _$setProp(_el$271, "selectable", false);
    _$insert(_el$271, () => ` ${fitWidth("ELAPSED", AGENTS_ELAPSED_WIDTH)}`);
    _$insert(_el$266, _$createComponent(Show, {
      get when() {
        return layout().activity > 0;
      },
      get children() {
        var _el$272 = _$createElement("text");
        _$setProp(_el$272, "selectable", false);
        _$insert(_el$272, () => ` ${fitWidth("ACTIVITY", layout().activity)}`);
        _$effect((_$p) => _$setProp(_el$272, "fg", palette.textMuted, _$p));
        return _el$272;
      }
    }), _el$273);
    _$setProp(_el$273, "selectable", false);
    _$insert(_el$273, () => ` ${fitWidth("TOKENS", AGENTS_TOKENS_WIDTH)}`);
    _$setProp(_el$274, "selectable", false);
    _$insert(_el$274, () => ` ${fitWidth("COST", AGENTS_COST_WIDTH)}`);
    _$insert(_el$266, _$createComponent(Show, {
      get when() {
        return layout().task > 0;
      },
      get children() {
        var _el$275 = _$createElement("text");
        _$setProp(_el$275, "selectable", false);
        _$insert(_el$275, () => ` ${fitWidth("TASK", layout().task)}`);
        _$effect((_$p) => _$setProp(_el$275, "fg", palette.textMuted, _$p));
        return _el$275;
      }
    }), null);
    _$effect((_p$) => {
      var _v$138 = palette.textMuted, _v$139 = palette.textMuted, _v$140 = palette.textMuted, _v$141 = palette.textMuted, _v$142 = palette.textMuted, _v$143 = palette.textMuted, _v$144 = palette.textMuted;
      _v$138 !== _p$.e && (_p$.e = _$setProp(_el$267, "fg", _v$138, _p$.e));
      _v$139 !== _p$.t && (_p$.t = _$setProp(_el$268, "fg", _v$139, _p$.t));
      _v$140 !== _p$.a && (_p$.a = _$setProp(_el$269, "fg", _v$140, _p$.a));
      _v$141 !== _p$.o && (_p$.o = _$setProp(_el$270, "fg", _v$141, _p$.o));
      _v$142 !== _p$.i && (_p$.i = _$setProp(_el$271, "fg", _v$142, _p$.i));
      _v$143 !== _p$.n && (_p$.n = _$setProp(_el$273, "fg", _v$143, _p$.n));
      _v$144 !== _p$.s && (_p$.s = _$setProp(_el$274, "fg", _v$144, _p$.s));
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
    return _el$266;
  })();
  return (() => {
    var _el$276 = _$createElement("box"), _el$278 = _$createElement("box"), _el$279 = _$createElement("text");
    _$insertNode(_el$276, _el$278);
    _$setProp(_el$276, "flexDirection", "column");
    _$setProp(_el$276, "borderStyle", "single");
    _$setProp(_el$276, "titleAlignment", "left");
    _$setProp(_el$276, "paddingLeft", 1);
    _$setProp(_el$276, "paddingRight", 1);
    _$insert(_el$276, header, _el$278);
    _$insert(_el$276, _$createComponent(Show, {
      get when() {
        return props.error() !== "";
      },
      get children() {
        var _el$277 = _$createElement("text");
        _$insert(_el$277, () => clipped(props.error(), 72));
        _$effect((_$p) => _$setProp(_el$277, "fg", palette.warning, _$p));
        return _el$277;
      }
    }), _el$278);
    _$insert(_el$276, _$createComponent(Show, {
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
    }), _el$278);
    _$insertNode(_el$278, _el$279);
    _$setProp(_el$278, "flexDirection", "row");
    _$setProp(_el$278, "marginTop", 1);
    _$insertNode(_el$279, _$createTextNode(`open enter \xB7 close esc \xB7 parent up`));
    _$setProp(_el$279, "selectable", false);
    _$effect((_p$) => {
      var _v$145 = palette.accentBorder, _v$146 = `${GLYPH.brand} SUBAGENTS [${running()} running \xB7 ${props.rows().length} total]`, _v$147 = palette.accent, _v$148 = palette.background, _v$149 = palette.textMuted;
      _v$145 !== _p$.e && (_p$.e = _$setProp(_el$276, "borderColor", _v$145, _p$.e));
      _v$146 !== _p$.t && (_p$.t = _$setProp(_el$276, "title", _v$146, _p$.t));
      _v$147 !== _p$.a && (_p$.a = _$setProp(_el$276, "titleColor", _v$147, _p$.a));
      _v$148 !== _p$.o && (_p$.o = _$setProp(_el$276, "backgroundColor", _v$148, _p$.o));
      _v$149 !== _p$.i && (_p$.i = _$setProp(_el$279, "fg", _v$149, _p$.i));
      return _p$;
    }, {
      e: void 0,
      t: void 0,
      a: void 0,
      o: void 0,
      i: void 0
    });
    return _el$276;
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
  const [minionsExpanded, setMinionsExpanded] = createSignal(getPref(MINIONS_EXPANDED_KEY, startingDensity === "expanded"));
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
    setSectionExpanded(MINIONS_EXPANDED_KEY, setMinionsExpanded, open);
    setSectionExpanded(ATTENTION_EXPANDED_KEY, setAttentionExpanded, open);
  };
  const toggleDensity = () => {
    const next = density() === "compact" ? "expanded" : "compact";
    applyDensity(next);
    showToast(next === "compact" ? `${MARK_COLLAPSED} Compact density` : `${MARK_EXPANDED} Expanded density`, next === "compact" ? "Sections collapsed to one line" : "Sections expanded with full detail", "info");
  };
  const spinner = createMemo(() => SPINNER_FRAMES[frame() % SPINNER_FRAMES.length]);
  const pulse = createMemo(() => NEURAL_PULSE_FRAMES[pulseFrame() % NEURAL_PULSE_FRAMES.length]);
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
    if (typeof sessionID !== "string" || sessionID === "") return;
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
  const activateMinionRow = (index) => {
    const row = agentRows()[index];
    if (row) openSubagentSession(row.sessionID);
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
    const stamp = snapshot().generated_at;
    if (!stamp) return;
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
          minions: agentRows,
          onActivateMinion: activateMinionRow,
          snapshotError,
          sessionElapsed,
          eta: boardEta,
          now,
          spinner,
          pulse,
          density,
          toggleDensity,
          tasksExpanded,
          minionsExpanded,
          attentionExpanded,
          toggleTasks: () => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded),
          toggleMinions: () => togglePreference(MINIONS_EXPANDED_KEY, minionsExpanded, setMinionsExpanded),
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
        var _el$281 = _$createElement("box");
        _$setProp(_el$281, "flexDirection", "column");
        _$setProp(_el$281, "width", "100%");
        _$insert(_el$281, _$createComponent(HomeLogo, {
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        _$insert(_el$281, _$createComponent(HomeStatsWidget, {
          stats: usageStats,
          loading: statsLoading,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        return _el$281;
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
          minions: agentRows,
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
                      minions: agentRows,
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
        minions: agentRows,
        onActivateMinion: activateMinionRow,
        snapshotError,
        sessionElapsed,
        eta: boardEta,
        now,
        spinner,
        pulse,
        density,
        toggleDensity,
        tasksExpanded,
        minionsExpanded,
        attentionExpanded,
        toggleTasks: () => togglePreference(TASKS_EXPANDED_KEY, tasksExpanded, setTasksExpanded),
        toggleMinions: () => togglePreference(MINIONS_EXPANDED_KEY, minionsExpanded, setMinionsExpanded),
        toggleAttention: () => togglePreference(ATTENTION_EXPANDED_KEY, attentionExpanded, setAttentionExpanded),
        get theme() {
          return ctx?.theme?.current || ctx?.theme || api.theme;
        }
      });
    },
    home_bottom(ctx) {
      updateActiveSession(ctx);
      return (() => {
        var _el$282 = _$createElement("box");
        _$setProp(_el$282, "flexDirection", "column");
        _$setProp(_el$282, "width", "100%");
        _$insert(_el$282, _$createComponent(HomeStatsWidget, {
          stats: usageStats,
          loading: statsLoading,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        _$insert(_el$282, _$createComponent(HomeBottomStatus, {
          snapshot,
          minions: agentRows,
          spinner,
          snapshotError,
          get theme() {
            return ctx?.theme?.current || ctx?.theme || api.theme;
          }
        }), null);
        return _el$282;
      })();
    },
    "home.footer.status"(ctx) {
      updateActiveSession(ctx);
      return _$createComponent(HomeBottomStatus, {
        snapshot,
        minions: agentRows,
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
        minions: agentRows,
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
        minions: agentRows,
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
