# Tasks: add-custom-providers

Same-board DAG mirrored 1:1 into control-plane board custom-providers (workflow sdd-lite, spec plane hybrid, workload policy flexible). Dependencies are same-board. Every verification command is raw and standalone; every oracle uses TestREQ naming per rule patterns/agent-testing-gates. Amended 2026-09-26: REQ-PROV-004 automatic twin-file reconciliation (backup-first, filemerge RemovePaths twin cleanup, dual-mutation receipt disclosure, restore-on-failure; unmanaged-winner refusal preserved). Wave-1 tasks cp-01-catalog and cp-02-state are done with PASS and are NOT amended; cp-03 through cp-06 pins re-bind to this amendment's artifact digests.

### Task: cp-01-catalog — providermgr catalog seed, load, and typed validation
- Requirements: REQ-PROV-001
- Test: TestREQ_PROV_001_* in internal/providermgr/catalog_test.go
- allowed_files: internal/providermgr/catalog.go, internal/providermgr/seed/nan.json, internal/providermgr/catalog_test.go
- dependencies: none (Wave 1; file-disjoint from cp-02-state)
- verification: go build ./... && go test -count=1 ./internal/providermgr -run '^TestREQ_PROV_001'
- Mutation evidence: run the mutation-testing probe over the seed-if-absent and each validation branch after GREEN; a SURVIVED outcome blocks the in_review transition until the covering test is strengthened
- Workload budget: Go source at most 350 LOC; test file at most 250 LOC; seed JSON is declarative data excluded from logic budgets

### Task: cp-02-state — additive Providers family in state v2
- Requirements: REQ-PROV-002
- Test: TestREQ_PROV_002_* in internal/state/provider_v2_test.go covering ProviderV2 normalize, validateCore, and omitempty round-trip only (digest and write-path oracles live in cp-03)
- allowed_files: internal/state/provider_v2.go, internal/state/metadata_v2.go, internal/state/provider_v2_test.go
- dependencies: none (Wave 1)
- verification: go build ./... && go test -count=1 ./internal/state -run '^TestREQ_PROV_002'
- Mutation evidence: probe normalize and validation branches; SURVIVED blocks transition to in_review
- Workload budget: source diff at most 250 LOC; test at most 250 LOC

### Task: cp-03-install — install.Service provider transaction with automatic twin reconciliation and secret-free digest
- Requirements: REQ-PROV-002, REQ-PROV-003, REQ-PROV-004
- Test: TestREQ_PROV_002_* and TestREQ_PROV_003_* in internal/install/provider_test.go (all models written, variants shape, comment preservation, ownership record, digest excludes token, dry-run inertness, reinstall convergence); TestREQ_PROV_004_* in internal/install/provider_conflict_test.go (backup-first reconciliation ordering for twin-only and divergent-twins cases, twin cleanup via RemovePaths, dual-mutation receipt disclosure with shared backup id, dry-run reconciliation disclosure with unchanged mtimes, fault-injected twin-edit and overlay failures restoring both files with state.json untouched, unmanaged-winner refusal with zero mutations, solo-twin no reconciliation planned)
- allowed_files: internal/install/provider.go, internal/install/provider_test.go, internal/install/provider_conflict_test.go
- dependencies: cp-01-catalog, cp-02-state (Wave 2)
- verification: go build ./... && go test -count=1 ./internal/install -run '^TestREQ_PROV_00[234]'
- Mutation evidence: probe the ownership-gate, reconciliation-ordering, restore-path, and overlay-construction branches; SURVIVED blocks transition to in_review
- Workload budget: source at most 600 LOC; each test file at most 250 LOC; temp home plus CORTEX_IA_HOME override always; synthetic sentinel tokens only

### Task: cp-04-screen — masked token input and providers phase machine
- Requirements: REQ-PROV-003, REQ-PROV-005
- Test: TestREQ_PROV_005_* (List renders catalog, Preview writes nothing and Escape rewinds, confirm yields Running phases, Receipt and rendered view contain no token characters, Error surface degrades) and TestREQ_PROV_003_* (bullet rendering, raw never rendered) in internal/tui/providers_screen_test.go
- allowed_files: internal/tui/masked_input.go, internal/tui/providers_screen.go, internal/tui/providers_screen_test.go
- dependencies: cp-03-install (Wave 3)
- verification: go build ./... && go test -count=1 ./internal/tui -run '^TestREQ_PROV_00[35]'
- Mutation evidence: probe phase-transition guards and masking branches; SURVIVED blocks transition to in_review
- Workload budget: source at most 550 LOC; test at most 250 LOC; seams override in tests (no real homes touched)

