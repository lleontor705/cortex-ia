---
description: Plan and execute a right-sized SDD Lite or Full change with preflight alignment and review budget protection
agent: orchestrator
subtask: false
---

1. Perform **SDD Preflight**: Ensure execution mode (Interactive vs Auto), review budget (<400 lines), and artifact mirroring preference are determined.
2. Probe project testing capabilities (test runner, linter, strict TDD compatibility).
3. Evaluate SDD depth (`sdd-lite` vs `sdd-full`).
4. Delegate planning to `planner`, persist validated ForgeSpec contracts (and mirror to `openspec/changes/` if requested).
5. Dispatch ready DAG tasks with strictly bounded file leases to leaf `implement` minions and verify with `reviewer`: $ARGUMENTS

