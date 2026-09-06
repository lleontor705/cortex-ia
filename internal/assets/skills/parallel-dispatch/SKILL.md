---
name: parallel-dispatch
description: Detect and execute independent task groups concurrently in OpenCode background subagents or isolated worktrees when dependencies are satisfied and allowed_files are disjoint.
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

## Step 1: Detect Parallel Candidates in the Board

1. **Query Board Tasks:**
   Call `cortex_ia_work_list({ board_id })`.
2. **Filter by Readiness:**
   Identify all tasks currently with `status: "ready"` (meaning all upstream dependencies have reached `done`).
3. **Analyze File Disjointness:**
   For all candidate tasks `[T1, T2, ...]`, compare their `allowed_files`:
   ```text
   Files(T1) ∩ Files(T2) == ∅  -->  PARALLEL CANDIDATES
   Files(T1) ∩ Files(T2) != ∅  -->  MUST REMAIN SEQUENTIAL
   ```
4. **Affinity & Locality Clustering (DynTaskMAS / DRAMA Pattern):**
   Group candidate tasks by package/subsystem directory prefix (e.g. `internal/tui/`, `internal/pipeline/`, `web/`):
   - Tasks within the same subsystem share semantic locality and AST symbols in Cortex MCP. Assign them sequentially to the same warm worker context to eliminate cold-start re-reading.
   - Form parallel execution waves across distinct subsystem boundaries (e.g. Subsystem A concurrently with Subsystem B).
5. **Form the Execution Wave:**
   Group candidate tasks into disjoint execution waves. Tasks sharing files are partitioned into successive waves.

---

## Step 2: Select Workspace Isolation Strategy

### Option A: `current_workspace` (Native Subagents)
- Suitable for 2–4 parallel native OpenCode `implement` controllers.
- Each controller claims a distinct `task_id`.
- Controllers reserve their disjoint paths atomically via `cortex_ia_work_claim({ task_id, paths: allowed_files })`.
- Because file sets are disjoint, per-file reservations succeed without collision.

### Option B: `isolated_worktree` (External AGY Leaves or High-Risk Work)
- Mandatory when delegating to external AGY leaves (`cortex_ia_delegate_start`).
- Each task runs in its own dedicated, clean worktree:
  ```bash
  cortex-ia worktree create .worktrees/<task-id>
  ```
- Eliminates git index and file locking contention completely.

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
| Task uses external AGY leaf | Use dedicated `isolated_worktree` per task |
| Worker fails or hits collision | Worker transitions to `blocked`; other parallel tasks continue unaffected |
| Reviewer returns `FAIL` | Task transitions to `blocked` for targeted retry; healthy tasks proceed |
