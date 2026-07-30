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

cortex-ia detects installed AI coding agents and compiles compatible workflow assets for them: persistent memory, SDD contracts, service bindings, prompts, skills, and diagnostics — all via a single Go binary with an interactive TUI. The external agent runtime executes those assets; cortex-ia is not a workflow scheduler.

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

# Reverse a previous install (snapshot first; use --all to wipe everything)
cortex-ia uninstall --component persona --component cortex
cortex-ia uninstall --all --dry-run

# Switch GGA provider (anthropic, openai, google, ollama, claude, opencode, gemini, codex)
cortex-ia gga --provider ollama

# OpenCode SDD profiles (per-phase semantic route assignment)
cortex-ia profiles create cheap:openai/gpt-4o-mini
cortex-ia profiles set cheap:sdd-design:route/v1/architecture
cortex-ia profiles list

# Build a custom skill via an installed AI engine
cortex-ia agent-builder create \
  --engine claude \
  --purpose "review go diffs against project conventions" \
  --target claude-code --target opencode
cortex-ia agent-builder list
```

## What It Configures

cortex-ia configures **3 current MCP services** plus direct SDD skills and orchestrator prompts:

| Component | MCP Tools | What It Does |
|-----------|:---------:|-------------|
| [**Cortex**](https://github.com/lleontor705/cortex) | 31 | Persistent memory with knowledge graph, FTS5, revision history, temporal tracking |
| [**ForgeSpec**](https://github.com/lleontor705/forgespec-mcp) | 15 | SDD contract validation (Zod), task board with inline creation, file reservation |
| [**Context7**](https://github.com/upstash/context7) | 2 | Live framework and library documentation via MCP |

Plus **3 content components**:

| Component | What It Does |
|-----------|-------------|
| **SDD Workflow** | 9-phase Spec-Driven Development with orchestrator + 19 specialized skills |
| **Conventions** | Shared cortex memory protocol + naming conventions for all agents |
| **Extra Skills** | Non-SDD utility skills (injected separately from SDD) |

Tool counts are derived from installed service schemas; documentation does not hard-code a combined count.

## Supported Agents and Tested Profiles

Every target has a `portable-sequential` conformance golden. Stronger profiles are emitted only when the compiler has fresh qualifying evidence. A golden proves deterministic lowering and manifest equivalence; it is **not** a universal claim about every runtime version, model, or environment.

| Agent target | MCP config | Tested profile fixtures |
|--------------|------------|-------------------------|
| **Claude Code** | Separate JSON files | sequential, flat, native-qualified* |
| **OpenCode** | Merge into settings | sequential, flat, native-qualified* |
| **Gemini CLI** | Merge into settings | sequential, flat, native-qualified* |
| **Cursor** | MCP config file | sequential, native-qualified* |
| **VS Code Copilot** | MCP config file | sequential; direct-child remains advisory |
| **Codex** | TOML file | sequential, flat, native-qualified* |
| **Windsurf** | MCP config file | sequential |
| **Antigravity** | MCP config file | sequential, native-qualified* |
| **Kilocode** | Merge into settings | sequential, flat |
| **Kimi** | MCP config file | sequential, flat |
| **Kiro IDE** | MCP config file | sequential, flat |
| **Qwen Code** | Merge into settings | sequential, flat |

`sequential` means no delegation is required. `flat` requires qualified direct-child delegation and never assumes nesting. `native-qualified*` means the repository has a `native-advanced` fixture; selection still requires fresh target-specific qualification. Any experimental native capability additionally requires **explicit operator opt-in** and is never selected implicitly. See [Agents](docs/agents.md) for the manifest-backed matrix.

## Presets

| Preset | Components | Use Case |
|--------|-----------|----------|
| **full** | All current components | Complete ecosystem (default) |
| **minimal** | Cortex + ForgeSpec + Context7 + SDD | Essential direct SDD workflow |
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
| apply | `implement` | Own one bounded work unit and write code satisfying specs |
| verify | `validate` | Run tests, generate spec compliance matrix |
| archive | `finalize` | Merge specs, close change, generate retrospective |

**Utility Skills**: `debug`, `ideate`, `debate`, `monitor`, `execute-plan`, `open-pr`, `file-issue`, `parallel-dispatch`, `scan-registry`

## Task Routing

<p align="center">
  <img src="docs/assets/task-routing.svg" alt="Task Routing" width="100%" />
</p>

## Apply Phase Workflow

ForgeSpec is authoritative for task readiness, claims, status, contracts, and negotiated file reservations. The orchestrator routes a ready task directly to `implement`; no current profile includes the retired historical `team-lead` role. Runtime-native dispatch is transport only.

## Direct Coordination

<p align="center">
  <img src="docs/assets/agent-coordination.svg" alt="Agent Coordination" width="100%" />
</p>

The orchestrator reads ForgeSpec readiness and dispatches one bounded reference through the runtime-native child-agent primitive. Cortex stores durable evidence. File reservations are used only when ForgeSpec advertises the qualified capability; otherwise execution is sequential with no concurrent writes. Provider-neutral remote A2A remains **unsupported and unbound**.

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

## Provider-Neutral Route Resolution

Phase configuration selects versioned semantic routes and typed capability requirements. Concrete provider/model references are resolved only from explicit user/provider configuration or fresh qualified discovery evidence.

```bash
cortex-ia profiles set default:sdd-design:route/v1/architecture
cortex-ia profiles set default:sdd-apply:route/v1/implementation
cortex-ia install --profile default
```

The resolver records provenance, freshness, capability evidence, and fallback/degradation reason. Missing or ineligible configuration fails closed before generation; there is no phase-to-provider assignment table or implicit model preset.

## OpenCode SDD Profiles

For OpenCode users who want per-phase routing, profiles save named bundles of semantic route assignments and apply configuration-backed resolutions to `opencode.json` automatically.

```bash
# Create a profile with a semantic route
cortex-ia profiles create default:route/v1/implementation

