---
name: parallel-dispatch
description: Detect and execute independent task groups concurrently in the host's background subagents when dependencies are satisfied and allowed_files are disjoint. Use when several ready tasks could run at the same time.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Parallel Dispatch

Run implementers in parallel only when the work is independent. Two tasks are independent when they write disjoint files and their design decisions do not depend on each other. Coupled work runs sequentially, either as one task or in dependency order.

## 1. Choose the wave

Start from the tasks that are `ready` in the board (`cortex_ia_work_list({ board_id })`). Put a task in the same wave only if:

- its writable `allowed_files` do not overlap with any other task in the wave, and
- its design does not depend on another task's result, and it does not define an interface another task uses.

Keep a wave to at most 4 implementers. Tasks that fail either check wait for a later wave.

## 2. Workspace

Use `current_workspace`; it is the only supported strategy. Each implementer holds its own live claim and reserves each writable file with `cortex_ia_file_reserve`, in sorted path order, before editing that file. It releases each file with `cortex_ia_file_release`. On a conflict it releases what it already holds and stops writing.

Never lease the plan file `docs/plans/<board-id>.md`. Two slices writing it would collide and break review fingerprints. Implementers report decisions in receipts; the planner records them in phase `update`.

## 3. Dispatch

Launch one `implement` subagent per task in the same turn, as background subagents. Give each the dispatch brief from the orchestrator prompt, with `allowed_files`, `non_goals`, and `acceptance_checks` filled in. Do not poll. Each subagent's completion arrives when it moves its task to `in_review`.

## 4. Join

For each task in `in_review`, choose orchestrator auto-approval or an independent `reviewer` per `cortex-work-protocol.md` §2.7a, then approve with `cortex_ia_work_approve` at the current revision and with an evidence pointer. On PASS, SQLite marks the task done, releases its remaining authority, and moves dependents whose prerequisites are now complete to `ready`. On FAIL, only that task is blocked for a targeted retry; the rest of the wave continues. Then recompute the ready set and start the next wave.

## Quick reference

| Situation | Action |
|---|---|
| Ready tasks with disjoint files and independent designs | Run them as one wave, at most 4 |
| Tasks share a file or one design depends on another | Run sequentially in dependency order |
| Task in `in_review` | Auto-approve or send to `reviewer` per §2.7a |
| Worker hits a file conflict or loses authority | Release partial reservations, stop writing, report `blocked` |
| Reviewer returns FAIL | Block that task for a targeted retry; other tasks continue |
