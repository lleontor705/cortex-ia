# Spec Delta: delegation (auto-update-pipeline)

## ADDED Requirements

### Requirement: REQ-DP-001 — Pre-open schema-drift guard with actionable recovery
Before `delegation.OpenStore` opens the database for writes, it SHALL run a read-only pre-open probe (`mode=ro`) that reads `SELECT MAX(version) FROM schema_migrations` when that table exists. If the on-disk schema exceeds the maximum supported version (currently 15, `store.go:180`), the open SHALL fail closed BEFORE creating the write handle, returning a typed `SchemaDriftError{OnDisk, Supported}` whose message names the recovery path `cortex-ia update`. The guard SHALL treat a missing database file, missing table, or empty ledger as not-drifted (fresh install). The existing in-initialize defense (`store.go:176-182`) SHALL remain as a second line of defense.

#### Scenario: newer schema fails closed pre-open with recovery hint
- **GIVEN** `~/.cortex-ia/delegation.db` has `schema_migrations` MAX(version) = 16 while the binary supports 15
- **WHEN** `OpenStore` is called
- **THEN** it returns a `SchemaDriftError` mentioning both versions and `cortex-ia update` and no write connection is kept open by the caller

#### Scenario: fresh database is not drifted
- **GIVEN** the database file does not exist yet
- **WHEN** `OpenStore` is called
- **THEN** the guard passes and normal initialization proceeds

#### Scenario: missing ledger table is not drifted
- **GIVEN** an existing database file without `schema_migrations`
- **WHEN** the pre-open probe runs
- **THEN** the guard treats the schema as version 0 and does not fail

### Requirement: REQ-DP-002 — Global active-authority probe gates binary replacement
The delegation package SHALL expose a read-only probe that reports whether ANY active work authority exists in the store: unexpired claims, unexpired file leases, or tasks in `in_progress`. The probe SHALL be usable from a read-only store handle and SHALL be the single source of truth for the auto-update apply gate.

#### Scenario: active claim blocks apply
- **GIVEN** a task holds a claim whose `expires_at` is in the future
- **WHEN** the active-authority probe runs
- **THEN** it reports `true`

#### Scenario: expired claims do not block apply
- **GIVEN** all claims are expired and no task is `in_progress`
- **WHEN** the probe runs
- **THEN** it reports `false`

#### Scenario: missing database does not block apply
- **GIVEN** `delegation.db` does not exist
- **WHEN** the probe is invoked through the TUI gate path
- **THEN** it reports `false` without error
