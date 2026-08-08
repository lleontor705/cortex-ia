---
description: Activate SDD bootstrap for current project
agent: orchestrator
subtask: true
---

<command_dispatch>
  <instruction>
    Activate SDD bootstrap. Capture working directory, project, artifact store, and context.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Inspect manifest files (`go.mod`, `package.json`, `pyproject.toml`) with citations.
    2. Detect test runners, linters, and CI workflows with executable proof.
    3. Inventory phase skills, conventions, and contract gates.
    4. Emit project context with evidence freshness and confidence.
  </execution_flow>
  <error_handling>
    - If manifest or test runner is missing: return blocked.
    - If human gate requested: present findings and wait for decision.
  </error_handling>
</command_dispatch>
