# Spec Delta: updater (auto-update-pipeline)

## ADDED Requirements

### Requirement: REQ-AU-001 — Trust store injection is build-time and fail-closed
The updater SHALL accept a production trust bundle injected at build time via `go build -ldflags -X` on a single string variable in `internal/updater/trust.go`. The bundle SHALL be decoded and validated with the existing invariants (`ValidateTrustStore`, `MaxTrustedKeys` = 2, 32-byte Ed25519 public keys). When the variable is unset or invalid, the trust store SHALL remain empty and `RequireTrust` SHALL fail closed with `ErrNoTrustedKey`. Tests SHALL inject keys only through `SetTrustedKeysForTesting`; the private signing key SHALL never exist in the repository or in any build input.

#### Scenario: empty bundle keeps fail-closed default
- **GIVEN** a build where the trust-bundle ldflags variable was never set
- **WHEN** `RequireTrust` is called before any test injection
- **THEN** it returns `ErrNoTrustedKey` and both check and apply refuse to proceed

#### Scenario: valid bundle activates packaged keys
- **GIVEN** a build ldflags-injected with an encoded bundle describing exactly one valid key record with ID, 32-byte public key, repository, and optional version interval
- **WHEN** the updater initializes and `LookupTrustedKey` is called with that ID
- **THEN** the key resolves with its repository binding and version interval intact

#### Scenario: malformed bundle is rejected and not partially applied
- **GIVEN** an injected bundle whose encoded payload truncates a public key to 31 bytes
- **WHEN** the bundle is decoded at init
- **THEN** decoding fails and the active trust store remains empty without any panic

### Requirement: REQ-AU-002 — Update state persists under the Cortex-IA home
The updater SHALL persist update state to `~/.cortex-ia/update-state.json` with an explicit `schema_version` (1), `last_checked_at`, `available` (empty when up to date), `applied_floor` (last version applied or verified on this machine), `managed_path` (`os.Executable()` resolved at check/apply time), and `install_candidates` (known binary locations observed). Writes SHALL be atomic through the existing `filemerge.WriteFileAtomic`, and a missing, empty, or corrupt file SHALL load as empty state with a typed soft error instead of crashing the caller.

#### Scenario: first check creates state atomically
- **GIVEN** no `update-state.json` exists
- **WHEN** a check-only pass completes with a newer release found
- **THEN** the file exists with `schema_version` 1, `available` set to the release tag, and `last_checked_at` set with no partial file ever observable

#### Scenario: corrupt state degrades to empty
- **GIVEN** `update-state.json` contains invalid JSON
- **WHEN** the state is loaded
- **THEN** the loader returns empty state with a typed error flag instead of failing the caller

#### Scenario: apply result is reflected in managed path and floor
- **GIVEN** a successful apply of `v0.5.1` with the running executable at a known path
- **WHEN** state is reloaded
- **THEN** `applied_floor` equals `v0.5.1`, `available` is empty, and `managed_path` records the executable path used

### Requirement: REQ-AU-003 — Applied floor is enforced across sessions
The apply path SHALL read `applied_floor` from persisted state and pass it to `VerifyVersionFloor` through `Client.AppliedFloor`, replacing the hardcoded empty floor at `download.go:152`. After a successful `ApplyUpdateToTarget`, the updater SHALL persist the applied release tag as the new floor. A candidate lower than or equal to the persisted floor SHALL be rejected with `ErrDowngradeOrReplay` even when it is newer than the running binary.

#### Scenario: replay of an already-applied version is rejected
- **GIVEN** persisted state records `applied_floor` `v0.5.0` and the running binary is `v0.4.9`
- **WHEN** an apply of release `v0.5.0` is attempted
- **THEN** `VerifyVersionFloor` fails with `ErrDowngradeOrReplay` before any download

#### Scenario: successful apply raises the floor
- **GIVEN** an apply of `v0.5.1` completes with digest-verified replacement
- **WHEN** state is reloaded
- **THEN** `applied_floor` equals `v0.5.1` and `available` is cleared

#### Scenario: failed apply never moves the floor
- **GIVEN** persisted `applied_floor` `v0.5.0` and an apply of `v0.5.2` that fails during artifact verification
- **WHEN** state is reloaded
- **THEN** `applied_floor` remains `v0.5.0` and no partial state is written

