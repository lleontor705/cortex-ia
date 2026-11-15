---
description: Plan a larger change with a short plan file and task DAG, then route its tasks to implementation
agent: orchestrator
subagent: false
---

1. Establish the execution mode (auto or interactive) and the workload policy from `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md` §4. Ask for the workload policy only when it is unset and the work is a planned initiative.
2. Decide whether a plan is needed using section 1 of the planner skill. A bug fix with a reproducing test, a refactor under green tests, test-only work, or a change to two files or fewer is one direct task on `default`; skip to step 6.
3. Dispatch `investigate` for the project's test runner, linter, and build commands, so the plan's checks use real commands.
4. If the requirements are unclear, interview the user with `grill-me` before planning, and carry the answers into the plan's Context.
5. Create one stable board ID for the initiative, then dispatch `planner` with workflow `plan` and phase `plan`. It writes `docs/plans/<board-id>.md` and materializes the same-board task DAG.
6. Dispatch ready tasks to `implement` under file leases. Route each to independent `reviewer` or orchestrator auto-approval per the review matrix in `cortex-work-protocol.md` §2.7a. Structural checks do not prove the change is correct. The plan's Acceptance Checks decide that: $ARGUMENTS
