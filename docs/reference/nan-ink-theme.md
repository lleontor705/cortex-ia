# Nan Ink Theme

Nan Ink is the cortex-ia visual identity: a single, high-density dark-and-light design language applied consistently across every shipped surface. The canonical palette lives in `internal/tui/styles/tokens.go`; the TUI theme seam, the web console CSS, and the embedded OpenCode theme asset all mirror those values.

## Palette

| Token | Dark | Light | Role |
| :--- | :--- | :--- | :--- |
| `BG` | `#0B0B0C` | `#EFEEEB` | Canvas background |
| `Surface` | `#121214` | `#E3E1DC` | Raised surface / cards |
| `Line` | `#3A3A3D` | `#D5D3CE` | Hairline borders and rules |
| `Text` | `#FFFFFF` | `#101010` | Primary text |
| `Body` | `#C9C9CC` | `#2E2E2E` | Body and secondary text |
| `Muted` | `#9A9A9E` | `#6F6F6F` | Muted / de-emphasized text |
| `AccentBorder` | `#7D39EB` | `#7D39EB` | Violet accent for borders, rules, and focus rings (decoration only) |
| `AccentText` | `#9B6BF0` | `#7D39EB` | Accent for text (AA-safe on its canvas) |
| `Selected` | `#1D1430` | `#E7DCFB` | Selected row / chip background |
| `Success` | `#22C55E` | `#166534` | Success status |
| `Warning` | `#F59E0B` | `#92400E` | Warning status |
| `Danger` | `#EF4444` | `#B91C1C` | Error / destructive status |

`AccentBorder` and `AccentText` are deliberately separate roles. On dark, `#7D39EB` reads well on borders but fails AA as a text foreground, so text uses `#9B6BF0` (5.39:1 against the dark canvas). On light, `#7D39EB` clears AA and serves both roles. Treat `AccentBorder` as structural only and never promote it to a text color.

## Identity rules

- **Radius 0.** No rounded corners anywhere.
- **Hairline borders.** 1px borders only on active panels; inactive panels are borderless and separated by gutters.
- **Titles.** Bold and uppercase, with numbered sections (`01 / LABEL` over a hairline).
- **Data.** Monospace for values and tables.
- **Iconography.** Glyph triads only — check, cross, dot, arrow — paired with color and text. No emoji, no Nerd Fonts.
- **Caret.** A `_` block caret.
- **Status.** Always a glyph + color + text triad, never color alone.
- **Depth.** No gradients beyond a 14 percent violet wash.
- **Typography (web).** System font stacks only.

## Activating the OpenCode theme

The OpenCode v2 terminal theme is the `cortex-ia` asset (`internal/assets/themes/cortex-ia.json`), which ships complete dark and light hue palettes.

1. Run `cortex-ia install` (or `cortex-ia sync`). Theme assets are copied to `~/.config/opencode/themes/`, so the file lands as `~/.config/opencode/themes/cortex-ia.json`. The filename without `.json` is the theme name.
2. Select it from the TUI (**Ctrl+P → Open settings → Theme**) or directly in `~/.config/opencode/cli.json`:

```json title="~/.config/opencode/cli.json"
{
  "theme": {
    "name": "cortex-ia",
    "mode": "system"
  }
}
```

`theme.mode` accepts `system` (follow the terminal appearance), `dark`, or `light`.

> Themes belong to the terminal client. Theme selection lives **only** in `cli.json` — never in `opencode.json` or `opencode.jsonc`.

### Verification note

There is no published JSON schema for custom themes, and OpenCode reacts to an invalid or unloadable theme by **silently falling back to a builtin theme** — no error, warning, or log line. Verify activation by switching modes (`theme.mode: "dark"` then `"light"`) and confirming the Nan Ink palette renders. The asset can also be validated against the loader contract before shipping changes:

```bash
node scripts/validate-theme.mjs internal/assets/themes/cortex-ia.json
```

### Managed theme key

`cortex-ia install` and `cortex-ia sync` manage the `theme` key in `cli.json` and refresh `theme.name` to the bundled `cortex` theme. Selecting `cortex-ia` is a manual choice, so re-select it (in settings or in `cli.json`) after a later install or sync.

## Surfaces

| Surface | Where | Mode selection |
| :--- | :--- | :--- |
| OpenCode TUI (v2 terminal client) | `themes/cortex-ia.json` | `cli.json` `theme.mode` / TUI Theme picker |
| cortex-ia Bubble Tea TUI | `internal/tui/styles/theme.go` (`ApplyTheme`) over `tokens.go` | Cortex-ia dark/light themes |
| Web console (`cortex-ia web`) | `web/src/styles.css` tokens | Nan Ink dark by default; light snow via `prefers-color-scheme` |
| OpenCode TUI plugin panels | `internal/tuiassets/cortex-ia-tui.tsx` (compiled `internal/assets/tui/cortex-ia-tui.js`) | Follows the OpenCode theme |

## Live Minions

The OpenCode TUI plugin's **Workers & Delegation** section read a dead delegation projection that no longer carried data. It is now **Live Minions**, fed by live subagent events and reusing the same agent-row pipeline as the rest of the plugin. Supporting counts were updated to match: the control health bar derives from `done / (done + blocked)`, the Kanban footer reads `(N tasks · N minions)`, and the home status line shows the live minion count.

## Web console CSP

The console no longer emits inline styles: the progress bar and canvas transforms are driven by CSS classes and data-attribute custom properties. The Content-Security-Policy `style-src 'self'` directive is unchanged and was not widened.
