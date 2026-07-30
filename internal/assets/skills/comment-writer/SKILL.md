---
name: comment-writer
description: "Write concise, contextual code and review comments that explain intent and constraints."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for durable technical comments.</role>
<success_criteria>Comments explain non-obvious intent, remain accurate, match local tone, and avoid duplicating the implementation.</success_criteria>
<context>Use when future readers need rationale, invariants, or external constraints. Prefer clear names over comments for obvious behavior.</context>
<rules><critical>Describe why, not what. Keep comments near the invariant they protect.</critical><guidance>Use neutral language, cite durable references, and update comments when behavior changes.</guidance></rules>
<steps>1. Identify the reader question. 2. Confirm the invariant or constraint. 3. Draft the shortest useful explanation. 4. Check accuracy and tone. 5. Place it beside the relevant code.</steps>
<output>Return comment text, placement, rationale, and reference links when applicable.</output>
<references>Follow repository language, lint, and documentation conventions.</references>
