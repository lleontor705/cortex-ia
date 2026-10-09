# Changelog

## Unreleased

### Removed

- **Custom provider installer retired** — the custom provider installer is removed because provider installation is now native in nan/OpenCode. The retired surface spans the `internal/providermgr` catalog package, the `install.Service` provider transaction (twin reconciliation and provider identity digest), the TUI Providers screen with its Home entry and `p`/`P` hotkey, and the state v2 `Providers` family. Breaking change for users who relied on the TUI installer; the replacement is native nan/OpenCode provider support. Existing `state.json` provider rows are passively retired: the schema version stays pinned at 2, existing documents keep loading, the stale `providers` key drops on the next state commit, and previously written provider blocks in `opencode.json(c)` remain valid native OpenCode config.

## v0.5.8 (2026-10-07) — decoupled claim TTL and scoped self-recovery

### Added

- **Implement-scoped self-recovery** — the bridge exposes `cortex_ia_work_recover_own(task_id)` and the CLI gains `cortex-ia work recover --task <id> --owner <identity>`; both recover only the caller's own expired claim and file leases, with in-memory plus durable ownership verification and fail-closed denials (REQ-WAUTH-002, REQ-WAUTH-003)

### Changed

- **Claim TTL decoupled from the stale-progress window** — the bridge maintenance policy grants a 30-minute claim validity window per heartbeat renewal, while the 15-minute no-host-activity orphan window (`stale_progress_ms`) is unchanged and still stops heartbeats (REQ-WAUTH-001)
- **Protocol and agent assets** — the canonical work protocol authority table and the `implement` agent guard document the decoupled TTL and the implement-scoped recovery boundary (REQ-WAUTH-004)

## v0.5.7 (2026-10-06) — responsive TUI home logo

### Fixed

- **TUI home logo** — make the logo responsive on small terminals (`79289a2`, PR #139)

## v0.5.6 (2026-10-06) — heartbeat renewal and telemetry classification

### Changed

- **Vendored report-hub reconcile script** — add the coalescing reconcile script (`71bd808`, PR #134)

### Fixed

- **Claim heartbeat renewal without a host status RPC** — the plugin renews the claim from its own maintenance loop instead of requiring a host status call (`7e51860`)
- **Telemetry error-code classification** — map every Standard Taxonomy code in the snapshot map (`c33e664`), rank `ERR_DELEGATION_FAIL` high (`8c4767f`), give refused work authority its own code (`621d328`), classify invented persistence codes by root cause (`eb9e231`), rank invalid-argument errors above the MCP bucket (`3696a59`, PR #121), classify snapshot timeouts as `ERR_TOOL_TIMEOUT` (`1d40893`, PR #126), and recover failure detail behind boolean error flags (`9832cf5`, PR #117)
- **Repeated read-only subagent aborts** elevated as telemetry (`3f5628d`)
- **`work create`** accepts the `ops-task` workflow in the schema and CLI (`ea37862`, PR #120)
- **`proxy-addr`** bumped to 2.0.8 for CVE-2026-90711 (`dc7d85c`, PR #136)

## v0.5.5 (2026-10-03) — tool execution error telemetry

### Added

- **Tool execution and usage error telemetry** — report tool execution and usage errors through the telemetry hub (`3d25e0d`, PR #114)

## v0.5.4 (2026-10-02) — branch boards, bounded plugin leases

### Added

- **Automatic branch boards, task pruning, hide-completed toggle** — derive per-branch boards, prune expired tasks, and hide completed cards (`31d9af8`, PR #110)

### Changed

- **Dependencies** — GitHub Actions and `modernc.org/sqlite` bumped (`1a086f2`, PR #112)

### Fixed

- **Plugin lease admission bound with latency telemetry** — `d6bae87`
- **Host-loop child process admission bound** — `2193b78`
- **Delegation directory fingerprinting outside git work trees** — `e5759e7`

## v0.5.3 (2026-10-02) — updater download host allowlist

### Fixed

- **Updater download allowlist** — permit the GitHub release-assets host when downloading self-update artifacts (`7e8d67f`, PR #98)

## v0.4.56 (2026-09-26) — merge-safe provider configurator

Merge-safe custom-provider installation (PR #71): the configurator now merges
provider entries instead of overwriting neighbouring keys, and the installer is
driven by the provider catalog. Tagged `v0.4.56`.

### Fixed

- **Provider configurator merge safety** — `internal/install/provider.go` merges into the existing `provider` container so unrelated entries and user keys survive an install (`6051c67`, PR #71)
- **Model manager plural containers** — `internal/modelmgr/manager.go` handles provider `models` containers that already hold entries while staying catalog-driven (`internal/modelmgr/plural_container_test.go`)

### Changed

- **Provider catalog** expanded in `internal/providermgr/catalog.go` and `seed/nan.json`; the TUI Providers screen reflects the catalog updates
- **GitHub Actions** bumped via Dependabot: `checkout` 7.0.1, `upload-artifact` 7.0.1, `setup-python` 7.0.0, `github-script` 9.0.0, `deploy-pages` 5.0.1

## v0.4.55 (2026-09-26) — custom providers, updater verification profiles

Adds the custom-provider install flow and the updater trust profiles. Tagged
`v0.4.55`.

### Added

- **Custom provider install flow** (PR #68, PR #69) — `internal/providermgr/catalog.go` plus `seed/nan.json` define the provider catalog, `internal/install/provider.go` writes the entry, and `internal/state/provider_v2.go` records its ownership metadata (`fbe510f`)
- **TUI Providers screen** — `internal/tui/providers_screen.go` with masked secret input (`internal/tui/masked_input.go`) drives an interactive provider install ("Install custom provider")
- **Updater verification profiles** (`00e75e3`, PR #66) — `internal/updater/profile.go` and the Ed25519/checksum verifiers add `strict` and checksum profiles; `--allow-checksum-updates` and `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` grant per-run checksum consent only when no trust bundle is packaged, and `--scheduled` adds a headless check-only mode. See [`docs/reference/updater-verification-profiles.md`](docs/reference/updater-verification-profiles.md)

### Changed

- **Agent flows** bake mutation-evidence gates into the shipped agent and skill assets (`29b3826`)

## v0.4.54 (2026-09-25) — bilingual front door, updater resilience

Documentation, discovery, and updater hardening. Tagged `v0.4.54`.

### Added

- **Bilingual front door** — English/Spanish README pair, a community skill set, and the published Pages site (PR #54)
- **Updater resilience** — backup management and GitHub API improvements in the self-update path
- **TUI plugin configuration tests** and OpenCode binary handling refactor

### Changed

- **Discovery narrowed to a quick-index contract** (PR #53)

### Fixed

- **Updater replacement errcheck** — the deferred `f.Close` return in `internal/updater/replacement.go` is explicitly discarded, clearing the `golangci-lint` finding that blocked PRs #53 and #54 (`42099d6`, PR #56)

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
