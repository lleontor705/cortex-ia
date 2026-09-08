---
description: Plan and execute a right-sized SDD Lite or Full change with preflight alignment and review budget protection
agent: orchestrator
subtask: false
---

1. Perform **SDD Preflight**: Establish execution mode (Interactive vs Auto), review budget (<400 lines), and `spec_plane=openspec|cortex|hybrid`; preserve the user's preference beyond any explicitly scoped exception.
2. Dispatch `investigate` for project testing capabilities (test runner, linter, strict TDD compatibility).
3. Load the canonical `~/.cortex-ia/opencode/contracts/workflow-map.md` and `cortex-convention.md` before selecting `decision-map`, `sdd-lite`, or `sdd-full`; carry `spec_plane` through every phase.
4. Dispatch `planner` for selected-plane artifacts and validation: cortex uses pinned contracts without OpenSpec writes/validation/archive; openspec/hybrid retain OpenSpec gates. Decision-map creates no board/tasks; only validated Lite/integrated or Full/tasks materializes the planner DAG. Any bootstrap follows only `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md`, not this command.
5. Materialize SDD tasks with typed contract pins and requirement IDs. Dispatch ready tasks to native `implement` controllers under session-owned file leases, then independent `reviewer`; route closure through planner's `cortex_ia_change_archive` only after required approval and current fingerprints. Structural validation does not prove semantic correctness. Missing/drifted contracts never authorize PASS: $ARGUMENTS
