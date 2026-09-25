#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

/**
 * OpenCode v2.0.15 theme contract validator.
 *
 * Mirrors the loader schema the running opencode2 binary enforces:
 *   { $schema?, base: <complete token tree>, light?: <mode>, dark?: <mode> }
 * A theme that fails that schema is NOT reported to the user: opencode logs
 * nothing usable and silently falls back to a builtin theme. That silent
 * fallback is why every mismatch here is a hard error with a JSON path.
 */

const STEPS = ["100", "200", "300", "400", "500", "600", "700", "800", "900"];
const BASE_HUES = ["gray", "red", "orange", "yellow", "green", "cyan", "blue", "purple"];
const HUE_ALIASES = ["accent", "interactive", "neutral"];
const ACTION_KEYS = ["primary", "secondary", "destructive"];
const STATE_KEYS = ["$hovered", "$focused", "$pressed", "$selected", "$disabled"];
const FEEDBACK_KEYS = ["error", "warning", "success", "info"];
const SYNTAX_KEYS = ["comment", "keyword", "function", "variable", "string", "number", "type", "operator", "punctuation"];
const MARKDOWN_KEYS = ["text", "heading", "link", "linkText", "code", "blockQuote", "emphasis", "strong", "horizontalRule", "listItem", "listEnumeration", "image", "imageText", "codeBlock"];
const DIFF_SHAPE = {
  text: ["added", "removed", "context", "hunkHeader"],
  background: ["added", "removed", "context"],
  highlight: ["added", "removed"]
};
const ROOT_KEYS = ["$schema", "base", "dark", "light"];
const TOKEN_GROUPS = ["text", "background", "border", "scrollbar", "diff", "syntax", "markdown"];
const COLOR_HEX = /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;
const HUE_STEP_REF = /^\$hue\..+\.(?:100|200|300|400|500|600|700|800|900)$/;
const HUE_SCALE_REF = /^\$hue\.[^.]+$/;
const REF_NAMESPACES = ["$text.", "$background.", "$border.", "$scrollbar.", "$diff.", "$syntax.", "$markdown."];

const isObject = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
const isHex = (value) => typeof value === "string" && COLOR_HEX.test(value);
const isColor = (value) => value === "transparent" || isHex(value) || (typeof value === "string" && value.startsWith("$"));

class Report {
  constructor() {
    this.errors = [];
    this.warnings = [];
  }
  error(at, message) {
    this.errors.push(`${at}: ${message}`);
  }
  warn(at, message) {
    this.warnings.push(`${at}: ${message}`);
  }
}

function checkColor(report, at, value, { hueStepOnly = false } = {}) {
  if (hueStepOnly) {
    if (isHex(value) || (typeof value === "string" && HUE_STEP_REF.test(value))) return;
    report.error(at, `expected a hex color or a $hue.<name>.<step> reference, received ${JSON.stringify(value)}`);
    return;
  }
  if (isColor(value)) return;
  report.error(at, `expected a hex color, "transparent" or a $ reference, received ${JSON.stringify(value)}`);
}

function checkOptionalColor(report, at, node, key, required) {
  if (node[key] === undefined) {
    if (required) report.error(`${at}.${key}`, "missing required color");
    return;
  }
  checkColor(report, `${at}.${key}`, node[key]);
}

function warnUnknown(report, at, node, known) {
  for (const key of Object.keys(node)) {
    if (!known.includes(key)) report.warn(`${at}.${key}`, "unknown key for the OpenCode v2.0.15 contract");
  }
}

function checkStateGroup(report, at, node, { requireBase }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkOptionalColor(report, at, node, "base", requireBase);
  for (const key of STATE_KEYS) checkOptionalColor(report, at, node, key, false);
  warnUnknown(report, at, node, ["base", ...STATE_KEYS]);
}

