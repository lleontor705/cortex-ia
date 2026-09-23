# Usage Stats Panel — spec delta

## ADDED Requirements

### Requirement: REQ-US-001 — Read-only OpenCode stats data source
The system SHALL resolve the OpenCode usage database path deterministically: the `CORTEX_IA_OPENCODE_DB` environment variable takes precedence when set to a non-empty value; otherwise the default path is `<user home>/.local/share/opencode/opencode.db`. The stats package SHALL open it strictly read-only through `modernc.org/sqlite` with DSN `file:<path>?mode=ro&_pragma=busy_timeout(10000)&_pragma=query_only(1)`, SHALL close the handle before the load result is consumed, and SHALL fail with typed errors (not-found vs unreadable) whose messages name the resolved path. It SHALL never write to, migrate, lock beyond busy-timeout, or snapshot-copy the OpenCode database.

#### Scenario: default path resolves to the verified location
- **GIVEN** the environment variable CORTEX_IA_OPENCODE_DB is unset and the user home is C:/Users/usrLuisLeon
- **WHEN** the stats package resolves the database path
- **THEN** the resolved path is C:/Users/usrLuisLeon/.local/share/opencode/opencode.db

#### Scenario: environment override wins
- **GIVEN** CORTEX_IA_OPENCODE_DB points to a synthetic SQLite fixture
- **WHEN** the loader runs
- **THEN** it reads the fixture and never touches the default path

#### Scenario: missing database degrades without crashing
- **GIVEN** the resolved path does not exist
- **WHEN** the loader runs
- **THEN** it returns a typed not-found error whose message contains the resolved path, and no TUI state is mutated

#### Scenario: live WAL database stays readable and unpinned
- **GIVEN** the OpenCode application is running and its database is in WAL mode
- **WHEN** the loader executes its bounded aggregate queries
- **THEN** all queries succeed read-only and the connection is closed before the report is returned

### Requirement: REQ-US-002 — Boot auto-show and menu visibility
The TUI SHALL open on the `screenStats` screen when cortex-ia is launched with zero arguments, dispatching the stats load asynchronously so the screen renders a loading state first. The Home menu SHALL gain one entry labeled "Estadísticas de uso" (inserted after "Agent Studio (Create Sub-agent)", shifting Doctor / Recovery, Uninstall and Quit), selectable by cursor, Enter, and its numeric hotkey; the Home footer key hint SHALL reflect 1-8. Pressing `esc` on the stats screen SHALL return to Home with the cursor resting on the stats entry; pressing `q` SHALL quit.

#### Scenario: TUI boots into the stats summary
- **GIVEN** cortex-ia is started with no arguments
- **WHEN** the Bubble Tea program initializes
- **THEN** the initial screen is screenStats in loading state, the load command is dispatched, and the screen renders the summary once the load message arrives

#### Scenario: stats entry is reachable from Home
- **GIVEN** the user is on the Home screen
- **WHEN** the user presses the stats entry hotkey or moves the cursor to "Estadísticas de uso" and presses Enter
- **THEN** the TUI switches to screenStats and re-dispatches a fresh load

#### Scenario: escape always returns to Home
- **GIVEN** the stats screen is visible in any state (loading, loaded, or degraded)
- **WHEN** the user presses esc
- **THEN** the screen becomes screenHome with the cursor on the stats entry

### Requirement: REQ-US-003 — Resumen metric cards
The Resumen tab SHALL render six metric cards in a 3x2 grid labeled (in Spanish): Sesiones, Mensajes, Tokens totales, Días activos, Hora pico, and Modelo favorito. Values SHALL derive from the current window's report: sessions = count of `session_v2` rows; messages = count of `session_message` rows; tokens total = sum of input+output+reasoning+cache_read+cache_write; active days = distinct local dates with at least one session; peak hour = hour-of-day with most messages rendered in local time; favorite model = top model by the REQ-US-008 ordering. Token and count values SHALL use grouped thousands separators without new dependencies.

#### Scenario: cards render exact window values
- **GIVEN** a synthetic database with 3 sessions, 14 messages, 12,345 tokens across 2 distinct days, most messages at 10:00 local, and model X dominating
- **WHEN** the Resumen tab renders for the Todo window
- **THEN** the six cards show exactly those values with grouped separators and the peak hour in local time

#### Scenario: loading state is visible before data arrives
- **GIVEN** the stats load command is still in flight
- **WHEN** the stats screen renders
- **THEN** it shows a loading indicator and no fabricated zeros presented as final values

#### Scenario: degraded state names the path
- **GIVEN** the load finished with a typed error
- **WHEN** the stats screen renders
- **THEN** it shows "No se encontró la base de OpenCode en <ruta>" (or the unreadable variant), the six cards are absent, and esc remains functional

### Requirement: REQ-US-004 — Window filters Todo, 30d, 7d
The stats screen SHALL offer three window filters selectable with keys `1`, `2`, `3`: Todo, 30d, and 7d. The default filter on entry SHALL be Todo. 30d and 7d SHALL cover the trailing N local calendar days ending today. Todo SHALL be bounded by the real data range (earliest to latest record), never a hardcoded 365-day span. Switching filters SHALL re-render purely from the already-loaded report without reopening the database.

#### Scenario: default window is Todo
- **GIVEN** the stats screen has loaded
- **WHEN** it first renders
- **THEN** the Todo filter is active and the visible range equals the real first-to-last data range

