# Codebase Guide

Index and reading map for the cortex-ia codebase documentation. All pages live in [`codebase/`](codebase/) and link back to this index.

If you read only one page: [mental-model.md](codebase/mental-model.md).

## Recommended Reading Path

| Step | Page | Why |
|------|------|-----|
| 1 | [mental-model.md](codebase/mental-model.md) | End-to-end data flow: selection → catalog → pipeline → injection |
| 2 | [repository-map.md](codebase/repository-map.md) | Directory-by-directory map of the source tree |
| 3 | [interfaces.md](codebase/interfaces.md) | Module boundaries defined by Go interfaces and types |
| 4 | [mcp-boundaries.md](codebase/mcp-boundaries.md) | The four MCP servers (Cortex, Mailbox, ForgeSpec, Context7) |
| 5 | [sdd-coordination.md](codebase/sdd-coordination.md) | Multi-agent SDD pipeline, messaging, A2A, file reservations |
| 6 | [dashboard.md](codebase/dashboard.md) | Bubbletea TUI architecture and the 28-screen router |
| 7 | [sync-and-cloud.md](codebase/sync-and-cloud.md) | State management, backup, local sync, cloud roadmap |
| 8 | [integrations.md](codebase/integrations.md) | GoReleaser, GitHub Actions, install scripts, Homebrew |
| 9 | [project-and-extension.md](codebase/project-and-extension.md) | How to add agents, skills, components, CLI commands |
| 10 | [maintainer-playbook.md](codebase/maintainer-playbook.md) | Release checklist, golden files, dependency updates |
| 11 | [reference-map.md](codebase/reference-map.md) | Quick-reference lookup of types, packages, commands |

Newcomers should read top to bottom. Jump in anywhere if you know what you need.

## Audience Guide

| Audience | Start Here | Then Read | Deep Dive |
|----------|-----------|-----------|-----------|
| **Newcomer** | [mental-model.md](codebase/mental-model.md) → [repository-map.md](codebase/repository-map.md) | [mcp-boundaries.md](codebase/mcp-boundaries.md) → [dashboard.md](codebase/dashboard.md) | [sdd-coordination.md](codebase/sdd-coordination.md) |
| **Contributor** | [interfaces.md](codebase/interfaces.md) → [project-and-extension.md](codebase/project-and-extension.md) | [mental-model.md](codebase/mental-model.md) → [repository-map.md](codebase/repository-map.md) | [sdd-coordination.md](codebase/sdd-coordination.md) → [reference-map.md](codebase/reference-map.md) |
| **Maintainer** | [maintainer-playbook.md](codebase/maintainer-playbook.md) → [integrations.md](codebase/integrations.md) | [sync-and-cloud.md](codebase/sync-and-cloud.md) → [reference-map.md](codebase/reference-map.md) | [sdd-coordination.md](codebase/sdd-coordination.md) |

## Page Index

| # | Page | What You'll Learn | Audience |
|---|------|-------------------|----------|
| 1 | [mental-model.md](codebase/mental-model.md) | End-to-end flow: selection → catalog → pipeline → agent injection | Newcomer |
| 2 | [repository-map.md](codebase/repository-map.md) | Directory-by-directory map of the codebase | Newcomer |
| 3 | [mcp-boundaries.md](codebase/mcp-boundaries.md) | The four MCP servers: Cortex, Mailbox, ForgeSpec, Context7 | Newcomer, Contributor |
| 4 | [sdd-coordination.md](codebase/sdd-coordination.md) | Multi-agent SDD pipeline, P2P messaging, A2A, file reservations | Contributor, Maintainer |
| 5 | [interfaces.md](codebase/interfaces.md) | Key Go interfaces and types defining module boundaries | Contributor |
| 6 | [sync-and-cloud.md](codebase/sync-and-cloud.md) | State management, backup, local sync, cloud roadmap | Maintainer |
| 7 | [dashboard.md](codebase/dashboard.md) | Bubbletea TUI architecture, 28-screen router | Contributor |
| 8 | [integrations.md](codebase/integrations.md) | GoReleaser, GitHub Actions, install scripts, Homebrew tap | Maintainer |
| 9 | [project-and-extension.md](codebase/project-and-extension.md) | How to add agents, skills, components, CLI commands | Contributor |
| 10 | [maintainer-playbook.md](codebase/maintainer-playbook.md) | Release checklist, golden files, dependency updates | Maintainer |
| 11 | [reference-map.md](codebase/reference-map.md) | Quick-reference index of types, packages, commands | All |

## Quick Navigation

1. [Mental Model](codebase/mental-model.md)
2. [Repository Map](codebase/repository-map.md)
3. [MCP Boundaries](codebase/mcp-boundaries.md)
4. [SDD Coordination](codebase/sdd-coordination.md)
5. [Interfaces](codebase/interfaces.md)
6. [Sync and Cloud](codebase/sync-and-cloud.md)
7. [Dashboard](codebase/dashboard.md)
8. [Integrations](codebase/integrations.md)
9. [Project and Extension](codebase/project-and-extension.md)
10. [Maintainer Playbook](codebase/maintainer-playbook.md)
11. [Reference Map](codebase/reference-map.md)
