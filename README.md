<p align="center">
  <br>
  <code>cortex-ia</code>
  <br>
  <strong>AI Agent Ecosystem Configurator</strong>
  <br>
  <em>One command. Any agent. Full SDD stack.</em>
  <br><br>
  <a href="https://github.com/lleontor705/cortex-ia/actions/workflows/ci.yml"><img src="https://github.com/lleontor705/cortex-ia/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/lleontor705/cortex-ia/releases/latest"><img src="https://img.shields.io/github/v/release/lleontor705/cortex-ia" alt="Release"></a>
  <a href="https://github.com/lleontor705/cortex-ia/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lleontor705/cortex-ia" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lleontor705/cortex-ia"><img src="https://goreportcard.com/badge/github.com/lleontor705/cortex-ia" alt="Go Report Card"></a>
</p>

---

cortex-ia detects your installed AI coding agents and configures them with a complete development ecosystem: persistent memory, SDD workflow, inter-agent messaging, multi-CLI orchestration, and live documentation — all via a single Go binary with an interactive TUI.

## Quick Start

```bash
# Install via Go
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest

# Install via Homebrew
brew install lleontor705/tap/cortex-ia

# Install via script (Linux/macOS)
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

```bash
# Interactive TUI installer
cortex-ia

# CLI: auto-detect agents, full preset
cortex-ia install

# CLI: specific agent + minimal preset
cortex-ia install --agent claude-code --preset minimal

# Preview without changes
cortex-ia install --dry-run

# Show detected agents
cortex-ia detect
```

## What It Configures

cortex-ia injects **5 MCP servers** + **19 SDD skills** + **orchestrator prompts** into any supported agent:

| Component | MCP Tools | What It Does |
|-----------|:---------:|-------------|
| [**Cortex**](https://github.com/lleontor705/cortex) | 19 | Persistent cross-session memory with knowledge graph, FTS5 search, importance scoring |
| [**ForgeSpec**](https://github.com/lleontor705/forgespec-mcp) | 15 | SDD contract validation (Zod), task board with dependencies, file reservation |
| [**Agent Mailbox**](https://github.com/lleontor705/agent-mailbox-mcp) | 9 | Inter-agent messaging, threads, broadcast, request/reply, deduplication |
| [**CLI Orchestrator**](https://github.com/lleontor705/cli-orchestrator-mcp) | 4 | Multi-CLI routing (Claude/Gemini/Codex) with circuit breaker, retry, fallback |
| [**Context7**](https://github.com/upstash/context7) | 2 | Live framework and library documentation via MCP |

Plus **3 content components**:

| Component | What It Does |
|-----------|-------------|
| **SDD Workflow** | 9-phase Spec-Driven Development with orchestrator + 19 specialized skills |
| **Conventions** | Shared cortex memory protocol + naming conventions for all agents |
| **Extra Skills** | Non-SDD utility skills (injected separately from SDD) |

**Total: 49 MCP tools, 91% referenced across skills** (4 admin-only tools correctly excluded).

## Supported Agents

| Agent | MCP Config | Prompt Strategy | Task Delegation | Sub-Agents | Slash Commands |
|-------|-----------|----------------|:-:|:-:|:-:|
| **Claude Code** | Separate JSON files | Markdown sections | ✅ | — | — |
| **OpenCode** | Merge into settings | File replace | ✅ | ✅ | ✅ |
| **Gemini CLI** | Merge into settings | File replace | — | — | — |
| **Cursor** | MCP config file | File replace (.mdc) | — | ✅ | — |
| **VS Code Copilot** | MCP config file | File replace | ✅ | — | — |
| **Codex** | TOML file | File replace | — | — | — |
| **Windsurf** | MCP config file | Append to file | — | — | — |
| **Antigravity** | MCP config file | Append to file | — | — | — |

Agents with **Task Delegation** get a multi-agent orchestrator that delegates work to sub-agents. Others get a single-agent prompt that executes SDD phases sequentially.

## Presets

| Preset | Components | Use Case |
|--------|-----------|----------|
| **full** | All 8 components | Complete ecosystem (default) |
| **minimal** | Cortex + ForgeSpec + Context7 + SDD | Essential SDD workflow (auto-pulls Mailbox via deps) |
| **custom** | User-selected via TUI | Pick exactly what you need |

## SDD Pipeline

Spec-Driven Development structures substantial changes through 9 phases:

```
init → explore → propose → spec → design → tasks → apply → verify → archive
```

```
proposal → spec ──┐
         ↘        ├→ tasks → apply → verify → archive
         design ──┘
