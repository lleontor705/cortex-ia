---
name: opencode-theme-dev
description: Author, migrate, inspect, and validate themes for OpenCode v2 (opencode2) according to the official v2 specification and schema.
license: MIT
metadata:
  author: lleontor705
  version: "2.1.0"
---

# OpenCode v2 Theme Development Skill

This skill provides comprehensive architectural guidelines, schema definitions, token contracts, and validation helpers for creating and maintaining themes in **OpenCode v2 (`opencode2`)**.

---

## 1. OpenCode v2 Theme Architecture

In OpenCode v2, themes follow a structured specification where semantic tokens and hue palettes are defined directly inside each mode (`dark` and/or `light`):

```
theme.json
├── $schema: "https://opencode.ai/theme.json"
├── version: 2                  <-- MUST be 2
├── dark: { ... }               <-- Dark mode definitions (REQUIRED if no light)
│   ├── hue: { ... }            <-- 8 base hues + 3 aliases + 9 contrast steps (100-900)
│   ├── categorical: [...]      <-- Ordered list of hues for agents/items
│   ├── text: { ... }           <-- default, subdued, action, formfield, status, feedback
│   ├── background: { ... }     <-- default, raised (base, high, max), action, formfield, feedback
│   ├── border: { default }     <-- Border default color
│   ├── scrollbar: { default }  <-- Scrollbar default color
│   ├── diff: { ... }           <-- text, background, highlight, lineNumber
│   ├── syntax: { ... }         <-- 9 syntax highlighting tokens
│   ├── markdown: { ... }       <-- 14 markdown structural tokens
│   ├── @context:elevated: { }  <-- Optional elevated surface overrides
│   └── @context:overlay: { }   <-- Optional overlay surface overrides
└── light: { ... }              <-- Light mode definitions (REQUIRED if no dark)
    └── (same token structure as dark)
```

> [!NOTE]
> OpenCode v2 does **not** recognize a top-level `"base"` block. Semantic tokens must be located directly inside the `"dark"` and/or `"light"` mode objects.

### Precedence and Discovery Locations
OpenCode resolves themes in the following order (later paths override earlier ones):
1. **Built-in themes**: Embedded in the `opencode2` binary (`opencode`, `tokyonight`, `catppuccin`, `nord`, etc.).
2. **Global user themes**: `~/.config/opencode/themes/<name>.json`
3. **Project root themes**: `<workspace-root>/.opencode/themes/<name>.json`
4. **Current directory themes**: `./.opencode/themes/<name>.json`

The filename without `.json` becomes the theme name. For example, `cortex.json` is selected in `cli.json` with:
```json
"theme": { "mode": "dark", "name": "cortex" }
```

---

## 2. Mandatory Contract Invariants

OpenCode v2 validates themes using its internal schema (`ae`). Any invalid value or missing required property will cause the theme to fail schema validation and silently fall back to the built-in default theme:

| Component | Token Keys | Requirement |
| :--- | :--- | :--- |
| `text` | `default`, `subdued`, `action`, `formfield`, `status`, `feedback` | `default` (not `base`), `subdued` (not `muted`) |
| `background` | `default`, `raised` (`base`, `high`, `max`), `action`, `formfield`, `feedback` | `default` (not `base`) |
| `border` | `default` | Color hex or reference |
| `scrollbar` | `default` | Color hex or reference |
| `diff.text` | `added`, `removed`, `context`, `hunkHeader` | Diff text colors |
| `diff.background` | `added`, `removed`, `context` | Diff background fill |
| `diff.highlight` | `added`, `removed` | Inline word-diff |
| `diff.lineNumber` | `text`, `background.added`, `background.removed` | Gutter numbering |

---

## 3. Hues and Contrast Scale Rules

Every mode (`dark` and/or `light`) defines the **8 base hues** and **3 aliases**:
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
node scripts/validate-theme.mjs internal/assets/themes/cortex.json
```
If the theme passes with 0 errors, it is guaranteed to load cleanly in `opencode2`.
