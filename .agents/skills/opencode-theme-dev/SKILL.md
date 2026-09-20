---
name: opencode-theme-dev
description: Author, migrate, inspect, and validate themes for OpenCode v2 (opencode2) according to the official v2 specification and Zod schema.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# OpenCode v2 Theme Development Skill

This skill provides comprehensive architectural guidelines, schema definitions, token contracts, and validation helpers for creating and maintaining themes in **OpenCode v2 (`opencode2`)**.

---

## 1. OpenCode v2 Theme Architecture

In OpenCode v2, themes follow a structured specification that decouples shared semantic token trees from mode-specific color scales:

```
theme.json
├── $schema: "https://opencode.ai/theme.json"
├── base: { ... }               <-- Shared semantic token tree (REQUIRED)
│   ├── categorical: [...]      <-- Ordered list of hues for agents/items
│   ├── text: { ... }           <-- Base, muted, actions, formfields, feedback
│   ├── background: { ... }     <-- Base, raised (base, high, max), actions, formfields, feedback
│   ├── border: { ... }         <-- Border base
│   ├── scrollbar: { ... }      <-- Scrollbar base
│   ├── diff: { ... }           <-- Text, background, highlight, lineNumber
│   ├── syntax: { ... }         <-- 9 syntax highlighting tokens
│   ├── markdown: { ... }       <-- 14 markdown structural tokens
│   └── @dialog: { ... }        <-- Optional dialog surface overrides
├── dark: { ... }               <-- Dark mode hue palette & overrides (REQUIRED if no light)
│   └── hue: { ... }            <-- 8 base hues + 3 aliases + 9 contrast steps (100-900)
└── light: { ... }              <-- Light mode hue palette & overrides (REQUIRED if no dark)
    └── hue: { ... }
```

### Precedence and Discovery Locations
OpenCode resolves themes in the following order (later paths override earlier ones):
1. **Built-in themes**: Embedded in the `opencode2` binary (`tokyonight`, `catppuccin`, `nord`, etc.).
2. **Global user themes**: `~/.config/opencode/themes/<name>.json`
3. **Project root themes**: `<workspace-root>/.opencode/themes/<name>.json`
4. **Current directory themes**: `./.opencode/themes/<name>.json`

The filename without `.json` becomes the theme name. For example, `cortex.json` is selected with `"name": "cortex"`.

---

## 2. Mandatory Contract Invariants (The 16 Required Tokens)

OpenCode v2 validates themes using a strict internal Zod schema (`F5`). Omitting any of the following 16 tokens will cause the theme to fail schema validation and silently fall back to the default theme:

| Path in `base` | Required Sub-keys | Failure Mode if Missing |
| :--- | :--- | :--- |
| `text.feedback` | `error.base`, `warning.base`, `success.base`, `info.base` | Crash/Fallback on alert notifications |
| `background.feedback` | `error.base`, `warning.base`, `success.base`, `info.base` | Crash/Fallback on banners & callouts |
| `scrollbar` | `base` (Note: v1 `thumb`/`track` is invalid in v2) | Scrollbar rendering fails |
| `diff.text` | `added`, `removed`, `context`, `hunkHeader` | Diff review screen failure |
| `diff.background` | `added`, `removed`, `context` | Diff background fill failure |
| `diff.highlight` | `added`, `removed` | Inline word-diff failure |
| `diff.lineNumber` | `text`, `background.added`, `background.removed` | Gutter numbering failure |

---

## 3. Hues and Contrast Scale Rules

Every mode (`dark` and/or `light`) must define the **8 base hues** and **3 aliases**:
- **Base Hues**: `gray`, `purple`, `cyan`, `blue`, `green`, `yellow`, `orange`, `red`.
- **Aliases**:
  - `accent`: Must point to `$hue.<hue-name>` (e.g. `"$hue.purple"`).
  - `interactive`: Must point to `$hue.<hue-name>` (e.g. `"$hue.cyan"`).
  - `neutral`: Must point to `$hue.gray`.
- **Contrast Direction**:
  - **Dark mode**: `100` is the lightest/brightest color; `900` is the darkest.
  - **Light mode**: `100` is the darkest color; `900` is the lightest.

---

## 4. Syntax and Markdown Token Dictionaries

### Syntax Keys (All 9 required)
- `comment`: Comments and annotations
- `keyword`: Language keywords (`const`, `function`, `return`, `package`)
- `function`: Function and method identifiers
- `variable`: Variable names and parameters
- `string`: String literals
- `number`: Numeric literals and constants
- `type`: Types, interfaces, classes, structs
- `operator`: Operators (`+`, `-`, `=`, `=>`)
- `punctuation`: Brackets, commas, semicolons

### Markdown Keys (All 14 required)
- `text`: Regular prose text
- `heading`: Headings (`#`, `##`, `###`)
- `link`: URL and link targets
- `linkText`: Visible anchor text
- `code`: Inline code spans
- `codeBlock`: Fenced code block foreground/background
- `blockQuote`: Blockquote border and text
- `emphasis`: Italicized text (`*text*`)
- `strong`: Bold text (`**text**`)
- `horizontalRule`: Divider rules (`---`)
- `listItem`: List item bullet or body
- `listEnumeration`: Numbered list counter
- `image`: Image markdown syntax
- `imageText`: Image alt text

---

## 5. Terminal Requirements

For themes to render accurately:
- Terminal must support 24-bit Truecolor (`$COLORTERM=truecolor` or `$COLORTERM=24bit`).
- Modern terminal emulators (Windows Terminal, Alacritty, Kitty, WezTerm, iTerm2) support Truecolor by default.
- If truecolor is unsupported, colors will be approximated to standard 256 ANSI colors.

---

## 6. How to Validate a Theme

Use the validation script located in `scripts/validate-theme.mjs`:
```bash
node scripts/validate-theme.mjs ~/.config/opencode/themes/cortex.json
```
If the theme passes with 0 errors, it is guaranteed to load cleanly in `opencode2`.
