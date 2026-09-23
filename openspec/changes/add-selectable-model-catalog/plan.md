# Selectable Model Catalog — integrated plan (sdd-lite)

Change: `add-selectable-model-catalog` · Board: `model-configurator` · Spec plane: hybrid · Workload policy: flexible

## Intent

Make the delivered model configurator SELECTABLE: the user picks provider/model and effort (variant) from a catalog acquired from OpenCode v2's own surfaces — the background daemon HTTP API (`GET /api/model`, documented default base `http://localhost:4096`) preferred, `opencode2 models` text as fallback, free-form input preserved as the last resort (`Source: "none"` or explicit opt-out). The catalog is an acquisition aid, never an allow-list, and never fails closed.

Non-goals: allow-list validation of references; persisted catalog caches; web-console surfaces; network access beyond the local daemon/`opencode2` binary; exposure of variant `settings`/`headers`/`body`; `--model*` flags; changes to set/unset semantics, ownership records, or schema.

## Requirements

Authoritative deltas with RFC 2119 keywords and Given/When/Then scenarios live in `specs/model-command/spec.md`:

- REQ-MODEL-009 Catalog acquisition tiers with graceful degradation (daemon-api → opencode2-models → none; enabled filter; Variant.id only; injectable seams; fully synthetic tests)
- REQ-MODEL-010 Catalog data model, shared lenient parser, deterministic bounded presentation (CatalogEntry/Catalog, ParseModelCatalog reuse, 512 cap + Truncated, deterministic sort, doctor count fix)
- REQ-MODEL-011 `model catalog` CLI subcommand (--json/--provider, grouped sanitized receipts, source/truncation disclosure, usage + unknown-action + AGENTS.md note)
- REQ-MODEL-012 TUI catalog-backed picker with free-form fallback (load-once cache via async seam + spinner, type-to-filter + arrows, variant selection, unchanged preview→confirm→service persistence)

## Design

Concise architecture (full rationale in `design.md`):

- **Catalog module** (`internal/modelmgr/catalog.go`): typed `CatalogEntry`/`Catalog`; daemon client (strict JSON decode into a forward-compatible struct, 2s timeout, `enabled == true`, variants from `Variant.id` only); shared lenient text parser `ParseModelCatalog` reusing `normalizeModelField`/`ParseModelRef` (`#variant` extends Variants, bare tokens variant-less, same provider/model merges); 512-entry cap with `Truncated`; deterministic ordering; `Catalog(...)` tier orchestration behind injectable HTTP round-tripper + `CommandRunner` seams.
- **Doctor refactor**: `checkOpencode2Models` consumes the shared parser; findings byte-identical ≤ 16 refs; the reported count now reflects all parsed references while the echo stays capped at 16 (deliberate fix of the latent undercount at `internal/modelmgr/doctor.go:461-469`/`:366-367`).
- **CLI** (`internal/app/model.go` + `app.go`): `model catalog [--json] [--provider <id>]` sibling of list|get|set|unset|doctor; provider-grouped text with variants inline; sanitized; stable JSON; usage + unknown-action updated; AGENTS.md surfaces note gains catalog.
- **TUI** (`internal/tui/models_screen.go`): picker as Input-phase acquisition step behind a new `modelsLoadCatalog` seam; load-once cache in `modelsState` via async `tea.Cmd` + existing spinner; type-to-filter + arrows; variant selection from `Variants` (default when none); free-form fallback when `Source == "none"` or opt-out; preview→confirm→service path unchanged.

## Tasks

DAG on board `model-configurator` (wave 1: model-012; wave 2: model-013 ∥ model-014 — disjoint files). Full contracts in `tasks.md`:

- model-012 [modelmgr] catalog module (daemon client + shared text parser) and doctor refactor — internal/modelmgr/catalog.go, catalog_test.go, doctor.go, doctor_test.go — deps: none
- model-013 [app] `model catalog` subcommand, receipts, usage/unknown-action, AGENTS.md note — internal/app/model.go, app.go, model_test.go, AGENTS.md — deps: model-012
- model-014 [tui] catalog-backed picker with free-form fallback — internal/tui/models_screen.go, models_screen_test.go — deps: model-012

**Verification strategy**: each task gates on its raw `go test` command (see tasks.md); wave 2 files are disjoint (app+AGENTS.md vs tui); persistent tests stay synthetic (no daemon, no network, no `opencode2` binary — live capture was guard-blocked during investigation); before archive the full local gate runs (`gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`) and the reviewer performs AST delta ingestion plus a cycle check before any PASS.
