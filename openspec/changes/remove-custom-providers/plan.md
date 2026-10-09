# Plan — Remove Custom-Provider Installation Feature (sdd-lite)

**Workflow**: sdd-lite (integrated) · **Spec plane**: hybrid · **Workload policy**: flexible
**Board**: remove-custom-providers (created by this plan; same-board DAG task-rem-001..task-rem-011)

## Intent & Non-Goals

The nan provider now ships native OpenCode support, making the cortex-ia custom-provider installer redundant. This change retires the entire custom-provider install surface — `internal/providermgr`, the `install.Service` provider transaction (twin reconciliation + `providerIdentityDigest`), the state v2 `Providers` family, and the TUI Providers screen — while preserving the generic install pipeline, state integrity, and nan model-catalog support. Docs and the CHANGELOG record the removal as a breaking change pointing to native OpenCode provider support as the replacement.

Non-goals:
- Do not remove nan model catalog support: `internal/modelmgr` (`nan_catalog.go`), `model doctor`, `model catalog`, the `--provider <id>` filter, and the models picker stay untouched.
- Do not introduce a state v3, a migration daemon, or an active scrub of existing state files (Decision D1).
- Do not rewrite archived OpenSpec history: `openspec/changes/archive/add-custom-providers-750a1208efa7/` stays unchanged.
- Do not damage the generic install machinery: `txn.go`, `rollback.go`, `backup`, `service.go`, `doctor.go`, `uninstall.go` (verified zero provider references) remain unchanged.
- Do not touch the web console or delegation projection (verified zero provider references).
- Do not reconcile the `mimo-v2.6-flash` static capability row in `modelmgr/nan_catalog.go` as part of this change (operator decision, deferred).
- Tests of the removed surface are deleted together with the surface, never weakened.

## Decisions

- **D1 — State migration: passive retirement, schema pinned at 2.** Retire the Providers family from the Go schema while pinning `MetadataSchemaV2 = 2`; no v3, no active scrub. Grounded in verified mechanics: (1) `LoadMetadataV2`/`LoadLockV2` unmarshal with plain `json.Unmarshal`, so unknown keys are tolerated and existing v2 documents carrying a `providers` array keep loading and validating after the Go field is removed; (2) the fail-closed invariant is preserved — schema_version stays 2 and unknown future versions stay rejected; (3) the stale `providers` key disappears on the next natural `commitStateV2` re-marshal — idempotent, non-destructive, zero user action; (4) a v3 would force migration paths, lock v3, provenance handling, and a wide test rewrite — a disproportionate blast radius for one optional field and a direct risk to already-migrated installs; (5) provider blocks previously written into `opencode.json(c)` remain valid native OpenCode config, and `doctor`/`rollback`/`uninstall` never read `Metadata.Providers` (verified). Not operator-significant (no data loss either way), so it ships as a recommendation proven by the REQ-REM-003 oracle.
- **D2 — Operator decision D1 (deferred): `mimo-v2.6-flash` static row.** With the file catalog retired, the known drift reduces to a missing static capability row in `modelmgr/nan_catalog.go`. Reconciliation requires ratified capability-matrix facts (context/quota/effort vocabulary) absent from this repository; inventing them violates the catalog provenance contract. Recommended: defer as an independent modelmgr follow-up.
- **D3 — Operator decision D2 (resolved in-plan): `custom-providers.md` disposition.** Retirement stub (recommended) versus deletion; the plan assumes the stub so the inbound link in `docs/operations/stats-and-ui.md` stays valid. Trivial to flip at review.
- **D4 — Review policy.** Mandatory independent reviewer: task-rem-001 (install pipeline boundary), task-rem-004 and task-rem-005 (state migration integrity), task-rem-007 (shared TUI navigation wiring). Auto-approval: pure deletions proven by compiler/suites (task-rem-002, task-rem-003, task-rem-006, task-rem-008) and docs (task-rem-009..011).

## Design

Blast radius (verified read-only): `internal/providermgr` (catalog.go, catalog_test.go, seed/nan.json) has exactly one consumer, `internal/install/provider.go`. `Metadata.Providers` is read only by `internal/install/provider.go`, its tests, and state tests. TUI wiring: `model.go` (`screenProviders`, `providersEntryIndex = 3`, hotkey `p`/`P`, `homeEntries` row, `providersCatalogMsg`/`providersResultMsg` routing, index constants incl. `uninstallEntryIndex = 9`), `actions.go` (`openProviders`/`updateProviders`), `views.go` (`homeDescriptions` row, `screenProviders` render case), `masked_input.go` (sole consumer is the screen), `navigation_test.go` (count oracle 11→10). Docs: six EN files plus `CHANGELOG.md`. Home entry indices are named constants, so renumbering is mechanical and guarded by the updated count oracle. Task DAG is defined in `tasks.md` (waves: 1 = install/providermgr code deletion; 2 = state family retirement; 3 = TUI removal; 4 = docs and release notes).

## Requirements