### Requirement: REQ-AU-004 — Check and apply agree on canonical versions
`CheckLatest` SHALL NOT fall back to the lax `IsNewer` comparison. When `CheckUpdateCandidate` fails because the current or candidate version is non-canonical (dev, unknown, or '+'/'-' suffixes), `CheckLatest` SHALL return a typed `ErrNonCanonicalVersion` error with `hasUpdate` false instead of announcing an update. Dev and unknown current versions SHALL continue to report no update without error, matching existing behavior.

#### Scenario: dirty build is not announced an update
- **GIVEN** the running binary reports `v0.4.50+dirty` and GitHub latest is `v0.5.0`
- **WHEN** `CheckLatest` runs
- **THEN** it returns a typed `ErrNonCanonicalVersion` error with `hasUpdate` false

#### Scenario: canonical pair still compares strictly
- **GIVEN** current `v0.4.9` and latest `v0.5.0`
- **WHEN** `CheckLatest` runs
- **THEN** it returns the release with `hasUpdate` true

#### Scenario: dev build reports no update without error
- **GIVEN** the running binary reports `dev` and GitHub latest is `v0.5.0`
- **WHEN** `CheckLatest` runs
- **THEN** it returns `hasUpdate` false with no error

### Requirement: REQ-AU-005 — User-level scheduler registration
The updater SHALL expose scheduler operations `EnableScheduledCheck`, `DisableScheduledCheck`, and `ScheduledCheckStatus` backed by an injectable command-runner seam. On Windows it SHALL use user-level `schtasks` (no administrator elevation) creating a daily task named `CortexIA Update Check` whose action is the cortex-ia executable with `update --check --scheduled`; on POSIX it SHALL manage a user crontab line with the same payload. Status SHALL report enabled, missing, or error with the underlying task state.

#### Scenario: enable registers a check-only user task
- **GIVEN** the injectable runner records executed commands
- **WHEN** Enable is called on Windows with the resolved executable path
- **THEN** exactly one `schtasks /Create` invocation is issued with the stable task name and `update --check --scheduled` as arguments with no elevation flags

#### Scenario: disable is idempotent
- **GIVEN** no scheduled task exists
- **WHEN** Disable is called
- **THEN** the operation reports not-present without erroring

#### Scenario: status reflects the registered task
- **GIVEN** Enable succeeded through the stub runner
- **WHEN** Status is called
- **THEN** it reports the task as enabled with the configured check-only action

### Requirement: REQ-AU-006 — Scheduled mode is quiet, check-only, and cache-writing
The CLI update command SHALL accept a `--scheduled` flag that is only valid together with `--check`. In scheduled mode the command SHALL produce no interactive prompt, apply nothing, keep output minimal, persist the check result to `update-state.json`, and exit 0 when the check completes even without an update. Failures (including fail-closed trust) SHALL print a concise message and exit non-zero without applying or corrupting state.

#### Scenario: scheduled check with no update
- **GIVEN** the latest release equals the running version
- **WHEN** `cortex-ia update --check --scheduled` runs
- **THEN** it exits 0, writes `last_checked_at`, and leaves `available` empty

#### Scenario: scheduled check never applies
- **GIVEN** a newer release exists
- **WHEN** the scheduled check runs
- **THEN** only `available` is persisted and no download or binary replacement occurs

#### Scenario: scheduled check without trust fails closed
- **GIVEN** no trust keys are packaged in the binary
- **WHEN** the scheduled check runs
- **THEN** it exits non-zero with a concise fail-closed message, applies nothing, and leaves prior state intact

### Requirement: REQ-AU-007 — Dual-install detection warns on ambiguous installs
The state layer SHALL resolve known install candidates (the running executable, the Windows `%LOCALAPPDATA%\Programs\cortex-ia\bin` target used by `scripts/install.ps1:11`, and the user GOPATH `bin` location) and record them in state. When more than one candidate binary exists, check and scheduled surfaces SHALL surface a warning naming all found paths without failing the operation.

#### Scenario: two installations produce a warning
- **GIVEN** binaries exist both under GOPATH bin and under the LOCALAPPDATA install dir
- **WHEN** a check-only pass completes
- **THEN** state records both candidates and the surface warns listing both paths

#### Scenario: single installation produces no warning
- **GIVEN** only the running executable own install location exists
- **WHEN** a check-only pass completes
- **THEN** no dual-install warning is emitted

#### Scenario: detection failure never blocks the check
- **GIVEN** candidate probing encounters an unreadable directory
- **WHEN** the check-only pass completes
- **THEN** the check result and state write still succeed and the warning degrades to best effort
