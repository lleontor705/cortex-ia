---
description: Activate finalization for a verified SDD change
agent: orchestrator
subtask: true
---

<command_dispatch>
  <instruction>
    Activate SDD finalization. Capture working directory, project, change, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical finalization skill and verify validation verdict is pass.
    2. Consolidate delta specifications and clean temporary state.
    3. Generate change retrospective with lessons and task board closure.
    4. Emit canonical archive contract and hand off to release workflow.
  </execution_flow>
  <error_handling>
    - If validation verdict is not pass: halt and block archive gate.
    - When human gate requested: present retrospective and wait for decision.
  </error_handling>
</command_dispatch>
