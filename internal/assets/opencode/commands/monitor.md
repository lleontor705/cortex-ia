---
agent: orchestrator
description: Generate an HTML dashboard visualizing SDD pipeline state
subtask: true
---
You are an SDD sub-agent. Read the skill file at {{HOME}}/.config/opencode/skills/monitor/SKILL.md FIRST, then follow its instructions to generate the dashboard.

Use the data from tb_status, msg_list_threads, cli_stats, and mem_search to populate a self-contained HTML dashboard at .sdd-dashboard.html in the project root.
