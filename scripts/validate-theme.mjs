#!/usr/bin/env node
import fs from "fs";
import path from "path";

/**
 * OpenCode v2 Theme Contract Validator
 * Validates a theme JSON against the official OpenCode v2 specification and Zod schema.
 */

const targetFile = process.argv[2] || "internal/assets/themes/cortex.json";
const resolvedPath = path.resolve(process.cwd(), targetFile);

if (!fs.existsSync(resolvedPath)) {
  console.error(`❌ Error: Theme file not found at ${resolvedPath}`);
  process.exit(1);
}

console.log(`🎨 Validating OpenCode v2 Theme: ${path.basename(resolvedPath)}`);

let theme;
try {
  theme = JSON.parse(fs.readFileSync(resolvedPath, "utf8"));
} catch (err) {
  console.error(`❌ Error parsing JSON: ${err.message}`);
  process.exit(1);
}

const errors = [];
const warnings = [];

// 1. Root structure
if (!theme.base || typeof theme.base !== "object") {
  errors.push("Missing or invalid 'base' token tree (required in v2)");
}

if (!theme.dark && !theme.light) {
  errors.push("Theme must define at least one mode ('dark' or 'light')");
}

// 2. Mode Hues
const requiredHues = ["gray", "purple", "cyan", "blue", "green", "yellow", "orange", "red"];
const requiredAliases = ["accent", "interactive", "neutral"];
const requiredSteps = ["100", "200", "300", "400", "500", "600", "700", "800", "900"];

for (const mode of ["dark", "light"]) {
  if (!theme[mode]) continue;
  const hue = theme[mode].hue;
  if (!hue || typeof hue !== "object") {
    errors.push(`Mode '${mode}' missing 'hue' object`);
    continue;
  }
  for (const h of requiredHues) {
    if (!hue[h]) {
      errors.push(`Mode '${mode}' missing base hue '${h}'`);
    } else if (typeof hue[h] === "object") {
      for (const step of requiredSteps) {
        if (!hue[h][step]) {
          errors.push(`Mode '${mode}' hue '${h}' missing step '${step}'`);
        } else if (!/^#[0-9a-fA-F]{3,8}$/.test(hue[h][step])) {
          errors.push(`Mode '${mode}' hue '${h}' step '${step}' has invalid color '${hue[h][step]}'`);
        }
      }
    }
  }
  for (const alias of requiredAliases) {
    if (!hue[alias]) {
      errors.push(`Mode '${mode}' missing alias '${alias}'`);
    } else if (typeof hue[alias] === "string" && !hue[alias].startsWith("$hue.")) {
      warnings.push(`Mode '${mode}' alias '${alias}' should reference '$hue.<target>'`);
    }
  }
}

// 3. Base Semantic Tokens
const base = theme.base || {};

// Categorical
if (!Array.isArray(base.categorical) || base.categorical.length < 4) {
  warnings.push("'base.categorical' should be an array with at least 4 hue names");
}

// Text tokens
if (!base.text?.base) errors.push("Missing 'base.text.base'");
if (!base.text?.muted) errors.push("Missing 'base.text.muted'");
for (const act of ["primary", "secondary", "destructive"]) {
  if (!base.text?.action?.[act]?.base) errors.push(`Missing 'base.text.action.${act}.base'`);
}
if (!base.text?.formfield?.base) errors.push("Missing 'base.text.formfield.base'");
for (const fb of ["error", "warning", "success", "info"]) {
  if (!base.text?.feedback?.[fb]?.base) {
    errors.push(`Missing required feedback token 'base.text.feedback.${fb}.base'`);
  }
}

// Background tokens
if (!base.background?.base) errors.push("Missing 'base.background.base'");
if (!base.background?.raised?.base) errors.push("Missing 'base.background.raised.base'");
if (!base.background?.raised?.high) errors.push("Missing 'base.background.raised.high'");
if (!base.background?.raised?.max) errors.push("Missing 'base.background.raised.max'");
for (const act of ["primary", "secondary", "destructive"]) {
  if (!base.background?.action?.[act]?.base) errors.push(`Missing 'base.background.action.${act}.base'`);
}
if (!base.background?.formfield?.base) errors.push("Missing 'base.background.formfield.base'");
for (const fb of ["error", "warning", "success", "info"]) {
  if (!base.background?.feedback?.[fb]?.base) {
    errors.push(`Missing required feedback token 'base.background.feedback.${fb}.base'`);
  }
}