function checkActions(report, at, node, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  for (const action of ACTION_KEYS) {
    if (node[action] === undefined) {
      if (requireAll) report.error(`${at}.${action}`, "missing required action");
      continue;
    }
    checkStateGroup(report, `${at}.${action}`, node[action], { requireBase: requireAll });
  }
  warnUnknown(report, at, node, ACTION_KEYS);
}

function checkFeedback(report, at, node, { requireAll, allowMuted }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  for (const key of FEEDBACK_KEYS) {
    const entry = node[key];
    if (entry === undefined) {
      if (requireAll) report.error(`${at}.${key}`, "missing required feedback token");
      continue;
    }
    if (!isObject(entry)) {
      report.error(`${at}.${key}`, "expected an object");
      continue;
    }
    checkOptionalColor(report, `${at}.${key}`, entry, "base", requireAll);
    if (entry.muted !== undefined && allowMuted) checkColor(report, `${at}.${key}.muted`, entry.muted);
    if (entry.muted !== undefined && !allowMuted) report.error(`${at}.${key}.muted`, "background feedback exposes only 'base'");
    warnUnknown(report, `${at}.${key}`, entry, allowMuted ? ["base", "muted"] : ["base"]);
  }
  warnUnknown(report, at, node, FEEDBACK_KEYS);
}

function checkText(report, at, node, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkOptionalColor(report, at, node, "base", requireAll);
  checkOptionalColor(report, at, node, "muted", requireAll);
  checkGroup(report, at, node, "action", requireAll, checkActions);
  checkGroup(report, at, node, "formfield", requireAll, (r, p, v, o) => checkStateGroup(r, p, v, { requireBase: o.requireAll }));
  checkGroup(report, at, node, "feedback", requireAll, (r, p, v, o) => checkFeedback(r, p, v, { requireAll: o.requireAll, allowMuted: true }));
  warnUnknown(report, at, node, ["base", "muted", "action", "formfield", "feedback"]);
}

function checkBackground(report, at, node, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkOptionalColor(report, at, node, "base", requireAll);
  checkGroup(report, at, node, "raised", requireAll, (r, p, v, o) => {
    if (!isObject(v)) {
      r.error(p, "expected an object");
      return;
    }
    for (const key of ["base", "high", "max"]) checkOptionalColor(r, p, v, key, o.requireAll);
    warnUnknown(r, p, v, ["base", "high", "max"]);
  });
  checkGroup(report, at, node, "action", requireAll, checkActions);
  checkGroup(report, at, node, "formfield", requireAll, (r, p, v, o) => checkStateGroup(r, p, v, { requireBase: o.requireAll }));
  checkGroup(report, at, node, "feedback", requireAll, (r, p, v, o) => checkFeedback(r, p, v, { requireAll: o.requireAll, allowMuted: false }));
  warnUnknown(report, at, node, ["base", "raised", "action", "formfield", "feedback"]);
}

function checkBorderLike(report, at, node, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkOptionalColor(report, at, node, "base", requireAll);
  warnUnknown(report, at, node, ["base"]);
}

function checkDiff(report, at, node, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  for (const [group, keys] of Object.entries(DIFF_SHAPE)) {
    const value = node[group];
    if (value === undefined) {
      if (requireAll) report.error(`${at}.${group}`, "missing required diff group");
      continue;
    }
    if (!isObject(value)) {
      report.error(`${at}.${group}`, "expected an object");
      continue;
    }
    for (const key of keys) checkOptionalColor(report, `${at}.${group}`, value, key, requireAll);
    warnUnknown(report, `${at}.${group}`, value, keys);
  }
  const lineNumber = node.lineNumber;
  if (lineNumber === undefined) {
    if (requireAll) report.error(`${at}.lineNumber`, "missing required diff group");
  } else if (!isObject(lineNumber)) {
    report.error(`${at}.lineNumber`, "expected an object");
  } else {
    checkOptionalColor(report, `${at}.lineNumber`, lineNumber, "text", requireAll);
    checkGroup(report, `${at}.lineNumber`, lineNumber, "background", requireAll, (r, p, v, o) => {
      if (!isObject(v)) {
        r.error(p, "expected an object");
        return;
      }
      for (const key of ["added", "removed"]) checkOptionalColor(r, p, v, key, o.requireAll);
      warnUnknown(r, p, v, ["added", "removed"]);
    });
    warnUnknown(report, `${at}.lineNumber`, lineNumber, ["text", "background"]);
  }
  warnUnknown(report, at, node, [...Object.keys(DIFF_SHAPE), "lineNumber"]);
}

