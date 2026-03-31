# CLAUDE.md

This file provides guidance to Claude Code when working on this repository.

## What This Is

**cortex-ia** is a Go CLI tool that configures AI coding agents with an ecosystem of MCP servers and SDD (Spec-Driven Development) workflow. It supports 8 agents (Claude Code, OpenCode, Gemini CLI, Cursor, VS Code Copilot, Codex, Windsurf, Antigravity) and injects 5 MCP servers (68 tools) + 19 SDD skills + orchestrator prompts + permissions + persona + theme.

## Architecture

- **Adapter pattern**: Each agent in `internal/agents/` implements the `Adapter` interface (21 methods). Components query adapters for paths and strategies — no agent-specific switch statements.
- **Strategy dispatch**: MCP injection uses `mcpinject.ServerTemplates` for per-strategy templates. Each MCP component defines templates and delegates to `mcpinject.Inject()`.
- **2-stage pipeline**: `Install()` uses Prepare (validate+backup) → Apply (components). Agents run in parallel via `RunParallelChains`. Components run sequentially per agent.
- **Topological sort**: `catalog.TopoSort()` uses Kahn's algorithm with `ParallelGroups` for dependency ordering.
- **Marker-based injection**: System prompt injection uses `<!-- cortex-ia:ID -->` markers via `filemerge.InjectMarkdownSection()`. Content outside markers is never touched.
- **Shared skills directory**: Skills are written to `~/.cortex-ia/skills/` (shared by all agents). Convention references use absolute paths.
- **Embedded assets**: All skills, prompts, and commands are embedded via `go:embed` in `internal/assets/`.

## Build & Test

```bash
go build ./cmd/cortex-ia              # Build binary
go test ./...                          # Run all tests (175 across 23 packages)
go run ./cmd/cortex-ia detect          # Test agent + system detection
go run ./cmd/cortex-ia install --dry-run  # Test pipeline without changes
go run ./cmd/cortex-ia doctor          # Run health checks
```

## Key Directories

- `internal/assets/skills/` — 19 SDD skill files (SKILL.md) + `_shared/cortex-convention.md`
- `internal/assets/generic/` — Orchestrator prompts (multi + single) + persona files + cortex protocol
- `internal/assets/opencode/commands/` — 10 slash commands for OpenCode
- `internal/components/` — Injection logic: sdd, conventions, cortex, forgespec, mailbox, orchestrator, context7, permissions, persona, theme
- `internal/agents/` — 8 agent adapters
- `internal/pipeline/` — 2-stage orchestrator, steps, runner (parallel chains)
- `internal/catalog/` — Component catalog, dependency resolution, topological sort
- `internal/verify/` — Health check framework (6 checks)
- `internal/config/` — Project-level .cortex-ia.yaml config
- `internal/model/` — Types, model routing presets, selection
- `internal/update/` — Self-update via GitHub releases
- `e2e/` — Docker E2E tests (Ubuntu + Fedora)

## Conventions

- Components use `adapter.MCPStrategy()` to dispatch injection — never switch on agent ID.
- All file writes go through `filemerge.WriteFileAtomic()` (temp + rename, no-op if identical).
- SDD skills are written to shared dir (`~/.cortex-ia/skills/`), not per-agent.
- Tests use `t.TempDir()` for isolation — no real agent configs are modified.
- Convention references in skills use absolute paths (resolved at injection time via `fixConventionRefs`).
- All SDD skills must call `sdd_validate` + `sdd_save` before returning contracts.
