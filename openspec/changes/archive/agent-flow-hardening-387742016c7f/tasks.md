# Tasks: agent-flow-hardening

Dependency-ordered. Every task runs on board agent-flow-hardening with same-board dependency edges only. GAP-03 tasks are compile-green at every node (test prunes precede engine removal). GAP-05/06 tasks touching internal/delegation/work.go or store.go start only after gap-02-recover-inreview-guard (board default) is approved.

RECONCILIATION (Cortex obs #152, T-AGY-6 blocked): the original T-AGY-6 -> T-AGY-7 edge was compile-unsound -- internal/app/cli.go held live references to the very symbols T-AGY-6 deletes (dead-at-runtime is not dead-at-compile). T-AGY-6 was decomposed in place into T-AGY-6a (CLI dispatch removal, runs FIRST) -> T-AGY-6b (targets symbol deletion). T-AGY-7 is superseded by T-AGY-6a and remains only as a zero-edit verification node. The same defect-class audit added T-AGY-12 (AGY job write-path orphan suites) and produced the Reconciliation Notes at the end of this file. A post-reconciliation micro-audit (Cortex obs #161) added the chained pure-test task T-AGY-13 for the orphaned cancellationStore fixture (Reconciliation Notes, residual 4).

RECONCILIATION (Cortex obs #171, T-AGY-3 blocked): the obs #152/#161 audit was one-directional. Blocked T-AGY-3 exposed a true directional compile 2-cycle between runner.go and config.go, so no linear T-AGY-3 -> T-AGY-4 ordering compiles. Per the orchestrator-approved atomic merge over staged split, T-AGY-3 was decomposed in place into T-AGY-3M (merged engine+config removal, 7 files) -> T-AGY-3V (zero-edit verification child mandated by decompose mechanics); the downstream edges of T-AGY-4 and T-AGY-5 were auto-redirected to T-AGY-3V by the store. T-AGY-4 is superseded to a zero-edit verification node (T-AGY-7 precedent). After this pass the artifacts are FROZEN until archive; see Reconciliation Notes for the mandatory plan-pin refresh procedure at dispatch.

## 1. GAP-03 — AGY removal: test prunes (pure-test, parallel)

- [ ] T-AGY-1 Prune AGY-only test suites from execution-environment, prompt, and job-cancellation tests.
  Requirements: REQ-AGY-001
  Delete the suites exercising runAGY, buildAGYArgs, executionEnvironment, externalPrompt, CompleteWorker, and MarkTerminationUnconfirmed.
  Files: internal/delegation/execution_environment_test.go, internal/delegation/prompt_test.go, internal/delegation/job_cancellation_test.go
  Verification: go test ./internal/delegation -count=1
- [ ] T-AGY-2 Prune AGY suites from store and implement-concurrency tests.
  Requirements: REQ-AGY-001
  Remove TestParseModelsOutput, TestBuildAGYArgs suites, ReadRequest/CreateFromRequest suites, and the workspace-baseline/concurrency suites tied to AGY runners.
  Files: internal/delegation/store_test.go, internal/delegation/implement_concurrency_test.go
  Verification: go test ./internal/delegation -count=1
- [ ] T-AGY-12 Prune AGY job write-path suites from query, reconciliation, and workspace-concurrency tests.
  Requirements: REQ-AGY-005
  Added by the obs #152 defect-class audit: three test files outside the original prune wave still exercise the AGY job write-path. Delete or rework every suite in these files whose compile dependencies are store.Create, store.Claim, store.MarkRunning, store.ExtendJobLease, store.Cancel, or the NewJob literal; where read-path coverage must survive per the legacy read-only audit decision (DP-2: Query, Get, List, reconciliation views), reseed through direct SQL fixtures instead of the deleted write helpers. Sequencing requirement: this task must be approved before T-AGY-3 dispatch (store.Cancel dies with job_cancellation.go) and before T-AGY-5 dispatch (the write methods die).
  Files: internal/delegation/job_query_test.go, internal/delegation/job_reconciliation_test.go, internal/delegation/workspace_concurrency_test.go
  Verification: go test ./internal/delegation -count=1
- [ ] T-AGY-13 Prune the orphaned cancellationStore fixture from job_cancellation_test.go.
  Requirements: REQ-AGY-005
  Added by the obs #161 micro-reconciliation (line-verified by T-AGY-2, obs #160): T-AGY-1 removed the AGY cancellation suites but left the shared fixture helper cancellationStore, and the approved T-AGY-12 removed its last consumer (job_reconciliation_test.go reseeds through the direct-SQL reconciliationFixture, DP-2). The file now contains only the orphaned 17-line helper, which still compiles a NewJob literal that dies in T-AGY-5 (store.go only). Delete the orphaned cancellationStore helper and any residual NewJob/Store.Create references including the then-unused path/filepath import, keeping the file a minimal valid test file; it holds no suites today, so a bare package clause suffices. Pure-test task: exempt from decomposition by policy.
  Files: internal/delegation/job_cancellation_test.go
  Verification: go test ./internal/delegation -count=1
  Dependencies: T-AGY-12 (real SQLite edge; delegation lane is serial).

## 2. GAP-03 — AGY removal: merged engine+config, store (chained)

- [ ] T-AGY-3M Remove the AGY execution engine and the AGY config/model surface (merged T-AGY-3 + T-AGY-4 atomic node).
  Requirements: REQ-AGY-001, REQ-AGY-005 (REQ-AGY-004 scope absorbed from the superseded T-AGY-4)
  Reconciles the obs #171 directional compile 2-cycle: runner.go and config.go consume each other's symbols, so engine and config removal must be ONE atomic attempt. From runner.go delete the entire AGY execution engine: RunWorker, runAGY, buildAGYArgs, resolveAGY, externalPrompt, validateStructuredReceipt and helpers, isQuotaOrRateLimit, ErrQuotaExceeded, remoteFailureDiagnostics/remoteFailureClass, Request/ReadRequest/CreateFromRequest and Validate, AGYModel/ParseModelsOutput/ListAvailableModels/supportsEffort, captureWorkspaceBaseline/validateWorkspaceChanges/workspacePathFingerprint/changedWorktreePaths and related git-status helpers, keepAliveAuthorityAndJob/watchCancellation, limitedBuffer, and the AGY stream console rendering. Delete internal/delegation/execution_environment.go entirely and the AGY worker completion/cancellation wrappers (CompleteWorker, MarkTerminationUnconfirmed, ErrTerminationUnconfirmed) from job_cancellation.go. Delete TestChangedWorktreePaths_GitOperations from runner_baseline_test.go. From config.go delete DefaultAGYModel, KnownAGYModels, NextModel/PrevModel/ModelDisplayName, skipPermissions and the CORTEX_AGY_SKIP_PERMISSIONS gate, and the role defaults that set CLI agy; keep Load and the RoleConfig struct shape compiling for the read-only web delegation_config display (DP-4; do not touch internal/cortexiaweb). Prune the matching suites in config_test.go (TestSupportsEffort and the model-helper suites, the agy CLI assertions in TestSaveAndLoadConfig) and in config_defaults_test.go (TestDelegationDefaultsEnableUnattendedPermissions; rework TestLoadPreservesExplicitPermissionSetting only if the non-AGY role surface keeps it meaningful). The delegation package must keep compiling for the web console and dashboard read paths (Job, Receipt, Get/List) and for the work-authority store.
  Files: internal/delegation/runner.go, internal/delegation/execution_environment.go, internal/delegation/job_cancellation.go, internal/delegation/config.go, internal/delegation/config_test.go, internal/delegation/config_defaults_test.go, internal/delegation/runner_baseline_test.go
  Verification: go test ./internal/delegation -count=1
  Dependencies: T-AGY-1, T-AGY-2 (SQLite edges inherited from the superseded T-AGY-3, both done); normative wave gate after T-AGY-13.
- [ ] T-AGY-3V Verify the merged engine+config removal left no AGY references. (zero-edit verification node)
  Requirements: REQ-AGY-001, REQ-AGY-005
  Second link of the T-AGY-3 decomposition (obs #171): the store mandates a 2-8 step chain and redirects downstream dependencies to the last child, so this node carries the redirected T-AGY-4/T-AGY-5 edges. Dispatch only after T-AGY-3M is done; confirm by inspection that no deleted engine or config symbol (RunWorker, runAGY, AGYModel, KnownAGYModels, DefaultAGYModel, skipPermissions, supportsEffort, changedWorktreePaths, CompleteWorker, MarkTerminationUnconfirmed) is referenced anywhere under internal/delegation and that the whole tree builds; submit PASS with zero edits. Do not expand scope.
  Files: internal/delegation/runner.go, internal/delegation/config.go
  Verification: go build ./...
  Dependencies: T-AGY-3M (SQLite edge created by the decomposition).
- [ ] T-AGY-4 Remove the AGY config and model surface. (SUPERSEDED -- zero-edit verification node)
  Requirements: REQ-AGY-004
  Scope absorbed by the merged T-AGY-3M during the obs #171 reconciliation; its dependency edge was auto-redirected to T-AGY-3V, preserving the transitive gate from T-AGY-11 to the merged node. Dispatch only after T-AGY-3V is done; confirm DefaultAGYModel, KnownAGYModels, NextModel/PrevModel/ModelDisplayName, skipPermissions, and the agy role defaults no longer exist and that the package tests pass; submit PASS with zero edits. Do not expand scope.
  Files: internal/delegation/config.go, internal/delegation/config_test.go, internal/delegation/config_defaults_test.go
  Verification: go test ./internal/delegation -count=1
  Dependencies: T-AGY-3V (redirected from T-AGY-3 by the decomposition).
- [ ] T-AGY-5 Remove the AGY job write-path from the store.
  Requirements: REQ-AGY-005
  Ordering note (obs #152/#161/#171 audits): dispatch only after T-AGY-12 AND T-AGY-13 are approved (normative R1 gates; T-AGY-12 is done) and after the redirected chain T-AGY-3V -> T-AGY-3M completed (SQLite edge redirected from the superseded T-AGY-3). Delete Create, Claim, MarkRunning, ExtendJobLease, the NewJob literal, and orphaned job write helpers; keep Get/List reads and the job tables untouched (legacy read-only audit).
  Files: internal/delegation/store.go
  Verification: go test ./internal/delegation -count=1

## 3. GAP-03 — AGY removal: install, CLI, detection, hook, herdr (parallel)

- [ ] T-AGY-6a Remove the agy target dispatch from install, sync, and uninstall.
  Requirements: REQ-AGY-002
  First link of the T-AGY-6 decomposition (obs #152): the internal/targets AGY symbols are compile-coupled into cli.go, so the dispatch must die before the symbols. Delete the three case targets.TargetAGY blocks (install ~:135, sync ~:187, uninstall ~:1008) including the InstallAGY/UninstallAGY calls, their error wrapping, and the Antigravity receipt printing; update the --target flag error example (~:62) to drop the agy mention. The targets import stays used via ParseTargets/TargetOpenCode/TargetClaude/InstallClaude/UninstallClaude, so the package reference survives and the tree compiles. Bounded intermediate state until T-AGY-6b: --target agy still parses (registry intact) but silently no-ops because the switch has no default case; the fail-closed rejection lands in T-AGY-6b.
  Files: internal/app/cli.go
  Verification: go build ./...
- [ ] T-AGY-6b Delete the now-unreferenced AGY install target symbols and registry entries.
  Requirements: REQ-AGY-002
  Second link of the T-AGY-6 decomposition (obs #152): with the dispatch gone, TargetAGY, InstallAGY, UninstallAGY, AGYPluginName, and adaptAgentForAGY are referenced only inside internal/targets, and Go compiles unused package-level symbols, so the deletion is compile-green. Delete internal/targets/agy.go entirely; remove TargetAGY from targets.go (const declaration, the ParseTargets all-expansion, and the validation case) so ParseTargets rejects the agy id with an explicit error; prune the AGY install/uninstall suites and agy cases from targets_test.go.
  Files: internal/targets/agy.go, internal/targets/targets.go, internal/targets/targets_test.go
  Verification: go test ./internal/targets -count=1
- [ ] T-AGY-7 Remove the agy target dispatch from install, sync, and uninstall. (SUPERSEDED -- zero-edit verification node)
  Requirements: REQ-AGY-002
  Scope already executed by T-AGY-6a during the obs #152 reconciliation. Dispatch only after T-AGY-6b is done; confirm no targets.TargetAGY reference remains anywhere in internal/app and that the build is green; submit PASS with zero edits. Do not expand scope.
  Files: internal/app/cli.go
  Verification: go build ./...
- [ ] T-AGY-8 Remove AGY CLI detection.
  Requirements: REQ-AGY-002
  Delete CLIAGY/DetectAGY and their tests.
  Files: internal/clidetect/detector.go, internal/clidetect/detector_test.go
  Verification: go test ./internal/clidetect -count=1
- [ ] T-AGY-9 Remove the AGY hook surface.
  Requirements: REQ-AGY-003
  Delete hook.go, the hook dispatch case, and help text; add hook to retired commands. Stop and report blocked if shared non-AGY handling is found (DP-3).
  Files: internal/app/hook.go, internal/app/app.go
  Verification: go build ./...
- [ ] T-AGY-10 Remove herdr AGY references.
  Requirements: REQ-AGY-001
  Delete ResolveAGY, the antigravity-cli integration messaging, and AGY status lines from setup/status.
  Files: internal/herdr/setup.go, internal/herdr/setup_test.go
  Verification: go test ./internal/herdr -count=1

## 4. GAP-05 — Review-FAIL circuit breaker (after gap-02 approval)

- [ ] T-CB-1 Enforce the review-FAIL retry circuit breaker in RetryWork.
  Requirements: REQ-WA-001
  Inside the existing immediate transaction and before lease/claim deletion, read work_approvals ordered by created_at and id, reduce to the PASS/FAIL subsequence, and refuse when the last two are both FAIL with a typed error routing to planner decomposition (cortex_ia_work_decompose); exempt pure-test/tooling tasks (all allowed_files match test/tooling patterns) and record the exemption in work_events. Constraint: work.go is in_review under gap-02-recover-inreview-guard on board default -- dispatch only after that task is approved.
  Files: internal/delegation/work.go, internal/delegation/work_retry_circuitbreaker_test.go
  Verification: go test ./internal/delegation -run TestRetryWorkCircuitBreaker -count=1

## 5. GAP-06 — Workload policy enforcement (chained after T-CB-1 and T-AGY-5)

- [ ] T-WL-1 Add workload_policy storage and creation plumbing.
  Requirements: REQ-WA-004
  Append the next ledger version adding work_items.workload_policy TEXT NOT NULL DEFAULT flexible, extend WorkDefinition and creation reads/writes, and add cortex-ia work create --workload-policy with enum validation. Constraint: store.go and work.go are shared with gap-02 (board default) and T-AGY-5 -- start after both are approved/done.
  Files: internal/delegation/store.go, internal/delegation/work.go, internal/app/cli.go
  Verification: go test ./internal/delegation -run TestWorkloadPolicy -count=1
- [ ] T-WL-2 Enforce strict workload budgets in work transition.
  Requirements: REQ-WA-002, REQ-WA-003
  Add internal/delegation/workload_budget.go (numstat parsing, language-aware classification, weighted TS/Python deletions 0.2x, declarative exclusions, caps 350/700 and 250/500, tests 600/1200) and wire it into TransitionWork: strict refusals with WORKLOAD_SOURCE_BUDGET_EXCEEDED / WORKLOAD_TEST_BUDGET_EXCEEDED / WORKLOAD_BUDGET_UNVERIFIABLE, flexible WORKLOAD_ADVISORY event detail, unbounded bypass.
  Files: internal/delegation/work.go, internal/delegation/workload_budget.go, internal/delegation/work_workload_budget_test.go
  Verification: go test ./internal/delegation -run TestWorkloadBudget -count=1

## 6. Doctrine reconciliation (final)

- [ ] T-AGY-11 Reconcile AGY doctrine in protocol, workflow map, and agent guide.
  Requirements: REQ-AGY-001, REQ-AGY-002, REQ-AGY-003, REQ-AGY-004
  Remove role-table AGY leaf grants and the AGY workspace-strategy wording from cortex-work-protocol.md, the AGY leaf sentence from workflow-map.md, and AGY/Herdr supervision claims from root AGENTS.md; state that all execution is native. Dependency note (obs #171): the T-AGY-4 edge now points at the superseded zero-edit verification node that gates transitively on the merged T-AGY-3M; the T-AGY-7/9/10 deps are unchanged.
  Files: AGENTS.md, internal/assets/skills/_shared/cortex-work-protocol.md, internal/assets/skills/_shared/workflow-map.md
  Verification: go test ./internal/pipeline -count=1

## 7. Reconciliation Notes (obs #152 defect-class audit)

Defect class: a removal task whose deleted symbols still have live compile-time references in files owned by a LATER task breaks compilation at the earlier node. Dead-at-runtime is not dead-at-compile -- Go tolerates unused package-level symbols, never dangling references from switch cases or call sites. Dependency edges must follow the compile-coupling direction.

Audit results (live-reference check per remaining AGY-removal task):
- T-AGY-3 (engine): SOUND with T-AGY-12 sequenced first. Engine symbols are referenced only inside internal/delegation; no internal/app, internal/herdr, or internal/cortexiaweb callers exist (RunWorker already has zero production callers). job_query_test.go and workspace_concurrency_test.go reference store.Cancel, which dies here with job_cancellation.go -- covered by T-AGY-12. (REVISED by obs #171: the audit direction was incomplete -- see the blocked-task reconciliation below.)
- T-AGY-4 (config catalog): SOUND. Symbols live only in config.go (owned) and runner.go (T-AGY-3 deletes those references first) plus store_test.go (T-AGY-2 prune). No cortexiaweb or app references. (REVISED by obs #171: see the blocked-task reconciliation below.)
- T-AGY-5 (store write-path): SOUND with T-AGY-12 sequenced first. Create/Claim/MarkRunning/ExtendJobLease/NewJob have no production callers outside internal/delegation; the only referencing files are the three orphan test files owned by T-AGY-12.
- T-AGY-6a/6b (decomposed from blocked T-AGY-6): SOUND by construction -- dispatch removal first, symbol deletion second; import survival verified (cli.go keeps ParseTargets/TargetOpenCode/TargetClaude/InstallClaude/UninstallClaude).
- T-AGY-7: superseded by T-AGY-6a; retained as zero-edit verification.
- T-AGY-8 (in_review): edits already applied; CLIAGY/DetectAGY have no remaining references anywhere; predicted PASS.
- T-AGY-9 (in_progress): hook.go already deleted and the dispatch case removed; app.go retains only the desired retiredCommands hook entry; no shared non-AGY hook logic found, so the DP-3 STOP clause should not trigger; predicted PASS.
- T-AGY-10 (in_review): edits already applied; ResolveAGY and antigravity-cli references are gone; predicted PASS.
- T-AGY-11 (docs): markdown-only, no compile coupling; pipeline asset tests unaffected.

Micro-reconciliation (Cortex obs #161, post-#152): GAP -- T-AGY-1 removed the AGY suites from job_cancellation_test.go but left the shared fixture helper cancellationStore (the file is now 17 lines containing only that helper), which compiles a NewJob literal. Its sole consumer was removed by the approved T-AGY-12, and T-AGY-5 (allowed_files: store.go only) would break package compilation on this unowned file when deleting NewJob. Live-reference re-audit confirmed job_query_test.go, workspace_concurrency_test.go, and store_test.go no longer reference any job write-path symbol, making this file the last remaining NewJob reference in the package tests. Resolution: chained pure-test task T-AGY-13 created on the same board with allowed_files [job_cancellation_test.go] and a real SQLite dependency edge T-AGY-13 -> T-AGY-12 (delegation lane is serial; T-AGY-12 was already done, so T-AGY-13 is ready immediately).

Blocked-task reconciliation (Cortex obs #171; lessons from obs #170 D1, obs #161 R4/R5):
- Defect-class extension: the obs #152/#161 audit verified live references in ONE direction only (deleted symbols consumed by later files). T-AGY-3's blocked STOP clause (zero edits, baseline go build/go vet clean) proved the reverse direction too: AGYModel (runner.go:1332) is consumed by config.go:54,71,88,105; supportsEffort (runner.go:1393) by config_test.go:149,151; changedWorktreePaths (runner.go:1002) by runner_baseline_test.go:46,48,65,67, an unowned residual of the same class T-AGY-13 fixed for job_cancellation_test.go; while runner.go:420,438 consume config.go's DefaultAGYModel/skipPermissions. This is a directional 2-cycle, so NO linear T-AGY-3 -> T-AGY-4 ordering compiles. Audit lesson: removal audits must sweep BOTH compile directions AND runtime default assertions.
- Runtime-assertion sweep (new evidence, obs #171 class): deleting the agy role defaults breaks config_defaults_test.go:12 (asserts every role delegates with CLI agy and SkipPermissions) and config_test.go:49,52 (asserts loaded roles use CLI agy). config_defaults_test.go was therefore ADDED to the merged node's file set -- the blocked submission's compile-only grep could not see it.
- Resolution (orchestrator-approved atomic merge over staged split): T-AGY-3 (blocked, revision 5) was decomposed in place into T-AGY-3M (merged engine+config atomic node, 7 files, SDD contract version 1 per obs #161 R5, workflow label sdd-lite per obs #170 D1, pins plan.md@236c4f4e2ec44eaa5d1121e798098cbccf9458def845a2a5bfe178368715522a + specs/agy-execution/spec.md@feddda8e92092726ea221a2597d38284bf08524b33ad31508cbfdcc3fd8d09b6, requirement set [REQ-AGY-001, REQ-AGY-005] carried immutably per sameStringSet, work_revise.go:226) -> T-AGY-3V (zero-edit verification child; decompose mandates 2-8 steps and redirects downstream edges to the last child). The store auto-redirected the T-AGY-4 and T-AGY-5 dependencies from T-AGY-3 to T-AGY-3V.
- T-AGY-4 is superseded to a zero-edit verification node (T-AGY-7 precedent); its REQ-AGY-004 binding is preserved and its acceptance is discharged by T-AGY-3M. Requirement sets are immutable per sameStringSet, so T-AGY-3M carries [REQ-AGY-001, REQ-AGY-005] exactly and T-AGY-4 keeps [REQ-AGY-004].
- Plan-pin refresh procedure (MANDATORY at each dispatch): the pre-existing non-done nodes T-AGY-4, T-AGY-5, T-AGY-11, T-AGY-13, T-CB-1, T-WL-1, and T-WL-2 still carry plan.md pins from before this final rewrite (d03c8e12... or 39a10f85...). The review binding verifies workspace pins against the CURRENT file digest, so before each of these tasks is dispatched the orchestrator must run `cortex-ia work revise` for it, re-pinning plan.md to this final digest (spec pins unchanged, requirement sets unchanged). T-AGY-13's re-pin to the final digest is explicitly required at its dispatch per the obs #171 reconciliation directive. The planner MCP surface exposes no revise tool; the CLI command is authoritative.
- R4 sweep reminder (obs #161 R4, carried): internal/app/app.go:298 help text still lists agy among --target examples. app.go was leased by T-AGY-9; after T-AGY-6b the mention is fail-closed-cosmetic (ParseTargets rejects the id). Sweep it after T-AGY-9 approval via the T-AGY-11 envelope file exception or a trivial direct-change.
- Artifact freeze: plan.md and tasks.md at this pass's digests are FINAL until archive. Residual for the orchestrator: done-task approval bindings predate this digest, so archive will require the established RefreshWorkReview pass for every done task whose binding no longer matches current fingerprints (historical approvals are preserved; this is the documented archive flow).

Residuals for the orchestrator:
1. The T-AGY-12 -> T-AGY-3/T-AGY-5 ordering is normative here and in plan.md but could not be written into the SQLite dependency arrays of the pre-existing backlog tasks (no planner tool mutates existing task edges). Enforce via wave discipline; if violated, the minion STOP clause yields a recoverable blocked -> decompose cycle. (obs #171 update: T-AGY-3 is superseded; the live normative gates are T-AGY-13 -> T-AGY-3M and T-AGY-12 + T-AGY-13 -> T-AGY-5.)
2. internal/app/app.go:298 help text still lists agy among --target examples. app.go is leased by the in-flight T-AGY-9; after T-AGY-6b the mention is fail-closed-cosmetic (ParseTargets rejects the id). Sweep it after T-AGY-9 is approved -- orchestrator-authorized file exception in the T-AGY-11 envelope or a trivial direct-change.
3. Between T-AGY-6a and T-AGY-6b approvals, --target agy parses but no-ops (no switch default); both nodes live in the same board and the window closes with T-AGY-6b.
4. T-AGY-5 dispatch gate extended (obs #161): requires T-AGY-12 AND T-AGY-13 done. T-AGY-13 carries the real SQLite edge to T-AGY-12; the T-AGY-5 side of the gate stays orchestrator-sequenced (pre-existing backlog node, edge array immutable by planner tools). (obs #171 update: the engine-side gate is now a real SQLite chain, T-AGY-5 -> T-AGY-3V -> T-AGY-3M.)
