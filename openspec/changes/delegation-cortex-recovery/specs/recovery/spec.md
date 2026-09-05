# Authorized recovery and delivery

## ADDED Requirements

### Requirement: REQ-RECOVERY-001 — Bounded bootstrap and workspace authority
An explicitly authorized bootstrap MAY create one bounded direct-change task; it MUST preserve independent review, role boundaries and SQLite claims/leases. Only planner decomposes the routed blocked parent at CAS6. The exception to use OpenSpec now MUST NOT change general preference. Implement MUST inspect authoritative workspace and reserve every writable path before edits. Gate outcome, not preferences, selects execution mode.

#### Scenario: Happy path
- GIVEN current root authority, a clean related isolated worktree at the same HEAD and fresh per-file leases
- WHEN one implement leaf is accepted
- THEN only that leaf edits allowlisted files and the controller reconciles its terminal receipt before further work.

#### Scenario: Edge case
- GIVEN a legitimate native gate result with no accepted job
- WHEN controller authority/reservations demonstrably cover its actual workspace
- THEN native work is allowed; native alone is not grounds to cancel. An unrelated or unaccredited worktree remains unauthorized.

#### Scenario: Error state
- GIVEN HEAD mismatch, expired authority or a previously accepted timed-out job
- WHEN recovery is attempted
- THEN no gate weakening, main ref movement, forced rejection, automatic postacceptance fallback or parent retry is allowed; preserve evidence and stop/reconcile.

### Requirement: REQ-RECOVERY-002 — Local-only verified delivery
Delivery MUST collect reviewed assets on `fix/delegation-cortex-recovery`, build with Go1.26.1, verify installation effects and backups before authorized local application, reload assets, and distinguish probe, installation and actual transport E2E verdicts. Persistent tests remain exclusively TUI/simple install-copy; other new checks are isolated ephemeral smokes. No remote push/PR or telemetry transmission is authorized here.

#### Scenario: Happy path
- GIVEN reviewed integrated assets and an accredited exact installation manifest/verified backup
- WHEN the local build is installed and OpenCode reloads
- THEN installed hashes match intended sources and one bounded real delegation produces a reconciled durable receipt with the actual execution mode.

#### Scenario: Edge case
- GIVEN Herdr is unavailable and a safe preacceptance direct fallback succeeds
- WHEN E2E is reported
- THEN direct success is distinguished from Herdr E2E; no pane-based claim of authority or false Herdr PASS is made.

#### Scenario: Error state
- GIVEN unmanaged drift, missing toolchain, unmatched install paths or failed/timeout/lost E2E
- WHEN delivery continues
- THEN stop, retain backup and evidence, do not publish remotely, and never claim installed/reloaded/E2E success from build or dry-run alone.

## Verification boundary
Branch/baseline, source diff, build, local installation and transport need separate receipts. Fixture homes/state MUST be isolated; the authorized real installation is a deployment, not a fixture. Installer-generated files require an enumerated manifest and applicable operational authority, not broad source leases. Current planner cannot attest that manifest or runtime readiness; the build/readiness task produces it for exact final operational task materialization on this same board.
