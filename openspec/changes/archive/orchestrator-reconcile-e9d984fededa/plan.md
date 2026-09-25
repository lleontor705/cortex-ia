# Orchestrator Work Reconcile & Bridge Deference — integrated plan (sdd-lite)

Change: `orchestrator-reconcile` · Board: `orchestrator-reconcile` · Spec plane: hybrid · Workload policy: flexible · Evidence: Cortex obs 176, 184, 152

## Intent

Give the orchestrator a bounded, auditable decision tool over orphaned work authority instead of forcing it to idle until TTL expiry. Incident evidence (obs 184): a controller lost its only copy of the claim token (PowerShell splatted `--claim-token @stdin`), the durable claim stayed `in_progress` with a 60m TTL, and `work recover` only reconciles EXPIRED claims, so the claim was undiagnosable-but-unactionable: `durable_owner_matches_bridge=false`, owner session dead, no tool to act. Independently (obs 176): the cortex-work bridge retained a stale in-memory authority handle after durable recovery and refused fresh claims with "authority already held by this controller" even though `durable_claim_live=false`. This change adds `cortex-ia work reconcile <task-id> --reason "<text>" ...` (orchestrator-scoped, fail-closed force-release of a live-but-orphaned claim into `blocked` or directly `ready`), plus the companion bridge fix so durable state is always authoritative over stale in-memory handles. Ratified by the user directive: "dar una herramienta al orquestador para que pueda decidir y poder reiniciar".

