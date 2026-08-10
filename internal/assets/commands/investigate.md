---
description: Activate investigation for a supplied topic
agent: investigate
subtask: true
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate SDD investigation. Capture working directory, project, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical investigation skill before reading codebase.
    2. Inspect affected code areas within probe budget.
    3. Evaluate multiple technical approaches and risks.
    4. Emit verified investigation findings and recommendation.
  </execution_flow>
  <error_handling>
    - If probe budget exhausted: checkpoint findings and state limits.
    - If human gate requested: present findings and wait for decision.
  </error_handling>
</command_dispatch>
