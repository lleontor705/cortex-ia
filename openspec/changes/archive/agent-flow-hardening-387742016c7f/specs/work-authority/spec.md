# Work Authority Hardening — spec delta

## ADDED Requirements

### Requirement: REQ-WA-001 — Review-FAIL retry circuit breaker
The work store SHALL refuse `RetryWork` on a blocked task when the task's `work_approvals` history contains two (2) consecutive FAIL verdicts, failing with a typed error that routes the task to planner decomposition (`cortex_ia_work_decompose`), except when the task is a pure-test/tooling task.

Consecutive definition: take all `work_approvals` rows for the task ordered by `created_at` then `id`; reduce them to the subsequence of PASS and FAIL verdicts only — BLOCKED and INCONCLUSIVE verdicts are ignored and neither count toward nor reset the streak (PASS-free gaps do not reset it). The breaker trips when the last two elements of that subsequence are both FAIL. A PASS resets the streak. Pure-test/tooling means every entry in the task's declared allowed_files matches test/tooling patterns (`*_test.go`, `*.test.*`, `test/**`, `scripts/tests/**`, mocks, fixtures); such tasks are exempt from decomposition routing and retry normally (fix or simplify assertions directly), subject to the existing attempt limit.

#### Scenario: two consecutive FAIL verdicts refuse retry
- **GIVEN** a blocked task whose approvals contain FAIL then FAIL, with any number of BLOCKED or INCONCLUSIVE verdicts in between
- **WHEN** `RetryWork` is invoked with the current revision
- **THEN** the retry is refused with an error naming the 2-FAIL circuit breaker and routing to planner decomposition, and the task remains blocked with its revision unchanged and no claims, leases, or review rows deleted

#### Scenario: PASS resets the FAIL streak
- **GIVEN** a blocked task whose approval history ends with a PASS after two earlier FAILs (for example after review refresh and re-approval)
- **WHEN** `RetryWork` is invoked
- **THEN** the circuit breaker does not trip and the task may return to ready when all other retry gates pass

#### Scenario: pure-test task is exempt from decomposition routing
- **GIVEN** a blocked task whose allowed_files are exclusively test files and whose approvals end with two FAIL verdicts
- **WHEN** `RetryWork` is invoked
- **THEN** the retry proceeds into ready (attempt limits still apply) and a `work_events` record notes the pure-test exemption

#### Scenario: breaker evaluated atomically with other retry gates
- **GIVEN** a blocked task with two consecutive FAILs and unresolved dependencies
- **WHEN** `RetryWork` is invoked
- **THEN** the refusal happens inside the same immediate transaction as the other gates, before any lease or claim deletion

### Requirement: REQ-WA-002 — Strict workload budget enforcement at transition
The work store SHALL be the single authoritative enforcement layer for workload budgets, evaluated inside `TransitionWork` when transitioning to `in_review` under `workload_policy=strict`: when computed churn exceeds the source or test budget, the transition is refused with `WORKLOAD_SOURCE_BUDGET_EXCEEDED` or `WORKLOAD_TEST_BUDGET_EXCEEDED`. Under `flexible`, churn never blocks and an advisory (`WORKLOAD_ADVISORY`) is attached to the transition event when guidelines are exceeded. Under `unbounded`, no churn computation occurs. Caps: source <= 350 LOC (Go/Rust/Java/C#) or <= 250 LOC (TS/Python, weighted deletions 0.2x) under strict and 700 / 500 under flexible; tests <= 600 under strict and 1200 under flexible; declarative data and schema files are exempt from both buckets.

#### Scenario: strict source budget exceeded refuses in_review
- **GIVEN** a strict task whose changed source files accumulate 800 weighted Go lines
- **WHEN** the implementer transitions the task to in_review
- **THEN** the transition is refused with WORKLOAD_SOURCE_BUDGET_EXCEEDED, the task stays in_progress, and no review row is created

#### Scenario: strict test budget exceeded refuses in_review
- **GIVEN** a strict task whose changed test files accumulate 700 lines
- **WHEN** transitioning to in_review
- **THEN** the transition is refused with WORKLOAD_TEST_BUDGET_EXCEEDED

#### Scenario: flexible overage is advisory only
- **GIVEN** a flexible task with 900 weighted Go source lines of churn
- **WHEN** transitioning to in_review
- **THEN** the transition succeeds and the recorded transition event carries a non-blocking WORKLOAD_ADVISORY detail

#### Scenario: unbounded bypasses churn computation
- **GIVEN** an unbounded task
- **WHEN** transitioning to in_review
- **THEN** no diff-size computation occurs and no workload refusal is possible

#### Scenario: non-git workspace fails closed under strict
- **GIVEN** a strict task whose workspace is not a git worktree
- **WHEN** transitioning to in_review
- **THEN** the transition is refused with WORKLOAD_BUDGET_UNVERIFIABLE rather than silently passing

### Requirement: REQ-WA-003 — Language-aware churn classification
Churn computation SHALL derive per-file addition/deletion counts from `git diff HEAD --numstat` in the task workspace, classify each changed file into a source-logic bucket or a test bucket by path and extension, weight TS/Python deletions at 0.2x, count Go/Rust/Java/C# additions and deletions at 1x, and exclude declarative data and schema files (JSON, YAML, TOML, SQL migrations, lockfiles) from both buckets.

#### Scenario: TS deletions are weighted at 0.2x
- **GIVEN** a strict TS task whose numstat shows 100 additions and 200 deletions in a .ts source file
- **WHEN** the budget is computed
- **THEN** the source churn is 140 (100 + 200 x 0.2)

#### Scenario: Go changes are weighted at 1x
- **GIVEN** a strict Go task with 200 additions and 100 deletions in .go source files
- **WHEN** the budget is computed
- **THEN** the source churn is 300

#### Scenario: declarative schema files are excluded
- **GIVEN** churn touching only .sql, .json, and .yaml files
- **WHEN** the budget is computed
- **THEN** both source and test churn are zero and no refusal occurs

#### Scenario: test files land in the test bucket
- **GIVEN** churn in internal/x/foo_test.go and web/src/bar.test.ts
- **WHEN** the budget is computed
- **THEN** those lines count against the test budget, not the source budget

### Requirement: REQ-WA-004 — Per-task workload policy storage
The work store SHALL persist `workload_policy` per task (`strict`, `flexible`, or `unbounded`, default `flexible`) via an additive, versioned SQLite migration, populated at task creation from the CLI (`cortex-ia work create --workload-policy`), read by the retry and transition paths, and preserved by recovery and decomposition.

#### Scenario: default policy is flexible
- **GIVEN** a task created without an explicit workload policy
- **WHEN** the task is read
- **THEN** workload_policy is flexible and strict-only enforcement never applies

#### Scenario: strict tasks carry the policy through the lifecycle
- **GIVEN** a task created with --workload-policy strict
- **WHEN** it is claimed, transitioned, recovered, or decomposed into children
- **THEN** workload_policy remains strict and the transition enforcement reads strict budgets

#### Scenario: migration is additive and fail-closed
- **GIVEN** an existing delegation.db at the previous schema version
- **WHEN** the store opens
- **THEN** the new migration adds the workload_policy column with default flexible, records the new schema_migrations version, and unknown future versions still fail closed
