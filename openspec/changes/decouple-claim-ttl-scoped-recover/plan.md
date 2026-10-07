# Plan — Decouple Claim TTL from Stale-Progress Window & Scoped Self-Recovery for Implement

Workflow: sdd-lite (integrated). Board: `ttl-recover-release`. Spec plane: hybrid. Workload policy: flexible.

## Intent

Railway telemetry hub filed three open bug reports against the cortex-work bridge. Cause A of #140 `ERR_TASK_BLOCKED` (5/7 signatures) and all of #142 `ERR_INVARIANT_VIOLATION` (1 signature) trace to the implementation claim TTL being deliberately coupled to the stale-progress window at `internal/assets/plugins/cortex-work.ts:128` (`maintenancePolicy = { ..., stale_progress_ms: 900000, ttl: "15m" }`): long silent build phases (e.g. dotnet restores on exFAT) outlive the coupled window, the heartbeat stops, and the claim expires before the minion can transition to `in_review`. The orphan-guard itself is deliberate and pinned by `scripts/harness-controller-authority.test.mjs` and must not be weakened. #143 `ERR_TOOL_LEASE_REQUIRED` is correct fail-closed behavior (envelope arrived with `task_id: null`); its remediation lives outside this repository and is out of scope.

User value: (1) slow-but-alive implementation sessions stop losing claims to the coupled 15-minute ceiling — the claim TTL is decoupled so renewed claims get 30 minutes of validity headroom while the 15-minute no-host-activity orphan window stays intact and detectable; (2) when expiry still happens, the `implement` role can recover **its own** expired task only — narrowly scoped, ownership-verified, fail-closed — instead of failing the whole attempt (today recovery is orchestrator-only, `work_recover: ["orchestrator"]` at `cortex-work.ts:1489`); (3) `CHANGELOG.md` (stale at v0.4.56) is backfilled with v0.5.3..v0.5.7 plus the v0.5.8 entry before the orchestrator tags the release.

Non-goals: no tags, releases, pushes, or GitHub issue comments/closures (the orchestrator coordinates the release separately after the DAG completes); no weakening of the orphan-guard authority model beyond the two scoped changes (stale-progress stop, recover-only-expired semantics, and harness authority tests stay intact); no progress-based dynamic TTL logic; `implement` may never recover other sessions' tasks, bypass file leases, or exceed the durable attempt counter.

Risks: worst-case orphan-detection visibility grows from ~30 to ~45 minutes with ttl=30m (accepted; the stale-progress stop itself is unchanged and scoped self-recovery shrinks the cost of the window); the authority-surface regression risk is mitigated by pinned harness tests, new delegation store tests, and mandatory independent review of the authority tasks; asset edits are go:embed, so the protocol task rebuilds the binary.

## Requirements

### Requirement: REQ-WAUTH-001 Claim TTL decoupled from stale-progress window

The bridge maintenance policy SHALL declare the claim `ttl` as an independent constant (`"30m"`) that is NOT derived from, equal to, or documented as coupled to `stale_progress_ms` (`900000`, unchanged).

Test: node --test --test-name-pattern TestREQ_WAUTH_001 scripts/harness-controller-authority.test.mjs

#### Scenario: Busy host renews with decoupled window

- **GIVEN** an implement controller claims a task and the host session stays busy
- **WHEN** the claim heartbeat renews every `interval_ms`
- **THEN** each renewal grants a 30-minute validity window independent of the stale-progress window

#### Scenario: Silent host still hits the stale-progress stop

- **GIVEN** a claim heartbeat renews at time T and the host session then goes silent
- **WHEN** 15 minutes of no host progress elapse
- **THEN** the heartbeat stops and the claim expires at most 30 minutes after the last successful renewal

#### Scenario: Policy constants are independent

- **GIVEN** the maintenance policy object in the shipped plugin
- **WHEN** any reviewer inspects `maintenancePolicy` and its derivation
- **THEN** `ttl` is not `"15m"` and no code path computes `ttl` from `stale_progress_ms`

### Requirement: REQ-WAUTH-002 Scoped self-recovery tool for the implement role

The bridge SHALL expose `cortex_ia_work_recover_own(task_id)` classified in the mutation gate as allowed for role `implement` only. The unscoped `cortex_ia_work_recover` SHALL remain orchestrator-only.

Test: node --test --test-name-pattern TestREQ_WAUTH_002 scripts/harness-controller-authority.test.mjs

#### Scenario: Owning implement session recovers its expired task

- **GIVEN** an implement session holds the in-memory work authority for `task_id` and the durable claim is expired
- **WHEN** it calls `cortex_ia_work_recover_own(task_id)`
- **THEN** the bridge delegates to `work recover --task <id> --owner opencode-session:<sessionID>` and returns the recovery receipt

#### Scenario: Foreign session fails closed with no CLI call

