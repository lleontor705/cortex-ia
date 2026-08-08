---
name: parallel-dispatch
description: "Split independent work into bounded parallel work units and merge their evidence safely."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for independent dispatch planning and result synthesis.</role>
<success_criteria>Independence is proven before dispatch, each bounded unit has isolated ownership and an explicit verification command, all direct-child results are collected, and aggregate verification finds no conflicting edits.</success_criteria>
<context>Use only for two or more work units that can proceed without shared mutable state or ordering dependencies. ForgeSpec owns readiness and claims. Dependent or overlapping work remains sequential.</context>
<rules><critical>Declare each unit's inputs, outputs, file boundaries, allowed effects, and merge owner before using native `task()`. Launch independent units together. Never dispatch a nested coordinator, assign the same writable file twice, or infer readiness.</critical><guidance>Prefer read-only parallelism. For writable work require disjoint ownership or qualified worktree isolation, collect command and exit-code evidence per unit, and stop on conflicts or hidden dependencies.</guidance></rules>
<steps>1. Map dependencies and current ForgeSpec readiness. 2. Prove which units are independent and bounded. 3. Assign isolated ownership and self-contained prompts. 4. Dispatch ready units in one batch with `task()`. 5. Collect every result without discarding partial successes. 6. Synthesize conflicts, reconcile through the declared merge owner, and run aggregate verification. Fall back to sequential execution when any independence proof is missing.</steps>
<output>Return dispatched groups, owners, dependencies, boundaries, evidence, failures, conflicts, synthesis, aggregate verification, and next ready references.</output>
<references>Use ForgeSpec readiness and claims, repository ownership rules, task specifications, and the portable parallel-apply contract.</references>
