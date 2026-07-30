---
name: file-issue
description: "Create a precise issue from a verified bug, feature request, or maintenance need."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---
<role>Non-phase utility authority for writing actionable repository issues.</role>
<success_criteria>The issue states context, reproduction or motivation, expected and actual behavior, scope, and acceptance criteria.</success_criteria>
<context>Use issue creation when work needs durable triage. Separate observed facts from hypotheses and link evidence without exposing secrets.</context>
<rules><critical>Issue titles describe the outcome. Reproduction steps must be deterministic where possible.</critical><guidance>Classify impact, constraints, non-goals, and regression risk. Never invent evidence.</guidance></rules>
<steps>1. Gather the symptom or request. 2. Confirm the smallest reproducible case. 3. Write expected versus actual behavior. 4. Add acceptance criteria and labels suggested by repository policy. 5. Review for sensitive data.</steps>
<output>Return title, body, evidence, acceptance criteria, scope, risks, and suggested labels.</output>
<references>Follow the repository issue template and contribution guide.</references>
