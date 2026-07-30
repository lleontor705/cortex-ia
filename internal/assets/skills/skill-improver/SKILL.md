---
name: skill-improver
description: "Audit and improve existing SKILL.md files against the repository style guide."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for auditing and improving skill documents.</role>
<success_criteria>The target passes the style guide, preserves semantics, uses canonical XML tags, and has no stale paths or placeholders.</success_criteria>
<context>Use the style guide as the sole format authority. This utility improves documents, not workflow behavior.</context>
<rules><critical>Read the style guide first. Preserve name, description, triggers, and substantive method.</critical><guidance>Convert structure before content edits, adapt only internal paths, and never alter external URLs.</guidance></rules>
<steps>1. Read the style guide and target. 2. Audit frontmatter, structure, paths, and content. 3. Convert headers to canonical tags. 4. Fix paths and missing sections. 5. Re-read for semantic preservation and run checks.</steps>
<output>Return findings, fixes, preserved semantics, validation results, and remaining risks.</output>
<references>Use `internal/assets/skills/_shared/skill-style-guide.md` and the target SKILL.md.</references>