Non-goals: no token resurrection or re-issue path for live claims; no permission bypass or unsafe-flag default; no auto-restart of OpenCode or of controllers; no change to `work recover`, `work retry`, `work claim`, `work approve`, or TTL semantics; no web-console or remote-reachable surface (loopback/local only, per project security invariants); no plaintext tokens in any receipt, event, log, or memory; no weakening of cross-initiative lease protection (a live foreign session's claim must stay untouchable).

## Requirements

Authoritative deltas with RFC 2119 keywords and Given/When/Then scenarios live in `specs/work-authority/spec.md`:

- REQ-RECON-001 Fail-closed force-release of live orphaned claims via `work reconcile` (release conditions, `--to blocked|ready`, expired claims remain `recover`'s domain)
- REQ-RECON-002 Append-only audit event and structured JSON receipt with decision-input snapshot and zero plaintext tokens
- REQ-RECON-003 Bridge deference: a stale in-memory authority handle never blocks a fresh claim when durable state shows no matching live claim
- REQ-RECON-004 Orchestrator-only MCP surface `cortex_ia_work_reconcile` with identical fail-closed semantics and host-attested session identity

## Design

- **Reconcile service** (new `internal/delegation/work_reconcile.go`, no logic in the dispatcher): `Store.ReconcileWork(ctx, ReconcileInput{TaskID, Reason, HostSessionID, ExpectedRevision, To, OwnerSessionInactive}) (ReconcileResult, error)` inside one `BEGIN IMMEDIATE` transaction (mirroring `RecoverWork`/`RetryWork`). Eligibility: the task is `in_progress` with a LIVE durable claim (`work_claims.expires_at > now`); expired claims are `work recover`'s domain. Refusal conditions (ALL must pass, otherwise `ErrWorkReconcileRefused` names the failed codes and nothing mutates): `reason_missing` (non-empty trimmed reason, bounded), `current_session_owner` (`claim.owner` must not equal `opencode-session:<HostSessionID>` — the current-session protection), `claim_fresh_no_inactivity_evidence` (staleness: `claim.updated_at` — bumped only by `RenewWorkClaim` heartbeats — is at most `workReconcileStaleWindow = 15 * time.Minute` old AND `OwnerSessionInactive` is false; 15m mirrors the bridge `maintenancePolicy.stale_progress_ms`), `revision_mismatch` (`ExpectedRevision` positive and equal to `work_items.revision`), `status_not_in_progress`, `task_not_found`, `claim_not_live`, `attempt_limit` (only for `To=ready`, mirroring `MaxWorkAttempts`). Release effect: capture the `work_leases.path` set, delete the task's `work_claims` and `work_leases` rows (mirroring `RecoverWork`), CAS `UPDATE work_items SET status=<to>, revision=revision+1`, append `work_events` kind `reconciled` whose detail is a JSON decision-input snapshot (actor host session, reason, durable owner, attempt, expires_at, last_renewed_at, staleness window, owner_session_inactive evidence bit, revision before/after, released lease paths) — token hashes are never written into events. `blocked` output is `work retry`-eligible today with no changes to retry semantics.
- **CLI wiring** (`internal/app/work.go` + one help line in `internal/app/app.go`): grammar `cortex-ia work reconcile <task-id> --reason <text> --session <host-session-id> --revision <n> [--to ready] [--owner-session-inactive <true|false>]`; parsed with the existing `workIDOptions`/`positiveRevision` discipline, dispatched straight to `ReconcileWork`, rendered via `printJSON` (structured receipt) plus one human summary line; unknown/missing options fail with the usage line. `--owner-session-inactive` is an evidence input: only the bridge sets it from real host session tracking (see below); a lying local caller is inside the existing local trust root (it can already pass any `--owner`), which is why the flag is recorded verbatim in the audit snapshot.
- **Bridge deference fix** (`internal/assets/plugins/cortex-work.ts`): in `cortex_ia_work_claim.execute`, the unconditional `workAuthority.has(task_id)` throw becomes: probe `durableWorkStatus(task_id)`; if the retained handle's claim is not durable-live or the durable owner no longer matches `controllerIdentity(handle.sessionID)`, `stopMaintenance(handle, "durable_deference")`, delete the stale handle, and continue to the fresh claim; only a still-live durable claim owned by the same bridge session may throw "already held by this controller"; a failed durable probe keeps the refusal (fail closed).
- **MCP surface** (same file): new tool `cortex_ia_work_reconcile {task_id, reason, revision, to?}` registered in `bridgeTools` and classified `work_reconcile: ["orchestrator"]` in the `mutations` role gate (unclassified tools fail at load, so the gate is mandatory). Execute derives `--session` exclusively from `context.sessionID` (never an argument), derives the inactivity evidence ONLY from the plugin's own host tracking (`activeSubagents`: the durable claim's owner session is a known host session that is no longer active → evidence true; otherwise false), invokes the CLI verb, and returns the durable JSON receipt untouched. Expired claims still refuse with `claim_not_live`, so `recover` ownership is unchanged.
- **Tests (persistent, whitelisted authority/transport boundaries only)**: `internal/delegation/work_reconcile_test.go` (fail-closed matrix: live current-session claim → refuse; stale dead-session claim → release to blocked with event; fresh foreign claim without evidence → refuse; CAS mismatch → refuse; missing reason → refuse; `--to ready` + attempt limit; expired claim → refuse; no tokens in event detail) on the existing recovery-fixture/temp-home pattern, ≤250 LOC. `scripts/harness-work-reconcile.test.mjs` (bridge: stale-handle deference permits fresh claim; genuinely-held live claim still refuses; reconcile tool argv/identity/role-gate matrix) on the `harness-authority.test.mjs` loader-mock pattern. `internal/app/work_reconcile_smoke_test.go` + `scripts/harness-reconcile-incidentsmoke.test.mjs` reproduce the incident end-to-end (claim → owner session dies → reconcile → retry → fresh claim) through the real CLI (`runWork`, `CORTEX_IA_HOME` temp home) and through the mocked bridge seam.

## Verification

Raw gates (per task in `tasks.md`):

```
go vet ./internal/delegation/... ./internal/app/...
node --test scripts/harness-authority.test.mjs
go test -count=1 ./internal/delegation -run '^TestReconcileWork'
node --test scripts/harness-work-reconcile.test.mjs
go test -count=1 ./internal/app -run '^TestWorkReconcileIncidentSmoke$'
node --test scripts/harness-reconcile-incidentsmoke.test.mjs
```

Before any archive: full local gate `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`, plus one independent reviewer with a security lens (authority-transition correctness, audit completeness, absence of bypass paths) runs the AST delta ingestion and `cortex_detect_cycles` before PASS, per the SDD lifecycle.

## Tasks

DAG on board `orchestrator-reconcile` (wave 1: `task-recon-delegation` ∥ `task-recon-bridge` — disjoint Go/TS files; wave 2: `task-recon-tests` ∥ `task-recon-bridge-tests`; wave 3: `task-recon-integration`). Full contracts in `tasks.md`:

- task-recon-delegation [delegation+app] ReconcileWork service + `work reconcile` CLI verb — internal/delegation/work_reconcile.go, internal/app/work.go, internal/app/app.go — deps: none
- task-recon-bridge [cortex-work] durable deference for stale handles + `cortex_ia_work_reconcile` tool — internal/assets/plugins/cortex-work.ts — deps: none
- task-recon-tests [delegation] persistent fail-closed reconcile matrix — internal/delegation/work_reconcile_test.go — deps: task-recon-delegation
- task-recon-bridge-tests [harness] bridge deference + reconcile tool gate matrix — scripts/harness-work-reconcile.test.mjs — deps: task-recon-bridge
- task-recon-integration [smoke] incident end-to-end reproduction through CLI and bridge seams — internal/app/work_reconcile_smoke_test.go, scripts/harness-reconcile-incidentsmoke.test.mjs — deps: task-recon-tests, task-recon-bridge-tests
