---
agent: orchestrator
description: Activate debate for an SDD topic
subtask: false
---

<command_dispatch>
  <instruction>
    Activate multi-position debate. Capture working directory, project, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical debate skill and identify conflicting positions.
    2. Formulate position arguments with evidence and trade-offs.
    3. Synthesize consensus recommendations.
    4. Emit canonical decision package for downstream design.
  </execution_flow>
  <error_handling>
    - If positions lack technical grounding: return blocked.
    - If human gate requested: present alternatives and wait for approval.
  </error_handling>
</command_dispatch>
