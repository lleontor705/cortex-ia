---
description: Launch Fast-TDD micro loop for bounded feature or bugfix
agent: implement
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate Fast-TDD execution loop. Focus on narrow Red-Green-Refactor test cycle.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical fast-tdd skill and micro-contract.
    2. Write focused failing unit test covering target requirements (RED).
    3. Implement minimal production code to pass test (GREEN).
    4. Clean code and verify no regressions (REFACTOR).
    5. Emit structured test evidence and update ForgeSpec/Cortex.
  </execution_flow>
  <error_handling>
    - If scope exceeds 2 files: abort fast loop and recommend SDD.
    - If test fails to reproduce problem: return blocked.
  </error_handling>
</command_dispatch>
