# CLAUDE.md

This file provides guidance to Claude Code when working on this repository.

## What This Is

**cortex-ia** is a Go CLI tool that configures AI coding agents with an ecosystem of MCP servers and SDD (Spec-Driven Development) workflow. It supports 8 agents (Claude Code, OpenCode, Gemini CLI, Cursor, VS Code Copilot, Codex, Windsurf, Antigravity) and injects 5 MCP servers + 19 SDD skills + orchestrator prompts.

## Architecture

- **Adapter pattern**: Each agent in `internal/agents/` implements the `Adapter` interface. Components query adapters for paths and strategies — no agent-specific switch statements in components.
- **Strategy dispatch**: MCP injection uses `mcpinject.ServerTemplates` to define per-strategy templates. Each MCP component (cortex, forgespec, mailbox, orchestrator, context7) defines templates and delegates to `mcpinject.Inject()`.
- **Marker-based injection**: System prompt injection uses `<!-- cortex-ia:ID -->` markers via `filemerge.InjectMarkdownSection()`. Content outside markers is never touched.
- **Embedded assets**: All skills, prompts, and commands are embedded via `go:embed` in `internal/assets/`. Skills are copied to agent config dirs during install.

## Build & Test

```bash
go build ./cmd/cortex-ia    # Build binary
go test ./...               # Run all tests (60+)
go run ./cmd/cortex-ia detect       # Test agent detection
go run ./cmd/cortex-ia install --dry-run  # Test pipeline without changes
```

## Key Directories

- `internal/assets/skills/` — 19 SDD skill files (SKILL.md) + shared conventions
- `internal/assets/generic/` — Orchestrator prompts + cortex protocol
- `internal/assets/opencode/commands/` — 10 slash commands for OpenCode
- `internal/components/` — Injection logic for each component
- `internal/agents/` — 8 agent adapters

## Conventions

- Components use `adapter.MCPStrategy()` to dispatch injection — never switch on agent ID.
- All file writes go through `filemerge.WriteFileAtomic()` (temp + rename, no-op if identical).
- SDD skills are written by the SDD component, not the skills component (to avoid duplicates).
- Tests use `t.TempDir()` for isolation — no real agent configs are modified.
