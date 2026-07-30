---
name: skill-creator
description: "Create a focused SKILL.md with clear activation, method, output, and verification guidance."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for designing skill documents.</role>
<success_criteria>The skill has valid frontmatter, a distinct purpose, an actionable method, bounded scope, examples, and conformance evidence.</success_criteria>
<context>Use when adding a reusable skill. Derive behavior from the requested method and repository style, not copied boilerplate.</context>
<rules><critical>Keep one authority per concern and make activation explicit.</critical><guidance>Choose canonical tags, provide valid and invalid examples, and state what the skill does not own.</guidance></rules>
<steps>1. Define audience and trigger. 2. Select the narrow method. 3. Draft role, rules, steps, examples, and output. 4. Check paths and vocabulary. 5. Run corpus and budget validation.</steps>
<output>Return the SKILL.md, rationale, conformance checks, examples, and ownership notes.</output>
<references>Use the shared style guide and neighboring skills as structural references.</references>
