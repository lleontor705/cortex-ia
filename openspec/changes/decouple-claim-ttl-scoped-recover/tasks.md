# Tasks — Decouple Claim TTL from Stale-Progress Window & Scoped Self-Recovery for Implement

Dependency-safe DAG on board `ttl-recover-release`; workload policy `flexible`; all tasks within budget (no `WORKLOAD_ADVISORY` expected). Review policy: T2 and T3 touch authority boundaries → mandatory independent reviewer; T1 is pure-test (never decompose on failure — fix assertions directly); T4 and T5 are docs → orchestrator auto-approval per the Adaptive Review Policy.

### T1 ttl-recover-01-contracts [work-authority] Harness tests for decoupled TTL and scoped recover guard

Requirements: REQ-WAUTH-001, REQ-WAUTH-002, REQ-WAUTH-003

Files: `scripts/harness-controller-authority.test.mjs` (206 LOC; appending allowed, keep additions modular). RED-first: encode `ttl !== "15m"` and non-derivation from `stale_progress_ms` (TestREQ_WAUTH_001); add scoped-recover cases (TestREQ_WAUTH_002): owning implement session recovers via `work recover --task --owner`; mismatched session throws with no CLI call; other roles rejected by the mutation gate; successful recover stops maintenance and drops the handle. Keep every existing orphan-guard assertion intact (stale-progress stop, no indefinite renewal, dispose/terminal-event stops). Depends on: none.

Verification: node --test --test-name-pattern TestREQ scripts/harness-controller-authority.test.mjs

### T2 ttl-recover-02-plugin [work-authority] Decouple maintenancePolicy TTL and add cortex_ia_work_recover_own

Requirements: REQ-WAUTH-001, REQ-WAUTH-002

Files: `internal/assets/plugins/cortex-work.ts`. Set `maintenancePolicy.ttl = "30m"` (stale_progress_ms unchanged); add the new tool and `work_recover_own: ["implement"]` classification per Design. Depends on: T1 (tests exist and define the contract).

Verification: node --test scripts/harness-controller-authority.test.mjs

### T3 ttl-recover-03-cli-store [delegation] Scoped recover in CLI and store with fail-closed ownership

Requirements: REQ-WAUTH-003

Files: `internal/app/work.go`, `internal/delegation/work.go`, `internal/delegation/recovery_test.go` (if recovery_test.go exceeds 300 LOC, create modular `internal/delegation/recovery_scoped_test.go` <= 250 LOC instead). Add scoped flags plus `RecoverWorkScoped` with typed fail-closed denial (TestREQ_WAUTH_003); shared per-task helper refactor; tests for owner-match success, owner mismatch, not-expired, not-found, flag-validation. Depends on: none (CLI contract is self-contained; the plugin slices against it in parallel).

Verification: go test -count=1 ./internal/delegation/... ./internal/app/...

### T4 ttl-recover-04-protocol-docs [protocol] Codify scoped recover authority and decoupled TTL in protocol and agent assets

Requirements: REQ-WAUTH-004

Files: `internal/assets/skills/_shared/cortex-work-protocol.md`, `internal/assets/agents/implement.md`, `AGENTS.md`. Split the authority-table row (`cortex_ia_work_recover` and `cortex_ia_work_retry` stay orchestrator-only; `cortex_ia_work_recover_own` is implement-scoped to own task, own session identity, fail-closed otherwise); update the implement agent guard (`resource: "cortex-ia work recover*"`) to allow only the scoped own-task form; update the root AGENTS.md role matrix lines for implement/orchestrator; document TTL 30m vs. the unchanged 15-minute stale-progress orphan window. Depends on: T2, T3 (documents shipped behavior, not intent).

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T5 ttl-recover-05-changelog [docs] Backfill CHANGELOG v0.5.3 through v0.5.7 and add v0.5.8 entry

Requirements: REQ-WAUTH-004

Files: `CHANGELOG.md`. Derive one keep-a-changelog section per tag v0.5.3..v0.5.7 from `git log v0.5.3..v0.5.7 --oneline` (feat/fix/refactor/perf prefixes; include short hashes; v0.5.6 must credit heartbeat fix `7e51860`). Add the v0.5.8 section covering this change (TTL decoupling, scoped self-recovery, protocol/asset updates) with an `(unreleased)` marker removable at tag time. Depends on: none (independent content; only ordering requirement is landing before the orchestrator's release step).

Verification: git diff --check CHANGELOG.md