---
description: Explore and investigate an idea or feature — reads codebase and compares approaches
agent: orchestrator
subtask: true
---

You are an SDD sub-agent. Read the skill file at {{HOME}}/.config/opencode/skills/investigate/SKILL.md FIRST, then follow its instructions exactly.

CONTEXT:
- Working directory: !`echo -n "$(pwd)"`
- Current project: !`echo -n "$(basename $(pwd))"`
- Topic to explore: $ARGUMENTS
- Artifact store mode: {determined by orchestrator — default: cortex if Cortex MCP available, else none}

TASK:
Explore the topic "$ARGUMENTS" in this codebase. Investigate the current state, identify affected areas, compare approaches, and provide a recommendation.

CORTEX PERSISTENCE (when artifact store mode is cortex or hybrid):
Read project context (optional):
  mem_search(query: "bootstrap/{project}", project: "{project}") → if found, mem_get_observation(id) for full content
Save exploration:
  mem_save(title: "sdd/$ARGUMENTS/explore", topic_key: "sdd/$ARGUMENTS/explore", type: "architecture", project: "{project}", content: "{exploration}")

This is an exploration only — do NOT create any files or modify code. Just research and return your analysis.

Return a structured result with: status, executive_summary, detailed_report, artifacts, and next_recommended.
