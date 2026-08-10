---
description: Launch exploratory spike and disposable proof-of-concept
agent: investigate
subtask: false
---

User arguments: $ARGUMENTS

<command_dispatch>
  <instruction>
    Activate technical Spike prototyping. Explore APIs, libraries, or benchmarks.
    Target Arguments: "$ARGUMENTS"
  </instruction>
  <execution_flow>
    1. Read canonical spike-prototype skill.
    2. Set up isolated scratchpad environment.
    3. Implement disposable prototype and run benchmarks.
    4. Synthesize viability metrics, trade-offs, and risks.
    5. Save observations to Cortex and recommend path forward.
  </execution_flow>
  <error_handling>
    - If scratchpad leaks into production code: clean up immediately.
    - If benchmark is inconclusive: record limits and report findings.
  </error_handling>
</command_dispatch>
