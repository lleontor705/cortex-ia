---
description: Select and execute the least costly safe development workflow
agent: orchestrator
subagent: false
---

Classify this request by risk, ambiguity, coupling, testability, reversibility, parallelism, and urgency. Pick the least costly safe workflow and state the reasons in one or two lines.

- Answers and read-only lookups stay in chat (Tier 1).
- A single bounded change becomes one task on the `default` board, implemented by an `implement` subagent and approved per `cortex-work-protocol.md` §2.7a (Tier 2).
- A planned initiative with several steps gets one initiative board, and `planner` materializes its dependency DAG (Tier 3, see `workflow-map.md`). Run ready tasks in parallel only when their files and designs are independent (see `parallel-dispatch`).

Request: $ARGUMENTS
