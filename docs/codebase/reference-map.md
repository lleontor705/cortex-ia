# Reference Map

← [Codebase Guide](../CODEBASE-GUIDE.md)

Quick-reference index of CLI commands, Go packages, key types, MCP tools, config files, and documentation pages. This page is a lookup table — for explanations see the sibling pages linked below. Use `Ctrl+F` to find any identifier.

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| CLI commands | ✅ | — |
| Go packages | ✅ | — |
| Key types | ✅ | — |
| MCP servers & tools | ✅ | — |
| Config files | ✅ | — |
| Documentation pages | ✅ | — |
| Architecture explanation | — | [mental-model.md](mental-model.md) |

## CLI Commands

| Command | Purpose |
|---------|---------|
| `cortex-ia` (no args) | Launch interactive TUI dashboard |
| `cortex-ia install` | Install agents + components from selection |
| `cortex-ia sync` | Reconcile installed state with saved config (local) |
| `cortex-ia detect` | Detect installed AI agents on the system |
| `cortex-ia doctor` | Verify installation health (alias: `verify`) |
| `cortex-ia verify` | Alias for `doctor` |
| `cortex-ia repair` | Repair a crashed/incomplete installation |
| `cortex-ia rollback` | Restore files from a backup snapshot |
| `cortex-ia uninstall` | Remove cortex-ia managed files |
| `cortex-ia gga` | Guardian Angel pre-commit review config |
| `cortex-ia profiles` | Manage named preset profiles |
| `cortex-ia agent-builder` | Build a custom agent adapter |
| `cortex-ia update` | Self-update to latest release |
| `cortex-ia config` | View/edit project `.cortex-ia.yaml` |
| `cortex-ia list` | List installed agents, components, skills |
| `cortex-ia init` | Initialize cortex-ia in a project |
| `cortex-ia skill` | Manage skills |
| `cortex-ia auto-install` | Auto-detect and install |

## Go Packages

| Package | Location | One-liner |
|---------|----------|-----------|
| Entry point | `cmd/cortex-ia/` | `main.go` — ldflags version injection |
| CLI dispatch | `internal/app/` | Subcommand routing + TUI launch (`app.go`, `version.go`) |
| Core types | `internal/model/` | Shared type vocabulary leaf (`types.go`, `selection.go`) |
| Agents | `internal/agents/` | Adapter interface + registry + 12 agent packages |
| Catalog | `internal/catalog/` | Component/skill registry, `ResolveDeps()` topological sort |
| Components | `internal/components/` | MCP injection engine + file-merge primitives + per-component injectors |
| Pipeline | `internal/pipeline/` | 2-stage install (Prepare → Apply), parallel chains, rollback |
| Assets | `internal/assets/` | `go:embed` access to skills, prompts, commands |
| Backup | `internal/backup/` | Snapshotter, manifest, restore, prune |
| State | `internal/state/` | `state.json` + lockfile persistence |
| System detect | `internal/system/` | OS/platform/package-manager detection |
| Config | `internal/config/` | Project `.cortex-ia.yaml` handling |
| Agent builder | `internal/agentbuilder/` | `agent-builder` subcommand support |
| OpenCode | `internal/opencode/` | OpenCode model-assignment application |
| Self-update | `internal/update/` | `cortex-ia update` support |
| Verify | `internal/verify/` | `doctor` verification logic |
| TUI | `internal/tui/` | Bubbletea dashboard, 28 screens |

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `AgentID` | `internal/model/types.go` | Identifies an AI agent (e.g., `claude`, `opencode`) |
| `ComponentID` | `internal/model/types.go` | Identifies an injectable component |
| `SkillID` | `internal/model/types.go` | Identifies a skill |
| `MCPStrategy` | `internal/model/types.go` | MCP injection strategy (separate file / merge / config file / TOML) |
| `SystemPromptStrategy` | `internal/model/types.go` | Prompt injection strategy (markdown sections / append / replace) |
| `PresetID` | `internal/model/types.go` | Identifies a preset bundle |
| `PersonaID` | `internal/model/types.go` | Identifies a communication-style persona |
| `ModelAssignments` | `internal/model/types.go` | Phase → model alias mapping |
| `OpenCodeModel` | `internal/model/types.go` | OpenCode model configuration |
| `Selection` | `internal/model/selection.go` | User's chosen agents + components + options |
| `Adapter` | `internal/agents/interface.go` | 23-method interface every agent implements |
| `RollbackStep` | `internal/pipeline/` | Interface for reversible pipeline steps |

## MCP Servers

