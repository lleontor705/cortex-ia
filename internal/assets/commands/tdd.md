---
description: Run a bounded Fast-TDD loop when a reliable oracle exists
agent: orchestrator
subagent: false
---

Check Fast-TDD eligibility, which needs a reliable oracle, then route through the `fast-tdd` workflow. If its safety gate fails, explain why and propose the closest safe workflow: $ARGUMENTS
