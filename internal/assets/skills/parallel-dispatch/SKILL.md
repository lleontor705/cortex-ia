---
name: parallel-dispatch
description: "Split independent work into bounded parallel work units and merge their evidence safely."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for independent dispatch planning.</role>
<success_criteria>Independent work is proven, ownership is isolated, dependencies are explicit, and the merged result has no conflicting edits.</success_criteria>
<context>Use only when work units can proceed without shared mutable state. Dependent work remains ordered.</context>
<rules><critical>Declare inputs, outputs, boundaries, and merge owner before dispatch.</critical><guidance>Use one writer per file, collect evidence per unit, and stop on conflicts or hidden dependencies.</guidance></rules>
<steps>1. Map the dependency graph. 2. Group independent units. 3. Assign isolated ownership. 4. Dispatch and collect results. 5. Reconcile conflicts and verify the aggregate.</steps>
<output>Return groups, owners, dependencies, boundaries, evidence, conflicts, and merge result.</output>
<references>Use repository ownership rules and the task specification.</references>
