# cortex-ia Agent Guide

## Project Contract

- `cortex-ia` is the local bridge and control plane for the OpenCode ecosystem. It installs the native OpenCode asset set, manages MCP configuration, owns durable task authority in SQLite, and may supervise one external AGY execution leaf directly or through Herdr.
- OpenCode native agents remain the controllers. External CLIs are bounded executors, never coordinators: they receive no Cortex session lifecycle, `cortex-ia work` claim/lease tokens, approval authority, or nested-delegation capability.
- The selected specification plane owns SDD contracts: OpenSpec for `openspec|hybrid`, pinned Cortex observations for `cortex`. Cortex MCP also owns durable evidence, memories, AST knowledge, and relationships. Neither replaces SQLite task authority. The canonical phase matrix is `internal/assets/skills/_shared/workflow-map.md`.
- ForgeSpec and the external task-board MCP are retired. Do not restore their plugin, protocol, tools, or runtime dependency. Task boards are built into this binary through `cortex-ia board`.
- The product targets OpenCode only. Do not reintroduce platform adapters, personas, profiles, model routing, or SDD compiler/registry surfaces; retired commands and flags fail closed in `internal/app/app.go`.

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. The root `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner. `web/package.json` is the isolated Preact/Vite toolchain for CortexIA Web.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.
- Build frontend assets with `npm --prefix web run build` before the Go build whenever `web/src/` changes. Vite writes only compiled assets to `internal/cortexiaweb/static/`; commit both source and generated output so Go builds do not require Node.
- Important built-in surfaces are `install`, `sync`, `mcp`, `herdr`, `delegate`, `work`, `board`, `doctor`, `rollback`, `recover`, and `uninstall`. Keep lifecycle and ownership policy out of the dispatcher.

## Execution Modes

All role controllers execute in `native` mode under the Cortex-IA Work Authority. OpenCode subagents proceed directly with local execution using their available tools, acquired claims, and file leases.

## Task Boards and Work Authority

- `~/.cortex-ia/delegation.db` is the single local SQLite database for delegation jobs, boards, DAG tasks, claims, file leases, approvals, and append-only operational events. `CORTEX_IA_HOME` is allowed only as an explicit isolated state-root override for automation and smokes.
- Use `cortex-ia board create|list|status|serve` for durable board grouping and the embedded Kanban. Every coordinated initiative should have one stable board ID; `default` is for direct ungrouped work and migrated tasks.
- Use `cortex-ia work create --board <board-id>` for tasks. Every dependency must exist in the same board. Browser card position is observational and never proves readiness, review, or authority.
- `work claim`, revision-CAS transitions, TTL renewals, and workspace-relative `work lease` reservations are authoritative. Tokens remain only in live controller memory; never place them in prompts, receipts, Cortex observations, logs, or files.
- Only an independent `work approve --verdict PASS` with evidence produces `done`. External receipts, chat messages, tests alone, and Kanban placement never complete a task.
- Canonical role and lifecycle rules live in `internal/assets/skills/_shared/cortex-work-protocol.md`; installed agents must follow that file.

## Agent and Subagent Boundaries

| Role | Allowed work-control behavior |
|---|---|
| `orchestrator` | Create/query boards and DAGs, recover expired attempts, retry reconciled blockers, dispatch native role controllers. Never claim tasks, lease files, edit product code, or launch AGY directly. |
| `discovery` | Project onboarding: inspect skills, stack, engines, and architecture into `./.cortex-ia/discovery.md`. Never mutate work state. |
| `investigate` | Read-only `board list|status` and `work list|status`; diagnose and save bounded evidence. Never mutate work state. |
| `planner` | Write OpenSpec planning artifacts, create the initiative board, and materialize its same-board dependency DAG. Never claim implementation work. |
| `implement` | Own exactly one ready task claim, lease every writable path, renew authority, supervise at most one optional AGY leaf, verify, then transition to `in_review`. Stop writing immediately if authority expires. |
| `reviewer` | Independently inspect and rerun checks; its only work-state mutation is `work approve`. Never edit, claim, lease, or self-approve as the active implementation owner. |
| external AGY leaf | Execute only the validated envelope in the current workspace under exclusive lease and baseline validation, and return a bounded receipt. No control-plane, Cortex MCP, or delegation authority. |

- Only the orchestrator owns `cortex_session_start`, summaries, and session end. All dispatched roles are ephemeral subagents within that session.
- A native controller may supervise no more than one external leaf for its bounded objective. The leaf cannot spawn another agent or CLI.
- Parallel native writers may share one workspace without Git worktrees only when each controller owns a distinct live task claim and reserves each writable file individually with `cortex_ia_file_reserve` before editing that file. Acquire multiple files in deterministic sorted order, release each with `cortex_ia_file_release`, clean partial acquisition immediately on conflict, and stop writing on conflict or expiry. Mailbox/resource locks do not replace file reservations.
- External AGY implementation requires `current_workspace` as the single supported workspace strategy (`isolated_worktree` is retired). A current-workspace external AGY leaf remains exclusive for its execution window, forbids concurrent native edits, and must preserve pre-existing unleased changes against a pre-run baseline.
- Implementation minions require a non-empty `allowed_files` list corresponding to leased repository paths. Read-only tasks, forensic audits, and unleased inspections (`allowed_files: []`) must route to `investigate` or `reviewer`. Verification commands in tasks and task DAGs must be raw, executable commands without comments or parenthetical explanations.
- Reviewer Proportionality & Anti-Overengineering: Reviewers audit against actual repository diffs and declared acceptance criteria; NEVER fail or block tasks on synthetic test helpers or hypothetical inputs uncalled in the codebase. Declarative configs (Docker, Compose, YAML, `.dockerignore`) use standard parsers or CLI checks, never custom lexers. Pure-test tasks must not undergo DAG decomposition upon failure.
- Zero-Noise Comments & Clean Code Policy: Implementers MUST NOT write narrative echo comments explaining what obvious code does, inline changelogs, or commented-out dead code. Comments explain non-obvious *WHY* or critical invariants only. Chat communication must remain terse, minimal, and surgical without conversational filler or intermediate narration.
- Orchestrator Executive Synthesis & Zero-Chatter: Orchestrators communicate using the 3-Layer Artifact Pyramid (Header, progressive disclosure synthesis, decoupled deep dossier). Chat is reserved exclusively for interactive decision gates and high-density delivery syntheses. Never emit stream-of-consciousness play-by-play chatter, copy-paste raw subagent receipts, or dump internal SQLite DAG state into chat.

## Verification

- Full local gates, in hook order: `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, then `go test -count=1 ./...`.
- **Testing Scope & Anti-Overengineering Rule**: No crear tests innecesarios ni sobreingenierizados. Las pruebas persistentes se limitan exclusivamente a:
  1. Interfaz TUI (`internal/tui/...`).
  2. Validación simple de existencia y copia de archivos/carpetas hacia OpenCode (`internal/pipeline/install_test.go`).
  3. Pruebas de regresión críticas acotadas y autorizadas para verificación de autoridad (`internal/delegation/...`), transporte (`internal/assets/plugins/...`), recuperación (`internal/install/...`) y actualizador (`internal/updater/...`).
