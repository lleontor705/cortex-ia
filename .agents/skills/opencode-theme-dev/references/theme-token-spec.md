# OpenCode v2 Theme Token Specification

Contract enforced by the `opencode2` loader. A file that violates it is rejected **silently**: the
TUI keeps running with a builtin theme, so the only trustworthy signal is the schema itself.

The canonical, complete, validator-clean example in this repository is
`internal/assets/themes/cortex.json` (dark-only, mirrors the Cortex TUI widget palette).

## Root shape

```text
{ $schema?: string, base: Base, light?: Mode, dark?: Mode }
```

- `base` is required and must be complete.
- At least one of `light` / `dark` is required. One mode alone is legal; OpenCode uses it when the
  other mode is requested.
- A mode must declare `hue`; `categorical` and any token group are optional overrides of `base`.

## Structural skeleton

```json title="skeleton (hue scales abbreviated: every hue lists all nine steps 100-900)"
{
  "base": {
    "categorical": ["accent", "purple", "cyan"],
    "text": {
      "base": "$hue.gray.100",
      "muted": "$hue.gray.400",
      "action": {
        "primary": { "base": "$hue.accent.400", "$hovered": "$hue.accent.300", "$disabled": "$hue.gray.600" },
        "secondary": { "base": "$hue.gray.200", "$hovered": "$hue.gray.100", "$disabled": "$hue.gray.600" },
        "destructive": { "base": "$hue.red.400", "$hovered": "$hue.red.300", "$disabled": "$hue.gray.600" }
      },
      "formfield": { "base": "$hue.gray.100", "$focused": "$hue.accent.300", "$disabled": "$hue.gray.500" },
      "feedback": {
        "error": { "base": "$hue.red.400", "muted": "$hue.red.300" },
        "warning": { "base": "$hue.yellow.400", "muted": "$hue.yellow.300" },
        "success": { "base": "$hue.green.400", "muted": "$hue.green.300" },
        "info": { "base": "$hue.cyan.400", "muted": "$hue.cyan.300" }
      }
    },
    "background": {
      "base": "#0a0e17",
      "raised": { "base": "#111927", "high": "#1e293b", "max": "#334155" },
      "action": {
        "primary": { "base": "$hue.interactive.500", "$hovered": "$hue.interactive.600", "$pressed": "$hue.interactive.700" },
        "secondary": { "base": "$background.raised.base", "$hovered": "$background.raised.high" },
        "destructive": { "base": "$hue.red.700", "$hovered": "$hue.red.600" }
      },
      "formfield": { "base": "$background.raised.base", "$focused": "$background.raised.high", "$disabled": "$background.base" },
      "feedback": {
        "error": { "base": "$hue.red.900" },
        "warning": { "base": "$hue.yellow.900" },
        "success": { "base": "$hue.green.900" },
        "info": { "base": "$hue.cyan.900" }
      }
    },
    "border": { "base": "$hue.gray.700" },
    "scrollbar": { "base": "$hue.gray.600" },
    "diff": {
      "text": { "added": "$hue.green.400", "removed": "$hue.red.400", "context": "$text.muted", "hunkHeader": "$hue.gray.400" },
      "background": { "added": "$hue.green.900", "removed": "$hue.red.900", "context": "$hue.gray.900" },
      "highlight": { "added": "$hue.green.400", "removed": "$hue.red.400" },
      "lineNumber": { "text": "$hue.gray.400", "background": { "added": "$hue.green.900", "removed": "$hue.red.900" } }
    },
    "syntax": {
      "comment": "$hue.gray.500", "keyword": "$hue.purple.400", "function": "$hue.cyan.400",
      "variable": "$hue.gray.200", "string": "$hue.green.400", "number": "$hue.yellow.400",
      "type": "$hue.blue.400", "operator": "$hue.cyan.400", "punctuation": "$hue.gray.400"
    },
    "markdown": {
      "text": "$hue.gray.100", "heading": "$hue.cyan.400", "link": "$hue.interactive.400",
      "linkText": "$hue.cyan.400", "code": "$hue.purple.400", "blockQuote": "$hue.gray.400",
      "emphasis": "$hue.yellow.400", "strong": "$hue.purple.400", "horizontalRule": "$hue.gray.700",
      "listItem": "$hue.gray.100", "listEnumeration": "$hue.cyan.400", "image": "$hue.blue.400",
      "imageText": "$hue.gray.400", "codeBlock": "$hue.gray.900"
    },
    "@dialog": {
      "text": { "action": { "primary": { "base": "$hue.gray.100" } } },
      "background": {
        "base": "$background.raised.base",
        "action": { "primary": { "base": "$hue.interactive.400", "$hovered": "$background.raised.high" } }
      }
    }
  },
  "dark": {
    "hue": {
      "gray": { "100": "#f8fafc", "200": "#e2e8f0", "300": "#cbd5e1", "400": "#94a3b8", "500": "#64748b", "600": "#475569", "700": "#334155", "800": "#1e293b", "900": "#0f172a" },
      "red": {}, "orange": {}, "yellow": {}, "green": {}, "cyan": {}, "blue": {}, "purple": {},
      "accent": "$hue.purple",
      "interactive": "$hue.cyan",
      "neutral": "$hue.gray"
    },
    "categorical": ["accent", "purple", "cyan"]
  }
}
```

