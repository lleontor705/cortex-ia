# Tasks — Nan Ink Visual Redesign (nan-ink-visual)

Dependency-safe DAG in board nan-ink-visual, flexible workload policy. Full contracts in plan.md; machine-readable requirements in specs/visual/spec.md.

- [ ] 1. nan-ink-tokens — Wave 1, no deps
  Requirements: REQ-TOKEN-001, REQ-THEME-001
  Add internal/tui/styles/tokens.go (canonical Nan Ink dark+light tokens), the bounded tokens_test.go (at most 250 lines; pins hexes, AA contrast, theme JSON spot checks), and internal/assets/themes/cortex-ia.json (full OpenCode v2 theme format).
  Verify: go test -count=1 ./internal/tui/styles/... ./internal/assets/...
  Review: independent reviewer (new install surface).

- [ ] 2. nan-ink-tui-theme — Wave 2, deps: nan-ink-tokens
  Requirements: REQ-TUI-001, REQ-TUI-002
  ApplyTheme to Nan Ink dark+light, hairline border discipline, consolidate views.go ad-hoc styles onto the styles package, replace Web screen emoji with glyph triads, bold+uppercase headers on Home and Review, update theme_test.go without weakening.
  Verify: go test -count=1 ./internal/tui/...
  Review: reviewer (test-pinned visuals).

- [ ] 3. nan-ink-web-css — Wave 2 parallel, deps: nan-ink-tokens
  Requirements: REQ-WEB-001, REQ-WEB-002
  styles.css token overhaul (dark default plus prefers-color-scheme light), radius 0, hairlines, density pass, system fonts dropping Inter, statusColors to tokens, replace the four inline style sites with classes and data-attribute custom properties, rebuild and commit the static output.
  Verify: npm --prefix web run build && go test -count=1 ./internal/cortexiaweb/...
  Review: reviewer (CSP fix is security-adjacent).

- [ ] 4. nan-ink-plugin — Wave 2 parallel, deps: nan-ink-tokens
  Requirements: REQ-PLUGIN-001
  Restyle NanUsageStrip, NanModelPickerPanel, NanDetailPanel, and the sidebar to Nan Ink in cortex-ia-tui.tsx; regenerate the compiled asset via tsup; never hand-edit the .js.
  Verify: npm --prefix internal/tuiassets run build
  Review: reviewer (compiled asset regeneration).

- [ ] 5. nan-ink-tui-stats — Wave 3, deps: nan-ink-tui-theme
  Requirements: REQ-TUI-002
  Stats screen and heatmap restyle to Nan Ink, consolidate stats_screen.go ad-hoc styles, numbered-section headers, update stats_screen_test.go assertions without weakening.
  Verify: go test -count=1 ./internal/tui/...
  Review: reviewer.

- [ ] 6. nan-ink-docs — Wave 3, deps: nan-ink-tokens, nan-ink-tui-theme, nan-ink-web-css, nan-ink-plugin, nan-ink-tui-stats
  Requirements: REQ-DOC-001
  Write docs/reference/nan-ink-theme.md (theme, palettes, cli.json theme.name activation, install-and-switch verification note) and add the CHANGELOG Unreleased entry.
  Verify: test -s docs/reference/nan-ink-theme.md && test -s CHANGELOG.md
  Review: orchestrator auto-approval (documentation).
