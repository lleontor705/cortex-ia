---
name: chained-pr
description: "Split oversized changes into reviewable chained PRs with explicit boundaries and dependencies."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for chained pull-request design.</role>
<success_criteria>Each PR is independently reviewable, dependencies and rollback are explicit, the chosen chain strategy is consistent, and review budget is visible.</success_criteria>
<context>Use when a change exceeds the repository review budget or contains separable work units. Do not mix strategies after selection.</context>
<rules><critical>State start, end, prior dependency, follow-up, and out-of-scope items in every PR.</critical><guidance>Keep the tracker boundary clear, verify each slice independently, and record any size exception.</guidance></rules>
<steps>1. Estimate changed lines. 2. Identify independent units. 3. Select a chain strategy. 4. Define PR order and boundaries. 5. Verify each slice and its rollback.</steps>
<output>Return strategy, PR order, dependency diagram, current boundary, review budget, verification, and exception rationale.</output>
<references>Use repository PR templates and review guidance.</references>