## Token reference

| Path | Cardinality | Required | Value domain |
| :--- | :--- | :--- | :--- |
| `base.categorical` | array | yes | Non-empty array of declared hue names/aliases |
| `text.base`, `text.muted` | scalar | yes | Any color |
| `text.action.<action>.base` | scalar | yes | Any color |
| `text.action.<action>.$<state>` | scalar | no | Any color (falls back to `.base`) |
| `text.formfield.base` / `.$<state>` | scalar | base only | Any color |
| `text.feedback.<kind>.base` | scalar | yes | Any color |
| `text.feedback.<kind>.muted` | scalar | no | Any color |
| `background.base` | scalar | yes | Any color |
| `background.raised.{base,high,max}` | scalar | yes | Any color |
| `background.action.<action>.base` / `.$<state>` | scalar | base only | Any color |
| `background.formfield.base` / `.$<state>` | scalar | base only | Any color |
| `background.feedback.<kind>.base` | scalar | yes | Any color |
| `border.base`, `scrollbar.base` | scalar | yes | Any color |
| `diff.text.{added,removed,context,hunkHeader}` | scalar | yes | Any color |
| `diff.background.{added,removed,context}` | scalar | yes | Any color |
| `diff.highlight.{added,removed}` | scalar | yes | Any color |
| `diff.lineNumber.text`, `diff.lineNumber.background.{added,removed}` | scalar | yes | Any color |
| `syntax.<key>` (9) | scalar | yes | Hex or `$hue.<name>.<step>` |
| `markdown.<key>` (14) | scalar | yes | Hex or `$hue.<name>.<step>` |
| `@dialog.<group>` | group | no | Same groups/keys as `base` |
| `<mode>.hue.<base hue>` | scale | yes (8) | Nine literal hex steps `100`–`900` |
| `<mode>.hue.{accent,interactive,neutral}` | alias | yes (3) | `$hue.<name>` or a full scale |

`<action>` = `primary` \| `secondary` \| `destructive`; `<state>` = `$hovered` \| `$focused` \|
`$pressed` \| `$selected` \| `$disabled`; `<kind>` = `error` \| `warning` \| `success` \| `info`.

## Value rules

- **Any color**: `#rgb`, `#rgba`, `#rrggbb`, `#rrggbbaa`, `transparent`, or a `$` reference.
- **Hue scale steps**: literal hex only; all nine steps required; unknown steps are rejected.
- **References**: `$hue.<name>` (aliases), `$hue.<name>.<step>`, `$text.*`, `$background.*`,
  `$border.*`, `$scrollbar.*`, `$diff.*`, `$syntax.*`, `$markdown.*`. An unresolvable reference
  rejects the theme.
- **`syntax.*` / `markdown.*`**: hex or `$hue.<name>.<step>` only.

## Migrating from the 2.0.8-era format

| Obsolete | v2.0.15 |
| :--- | :--- |
| Tokens nested inside `dark`/`light` only | Complete top-level `base` tree; modes carry `hue` plus optional overrides |
| `text.default` / `text.subdued` | `text.base` / `text.muted` |
| `<group>.default` for backgrounds, borders, scrollbars | `<group>.base` |
| `background.<group>.default` on actions and form fields | `.base` (plus `$hovered`, `$focused`, …) |
| `text.status.{running,question,permission,unread}` | Removed; derived internally from feedback and action tokens |
| `@context:elevated`, `@context:overlay` | `@dialog` |
| `version`, `$schema` wrapper | No `version` field; `$schema` is an optional string |
| Light and dark both required | One mode alone is legal |

## Validation

```bash
node scripts/validate-theme.mjs <theme.json>
```

Exit 0 means the file satisfies the loader contract, including reference resolution. The validator
rejects the obsolete format by name (`default`, `subdued`, `text.status`, `@context:*`, `version`) so
a stale theme cannot pass review again.
