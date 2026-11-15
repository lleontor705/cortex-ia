# Plan — Nan Ink Visual Redesign (nan-ink-visual)

Change: nan-ink-visual | Workflow: sdd-lite | Spec plane: hybrid | Workload policy: flexible | Board: nan-ink-visual

## 1. Intent

Restyle the cortex-ia visual surfaces to the operator-approved Nan Ink design language, keeping all current functionality, pinned navigation labels, and the responsive logo geometry intact. Surfaces in scope for this wave: shared design tokens plus the official OpenCode v2 theme asset (dark and light), the Bubble Tea TUI restyle through the single ApplyTheme seam, the web operations console restyle including the CSP inline-style fix, and the Nan panels in the OpenCode TUI plugin.

Non-goals (binding, prevent cascade amplification): no implementation in this planning phase; no RPC plugin, TUI slots, PTY terminal, or usage dashboard (excluded from this wave); no OpenCode core transcript/layout restyle (v2 themes are color-token substitution only); no breakage of the responsive logo system (thresholds 37/24/20/16 pinned by theme_test.go) or navigation labels pinned by navigation_test.go (EN/ES mix stays); no new test suites beyond the repository Testing Scope rule (update existing persistent tests only, never weaken); no font packaging (drop the declared-but-unpackaged Inter, system font stacks only); no CSP relaxation (the fix removes inline styles rather than widening style-src).

## 2. Requirements

Machine-readable requirements with Given/When/Then scenarios are defined in specs/visual/spec.md under ADDED Requirements:

- REQ-TOKEN-001 — Canonical Nan Ink token module in internal/tui/styles/tokens.go with bounded pin/contrast tests.
- REQ-THEME-001 — OpenCode v2 theme asset internal/assets/themes/cortex-ia.json (full base tree, dark and light hue scales, categorical, raised backgrounds, action states, diff/syntax/markdown).
- REQ-TUI-001 — ApplyTheme seam rebuilt on Nan Ink tokens with hairline border discipline.
- REQ-TUI-002 — TUI ad-hoc style consolidation, glyph-triad iconography, bold+uppercase numbered headers; pinned logo and labels untouched.
- REQ-WEB-001 — Web token overhaul: dark default plus prefers-color-scheme light, radius 0, hairlines, density pass, system fonts, shared statusColors.
- REQ-WEB-002 — CSP inline-style fix: the four inline style sites replaced with classes and data-attribute custom properties.
- REQ-PLUGIN-001 — Nan panels restyled in cortex-ia-tui.tsx; compiled asset regenerated via tsup and committed; the .js is never hand-edited.
- REQ-DOC-001 — Theme documentation and CHANGELOG Unreleased entry.

## 3. Design

### 3.1 Nan Ink direction (operator-approved)

Dark palette: bg #0B0B0C, surface #121214, text #FFFFFF, body #C9C9CC, muted #9A9A9E, accent-border #7D39EB (decoration and borders only, never text on dark), accent-text #9B6BF0 (5.39:1 AA), line #3A3A3D hairlines, selected bg #1D1430 (14 percent violet blend), status green #22C55E / amber #F59E0B / red #EF4444 always as glyph+color+text triad. Light "snow" palette: bg #EFEEEB, surface #E3E1DC, ink #101010, muted #6F6F6F, accent #7D39EB (usable as text on light). Formal identity: radius 0 everywhere; 1px hairline borders only on active panels (inactive borderless, gutter-separated); bold+uppercase titles; mono for data; numbered sections (01/label over hairline); caret _ cursor; no gradients beyond a 14 percent violet wash; system font stacks on web; glyph triads only (check, cross, dot, arrow) with no emoji and no Nerd Fonts.

### 3.2 Token source mechanism (pinned decision)

DECISION: internal/tui/styles/tokens.go is the single canonical Nan Ink palette source. The OpenCode theme JSON format is platform-dictated and cannot be generated into lipgloss or CSS without codegen machinery, which the anti-overengineering rule forbids; CSS custom properties are hand-authored per surface. Cross-surface consistency is enforced by (a) tokens_test.go pinning canonical hexes, asserting AA contrast, and spot-asserting that cortex-ia.json carries the same critical hexes for base/dark/light tokens, and (b) the review policy flagging any new hex literal introduced outside tokens.go.

### 3.3 Component seams

TUI: styles/theme.go ApplyTheme is the single restyle seam (13 global styles + 8 color roles); ShimmerLogoArt recoloring is allowed because it is width-preserving, geometry frozen. Web: styles.css :root is the seam; components consume custom properties; the duplicated flow-canvas statusColors map is replaced with token lookups. Plugin: cortex-ia-tui.tsx source is the seam; tsup outDir ../assets/tui regenerates internal/assets/tui/cortex-ia-tui.js deterministically from the fixed entry name. Install surface: assets.Classify already maps themes/*.json to KindTheme and assetmap Destination routes to ThemesRoot, so no assetmap.go change is required (verified: inventory_test.go uses a synthetic FS and install_test.go asserts kind coverage, not file counts).

### 3.4 Build ordering constraints

The web build (npm --prefix web run build) MUST run before any go build because compiled static assets under internal/cortexiaweb/static/ are go:embed'ed; both source and generated output are committed. The plugin asset build regenerates the compiled .js which is committed. Conventional commits are required (feat/fix prefixes drive changelog eligibility).

## 4. Tasks

Dependency-safe DAG in board nan-ink-visual; every work item pins this plan.md via sdd_contract. Waves with disjoint allowed_files:

- [ ] 1. nan-ink-tokens — REQ-TOKEN-001, REQ-THEME-001
- [ ] 2. nan-ink-tui-theme — REQ-TUI-001, REQ-TUI-002 (deps: 1)
- [ ] 3. nan-ink-web-css — REQ-WEB-001, REQ-WEB-002 (deps: 1; parallel with 2 and 4)
- [ ] 4. nan-ink-plugin — REQ-PLUGIN-001 (deps: 1; parallel with 2 and 3)
- [ ] 5. nan-ink-tui-stats — REQ-TUI-002 (deps: 2)
- [ ] 6. nan-ink-docs — REQ-DOC-001 (deps: 1, 2, 3, 4, 5)

Verification gates are the raw commands recorded in tasks.md and each work item verification field. Review policy: independent reviewer for tasks 1-5 (new install surface, test-pinned visuals, security-adjacent CSP fix, compiled asset regeneration); orchestrator auto-approval for task 6 (documentation, Case 2).

## 5. Risks

harness_test.go or navigation_test.go may pin visuals that break inside a TUI task outside its allowed_files; the orchestrator then dispatches a bounded follow-up fix, updating tests without weakening and never decomposing pure-test tasks. The theme JSON has no published schema, so install-and-switch manual verification is documented in the docs task. views.go consolidation churn may exceed 150 LOC under the flexible budget: non-blocking WORKLOAD_ADVISORY evaluated by the reviewer. The web static output is a hashed bundle, so the generated directory is leased as a single allowed path (documented exception to the 1-3 source-file rule; it is build output, not source logic).
