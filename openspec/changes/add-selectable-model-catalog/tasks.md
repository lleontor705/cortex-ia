# Task DAG

Board: `model-configurator` (continuation of the delivered `add-model-configurator`; this change adds `model-012..014` to the same board). Waves: `model-012` → (`model-013` ∥ `model-014`; disjoint files). Workload policy `flexible`: source ≤ 700 LOC Go per task, persistent tests ≤ 1200 LOC per task, dedicated test files ≤ 250 LOC each. Persistent tests stay fully synthetic (fixtures only — no daemon, no network, no `opencode2` binary; live capture was guard-blocked during investigation). Deeper daemon/HTTP matrices run as ephemeral smokes per AGENTS.md testing scope.

- [ ] model-012 Add the modelmgr catalog module (daemon client, shared text parser, tier orchestration) and refactor doctor onto it
  Requirements: REQ-MODEL-009, REQ-MODEL-010
  Dependencies: none
  Objective: New internal/modelmgr/catalog.go defines CatalogEntry {Provider, Model, Variants []string} (provider ends at the first '/', Model may contain further segments such as openrouter/anthropic/claude-sonnet-4.5) and Catalog {Entries, Truncated, Source} with Source in {daemon-api, opencode2-models, none}, orchestrated by Catalog(...) through three tiers: (1) daemon-api — HTTP GET /api/model against the documented default base http://localhost:4096 with a short 2s timeout, strict JSON decoding into a forward-compatible struct that ignores unknown members, filtering to enabled == true, and mapping effort variants exclusively from variants[].id (settings/headers/body are never retained — they can carry secrets); any non-2xx status, timeout, unreachable daemon, or malformed payload abandons the tier; (2) opencode2-models — the shared lenient parser ParseModelCatalog reusing normalizeModelField and ParseModelRef: '#variant' tokens extend the matching entry's Variants (deduplicated), bare provider/model tokens create variant-less entries, same provider/model tokens merge; (3) none — Catalog{Source: "none"} with no entries and no error. Entries are deduplicated, capped at 512 (Truncated=true beyond), and ordered deterministically: providers ascending, models ascending within a provider, variants deduplicated ascending. Both transports are injectable seams (HTTP round-tripper + the existing modelmgr.CommandRunner) so tests are fully synthetic. The same task refactors the doctor opencode2 cross-check (checkOpencode2Models) onto ParseModelCatalog for its refs projection: findings stay byte-identical for at most 16 references, and the reported count now reflects every parsed reference while the echo list stays capped at opencode2ModelsRefLimit — the deliberate fix of the latent undercount where refs stop appending past the cap but len(refs) is reported (internal/modelmgr/doctor.go:461-469, finding at :366-367). Tests use synthetic fixtures in dedicated files ≤ 250 LOC each.
  Acceptance:
    - The daemon tier returns only enabled models with variant identifiers from variants[].id and never retains settings/headers/body; 503/401/timeout/unreachable/malformed payloads fall through to the text tier
    - ParseModelCatalog merges #variant tokens into Variants, keeps bare provider/model tokens variant-less, merges duplicate provider/model tokens, deduplicates, and orders deterministically
    - More than 512 unique entries yield exactly 512 with Truncated=true
    - Doctor findings are byte-identical for at most 16 references and report the true count for more while echoing at most 16; all other doctor checks unchanged
    - Both tiers failing yields Catalog{Source: "none"} with zero entries and no error
    - All tests are synthetic (no daemon, no network, no opencode2 binary)
  Allowed files: internal/modelmgr/catalog.go, internal/modelmgr/catalog_test.go, internal/modelmgr/doctor.go, internal/modelmgr/doctor_test.go
  Verification: go test ./internal/modelmgr/... -count=1

- [ ] model-013 Add the cortex-ia model catalog subcommand with grouped receipts and the AGENTS.md surfaces note
  Requirements: REQ-MODEL-011
  Dependencies: model-012
  Objective: internal/app/model.go registers catalog as a sibling action of list|get|set|unset|doctor in runModel: parse 'model catalog [--json] [--provider <id>]' following the parseModelList discipline (unknown arguments fail with usage; no --model* flag anywhere), acquire the catalog through the modelmgr tier orchestration (HTTP client for the daemon tier wired like modelDoctorRunCommand; command runner reuses the existing seam), filter by exact case-sensitive provider match when --provider is given, and render text receipts grouped by provider with each entry's model and variants inline plus source and truncation disclosure, or the stable Catalog JSON document for --json. Output is sanitized to provider/model/variant identifiers only — never variant settings/headers/body values, never secrets. modelUsage and the unknown-action message gain catalog. AGENTS.md's built-in surfaces note gains the catalog subcommand with a one-line scope statement; no other AGENTS.md section is reworded. model_test.go covers the parse matrix, grouped text shape, provider filter, JSON shape, sanitization, and the updated unknown-action message, respecting the modular-test policy (dedicated file ≤ 250 LOC).
  Acceptance:
    - catalog parses --json and --provider only; unknown arguments fail with the usage line
    - Text output groups entries by provider with variants inline and discloses source and truncation
    - --provider filters exactly and case-sensitively
    - Text and JSON receipts contain no settings/headers/body values or secrets
    - modelUsage and the unknown-action error list catalog
    - AGENTS.md documents the catalog subcommand with no other section reworded
  Allowed files: internal/app/model.go, internal/app/app.go, internal/app/model_test.go, AGENTS.md
  Verification: go test ./internal/app/... -count=1

- [ ] model-014 Add the catalog-backed model picker with free-form fallback to the TUI models screen
  Requirements: REQ-MODEL-012
  Dependencies: model-012
  Objective: internal/tui/models_screen.go makes Input-phase acquisition catalog-backed: a new package seam modelsLoadCatalog (mirroring the modelsLoadReport pattern at models_screen.go:60-82) fetches the modelmgr catalog through an async tea.Cmd; modelsState holds a load-once cache (fetched once per session, reused afterwards) and renders the existing loading/spinner view while the first fetch is in flight. Entering edit (beginEdit) routes to the picker when the catalog Source is not none: type-to-filter narrows provider/model entries, up/down navigate, enter selects, and the chosen entry's Variants drive variant/effort selection (the default applies when the entry has none); the composed provider/model#variant reference enters the unchanged ParseDesired → dry-run preview → confirm → modelsSetRef/modelsUnsetRef service path. Free-form input is preserved as an explicit fallback: it opens directly with a disclosure notice when Source is none or the catalog is truncated, and a user opt-out from the picker always reaches it. No TUI code path writes configuration; typed conflicts keep rendering in-screen. models_screen_test.go drives the seams over temporary homes with synthetic catalogs (dedicated file ≤ 250 LOC). If the screen exceeds the flexible source budget, the seam/state/keybinding layer lands first and view polish follows as a stacked follow-up on the same files.
  Acceptance:
    - The picker filters as the user types and navigates with arrows; selection composes the compact reference with the chosen variant
    - Source none opens the free-form input directly with a notice; opt-out is always available
    - The catalog is fetched once per session and reused; the first fetch shows the existing spinner
    - Preview and confirm are unchanged: dry-run first, service-seam persistence, in-screen conflicts
    - No TUI path writes config files; all tests use injected seams and temporary homes
  Allowed files: internal/tui/models_screen.go, internal/tui/models_screen_test.go
  Verification: go test ./internal/tui/... -count=1
