# Tasks

- [ ] ljr-1.1 Add audited prior-boot lost-job reconciliation
Requirements: REQ-RECON-001, REQ-RECON-002, REQ-RECON-003

Dependencies: none. One coherent authority/recovery slice; independent reviewer follows implementation. Workload policy: flexible.

Allowed files:
- internal/delegation/store.go
- internal/delegation/job_reconciliation.go
- internal/delegation/process_identity_windows.go
- internal/delegation/process_identity_other.go
- internal/delegation/job_reconciliation_test.go
- internal/app/delegation.go
- internal/assets/plugins/herdr-bridge.ts
- internal/assets/agents/orchestrator.md
- scripts/harness-cancellation.test.mjs

Verification:
```
go test ./internal/delegation/... ./internal/app/... -count=1
go vet ./internal/delegation/... ./internal/app/...
node --test scripts/harness-cancellation.test.mjs scripts/harness-authority.test.mjs
go build -o bin/cortex-ia ./cmd/cortex-ia
git diff --check
```

Use Go 1.26.1. New persistent authority test file remains <=250 lines; no new suites in oversized files. Reviewer checks schema upgrade/rollback behavior, stale proof races, actual bridge role filtering and independent test outcomes. Archive only after independent PASS and current fingerprints. Live operational recovery and independent HTTP 401 investigation are outside this implementation task.
