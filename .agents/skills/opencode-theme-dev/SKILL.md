---
name: opencode-theme-dev
description: Author, migrate, inspect, and validate themes for OpenCode v2 (opencode2) according to the official v2 specification and schema.
license: MIT
metadata:
  author: lleontor705
  version: "2.2.0"
---

# OpenCode v2 Theme Development Skill

Schema reference for authoring and migrating themes for **OpenCode v2 (`opencode2`)**, aligned with
the loader the running binary actually enforces (verified against `opencode2` v2.0.15).

---

## 1. Theme architecture

Every v2 theme declares **one complete `base` token tree** plus **at least one hue palette**
(`dark`, `light`, or both):

```
theme.json
├── $schema: <string>              <-- optional
├── base: { ... }                  <-- REQUIRED: the complete token tree
│   ├── categorical: [...]         <-- REQUIRED: ordered hues for agents/items
│   ├── text: { ... }              <-- base, muted, action, formfield, feedback
│   ├── background: { ... }        <-- base, raised (base|high|max), action, formfield, feedback
│   ├── border: { base }
│   ├── scrollbar: { base }
│   ├── diff: { ... }
│   ├── syntax: { ... }            <-- 9 keys
│   ├── markdown: { ... }          <-- 14 keys
│   └── @dialog: { ... }           <-- optional contextual dialog surface
├── dark: { hue: { ... }, categorical?, <token overrides>?, @dialog? }
└── light: { hue: { ... }, ... }   <-- optional
```

> [!IMPORTANT]
> - The top-level `base` tree is **required and complete**. A theme that defines tokens only inside
>   `dark`/`light` is rejected by the schema, and OpenCode reacts by **silently falling back to a
>   builtin theme** — there is no user-visible error, no warning, and no log line worth grepping.
> - Only `base`/`muted` naming exists: `text.base`, `text.muted`, `feedback.<kind>.base|.muted`,
>   `background.base`, `background.raised.base|high|max`, `border.base`, `scrollbar.base`.
>   The 2.0.8-era `default`/`subdued` names are invalid.
> - `text.status.*` (`running`, `question`, `permission`, `unread`) was removed in v2.0.15; status
>   colors are derived internally from feedback and action tokens. Do not emit it.
> - `@context:elevated` / `@context:overlay` are gone. Contextual surfaces are declared as a single
>   `@dialog` group over the raised surface, allowed in `base` and inside a mode.
> - A theme may declare **only one mode**; OpenCode uses it when the other mode is requested.
> - Theme selection lives **only** in `cli.json` (`theme.mode`, `theme.name`), never in `opencode.json`.

### Discovery and precedence
1. **Builtin themes** embedded in the `opencode2` binary.
2. **Global**: `~/.config/opencode/themes/<name>.json`
3. **Project**: `<project-root>/.opencode/themes/<name>.json`

Only `.json` files are read; a theme closer to the current directory replaces a global or parent
theme with the same name. The filename without `.json` is the theme name:

```json title="~/.config/opencode/cli.json"
"theme": { "mode": "dark", "name": "cortex" }
```

---

## 2. Mandatory contract invariants

The loader validates `{ $schema?, base, light?, dark? }` and requires at least one mode. Any missing
required key, invalid color, or unresolvable `$` reference rejects the whole file.

| Component | Required keys | Notes |
| :--- | :--- | :--- |
| `base.categorical` | non-empty string array | Ordered hues used for agents/items |
| `text` | `base`, `muted`, `action.{primary,secondary,destructive}.base`, `formfield.base`, `feedback.{error,warning,success,info}.base` | `feedback.*.muted` is optional; there is no `status` group |
| `background` | `base`, `raised.{base,high,max}`, `action.*.base`, `formfield.base`, `feedback.*.base` | Background feedback entries expose only `base` |
| `border` / `scrollbar` | `base` | |
| `diff` | `text.{added,removed,context,hunkHeader}`, `background.{added,removed,context}`, `highlight.{added,removed}`, `lineNumber.text`, `lineNumber.background.{added,removed}` | |
| `syntax` | `comment`, `keyword`, `function`, `variable`, `string`, `number`, `type`, `operator`, `punctuation` | Value: hex or `$hue.<name>.<step>` |
| `markdown` | `text`, `heading`, `link`, `linkText`, `code`, `blockQuote`, `emphasis`, `strong`, `horizontalRule`, `listItem`, `listEnumeration`, `image`, `imageText`, `codeBlock` | Value: hex or `$hue.<name>.<step>` |
| `dark` / `light` | `hue` | `categorical` and any token group are optional overrides |
| `@dialog` | — | Optional; accepts the same token groups as the base tree |

**States**: `$hovered`, `$focused`, `$pressed`, `$selected`, `$disabled` are optional on every action
and form-field entry; an unspecified state falls back to that entry's `base`.

**Colors**: `#rgb`, `#rgba`, `#rrggbb`, `#rrggbbaa`, `transparent`, or a `$` reference
(`$hue.<name>.<step>`, `$text.base`, `$text.muted`, `$background.raised.base`, …). A reference that
does not resolve rejects the theme.

---

## 3. Hues and contrast scale rules

Every mode palette declares the **8 base hues** and **3 aliases**:
- **Base hues**: `gray`, `red`, `orange`, `yellow`, `green`, `cyan`, `blue`, `purple`.
- **Aliases**: `accent`, `interactive`, `neutral`. Each must either reference `$hue.<name>` or
  provide a full scale.
- **Steps**: `100`–`900`. All nine steps are mandatory and must be **literal hex** colors (a scale
  cannot reference another hue); an unknown step is rejected.
- **Contrast direction**:
  - **Dark mode**: `100` is the lightest/brightest color; `900` is the darkest.
  - **Light mode**: `100` is the darkest color; `900` is the lightest.
- `categorical` entries must resolve to a declared hue name or alias of the same theme.

---

## 4. Syntax and Markdown token dictionaries

Color values for `syntax.*` and `markdown.*` accept **only** a literal hex color or a
`$hue.<name>.<step>` reference (no `transparent`, no `$text.*`/`$background.*` references).

### Syntax keys (all 9 required)
- `comment`: Comments and annotations
- `keyword`: Language keywords (`const`, `function`, `return`, `package`)
- `function`: Function and method identifiers
- `variable`: Variable names and parameters
- `string`: String literals
- `number`: Numeric literals and constants
- `type`: Types, interfaces, classes, structs
- `operator`: Operators (`+`, `-`, `=`, `=>`)
- `punctuation`: Brackets, commas, semicolons

### Markdown keys (all 14 required)
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

## 5. Terminal requirements

For themes to render accurately:
- Terminal must support 24-bit Truecolor (`$COLORTERM=truecolor` or `$COLORTERM=24bit`).
- Modern terminal emulators (Windows Terminal, Alacritty, Kitty, WezTerm, iTerm2) support Truecolor by default.
- If truecolor is unsupported, colors will be approximated to standard 256 ANSI colors.

---

## 6. How to validate a theme

Use the validator located in `scripts/validate-theme.mjs`:

```bash
node scripts/validate-theme.mjs internal/assets/themes/cortex.json
```

It checks the same contract the running `opencode2` enforces: required top-level `base` tree,
`base`/`muted` naming (rejecting `default`, `subdued`, `text.status.*`, `@context:*`, `version`),
8 hue scales x 9 hex steps plus aliases, `@dialog`, every group/key shape, and reference resolution.
Exit 0 means the file matches the loader contract; a failing file is reported with JSON paths, since
opencode itself would only fall back silently. The validator never replaces a real render check in
the terminal: schema validity proves loadability, not taste.
