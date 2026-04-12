---
description: Implement SDD tasks — delegates to @team-lead who coordinates parallel @implement sub-agents
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

3. **Delegate to @team-lead**: Launch a single team-lead agent that owns the entire apply phase:
   ```
   task(@team-lead, "
     Execute the apply phase.
     Change: $ARGUMENTS | Project: {project} | Board: {board_id}
     artifact_store.mode: {mode}
   ")
   ```
   The team-lead handles all group coordination, parallel @implement launches, file reservations, and retries.

4. **Process report**: When team-lead returns:
   - Validate contract: `sdd_validate(phase: "apply", agent_output: "{output}")`
   - If success → proceed to /validate
   - If partial → present failures, ask user to retry or proceed
   - If blocked → report to user

SINGLE-TASK SHORTCUT:
If only 1 task on the board, skip @team-lead and delegate directly to @implement.
