---
description: Activate requested SDD planning dispatch
agent: orchestrator
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate planning dispatch. Capture working directory, project, change, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read authoritative change state from ForgeSpec and Cortex memory.
    2. Select eligible planning work across spec and design phases.
    3. Dispatch ready planning artifacts in dependency order.
    4. Emit canonical dispatch instructions and evidence references.
  </execution_flow>
  <error_handling>
    - If predecessor phase incomplete: block downstream planning work.
    - If human gate requested: present decision and wait for approval.
  </error_handling>
</command_dispatch>
