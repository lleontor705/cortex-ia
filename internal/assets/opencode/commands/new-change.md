---
description: Activate a new SDD change request
agent: orchestrator
subtask: false
---

<command_dispatch>
  <instruction>
    Activate a new change and capture its directory, project, name, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Dispatch bootstrap via native `task` with context.
    2. Dispatch investigate with the successful bootstrap reference.
    3. Dispatch draft-proposal with the successful investigate reference.
    4. Return child references and proposal handoff.
  </execution_flow>
  <error_handling>
    - After each child, stop on `blocked` and return its evidence.
    - Route only: never load child skills or perform phase work.
    - Each child performs one phase and never continues the chain.
  </error_handling>
</command_dispatch>
