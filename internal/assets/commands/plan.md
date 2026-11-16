---
description: Plan a larger change with a short plan file and task DAG, then route its tasks to implementation
agent: orchestrator
subagent: false
---

1. Establish the execution mode (auto or interactive) and the workload policy from `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md` §4. Ask for the workload policy only when it is unset and the work is a planned initiative.
2. Decide whether a plan is needed using section 1 of the planner skill. A bug fix with a reproducing test, a refactor under green tests, test-only work, or a change to two files or fewer is one direct task on `default`; skip to step 6.
3. Dispatch `investigate` for the project's test runner, linter, and build commands, so the plan's checks use real commands.
4. If the requirements are unclear, interview the user with `grill-me` before planning, and carry the answers into the plan's Context.
5. Choose one stable board slug for the initiative (lowercase letters, digits and hyphens) and pass it to `planner`. Dispatch `planner` with workflow `plan` and phase `plan`. The planner creates the board, writes `docs/plans/<board-id>.md`, and materializes the same-board task DAG. The orchestrator does not create boards.
6. Dispatch ready tasks to `implement` under file leases. Route each to independent `reviewer` or orchestrator auto-approval per the review matrix in `cortex-work-protocol.md` §2.7a. Structural checks do not prove the change is correct. The plan's Acceptance Checks decide that.
7. When the last approval is recorded, dispatch `planner` with phase `update` to append Decisions and Surprises from the receipts and fill Outcome. Dispatch `planner` with phase `update` also when a check must change, naming the check and the reason. Only the planner writes the plan file.

Request: $ARGUMENTS
