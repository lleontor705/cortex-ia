# Codebase Guide

Index and reading map for the `cortex-ia` codebase documentation. All pages live in [`codebase/`](codebase/) and link back to this index.

If you read only one page: [mental-model.md](codebase/mental-model.md).

---

## 1. What cortex-ia Is

`cortex-ia` is the local bridge and control plane for the OpenCode ecosystem. It installs the native OpenCode asset set, manages MCP configuration, and owns durable task authority in SQLite. Execution is native-only: every role controller runs inside OpenCode, there is no external execution leaf, and nothing is supervised through Herdr. The retired ForgeSpec surface and external task-board MCP are not part of the product.

The selected specification plane owns SDD contracts: OpenSpec for `openspec|hybrid`, pinned Cortex observations for `cortex`. Cortex MCP also owns durable evidence, memories, AST knowledge, and relationships. Neither replaces SQLite task authority.

## 2. Architecture Summary

| Layer | Package | Responsibility |
| :--- | :--- | :--- |
| Entry point | `cmd/cortex-ia/` | Sets the release version and calls `internal/app`; no install, merge, or ownership logic. |
| CLI dispatch | `internal/app` | Parses intent and renders receipts; split between argument dispatch and the zero-argument Bubble Tea TUI. |
| Service facade | `internal/install` | Owns install, sync, doctor, rollback, uninstall, and every MCP operation; all ownership decisions belong here. |
| Transactional pipeline | `internal/pipeline` | Plans first, captures a verified backup, applies atomic writes, and restores from backup if apply fails. |
| MCP catalog | `internal/mcpmanager` | Managed preset catalog, desired-entry validation, qualification, and typed fail-closed conflicts. |
| Work authority | `internal/delegation` | SQLite schema and migrations, task boards, DAG state, claims, TTL file leases, approvals, and recovery. |
| Web console | `internal/cortexiaweb` | Preact operations console embedded via `go:embed`, served loopback-only over HTTP/API. |
| Asset mapping | `internal/agents/opencode` | Declares the OpenCode native layout (`layout.go`) and the pure asset mapping (`assetmap.go`). |
| Merge helper | `internal/components/filemerge` | JSONC decode/merge and atomic writes; reused instead of ad-hoc merge code. |
| Embedded assets | `internal/assets/` | Runtime skills, prompts, commands, and plugins embedded with `go:embed`; changing them requires rebuilding the binary. |

Skills, prompts, commands, and plugins under `internal/assets/` are runtime source files. Shared skill contracts under `internal/assets/skills/_shared/` support installed agent instructions and must stay aligned with the role files.

---

## 3. Recommended Reading Path

| Step | Page | Why |
| :--- | :--- | :--- |
| 1 | [mental-model.md](codebase/mental-model.md) | End-to-end data flow: planning → snapshot verification → atomic apply → commit |
| 2 | [repository-map.md](codebase/repository-map.md) | Directory-by-directory map of active packages |
| 3 | [interfaces.md](codebase/interfaces.md) | Module boundaries defined by `ServiceAPI`, `Plan`, `Effect`, `Receipt` |
| 4 | [dashboard.md](codebase/dashboard.md) | Bubble Tea TUI architecture, 5-screen workflow, Lip Gloss themes |
| 5 | [mcp-boundaries.md](codebase/mcp-boundaries.md) | MCP catalog ownership, qualification, and fail-closed conflict rules |
| 6 | [project-and-extension.md](codebase/project-and-extension.md) | Adding OpenCode skills, agents, commands, and MCP presets |
| 7 | [sdd-coordination.md](codebase/sdd-coordination.md) | SDD workflow, sub-agent coordination, and file reservations |
| 8 | [sync-and-cloud.md](codebase/sync-and-cloud.md) | State management (`MetadataV2`), backup, and local sync |
| 9 | [integrations.md](codebase/integrations.md) | Release pipeline, CI workflows, installer script, and distribution |
| 10 | [maintainer-playbook.md](codebase/maintainer-playbook.md) | Release checklist, gates, and dependency updates |
| — | [reference-map.md](codebase/reference-map.md) | Lookup table for CLI commands, Go packages, key types, and config files |

---

## 4. Platform Support Status

- **OpenCode**: Fully supported active native target (`~/.config/opencode/`).
- **Google Antigravity**: No committed support; any future evaluation stays native-only with no external execution (`~/.gemini/antigravity/`).
- **Claude CLI**: Upcoming roadmap platform (`~/.claude/`).

---

## See Also

- [`architecture.md`](architecture.md) — Internal engine layers and package responsibilities
- [`agents.md`](agents.md) — 6-role topology, authority invariants, and typed receipt contracts
- [`sdd-workflow.md`](sdd-workflow.md) — SDD lifecycle with role-specific responsibilities
- [`mcp.md`](mcp.md) — Operator-facing MCP management reference
