---
name: parallel-dispatch
description: Detect and execute independent task groups concurrently in OpenCode background subagents when dependencies are satisfied and allowed_files are disjoint.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Parallel Dispatch Protocol

## Overview

Execute independent tasks concurrently instead of forcing sequential execution. Concurrency dramatically reduces total initiative lead time while preserving SQLite work authority and file safety.

**Core Principle:** Parallelism is valid ONLY when tasks are `ready` and their writable `allowed_files` are strictly disjoint (zero intersection).

**Announce at start:** "I am using the parallel-dispatch skill to execute independent tasks concurrently."

---

## Step 1: Detect Independent Ready Tasks

1. Run `cortex_ia_work_list` with `--status ready`.
2. Inspect the `allowed_files` array for each candidate task.
3. Compute the intersection of `allowed_files` across all candidates:
   - If sets are **disjoint**: The tasks can be executed in parallel.
   - If sets **overlap**: The tasks MUST be executed sequentially (enforce dependency order).

---

## Step 2: Workspace Strategy

### `current_workspace` (Sole Supported Strategy)
- Suitable for parallel native OpenCode `implement` controllers with disjoint file scopes.
- Each controller claims a distinct `task_id`.
- Controllers reserve their disjoint paths atomically via `cortex_ia_work_claim({ task_id, paths: allowed_files })` or `cortex_ia_file_reserve`.
- Because file sets are disjoint, per-file reservations succeed without collision.
- If delegating an external AGY leaf, execution remains exclusive during its execution window under pre-run baseline verification; `isolated_worktree` is retired.

---

## Step 3: Concurrent Dispatch

For each task in the parallel wave, dispatch an `implement` controller in the same orchestrator turn using native background subagents and formal `<minion-contract>`:

```json
<minion-contract>
{
  "task_id": "task-auth-jwt",
  "objective": "Implement JWT validation middleware with claims checking",
  "allowed_files": ["internal/auth/jwt.go", "internal/auth/jwt_test.go"],
  "acceptance_checks": ["go test -v ./internal/auth/... -run TestJWT"],
  "workspace_strategy": "current_workspace",
  "worktree": null,
  "artifact_refs": ["specs/auth/spec.md"],
  "max_steps": 30,
  "budget_tier": "medium"
}
</minion-contract>
```

When using OpenCode's `task` tool:
```javascript
// Launch Task 1 concurrently
task({ subagent: "implement", prompt: envelopeTask1, background: true });

// Launch Task 2 concurrently
task({ subagent: "implement", prompt: envelopeTask2, background: true });
```

**Capacity Rule:** Default maximum of 3 concurrent background writers to prevent CPU/memory exhaustion.

---

## Step 4: Reactive Join & Independent Verification

1. **Reactive Completion:**
   Do not poll in a sleep loop. The orchestrator receives completion notifications automatically as each background subagent finishes and transitions its task to `in_review`.
2. **Dispatch Independent Review:**
   For each task reaching `in_review`, dispatch the `reviewer` controller:
   - Reviewer runs the acceptance checks independently.
   - Reviewer executes `cortex_ia_work_approve({ task_id, verdict: "PASS" })`.
3. **Atomic DAG Unlocking:**
   Upon reviewer `PASS`, SQLite atomically:
   - Marks the task `done`.
   - Releases any remaining claim/leases.
   - Evaluates downstream dependents in the board. Dependents whose prerequisites are now all `done` automatically transition to `ready`.
4. **Next Wave:**
   Repeat Step 1 to detect newly ready tasks and launch the next parallel wave.

---

## Quick Reference

| Condition | Action |
|---|---|
| Multiple tasks `ready` with disjoint `allowed_files` | Launch parallel background subagents (`parallel-dispatch`) |
| Tasks share any file in `allowed_files` | Execute sequentially in dependency/sorted order |
| Task uses external AGY leaf | Use exclusive `current_workspace` window with baseline checks; `isolated_worktree` is retired |
| Worker fails or hits collision | Worker transitions to `blocked`; other parallel tasks continue unaffected |
| Reviewer returns `FAIL` | Task transitions to `blocked` for targeted retry; healthy tasks proceed |
