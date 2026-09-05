---
description: Plan and execute a right-sized SDD Lite or Full change with preflight alignment and review budget protection
agent: orchestrator
subtask: false
---

1. Perform **SDD Preflight**: Establish execution mode (Interactive vs Auto), review budget (<400 lines), and `spec_plane=openspec|cortex|hybrid`; preserve the user's preference beyond any explicitly scoped exception.
2. Dispatch `investigate` for project testing capabilities (test runner, linter, strict TDD compatibility).
3. Load the `orchestrator` skill's phase/plane routing matrix and `~/.cortex-ia/opencode/contracts/cortex-convention.md` before selecting `decision-map`, `sdd-lite`, or `sdd-full`; carry `spec_plane` through every phase.
4. Dispatch `planner` for selected-plane artifacts and validation: cortex uses pinned contracts without OpenSpec writes/validation/archive; openspec/hybrid retain OpenSpec gates. Decision-map creates no board/tasks; only validated Lite/integrated or Full/tasks materializes the planner DAG. Any bootstrap follows only `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md`, not this command.
5. Dispatch ready tasks to native `implement` controllers under file leases, then independent `reviewer`; route archival to planner only after required approval, using the matrix. Missing/drifted contracts never authorize PASS: $ARGUMENTS