| Server | Binary | Purpose | Injection |
|--------|--------|---------|-----------|
| Cortex | `cortex` | Persistent memory + knowledge graph | `internal/components/cortex/` |
| Mailbox | (npm) | Agent messaging + A2A task protocol | `internal/components/mailbox/` |
| ForgeSpec | (npm) | SDD contracts + task boards + file reservations | `internal/components/forgespec/` |
| Context7 | (npm/remote) | Library documentation lookup | `internal/components/context7/` |

## Config & Build Files

| File | Location | Purpose |
|------|----------|---------|
| `go.mod` | root | Module `github.com/lleontor705/cortex-ia`, Go 1.26.1 |
| `Makefile` | root | build, test, lint, fmt, tidy, docker, install, security, check |
| `.goreleaser.yaml` | root | Cross-platform release config |
| `.golangci.yml` | root | Lint config (errcheck, govet, staticcheck, unused, ineffassign) |
| `Dockerfile` | root | Multi-stage Alpine build |
| `.gitattributes` | root | Pins `testdata/golden/**` to `eol=lf` |
| `.github/workflows/ci.yml` | `.github/workflows/` | Quality + security CI |
| `.github/workflows/release.yml` | `.github/workflows/` | Tag-triggered release pipeline |
| `.github/workflows/pr-check.yml` | `.github/workflows/` | PR compliance checks |
| `.github/workflows/stale.yml` | `.github/workflows/` | Issue/PR lifecycle (30d stale, 14d close) |
| `scripts/install.sh` | `scripts/` | Curl-pipe installer with SHA-256 verify |

## State Files

| File | Location | Purpose |
|------|----------|---------|
| `state.json` | `~/.cortex-ia/` | Installed agents, preset, components — sync source of truth |
| `cortex-ia.lock` | `~/.cortex-ia/` | Concrete written-file list with checksums |
| `install-status.json` | `~/.cortex-ia/` | Crash-detection marker (ephemeral) |
| `profiles/` | `~/.cortex-ia/` | Named preset profiles |
| `skills/` | `~/.cortex-ia/` | Installed skill manifests |

## Key Counts

| Metric | Value |
|--------|-------|
| AI agents | 12 |
| Components | 14 |
| Skills | ~27 (see `catalog/skills.go`) |
| CLI subcommands | 18 |
| TUI screens | 28 |
| Adapter interface methods | 23 |
| Build targets | 6 (3 OS × 2 arch) |
| Go version | 1.26.1 |

## Documentation Pages

| Page | Location | Covers |
|------|----------|--------|
| Mental Model | `docs/codebase/mental-model.md` | End-to-end data flow |
| Repository Map | `docs/codebase/repository-map.md` | Directory-by-directory index |
| MCP Boundaries | `docs/codebase/mcp-boundaries.md` | MCP server contracts & limits |
| SDD Coordination | `docs/codebase/sdd-coordination.md` | Spec-Driven Development workflow |
| Interfaces | `docs/codebase/interfaces.md` | Go interface contracts |
| Sync, State & Backup | `docs/codebase/sync-and-cloud.md` | State files, snapshots, local sync |
| Dashboard & TUI | `docs/codebase/dashboard.md` | Bubbletea architecture, screens |
| Integrations | `docs/codebase/integrations.md` | CI/CD, GoReleaser, installer |
| Project & Extension | `docs/codebase/project-and-extension.md` | Adding agents/skills/components/subcommands |
| Maintainer Playbook | `docs/codebase/maintainer-playbook.md` | Release & dependency process |
| Reference Map | `docs/codebase/reference-map.md` | This page — quick lookup index |
| Architecture | `docs/architecture.md` | High-level architecture overview |
| Agents | `docs/AGENTS.md` | Per-agent reference (12 agents) |
| SDD Workflow | `docs/sdd-workflow.md` | SDD skills & workflow docs |

## Invariants

- This page is a lookup index — if you need explanation, follow the sibling page link.
- Counts are maintained manually; update them when adding agents/skills/components.
- Package locations reflect `internal/` structure; `cmd/` is the only non-internal importable package.
- MCP servers map 1:1 to component packages under `internal/components/`.

## Contributor Checklist

- [ ] Added a new agent/skill/component/subcommand? Update the corresponding table and count above.
- [ ] Added a new Go package? Add it to the Go Packages table.
- [ ] Added a new config/build file? Add it to the Config & Build Files table.
- [ ] Added a new doc page? Add it to the Documentation Pages table.
- [ ] Keep this page as a pure index — move explanations to sibling pages.

---

← Prev: [Maintainer Playbook](maintainer-playbook.md) · Next: (end) →
