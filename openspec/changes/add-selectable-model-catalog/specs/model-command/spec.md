# Model Command — spec delta (selectable catalog)

## ADDED Requirements

### Requirement: REQ-MODEL-009 — Catalog acquisition tiers with graceful degradation
The manager SHALL acquire a selectable model catalog through three tiers tried in order: (1) `daemon-api` — an HTTP `GET /api/model` request against the OpenCode v2 background daemon at the documented default base `http://localhost:4096` with a short timeout, strict JSON decoding, filtering entries to `enabled == true`, and mapping each model's effort variants exclusively from `variants[].id`; (2) `opencode2-models` — lenient parsing of the `opencode2 models` text output through the shared catalog parser; (3) `none` — when both preceding tiers fail, the result SHALL be `Catalog{Source: "none"}` with no entries. Every acquisition failure mode (non-2xx status, timeout, unreachable daemon, malformed or unparseable payload, absent binary) SHALL degrade to the next tier instead of surfacing an error, and catalog acquisition SHALL never block, mutate, or fail any other modelmgr, CLI, or TUI behavior. The HTTP transport and command runner SHALL be injectable seams so all persistent tests are fully synthetic; no persistent test SHALL require the daemon, the network, or the `opencode2` binary.

#### Scenario: daemon tier returns enabled models with identifier-only variants
- **GIVEN** a daemon responding to `GET /api/model` with models where some are `enabled == true` and carry `variants` with `id`, `settings`, `headers`, and `body` members
- **WHEN** the catalog is acquired
- **THEN** `Source` is `daemon-api`, only enabled models appear, every variant list contains exactly the `variants[].id` values, and no `settings`, `headers`, or `body` content is retained anywhere in the result

#### Scenario: daemon failure falls through to the text tier
- **GIVEN** the daemon answers with 503 (or 401), times out, or is unreachable, and `opencode2 models` prints parseable text
- **WHEN** the catalog is acquired
- **THEN** `Source` is `opencode2-models` and the entries come from the parsed text

#### Scenario: both tiers unavailable degrade to none
- **GIVEN** no reachable daemon and no parseable `opencode2 models` output
- **WHEN** the catalog is acquired
- **THEN** the result is `Catalog{Source: "none"}` with zero entries, no error is returned, and free-form reference input remains fully functional

#### Scenario: malformed daemon payload never leaks content
- **GIVEN** a daemon response that is not valid JSON
- **WHEN** the catalog is acquired
- **THEN** the daemon tier is abandoned, the text tier runs, and nothing from the malformed payload is exposed in any tier result

### Requirement: REQ-MODEL-010 — Catalog data model, shared parser, deterministic bounded presentation
The catalog SHALL be typed as `CatalogEntry {Provider string, Model string, Variants []string}` — where `Provider` ends at the first `/`, `Model` may itself contain `/` (for example `openrouter/anthropic/claude-sonnet-4.5`), and `Variants` holds effort identifiers — and `Catalog {Entries []CatalogEntry, Truncated bool, Source string ∈ {daemon-api, opencode2-models, none}}`. The text tier and the `model doctor` cross-check SHALL share one lenient parser (`ParseModelCatalog`) that reuses the existing `normalizeModelField` and `ParseModelRef` helpers: tokens containing `#variant` SHALL extend the matching entry's `Variants` (deduplicated), bare `provider/model` tokens SHALL create variant-less entries, and tokens with the same provider/model SHALL merge into one entry. Entries SHALL be deduplicated and capped at 512; when a source yields more, exactly 512 SHALL be retained and `Truncated` SHALL be true. Ordering SHALL be deterministic regardless of source order: providers ascending, models ascending within a provider, variants deduplicated and ascending. The doctor cross-check finding SHALL remain byte-identical for catalogs of at most 16 references, except that the reported reference count SHALL now reflect every parsed reference while the echoed list remains capped at 16, deliberately resolving the prior past-cap undercount.

#### Scenario: text parser merges variants and keeps bare entries variant-less
- **GIVEN** `opencode2 models` output containing `anthropic/claude-sonnet-4-5#high`, `anthropic/claude-sonnet-4-5#low`, and `anthropic/claude-haiku-4-5`
- **WHEN** the shared parser runs
- **THEN** the result contains one entry `anthropic/claude-sonnet-4-5` with variants `high` and `low` and one variant-less entry `anthropic/claude-haiku-4-5`, deterministically ordered

#### Scenario: truncation is disclosed
- **GIVEN** a source yielding more than 512 unique entries
- **WHEN** the catalog is built
- **THEN** exactly 512 entries are retained, `Truncated` is true, and every renderer discloses the truncation

#### Scenario: doctor reports the true count beyond the echo cap
- **GIVEN** `opencode2 models` output normalizing to 20 unique valid references
- **WHEN** `model doctor` runs the cross-check
- **THEN** the finding reports 20 references while echoing at most 16, and all other doctor findings are unchanged

