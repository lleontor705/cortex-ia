# Plan: usage-stats-panel

## Intent

Show an OpenCode usage summary panel ("login summary") when the cortex-ia TUI boots with zero arguments, replicating a reference capture: six metric cards (3x2), Resumen/Modelos tabs, Todo/30d/7d window filters, a GitHub-style activity heatmap, and a playful Gatsby token footer. The panel is also reachable from a new Home menu entry "Estadísticas de uso". Data comes strictly read-only from OpenCode's own `opencode.db` (verified schema, Cortex observation #217) with zero new go.mod dependencies.

**Non-goals:** no web dashboard or CSV/JSON exports; no USD cost metrics (session.cost is 0); no hardcoded 365-day window or projections; no OpenCode plugin; no internal/cortexiaweb or internal/app dispatcher changes; no persistent DB handle or background polling; planner writes no product code.

## Requirements

Normative deltas with Given/When/Then scenarios live in `specs/usage-stats/spec.md` (8 requirements, 25 scenarios):

- **REQ-US-001** Read-only data source: `CORTEX_IA_OPENCODE_DB` override → default `~/.local/share/opencode/opencode.db`; DSN `mode=ro` + `query_only` + `busy_timeout(10000)`; typed not-found/unreadable errors naming the path; handle closed after one load pass.
- **REQ-US-002** Boot auto-show on `screenStats` (async load, loading state first) + Home menu entry (index 4, hotkey renumber to 1-8, footer hint updated); `esc` → Home, `q` quits.
- **REQ-US-003** Resumen: six Spanish-labeled cards (Sesiones, Mensajes, Tokens totales, Días activos, Hora pico, Modelo favorito) with grouped separators; loading state; degraded state "No se encontró la base de OpenCode en <ruta>".
- **REQ-US-004** Window filters: keys 1/2/3 = Todo/30d/7d; default Todo bounded by the real data range; filter switch is pure in-memory render.
- **REQ-US-005** Heatmap: Sunday-first weeks × 7 weekday rows, 5 intensity levels from messages/day relative to window max; window-scoped range (no 365-column grid); month labels best-effort.
- **REQ-US-006** Modelos tab: per-model sessions/tokens/share% (one decimal) via `json_extract(model,'$.id')`; order sessions DESC, tokens DESC, id ASC; NULL ids excluded.
- **REQ-US-007** Gatsby footer: 50,000 tokens ≈ 1 novela; ratio ≥1 → "Nx más tokens que The Great Gatsby" (one decimal), <1 → percentage; no NaN on 0 tokens; recomputes per window.
- **REQ-US-008** Determinism: epoch-ms → process-local tz buckets; earliest-hour and lexicographic tie-breaks; aggregation oracle on synthetic temp DBs whose schema mirrors the PRAGMA-verified columns.

## Design

