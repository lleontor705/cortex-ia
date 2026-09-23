# Proposal: usage-stats-panel

## Why

Cortex observation #217 (topic `architecture/opencode-stats-panel`) verified, via live read-only queries on win32, that OpenCode v2 persists rich usage data in `C:/Users/usrLuisLeon/.local/share/opencode/opencode.db`: `session_v2` carries pre-aggregated token columns (input/output/reasoning/cache_read/cache_write), a JSON `model` column mergeable with `json_extract(model,'$.id')`, and epoch-ms `time_created`; `session_message` (~90K rows) is indexed by `time_created`. Read-only access via `modernc.org/sqlite` (already a direct dependency) was proven to work while OpenCode runs live.

cortex-ia launches its TUI with zero arguments (internal/app routes to internal/tui.Run) and has no usage-visibility surface. The user intent is a "login summary" panel replicating a reference capture: metric cards, tabs, a GitHub-style heatmap, and a playful footer — shown automatically at TUI boot and reachable from the Home menu.

## What Changes

- **New read-only stats package `internal/ocstats`**: resolves the OpenCode DB path (`CORTEX_IA_OPENCODE_DB` env override, default `~/.local/share/opencode/opencode.db`), opens it strictly read-only (`mode=ro`, `query_only`, `busy_timeout=10000`), runs a bounded set of aggregate bucket queries (day / day+hour / model+day), and returns an immutable report covering all three windows (Todo / 30d / 7d) computed in Go. The DB handle is closed immediately after the single load pass (WAL-pin avoidance per observation #217).
- **New TUI screen `screenStats`**: shown automatically as the initial screen at boot (async load with visible loading state) and reachable from a new Home menu entry "Estadísticas de uso". Renders six metric cards in a 3x2 grid (Sesiones, Mensajes, Tokens totales, Días activos, Hora pico, Modelo favorito), Resumen/Modelos tabs, Todo/30d/7d window filters, a GitHub-style heatmap, per-model ranking (sessions, tokens, share %), and a playful footer comparing token totals against The Great Gatsby (50,000 tokens ≈ 1 novela).
- **Graceful degradation**: if the OpenCode DB is missing or unreadable, the screen renders a clear Spanish message naming the resolved path; the TUI never crashes and Home remains reachable.
- **Test maintenance**: `TestScreenSet` (pins exactly 7 screens), `TestHomeNumericHotkeys` (1-7) and Home navigation tests are updated for the 8th screen and renumbered entries; new bounded test files (≤250 LOC each) cover the stats package and the screen.

## Capabilities

- `specs/usage-stats/spec.md` — ADDED requirements: REQ-US-001 read-only data source resolution, REQ-US-002 boot auto-show and menu visibility, REQ-US-003 Resumen metric cards, REQ-US-004 window filters, REQ-US-005 GitHub-style heatmap, REQ-US-006 Modelos ranking, REQ-US-007 Gatsby footer comparison, REQ-US-008 aggregation determinism.

## Impact

- **Code (new):** `internal/ocstats/ocstats.go`, `internal/ocstats/aggregate.go`, `internal/ocstats/ocstats_test.go`; `internal/tui/stats_screen.go`, `internal/tui/stats_screen_test.go`, `internal/tui/stats_heatmap.go`, `internal/tui/stats_heatmap_test.go`, `internal/tui/stats_boot_test.go`.
- **Code (modified):** `internal/tui/model.go` (screen const, state, dispatch, boot, menu entry), `internal/tui/views.go` (View case, homeDescriptions, footer keys), `internal/tui/navigation_test.go` (existing-assertion maintenance only; no new suites appended to this >300 LOC file).
- **Data:** read-only external access to OpenCode's own SQLite database; zero writes to it, zero changes to cortex-ia's delegation schema.
- **Dependencies:** zero new go.mod entries. `modernc.org/sqlite`, `bubbletea`, `bubbles`, `lipgloss` are already available; number formatting is hand-rolled so `humanize` stays indirect.

## Non-Goals

- No web dashboard, no export formats (CSV/JSON), no internal/cortexiaweb changes.
- No USD cost metrics (`session.cost` is 0 in the verified data source).
- No hardcoded 365-day window and no projection/inference beyond recorded data.
- No OpenCode plugin and no OpenCode-side modifications.
- No CLI dispatcher changes in internal/app beyond the existing zero-argument TUI boot.
- No persistent OpenCode DB handle in the TUI; no background polling/refresh loop.
- The planner writes no product code and claims no tasks; this change only specifies and materializes the DAG.
