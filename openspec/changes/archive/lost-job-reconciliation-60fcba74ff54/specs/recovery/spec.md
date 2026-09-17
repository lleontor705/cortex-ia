# Delta: audited lost-job recovery

## ADDED Requirements

### Requirement: REQ-RECON-001: Evidence-gated reconciliation
The service MUST reconcile only a lost job when internally collected trusted OS boot evidence proves that its recorded execution started before the current boot. Caller-supplied timestamps and missing or reused PIDs MUST NOT authorize release.

#### Scenario: Prior-boot lost worker
- GIVEN a lost job with a valid recorded start preceding verified boot time
- WHEN explicit reconciliation with a bounded reason is requested
- THEN the service records termination evidence for that exact job attempt

#### Scenario: Same-boot absent or reused PID
- GIVEN a lost job from the current boot with an absent PID or a PID now used by another process
- WHEN reconciliation is requested
- THEN it fails closed and retains the fence because descendants remain unproven

#### Scenario: Active or unverifiable execution
- GIVEN an active job, missing start, unsupported platform, unavailable boot evidence or ambiguous time ordering
- WHEN reconciliation is requested
- THEN it refuses without recording a successful proof or altering authority

### Requirement: REQ-RECON-002: Durable atomic proof and admission
The service MUST atomically revalidate job identity and state and append one immutable proof per job and attempt. It MUST preserve lost status, error history and receipt bytes; proof MUST clear only the matching reconciliation flag and admission fence without restoring old authority.

#### Scenario: Successful release
- GIVEN a lost fenced attempt and valid proof
- WHEN reconciliation commits
- THEN status and result report no reconciliation requirement, history remains unchanged and new admission can proceed under fresh authority

#### Scenario: Duplicate and concurrent requests
- GIVEN an already reconciled attempt or concurrent reconciliation calls
- WHEN the operation is repeated
- THEN it returns the durable proof idempotently without duplicate proof/events or affecting other jobs

#### Scenario: Concurrent change or persistence error
- GIVEN the attempt, start, status or identity changes before commit, or proof persistence fails
- WHEN reconciliation attempts to commit
- THEN no fence is released and no partial proof survives

### Requirement: REQ-RECON-003: Explicit operation and actionable errors
The CLI MUST expose delegate reconcile ID --reason TEXT. The bridge MUST expose the operation only to the orchestrator, require host identity, and return truthful proof/state without closing panes. Admission conflicts MUST name the blocking job and an applicable next action.

#### Scenario: Authorized recovery
- GIVEN an orchestrator requesting reconciliation of a prior-boot lost job
- WHEN the bridge calls the CLI
- THEN it returns the audited outcome without killing processes, closing Herdr resources or starting replacement work

#### Scenario: Admission remains blocked elsewhere
- GIVEN another active or unproven job in the same workspace
- WHEN a new job is requested after one job is reconciled
- THEN admission names the remaining blocker and retains exclusivity

#### Scenario: Unauthorized or invalid request
- GIVEN a non-orchestrator role or a missing, empty or excessive reason
- WHEN reconciliation is requested
- THEN the boundary rejects it without a mutation and without granting old claims or leases