### Requirement: REQ-REM-001 Provider install transaction retired
The install service exposes no provider install, preview, or catalog API, no provider identity digest, and no twin reconciliation; the generic pipeline is untouched.

#### Scenario: Provider API symbols no longer compile
- **GIVEN** the repository
- **WHEN** `go build ./...` runs
- **THEN** no symbol `ProviderInstall`, `ProviderPreview`, `ProviderCatalog`, `providerIdentityDigest`, or `ErrProviderUnmanaged` exists

#### Scenario: Generic pipeline unchanged
- **GIVEN** the existing install and sync test suites
- **WHEN** the internal/install package suite runs
- **THEN** backup, apply, rollback, and state commit behavior is unchanged and every suite passes

#### Scenario: Provider tests removed with the surface
- **GIVEN** the install package directory
- **WHEN** inspected after the change
- **THEN** `provider.go`, `provider_test.go`, `provider_container_test.go`, and `provider_conflict_test.go` no longer exist

### Requirement: REQ-REM-002 providermgr package removed with out-of-scope surfaces preserved
The catalog seeding package disappears; nan model-catalog support, archived history, and the web/delegation surfaces are preserved.

#### Scenario: Package removed
- **GIVEN** the repository
- **WHEN** `go build ./...` runs
- **THEN** `internal/providermgr` no longer exists and no import of it remains

#### Scenario: No seed materialization
- **GIVEN** a fresh isolated state root
- **WHEN** any remaining install flow runs
- **THEN** no `providers/` directory or `nan.json` seed is materialized

#### Scenario: Out-of-scope surfaces preserved
- **GIVEN** `internal/modelmgr`, the model commands, the archived change `openspec/changes/archive/add-custom-providers-750a1208efa7/`, and the web/delegation packages
- **WHEN** the full build and test gates run
- **THEN** nan catalog support, `model doctor`, `model catalog`, and the `--provider` filter are unchanged, the archived change content is unchanged, and no new provider references appear in web/delegation

### Requirement: REQ-REM-003 State v2 Providers family retired with compatibility preserved
The Go family and validators are removed while the schema version stays pinned at 2 and existing documents keep loading; a dedicated oracle proves passive retirement.

#### Scenario: Existing v2 state with providers rows still loads
- **GIVEN** a v2 `state.json` containing a `providers` array
- **WHEN** `LoadMetadataV2` reads it
- **THEN** the document classifies as `PresenceV2` and validates successfully

#### Scenario: Next commit drops the stale key
- **GIVEN** that same state with a stale `providers` key
- **WHEN** a new state commit is written
- **THEN** the persisted document no longer contains the `providers` key and all remaining families are intact

#### Scenario: Fail-closed versioning preserved
- **GIVEN** a document declaring an unknown future `schema_version`
- **WHEN** it is loaded
- **THEN** it is classified malformed (fail closed) and `MetadataSchemaV2` remains `2` with no v3 constant introduced

### Requirement: REQ-REM-004 TUI surface removed and Home consistent
The Providers screen, its Home entry, hotkey, message routing, and orphaned masked input are removed; the Home menu stays consistent.

#### Scenario: Home menu consistency
- **GIVEN** the TUI Home menu
- **WHEN** it renders
- **THEN** it lists 10 entries with `homeEntries` and `homeDescriptions` equally sized and no "Install custom provider" row, and subsequent indices renumber (uninstall 9→8)

#### Scenario: Hotkey retired
- **GIVEN** the keys `p` or `P` pressed at Home
- **WHEN** the update loop handles them
- **THEN** no providers screen transition occurs

#### Scenario: Screen and oracles deleted together
- **GIVEN** `masked_input.go`, `providers_screen.go`, `providers_screen_test.go`, and `providers_home_test.go`
- **WHEN** the TUI suite runs
- **THEN** those files no longer exist and the full `internal/tui` suite passes without them

### Requirement: REQ-REM-005 Documentation and release notes reflect the retirement
Docs describe provider installation as native (nan/OpenCode) and the installer as removed; the CHANGELOG carries the breaking-change note.

#### Scenario: Retirement stub keeps links valid
- **GIVEN** `docs/getting-started/custom-providers.md`
- **WHEN** updated
- **THEN** it is a short retirement stub containing the marker "This feature was removed" and stating provider installation is native in OpenCode, keeping inbound links valid

#### Scenario: No live surface documented
- **GIVEN** `docs/operations/stats-and-ui.md`, `docs/architecture/codebase/dashboard.md`, `repository-map.md`, `reference-map.md`, and `sync-and-cloud.md`
- **WHEN** they are updated
- **THEN** no provider-installer surface (`providermgr`, Providers screen) is documented as live

#### Scenario: Breaking-change recorded
- **GIVEN** `CHANGELOG.md`
- **WHEN** updated
- **THEN** a new entry records the removal of the custom provider installer as a breaking change with native nan/OpenCode provider support as the replacement

## Tasks

Sequential implementation DAG with per-task verification, review policy, and wave structure is defined in `tasks.md`.
