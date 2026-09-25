# Work Authority — Specification

## Purpose

Canonical control-plane contract for orchestrator-scoped reconciliation of orphaned live claims (`work reconcile`), its append-only audit event and secret-free structured receipt, the cortex-work bridge deference to durable state on `cortex_ia_work_claim`, and the orchestrator-only MCP reconcile surface. Requirements REQ-RECON-001..REQ-RECON-004 merged from change `orchestrator-reconcile` (board `orchestrator-reconcile`).

## Requirements

### Requirement: REQ-RECON-001 — Fail-closed force release of orphaned live claims
The control plane SHALL provide `cortex-ia work reconcile <task-id> --reason <text> --session <host-session-id> --revision <n> [--to ready] [--owner-session-inactive <true|false>]` which force-releases a LIVE-but-orphaned claim: the service SHALL act only when every release condition holds — (a) a trimmed non-empty bounded `--reason`; (b) the durable claim owner is NOT `opencode-session:<host-session-id>` (current-session protection); (c) the claim has no renewal within the 15-minute staleness window (mirroring the bridge `stale_progress_ms` maintenance window) OR host-attested `--owner-session-inactive true` evidence; (d) the explicit positive `--revision` matches the durable task revision (compare-and-set); and the task is `in_progress` with a non-expired claim (expired claims remain `work recover`'s exclusive domain). On release the service SHALL atomically delete the task's claim and all its file leases, transition the task via CAS to `blocked` (default) or to `ready` only when `--to ready` is given and the attempt limit `MaxWorkAttempts` is not reached, and increment the revision. Any unmet condition SHALL refuse with a typed error listing every failed condition code and SHALL mutate nothing. A task released to `blocked` SHALL remain eligible for the existing `work retry` path with unchanged retry semantics.

#### Scenario: stale orphaned claim is force-released to blocked
- **GIVEN** an `in_progress` task whose live claim owner is `opencode-session:dead-session`, whose last renewal is older than 15 minutes, with two retained file leases
- **WHEN** the orchestrator runs `work reconcile <task> --reason "controller died with lost token" --session <orchestrator-session> --revision <n>`
- **THEN** the claim and both leases are deleted, the task is `blocked` at revision n+1, and the receipt reports the released lease paths

#### Scenario: a live claim owned by the current session is never releasable
- **GIVEN** an `in_progress` task whose live claim owner equals the executing host session identity
- **WHEN** `work reconcile` runs for that task with any reason and a matching revision
- **THEN** the command refuses with the `current_session_owner` condition and no state mutates

#### Scenario: a recently renewed foreign claim stays untouchable
- **GIVEN** an `in_progress` task whose live foreign claim was renewed less than 15 minutes ago and no owner-inactivity evidence is supplied
- **WHEN** `work reconcile` runs for that task
- **THEN** the command refuses with the `claim_fresh_no_inactivity_evidence` condition, preserving cross-initiative lease protection

#### Scenario: direct-to-ready requires the attempt guard
- **GIVEN** an orphaned live claim whose task has already consumed `MaxWorkAttempts` claim attempts
- **WHEN** `work reconcile <task> ... --to ready` runs with otherwise valid conditions
- **THEN** the command refuses with the `attempt_limit` condition; the same invocation without `--to ready` releases to `blocked` where `work retry` enforces its own guards

### Requirement: REQ-RECON-002 — Append-only audit event and secret-free structured receipt
Every reconcile release SHALL append exactly one operational event to the existing append-only `work_events` ledger with kind `reconciled`, the from/to statuses, and a JSON detail snapshot recording: actor host session, reason text, durable claim owner, attempt, claim expiry, last renewal, staleness window, the owner-session-inactivity evidence bit, revisions before and after, and the released lease paths. Neither the event, the JSON receipt, nor any log line SHALL ever contain a plaintext claim or lease token or its hash. The CLI SHALL render a stable structured JSON receipt (decision, task id, from/to status, revisions, released leases, decision inputs) that MCP and human consumers share. A refusal SHALL leave the ledger, claim, leases, status, and revision byte-identical to before the invocation.

#### Scenario: release writes the decision-input snapshot
- **GIVEN** a successful reconcile of a stale orphaned claim
- **WHEN** the task's `work_events` history is read
- **THEN** exactly one new `reconciled` event exists whose detail JSON carries the actor, reason, owner, last-renewal, inactivity-evidence, revision pair, and released lease paths

#### Scenario: no token material is persisted
- **GIVEN** any reconcile outcome (release or refusal) executed after a real claim issuance
- **WHEN** the audit event detail and CLI receipt are scanned
- **THEN** no claim token, lease token, or token hash appears in either output

#### Scenario: refusal mutates nothing
- **GIVEN** an `in_progress` task with a live current-session claim and retained leases
- **WHEN** `work reconcile` refuses it with a `current_session_owner` error listing that failed condition
- **THEN** the claim row, every lease row, the task status, the task revision, and the `work_events` count are unchanged

#### Scenario: text receipt for humans
- **GIVEN** a successful `work reconcile` CLI invocation
- **WHEN** the command output is captured
- **THEN** it contains the machine-readable JSON receipt document and one human-readable summary line naming task, transition, revision, and released-lease count

### Requirement: REQ-RECON-003 — Bridge defers stale in-memory authority to durable state
The cortex-work bridge SHALL treat durable state as authoritative over its in-memory authority map. On `cortex_ia_work_claim`, when a retained handle exists for the task, the bridge SHALL first probe durable status: if the durable claim is not live OR the durable owner no longer matches the handle's controller identity, the bridge SHALL stop that handle's maintenance loop, drop the stale handle, and proceed with the fresh claim; it SHALL throw "authority already held by this controller" ONLY while a durable live claim still matches the retained handle's own session identity. If the durable probe itself fails, the bridge SHALL keep refusing (fail closed) rather than claim blindly.

#### Scenario: fresh claim succeeds over a durable-void handle
- **GIVEN** the bridge retains an in-memory authority handle for a task whose durable status shows no live claim (`durable_claim_live=false`) as after the obs-176 recovery
- **WHEN** an implement controller calls `cortex_ia_work_claim` for that task
- **THEN** the stale handle is dropped and the fresh CLI claim is issued and retained without any "already held" error

#### Scenario: genuinely held live claim still refuses a second handle
- **GIVEN** the bridge retains a handle whose durable claim is live and owned by that same handle session
- **WHEN** `cortex_ia_work_claim` is called again for the task on the same bridge
- **THEN** the "already held by this controller" refusal is raised and no second durable claim is attempted

#### Scenario: durable probe failure keeps the refusal fail-closed
- **GIVEN** the bridge retains a handle for a task and the durable status probe errors or is unavailable
- **WHEN** `cortex_ia_work_claim` is called for that task
- **THEN** the bridge refuses without mutating durable state and surfaces the status-unavailability error

### Requirement: REQ-RECON-004 — Orchestrator-only MCP reconcile surface
The bridge SHALL expose `cortex_ia_work_reconcile {task_id, reason, revision, to?}` classified in the role gate for the `orchestrator` role only, delegating to the CLI verb so that fail-closed semantics are identical. The tool SHALL derive the host session identity exclusively from its own execution context (never a caller-supplied argument) and SHALL set the owner-inactivity evidence bit exclusively from the bridge's own host session tracking when the durable claim's owner session is known to have been an active host session that is no longer active; every other invocation SHALL pass evidence false. Refusions SHALL propagate the durable typed error with its failed-condition codes verbatim.

#### Scenario: orchestrator reconciles through the bridge
- **GIVEN** an orphaned live foreign claim on a task and a tool call from an orchestrator session
- **WHEN** `cortex_ia_work_reconcile` executes with reason and matching revision
- **THEN** the CLI verb runs with `--session` bound to the orchestrator's context session and the durable JSON receipt is returned unchanged

#### Scenario: every other role is denied
- **GIVEN** a controller session with agent role implement, planner, investigate, discovery, or reviewer
- **WHEN** it attempts `cortex_ia_work_reconcile`
- **THEN** the bridge refuses with `BRIDGE_ROLE_DENIED` before any CLI invocation

#### Scenario: inactivity evidence is host-attested only
- **GIVEN** a reconcile request whose durable owner session is still tracked as an active host subagent session
- **WHEN** the bridge builds the CLI arguments
- **THEN** the owner-inactivity evidence is passed as false, so a recently renewed live foreign claim remains protected

#### Scenario: expired claims are routed to recover
- **GIVEN** a task whose claim has already passed its durable expiry
- **WHEN** `cortex_ia_work_reconcile` executes for it
- **THEN** the durable service refuses with `claim_not_live` and the orchestrator uses `work recover` plus `work retry` instead
