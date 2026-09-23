# Tasks: usage-stats-panel

Dependency-ordered DAG materialized on board `usage-stats-panel` (workflow sdd-lite, spec_plane hybrid, workload_policy flexible). Every task respects the 1-3 file lease bound and the flexible Go budget (source ≤700 LOC, test files ≤250 LOC each). Verification fields are raw executable commands.

## usp-01 — [ocstats] Read-only OpenCode usage stats package

- **Files:** `internal/ocstats/ocstats.go`, `internal/ocstats/aggregate.go`, `internal/ocstats/ocstats_test.go` (new)
- **Scope:** path resolver (`CORTEX_IA_OPENCODE_DB` override → `~/.local/share/opencode/opencode.db`), read-only opener with the pinned DSN, typed not-found/unreadable errors, Window/DayBucket/ModelUsage/Summary/Report types, bucket queries (day, day+hour, model×day, range) + Go reduction for all three windows, deterministic tie-breaks, epoch-ms → local tz.
- **Pre-step:** read-only `PRAGMA table_info(session_v2)` and `PRAGMA table_info(session_message)` against the live DB; record verified column names in the task receipt.
- **Tests (≤250 LOC):** synthetic temp fixture DBs (session_v2 + session_message minimal schema); exact aggregates, window clipping, tie-breaks, missing-DB typed error, env override, local-tz bucket expectations derived from time.Local.
- **Verification:** `go test ./internal/ocstats -count=1`
- **Depends on:** — (Wave 1)

## usp-02 — [tui] Heatmap widget: GitHub-style weekly intensity grid

- **Files:** `internal/tui/stats_heatmap.go`, `internal/tui/stats_heatmap_test.go` (new)
- **Scope:** `renderHeatmap(days []ocstats.DayBucket, s ocstats.Summary) string`: Sunday-first week columns × 7 weekday rows, 5 intensity levels from the day's messages relative to the window max (level 0 = none), range bounded by the window (Todo = real first-to-last range; no 365-column grid), colors derived from the existing styles palette, month labels as best-effort.
- **Tests (≤250 LOC):** level monotonicity, max-level saturation, zero-activity level 0, single-week 7d grid, real-range Todo span, empty window renders without panic.
- **Verification:** `go test ./internal/tui -run TestHeatmap -count=1`
- **Depends on:** usp-01 (Wave 2)

## usp-03 — [tui] statsState screen core: cards, tabs, filters, Modelos, footer, degradation

- **Files:** `internal/tui/stats_screen.go`, `internal/tui/stats_screen_test.go` (new)
- **Scope:** self-contained `statsState` struct: loading/loaded/degraded states, `update(tea.KeyMsg)` (tab/←/→ tabs Resumen|Modelos; 1/2/3 window Todo|30d|7d; q/ctrl+c quit), `view(width)`: 3x2 card grid (Sesiones, Mensajes, Tokens totales, Días activos, Hora pico, Modelo favorito) with grouped separators, Modelos ranking table (sessions, tokens, share % one decimal), Gatsby footer (50,000 tokens ≈ 1 novela; ≥1 → "Nx más tokens que…", <1 → percentage; no NaN), degraded message "No se encontró la base de OpenCode en <ruta>", `statsLoadCmd` + `statsLoadedMsg` with var-indirection seam, heatmap wired into Resumen via usp-02 widget.
- **Tests (≤250 LOC):** card values from a fixture report, default Todo, window switching is pure render (no loader call), footer branches, degraded rendering, tab/filter key handling.
- **Verification:** `go test ./internal/tui -run TestStats -count=1`
- **Depends on:** usp-01, usp-02 (Wave 3)

## usp-04 — [tui] Wire screenStats: boot auto-show, home menu entry, navigation maintenance

- **Files:** `internal/tui/model.go`, `internal/tui/views.go`, `internal/tui/navigation_test.go` (modified)
- **Scope:** `screenStats` const + `stats statsState` field + Update/View dispatch; `newModel` boots into screenStats with the async load; `Init` returns the load command; Home menu gains "Estadísticas de uso" after Agent Studio (index 4; Doctor→5, Uninstall→6, Quit→7); `homeDescriptions` aligned; Home footer hint "1-8/enter select"; esc from stats returns Home with cursor on the entry; menu selection re-dispatches a fresh load. navigation_test.go: **modify existing assertions only** (TestScreenSet 7→8, TestHomeNumericHotkeys 1-8, boot-screen expectations) — never append new suites to this >300 LOC file.
- **Verification:** `go test ./internal/tui -count=1`
- **Depends on:** usp-03 (Wave 4)

## usp-05 — [tui] Stats integration oracle and full local gates

- **Files:** `internal/tui/stats_boot_test.go` (new)
- **Scope:** end-to-end boot/navigation oracle (≤250 LOC): zero-arg boot lands on screenStats in loading state; executing the returned command delivers statsLoadedMsg and renders loaded values; degraded path via failing loader seam; esc→home with cursor on entry; hotkey reachability from Home; heatmap+cards+footer co-render sanity on one loaded frame.
- **Verification:** `go test ./internal/tui -run TestStatsBoot -count=1`
- **Depends on:** usp-04 (Wave 5)

## Integral verification checklist (reviewer / operator)

- [ ] `gofmt -s -l ./internal/ocstats ./internal/tui` → empty
- [ ] `go vet ./...` → clean
- [ ] `go build ./...` → ok
- [ ] `go test ./internal/... -count=1` → green
- [ ] Manual smoke (operator): `go run ./cmd/cortex-ia` boots into Estadísticas with the real DB (cards, tabs, heatmap, footer render); with `CORTEX_IA_OPENCODE_DB=X:/missing.db` boots into the degraded message naming the path; esc → Home; menu entry reopens with fresh load; 1/2/3 and tab keys switch filter/tab; `q` quits from the stats screen.
- [ ] No go.mod changes; no writes to opencode.db; no internal/cortexiaweb or internal/app edits.
