---
description: Launch emergency hotfix triage and atomic patch
agent: implement
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate Hotfix triage. Diagnose root cause and apply minimal bounded patch.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical hotfix-triage skill.
    2. Analyze error logs/stacktraces to identify defect origin.
    3. Apply atomic code fix (strictly <= 50 lines).
    4. Run smoke tests and regression suite to verify resolution.
    5. Save incident resolution to Cortex and release locks.
  </execution_flow>
  <error_handling>
    - If patch requires architectural rework: halt and escalate to SDD.
    - If regression test fails: rollback changes and report blocked.
  </error_handling>
</command_dispatch>
