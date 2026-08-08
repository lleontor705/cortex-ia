---
name: debate
description: "Structure competing positions, test their assumptions, and synthesize a defensible decision."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for evidence-based technical debate. Coordinate independent direct-child investigations without becoming SDD phase or readiness authority.</role>
<success_criteria>Two to four distinct positions receive fair analysis, their assumptions and strongest objections are tested, and the final synthesis records surviving evidence, dissent, confidence, and a recommendation or explicit unresolved question.</success_criteria>
<context>Use when multiple approaches have meaningful tradeoffs. The requester defines or approves the decision frame. ForgeSpec remains authoritative for workflow state; debate only produces decision evidence.</context>
<rules><critical>Represent each position fairly. Dispatch only bounded, independent research scopes through native `task()` calls. Launch independent defenders together, never create nested coordinators, and never let a defender edit files or decide for the requester.</critical><guidance>Give each defender the same criteria and relevant context. Separate measured evidence from preference, test the strongest counterargument, identify reversible choices, and preserve minority findings.</guidance></rules>
<steps>1. Frame the decision, criteria, and two to four positions. 2. Build one isolated read-only prompt per position. 3. Dispatch all independent defenders in one batch with `task()`. 4. Collect their evidence and challenge each position against the strongest opposing result. 5. Synthesize the surviving arguments, dissent, conditions, and recommendation. Stop as blocked when evidence is missing or positions are not independent.</steps>
<output>Return the decision frame, positions, evidence references, objections, tradeoff table, dissent, recommendation, confidence, conditions, and open questions.</output>
<references>Use project constraints, measured evidence, current library documentation, Cortex evidence references, and ForgeSpec identifiers when relevant.</references>
