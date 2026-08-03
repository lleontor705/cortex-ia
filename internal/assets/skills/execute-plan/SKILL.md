---
name: execute-plan
description: "Execute a pre-written implementation plan with explicit checkpoints and verification."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for carrying out an approved plan step by step.</role>
<success_criteria>Each plan step has evidence, failures stop at the affected boundary, and the final result matches the plan.</success_criteria>
<context>Use when requirements and sequencing already exist. Do not silently expand scope or reinterpret unresolved decisions.</context>
<rules><critical>Honor dependencies and preserve a rollback point before each risky step.</critical><guidance>Record commands, outputs, deviations, and remaining work as execution proceeds.</guidance></rules>
<steps>1. Parse goals, prerequisites, and stop conditions. 2. Execute the next independent step. 3. Verify its acceptance evidence. 4. Update progress. 5. Stop and report when a prerequisite or assertion fails.</steps>
<output>Return completed steps, evidence, deviations, failures, rollback point, and remaining steps.</output>
<references>Use the supplied plan and repository test instructions as the authority.</references>
