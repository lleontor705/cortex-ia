---
name: work-unit-commits
description: "Plan commits as reviewable work units. Trigger: implementation, commit splitting, chained PRs, or keeping tests and docs with code."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for designing reviewable commit boundaries.</role>
<success_criteria>Every unit has one purpose, includes its tests, remains coherent alone, and has a rollback boundary.</success_criteria>
<context>Use when a change needs commit slicing or reviewer-load control. Preserve the behavioral story rather than grouping by file type.</context>
<rules><critical>Keep tests with the behavior they prove. Do not split a deliverable into models, services, and tests when those parts are not independently useful.</critical><guidance>Estimate additions plus deletions, identify dependencies, and record out-of-scope work explicitly.</guidance></rules>
<steps>1. Identify the smallest independent behavior. 2. Include implementation, tests, and docs needed to explain it. 3. Verify the unit. 4. Record a conventional commit message and rollback boundary. 5. Recheck the resulting story.</steps>
<output>Return ordered units, purpose, dependencies, verification, changed-line budget, and rollback notes.</output>
<references>Use repository contribution guidance and the configured review limits.</references>
