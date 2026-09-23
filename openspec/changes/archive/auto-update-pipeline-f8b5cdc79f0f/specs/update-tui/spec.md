# Spec Delta: update-tui (auto-update-pipeline)

## ADDED Requirements

### Requirement: REQ-UT-001 — Boot prompt asks before applying, never silently
At TUI startup (`internal/tui.Run`), if the persisted update state marks an update available (or an explicitly opted-in inline check with a short, bounded timeout finds one), the TUI SHALL present an interactive confirmation `¿Actualizar a vX.Y.Z? [y/N]` before any download or apply. Absent confirmation (any key other than y/Y), the TUI SHALL continue booting normally and preserve the cached state. The TUI SHALL NEVER replace the binary without this confirmation, and the scheduled headless task SHALL never reach an apply path.

#### Scenario: cached update prompts at boot
- **GIVEN** `update-state.json` records `available` `v0.5.0`
- **WHEN** the TUI boots
- **THEN** the prompt names `v0.5.0` and waits for y/N before proceeding

#### Scenario: decline preserves state and boots normally
- **GIVEN** the prompt is shown and the user presses `n` or EOF arrives
- **WHEN** the TUI continues
- **THEN** the binary is untouched and the cached `available` entry remains for the next boot

#### Scenario: no cached state boots silently
- **GIVEN** no update state exists and the inline check is not opted in
- **WHEN** the TUI boots
- **THEN** no prompt appears and startup latency is unchanged

### Requirement: REQ-UT-002 — Active work authority is a hard apply gate
Before applying, the TUI flow SHALL consult the global active-authority probe (REQ-DP-002). If any claim, lease, or in-progress task is active, the update SHALL NOT be downloaded or applied; the TUI SHALL state that an update to the target version is available but blocked while work authority is active, and SHALL leave the cached state intact for a later attempt.

#### Scenario: live claim defers the apply
- **GIVEN** an active claim exists in `delegation.db` and the user confirms the prompt
- **WHEN** the apply gate evaluates
- **THEN** no download starts and the message names the blocking condition

#### Scenario: gate stays open without a database
- **GIVEN** `delegation.db` does not exist and the user confirms the prompt
- **WHEN** the apply gate evaluates
- **THEN** the apply proceeds because no authority can be active

#### Scenario: blocked apply preserves the cached offer
- **GIVEN** the gate blocked an apply of `v0.5.0`
- **WHEN** state is reloaded
- **THEN** `available` still records `v0.5.0` for the next boot

### Requirement: REQ-UT-003 — Confirmed apply reuses the shipped flow and announces restart
On confirmation with the gate open, the TUI SHALL run the existing `ApplyUpdate` path (download + manifest/signature verification + digest-verified staged replacement with rollback via `replacement.go`) and, on success, print a restart notice instructing the user to restart cortex-ia. On failure, the TUI SHALL surface the typed error, keep the old binary, and continue booting.

#### Scenario: confirmed apply succeeds end to end
- **GIVEN** a canonical current version, packaged trust keys, and no active authority
- **WHEN** the user confirms the prompt
- **THEN** the binary is replaced with the digest-verified release, state records the new floor, and the restart notice is shown

#### Scenario: apply failure keeps the old binary
- **GIVEN** the artifact download produces a digest mismatch
- **WHEN** the apply fails
- **THEN** the running binary is unchanged and the typed error is displayed

#### Scenario: declined apply never touches the floor
- **GIVEN** persisted `applied_floor` `v0.4.50` and a declined prompt for `v0.5.0`
- **WHEN** the TUI boots through
- **THEN** `applied_floor` remains `v0.4.50` and the offer stays cached

### Requirement: REQ-UT-004 — Schema drift at boot offers the update flow
When TUI or CLI store access fails with the typed `SchemaDriftError` (REQ-DP-001), the surface SHALL render an actionable message naming the stale binary and `cortex-ia update` as the recovery path, instead of the bare internal initialization error.

#### Scenario: drifted schema message names the recovery command
- **GIVEN** the on-disk delegation schema is newer than the binary supports
- **WHEN** the TUI attempts to open the store
- **THEN** the rendered guidance names `cortex-ia update` with the detected and supported schema versions

#### Scenario: healthy schema shows no guidance
- **GIVEN** the on-disk delegation schema equals the supported version
- **WHEN** the TUI opens the store
- **THEN** no drift guidance is rendered

#### Scenario: missing database shows no drift guidance
- **GIVEN** no delegation database exists
- **WHEN** the TUI or CLI evaluates store health
- **THEN** the fresh-install path renders no drift guidance
