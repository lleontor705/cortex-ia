# Spec Delta — Work Authority (summary)

> Canonical requirement set with full Given/When/Then scenarios: [plan.md](plan.md) (REQ-WAUTH-001..004, validated). This file is a navigational summary; plan.md is authoritative.

- **REQ-WAUTH-001 Claim TTL decoupled from stale-progress window** — `ttl: "30m"` independent constant; not derived from or equal to `stale_progress_ms` (900000, unchanged); stale-progress stop intact. Oracle: `node --test --test-name-pattern TestREQ_WAUTH_001 scripts/harness-controller-authority.test.mjs`.
- **REQ-WAUTH-002 Scoped self-recovery tool for the implement role** — `cortex_ia_work_recover_own(task_id)` implement-only in the mutation gate; in-memory ownership check fail-closed; CLI delegation with task/owner scoping; handle cleanup on success; unscoped `cortex_ia_work_recover` stays orchestrator-only. Oracle: `node --test --test-name-pattern TestREQ_WAUTH_002 scripts/harness-controller-authority.test.mjs`.
- **REQ-WAUTH-003 CLI scoped recover is ownership-verified and fail-closed** — `work recover --task <id> --owner <identity>` (flags mandatory together); durable owner+expiry verified in a write transaction; typed denial, zero writes; unscoped behavior unchanged. Oracle: `go test -count=1 -run TestREQ_WAUTH_003 ./internal/delegation/...`.
- **REQ-WAUTH-004 Protocol and agent assets codify the new boundary** — authority table splits recover/retry (orchestrator-only) from recover_own (implement-scoped); implement agent guard permits only the scoped form; rebuilt binary embeds the assets. Oracle: `go build -o bin/cortex-ia ./cmd/cortex-ia`.
