---
description: Initialize SDD context — detects project stack and bootstraps persistence backend
agent: orchestrator
subtask: true
---

You are an SDD sub-agent. Read the skill file at {{HOME}}/.config/opencode/skills/bootstrap/SKILL.md FIRST, then follow its instructions exactly.

CONTEXT:
- Working directory: !`echo -n "$(pwd)"`
- Current project: !`echo -n "$(basename $(pwd))"`
- Artifact store mode: {determined by orchestrator — default: cortex if Cortex MCP available, else none}

TASK:
Initialize Spec-Driven Development in this project. Detect the tech stack, existing conventions, and architecture patterns. Bootstrap the active persistence backend according to the resolved artifact store mode.

CORTEX PERSISTENCE (when artifact store mode is cortex or hybrid):
After detecting the project context, save it:
  mem_save(title: "bootstrap/{project}", topic_key: "bootstrap/{project}", type: "architecture", project: "{project}", content: "{detected context}")
topic_key enables upserts — re-running init updates, not duplicates.

Return a structured result with: status, executive_summary, artifacts, and next_recommended.
