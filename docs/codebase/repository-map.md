# Repository Map

← [Codebase Guide](../CODEBASE-GUIDE.md)

Directory-by-directory map of the cortex-ia codebase. This page is a navigational index of where things live and what owns them — it does not explain how to extend the system (see [mental-model.md](mental-model.md)) or list Go contracts (see [interfaces.md](interfaces.md)).

## Top-Level

| Path | Purpose | Key Files |
|------|---------|-----------|
| `cmd/cortex-ia/` | Entry point. ldflags version injection. | `main.go` |
| `go.mod` | Module `github.com/lleontor705/cortex-ia`, Go 1.26.1. Deps: bubbletea, bubbles, lipgloss, yaml.v3. | — |

## internal/app

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/app/` | CLI dispatch + TUI launch. Routes subcommands to handlers. | `app.go`, `version.go` |

CLI subcommands: `install`, `sync`, `detect`, `doctor`/`verify`, `repair`, `rollback`, `uninstall`, `gga`, `profiles`, `agent-builder`, `update`, `config`, `list`, `init`, `skill`, `auto-install`.

## internal/model

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/model/` | Core type definitions shared across all packages. | `types.go`, `selection.go` |

Defines: `AgentID`, `ComponentID`, `SkillID`, `MCPStrategy`, `SystemPromptStrategy`, `PresetID`, `PersonaID`, `ModelAssignments`, `OpenCodeModel` types.

## internal/agents

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/agents/` | Adapter interface + registry + per-agent implementations. | `interface.go`, `registry.go`, `factory.go`, `errors.go` |
| `internal/agents/<name>/` | One package per agent (12 total). | `<name>/adapter.go` |

Agents: `claude`, `opencode`, `gemini`, `cursor`, `vscode`, `codex`, `windsurf`, `antigravity`, `kilocode`, `kimi`, `kiro`, `qwen`. Registry preserves insertion order; `factory.go` builds the default registry.

## internal/catalog

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/catalog/` | Component registry, dependency declarations, preset expansion, `ResolveDeps()` topological sort. | `components.go`, `skills.go` |

## internal/components

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/components/mcpinject/` | Shared MCP injection engine. Per-strategy `ServerTemplates` + dispatch. | `mcpinject.go` |
| `internal/components/filemerge/` | File-operation primitives used by all injectors. | `section.go`, `json_merge.go`, `toml.go`, `writer.go` |
| `internal/components/cortex/` | Cortex MCP server injection (Go binary). | — |
| `internal/components/mailbox/` | Agent Mailbox MCP server injection (npm). | — |
| `internal/components/forgespec/` | ForgeSpec MCP server injection (npm). | — |
| `internal/components/context7/` | Context7 MCP server injection (npm/remote). | `config.go`, `inject.go` |
| `internal/components/sdd/` | SDD workflow injection: orchestrator prompt + 19 skills + commands + sub-agents. | `inject.go` |
| `internal/components/skills/` | Non-SDD utility skills injection (with SDD-skip logic). | `inject.go` |
| `internal/components/conventions/` | Cortex convention + memory protocol injection. | `inject.go` |
| `internal/components/gga/` | Guardian Angel pre-commit code review hook config + provider switcher. | `config.go` |
| `internal/components/persona/` | Communication-style persona injection. | — |
| `internal/components/permissions/` | Permissions component. | — |
| `internal/components/theme/` | Terminal theme component. | — |
| `internal/components/uninstall/` | Uninstall command support. | — |

## internal/pipeline

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/pipeline/` | 2-stage install pipeline (Prepare → Apply), parallel-chain execution, rollback. | `pipeline.go`, `runner.go`, `types.go` |

## internal/assets

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/assets/` | `go:embed` access to all injectable content. Ships in the binary. | `assets.go` |
| `internal/assets/skills/` | 19+ `SKILL.md` files + `_shared/` conventions. | `skills/<skill>/SKILL.md` |
| `internal/assets/generic/` | Orchestrator prompts + cortex protocol. | `generic/sdd-orchestrator.md`, `generic/sdd-orchestrator-reference.md` |
| `internal/assets/opencode/` | OpenCode slash commands. | `opencode/commands/*.md` |
| `internal/assets/gga/` | GGA review templates. | — |

Embed directive: `//go:embed all:skills all:generic all:opencode all:gga`.

## Supporting packages

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/backup/` | File snapshotter, JSON manifest, restore from snapshot. | `snapshot.go`, `manifest.go`, `restore.go` |
| `internal/state/` | `~/.cortex-ia/state.json` + lockfile persistence; shared dirs. | `state.go` |
| `internal/system/` | OS/platform/package-manager detection. | `detect.go` |
| `internal/config/` | Project `.cortex-ia.yaml` config handling. | — |
| `internal/agentbuilder/` | `cortex-ia agent-builder` support. | — |
| `internal/opencode/` | OpenCode config model-assignment application. | — |
| `internal/update/` | `cortex-ia update` self-update support. | — |
| `internal/verify/` | `cortex-ia doctor` verification logic. | — |

## internal/tui

| Path | Purpose | Key Files |
|------|---------|-----------|
| `internal/tui/` | Bubbletea model with 6 screens. | `tui.go` |
| `internal/tui/screens/` | Screen implementations. | — |
| `internal/tui/styles/` | Colors and layout styles (lipgloss). | `theme.go` |

## Planned packages (not yet present)

> The following directories are part of the in-progress `port-gentle-ai-patterns` change and do **not** exist in the tree yet. Listed here so contributors don't go looking for them.

| Path | Planned purpose |
|------|----------------|
| `internal/skillregistry/` | Deterministic Go skill-registry builder + `cortex-ia skill-registry` CLI subcommand. |
| `internal/planner/` | Dependency-driven planner separating resolution from execution ordering. |

## Project-level

| Path | Purpose |
|------|---------|
| `docs/` | Documentation (`architecture.md`, `agents.md`, `sdd-workflow.md`, `codebase/`). |
| `scripts/install.sh` | Curl-pipe installer. |
| `.goreleaser.yaml` | Cross-platform release config. |
| `.golangci.yml` | Lint config (errcheck, govet, staticcheck, unused, ineffassign). |
| `Makefile` | Build, test, lint, coverage, docker, install targets. |
| `Dockerfile` | Multi-stage Alpine build. |
| `.github/workflows/` | CI, PR checks, release, stale. |
| `skills/` | Project-level community skills (`issue-creation`, `branch-pr`). |

## Invariants

- `cmd/` is the only package importable by `main`; everything else lives under `internal/`.
- `internal/model` has zero imports from other internal packages — it is the shared vocabulary leaf.
- `internal/assets` embeds at compile time; no external files are read at runtime.
- `internal/components/filemerge` is the only package allowed to write managed files.

## Contributor Checklist

- [ ] New package? Place it under `internal/`. Add it to this map.
- [ ] New CLI subcommand? Add the case to `internal/app/app.go:runCLI` and document it in `printHelp()`.
- [ ] New embedded asset? Put it under `internal/assets/<dir>/` and ensure the `//go:embed` directive covers it.
- [ ] New testdata? Use `testdata/golden/` for component output fixtures and `t.TempDir()` for isolation.
- [ ] Confirm `make lint` and `make test` pass before committing.

---

← Prev: [Mental Model](mental-model.md) · Next: [MCP Boundaries](mcp-boundaries.md) →
