# Backup Safety Delta Specification

## ADDED Requirements

### REQ-BACKUP-001: Backup operations use trusted roots and IDs

Restore, delete, rename, and prune MUST accept a trusted backup root and a validated backup ID. Manifest `RootDir`, `OriginalPath`, and `SnapshotPath` fields MUST be treated as untrusted metadata. Snapshot paths MUST remain beneath the selected backup's files root, and original paths MUST remain beneath an allowlisted adapter or cortex-ia metadata root.

Primary oracle: `internal/backup` malicious-manifest tests.

#### Scenario: Valid backup selected by ID

- **Given** a backup under the trusted backup root with contained entries
- **When** it is loaded by ID
- **Then** the manifest is accepted and its canonical root is derived by the store

#### Scenario: Manifest moved to another backup ID

- **Given** a manifest whose embedded root or entry paths refer to another directory
- **When** it is loaded
- **Then** validation fails before any mutation

#### Scenario: External original or snapshot path

- **Given** a manifest containing a valid hash but an external original or snapshot path
- **When** restore, delete, or rename is requested
- **Then** the operation returns an error and external sentinels remain unchanged

Coverage: missing.

### REQ-BACKUP-002: Backups are verified before mutation

Every rollback or restore MUST validate structure, containment, regular-file type, and digest before its first mutation. A failed verification MUST prevent Apply. A rollback error MUST be joined with the original operation error and MUST NOT be reported as success.

Primary oracle: backup verification tests and pipeline fault-injection tests.

#### Scenario: Verified snapshot

- **Given** a complete snapshot with matching hashes
- **When** rollback begins
- **Then** verification completes before any target is changed

#### Scenario: Tampered snapshot

- **Given** one snapshot file whose bytes no longer match its digest
- **When** rollback is requested
- **Then** no target is changed and the verification error identifies the entry

#### Scenario: Restore fails after an operation error

- **Given** an install failure followed by a simulated restore failure
- **When** rollback completes
- **Then** the returned error preserves both failures and install status records `rollback-failed`

Coverage: partial; digest verification exists, transaction integration is missing.

### REQ-BACKUP-003: Backup IDs are collision resistant

Backup creation MUST use exclusive directory creation with a high-resolution UTC timestamp and random suffix, or an equivalent collision-resistant mechanism. It MUST never reuse an existing backup directory.

Primary oracle: concurrent backup creation tests with a fixed clock.

#### Scenario: Sequential creation

- **Given** two backups created within the same second
- **When** both complete
- **Then** each has a distinct ID and manifest root

#### Scenario: Concurrent creation

- **Given** multiple goroutines or processes creating backups concurrently
- **When** creation completes
- **Then** no snapshot or manifest is overwritten

#### Scenario: Repeated collisions

- **Given** an injected identifier source that collides repeatedly
- **When** retry capacity is exhausted
- **Then** creation fails without modifying an existing backup

Coverage: missing.
