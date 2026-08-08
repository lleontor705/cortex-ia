---
description: Activate a new SDD change request
agent: orchestrator
subtask: false
---

<command_dispatch>
  <instruction>
    Activate new SDD change. Capture working directory, project, change, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical proposal skill and review exploration evidence.
    2. Draft change proposal with bounded scope and rollback plan.
    3. Select dependency route and map requirement candidates.
    4. Emit canonical proposal contract for specification phase.
  </execution_flow>
  <error_handling>
    - If proposal contradicts repository stack: return blocked.
    - When human gate requested: present scope and wait for approval.
  </error_handling>
</command_dispatch>
