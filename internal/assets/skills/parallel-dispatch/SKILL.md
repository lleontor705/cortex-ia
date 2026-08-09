---
name: parallel-dispatch
description: "Split independent work into bounded parallel work units and merge their evidence safely."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for plan-only independent dispatch planning.</role>
<success_criteria>Independence is proven before execution, each bounded unit has isolated ownership and an explicit verification command, and dispatch waves expose ordering, merge ownership, and conflict gates for orchestrator approval.</success_criteria>
<context>Use only for two or more work units that can proceed without shared mutable state or ordering dependencies. ForgeSpec owns readiness and claims. Dependent or overlapping work remains sequential.</context>
<rules><critical>Declare each unit's inputs, outputs, file boundaries, allowed effects, and merge owner without invoking child execution. Never require a dispatch tool, create a nested coordinator, assign the same writable file twice, infer readiness, or execute the plan.</critical><guidance>Prefer read-only parallelism. For writable work require disjoint ownership or qualified worktree isolation, request command and exit-code evidence per unit, and place conflicts or hidden dependencies behind an explicit stop gate.</guidance></rules>
<steps>1. Map dependencies and current ForgeSpec readiness. 2. Prove which units are independent and bounded. 3. Assign isolated ownership and self-contained prompts. 4. Group ready units into dispatch waves, preserving dependencies between waves. 5. Define result collection, conflict detection, merge ownership, and aggregate verification. 6. Return the plan for orchestrator approval and direct-child execution. Fall back to a sequential wave when independence proof is missing.</steps>
<output>Return a plan-only set of dispatch waves with owners, dependencies, boundaries, prompts, required evidence, stop gates, merge owner, aggregate verification, and next-ready references. The orchestrator executes approved direct-child tasks.</output>
<references>Use ForgeSpec readiness and claims, repository ownership rules, task specifications, and the portable parallel-apply contract.</references>