function checkHueKeyedMap(report, at, node, keys, { requireAll }) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  for (const key of keys) {
    if (node[key] === undefined) {
      if (requireAll) report.error(`${at}.${key}`, "missing required token");
      continue;
    }
    checkColor(report, `${at}.${key}`, node[key], { hueStepOnly: true });
  }
  warnUnknown(report, at, node, keys);
}

function checkGroup(report, at, node, key, requireAll, checker) {
  const value = node[key];
  if (value === undefined) {
    if (requireAll) report.error(`${at}.${key}`, "missing required token group");
    return;
  }
  checker(report, `${at}.${key}`, value, { requireAll });
}

function checkTokenOverride(report, at, node, extraKeys) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkGroup(report, at, node, "text", false, checkText);
  checkGroup(report, at, node, "background", false, checkBackground);
  checkGroup(report, at, node, "border", false, checkBorderLike);
  checkGroup(report, at, node, "scrollbar", false, checkBorderLike);
  checkGroup(report, at, node, "diff", false, (r, p, v) => checkDiff(r, p, v, { requireAll: false }));
  checkGroup(report, at, node, "syntax", false, (r, p, v) => checkHueKeyedMap(r, p, v, SYNTAX_KEYS, { requireAll: false }));
  checkGroup(report, at, node, "markdown", false, (r, p, v) => checkHueKeyedMap(r, p, v, MARKDOWN_KEYS, { requireAll: false }));
  warnUnknown(report, at, node, [...TOKEN_GROUPS, ...extraKeys]);
}

function checkHueScale(report, at, scale) {
  if (!isObject(scale)) {
    report.error(at, "expected an object with steps 100..900");
    return;
  }
  for (const step of STEPS) {
    if (scale[step] === undefined) {
      report.error(`${at}.${step}`, "missing required hue step");
      continue;
    }
    if (!isHex(scale[step])) report.error(`${at}.${step}`, `hue steps must be literal hex colors, received ${JSON.stringify(scale[step])}`);
  }
  for (const step of Object.keys(scale)) {
    if (!STEPS.includes(step)) report.error(`${at}.${step}`, "unknown hue step; only 100..900 exist");
  }
}

function checkHues(report, at, hue) {
  if (!isObject(hue)) {
    report.error(at, "missing required hue palette");
    return;
  }
  for (const name of BASE_HUES) {
    if (hue[name] === undefined) {
      report.error(`${at}.${name}`, "missing required base hue scale");
      continue;
    }
    checkHueScale(report, `${at}.${name}`, hue[name]);
  }
  for (const alias of HUE_ALIASES) {
    const value = hue[alias];
    if (value === undefined) {
      report.error(`${at}.${alias}`, "missing required semantic hue alias");
      continue;
    }
    if (isObject(value)) {
      checkHueScale(report, `${at}.${alias}`, value);
      continue;
    }
    if (typeof value !== "string" || !HUE_SCALE_REF.test(value)) {
      report.error(`${at}.${alias}`, `expected a $hue.<name> reference or a full scale, received ${JSON.stringify(value)}`);
      continue;
    }
    if (hue[value.slice("$hue.".length)] === undefined) report.error(`${at}.${alias}`, `references undefined hue ${JSON.stringify(value)}`);
  }
  for (const name of Object.keys(hue)) {
    if (!BASE_HUES.includes(name) && !HUE_ALIASES.includes(name)) report.warn(`${at}.${name}`, "hue outside the documented base hues and aliases");
  }
}

