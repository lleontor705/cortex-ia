<p align="center">
  <br>
  <img src="docs/assets/logo.svg" alt="cortex-ia" width="400" />
  <br><br>
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

# Cost-optimize with model routing
cortex-ia install --model-preset economy

# Set persona (professional/mentor/minimal)
cortex-ia install --persona mentor

# Use project-level config
cortex-ia init                        # Create .cortex-ia.yaml
cortex-ia install --local             # Apply project config

# Preview without changes
cortex-ia install --dry-run

# Show detected agents + runtime deps
cortex-ia detect
```

## What It Configures

cortex-ia injects **5 MCP servers** + **19 SDD skills** + **orchestrator prompts** into any supported agent:

| Component | MCP Tools | What It Does |
|-----------|:---------:|-------------|
| [**Cortex**](https://github.com/lleontor705/cortex) | 31 | Persistent memory with knowledge graph, FTS5, revision history, temporal tracking |
| [**ForgeSpec**](https://npmjs.com/package/forgespec-mcp) | 19 | SDD contract validation (Zod), task board with dependencies, file reservation |
| [**Agent Mailbox**](https://npmjs.com/package/agent-mailbox-mcp) | 14 | Inter-agent messaging, threads, broadcast, request/reply, deduplication, registry |
| [**CLI Orchestrator**](https://npmjs.com/package/cli-orchestrator-mcp) | 4 | Multi-CLI routing (Claude/Gemini/Codex) with circuit breaker, retry, fallback |
| [**Context7**](https://github.com/upstash/context7) | 2 | Live framework and library documentation via MCP |

Plus **3 content components**:

| Component | What It Does |
|-----------|-------------|
| **SDD Workflow** | 9-phase Spec-Driven Development with orchestrator + 19 specialized skills |
| **Conventions** | Shared cortex memory protocol + naming conventions for all agents |
| **Extra Skills** | Non-SDD utility skills (injected separately from SDD) |

**Total: 70 MCP tools across 4 MCPs + Context7**, all documented in skills and orchestrator prompts.

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

<p align="center">
  <img src="docs/assets/sdd-pipeline.svg" alt="SDD Pipeline" width="100%" />
</p>

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

## Multi-Agent Orchestration

<p align="center">
  <img src="docs/assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

## Task Routing

<p align="center">
  <img src="docs/assets/task-routing.svg" alt="Task Routing" width="100%" />
</p>

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

## Per-Phase Model Routing

Assign different Claude model tiers to SDD phases for cost/quality optimization:

```bash
cortex-ia install --model-preset economy    # Sonnet everywhere, Haiku for archive
cortex-ia install --model-preset balanced   # Opus for design+validate, Sonnet for apply
cortex-ia install --model-preset performance # Opus for critical phases, Sonnet for rest
```

| Preset | Orchestrator | Investigate | Architect | Implement | Validate | Finalize |
|--------|:-:|:-:|:-:|:-:|:-:|:-:|
| **balanced** | opus | sonnet | opus | sonnet | opus | haiku |
| **performance** | opus | sonnet | opus | sonnet | opus | haiku |
| **economy** | sonnet | sonnet | sonnet | sonnet | sonnet | haiku |

## Persona System

Choose the communication style for all configured agents:

| Persona | Style |
|---------|-------|
| `professional` | Direct, concise, technical terminology (default) |
| `mentor` | Teaching-oriented, explains trade-offs and patterns |
| `minimal` | Code only, no explanations unless asked |

```bash
cortex-ia install --persona mentor
cortex-ia sync --persona minimal    # Change persona without reinstalling
```

## Project Configuration

Create a `.cortex-ia.yaml` in your repo root to standardize settings across your team:

```bash
cortex-ia init    # Creates .cortex-ia.yaml with defaults
```

```yaml
# .cortex-ia.yaml
preset: full
persona: professional
model-preset: balanced
agents:
  - claude-code
  - opencode
custom-skills:
  - path: ./skills/domain-validator
```

```bash
cortex-ia install --local    # Applies project config
```

## How It Works

### Installation Flow

```
cortex-ia install
    │
    ├─ Stage 1: PREPARE (stops on error, rolls back)
    │   ├─ Validate agents exist in registry
    │   └─ Create backup snapshot (~/.cortex-ia/backups/)
    │
    ├─ Stage 2: APPLY (continues on error, agents in parallel)
    │   ├─ For each agent (concurrent via RunParallelChains):
    │   │   ├─ Inject MCP configs (strategy-specific: JSON / merge / TOML)
    │   │   ├─ Inject orchestrator prompt (markdown sections / file replace / append)
    │   │   ├─ Inject permissions & security guardrails
    │   │   ├─ Inject persona (professional / mentor / minimal)
    │   │   ├─ Inject theme overlay
    │   │   └─ Write sub-agent definitions (OpenCode, Cursor)
    │   ├─ Write SDD skills to shared dir (~/.cortex-ia/skills/)
    │   ├─ Write convention + orchestrator prompt to shared dir
    │   └─ Load community skills (~/.cortex-ia/skills-community/)
    │
    └─ Save state + lock (~/.cortex-ia/)
```

### Key Design Principles

- **Non-destructive**: Uses `<!-- cortex-ia:ID -->` markers. Content outside markers is never touched.
- **Backup-first**: Automatic snapshot before every install with restore capability.
- **Idempotent**: Running install twice produces identical results with zero file changes.
- **Adapter pattern**: Each agent implements an interface. Adding a new agent requires zero changes to components.
- **Strategy dispatch**: MCP injection is template-based — adding a new MCP server is one file.

## CLI Commands

```
cortex-ia                    Interactive TUI
cortex-ia install            Install ecosystem (auto-detect agents)
cortex-ia sync               Refresh managed files from current state
cortex-ia detect             Detect agents + runtime dependencies (Node, npx, Git, Go, Cortex)
cortex-ia config             Show current configuration
cortex-ia list agents        List detected agents with status
cortex-ia list components    List installed components
cortex-ia list backups       List available backups
cortex-ia init               Create .cortex-ia.yaml in current dir
cortex-ia skill add <path>   Add community skill from directory
cortex-ia skill list         List installed community skills
cortex-ia skill remove <id>  Remove community skill
cortex-ia auto-install       Install missing agents via package managers
cortex-ia doctor             Run 6 health checks against installation
cortex-ia repair             Re-apply from lockfile/state
cortex-ia rollback           Restore from backup
cortex-ia update             Check for available updates
```

## Documentation

| Doc | Description |
|-----|-------------|
| [Installation](docs/installation.md) | All installation methods, prerequisites, platform notes |
| [Agents](docs/agents.md) | Per-agent configuration details, paths, strategies |
| [Components](docs/components.md) | Component catalog, dependencies, what each injects |
| [SDD Workflow](docs/sdd-workflow.md) | 9-phase pipeline, commands, contract validation, prompting techniques |
| [Architecture](docs/architecture.md) | Codebase structure, patterns, testing, contributing |
| [Configuration](docs/configuration.md) | Presets, CLI flags, model routing, personas, project config |
| [Changelog](CHANGELOG.md) | Version history (v0.1.0 → v0.2.0) |
| [llms.txt](llms.txt) | LLM-readable project index |

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