- **GIVEN** a caller whose session does not match the in-memory authority for `task_id`
- **WHEN** it calls `cortex_ia_work_recover_own(task_id)`
- **THEN** the call throws and no CLI recovery command executes

#### Scenario: Non-implement roles are denied by the gate

- **GIVEN** any role other than `implement` invoking `cortex_ia_work_recover_own`
- **WHEN** the mutation gate evaluates the call
- **THEN** the call is rejected with `BRIDGE_ROLE_DENIED`

#### Scenario: Success drops the stale in-memory handle

- **GIVEN** a successful scoped recovery of the task
- **WHEN** the bridge returns the receipt
- **THEN** the stale in-memory authority handle and its maintenance loop are stopped and removed for that task

### Requirement: REQ-WAUTH-003 CLI scoped recover is ownership-verified and fail-closed

`cortex-ia work recover` SHALL accept an optional scoped form `--task <id> --owner <identity>`. Both flags MUST be supplied together; otherwise usage fails.

Test: go test -count=1 -run TestREQ_WAUTH_003 ./internal/delegation/...

#### Scenario: Owner-matched expired claim is recovered alone

- **GIVEN** a scoped recover request for task T with owner identity O where the durable claim on T is expired and owned by O
- **WHEN** the store executes the scoped recovery
- **THEN** only T's expired claim and file leases are recovered and the receipt reports exactly one recovered task

#### Scenario: Ownership or expiry mismatch fails closed

- **GIVEN** a scoped recover request where the durable claim owner does not match the supplied identity or the claim is not expired
- **WHEN** the store executes the recovery
- **THEN** it fails closed with a typed denial error and writes nothing

#### Scenario: Unscoped form is unchanged

- **GIVEN** the unscoped form `cortex-ia work recover` without flags
- **WHEN** executed
- **THEN** behavior is unchanged for all expired claims and its bridge-tool exposure remains gated to `orchestrator`

### Requirement: REQ-WAUTH-004 Protocol and agent assets codify the new boundary

The canonical work protocol and implement agent assets SHALL document (a) the decoupled TTL values and (b) the scoped self-recovery grant limited to the caller's own expired task, fail-closed on any other target.

Test: go build -o bin/cortex-ia ./cmd/cortex-ia

#### Scenario: Protocol authority table distinguishes the recovery tools

- **GIVEN** the canonical work protocol authority table
- **WHEN** a reader looks up recovery tooling
- **THEN** `cortex_ia_work_recover` and `cortex_ia_work_retry` remain orchestrator-only and `cortex_ia_work_recover_own` is listed as implement-scoped to its own expired task

#### Scenario: Implement agent guard permits only the scoped form

- **GIVEN** the implement agent permission surface with its guard on `cortex-ia work recover*`
- **WHEN** the guard is evaluated against the scoped own-task form and the unscoped form
- **THEN** the scoped own-task form is explicitly permitted and unscoped recovery remains denied

#### Scenario: Rebuilt binary embeds the updated assets

- **GIVEN** the edited protocol and agent assets are embedded via go:embed
- **WHEN** the binary is rebuilt from the working tree
- **THEN** the build succeeds with the updated assets included

## Design

#### Data & Policy Model

```ts
const maintenancePolicy = { interval_ms: 30000, status_timeout_ms: 5000, stale_progress_ms: 900000, ttl: "30m" };
```

- `ttl` becomes an independent constant (claim validity granted per heartbeat renewal); no longer equal to `stale_progress_ms`.
- `stale_progress_ms` (900000 = 15 min) is the **no-host-activity orphan window** and stays untouched: the heartbeat stops when `Date.now() - state.lastProgress >= stale_progress_ms` (call sites ~653/679/751), so stale claims remain detectable by recovery.
- Rejected alternative — progress-based dynamic TTL: renewal duration would depend on host event cadence, coupling orthogonal concerns; static constants keep the harness deterministic and the orphan-guard provable.

#### New bridge tool (plugin)

```ts
cortex_ia_work_recover_own: tool({
  description: "Recover the caller's own expired work claim and file leases for exactly one task (fail-closed ownership check).",
  args: { task_id: tool.schema.string() },
  async execute(args, context) {
    const authority = workAuthority.get(args.task_id);
    if (!authority || authority.sessionID !== context.sessionID) {
      throw new Error("scoped recover denied: session does not own the in-memory authority for this task");
    }
    const result = cortex(["work", "recover", "--task", args.task_id, "--owner", controllerIdentity(context.sessionID)]);
    stopMaintenance(authority, "recovered");
    workAuthority.delete(args.task_id);
    saveAuthorityState();
    return result;
  }
})
```

Mutation-classification map: `work_recover_own: ["implement"]` (new capability entry; the generic fail-closed gate loop covers it — an unclassified capability throws `BRIDGE_POLICY_UNCLASSIFIED`).

#### CLI surface (Go)