function checkCategorical(report, at, value, required) {
  if (value === undefined) {
    if (required) report.error(at, "missing required 'categorical'");
    return;
  }
  if (!Array.isArray(value) || value.length === 0 || !value.every((entry) => typeof entry === "string")) {
    report.error(at, "expected a non-empty array of hue names");
  }
}

function checkMode(report, at, node) {
  if (!isObject(node)) {
    report.error(at, "expected an object");
    return;
  }
  checkHues(report, `${at}.hue`, node.hue);
  checkCategorical(report, `${at}.categorical`, node.categorical, false);
  checkTokenOverride(report, at, node, ["hue", "categorical"]);
}

function scanLegacy(report, node, at) {
  if (!isObject(node)) return;
  for (const [key, value] of Object.entries(node)) {
    const childPath = at ? `${at}.${key}` : key;
    if (key.startsWith("@context")) report.error(childPath, "obsolete annotation; OpenCode v2.0.15 uses '@dialog' over the raised surface");
    else if (key === "default") report.error(childPath, "obsolete token name; OpenCode v2.0.15 uses 'base'");
    else if (key === "subdued") report.error(childPath, "obsolete token name; OpenCode v2.0.15 uses 'muted'");
    else if (key === "status" && at.endsWith("text")) report.error(childPath, "'text.status.*' was removed in OpenCode v2.0.15");
    else if (key === "version") report.error(childPath, "obsolete wrapper key; the OpenCode v2.0.15 schema has no 'version' field");
    scanLegacy(report, value, childPath);
  }
}

function resolvePath(root, dotted) {
  return dotted.split(".").reduce((acc, key) => (isObject(acc) ? acc[key] : undefined), root);
}

function checkReference(report, at, value, hueNames) {
  if (value.startsWith("$hue.")) {
    const parts = value.slice("$hue.".length).split(".");
    if (!hueNames.has(parts[0])) {
      report.error(at, `reference ${JSON.stringify(value)} points at an undefined hue`);
      return;
    }
    if (parts.length > 1 && !STEPS.includes(parts[1])) report.error(at, `reference ${JSON.stringify(value)} must use a step between 100 and 900`);
    if (parts.length > 2) report.error(at, `reference ${JSON.stringify(value)} has too many segments`);
    return;
  }
  if (REF_NAMESPACES.some((prefix) => value.startsWith(prefix))) return;
  report.warn(at, `reference ${JSON.stringify(value)} is outside the documented namespaces`);
}

function walkReferences(report, node, at, hueNames) {
  if (typeof node === "string") {
    if (node.startsWith("$")) checkReference(report, at, node, hueNames);
    return;
  }
  if (!isObject(node)) return;
  for (const [key, value] of Object.entries(node)) walkReferences(report, value, `${at}.${key}`, hueNames);
}

function checkReferences(report, theme) {
  const hueNames = new Set();
  for (const mode of ["dark", "light"]) {
    const hue = isObject(theme[mode]) ? theme[mode].hue : undefined;
    if (isObject(hue)) for (const name of Object.keys(hue)) hueNames.add(name);
  }
  if (isObject(theme.base)) walkReferences(report, theme.base, "base", hueNames);
  for (const mode of ["dark", "light"]) {
    if (!isObject(theme[mode])) continue;
    for (const [key, value] of Object.entries(theme[mode])) {
      if (key === "hue") continue;
      walkReferences(report, value, `${mode}.${key}`, hueNames);
    }
  }
  const used = new Set();
  const collect = (node) => {
    if (typeof node === "string" && node.startsWith("$hue.")) used.add(node.slice("$hue.".length).split(".")[0]);
    else if (Array.isArray(node)) for (const entry of node) if (typeof entry === "string") used.add(entry);
    if (!isObject(node)) return;
    for (const value of Object.values(node)) collect(value);
  };
  collect(theme.base);
  for (const mode of ["dark", "light"]) {
    if (isObject(theme[mode])) collect(theme[mode].categorical);
  }
  for (const name of hueNames) {
    if (!used.has(name) && !HUE_ALIASES.includes(name)) report.warn(`dark.hue.${name}`, "declared hue scale is never referenced by the base token tree");
  }
}

