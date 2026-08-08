---
description: Activate implementation for ready SDD work
agent: orchestrator
subtask: true
---

<command_dispatch>
  <instruction>
    Activate SDD implementation. Capture working directory, project, task ID, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Query ForgeSpec task board to confirm task readiness.
    2. Load task specification and design artifacts.
    3. Execute Red-Green-Refactor TDD cycle on bounded work unit.
    4. Validate result and mark task complete in ForgeSpec.
  </execution_flow>
  <error_handling>
    - If no ready tasks on task board: return blocked.
    - If human gate requested: present failure and wait for approval.
  </error_handling>
</command_dispatch>
