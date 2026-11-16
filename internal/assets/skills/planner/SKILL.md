---
name: planner
description: Write a short plan file with executable acceptance checks and a same-board task DAG, without implementing it. Use when a request needs a plan before implementation.
license: MIT
metadata:
  author: lleontor705
  version: "3.1.0"
---

# Planner

A plan is one file, `docs/plans/<board-id>.md`, plus the task DAG on the board. The plan says what done means and which checks prove it. Task state lives in SQLite, so the plan has no progress list. Long specification bundles added overhead without helping bug fixes or refactors, so the plan stays short and its checks stay executable.

## 1. Do you need a plan?

No plan, one direct task on `default`:
- a bug fix with a reproducing test;
- a refactor under green tests;
- test-only work;
- a change touching two files or fewer.

Plan when any of these hold:
- feature work touching three or more files;
- requirements are unclear (return `blocked` with focused questions in `open_questions`; the orchestrator interviews the user, then dispatches you);
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
## Decisions and Surprises
## Outcome
```

- **Goal and Done**: two to four sentences on the user-visible outcome and how it is judged done.
- **Context and Non-goals**: key files, constraints, and what is out of scope. Non-goals keep downstream work from growing.
- **Acceptance Checks**: raw executable commands with the expected result, IDs `AC-1`, `AC-2`, and so on. The first slice writes them as failing tests; later slices turn them green. Only phase `update` changes this table, with a Decisions entry.
- **Slices**: per slice, its id, `allowed_files`, `depends_on`, and the check IDs it must make green. Never list the plan file in `allowed_files`.
- **Decisions and Surprises**: dated entries of decision, reason, and what differed from the plan. Implementers report surprises in receipts; you record them here at update.
- **Outcome**: what changed, deviations, follow-ups. Filled at close.

## 3. Slicing

A slice becomes its own DAG task only when all three hold: its files do not overlap another slice's files, a fresh agent can resume it from the request text plus `git diff` alone, and it has its own check IDs. Otherwise the plan runs as one sequential task. Keep tightly coupled files in one slice, and allow at most about four parallel writers.

## 4. Checks, updates and close

- Each task carries its checks as `AC-<n>: <command>` strings, in `acceptance_checks` and in its verification. A reviewer verifies the checks its own task names. The whole plan's checks are verified at close.
- When the plan and the code disagree, the code and the checks win. Record the disagreement as a Decisions entry in phase `update`.
- Phase `update` has two uses: a check change the orchestrator names, and close. At close you append Decisions and Surprises from the receipts and fill Outcome. Make no commits.
- A durable decision moves to `AGENTS.md` or an ADR. The repository tests remain the living spec. There is no archive step.

## 5. Tasks

Each task carries a title naming the module, an objective with its non-goals, observable acceptance criteria, one raw verification command, and its `allowed_files`. Depend only on tasks that must finish first, on the same board. Use `cortex_ia_work_decompose` only for a blocked task the orchestrator routes back to you.
