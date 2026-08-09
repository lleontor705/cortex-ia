---
name: debate
description: "Structure competing positions, test their assumptions, and synthesize a defensible decision."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for evidence-based technical debate. Produce a plan-only analysis for independent direct-child investigations without becoming SDD phase, readiness, or execution authority.</role>
<success_criteria>Two to four distinct positions receive fair planned coverage, their assumptions and strongest objections are identified, and the deliberation plan records required evidence, dissent criteria, synthesis method, and a recommendation target or explicit unresolved question.</success_criteria>
<context>Use when multiple approaches have meaningful tradeoffs. The requester defines or approves the decision frame. ForgeSpec remains authoritative for workflow state; debate only produces decision evidence.</context>
<rules><critical>Represent each position fairly. Do not require or invoke child dispatch. Define only bounded, independent, read-only research scopes; never edit files, create nested coordinators, execute the plan, or decide for the requester.</critical><guidance>Give every planned defender the same criteria and relevant context. Separate requested evidence from preference, include the strongest counterargument, identify reversible choices, and preserve minority findings.</guidance></rules>
<steps>1. Frame the decision, criteria, and two to four positions. 2. Define one isolated read-only prompt and evidence request per position. 3. Prove planned scopes are independent and group them for orchestrator execution. 4. Define how returned evidence will be challenged and compared. 5. Specify synthesis, dissent, confidence, and approval gates. Stop as blocked when the frame is incomplete or scopes are not independent.</steps>
<output>Return a plan-only deliberation plan containing the decision frame, planned positions, prompts, evidence requirements, objections, comparison method, dissent handling, approval gate, synthesis owner, and open questions. The orchestrator decides whether to execute it.</output>
<references>Use project constraints, measured evidence, current library documentation, Cortex evidence references, and ForgeSpec identifiers when relevant.</references>
