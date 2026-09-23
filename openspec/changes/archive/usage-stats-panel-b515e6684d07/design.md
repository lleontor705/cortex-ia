# Design: usage-stats-panel

## Data Source (verified — Cortex observation #217)

- Path: `~/.local/share/opencode/opencode.db` (win32-verified; 12.5 GB, WAL, epoch-ms timestamps).
- `session_v2`: pre-aggregated `tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write`, JSON `model` (`{"id","providerID","variant"}`), `directory`, `time_created`/`time_updated`. Implementer MUST re-verify exact token column names with a read-only `PRAGMA table_info(session_v2)` probe against the live DB before coding; the receipt records the evidence.
- `session_message`: ~90K rows, indexed `time_created` — the density signal for heatmap intensity and peak hour.
- `session`/`project`: not consumed by this change.
- Read-only DSN: `file:<escaped-path>?mode=ro&_pragma=busy_timeout(10000)&_pragma=query_only(1)` via `modernc.org/sqlite`. Snapshot-copying 12.5 GB was rejected (I/O cost, disk usage); bounded aggregates over indexed columns were measured feasible by the diagnosis.

## Package `internal/ocstats`

### Public surface

```go
type Window int            // WindowAll, Window30d, Window7d
type DayBucket struct {    // one local calendar day
    Date  string           // "YYYY-MM-DD" local
    Sessions, Messages int
    Tokens int64
}
type ModelUsage struct { ModelID string; Sessions int; Tokens int64; SharePct float64 }
type Summary struct {
    Window Window; Sessions, Messages int; Tokens int64; ActiveDays int
    PeakHour int; PeakHourMessages int; FavoriteModel string
    FirstDay, LastDay string
}
type Report struct {
    DBPath string; Days []DayBucket; Hours map[string][]HourBucket; Models map[Window][]ModelUsage
    Summaries map[Window]Summary
}
func DefaultDBPath() (string, error)   // CORTEX_IA_OPENCODE_DB -> $HOME/.local/share/opencode/opencode.db
func LoadReport() (Report, error)      // open ro -> 4 bounded queries -> reduce in Go -> close
```

### Query plan (one load pass, then close)

1. `SELECT MIN(time_created), MAX(time_created), COUNT(*) FROM session_v2` — real data range + session count.
2. `SELECT date(time_created/1000,'unixepoch','localtime') d, COUNT(*), SUM(<token expr>) FROM session_v2 GROUP BY d ORDER BY d` — daily session/token buckets.
3. `SELECT date(time_created/1000,'unixepoch','localtime') d, COUNT(*) FROM session_message GROUP BY d ORDER BY d` — daily message buckets.
4. `SELECT date(...) d, strftime('%H', ...) h, COUNT(*) FROM session_message GROUP BY d, h` — per-day-hour histogram (≤ days×24 rows).
5. `SELECT json_extract(model,'$.id') m, date(...) d, COUNT(*), SUM(<token expr>) FROM session_v2 WHERE m IS NOT NULL GROUP BY m, d` — model×day (models × days, small).

Window reduction happens in Go by clipping `DayBucket`/`HourBucket`/`model×day` rows to the window start date. One load serves all three windows; **filter switching never reopens the DB**. Row counts are bounded by distinct days (~60 in verified history), keeping live-WAL pinning negligible.

### Semantics pinned

- Token total = input + output + reasoning + cache_read + cache_write.
- ActiveDays = distinct local dates with ≥1 session in window.
- Heatmap intensity = messages/day (denser signal than sessions; decision recorded).
- Peak hour = hour-of-day with most messages; tie → earliest hour.
- Model order: sessions DESC, tokens DESC, id ASC; favorite model = first row.
- Deterministic formatting: hand-rolled thousands grouping (`12,345`); no new imports (`humanize` stays indirect).

## TUI integration

### Seam for testability (enables disjoint file waves)

`statsState` is a **self-contained struct** in `internal/tui/stats_screen.go` owning all stats fields (report, path, err, loading, tab, window) plus its own key handling (`update(tea.KeyMsg)`) and rendering (`view(width int) string`). `model.go` only embeds `stats statsState`, adds the `screenStats` const, and dispatches to it. This keeps `stats_screen.go` compiling green before wiring lands and keeps usp-03/usp-04 file sets disjoint.

- Keys: `tab`/`left`/`right` switch Resumen/Modelos; `1`/`2`/`3` select Todo/30d/7d; `esc` → Home (cursor on stats entry); `q`/`ctrl+c` quit.
- Loader: `statsLoadCmd` (var-indirection seam like `cortexBinaryMissing`) returns `statsLoadedMsg{report, dbPath, err}`; `Init` batches it for boot; the Home menu entry re-dispatches a fresh load every time (bounded, cheap).
- Degradation: on typed error the view renders the Spanish message with the resolved path; no cards; esc works.

### Rendering (reuses house patterns)

- Styles from `internal/tui/styles` (`Primary/Secondary/Success/Muted`, view precedents in views.go); cards rendered with `lipgloss.JoinHorizontal` in a 3x2 grid; `truncate`/`clampScreen` reuse; header/footer reuse.
- Heatmap (`stats_heatmap.go`): `renderHeatmap(days []ocstats.DayBucket, s Summary) string` — Sunday-first week columns × 7 rows, 5 intensity levels ramping from `Muted` to `Success` family, month labels desired-not-mandatory (SHOULD). Todo clamps to the real data range; no 365-column grid.
- UI copy in Spanish (menu entry, card labels, tabs, filters, footer) matching the existing `viewWeb` precedent; artifacts and code comments stay English.

### Boot sequence

`newModel` sets `screen: screenStats` (loading) → `Init` returns `statsLoadCmd` → `statsLoadedMsg` fills the report and re-renders. A failed load renders the degraded state. Home entry selection re-dispatches the load. No polling, no persistent handle.

## Trade-offs

| Decision | Alternative | Rationale |
|---|---|---|
| Bucket queries over full history, window reduce in Go | Per-window SQL (3× queries) | One bounded load; WAL-safe; instant filter switches |
| Live read-only bounded queries | Snapshot-copy 12.5 GB | I/O cost and disk usage unacceptable; busy_timeout proven sufficient |
| `statsState` self-contained struct | Fields scattered on model | Keeps implementation waves disjoint; screen testable without wiring |
| Spanish UI copy | English copy | Feature intent and existing viewWeb precedent are Spanish |
| Intensity = messages/day | sessions/day | ~90K messages vs ~hundreds of sessions gives a usable gradient |

## Risks

- **Schema drift** in OpenCode's DB: mitigated by the PRAGMA verification receipt step and fixture-anchored tests; typed degradation covers runtime drift.
- **Local timezone assumptions**: tests derive expectations from `time.Local` rather than hardcoding offsets.
- **Home renumbering ripple**: existing tests pin 7 screens and hotkeys 1-7; usp-04 owns the assertion maintenance in the same task that changes them (no intermediate red state).