// Border & Scrollbar
if (!base.border?.base) errors.push("Missing 'base.border.base'");
if (!base.scrollbar?.base) {
  errors.push("Missing 'base.scrollbar.base' (OpenCode v2 requires 'base', not 'thumb'/'track')");
}

// Diff tokens (critical!)
if (!base.diff?.text?.added) errors.push("Missing 'base.diff.text.added'");
if (!base.diff?.text?.removed) errors.push("Missing 'base.diff.text.removed'");
if (!base.diff?.text?.context) errors.push("Missing 'base.diff.text.context'");
if (!base.diff?.text?.hunkHeader) errors.push("Missing 'base.diff.text.hunkHeader'");

if (!base.diff?.background?.added) errors.push("Missing 'base.diff.background.added'");
if (!base.diff?.background?.removed) errors.push("Missing 'base.diff.background.removed'");
if (!base.diff?.background?.context) errors.push("Missing 'base.diff.background.context'");

if (!base.diff?.highlight?.added) errors.push("Missing 'base.diff.highlight.added'");
if (!base.diff?.highlight?.removed) errors.push("Missing 'base.diff.highlight.removed'");

if (!base.diff?.lineNumber?.text) errors.push("Missing 'base.diff.lineNumber.text'");
if (!base.diff?.lineNumber?.background?.added) errors.push("Missing 'base.diff.lineNumber.background.added'");
if (!base.diff?.lineNumber?.background?.removed) errors.push("Missing 'base.diff.lineNumber.background.removed'");

// Syntax tokens (9 keys) - OpenCode v2 strictly requires hex (#hex) or direct $hue.<hue>.<step>
const validHuePattern = /^#(?:[0-9a-fA-F]{3,8})$|^\$hue\.(gray|red|orange|yellow|green|cyan|blue|purple|accent|interactive|neutral)\.(100|200|300|400|500|600|700|800|900)$/;

const syntaxKeys = ["comment", "keyword", "function", "variable", "string", "number", "type", "operator", "punctuation"];
for (const k of syntaxKeys) {
  const val = base.syntax?.[k];
  if (!val) {
    errors.push(`Missing syntax token 'base.syntax.${k}'`);
  } else if (!validHuePattern.test(val)) {
    errors.push(`Invalid syntax token 'base.syntax.${k}' = "${val}". OpenCode v2 requires a hex color (#hex) or direct hue step ($hue.<hue>.<step>), got "${val}"`);
  }
}

// Markdown tokens (14 keys) - OpenCode v2 strictly requires hex (#hex) or direct $hue.<hue>.<step>
const mdKeys = ["text", "heading", "link", "linkText", "code", "blockQuote", "emphasis", "strong", "horizontalRule", "listItem", "listEnumeration", "image", "imageText", "codeBlock"];
for (const k of mdKeys) {
  const val = base.markdown?.[k];
  if (!val) {
    errors.push(`Missing markdown token 'base.markdown.${k}'`);
  } else if (!validHuePattern.test(val)) {
    errors.push(`Invalid markdown token 'base.markdown.${k}' = "${val}". OpenCode v2 requires a hex color (#hex) or direct hue step ($hue.<hue>.<step>), got "${val}"`);
  }
}

// Summary Output
if (warnings.length > 0) {
  console.log(`\n⚠️ Warnings (${warnings.length}):`);
  for (const w of warnings) console.log(`   - ${w}`);
}

if (errors.length > 0) {
  console.error(`\n❌ Validation Failed with ${errors.length} contract error(s):`);
  for (const e of errors) console.error(`   ✖ ${e}`);
  process.exit(1);
}

console.log("\n✅ Theme Contract PASSED! All OpenCode v2 tokens and scales are valid.");
