---
description: Activate validation for an SDD change
agent: validate
subtask: true
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate SDD validation. Capture working directory, project, change, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical validation skill before running tests.
    2. Execute independent test commands (`FAIL_TO_PASS` and `PASS_TO_PASS`).
    3. Construct compliance matrix mapping requirements to proof.
    4. Emit typed verification verdict and quality findings.
  </execution_flow>
  <error_handling>
    - If regression or critical defect occurs: verdict is fail.
    - If human gate requested: present findings and wait for decision.
  </error_handling>
</command_dispatch>
