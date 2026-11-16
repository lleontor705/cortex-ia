---
name: grill-me
description: Relentless structured interview to sharpen a plan, architecture decision, or feature design before implementation. Use when a plan or design needs pressure-testing before implementation.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Design Grilling

Use this to pressure-test a plan, architecture decision, or feature design before implementation starts. Interview the user until requirements, constraints, and trade-offs are concrete. Treat the design as a tree: each foundational decision unlocks the decisions that depend on it.

## Rounds

Each round asks the **frontier**: every decision whose prerequisites are already settled. Ask the whole frontier at once so the user can answer in one pass. A question that depends on an unanswered one waits for a later round.

For each question, give a number (`Q1`, `Q2`), the decision, the viable options (A/B/C), the trade-offs, and your recommendation with its technical reason. Then wait for the answers.

```
Q1 - <decision title>: <context, trade-offs, options A/B/C>
Recommendation: <option and reason>
```

## Facts and decisions

The user decides. The repository answers factual questions such as APIs, schemas, versions, test runners, and layout. When a question depends on a fact, the orchestrator dispatches `investigate` to find it. Do not ask the user for anything the codebase can answer.

## Stop condition

Stop when the frontier is empty: every branch has been visited, edge cases have been weighed, and nothing is left assumed silently. Then summarize the agreed design. Grilling writes nothing: if a plan exists, the orchestrator dispatches `planner` with phase `update` to record the decisions; otherwise the answers go into the planner's request.
