---
description: Select and execute the least costly safe development workflow
agent: orchestrator
subagent: false
---

Classify this request by risk, ambiguity, coupling, testability, reversibility, parallelism, and urgency. Pick the least costly safe workflow and state the reasons in one or two lines.

- Answers and read-only lookups stay in chat (Tier 1).
- A single bounded change becomes one task on the `default` board, implemented by an `implement` subagent and approved per `cortex-work-protocol.md` §2.7a (Tier 2). Set `task_id` as a lowercase slug, `allowed_files` to the files the dispatch names, `verification` to the acceptance command, and `workload_policy` (default `flexible`).
- A planned initiative with several steps gets one board slug chosen by the orchestrator. `planner` creates the board and materializes its dependency DAG (Tier 3, see `workflow-map.md`). Run ready tasks in parallel only when their files and designs are independent (see `parallel-dispatch`).

Request: $ARGUMENTS