```

### 19 Specialized Skills

| Phase | Skill | Role |
|-------|-------|------|
| init | `bootstrap` | Detect stack, bootstrap persistence, build skill registry |
| explore | `investigate` | Read codebase, compare approaches, rate effort/risk |
| propose | `draft-proposal` | Create change proposal with scope, risks, rollback plan |
| spec | `write-specs` | Write delta specs with Given/When/Then scenarios |
| design | `architect` | Technical design with architecture decisions |
| tasks | `decompose` | Break specs + design into dependency-ordered tasks |
| apply | `team-lead` | Coordinate parallel @implement agents |
| apply | `implement` | Write production code satisfying specs |
| verify | `validate` | Run tests, generate spec compliance matrix |
| archive | `finalize` | Merge specs, close change, generate retrospective |

**Utility Skills**: `debug`, `ideate`, `debate`, `monitor`, `execute-plan`, `open-pr`, `file-issue`, `parallel-dispatch`, `scan-registry`

### Modern Prompting Techniques

Skills incorporate research-backed techniques for better AI performance:

| Technique | Applied To | Impact |
|-----------|-----------|--------|
| **Chain-of-Verification** | validate | 30-50% fewer hallucinations in verification |
| **Constitutional Self-Critique** | implement | Code critiqued against specs before submission |
| **Skeleton-of-Thought** | draft-proposal, write-specs | Outline → validate → expand reduces omissions |
| **Extended Thinking** | architect, decompose | Explicit trade-off analysis, 2+ alternatives |
| **ReAct** (Thought/Action/Observation) | debug | Grounded debugging with evidence loops |
| **Step-Back Prompting** | architect | Abstract principles before specific design |
| **Inline WHY** | orchestrator, all rules | Motivation on every rule improves compliance |

## How It Works

### Installation Flow

```
cortex-ia install
    │
    ├─ Detect system (OS, arch, package manager)
    ├─ Detect installed agents (scan PATH + config dirs)
    ├─ Select preset (full / minimal / custom)
    ├─ Resolve dependencies (SDD → cortex + forgespec + mailbox)
    ├─ Create backup snapshot (~/.cortex-ia/backups/)
    ├─ Apply per agent:
    │   ├─ Inject MCP configs (strategy-specific: JSON / merge / TOML)
    │   ├─ Inject orchestrator prompt (markdown sections / file replace / append)
    │   ├─ Write SDD skill files to skills directory
    │   ├─ Write shared conventions to _shared/
    │   ├─ Write slash commands (OpenCode only)
    │   └─ Write sub-agent definitions (OpenCode, Cursor)
    └─ Save state (~/.cortex-ia/state.json)
```

### Key Design Principles

- **Non-destructive**: Uses `<!-- cortex-ia:ID -->` markers. Content outside markers is never touched.
- **Backup-first**: Automatic snapshot before every install with restore capability.
- **Idempotent**: Running install twice produces identical results with zero file changes.
- **Adapter pattern**: Each agent implements an interface. Adding a new agent requires zero changes to components.
- **Strategy dispatch**: MCP injection is template-based — adding a new MCP server is one file.

## Documentation

| Doc | Description |
|-----|-------------|
| [Installation](docs/installation.md) | All installation methods, prerequisites, platform notes |
| [Agents](docs/agents.md) | Per-agent configuration details, paths, strategies |
| [Components](docs/components.md) | Component catalog, dependencies, what each injects |
| [SDD Workflow](docs/sdd-workflow.md) | 9-phase pipeline, commands, contract validation, prompting techniques |
| [Architecture](docs/architecture.md) | Codebase structure, patterns, testing, contributing |
| [Configuration](docs/configuration.md) | Presets, CLI flags, state management, backup/restore |

## Prerequisites

- **Go 1.22+** — for building cortex-ia
- **Node.js 18+** with `npx` — for npm-based MCP servers (forgespec, mailbox, orchestrator, context7)
- **Cortex binary** — `go install github.com/lleontor705/cortex/cmd/cortex@latest` or `brew install lleontor705/tap/cortex`
- At least one [supported agent](#supported-agents) installed

## Related Projects

| Project | Description |
|---------|-------------|
| [cortex](https://github.com/lleontor705/cortex) | Persistent memory MCP server (Go binary) |
| [forgespec-mcp](https://github.com/lleontor705/forgespec-mcp) | SDD contracts + task board + file reservation |
| [agent-mailbox-mcp](https://github.com/lleontor705/agent-mailbox-mcp) | Inter-agent messaging system |
| [cli-orchestrator-mcp](https://github.com/lleontor705/cli-orchestrator-mcp) | Multi-CLI routing with circuit breaker |

## License

[MIT](LICENSE)
