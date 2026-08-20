# Codebase Guide

Index and reading map for the `cortex-ia` codebase documentation. All pages live in [`codebase/`](codebase/) and link back to this index.

If you read only one page: [mental-model.md](codebase/mental-model.md).

---

## 1. Recommended Reading Path

| Step | Page | Why |
| :--- | :--- | :--- |
| 1 | [mental-model.md](codebase/mental-model.md) | End-to-end data flow: planning → snapshot verification → atomic apply → commit |
| 2 | [repository-map.md](codebase/repository-map.md) | Directory-by-directory map of active packages |
| 3 | [interfaces.md](codebase/interfaces.md) | Module boundaries defined by `ServiceAPI`, `Plan`, `Effect`, `Receipt` |
| 4 | [dashboard.md](codebase/dashboard.md) | Bubble Tea TUI architecture, 5-screen workflow, Lip Gloss themes |
| 5 | [project-and-extension.md](codebase/project-and-extension.md) | Adding OpenCode skills, agents, commands, and MCP presets |
| 6 | [sdd-coordination.md](codebase/sdd-coordination.md) | SDD workflow, sub-agent coordination, and file reservations |
| 7 | [sync-and-cloud.md](codebase/sync-and-cloud.md) | State management (`MetadataV2`), backup, and local sync |
| 8 | [maintainer-playbook.md](codebase/maintainer-playbook.md) | Release checklist, gates, and dependency updates |

---

## 2. Platform Support Status

- **OpenCode**: Fully supported active native target (`~/.config/opencode/`).
- **Google Antigravity**: Upcoming roadmap platform (`~/.gemini/antigravity/`).
- **Claude CLI**: Upcoming roadmap platform (`~/.claude/`).

