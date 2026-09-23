# Integrated Plan: agent-flow-hardening (sdd-lite)

Workflow: sdd-lite · Phase: integrated · Spec plane: hybrid · Workload policy: flexible · Board: `agent-flow-hardening` · Change: `agent-flow-hardening`

## 1. Intent & Non-Goals

Harden the agent-flow along three approved audit gaps (Cortex observation #136): retire the external AGY execution path in full (GAP-03, user doctrine, FINAL), enforce the documented 2-consecutive-FAIL retry circuit breaker (GAP-05), and enforce language-aware workload budgets at the authoritative transition seam (GAP-06). Artifacts: proposal.md (why/what), specs/agy-execution/spec.md (REMOVED REQ-AGY-001..005), specs/work-authority/spec.md (ADDED REQ-WA-001..004), design.md (decisions/risks), tasks.md (18 active task nodes after the obs #152 reconciliation, the obs #161 micro-reconcile, and the obs #171 blocked-task reconciliation: T-AGY-6 decomposed into T-AGY-6a/6b, T-AGY-12 and chained pure-test T-AGY-13 added, and blocked T-AGY-3 decomposed into the merged atomic node T-AGY-3M plus zero-edit verification T-AGY-3V, with T-AGY-4 superseded to a zero-edit verification node).

Non-goals: no GAP-07 (speckit undecided); no edits under `internal/assets/agents/*` (gap-01 in flight); no destructive purge of AGY job history (DP-2); no native work-authority changes beyond REQ-WA-001..004; no bridge behavioral enforcement; the requirement set REQ-AGY-001..005 / REQ-WA-001..004 is final. These artifacts are FROZEN at this digest until archive (obs #171 reconciliation, final pass).

## 2. Requirements Summary

| ID | Delta | Subject | Covered by |
|---|---|---|---|
| REQ-AGY-001 | REMOVED | External AGY job execution engine (+herdr AGY references) | T-AGY-1/2/3M/10/11 |
| REQ-AGY-002 | REMOVED | AGY install target & CLI detection | T-AGY-6a/6b/7/8/11 |
| REQ-AGY-003 | REMOVED | AGY CLI hook surface | T-AGY-9/11 |
| REQ-AGY-004 | REMOVED | AGY configuration & model catalog | T-AGY-3M (executes) / T-AGY-4 (verifies) / T-AGY-11 |
| REQ-AGY-005 | REMOVED | Delegation job write-path (legacy read-only compat) | T-AGY-3M/5/12/13 |
| REQ-WA-001 | ADDED | Review-FAIL retry circuit breaker + pure-test exemption | T-CB-1 |
| REQ-WA-002 | ADDED | Strict workload budget enforcement at transition | T-WL-2 |
| REQ-WA-003 | ADDED | Language-aware churn classification | T-WL-2 |
| REQ-WA-004 | ADDED | Per-task workload policy storage | T-WL-1 |

## 3. Design Summary

Single authoritative enforcement layer in the Go store (`RetryWork`, `TransitionWork`); the bridge stays informational. Compile-green AGY removal: test prunes (incl. job write-path orphan suites, T-AGY-12, and the orphaned cancellationStore fixture, T-AGY-13) -> CLI dispatch removal (T-AGY-6a) -> targets symbol deletion (T-AGY-6b) -> merged engine+config atomic node (T-AGY-3M; obs #171 true compile 2-cycle runner.go <-> config.go) -> store write-path -> detection/hook/herdr (parallel) -> docs. Ordering follows the compile-coupling direction (obs #152, extended bidirectionally by obs #171): a node may not delete symbols still referenced, at compile time or through runtime default assertions, by files owned by a later node. Additive migration v15 for `work_items.workload_policy` (default `flexible`); job tables untouched (append-only read-only legacy). Churn from `git diff HEAD --numstat` with pure classifier `workload_budget.go`. Full rationale and risks in design.md.

## 4. Verification Strategy & Task DAG

Verification commands are raw executables; each task keeps its package green. Waves are parallel sets with mutually disjoint allowed_files; T-AGY-3M is the single orchestrator-approved multi-file exception (atomic merge, obs #171).

| Wave | Task | Deps | Allowed files | Verification |
|---|---|---|---|---|
| 1 | T-AGY-1 | — | internal/delegation/execution_environment_test.go, prompt_test.go, job_cancellation_test.go | go test ./internal/delegation -count=1 |
| 1 | T-AGY-2 | — | internal/delegation/store_test.go, implement_concurrency_test.go | go test ./internal/delegation -count=1 |
| 1 | T-AGY-12 | — | internal/delegation/job_query_test.go, job_reconciliation_test.go, workspace_concurrency_test.go | go test ./internal/delegation -count=1 |
| 1 | T-AGY-6a | — | internal/app/cli.go | go build ./... |
| 1 | T-AGY-6b | T-AGY-6a | internal/targets/agy.go, targets.go, targets_test.go | go test ./internal/targets -count=1 |
| 1 | T-AGY-8 | — | internal/clidetect/detector.go, detector_test.go | go test ./internal/clidetect -count=1 |
| 1 | T-AGY-9 | — | internal/app/hook.go, app.go | go build ./... |
| 1 | T-AGY-10 | — | internal/herdr/setup.go, setup_test.go | go test ./internal/herdr -count=1 |
| 1 | T-CB-1 | (gap-02 approval) | internal/delegation/work.go, work_retry_circuitbreaker_test.go | go test ./internal/delegation -run TestRetryWorkCircuitBreaker -count=1 |
| 2 | T-AGY-13 | T-AGY-12 (SQLite edge) | internal/delegation/job_cancellation_test.go | go test ./internal/delegation -count=1 |
| 2 | T-AGY-3M | T-AGY-1, T-AGY-2 (SQLite, done; normative wave gate after T-AGY-13) | internal/delegation/runner.go, execution_environment.go, job_cancellation.go, config.go, config_test.go, config_defaults_test.go, runner_baseline_test.go | go test ./internal/delegation -count=1 |
| 3 | T-AGY-3V | T-AGY-3M | internal/delegation/runner.go, config.go (zero-edit verification scope) | go build ./... |
| 3 | T-AGY-4 | T-AGY-3V (redirected edge) | internal/delegation/config.go, config_test.go, config_defaults_test.go (zero-edit verification scope) | go test ./internal/delegation -count=1 |
| 3 | T-AGY-5 | T-AGY-3V (redirected from T-AGY-3), T-AGY-12 + T-AGY-13 (normative wave gate) | internal/delegation/store.go | go test ./internal/delegation -count=1 |
| 4 | T-WL-1 | T-CB-1, T-AGY-5 | internal/delegation/store.go, work.go, internal/app/cli.go | go test ./internal/delegation -run TestWorkloadPolicy -count=1 |
| 5 | T-WL-2 | T-WL-1 | internal/delegation/work.go, workload_budget.go, work_workload_budget_test.go | go test ./internal/delegation -run TestWorkloadBudget -count=1 |
| 5 | T-AGY-11 | T-AGY-4, T-AGY-5, T-AGY-7, T-AGY-9, T-AGY-10 | AGENTS.md, internal/assets/skills/_shared/cortex-work-protocol.md, internal/assets/skills/_shared/workflow-map.md | go test ./internal/pipeline -count=1 |

Wave-gate note (obs #152 residual R1, obs #171 update): the engine lane now carries real SQLite edges — T-AGY-3M inherits T-AGY-1/T-AGY-2 (done), and T-AGY-4/T-AGY-5 were auto-redirected to T-AGY-3V by the decomposition downstream-redirect, chaining them to T-AGY-3M. Still normative at plan level because no planner tool can add edges to pre-existing nodes: T-AGY-13 completes before T-AGY-3M dispatch, and T-AGY-5 dispatches only after T-AGY-12 AND T-AGY-13 are done (T-AGY-12 done; T-AGY-13 ready). The orchestrator enforces the normative gates through wave discipline; violations surface as recoverable blocked -> decompose cycles.

### 4.1 Reconciliation (Cortex obs #152, #161, #171)

obs #152: T-AGY-6 was blocked by a compile-coupling ordering defect (dead-at-runtime is not dead-at-compile) and was decomposed in place into T-AGY-6a (dispatch removal first) -> T-AGY-6b (symbol deletion), with T-AGY-7 retained as a zero-edit verification node and T-AGY-12 added ahead of the job write-path nodes; details and residuals in tasks.md Reconciliation Notes. obs #161: the post-#152 micro-audit added chained pure-test T-AGY-13 for the orphaned cancellationStore fixture left by executed T-AGY-1.

obs #171 (T-AGY-3 blocked, zero edits): the one-directional audit had missed the reverse compile direction. A directional 2-cycle exists between runner.go and config.go — AGYModel (runner.go:1332) is consumed by config.go:54,71,88,105, supportsEffort (runner.go:1393) by config_test.go:149,151, changedWorktreePaths (runner.go:1002) by the unowned runner_baseline_test.go:46,48,65,67, while runner.go:420,438 consume config.go's DefaultAGYModel/skipPermissions — so no linear T-AGY-3 -> T-AGY-4 ordering compiles. Resolution (orchestrator-approved atomic merge over staged split): T-AGY-3 was decomposed in place into T-AGY-3M (merged engine+config removal owning seven files — including config_defaults_test.go, which the bidirectional runtime-assertion sweep added because deleting the agy role defaults breaks config_defaults_test.go:12 and config_test.go:49,52) and T-AGY-3V (zero-edit verification child required by the 2-step decompose mechanics). T-AGY-4 is superseded to a zero-edit verification node (T-AGY-7 precedent); its scope was absorbed by T-AGY-3M. Task-plane note: the pre-existing non-done nodes (T-AGY-4, T-AGY-5, T-AGY-11, T-AGY-13, T-CB-1, T-WL-1, T-WL-2) carry plan.md pins from before this final rewrite; each MUST have its plan pin revised to this final digest via `cortex-ia work revise` immediately before its own dispatch, because the review binding verifies workspace pins against current file digests. These artifacts are FROZEN at this digest until archive.

Cross-board constraint: `internal/delegation/work.go` is in_review under `gap-02-recover-inreview-guard` (board `default`, Cortex observation #141); T-CB-1, T-WL-1, and T-WL-2 dispatch only after that task is approved. The orchestrator sequences waves manually.

## 5. Decision Points (user)

- **DP-1 — herdr package fate** after AGY-reference removal (only consumer: web status display). Default: keep as optional diagnostics.
- **DP-2 — legacy job read path**: keep read-only display (default) vs remove endpoints vs user-approved destructive purge with verified backup/rollback.
- **DP-3 — `hook` command scope**: remove with AGY payload (default); implementer stops on shared non-AGY handling.
- **DP-4 — `RoleConfig` field shape**: keep inert shape for web display (default) vs prune later.

## 6. Sizing & Compliance

All task nodes are ≤ 3 tightly-coupled files (T-AGY-6a, T-AGY-6b, T-AGY-12, and T-AGY-13 included) with ONE orchestrator-approved exception: T-AGY-3M owns 7 files because runner.go <-> config.go form a true compile 2-cycle (obs #171) that no staged split can compile through; flexible policy keeps its expected churn well under the 700-LOC Go source cap because the node is dominated by deletions and test prunes. Single responsibility per node; dedicated modular test files ≤ 250 LOC (new files only; no appends to files > 300 LOC); verification fields raw. T-AGY-12 and T-AGY-13 are pure-test tasks and are exempt from decomposition by policy. Doctrine artifacts stay in English; direct operator communication matches the user's language.
