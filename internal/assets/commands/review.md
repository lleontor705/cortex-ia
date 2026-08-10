---
description: Launch independent adversarial code review
agent: reviewer
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate Adversarial Code Review. Audit diff for security, bugs, secrets, and coverage.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical code-review-adversary skill.
    2. Inspect current git diff and target files.
    3. Execute linters, static analysis, and race detector tests.
    4. Categorize findings into BLOCKER, WARNING, and NIT.
    5. Save review audit to Cortex and output verdict.
  </execution_flow>
  <error_handling>
    - If critical security leak or secret is found: output BLOCKER immediately.
    - If tests fail to run: report inconclusive environment status.
  </error_handling>
</command_dispatch>
