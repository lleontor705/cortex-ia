---
description: Implement SDD tasks by routing ForgeSpec-ready work directly to @implement
agent: orchestrator
subtask: true
---

Execute the apply phase for the active SDD change.

CONTEXT:
- Working directory: !`echo -n "$(pwd)"`
- Current project: !`echo -n "$(basename $(pwd))"`
- Artifact store mode: {determined by orchestrator — default: cortex if Cortex MCP available, else none}

WORKFLOW:

1. **Pre-flight**: Verify the task board exists via `tb_status`. If not, run /decompose first.

2. **Pre-flight artifacts**: Verify required Cortex artifacts:
   - `mem_search(query: "sdd/$ARGUMENTS/tasks")` — REQUIRED
   - `mem_search(query: "sdd/$ARGUMENTS/spec")` — recommended
   - `mem_search(query: "sdd/$ARGUMENTS/design")` — recommended

3. **Route ready work directly**: Query `tb_unblocked`, claim each ready bounded work unit, and launch direct-child implement agents only:
   ```
   task(@implement, "
     Implement task {task_id}.
     Change: $ARGUMENTS | Project: {project} | Board: {board_id} | Task: {task_id}
     artifact_store.mode: {mode}
   ")
   ```
   Portable sequential runs one ready task at a time. Portable flat may launch independent ready tasks as direct children when the host runtime is qualified. Never add a nested coordinator.

4. **Process reports**: Validate each returned apply contract, update ForgeSpec status, and query readiness again until no tasks remain:
   - Validate contract: `sdd_validate(phase: "apply", agent_output: "{output}")`
   - If success → proceed to /validate
   - If partial → present failures, ask user to retry or proceed
   - If blocked → report to user
