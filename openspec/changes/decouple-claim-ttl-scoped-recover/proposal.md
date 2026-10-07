# Proposal — Decouple Claim TTL from Stale-Progress Window & Scoped Self-Recovery for Implement

> Canonical integrated contract: [plan.md](plan.md) (validated, sdd-lite integrated). Requirement IDs: REQ-WAUTH-001..004. Task DAG: [tasks.md](tasks.md) on board `ttl-recover-release`.

## Problem

Railway telemetry hub filed three open bug reports against the cortex-work bridge:

- **#140 ERR_TASK_BLOCKED** (7 signatures): Cause A (5/7) — the implementation claim TTL is coupled to the stale-progress window at `internal/assets/plugins/cortex-work.ts:128` (`maintenancePolicy = { ..., stale_progress_ms: 900000, ttl: "15m" }`). Long silent build phases (dotnet restores on exFAT) outlive the window, the heartbeat stops, and the claim expires before transition to `in_review`. The orphan-guard is deliberate and pinned by `scripts/harness-controller-authority.test.mjs`; it must not be weakened.
- **#142 ERR_INVARIANT_VIOLATION** (1 signature): same Cause A, different derived code. Declared critical, real severity low.
- **#143 ERR_TOOL_LEASE_REQUIRED**: fail-closed worked correctly (envelope arrived with `task_id: null`); remediation lives outside this repository. No code change.

Recovery is orchestrator-only today (`work_recover: ["orchestrator"]`, `cortex-work.ts:1489`), so an implement controller whose own claim expired cannot recover it.

## Approach

1. Decouple the claim TTL: `maintenancePolicy.ttl = "30m"` as an independent constant; `stale_progress_ms` (15 min orphan window) unchanged — REQ-WAUTH-001.
2. Scoped self-recovery: new bridge tool `cortex_ia_work_recover_own(task_id)` gated to `implement` (in-memory ownership check, fail-closed) plus a durable CLI/store scoped form `work recover --task <id> --owner <identity>` with typed fail-closed denial — REQ-WAUTH-002, REQ-WAUTH-003.
3. Codify the boundary in `cortex-work-protocol.md`, `agents/implement.md`, and root `AGENTS.md` — REQ-WAUTH-004.
4. Backfill `CHANGELOG.md` v0.5.3..v0.5.7 + v0.5.8 (unreleased) entry.

## Non-Goals

- No tags, releases, or push; no GitHub issue comments/closures (orchestrator coordinates the release after the DAG completes).
- No weakening of the orphan-guard beyond the two scoped changes; harness authority tests keep their assertions.
- No progress-based dynamic TTL logic; `implement` never recovers other sessions' tasks or bypasses leases.