#### Scenario: 7d window clips to trailing week
- **GIVEN** data exists on 40 distinct days
- **WHEN** the user selects 7d
- **THEN** only local dates within the last 7 days contribute to every card, the heatmap, the ranking, and the footer

#### Scenario: filter switch does not re-query the database
- **GIVEN** a loaded report
- **WHEN** the user presses 1, 2 or 3
- **THEN** the render derives from in-memory report state and no database connection is opened

### Requirement: REQ-US-005 — GitHub-style activity heatmap
The Resumen tab SHALL render a heatmap below the cards: one column per week, 7 rows per weekday (Sunday-first), one cell per local calendar day. Cell intensity SHALL have 5 levels derived from the day's message count relative to the window maximum (level 0 for zero activity). The heatmap SHALL cover exactly the active window's range: for Todo the real data range, for 30d/7d the trailing window; the grid SHALL NOT hardcode 365 columns. Month labels above the grid and weekday markers are desired but not mandatory.

#### Scenario: intensity scales within the window
- **GIVEN** a window whose busiest day has 40 messages
- **WHEN** the heatmap renders
- **THEN** a day with 0 messages shows level 0, a day with 40 shows the maximum level, and intermediate days scale monotonically

#### Scenario: Todo grid spans the real range
- **GIVEN** the earliest record is 2026-08-01 and the latest is today
- **WHEN** the Todo heatmap renders
- **THEN** the grid spans only the weeks from that first day through today, not a fixed 365-column grid

#### Scenario: 7d heatmap is a single week column
- **GIVEN** the 7d filter is active
- **WHEN** the heatmap renders
- **THEN** the grid contains exactly the trailing 7 days placed on their weekday rows

### Requirement: REQ-US-006 — Modelos ranking tab
The Modelos tab SHALL list one row per distinct model id extracted with `json_extract(model,'$.id')` from `session_v2`, showing sessions, total tokens (same sum as REQ-US-003), and share percentage of the window's tokens rounded to one decimal. Rows SHALL be ordered by sessions descending, then tokens descending, then model id ascending. Models with a NULL id SHALL be excluded.

#### Scenario: ranking aggregates and orders deterministically
- **GIVEN** a synthetic database where model A has 2 sessions and 300 tokens, model B has 2 sessions and 700 tokens
- **WHEN** the Modelos tab renders for Todo
- **THEN** B precedes A, each row shows correct sessions/tokens/share, and shares sum to approximately 100%

#### Scenario: variants merge under the same model id
- **GIVEN** session rows whose model JSON differs only in $.variant but share $.id
- **WHEN** the ranking is computed
- **THEN** those rows aggregate under a single model id

#### Scenario: null model ids are excluded from the ranking
- **GIVEN** session rows whose model column is NULL or lacks $.id
- **WHEN** the ranking is computed
- **THEN** those rows produce no ranking entry and do not distort other shares

### Requirement: REQ-US-007 — Playful Gatsby footer
The Resumen tab footer SHALL render a playful comparison computed from the current window's total tokens using the equivalence 50,000 tokens ≈ 1 novela (The Great Gatsby). When the ratio is ≥ 1 it SHALL read "Nx más tokens que The Great Gatsby" with one decimal; when below 1 it SHALL express the percentage of one Gatsby. The line SHALL never crash or render NaN for empty windows (0 tokens renders the below-1 branch).

#### Scenario: large totals exceed one Gatsby
- **GIVEN** the window total is 150,000 tokens
- **WHEN** the footer renders
- **THEN** it reads "3.0x más tokens que The Great Gatsby"

#### Scenario: small or empty totals stay below one Gatsby
- **GIVEN** the window total is 5,000 tokens or 0 tokens
- **WHEN** the footer renders
- **THEN** it expresses the fraction/percentage of one Gatsby without NaN or division errors

#### Scenario: footer follows the active window
- **GIVEN** the Todo window reports 150,000 tokens and the 7d window reports 10,000 tokens
- **WHEN** the user switches from Todo to 7d
- **THEN** the footer recomputes from 10,000 tokens and renders the below-1 branch

### Requirement: REQ-US-008 — Aggregation determinism and timezone handling
All aggregations SHALL treat `time_created` as epoch milliseconds and bucket by the process-local timezone for daily, weekday, and hourly groupings. Tie-breaking SHALL be deterministic: peak hour resolves to the earliest hour on equal counts; model ordering follows REQ-US-006; favorite model is the first row of that ordering. The aggregation oracle SHALL run against synthetic temporary databases mirroring the verified schema (token column names verified against the live DB via a read-only PRAGMA probe before implementation, recorded in the task receipt).

#### Scenario: epoch milliseconds convert to local buckets
- **GIVEN** fixture sessions recorded at known UTC instants
- **WHEN** buckets are computed
- **THEN** the daily/weekday/hour placement matches the expected local-time conversion derived from time.Local in the test

#### Scenario: ties resolve deterministically
- **GIVEN** two hours with equal message counts and two models with equal sessions and tokens
- **WHEN** the report is computed
- **THEN** the peak hour is the earlier hour and the favorite model is the lexicographically smaller id

#### Scenario: fixture schema matches the verified source
- **GIVEN** the ocstats aggregation oracle
- **WHEN** it builds its synthetic databases
- **THEN** the session_v2 token column names match the names verified in the implementation receipt (tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write per observation #217), and the fixture mirrors session_v2/session_message minimally