# Override specific phases with semantic routes
cortex-ia profiles set default:sdd-design:route/v1/architecture
cortex-ia profiles set default:sdd-apply:route/v1/implementation

# Use the profile during install — auto-applied to opencode.json
cortex-ia install --profile default

# Or apply to an existing install without re-injecting everything
cortex-ia profiles apply default

cortex-ia profiles list
cortex-ia profiles delete default
```

Values are versioned semantic route IDs. Provider/model mappings may be supplied separately through explicit provider configuration and are never invented by profile selection. Profiles persist in `~/.cortex-ia/profiles.json` and the active one is recorded in `state.json` so `cortex-ia sync` keeps using it.

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
profile: cheap          # optional: name of a saved OpenCode SDD profile
strict-tdd: false       # optional: enforce TDD across SDD apply/verify
agents:
  - claude-code
  - opencode
custom-skills:
  - path: ./skills/domain-validator
```

```bash
cortex-ia install --local    # Applies project config
```

Full schema reference: [`docs/cortex-ia.yaml.example`](docs/cortex-ia.yaml.example) — every field documented with valid values and behavior notes. CLI flags always override yaml; yaml overrides CLI defaults.

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

### Compiler and Installer Boundary

The capability-aware compiler resolves each requested semantic capability as `native`, `emulated`, `advisory`, or `unsupported`, then emits target assets plus semantic, security, and degradation manifests. Enforcement is separately classified as `runtime`, `hook`, `mcp`, `prompt`, or `none`; prompt text is advisory and is never described as enforced.

Installation consumes the immutable compiled bundle. Dry-run and apply use the same plan; doctor must qualify the selected profile before mutation. Managed assets are backed up, ownership-tracked, and three-way merged. Rollback restores a selected prior asset/configuration bundle and reports customization conflicts. This process migrates generated assets and configuration only—there is **no runtime-session or in-flight task-state migration**.

Service authority remains external:

- **ForgeSpec** owns SDD contracts and task dependency/readiness/claim/status. Transactional task capability is an explicit upstream ForgeSpec version dependency.
- **Cortex** owns durable memory, evidence, provenance, and relationships.
- The retired historical Agent Mailbox provider has no current ownership role. Its database, WAL/SHM, caches, archives, and repository checkout are never automatically mutated or deleted; cleanup is operator-controlled after preservation checks.
- Provider-neutral remote A2A is unsupported and unbound.
- **cortex-ia** compiles, configures, validates, installs, backs up, and restores assets; it does not duplicate those mutable authorities.

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
cortex-ia uninstall          Reverse cortex-ia injections (with pre-uninstall snapshot)
cortex-ia gga --provider <id>      Switch GGA provider (anthropic, openai, google, ollama, claude, opencode, gemini, codex)
cortex-ia profiles list|create|set|apply|delete   Manage OpenCode SDD profiles
cortex-ia agent-builder list|create|remove        Generate custom skills via an installed AI engine
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
| [Quickstart](docs/quickstart.md) | Three-command setup |
| [Platforms](docs/platforms.md) | OS support matrix, Windows symlink note |
| [Cortex memory](docs/cortex-memory.md) | The cortex MCP — 31 tools across 4 groups |
| [Rollback](docs/rollback.md) | Backups, retention, dedup, pinning, uninstall snapshots |
| [Non-interactive](docs/non-interactive.md) | CLI-only recipes for CI |
| [Docker E2E](docs/docker-e2e-testing.md) | Three-distro test harness (ubuntu/fedora/arch) |
| [Changelog](CHANGELOG.md) | Version history (v0.1.0 → v0.3.0) |
| [llms.txt](llms.txt) | LLM-readable project index |

## Prerequisites

- **Go 1.22+** — for building cortex-ia
- **Node.js 18+** with `npx` — for npm-based MCP servers (ForgeSpec and Context7)
- **Cortex binary** — `go install github.com/lleontor705/cortex/cmd/cortex@latest` or `brew install lleontor705/tap/cortex`
- At least one [supported agent](#supported-agents-and-tested-profiles) installed

## Related Projects

| Project | Description |
|---------|-------------|
| [cortex](https://github.com/lleontor705/cortex) | Persistent memory MCP server (Go binary) |
| [forgespec-mcp](https://github.com/lleontor705/forgespec-mcp) | SDD contracts + task board + file reservation |
| Historical Agent Mailbox provider | Retired from built-ins; external data cleanup remains operator-controlled |


## License

[MIT](LICENSE)
