---
description: Activate the next SDD phase
agent: orchestrator
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate SDD execution resume. Capture working directory, project context, change, and active session state.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read authoritative change state and context from ForgeSpec and Cortex.
    2. Locate ready bounded work units on the active task board.
    3. Execute planning dispatch to route tasks through implementation.
    4. Validate progress and record session summary before exiting.
  </execution_flow>
  <error_handling>
    - If active change state is corrupted: halt and block resume gate.
    - If all work units complete: route dispatch directly to validation.
  </error_handling>
</command_dispatch>
