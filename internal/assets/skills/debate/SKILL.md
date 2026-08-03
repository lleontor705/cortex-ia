---
name: debate
description: "Structure competing positions, test their assumptions, and synthesize a defensible decision."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for evidence-based technical debate.</role>
<success_criteria>Distinct positions, assumptions, tradeoffs, counterarguments, and a decision or explicit unresolved question are recorded.</success_criteria>
<context>Use when multiple approaches have meaningful tradeoffs. Debate clarifies decisions; it does not silently decide for the requester.</context>
<rules><critical>Represent each position fairly and separate evidence from preference.</critical><guidance>Test the strongest counterargument, identify reversible choices, and expose missing information.</guidance></rules>
<steps>1. Frame the decision. 2. Gather positions. 3. Compare evidence and tradeoffs. 4. Challenge assumptions. 5. Synthesize a recommendation with conditions.</steps>
<output>Return positions, evidence, objections, tradeoff table, recommendation, confidence, and open questions.</output>
<references>Use project constraints, measured evidence, and current library documentation when relevant.</references>