function validate(theme) {
  const report = new Report();
  if (!isObject(theme)) {
    report.error("$", "theme root must be a JSON object");
    return report;
  }
  scanLegacy(report, theme, "");
  for (const key of Object.keys(theme)) {
    if (!ROOT_KEYS.includes(key)) report.warn(key, "unknown top-level key");
  }
  if (theme.$schema !== undefined && typeof theme.$schema !== "string") report.error("$schema", "expected a string");
  if (!isObject(theme.base)) report.error("base", "missing required complete token tree; OpenCode v2.0.15 has no per-mode-only layout");
  else {
    checkCategorical(report, "base.categorical", theme.base.categorical, true);
    checkGroup(report, "base", theme.base, "text", true, checkText);
    checkGroup(report, "base", theme.base, "background", true, checkBackground);
    checkGroup(report, "base", theme.base, "border", true, checkBorderLike);
    checkGroup(report, "base", theme.base, "scrollbar", true, checkBorderLike);
    checkGroup(report, "base", theme.base, "diff", true, (r, p, v) => checkDiff(r, p, v, { requireAll: true }));
    checkGroup(report, "base", theme.base, "syntax", true, (r, p, v) => checkHueKeyedMap(r, p, v, SYNTAX_KEYS, { requireAll: true }));
    checkGroup(report, "base", theme.base, "markdown", true, (r, p, v) => checkHueKeyedMap(r, p, v, MARKDOWN_KEYS, { requireAll: true }));
    checkGroup(report, "base", theme.base, "@dialog", false, (r, p, v) => checkTokenOverride(r, p, v, []));
    warnUnknown(report, "base", theme.base, [...TOKEN_GROUPS, "categorical", "@dialog"]);
  }
  const modes = ["light", "dark"].filter((mode) => theme[mode] !== undefined);
  if (modes.length === 0) report.error("theme", "must define at least one of 'light' or 'dark'");
  for (const mode of modes) checkMode(report, mode, theme[mode]);
  checkReferences(report, theme);
  return report;
}

const targetFile = process.argv[2] || "internal/assets/themes/cortex.json";
const resolvedPath = path.resolve(process.cwd(), targetFile);

if (!fs.existsSync(resolvedPath)) {
  console.error(`ERROR: theme file not found at ${resolvedPath}`);
  process.exit(1);
}

console.log(`Validating OpenCode v2.0.15 theme contract: ${path.basename(resolvedPath)}`);

let theme;
try {
  theme = JSON.parse(fs.readFileSync(resolvedPath, "utf8"));
} catch (err) {
  console.error(`ERROR: invalid JSON in ${resolvedPath}: ${err.message}`);
  process.exit(1);
}

const report = validate(theme);

if (report.warnings.length > 0) {
  console.log(`\nWarnings (${report.warnings.length}):`);
  for (const warning of report.warnings) console.log(`   - ${warning}`);
}

if (report.errors.length > 0) {
  console.error(`\nValidation FAILED with ${report.errors.length} contract error(s):`);
  for (const error of report.errors) console.error(`   x ${error}`);
  console.error(`\nAn invalid theme is not rejected loudly by opencode: it silently falls back to a builtin theme.`);
  process.exit(1);
}

const modes = ["light", "dark"].filter((mode) => theme[mode] !== undefined);
console.log(`\nPASSED: complete 'base' token tree with mode(s) [${modes.join(", ")}]; every reference resolves.`);
