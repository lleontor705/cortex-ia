# Spec delta — Nan Ink visual redesign

Domain: visual | Plane: hybrid | Change: nan-ink-visual

## ADDED Requirements

### Requirement: REQ-TOKEN-001 Canonical Nan Ink token module
The Nan Ink dark and light palettes SHALL be defined exactly once in Go in a new internal/tui/styles/tokens.go exporting typed token sets as hex constants. A bounded new test file internal/tui/styles/tokens_test.go (at most 250 lines, within the authorized TUI testing scope) SHALL pin the canonical hexes, assert AA contrast for accent-text on dark (#9B6BF0 on #0B0B0C, 5.39:1), and spot-assert that the theme asset carries the same critical hexes.

#### Scenario: Restyled surfaces reference tokens
- **GIVEN** the token module exists with the pinned dark and light hexes
- **WHEN** any restyled surface needs a Nan Ink color
- **THEN** it references the token and introduces no ad-hoc hex literal outside tokens.go

#### Scenario: Contrast pin holds
- **GIVEN** tokens_test.go computes contrast for accent-text on the dark background
- **WHEN** the test runs
- **THEN** the ratio is at least 4.5:1 and matches the pinned 5.39:1 value

#### Scenario: Drift detection
- **GIVEN** any canonical hex constant is changed
- **WHEN** tokens_test.go runs
- **THEN** the test fails, naming the drifted token

### Requirement: REQ-THEME-001 OpenCode v2 theme asset
A new embedded theme internal/assets/themes/cortex-ia.json SHALL follow the documented OpenCode v2 theme format: full base token tree, complete dark AND light hue palettes (gray, red, orange, yellow, green, cyan, blue, purple; steps 100-900; contrast direction inverting between modes), categorical hues, background.raised base/high/max, action states primary/secondary/destructive across hovered/focused/pressed/selected/disabled with fallbacks, and diff/syntax/markdown groups.

#### Scenario: Asset maps to the managed theme destination
- **GIVEN** the embedded asset themes/cortex-ia.json
- **WHEN** assets.Classify and the asset mapping run
- **THEN** the file maps to KindTheme and installs to the OpenCode themes root as cortex-ia.json without any assetmap.go change

#### Scenario: Both modes are complete
- **GIVEN** a user sets cli.json theme.name to cortex-ia and switches dark and light
- **WHEN** the theme renders in either mode
- **THEN** every hue scale has all steps 100-900 and no missing-step fallback appears

#### Scenario: Structural validity is machine-checked
- **GIVEN** the theme JSON is embedded
- **WHEN** tokens_test.go parses and spot-checks it
- **THEN** the JSON is valid and the critical base/dark/light hexes match the canonical tokens

### Requirement: REQ-TUI-001 TUI theme seam rebuilt on Nan Ink
ApplyTheme in internal/tui/styles/theme.go SHALL rebuild all global styles and color roles from the Nan Ink tokens for dark and light, and SHALL replace the mixed double-border standard (Box rounded vs Panel normal vs global frame) with the hairline rule: 1px hairline borders only on active panels.

#### Scenario: Theme switch rebuilds every style
- **GIVEN** the TUI switches between dark and light
- **WHEN** ApplyTheme runs
- **THEN** all 13 global styles and 8 color roles are rebuilt from tokens.go values for the active mode

#### Scenario: Hairline border discipline
- **GIVEN** the restyled global Box, Frame, and Panel styles
- **WHEN** any panel renders
- **THEN** active panels use hairline borders and no rounded double-border combination remains

#### Scenario: Theme tests updated without weakening
- **GIVEN** theme_test.go pins palette or style assertions
- **WHEN** the restyle changes pinned values
- **THEN** assertions are updated to the new Nan Ink values and no assertion is deleted or loosened

### Requirement: REQ-TUI-002 TUI consolidation, iconography, and pinned invariants
Ad-hoc lipgloss duplicates in views.go (styleTitle through styleFrame) and stats_screen.go (statsCardStyle and siblings) SHALL be consolidated onto the styles package, the Web screen emoji block in views.go SHALL become glyph triads, and headers on Home, Review, and Stats SHALL use bold+uppercase titles with numbered sections. The responsive logo geometry (thresholds 37/24/20/16) and all navigation labels pinned by navigation_test.go MUST NOT change.

#### Scenario: Ad-hoc styles consolidated
- **GIVEN** views.go and stats_screen.go define local lipgloss styles
- **WHEN** the consolidation task completes
- **THEN** those styles resolve through the styles package tokens with no duplicate hex literals

#### Scenario: Iconography unified to glyph triads
- **GIVEN** the Web screen emoji block in views.go
- **WHEN** the screen renders
- **THEN** status uses glyph+color+text triads (check, cross, dot, arrow) with no emoji and no Nerd Fonts

#### Scenario: Pinned logo and labels survive
- **GIVEN** theme_test.go logo thresholds and navigation_test.go labels
- **WHEN** the full TUI test suite runs
- **THEN** all logo geometry and navigation string assertions pass unchanged

### Requirement: REQ-WEB-001 Web token overhaul
web/src/styles.css SHALL define the Nan Ink tokens in :root with dark as default and override them under @media (prefers-color-scheme: light), apply radius 0 and hairline components, run a density pass on Kanban and tables, switch to system font stacks removing the declared-but-unpackaged Inter, and the duplicated statusColors map in flow-canvas.jsx SHALL reference the shared tokens.

#### Scenario: Light mode arrives via media query
- **GIVEN** a user prefers light color scheme
- **WHEN** the console renders
- **THEN** the snow palette overrides apply without a runtime toggle and dark remains the default

#### Scenario: statusColors share the token source
- **GIVEN** flow-canvas.jsx declares status colors
- **WHEN** the restyle completes
- **THEN** the duplicated local hex map is replaced by the shared token values from styles.css

#### Scenario: Font stack cleanup
- **GIVEN** the current CSS declares Inter without packaging it
- **WHEN** the restyle lands
- **THEN** only system font stacks remain and no unpackaged font is referenced

### Requirement: REQ-WEB-002 CSP inline-style fix
The content security policy remains style-src 'self'. The four inline style sites (main.jsx progress bar near line 754; flow-canvas.jsx transforms near lines 438, 443, and 535) SHALL be replaced with CSS classes and custom properties driven by data-attributes so that no inline style attributes are emitted.

#### Scenario: Zero inline styles at runtime
- **GIVEN** the console is served with style-src 'self'
- **WHEN** any page renders including the flow canvas and progress bar
- **THEN** no inline style attribute is emitted and transforms apply via classes and custom properties

#### Scenario: CSP is not widened
- **GIVEN** the server CSP configuration
- **WHEN** the restyle lands
- **THEN** style-src remains 'self' and the server tests pass unchanged in policy

#### Scenario: Embedded build stays green
- **GIVEN** web sources changed
- **WHEN** npm --prefix web run build and the cortexiaweb server tests run
- **THEN** the build succeeds, compiled static output is regenerated and committed, and go test on internal/cortexiaweb passes

### Requirement: REQ-PLUGIN-001 Nan panels restyled at the source seam
The cortex-ia-tui.tsx panels (NanUsageStrip, NanModelPickerPanel, NanDetailPanel, and the persistent sidebar with Kanban snapshot and braille sparklines) SHALL be restyled to Nan Ink (density, glyph hierarchy, violet accent, hairlines), and the compiled internal/assets/tui/cortex-ia-tui.js SHALL be regenerated via the tsup pipeline and committed. The .js file MUST never be hand-edited.

#### Scenario: Restyle happens only in the TSX source
- **GIVEN** the compiled asset and its TSX source
- **WHEN** the panels are restyled
- **THEN** every visual change is present in cortex-ia-tui.tsx and the .js diff is exactly the tsup regeneration output

#### Scenario: Compiled asset regenerated deterministically
- **GIVEN** the changed TSX source
- **WHEN** npm --prefix internal/tuiassets run build runs
- **THEN** internal/assets/tui/cortex-ia-tui.js is regenerated from the fixed entry name and committed

#### Scenario: Sidebar density and glyphs follow Nan Ink
- **GIVEN** the persistent sidebar with Kanban snapshot and sparklines
- **WHEN** the panel renders
- **THEN** spacing density, glyph triads, violet accent, and hairlines match the Nan Ink identity

### Requirement: REQ-DOC-001 Theme documentation and release note
docs/reference/nan-ink-theme.md SHALL document the theme, both palettes, and activation via cli.json theme.name (themes belong to the terminal client, never opencode.json), and CHANGELOG.md SHALL gain an Unreleased entry covering the visual refresh.

#### Scenario: Activation is documented
- **GIVEN** the new docs page
- **WHEN** a user wants to activate the theme
- **THEN** the page states the cli.json theme.name step and notes install-and-switch verification because no JSON schema is published

#### Scenario: Both palettes are documented with pinned hexes
- **GIVEN** the docs page describes the Nan Ink identity
- **WHEN** the dark and light palettes are listed
- **THEN** every documented hex matches the canonical values in tokens.go

#### Scenario: Changelog entry exists
- **GIVEN** the completed visual wave
- **WHEN** the repository CHANGELOG is inspected
- **THEN** an Unreleased entry describes the Nan Ink theme and visual refresh
