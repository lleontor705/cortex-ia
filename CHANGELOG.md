# Changelog

## v0.2.0 (2026-03-31)

### Infrastructure
- **2-stage pipeline** with rollback (Prepare→Apply, FailurePolicy, RunParallelChains)
- **Parallel agent execution** — agents run concurrently, components sequential per agent
- **Topological sort** — Kahn's algorithm with ParallelGroups for dependency ordering
- **Health check framework** — 6 checks (files, cortex, node/npx, skills, convention, state/lock)
- **System detection extended** — Node.js, npx, Git, Go, Cortex binary, shell
- **Self-update** — `cortex-ia update` checks GitHub releases

### New Components
- **Permissions** — security guardrails with deny lists per-agent
- **Persona** — professional/mentor/minimal communication styles
- **Theme** — cortex theme overlay for agent settings
- **Auto-install** — agents installable via npm/brew

### SDD Workflow
- **Multi team-lead pattern** — independent groups run parallel team-leads, dependent groups self-coordinate via P2P messaging
- **Adaptive pipeline** — escalation/de-escalation by confidence + task failures
- **Per-phase model assignments** — opus/sonnet/haiku with 3 presets (balanced/performance/economy)
- **68 MCP tools** fully documented across 4 MCPs (Cortex v0.2.1, ForgeSpec, Mailbox, CLI Orchestrator)
- **sdd_validate + sdd_save** added to all pipeline skills (draft-proposal, write-specs, decompose, debate)
- **execute-plan** migrated from TodoWrite to ForgeSpec task board (tb_*)
- **Convention** updated with revision history, timeline, hybrid search, project hygiene, temporal tools

### New Features
- **Project config** — `.cortex-ia.yaml` for per-repo preset, persona, model-preset, agents, custom-skills
- **Dynamic skill loading** — 3 layers: embedded → community → project
- **Shared skills directory** — `~/.cortex-ia/skills/` replacing per-agent duplication
- **Convention refs** resolved with absolute paths (no more broken relative refs)

### CLI (7→17 commands)
- `cortex-ia sync` — refresh managed files
- `cortex-ia config` — show configuration
- `cortex-ia list agents|components|backups`
- `cortex-ia init` — create .cortex-ia.yaml
- `cortex-ia skill add|list|remove` — manage community skills
- `cortex-ia auto-install [--dry-run]` — install missing agents
- `cortex-ia update` — check for updates
- `--model-preset`, `--persona`, `--local` flags

### TUI
- Detection screen (platform, tools, agents)
- Persona picker screen

### Testing
- 175 test functions across 23 packages (was 123/18)
- Pipeline coverage 98.9%, SDD coverage 93.8%
- E2E Docker tests: Ubuntu + Fedora (29 assertions)

## v0.1.0 (2026-03-29)

### Initial Release
- 8 agent adapters (Claude Code, OpenCode, Gemini CLI, Cursor, VS Code Copilot, Codex, Windsurf, Antigravity)
- 5 MCP server components (Cortex, ForgeSpec, Agent Mailbox, CLI Orchestrator, Context7)
- 19 SDD skills with orchestrator prompts
- Interactive TUI installer (Bubbletea)
- Idempotent injection with `<!-- cortex-ia:ID -->` markers
- Backup/restore with manifest
- Install, detect, doctor, repair, rollback commands
