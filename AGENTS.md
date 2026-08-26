# cortex-ia Agent Guide

## Project Contract

- `cortex-ia` is the local bridge and control plane for the OpenCode ecosystem. It installs the native OpenCode asset set, manages MCP configuration, owns durable task authority in SQLite, and may supervise one external AGY execution leaf directly or through Herdr.
- OpenCode native agents remain the controllers. External CLIs are bounded executors, never coordinators: they receive no Cortex session lifecycle, `cortex-ia work` claim/lease tokens, approval authority, or nested-delegation capability.
- OpenSpec owns human-reviewable SDD contracts. Cortex MCP owns durable evidence, memories, AST knowledge, and relationships. Neither replaces SQLite task authority.
- ForgeSpec and the external task-board MCP are retired. Do not restore their plugin, protocol, tools, or runtime dependency. Task boards are built into this binary through `cortex-ia board`.
- The product targets OpenCode only. Do not reintroduce platform adapters, personas, profiles, model routing, or SDD compiler/registry surfaces; retired commands and flags fail closed in `internal/app/app.go`.

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. The root `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner. `web/package.json` is the isolated Preact/Vite toolchain for CortexIA Web.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.
- Build frontend assets with `npm --prefix web run build` before the Go build whenever `web/src/` changes. Vite writes only compiled assets to `internal/cortexiaweb/static/`; commit both source and generated output so Go builds do not require Node.
- Important built-in surfaces are `install`, `sync`, `mcp`, `herdr`, `delegate`, `work`, `board`, `doctor`, `rollback`, `recover`, and `uninstall`. Keep lifecycle and ownership policy out of the dispatcher.

## Execution Modes

The delegation bridge returns the effective mode. That return value is authoritative for every controller and subagent:

| Mode | Required behavior |
|---|---|
| `native` | No external job was accepted; the native OpenCode role controller executes the objective. |
| `direct_cli` | Cortex-IA accepted and launched AGY directly; the native controller supervises and independently verifies it. |
| `herdr_multiplexed` | Cortex-IA accepted and launched AGY through Herdr; Herdr changes presentation/transport, not authority. |

- Never infer the effective mode from TUI preferences, `use_herdr`, installed binaries, or pane visibility.
- After `delegated=true` plus `job_id`, never execute the same objective natively in parallel or fall back automatically after failure, timeout, cancellation, pane loss, or `lost`. Reconcile the durable job and retry explicitly under fresh authority.
- Installer semantics are fixed: delegation disabled means `native`; delegation enabled plus Herdr disabled may produce `direct_cli`; delegation enabled plus Herdr enabled may produce `herdr_multiplexed` or a safe pre-acceptance `direct_cli` fallback.

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
| `investigate` | Read-only `board list|status` and `work list|status`; diagnose and save bounded evidence. Never mutate work state. |
| `planner` | Write OpenSpec planning artifacts, create the initiative board, and materialize its same-board dependency DAG. Never claim implementation work. |
| `implement` | Own exactly one ready task claim, lease every writable path, renew authority, supervise at most one optional AGY leaf, verify, then transition to `in_review`. Stop writing immediately if authority expires. |
| `reviewer` | Independently inspect and rerun checks; its only work-state mutation is `work approve`. Never edit, claim, lease, or self-approve as the active implementation owner. |
| external AGY leaf | Execute only the validated envelope in the isolated worktree and return a bounded receipt. No control-plane, Cortex MCP, or delegation authority. |

- Only the orchestrator owns `cortex_session_start`, summaries, and session end. All dispatched roles are ephemeral subagents within that session.
- A native controller may supervise no more than one external leaf for its bounded objective. The leaf cannot spawn another agent or CLI.
- Parallel writers require distinct live task claims and a `work lease` for every path before editing. Mailbox/resource locks do not replace file leases.

## Verification

- Full local gates, in hook order: `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, then `go test -count=1 ./...`.
- **Testing Scope & Anti-Overengineering Rule**: No crear tests innecesarios ni sobreingenierizados. Las pruebas persistentes se limitan exclusivamente a:
  1. Interfaz TUI (`internal/tui/...`).
  2. Validación simple de existencia y copia de archivos/carpetas hacia OpenCode (`internal/pipeline/install_test.go`).
- Persistent tests protect the TUI and clean asset installation/copying into OpenCode. Deeper SQLite, delegation, CLI, and embedded-web transactional oracles run as isolated ephemeral smokes and are deleted after execution.
- Focus a package with `go test ./internal/tui/...`; focus a test with `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1`.
- Tests use temporary home directories. Never point pipeline or TUI tests at the developer's real agent configuration.

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

## Security and Persistence Invariants

- SQLite uses `STRICT` tables, WAL, foreign keys, `busy_timeout`, a migration ledger, bounded values, and hashed authority tokens. Never store plaintext claim/lease tokens, secrets, full prompts, or unbounded stdout.
- AGY execution uses argv without a shell, an isolated worktree and temporary home, bounded output, timeouts, and structured receipts. Do not enable unsafe permission bypasses by default.
- The embedded board server accepts only `localhost` or loopback IP addresses. Do not add CORS, remote binding, external assets, CDN dependencies, or browser endpoints that bypass work-control authority.
- Install/sync/uninstall ownership remains accreditation-based. Preserve verified backups, stale-plan detection, atomic writes, rollback on apply failure, and fail-closed behavior for unmanaged drift.
- Tests and smokes must use temporary homes. Never aim pipeline, delegation, board, or TUI verification at the developer's real OpenCode or Cortex state.

## Repository Workflow

- Before code changes, load the task-matched `SKILL.md`; SDD and utility skill sources are under `internal/assets/skills/`. Load `go-testing` when writing tests. See `docs/sdd-workflow.md` only when the phase map is needed.
- PR CI requires a branch matching `<type>/<lowercase-name>`, a body containing `Closes #N`, `Fixes #N`, or `Resolves #N`, every linked issue labeled `status:approved`, and exactly one `type:*` PR label.
- Commit first lines are enforced only to 10-72 characters by Husky, but repository convention is Conventional Commits; release changelog inclusion depends on `feat`, `fix`, `refactor`, and `perf` prefixes.

Installed skill assets and their triggers:

| Skill | Phase | When to load |
|-------|-------|--------------|
| `bootstrap` | SDD-0 | Starting a new SDD session |
| `investigate` | SDD-1 | Mapping unknown areas of the codebase |
| `draft-proposal` | SDD-2 | Drafting a change proposal |
| `write-specs` | SDD-3 | Producing Given/When/Then scenarios |
| `architect` | SDD-4 | Designing the implementation approach |
| `decompose` | SDD-5 | Breaking the design into tasks |
| `implement` | SDD-6 | Applying spec → code |
| `validate` | SDD-7 | Verifying scenarios pass |
| `finalize` | SDD-8 | Archiving the change set |
| `orchestrator`, `planner`, `reviewer` | cross-phase | Routing, planning, and review roles |
| `debate`, `code-review-adversary` | cross-phase | Multi-position and adversarial deliberation |
| `fast-tdd`, `property-based-testing`, `mutation-testing`, `ast-impact-analysis` | utility | Verification acceleration |
| `context-distiller`, `parallel-dispatch`, `spike-prototype`, `hotfix-triage` | utility | See each SKILL.md for the trigger |
