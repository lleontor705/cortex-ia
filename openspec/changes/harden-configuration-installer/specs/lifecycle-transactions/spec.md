# Lifecycle Transaction Delta Specification

## ADDED Requirements

### REQ-INSTALL-001: Install is all-or-restored

Install MUST construct a complete write set, snapshot configuration plus state/lock/status, verify that snapshot, and stop each agent chain after its first failure. Any failure after snapshot creation MUST prevent dependent steps, wait for active writers, and restore the verified pre-install state.

Primary oracle: `internal/pipeline` multi-agent fault-injection tests.

#### Scenario: Successful multi-agent install

- **Given** valid configuration roots for multiple supported agents
- **When** all component steps, metadata writes, and post-validation succeed
- **Then** the new configuration, state, lock, and cleared status are committed

#### Scenario: Component failure

- **Given** a chain that fails after an earlier write
- **When** install handles the failure
- **Then** later steps in that chain do not run and all observable bytes and modes return to their pre-install values

#### Scenario: Metadata commit failure

- **Given** successful component writes followed by a state or lock write failure
- **When** install returns
- **Then** configuration and metadata are restored and the error identifies the failed commit step

Coverage: partial; snapshots exist but rollback is not currently automatic.

### REQ-INSTALL-002: Dry-run is read-only and errors are complete

Dry-run MUST NOT create, migrate, rename, or delete files. Concurrent results MUST preserve all agent/component failures in deterministic order. State, lock, profiles, install status, and manifests MUST use atomic persistence.

Primary oracle: full-tree before/after comparison and structured-error tests.

#### Scenario: Current-format dry-run

- **Given** a temporary home with current-format metadata
- **When** install, repair, or sync runs with dry-run
- **Then** the complete filesystem tree remains byte-for-byte unchanged

#### Scenario: Legacy profile dry-run

- **Given** a legacy profile file
- **When** dry-run loads it
- **Then** no backup, digest, migration, or directory is created

#### Scenario: Multiple concurrent failures

- **Given** failures in more than one agent chain
- **When** install returns
- **Then** every failure is available in stable agent/component order and no install status falsely indicates success

Coverage: missing for legacy migration and complete error aggregation.

### REQ-BUILDER-001: Multi-target rollback restores preimages

While Agent Builder remains compiled, each target write MUST record whether the target existed plus its bytes and mode. Rollback MUST restore replaced files and remove only files created by that operation.

Primary oracle: `internal/agentbuilder` rollback tests.

#### Scenario: All targets succeed

- **Given** existing and absent target files
- **When** installation succeeds everywhere
- **Then** all targets contain the generated skill and no rollback occurs

#### Scenario: Later target fails

- **Given** one replaced target, one newly created target, and a later failure
- **When** rollback runs
- **Then** the replaced file regains its original bytes/mode and the newly created file is removed

#### Scenario: Rollback itself fails

- **Given** a restore failure after an installation failure
- **When** the operation returns
- **Then** both errors are reported and no successful rollback is claimed

Coverage: missing for pre-existing targets.

### REQ-UNINSTALL-001: Uninstall removes only owned artifacts

Uninstall MUST plan concrete markers, files, JSON keys, and TOML tables from ownership evidence. It MUST include Agent Mailbox configuration but MUST NOT remove Mailbox runtime data. Shared directories MUST only be removed after owned children are removed and the directory is empty.

Primary oracle: install-then-uninstall tests for all four adapters.

#### Scenario: Complete owned uninstall

- **Given** a full installation with ownership records
- **When** `uninstall --all` succeeds
- **Then** all owned configuration, including Mailbox MCP registration, is absent and user content remains

#### Scenario: Codex TOML contains user tables

- **Given** managed and user-owned TOML tables
- **When** a managed MCP component is removed
- **Then** only its managed table disappears and the remaining TOML is valid

#### Scenario: Owned artifact cannot be removed

- **Given** an artifact that fails removal
- **When** uninstall runs
- **Then** state and lock remain sufficient for retry and the command returns a non-zero error

Coverage: partial; current uninstall does not include Mailbox and lacks complete TOML ownership handling.

### REQ-UNINSTALL-002: Uninstall is transactional

Uninstall MUST snapshot and verify configuration, state, lock, and status before mutation. Any failure MUST restore that snapshot. `--all` MUST clear metadata only after post-uninstall verification confirms no owned configuration remains.

Primary oracle: uninstall fault-injection and rollback integration tests.

#### Scenario: Successful uninstall

- **Given** a verified pre-uninstall snapshot
- **When** all planned removals and metadata commits succeed
- **Then** state reflects exactly the remaining installation

#### Scenario: Mid-operation failure

- **Given** an error after at least one owned artifact is removed
- **When** uninstall returns
- **Then** configuration, state, lock, and modes match the pre-uninstall snapshot

#### Scenario: Snapshot verification fails

- **Given** a corrupt pre-uninstall snapshot
- **When** uninstall reaches Apply
- **Then** Apply does not begin and no metadata is cleared

Coverage: missing.