#### Scenario: selector grammar composes catalog selections
- **GIVEN** a catalog entry with provider `openrouter`, model `anthropic/claude-sonnet-4.5`, and variant `xhigh`
- **WHEN** a selection is composed into a model reference
- **THEN** the provider ends at the first `/`, the model is `anthropic/claude-sonnet-4.5`, and the compact reference with effort is `openrouter/anthropic/claude-sonnet-4.5#xhigh` (case-sensitive matching throughout)

### Requirement: REQ-MODEL-011 — `model catalog` CLI subcommand with sanitized receipts
The CLI SHALL provide `cortex-ia model catalog [--json] [--provider <id>]` registered as a sibling of `list|get|set|unset|doctor`. Text output SHALL group entries by provider, render each entry's model with its variants inline, and disclose the acquisition source and any truncation. `--provider <id>` SHALL filter entries by exact, case-sensitive provider match. `--json` SHALL emit the stable machine-readable Catalog document. Output SHALL be sanitized: only provider, model, and variant identifiers SHALL appear — never variant `settings`, `headers`, or `body` values, never secrets. The usage line and the unknown-action message SHALL include `catalog`, the grammar SHALL add no flag other than `--json` and `--provider`, and the AGENTS.md built-in surfaces note SHALL document the catalog subcommand.

#### Scenario: grouped text output with disclosures
- **GIVEN** a catalog with providers `anthropic` and `openrouter`
- **WHEN** `cortex-ia model catalog` runs
- **THEN** entries render grouped under each provider with variants inline, and the output states the acquisition source plus a truncation notice when `Truncated` is true

#### Scenario: provider filter is exact
- **GIVEN** a catalog with providers `anthropic` and `openrouter`
- **WHEN** `cortex-ia model catalog --provider openrouter` runs
- **THEN** only `openrouter` entries appear and `anthropic` entries are absent

#### Scenario: JSON receipt is stable and sanitized
- **GIVEN** a daemon variant carrying `settings`, `headers`, or `body` content
- **WHEN** `cortex-ia model catalog --json` runs
- **THEN** the JSON document matches the Catalog shape (entries, truncated, source) and contains no settings, headers, body, or secret material

#### Scenario: unknown action names the catalog subcommand
- **GIVEN** the `model` command surface
- **WHEN** the user runs `cortex-ia model frobnicate`
- **THEN** the error names the valid subcommands including `catalog`

#### Scenario: AGENTS.md surfaces note updated
- **GIVEN** the repository AGENTS.md
- **WHEN** the built-in surfaces note is read after this change
- **THEN** it documents the `model catalog` subcommand with its scope, and no other AGENTS.md section is reworded

### Requirement: REQ-MODEL-012 — TUI catalog-backed picker with free-form fallback
The models screen SHALL make provider/model selection catalog-backed as the acquisition step of its Input phase: a load-once catalog cache in `modelsState` SHALL be fetched through an injectable async `tea.Cmd` seam (rendering the existing spinner while loading), a picker SHALL list provider/model entries with type-to-filter and arrow navigation, and variant/effort SHALL be selectable from the chosen entry's `Variants` (the default applies when the entry has none). Free-form input SHALL remain available as an explicit fallback whenever the catalog `Source` is `none` or the user opts out of the picker, and the screen SHALL disclose an unavailable or truncated catalog. The composed reference SHALL flow through the unchanged preview (dry-run) → confirm → service-seam persistence path; no TUI code path SHALL write configuration directly.

#### Scenario: picker composes model and variant
- **GIVEN** a catalog containing `anthropic/claude-sonnet-4-5` with variants `high` and `low`
- **WHEN** the user filters, selects the entry, and chooses variant `high`
- **THEN** the Input phase carries the compact reference `anthropic/claude-sonnet-4-5#high` into the unchanged preview → confirm flow

#### Scenario: empty catalog opens free-form directly
- **GIVEN** an acquired catalog with `Source == "none"`
- **WHEN** the user enters the Input phase
- **THEN** the free-form input opens directly with a notice that no catalog is available, preserving today's behavior

#### Scenario: explicit opt-out preserves free-form
- **GIVEN** a populated catalog
- **WHEN** the user opts out of the picker
- **THEN** the free-form input opens and accepts any shape-valid reference exactly as before

#### Scenario: load-once cache with spinner
- **GIVEN** the user enters the Input phase twice in one session
- **WHEN** the catalog was already fetched
- **THEN** the cached catalog is reused without a second fetch, and the first fetch renders the existing spinner while in flight

#### Scenario: persistence path is unchanged
- **GIVEN** a picker-composed reference
- **WHEN** the user previews and confirms
- **THEN** persistence flows exclusively through the existing install service seams with a dry-run preview first, and typed conflicts render in-screen without direct config writes