- **New package `internal/ocstats`** (ocstats.go: resolver, opener, types, errors; aggregate.go: queries + Go reduction): one load pass runs 5 bounded aggregates over full history (range totals; daily session/token buckets; daily message buckets; day+hour histogram; model×day), closes the DB, then reduces the buckets per window in Go. One load serves all three windows; filter switching never reopens the DB (WAL-pin safety per observation #217; snapshot-copy of the 12.5 GB file rejected).
- **TUI seam:** `statsState` is a self-contained struct in `internal/tui/stats_screen.go` owning report/path/error/loading/tab/window state, its own key handling and `view(width)`; `model.go` only adds the `screenStats` const, the embedded field, dispatch cases, boot command and menu entry. Loader uses a var-indirection seam (`statsLoadCmd`/`statsLoadedMsg`) like the existing `cortexBinaryMissing` pattern, keeping the screen testable without wiring and the implementation waves disjoint.
- **Rendering:** reuses `internal/tui/styles` palette and house helpers (`truncate`, `clampScreen`, header/footer, `lipgloss.JoinHorizontal` for the 3x2 grid). Heatmap in `stats_heatmap.go`: `renderHeatmap(days, summary)` pure widget, 5-level ramp Muted→Success. UI copy in Spanish matching the `viewWeb` precedent; code and artifacts in English. Thousands grouping hand-rolled (`humanize` stays indirect).
- **Semantics pinned:** tokens = input+output+reasoning+cache_read+cache_write; ActiveDays = distinct local dates with ≥1 session; intensity = messages/day (denser signal); peak hour = most messages, tie → earliest hour; model order per REQ-US-006; favorite = first row.
- **Boot sequence:** `newModel` → `screenStats` (loading) → `Init` returns `statsLoadCmd` → `statsLoadedMsg` fills and re-renders; typed failure renders degradation; Home entry re-dispatches a fresh bounded load.
- **Workload:** flexible policy; every task ≤3 leased files, source ≤700 LOC Go, each new test file ≤250 LOC; navigation_test.go receives modifications to existing assertions only (no new suites appended to that >300 LOC file).

## Tasks

Dependency-ordered DAG: usp-01 → usp-02 → usp-03 → usp-04 → usp-05. Each entry maps 1:1 to a `cortex-ia work` node on board `usage-stats-panel`.

- [ ] usp-01 Implement the internal/ocstats read-only OpenCode usage stats package: path resolver with CORTEX_IA_OPENCODE_DB override, read-only opener with the pinned DSN, typed not-found/unreadable errors, Window/DayBucket/ModelUsage/Summary/Report types, bucket queries plus Go per-window reduction, deterministic tie-breaks, epoch-ms to local-timezone buckets. Files: internal/ocstats/ocstats.go, internal/ocstats/aggregate.go, internal/ocstats/ocstats_test.go. Depends on: none. Verification: go test ./internal/ocstats -count=1
  Requirements: REQ-US-001, REQ-US-008
- [ ] usp-02 Implement the GitHub-style heatmap widget renderHeatmap in internal/tui: Sunday-first week columns by 7 weekday rows, five intensity levels from messages per day relative to the window max, window-scoped range without a hardcoded 365-column grid, styles palette ramp. Files: internal/tui/stats_heatmap.go, internal/tui/stats_heatmap_test.go. Depends on: usp-01. Verification: go test ./internal/tui -run TestHeatmap -count=1
  Requirements: REQ-US-005
- [ ] usp-03 Implement the self-contained statsState screen core in internal/tui/stats_screen.go: loading/loaded/degraded states, key handling for tabs and window filters, 3x2 Spanish metric card grid with grouped separators, Modelos ranking table, Gatsby footer branches, degradation message naming the resolved path, statsLoadCmd/statsLoadedMsg loader seam, heatmap integration via the usp-02 widget. Files: internal/tui/stats_screen.go, internal/tui/stats_screen_test.go. Depends on: usp-01, usp-02. Verification: go test ./internal/tui -run TestStats -count=1
  Requirements: REQ-US-003, REQ-US-004, REQ-US-006, REQ-US-007
- [ ] usp-04 Wire screenStats into the TUI: screenStats const, stats state field, Update and View dispatch, boot into screenStats with async load from Init and newModel, Home menu entry Estadísticas de uso after Agent Studio with homeDescriptions alignment and 1-8 footer hint, esc cursor restore, fresh load on menu re-entry, and maintenance of existing navigation_test.go assertions (screen set 7 to 8, hotkeys 1 to 8, boot-screen expectations; no new suites appended). Files: internal/tui/model.go, internal/tui/views.go, internal/tui/navigation_test.go. Depends on: usp-03. Verification: go test ./internal/tui -count=1
  Requirements: REQ-US-002
- [ ] usp-05 Add the stats boot and navigation integration oracle in internal/tui/stats_boot_test.go: zero-argument boot lands on screenStats in loading state, executing the returned command delivers statsLoadedMsg and renders loaded values, failing loader seam renders degradation, esc returns Home with cursor on the stats entry, hotkey reachability from Home, one loaded frame co-renders cards heatmap and footer. Files: internal/tui/stats_boot_test.go. Depends on: usp-04. Verification: go test ./internal/tui -run TestStatsBoot -count=1
  Requirements: REQ-US-002, REQ-US-003, REQ-US-004, REQ-US-005, REQ-US-006, REQ-US-007

Integral reviewer gates: gofmt -s -l ./internal/ocstats ./internal/tui; go vet ./...; go build ./...; go test ./internal/... -count=1; plus the manual TUI smoke documented in tasks.md.
