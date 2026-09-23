# Design: agent-flow-hardening

## Context

All controllers execute natively; external AGY delegation is doctrine-retired but its code remains (`internal/delegation/runner.go` engine with zero production callers, retired `delegate`/`herdr` commands, active `--target agy` install surface and `hook` command). The work-authority store (`internal/delegation`, SQLite STRICT + WAL + versioned migration ledger reaching v14) already keeps append-only `work_approvals` verdict history that survives retries (`RetryWork` deletes `work_reviews` but not `work_approvals`), which is the exact seam GAP-05 needs. `workload_policy` exists today only in the AGY-only `Request` struct. The bridge `cortex_ia_work_transition` (internal/assets/plugins/cortex-work.ts:1305-1344) is a thin CLI passthrough. Cross-board constraint: `internal/delegation/work.go` is in_review under `gap-02-recover-inreview-guard` (board `default`, fix in Cortex observation #141); `internal/assets/agents/*` is frozen for `gap-01-shell-permission-model`.

## Goals / Non-Goals

**Goals:**
- Remove every AGY-sole-purpose surface while keeping every DAG node compile-green and every persistent critical-boundary test suite coherent.
- Enforce the documented 2-FAIL retry circuit breaker inside the store's immediate transaction.
- Make workload budgets real, language-aware, and authoritative at the transition seam, with per-task policy persistence.

**Non-Goals:**
- No GAP-07 (speckit undecided). No edits under `internal/assets/agents/*`.
- No destructive data purge of AGY job history (DP-2 governs).
- No native work-authority behavior changes beyond REQ-WA-001..004.
- No bridge behavioral enforcement (informational only).

## Decisions

1. **Go store is the single authoritative enforcement layer** (GAP-06, and GAP-05 by construction). `Store.TransitionWork` and `Store.RetryWork` are on every path — direct CLI and bridge alike — so enforcement cannot be bypassed and is not duplicated. Alternative considered: enforce in `cortex-work.ts` (rejected: CLI users bypass the bridge; two implementations drift). The bridge keeps documenting `workload_policy` as informational and forwards task metadata unchanged.
2. **Test-prune-first DAG ordering for the AGY removal.** Go compiles test files with the package, so removing product symbols while AGY-referencing tests still exist would break `go test ./internal/delegation` at intermediate DAG nodes. The removal therefore runs: prune AGY-only tests (T-AGY-1, T-AGY-2, plus the job write-path orphan suites T-AGY-12) -> remove engine (T-AGY-3) -> config surface (T-AGY-4) -> store job write-path (T-AGY-5). Reconciliation (obs #152): compile coupling is directional — a task may not delete symbols still referenced by files owned by a later task (dead-at-runtime is not dead-at-compile), so the AGY install-target removal was re-ordered to dispatch-removal-first (T-AGY-6a) then symbol deletion (T-AGY-6b).
3. **Legacy read-only compat for AGY job data.** The migration ledger stays untouched for job tables; `delegation_jobs`/`delegation_receipts`/`delegation_events` remain append-only audit history with their read paths (`Get`, `List`, web `getDelegation`, dashboard views). Write methods die with the engine. Destructive purge only via DP-2 with verified backup and exact rollback.
4. **Additive migration for workload policy.** Next ledger version (`if version < 15`): `ALTER TABLE work_items ADD COLUMN workload_policy TEXT NOT NULL DEFAULT 'flexible'`; the write layer validates the enum (STRICT tables cannot express the CHECK via ALTER). Default `flexible` preserves today's behavior for every existing and unflagged task.
5. **Churn source:** `git diff HEAD --numstat` (format `added\tdeleted\tpath`, binary entries show `-` and are skipped), computed in the task workspace over the task's leased files. Non-git workspaces fail closed under strict (`WORKLOAD_BUDGET_UNVERIFIABLE`), matching the protocol's preflight spirit. Classification is a pure function in a new `internal/delegation/workload_budget.go` for cheap unit testing.
6. **Consecutive-FAIL definition (GAP-05)** lives in the spec, not in prose: ordered by `created_at, id`; reduce to PASS/FAIL subsequence; BLOCKED/INCONCLUSIVE ignored; last two both FAIL imply refuse. Exemption predicate reads the task's declared allowed_files (persisted in `work_definitions.allowed_files_json`).
7. **Compile-green file batching.** Each removal task owns the exact files whose symbols it deletes, and test-prune tasks precede their product counterparts; parallel waves are chosen so no two ready tasks share a file.

## Risks / Trade-offs

- [Test files referenced by removed symbols fail compilation] -> DAG ordering (tests pruned before engine removal; obs #152 added T-AGY-12 for the job write-path orphan suites) plus per-task `go test` verification.
- [`work.go`/`store.go` contention with gap-02 on board `default`] -> GAP-05/06 tasks chained after T-CB-1 and T-AGY-5 respectively; objectives carry the explicit sequencing constraint; orchestrator gates dispatch on gap-02 approval.
- [`hook.go` may contain shared non-AGY handling] -> DP-3: implementer stops and reports blocked instead of expanding scope.
- [Web `delegation_config` display references config fields] -> T-AGY-4 removes only symbols without remaining consumers; final pruning deferred to DP-4.
- [Strict enforcement surprises tasks created before the feature] -> default policy `flexible`; only tasks explicitly created strict are subject to blocking.
- [Numstat churn includes pre-existing dirty worktree lines] -> proportional to leased paths and consistent with the protocol's `git diff --numstat` preflight; reviewer proportionality rules cover residual noise.

## Migration Plan

1. Materialize the reconciled 15-node DAG on the `agent-flow-hardening` board (obs #152 decomposition); no product edits until tasks are claimed.
2. AGY removal waves 1-4 (pure-test prunes -> dispatch removal and targets deletion -> engine -> config/store -> install/CLI/detection/hook/herdr) with docs reconciliation last.
3. GAP-05 (`T-CB-1`) and GAP-06 (`T-WL-1`, `T-WL-2`) chained on shared files after gap-02 approval.
4. Rollback: each task is an independent reversible diff; the additive migration requires no data rollback; no destructive operations anywhere in the plan.

## Open Questions -> Decision Points (user)

- **DP-1 — herdr package fate.** After AGY references are removed (T-AGY-10), the remaining herdr surface (install, integration, status) has exactly one consumer: the web console status display (`internal/cortexiaweb/server.go:253-254`). Options: keep as optional diagnostics (default) vs remove the package and web status fields in a follow-up.
- **DP-2 — legacy job read path.** Keep web/dashboard read-only display (default), remove the job endpoints, or perform a user-approved destructive purge with verified backup + exact rollback.
- **DP-3 — `hook` command scope.** Remove with the AGY payload (default). Implementer must stop and report blocked if shared non-AGY handling is discovered inside `hook.go`.
- **DP-4 — `RoleConfig` field shape.** After AGY defaults die: keep the inert shape for web display (default) vs prune fields in a follow-up aligned with DP-2.