`cortex-ia work recover [--task <id> --owner <identity>]` in `internal/app/work.go`: flags mandatory together (usage error otherwise); unscoped invocation keeps calling `store.RecoverWork(ctx)`; scoped invocation calls the new `store.RecoverWorkScoped(ctx, taskID, owner)` in `internal/delegation/work.go`.

- Reuse the expiry/lease-release machinery inside `RecoverWork` (refactor its per-task body into a shared helper rather than duplicating SQL; both paths run under `BEGIN IMMEDIATE`).
- Denial is a typed sentinel error (e.g. `ErrScopedRecoverDenied`) wrapping reason (`owner_mismatch` | `claim_not_expired` | `task_not_found`); nothing is written on denial.

#### Sequence Flow (scoped self-recovery)

1. Implement minion's heartbeat stopped earlier via stale-progress; claim expired in SQLite.
2. Minion calls `cortex_ia_work_recover_own({ task_id })` from the same host session.
3. Plugin checks in-memory authority ownership (fail closed on mismatch or absent handle).
4. Plugin shells out to `cortex-ia work recover --task T --owner opencode-session:S`.
5. Store verifies durable owner + expiry inside the write transaction, releases the claim and file leases, returns receipt `{ recovered: 1 }`.
6. Plugin drops the stale in-memory handle; the minion re-claims T fresh (new attempt counter, new tokens) — recovery never resurrects old tokens.

#### Trade-offs & Alternatives Considered

- **Pure plugin-side gating (no CLI change)**: rejected. Role gates are in-memory plugin policy; durable ownership must be enforced where state lives (SQLite) or any direct CLI caller could bypass the scope.
- **Extend `work_reconcile` instead of a new tool**: rejected — reconcile carries owner-inactivity evidence semantics for orchestrators and its fail-closed conditions differ from claim-expiry recovery.
- **TTL 30m vs. larger**: 30m covers observed slow builds (Issue #140) without stretching worst-case orphan visibility beyond ~45 minutes.
- **Deleting the in-memory handle on scoped recover** mirrors the existing `work_approve` cleanup pattern (line ~1451) and prevents a stale handle from blocking a fresh claim (durable-deference behavior shipped in v0.4.53).

#### Security & Authority Invariants

- Ownership check is two-layer: in-memory (plugin, session identity) and durable (store, claim owner identity) — both must agree; any mismatch fails closed.
- No tokens appear in CLI argv beyond the owner identity (already the pattern for `controller-renew`); claim/lease tokens never transit the scoped-recover path.
- The mutation gate remains fail-closed for unclassified capabilities; `work_recover` stays `["orchestrator"]`.
- Existing orphan-guard assertions in `scripts/harness-controller-authority.test.mjs` stay untouched; only TTL-coupling expectations and new scoped-recover cases are added.

## Tasks

Dependency-safe DAG on board `ttl-recover-release`; workload policy `flexible`; all tasks within budget (no `WORKLOAD_ADVISORY` expected). Review policy: T2 and T3 touch authority boundaries → mandatory independent reviewer; T1 is pure-test (never decompose on failure — fix assertions directly); T4 and T5 are docs → orchestrator auto-approval per the Adaptive Review Policy. The authoritative task list with requirement traceability lives in tasks.md.

### T1 ttl-recover-01-contracts [work-authority] Harness tests for decoupled TTL and scoped recover guard

Requirements: REQ-WAUTH-001, REQ-WAUTH-002, REQ-WAUTH-003

Files: `scripts/harness-controller-authority.test.mjs`. RED-first contracts for both scoped authority changes. Depends on: none.

Verification: node --test --test-name-pattern TestREQ scripts/harness-controller-authority.test.mjs

### T2 ttl-recover-02-plugin [work-authority] Decouple maintenancePolicy TTL and add cortex_ia_work_recover_own

Requirements: REQ-WAUTH-001, REQ-WAUTH-002

Files: `internal/assets/plugins/cortex-work.ts`. Depends on: T1.

Verification: node --test scripts/harness-controller-authority.test.mjs

### T3 ttl-recover-03-cli-store [delegation] Scoped recover in CLI and store with fail-closed ownership

Requirements: REQ-WAUTH-003

Files: `internal/app/work.go`, `internal/delegation/work.go`, `internal/delegation/recovery_test.go`. Depends on: none.

Verification: go test -count=1 ./internal/delegation/... ./internal/app/...

### T4 ttl-recover-04-protocol-docs [protocol] Codify scoped recover authority and decoupled TTL in protocol and agent assets

Requirements: REQ-WAUTH-004

Files: `internal/assets/skills/_shared/cortex-work-protocol.md`, `internal/assets/agents/implement.md`, `AGENTS.md`. Depends on: T2, T3.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T5 ttl-recover-05-changelog [docs] Backfill CHANGELOG v0.5.3 through v0.5.7 and add v0.5.8 entry

Requirements: REQ-WAUTH-004

Files: `CHANGELOG.md`. Depends on: none.

Verification: git diff --check CHANGELOG.md