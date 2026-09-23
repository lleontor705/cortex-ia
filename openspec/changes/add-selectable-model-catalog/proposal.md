# Selectable Model Catalog — OpenCode v2-backed model/effort picker

## Why

The delivered model configurator (`add-model-configurator`) validates free-form `provider/model[#variant]` references but cannot tell users which models or effort variants exist: every `set` requires the user to already know the exact provider, model, and variant identifiers. The product owner requires the configurator to be SELECTABLE, leveraging OpenCode v2's own surfaces to enumerate models and effort variants, with free-form input demoted to an explicit fallback. OpenCode v2 exposes exactly this data: its background daemon HTTP API (`GET /api/model`, documented default base `http://localhost:4096`, published OpenAPI) returns models ordered by release date with per-model `variants` (`Model.Variant {id, settings?, headers?, body?}`) and an `enabled` flag, and the `opencode2 models` text output exposes `#variant` tokens and bare `provider/model` identifiers.

## What Changes

- New `internal/modelmgr` catalog module with a typed acquisition result: `CatalogEntry {Provider, Model, Variants []string}` (Model may itself contain `/`, e.g. `openrouter/anthropic/claude-sonnet-4.5`) and `Catalog {Entries, Truncated, Source ∈ {daemon-api, opencode2-models, none}}`, acquired through three tiers tried in order — daemon HTTP API (strict JSON decode, short timeout, `enabled == true` filter, variants from `Variant.id` only), then lenient parsing of `opencode2 models` text through a shared `ParseModelCatalog` parser (reusing `normalizeModelField`/`ParseModelRef`), then `Catalog{Source: "none"}`. Both transport seams (HTTP round-tripper, `CommandRunner`) are injectable; tests are fully synthetic.
- Bounded presentation: 512-entry cap with `Truncated=true` disclosure; deterministic ordering (providers ascending, models ascending within a provider, variants deduplicated and ascending).
- `model doctor` refactor onto the shared parser: findings stay byte-identical except a deliberate fix of the latent past-cap undercount (the cross-check echoed at most 16 references but reported `len(refs)` as the total; it now reports every parsed reference while still echoing at most 16).
- New `cortex-ia model catalog [--json] [--provider <id>]` CLI subcommand as a sibling of `list|get|set|unset|doctor`: provider-grouped text output with variants inline, source and truncation disclosure, sanitized receipts, stable `--json`; the usage line and unknown-action message gain `catalog`, and the AGENTS.md built-in surfaces note is updated.
- TUI models screen: a catalog-backed picker becomes the acquisition step of the Input phase — provider/model selectable with type-to-filter and arrow navigation, variant/effort selectable from the entry's `Variants` (default when the entry has none), a load-once catalog cache in `modelsState` fetched through an async `tea.Cmd` reusing the existing spinner — with free-form input preserved as an explicit fallback when `Source == "none"` or the user opts out, and the preview → confirm → service-seam persistence flow unchanged.

## Scope

Read-only catalog acquisition from OpenCode v2's own local surfaces (daemon HTTP API and `opencode2 models` text) plus CLI/TUI presentation and selection. No config mutation semantics change: `set`/`unset`/`list`/`get`/`doctor` behavior is untouched apart from the doctor's parser reuse and count fix, and persistence still flows exclusively through the transactional install service.

## Non-Goals

- No allow-list enforcement: a typed or selected reference absent from the catalog remains settable — the catalog is an acquisition aid, never a gate (prevents validation drift when OpenCode adds models faster than the catalog refreshes).
- No catalog-gated changes to ownership records, metadata schema, lock documents, or migrations (purely additive change).
- No web-console surface and no new TUI screens beyond the existing models screen.
- No network access beyond the local daemon base URL and the local `opencode2` binary: no remote registries, telemetry, or update checks.
- No persisted catalog cache (the TUI cache is in-memory, load-once per session).
- No exposure of variant `settings`, `headers`, or `body` anywhere — identifier-only disclosure (`Variant.id`), because those members can carry secrets.
- No new flags beyond `--json` and `--provider`; no flag or option beginning with the retired `--model` prefix.
- No attempt to reconcile catalog contents with recorded managed entries or markdown frontmatter pins.

## Impact

- **Code**: new `internal/modelmgr/catalog.go` + `catalog_test.go`; edits to `internal/modelmgr/doctor.go` + `doctor_test.go` (shared parser + count fix); `internal/app/model.go` + `internal/app/app.go` + `internal/app/model_test.go` (catalog subcommand); `AGENTS.md` (built-in surfaces note); `internal/tui/models_screen.go` + `models_screen_test.go` (picker).
- **Specs**: `model-command` capability delta (REQ-MODEL-009 through REQ-MODEL-012).
- **Compatibility**: purely additive; free-form references remain valid everywhere; doctor findings are unchanged for catalogs of ≤ 16 references; no metadata, lock, or template changes.
