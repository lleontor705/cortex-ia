---
description: Activate monitoring for SDD state
agent: orchestrator
subtask: true
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate SDD monitoring. Capture working directory, project context, change, and active session state.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read authoritative change status, task board, and active leases.
    2. Poll background test executions and verify worker health.
    3. Aggregate completion metrics and remaining implementation risks.
    4. Execute dispatch reporting to emit progress summary to operator.
  </execution_flow>
  <error_handling>
    - If background test fails: report task failure and suggest retry.
    - If human gate requested: present context and wait for command.
  </error_handling>
</command_dispatch>