- Persistent tests protect the TUI, clean asset installation/copying into OpenCode, and user-approved critical boundaries (authority, transport, recovery, updater). Deeper SQLite, delegation, CLI, and embedded-web transactional oracles run as isolated ephemeral smokes and are deleted after execution.
- Tests must remain modular: dedicated new test files at most 250 lines, and never append new suites to existing test files exceeding 300 lines.
- Tests use temporary home directories and synthetic inputs. Never point tests at the developer's real agent configuration or user state, and never weaken contracts by silently skipping invalid records.
- Focus a package with `go test ./internal/tui/...`; focus a test with `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1`.

## Architecture

- `cmd/cortex-ia/main.go` only sets the release version and calls `internal/app`; CLI dispatch and the zero-argument TUI split live in `internal/app/app.go`. The dispatcher owns no install, merge, or ownership logic; it parses intent and renders receipts.
- `internal/install` is the service facade: install, sync, doctor, rollback, uninstall, and every MCP operation. All ownership decisions belong to the service and its collaborators.
- `internal/pipeline` plans and applies the asset copy transactionally: `InstallV2` plans first, captures a verified backup, applies, and restores from the backup if the apply phase fails.
- `internal/backup` owns snapshots, manifests, verification, dedup, and retention pruning.
- `internal/mcpmanager` owns the managed MCP catalog (`presets.go`), desired-entry validation, qualification, and conflict errors for the `mcp add/list/remove` surface.
- `internal/state` and `internal/installmeta` own installation metadata, lock, agreement checks, and MCP digests under `~/.cortex-ia/`.
- `internal/delegation` owns the SQLite schema/migrations, AGY job lifecycle, task boards, DAG state, claims, TTL file leases, approvals, recovery, and structured receipts. Use `BEGIN IMMEDIATE` for multi-step authority transitions and fail closed on unknown future schema versions.
- `internal/herdr` owns optional Herdr installation/setup and pane transport. Herdr never owns task state or approval.
- `internal/cortexiaweb` owns the Preact operations console compiled by Vite, embedded by `go:embed`, and served through its loopback-only HTTP/API server. It shows task-board sessions, work/claim/lease state, delegation jobs/transports, and the append-only activity stream. It may create boards/tasks but must not expose claim, lease, transition, retry, recovery, or approval mutations. Preserve CSP, request limits, server timeouts, auto-refresh, and the non-loopback rejection.
- `internal/agents/opencode` declares the OpenCode native layout (`layout.go`) and the pure asset mapping (`assetmap.go`): every destination is `path.Join(config root, source)`, validated against the single layout declaration, failing closed on unsafe paths, off-surface destinations, and collisions (including case-insensitive ones on Windows/macOS).
- `internal/components/filemerge` owns JSONC decode/merge and atomic writes; reuse it instead of writing ad-hoc merge code.
- Skills, prompts, commands, and plugins under `internal/assets/` are runtime source files embedded by `go:embed`; changing them requires rebuilding the binary. Shared skill contracts under `internal/assets/skills/_shared/` support installed agent instructions and must stay aligned with role files.

