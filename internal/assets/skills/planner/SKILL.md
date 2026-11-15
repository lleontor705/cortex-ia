---
name: planner
description: Write a short plan file with executable acceptance checks and a same-board task DAG, without implementing it. Use when a request needs a plan before implementation.
license: MIT
metadata:
  author: lleontor705
  version: "3.0.0"
---

# Planner

A plan is one file, `docs/plans/<board-id>.md`, plus the task DAG on the board. Implementers read the file, run its checks, and update it as they go. Long specification bundles added overhead without helping bug fixes or refactors, so the plan stays short and its checks stay executable.

## 1. Do you need a plan?

No plan, one direct task on `default`:
- a bug fix with a reproducing test;
- a refactor under green tests;
- test-only work;
- a change touching two files or fewer.

Plan when any of these hold:
- feature work touching three or more files;
- requirements are unclear (return `blocked` with focused questions in `open_questions`; the orchestrator interviews the user, then you write the plan);
- the work spans more than one session;
- the work needs more than one task.

## 2. The plan file

Path `docs/plans/<board-id>.md`, 40 to 120 lines, these sections in this order:

```markdown
# <Initiative> — Plan (board: <id>)
## Goal and Done
## Context and Non-goals
## Acceptance Checks
| ID | Behaviour | Command | Expected |
## Slices
## Progress
## Decisions and Surprises
## Outcome
```

- **Goal and Done**: two to four sentences on the user-visible outcome and how it is judged done.
- **Context and Non-goals**: key files, constraints, and what is explicitly out of scope. Non-goals keep downstream work from growing.
- **Acceptance Checks**: raw executable commands with the expected result. The first slice, or the only task, writes them as failing tests; later slices turn them green. Implementers do not edit this table. Only the orchestrator changes it, and records a Decisions entry for the change.
- **Slices**: per slice, the id, `allowed_files`, `depends_on`, and the check IDs it must make green.
- **Progress**: dated checkboxes.
- **Decisions and Surprises**: append-only entries of date, decision, and reason.
- **Outcome**: commits, deviations from the plan, follow-ups. Filled when the work closes.

## 3. Slicing

A slice becomes its own DAG task only when all three hold: its files do not overlap another slice's files, a fresh agent can resume it from the request text plus `git diff` alone, and it has its own check IDs. Otherwise the plan runs as one sequential task. Keep tightly coupled files in one slice, and allow at most about four parallel writers.

## 4. Living plan

- The implementer appends to Progress and Decisions and Surprises before moving its task to `in_review`. The plan file belongs in the `allowed_files` of plan tasks for that reason.
- When the plan and the code disagree, the code and the checks win. Record the disagreement as a Decisions entry.
- At close, the last implementer fills Outcome. The file stays as history. A durable decision moves to `AGENTS.md` or an ADR, and the repository tests remain the living spec. There is no archive step.

## 5. Tasks

Each task carries a title naming the module, an objective with its non-goals, observable acceptance criteria, one raw verification command, and its `allowed_files`. Depend only on tasks that must finish first, on the same board. Use `cortex_ia_work_decompose` only for a blocked task the orchestrator routes back to you.
