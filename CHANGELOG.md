# Changelog

## v0.4.53 (2026-09-24) — orchestrator reconcile, nan effort catalog, TUI cockpit

Consolidates the work-authority reconcile path, the nan model effort/usage
surface, and the redesigned Cortex-IA TUI cockpit. No push, tag, or publish was
performed for this entry.

### Added

- **`cortex-ia work reconcile`** — orchestrator-only force-release of a live-but-orphaned claim: fail-closed release conditions (`claim_not_live`, `current_session_owner`, `claim_fresh_no_inactivity_evidence`), compare-and-set on the expected revision, atomic lease release, and an immutable `reconciled` audit event carrying the decision inputs
- **`cortex_ia_work_reconcile` bridge tool** — derives host session identity and owner-inactivity evidence only from the bridge's own execution context and host tracking, so an owner counts as inactive only when this bridge observed it active and it has since stopped
- **nan catalog metadata** — `internal/modelmgr/nan_catalog.go` publishes per-model monthly quota tokens, effort vocabulary, and effort posture (`adjustable`, `adaptive`, `accepted-not-adjustable`); a zero quota means unknown and renders as no percentage instead of dividing by zero
- **`model set <agent> <provider/model[#variant]> --effort <level>`** — a nan `set` auto-authors the matching `{id, settings.reasoningEffort}` variant entry when the provider model has none, previewing the appended variant in `--dry-run` and reporting it through `AuthoredVariants`
- **`model doctor` nan-variant check** — flags nan references whose requested effort has no corresponding provider variant
- **TUI nan usage strip** and `:model` picker — model selection with effort choice plus per-model month-to-date and 24h usage against quota
- **TUI `:cortex-agents` panel** — live agent and work-authority view inside the cockpit
- **Read-only cockpit snapshot** — the `ui` dashboard command opens the state database read-only, so the poll loop never contends with work-authority writers, and a fresh, unmigrated install renders an empty snapshot instead of failing

### Changed

- **TUI cockpit redesign** onto the OpenCode v2 theme schema (one complete `base` token tree plus hue palettes); the cortex theme was regenerated and the theme validator extended
- **`opencode-theme-dev` skill** updated to the v2 theme specification and reference token spec
- **Work-control plugin durable deference** — a retained in-memory claim handle no longer blocks a fresh claim when durable state shows the claim was recovered or reassigned; the stale handle is dropped and its maintenance stopped
- **Agent guidance and discovery skill** refreshed (`investigate` permission ordering, `reviewer` read-only receipt allowlist with mutating carve-outs, canonical discovery heading invariant)
- **OpenSpec** — archived the `nan-model-effort-usage` and `orchestrator-reconcile` changes and synced their canonical specs under `openspec/specs/`
- `.gitignore` now ignores `.opencode/themes/`

### Known Issues

- **Task `allowed_files` entries are matched literally** — glob patterns are not expanded, so declare explicit file paths; the latent match behavior is most visible on Windows
- **Error telemetry requires `CORTEX_REPORT_SECRET`** — reports are suppressed, not queued, when the secret is absent

## v0.3.0 (2026-04-25) — gentle-ai parity sweep

This release ports the high-value functionality and governance assets from the
upstream `gentle-ai` project while keeping cortex-ia's identity (granular
components, 2-stage pipeline, persona system, doctor, `cortex` memory).

### New agents (8 → 12)

- **kilocode** — adapter for Kilo (`~/.config/kilo/`)
- **kimi** — Kimi CLI with shared skills root (`~/.config/agents/skills`)
- **kiro-ide** — Kiro IDE (split-root layout, native sub-agents)
- **qwen-code** — Qwen Code (`~/.qwen/`)

### New top-level CLI commands

- **`cortex-ia uninstall`** — reverse cortex-ia injections per agent or component, with snapshot rollback (`--agent`, `--component`, `--all`, `--dry-run`, `--no-backup`)
- **`cortex-ia gga --provider <id>`** — switch GGA provider explicitly (anthropic, openai, google, ollama in addition to the agent-routed providers); `--list`, `--show` subcommands
- **`cortex-ia profiles list|create|set|delete`** — manage saved OpenCode SDD profiles (per-phase model assignments)
- **`cortex-ia agent-builder list|create|remove`** — generate custom skills via an installed AI engine (Claude Code, OpenCode, Gemini CLI, Codex), parse the output, install across selected adapters with rollback, and persist a registry under `~/.cortex-ia/agentbuilder/registry.json`

### New components

- **`uninstall`** — first-class component with marker-aware cleaners (rewrite, remove, remove-tree, remove-if-empty, remove-json-key) for every cortex-ia injection
- **`agentbuilder`** — engine + parser + prompt + registry + multi-installer for AI-generated skills

### Infrastructure

- **`agents.DiscoverInstalled` + `ConfigRootsForBackup`** — pure FS-based detection used by detection / backup pipeline / agent-builder target picker
- **Backup compression + retention** — tar.gz archives, SHA-256 dedup (`IsDuplicate`), `Prune` (default keep 5 unpinned), `Manifest.Pinned` / `Checksum` / `BackupSourceUninstall`, `BackupRootFn` swap point
- **Golden file testing** — `internal/components/golden_test.go` + 20 fixtures in `testdata/golden/` covering cortex / forgespec / mailbox / context7 / persona / conventions across claude / opencode / windsurf / antigravity. Regenerate with `go test -update ./internal/components/...`
- **`judgment-day` skill** — adversarial dual-judge review protocol added to the skills bundle

### Governance & docs

- **`CONTRIBUTING.md`** — issue-first workflow with cortex-ia label system
- **`AGENTS.md`** (root) — index of community + built-in skills
- **`CONTRIBUTORS.md`** with explicit gentle-ai lineage acknowledgement
- **`PRD.md`** + **`PRD-AGENT-BUILDER.md`** — vision and design docs
- **`docs/`** expanded with `quickstart`, `platforms`, `rollback`, `cortex-memory`, `non-interactive`, `docker-e2e-testing`
- **`openspec/`** scaffolding (`config.yaml` + `changes/` + `specs/cortex-ia/`)
- **`skills/`** community skills (`issue-creation`, `branch-pr`)
- **`.github/ISSUE_TEMPLATE/`** with `bug_report`, `feature_request`, `config`
- **`pr-check.yml`** gains a `check-branch-name` job and emoji-aware logging

### E2E

- **`Dockerfile.arch`** — third Linux distro target (forces `--platform=linux/amd64`, disables pacman seccomp under QEMU)
- **`e2e/lib.sh`** — shared shell helpers (`assert_*`, `log_*`, `resolve_binary`, `cleanup_test_env` covering all 12 agents)

### Model

Additive constants (no breaking changes):

- `model.AgentKilocode`, `AgentKimi`, `AgentKiroIDE`, `AgentQwenCode`
- `model.SkillJudgmentDay`
- `model.ComponentPersona`, `ComponentPermissions`, `ComponentTheme`
- `backup.BackupSourceUninstall`
- `backup.Manifest.Pinned`, `.Checksum` (both `omitempty`)

### Testing

44 packages, all green. Backwards-compatible manifest format — older backups load without modification.

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