### Task: cp-05-home — Home 3-place registration and key scheme wiring
- Requirements: REQ-PROV-005, REQ-PROV-006
- Test: TestREQ_PROV_006_* lives in cp-06 (providers_home_test.go); this task keeps every existing TUI suite green (cursor clamps, boot, navigation) and compiles the openProviders path
- allowed_files: internal/tui/model.go, internal/tui/views.go, internal/tui/actions.go
- dependencies: cp-04-screen (Wave 4)
- verification: go build ./... && go vet ./internal/tui/... && go test -count=1 ./internal/tui
- Mutation evidence: probe the new key and selectHomeEntry branches; SURVIVED blocks transition to in_review
- Workload budget: combined source diff at most 120 LOC; numeric keys 1-9 frozen; never append to test files above 300 LOC

### Task: cp-06-flow — end-to-end key and flow regression oracle
- Requirements: REQ-PROV-001, REQ-PROV-005, REQ-PROV-006
- Test: TestREQ_PROV_006_* (hotkey p opens providers, numeric keys 1-9 unchanged, m still opens models, entry 9 cursor and enter navigable) and TestREQ_PROV_005_* (full List to Receipt flow over seams with temp home and CORTEX_IA_HOME override) in internal/tui/providers_home_test.go; pure-test task, never decomposed
- allowed_files: internal/tui/providers_home_test.go
- dependencies: cp-05-home (Wave 5)
- verification: go test -count=1 ./internal/tui -run '^TestREQ_PROV_00[156]' && go test -count=1 ./...
- Mutation evidence: full-suite green is required but insufficient; strengthen any covering test proven perpetually-green before approval
- Workload budget: single new test file at most 250 LOC

## Board-level review gates (not tasks)
- Live check R1: opencode models shows the installed provider, models, and variants after a real install (review phase, not a repo-side parser).
- Reconciliation audit R2: verify the cp-03 fault-injection oracles prove backup-before-twin-edit ordering and restore-on-failure; reconcile run leaves exactly one managed provider.nan.
- Token grep audit: sentinel token absent from receipts, state.json, logs, and fixtures (REQ-PROV-003).
- Full repo gate in hook order: gofmt -s -w ., go vet ./..., golangci-lint run ./..., go test -count=1 ./...

## Closure status (2026-09-26)
- 6/6 done with independent PASS verdicts. Durable review evidence: cp-01-catalog and cp-02-state approved at rev 3 (Cortex observation 71, 5/5 OpenSpec pin digests matched at Wave-1 review); cp-03-install approved at rev 5 with review_id 3bf80801ef218aee6716d078474bc20a (observation 76); cp-04-screen approved at rev 5 (observation 78; 5/5 KILLED mutants, mutation gate genuine); cp-05-home approved at rev 5 (observation 79; 5/5 KILLED via go test -overlay, no repo test file touched); cp-06-flow approved at rev 5 with review_id 496675c3e828e2b9864653f55f80d567, closing the board 6/6 (observation 69).
- Archive closure re-blocked at the durable fingerprint gate (verbatim error: workspace specification pin changed; fresh contract review required). Measured 2026-09-26 drift against current disk: cp-03 through cp-06 (rev 6) match disk on design/plan/proposal/spec and pin only a pre-Closure-status tasks.md digest da8b87c5; cp-01-catalog and cp-02-state (rev 4) additionally pin pre-amendment design.md 817afc7f and spec.md 3dcd761f. Resolution mechanics are unchanged but must now cover all six tasks and the final tasks.md bytes: per-task pin-only rebind via cortex-ia work revise --plan <file> to the current on-disk digests (cp-01/cp-02 to the same design/plan/proposal/spec/tasks digest set as the other four), then cortex-ia work review-refresh <task-id> --revision <current-revision> per task, then re-run cortex_ia_change_archive. Planner authority excludes work-item mutation, so the rebinds are orchestrator-executed; this section is the final tasks.md content and no further pre-closure edits to any pinned artifact are permitted.
- CLI-surface correction 1: no work-revise tool exists on any agent MCP surface (planner or orchestrator); work create rejects duplicate task ids. The only path is the CLI, and its exact invocation is cortex-ia work revise --plan <file-or-stdin>: task_id, expected_revision, and expected_status live INSIDE the plan JSON; the earlier instruction form shown as work revise <task-id> --revision <n> --plan ... is rejected by the app dispatcher as an unknown argument.
- CLI-surface correction 2: done tasks cannot be redefined; ReviseWorkDefinition accepts done tasks only for pin-only rebind (definition stays byte-identical, only SDD contract pins move to the current on-disk digests), and approved-binding coherence is restored afterwards via cortex-ia work review-refresh <task-id> --revision <current-revision> (or the orchestrator tool), which preserves every historical approval.
- Lineage note: observation 69's earlier planning-decision content was superseded in place by the cp-06 closure record (same topic_key); the full planning-decision lineage survives in observation 72 and this section.
