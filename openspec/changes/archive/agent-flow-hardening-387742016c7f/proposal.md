# Proposal: agent-flow-hardening

## Why

The agent-flow audit (Cortex observation #136, topic `architecture/agent-flow-audit`) identified three approved gaps:

1. **GAP-03 — Kill AGY (user doctrine decision, FINAL).** The external AGY (Antigravity CLI) execution path is doctrine-retired (`cortex-work-protocol.md` §5: "External AGY delegation has been retired") yet its engine, job write-path, install target, CLI detection, hook surface, config model catalog, herdr references, and documentation remain shipped in the product. The `delegate` and `herdr` CLI commands are already in `retiredCommands` (internal/app/app.go:244-245), which leaves the execution engine (`RunWorker`, `runAGY`, `buildAGYArgs`, `resolveAGY`, `externalPrompt`, receipt validation, quota classification) as dead code with zero production callers.
2. **GAP-05 — Review-FAIL circuit breaker.** `cortex-work-protocol.md` §4 mandates that a task with two (2) consecutive review FAIL verdicts MUST NOT be retried and must route to planner decomposition. `Store.RetryWork` (internal/delegation/work.go:1139-1193) enforces the five-attempt limit but never inspects `work_approvals` verdict history.
3. **GAP-06 — Workload policy enforcement.** Strict workload budgets are documented in the protocol and mirrored in the bridge, yet no authoritative layer enforces them at transition time and `workload_policy` is not even persisted per task (only the AGY-only `Request` struct carried it, and it dies with GAP-03).

## What Changes

- **BREAKING** Remove the external AGY execution path end-to-end (eight stacked tasks): AGY-only test suites pruned first (keeps the package compile-green), then the execution engine, then the AGY config/model surface and the AGY job write-path, then the `--target agy` install surface, CLI dispatch cases, AGY CLI detection, the AGY hook payload surface, and herdr AGY references, then doctrine docs reconciliation.
- **Data doctrine — legacy read-only compat:** `delegation_jobs`, `delegation_receipts`, and `delegation_events` remain as append-only historical audit records. The AGY job write methods are removed; read paths used by the web console (`GET /api/delegations/{id}`) and the dashboard stay. No destructive purge without an explicit user decision (DP-2).
- **GAP-05:** `Store.RetryWork` refuses retry when the task's `work_approvals` history ends in two consecutive FAIL verdicts (precise definition in `specs/work-authority/spec.md` REQ-WA-001), failing with a typed error that routes to planner decomposition. Pure-test/tooling tasks are exempt (fix or simplify assertions directly; never decompose tests).
- **GAP-06:** `workload_policy` is persisted per task via an additive versioned SQLite migration (default `flexible`), settable at creation (`cortex-ia work create --workload-policy`). `Store.TransitionWork` becomes the single authoritative enforcement layer: under `strict`, transitions to `in_review` are refused with `WORKLOAD_SOURCE_BUDGET_EXCEEDED` / `WORKLOAD_TEST_BUDGET_EXCEEDED` when language-aware churn exceeds the caps; `flexible` stays advisory; `unbounded` bypasses. The bridge (`cortex-work.ts`) remains a passthrough and documents the policy as informational.

## Capabilities

- `specs/agy-execution/spec.md` — REMOVED requirements: REQ-AGY-001 execution engine, REQ-AGY-002 install target & CLI detection, REQ-AGY-003 hook surface, REQ-AGY-004 config & model catalog, REQ-AGY-005 job write-path.
- `specs/work-authority/spec.md` — ADDED requirements: REQ-WA-001 retry circuit breaker, REQ-WA-002 strict workload enforcement at transition, REQ-WA-003 language-aware churn classification, REQ-WA-004 per-task workload policy storage.

## Impact

- **Code:** `internal/delegation` (runner.go, execution_environment.go, job_cancellation.go, store.go, config.go + tests), `internal/targets` (agy.go, targets.go + tests), `internal/clidetect` (detector.go + tests), `internal/app` (cli.go, hook.go, app.go), `internal/herdr` (setup.go + tests), `internal/assets/skills/_shared` (cortex-work-protocol.md, workflow-map.md), root `AGENTS.md`.
- **Data:** one additive migration (`work_items.workload_policy`, default `flexible`) appended to the versioned ledger (currently reaches v14); unknown future versions keep failing closed. No destructive change to AGY job tables.
- **Sequencing:** `internal/delegation/work.go` is currently in_review under `gap-02-recover-inreview-guard` on board `default` (fix recorded in Cortex observation #141). GAP-05/06 tasks touching `work.go`/`store.go` start only after that task's approval; the orchestrator sequences waves manually. `internal/assets/agents/*` is frozen (`gap-01-shell-permission-model` in flight) and is not touched by any task.

## Non-Goals

- No GAP-07 (speckit decision still undecided by the user).
- No product-code edits or task claims by the planner; this change only specifies and materializes the DAG.
- No destructive purge, table drop, or row deletion of AGY job history without an explicit user-approved decision (DP-2).
- No native work-authority behavior changes beyond REQ-WA-001..004 (no new transition states, no new approval semantics).
- No behavioral enforcement in the bridge (`cortex-work.ts`); it stays informational.
- No edits under `internal/assets/agents/*`.
