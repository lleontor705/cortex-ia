#!/usr/bin/env node
import fs from "fs";
import path from "path";

/**
 * OpenCode v2 Theme Contract Validator
 * Validates a theme JSON against the official OpenCode v2 specification and schema.
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
if (theme.version !== 2) {
  errors.push("Missing or invalid 'version' (OpenCode v2 requires 'version: 2')");
}

if (theme.base) {
  errors.push("Deprecated 'base' token tree detected. In OpenCode v2, semantic tokens must be placed directly inside 'dark' and/or 'light' mode blocks.");
}

if (!theme.dark && !theme.light) {
  errors.push("Theme must define at least one mode ('dark' or 'light')");
}

// 2. Mode Hues & Tokens
const requiredHues = ["gray", "purple", "cyan", "blue", "green", "yellow", "orange", "red"];
const requiredAliases = ["accent", "interactive", "neutral"];
const requiredSteps = ["100", "200", "300", "400", "500", "600", "700", "800", "900"];
const syntaxKeys = ["comment", "keyword", "function", "variable", "string", "number", "type", "operator", "punctuation"];
const mdKeys = ["text", "heading", "link", "linkText", "code", "blockQuote", "emphasis", "strong", "horizontalRule", "listItem", "listEnumeration", "image", "imageText", "codeBlock"];

const validColorRef = /^#(?:[0-9a-fA-F]{3,8})$|^\$([a-zA-Z0-9_@:.-]+)$|^transparent$/;

for (const mode of ["dark", "light"]) {
  if (!theme[mode]) continue;
  const m = theme[mode];

  // Hue
  const hue = m.hue;
  if (!hue || typeof hue !== "object") {
    errors.push(`Mode '${mode}' missing 'hue' object`);
  } else {
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

  // Categorical
  if (m.categorical && (!Array.isArray(m.categorical) || m.categorical.length < 1)) {
    errors.push(`Mode '${mode}' 'categorical' must be a non-empty array of hue names`);
  }

  // Text tokens
  if (!m.text?.default) errors.push(`Mode '${mode}' missing 'text.default'`);
  if (!m.text?.subdued) errors.push(`Mode '${mode}' missing 'text.subdued'`);
  for (const act of ["primary", "secondary", "destructive"]) {
    if (!m.text?.action?.[act]?.default) errors.push(`Mode '${mode}' missing 'text.action.${act}.default'`);
  }
  if (!m.text?.formfield?.default) errors.push(`Mode '${mode}' missing 'text.formfield.default'`);
  for (const fb of ["error", "warning", "success", "info"]) {
    if (!m.text?.feedback?.[fb]?.default) {
      errors.push(`Mode '${mode}' missing required feedback token 'text.feedback.${fb}.default'`);
    }
  }

  // Background tokens
  if (!m.background?.default) errors.push(`Mode '${mode}' missing 'background.default'`);
  if (!m.background?.raised?.base) errors.push(`Mode '${mode}' missing 'background.raised.base'`);
  if (!m.background?.raised?.high) errors.push(`Mode '${mode}' missing 'background.raised.high'`);
  if (!m.background?.raised?.max) errors.push(`Mode '${mode}' missing 'background.raised.max'`);
  for (const act of ["primary", "secondary", "destructive"]) {
    if (!m.background?.action?.[act]?.default) errors.push(`Mode '${mode}' missing 'background.action.${act}.default'`);
  }
  if (!m.background?.formfield?.default) errors.push(`Mode '${mode}' missing 'background.formfield.default'`);
  for (const fb of ["error", "warning", "success", "info"]) {
    if (!m.background?.feedback?.[fb]?.default) {
      errors.push(`Mode '${mode}' missing required feedback token 'background.feedback.${fb}.default'`);
    }
  }

  // Border & Scrollbar
  if (!m.border?.default) errors.push(`Mode '${mode}' missing 'border.default'`);
  if (!m.scrollbar?.default) errors.push(`Mode '${mode}' missing 'scrollbar.default'`);

  // Diff tokens
  if (!m.diff?.text?.added) errors.push(`Mode '${mode}' missing 'diff.text.added'`);
  if (!m.diff?.text?.removed) errors.push(`Mode '${mode}' missing 'diff.text.removed'`);
  if (!m.diff?.text?.context) errors.push(`Mode '${mode}' missing 'diff.text.context'`);
  if (!m.diff?.text?.hunkHeader) errors.push(`Mode '${mode}' missing 'diff.text.hunkHeader'`);

  if (!m.diff?.background?.added) errors.push(`Mode '${mode}' missing 'diff.background.added'`);
  if (!m.diff?.background?.removed) errors.push(`Mode '${mode}' missing 'diff.background.removed'`);
  if (!m.diff?.background?.context) errors.push(`Mode '${mode}' missing 'diff.background.context'`);

  if (!m.diff?.highlight?.added) errors.push(`Mode '${mode}' missing 'diff.highlight.added'`);
  if (!m.diff?.highlight?.removed) errors.push(`Mode '${mode}' missing 'diff.highlight.removed'`);

  if (!m.diff?.lineNumber?.text) errors.push(`Mode '${mode}' missing 'diff.lineNumber.text'`);
  if (!m.diff?.lineNumber?.background?.added) errors.push(`Mode '${mode}' missing 'diff.lineNumber.background.added'`);
  if (!m.diff?.lineNumber?.background?.removed) errors.push(`Mode '${mode}' missing 'diff.lineNumber.background.removed'`);

  // Syntax tokens (9 keys)
  for (const k of syntaxKeys) {
    const val = m.syntax?.[k];
    if (!val) {
      errors.push(`Mode '${mode}' missing syntax token 'syntax.${k}'`);
    } else if (!validColorRef.test(val)) {
      errors.push(`Mode '${mode}' invalid syntax token 'syntax.${k}' = "${val}". Must be hex color or $ref.`);
    }
  }

  // Markdown tokens (14 keys)
  for (const k of mdKeys) {
    const val = m.markdown?.[k];
    if (!val) {
      errors.push(`Mode '${mode}' missing markdown token 'markdown.${k}'`);
    } else if (!validColorRef.test(val)) {
      errors.push(`Mode '${mode}' invalid markdown token 'markdown.${k}' = "${val}". Must be hex color or $ref.`);
    }
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