## OpenCode v2 (`opencode2`) Knowledge & Search Index

When researching, developing, or debugging capabilities for **OpenCode v2 (`opencode2`)**, use this canonical index to locate authoritative specifications, official documentation, and local inspection commands.

### 1. Official Documentation Mapping

| Topic & URL | Scope & Key Concepts | When to Consult |
| :--- | :--- | :--- |
| **[Core Docs](https://opencode.ai/v2/docs/)** | Core runtime architecture, Daemon/Server model, File hierarchy (`.config/opencode/` vs `.opencode/`), Precedence & merging rules, `opencode.jsonc` schema, Permissions array format (`[{ action, resource, effect }]`). | When designing configuration templates, setting permissions, or understanding directory precedence. |
| **[CLI & TUI](https://opencode.ai/v2/docs/cli/)** | Global CLI commands, TUI navigation (`opencode2`), `cli.json` configuration, Theme switching (`/themes`), Keybindings, Terminal Truecolor requirement (`COLORTERM=truecolor`). | When configuring user TUI preferences, themes, keybindings, or troubleshooting TUI rendering. |
| **[Build & Plugins](https://opencode.ai/v2/docs/build/)** | Plugin architecture (`@opencode/plugin`), Tool hooks (`ctx.tool.hook`), Transforms (`ctx.tool.transform`), Event subscriptions (`ctx.event`), Context extensions (`ctx.agent`, `ctx.provider`, `ctx.model`, `ctx.mcp`, `ctx.command`), Custom tools. | When authoring plugins, guards, telemetry interceptors, or runtime middleware. |
| **[API & Server](https://opencode.ai/v2/docs/api/)** | OpenAPI 3.1.0 specification, Background service daemon, HTTP `/api/*` endpoints, WebSocket event streaming, Session compaction, Snapshot management. | When interacting directly with the local OpenCode daemon via HTTP or building client bridges. |

### 2. Live CLI Inspection Helpers (`opencode2`)

Use the native binary (`opencode2`) directly to inspect live runtime state:

- `opencode2 debug paths`: Print active filesystem locations (`home`, `data`, `cache`, `config`, `state`, `log`, `db`).
- `opencode2 debug config`: Print all resolved configuration sources and the fully merged active configuration tree.
- `opencode2 models`: List all active AI models and provider connectivity.
- `opencode2 --print-logs`: Stream real-time diagnostic server logs to stderr.
- `opencode2 stats`: Output shareable usage statistics.

### 3. Project Skills & Developer Helpers

Project-level skills are located in `.agents/skills/` (ready for use in this repository without asset embedding):

- **`opencode-theme-dev`** (`.agents/skills/opencode-theme-dev/SKILL.md`):
  - Author and migrate v2 themes (`base`, `dark`, `light`, 9-step `hue` scales, `categorical`).
  - Validation helper: `node scripts/validate-theme.mjs <theme.json>` (checks all 16 required tokens).
  - Reference: `.agents/skills/opencode-theme-dev/references/theme-token-spec.md`.
- **`opencode-plugin-dev`** (`.agents/skills/opencode-plugin-dev/SKILL.md`):
  - Develop native plugins using `@opencode/plugin` and the universal dual-mode wrapper (`Plugin.define`).
  - Reference: `.agents/skills/opencode-plugin-dev/references/plugin-api-reference.md`.
  - Examples: `.agents/skills/opencode-plugin-dev/examples/tool-guard-plugin.ts`.
- **`opencode-installer-dev`** (`.agents/skills/opencode-installer-dev/SKILL.md`):
  - Best practices for configuration installers and environment orchestrators targeting `opencode2`.
  - Reference: `.agents/skills/opencode-installer-dev/references/opencode-v2-precedence.md`.

## Security and Persistence Invariants

- SQLite uses `STRICT` tables, WAL, foreign keys, `busy_timeout`, a migration ledger, bounded values, and hashed authority tokens. Never store plaintext claim/lease tokens, secrets, full prompts, or unbounded stdout.
- AGY execution uses argv without a shell, an explicitly selected workspace strategy (`current_workspace`), a temporary home, bounded output, timeouts, and structured receipts. `current_workspace` requires exclusive execution plus baseline/allowlist validation; `isolated_worktree` is retired. Do not enable unsafe permission bypasses by default.
- The embedded board server accepts only `localhost` or loopback IP addresses. Do not add CORS, remote binding, external assets, CDN dependencies, or browser endpoints that bypass work-control authority.
- Install/sync/uninstall ownership remains accreditation-based. Preserve verified backups, stale-plan detection, atomic writes, rollback on apply failure, and fail-closed behavior for unmanaged drift.
- Tests and smokes must use temporary homes. Never aim pipeline, delegation, board, or TUI verification at the developer's real OpenCode or Cortex state.

## Repository Workflow

- Before code changes, load the task-matched `SKILL.md`; SDD and utility skill sources are under `internal/assets/skills/`. Load `go-testing` when writing tests. See `docs/sdd-workflow.md` only when the phase map is needed.
- PR CI requires a branch matching `<type>/<lowercase-name>`, a body containing `Closes #N`, `Fixes #N`, or `Resolves #N`, every linked issue labeled `status:approved`, and exactly one `type:*` PR label.
- Commit first lines are enforced only to 10-72 characters by Husky, but repository convention is Conventional Commits; release changelog inclusion depends on `feat`, `fix`, `refactor`, and `perf` prefixes.

The canonical workflow/phase matrix is `internal/assets/skills/_shared/workflow-map.md` (installed as `~/.cortex-ia/opencode/contracts/workflow-map.md`). Use it before routing SDD; do not assume one agent or skill per phase. `orchestrator` routes, `discovery` profiles the project, `investigate` diagnoses, `planner` owns proposal/spec/design/tasks/archive, `implement` executes, and `reviewer` independently verifies using `code-review-adversary`. Other installed utility skills are discovered from `internal/assets/skills/*/SKILL.md` and loaded only for their actual task trigger.
