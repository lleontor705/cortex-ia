# cortex-ia Agent Guide

## Project Contract

- `cortex-ia` is the local bridge and control plane for the OpenCode ecosystem. It installs the native OpenCode asset set, manages MCP configuration, and owns durable task authority in SQLite. Execution is native-only: there is no external execution leaf, and nothing is supervised through Herdr.
- OpenCode native agents are the only controllers and executors. The external AGY execution path is removed: no external CLI receives a Cortex session lifecycle, `cortex-ia work` claim/lease tokens, approval authority, or nested-delegation capability.
- The selected specification plane owns SDD contracts: OpenSpec for `openspec|hybrid`, pinned Cortex observations for `cortex`. Cortex MCP also owns durable evidence, memories, AST knowledge, and relationships. Neither replaces SQLite task authority. The canonical phase matrix is `internal/assets/skills/_shared/workflow-map.md`.
- ForgeSpec and the external task-board MCP are retired. Do not restore their plugin, protocol, tools, or runtime dependency. Task boards are built into this binary through `cortex-ia board`.
- The product targets OpenCode only. Do not reintroduce platform adapters, personas, profiles, model routing, or SDD compiler/registry surfaces; retired commands and flags fail closed in `internal/app/app.go`.

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. The root `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner. `web/package.json` is the isolated Preact/Vite toolchain for CortexIA Web.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.
- Build frontend assets with `npm --prefix web run build` before the Go build whenever `web/src/` changes. Vite writes only compiled assets to `internal/cortexiaweb/static/`; commit both source and generated output so Go builds do not require Node.
- Important built-in surfaces are `install`, `sync`, `mcp`, `model`, `work`, `board`, `openspec`, `web`, `doctor`, `rollback`, `recover`, and `uninstall`. `cortex-ia model list|get|set|unset|doctor` is global-config-only in v1; `model doctor` surfaces markdown-frontmatter model pins as WARNING findings and never silently rewrites them. `model catalog` is a read-only receipt of provider/model/variant identifiers only. Keep lifecycle and ownership policy out of the dispatcher.

## Execution Modes

All role controllers execute in `native` mode under the Cortex-IA Work Authority. OpenCode subagents proceed directly with local execution using their available tools, acquired claims, and file leases.

## Task Boards and Work Authority

- `~/.cortex-ia/delegation.db` is the single local SQLite database for delegation jobs, boards, DAG tasks, claims, file leases, approvals, and append-only operational events. `CORTEX_IA_HOME` is allowed only as an explicit isolated state-root override for automation and smokes.
- Use `cortex-ia board create|list|status|serve` for durable board grouping and the embedded Kanban. Every coordinated initiative should have one stable board ID; `default` is for direct ungrouped work and migrated tasks.
- Use `cortex-ia work create` for tasks (defaults to `--board default`, or pass `--board <board-id>` for coordinated initiatives). Every dependency must exist in the same board. Browser card position is observational and never proves readiness, review, or authority.
- `work claim`, revision-CAS transitions, TTL renewals, and workspace-relative `work lease` reservations are authoritative. Tokens remain only in live controller memory; never place them in prompts, receipts, Cortex observations, logs, or files.
- Only an approved `work approve --verdict PASS` (from an independent reviewer, or orchestrator auto-approval on low-risk/data/docs/config tasks) with evidence produces `done`. External receipts, chat messages, tests alone, and Kanban placement never complete a task.
- Canonical role and lifecycle rules live in `internal/assets/skills/_shared/cortex-work-protocol.md`; installed agents must follow that file.

## Agent and Subagent Boundaries

| Role | Allowed work-control behavior |
|---|---|
| `orchestrator` | Create/query boards and DAGs, recover expired attempts and retry reconciled blockers (unscoped recovery and retry remain orchestrator-only), dispatch native role controllers, auto-approve low-risk/data/docs/config tasks under the Adaptive Review Case Matrix (§2.7a digest below). Never claim tasks, lease files, or edit product code. |
| `discovery` | Agentic environment discovery: build the skills dictionary (project-local + installed global), run/test execution info, minimal governance list, and quick index into `./.cortex-ia/discovery.md`. Never mutate work state. |
| `investigate` | Read-only `board list|status` and `work list|status`; diagnose and save bounded evidence. Never mutate work state. |
| `planner` | Write OpenSpec planning artifacts, create the initiative board, and materialize its same-board dependency DAG. Never claim implementation work. |
| `implement` | Own exactly one ready task claim, lease every writable path, renew authority, verify, then transition to `in_review`. Stop writing immediately if authority expires. May request scoped self-recovery of its own expired task (`cortex_ia_work_recover_own`); unscoped recovery and retry stay orchestrator-only. |
| `reviewer` | Independently inspect and rerun checks; its only work-state mutation is `work approve`. Never edit, claim, lease, or self-approve as the active implementation owner. |

All roles execute natively; there is no external execution leaf, and no role may hand work to an external CLI.

- Only the orchestrator owns `cortex_session_start`, summaries, and session end. All dispatched roles are ephemeral subagents within that session.
- Parallel native writers may share one workspace without Git worktrees only when each controller owns a distinct live task claim and reserves each writable file individually with `cortex_ia_file_reserve` before editing that file. Acquire multiple files in deterministic sorted order, release each with `cortex_ia_file_release`, clean partial acquisition immediately on conflict, and stop writing on conflict or expiry. Mailbox/resource locks do not replace file reservations.
- `current_workspace` is the single supported implementation workspace strategy; `isolated_worktree` requests fail closed as retired.
- Implementation minions require a non-empty `allowed_files` list corresponding to leased repository paths. Read-only tasks, forensic audits, and unleased inspections (`allowed_files: []`) must route to `investigate` or `reviewer`. Verification commands in tasks and task DAGs must be raw, executable commands without comments or parenthetical explanations.
- Reviewer Proportionality & Adaptive Review Policy: Reviewers audit against actual repository diffs and declared acceptance criteria; NEVER fail or block tasks on synthetic test helpers or hypothetical inputs uncalled in the codebase. Non-code tasks (generated data/artifacts like Excel spreadsheets or CSVs, pure documentation, and declarative configs) strictly bypass AST re-indexing, cycle detection, code linters, and mutation testing; their verification is limited to artifact presence, format integrity, and security hygiene. Pure-test tasks must not undergo DAG decomposition upon failure.
- Zero-Noise Comments & Clean Code Policy: Implementers MUST NOT write narrative echo comments explaining what obvious code does, inline changelogs, or commented-out dead code. Comments explain non-obvious *WHY* or critical invariants only. Chat communication must remain terse, minimal, and surgical without conversational filler or intermediate narration.
- Orchestrator Executive Synthesis & Zero-Chatter: Orchestrators communicate using the 3-Layer Artifact Pyramid (Header, progressive disclosure synthesis, decoupled deep dossier). Chat is reserved exclusively for interactive decision gates and high-density delivery syntheses. Never emit stream-of-consciousness play-by-play chatter, copy-paste raw subagent receipts, or dump internal SQLite DAG state into chat.
- Adaptive Review Case Matrix (digest): the orchestrator auto-approves non-code kinds — generated data/artifacts, documentation, declarative config, and operational/DB scripts — per the matching case; an independent `reviewer` is mandatory only for high-risk code (concurrency/locks, production schema or irreversible DDL, public APIs/auth/crypto/security boundaries, > 3 files or > 150 LOC core logic, or failed/ambiguous/missing tests). Auto-approval runs exclusively through `cortex_ia_work_approve` with an evidence pointer.
  - Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §2.7a
- Workload LOC budget (digest): `strict` (source logic <= 350/250 LOC Go-family/TS-family, blocked + atomic decomposition), `flexible` (source logic <= 700/500 LOC, non-blocking advisory), or `unbounded` (diff checks bypassed); source logic covers Go/Rust/Java/C# and TS/Python (0.2x deletions), with test/fixture ceilings per tier.
  - Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §4

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
- `internal/delegation` owns the SQLite schema/migrations, task boards, DAG state, claims, TTL file leases, approvals, recovery, and structured receipts, plus the legacy read-only delegation-job projection served to the web console. Use `BEGIN IMMEDIATE` for multi-step authority transitions and fail closed on unknown future schema versions.
- `internal/herdr` retains optional Herdr setup and detection helpers for diagnostics; its only production consumer is the web console status display. Herdr never owns task state or approval.
- `internal/cortexiaweb` owns the Preact operations console compiled by Vite, embedded by `go:embed`, and served through its loopback-only HTTP/API server. It shows task-board sessions, work/claim/lease state, delegation jobs/transports, and the append-only activity stream. It may create boards/tasks but must not expose claim, lease, transition, retry, recovery, or approval mutations. Preserve CSP, request limits, server timeouts, auto-refresh, and the non-loopback rejection.
- `internal/agents/opencode` declares the OpenCode native layout (`layout.go`) and the pure asset mapping (`assetmap.go`): every destination is `path.Join(config root, source)`, validated against the single layout declaration, failing closed on unsafe paths, off-surface destinations, and collisions (including case-insensitive ones on Windows/macOS).
- `internal/components/filemerge` owns JSONC decode/merge and atomic writes; reuse it instead of writing ad-hoc merge code.
- Skills, prompts, commands, and plugins under `internal/assets/` are runtime source files embedded by `go:embed`; changing them requires rebuilding the binary. Shared skill contracts under `internal/assets/skills/_shared/` support installed agent instructions and must stay aligned with role files.

## OpenCode v2 (`opencode2`) Knowledge & Search Index

OpenCode v2 knowledge index and workflow diagrams moved to the `opencode2-knowledge` skill (`autoinvoke: false`; load explicitly when researching OpenCode v2).

## Security and Persistence Invariants

- SQLite uses `STRICT` tables, WAL, foreign keys, `busy_timeout`, a migration ledger, bounded values, and hashed authority tokens. Never store plaintext claim/lease tokens, secrets, full prompts, or unbounded stdout.
- External CLI execution is retired: there is no argv-based worker, temporary home, workspace-baseline acceptance, or external permission-skip path. All role controllers execute natively through OpenCode tools under work authority, and `isolated_worktree` requests fail closed. Do not enable unsafe permission bypasses by default.
- The embedded board server accepts only `localhost` or loopback IP addresses. Do not add CORS, remote binding, external assets, CDN dependencies, or browser endpoints that bypass work-control authority.
- Install/sync/uninstall ownership remains accreditation-based. Preserve verified backups, stale-plan detection, atomic writes, rollback on apply failure, and fail-closed behavior for unmanaged drift.
- Tests and smokes must use temporary homes. Never aim pipeline, delegation, board, or TUI verification at the developer's real OpenCode or Cortex state.

## Repository Workflow

- Before code changes, load the task-matched `SKILL.md`; SDD and utility skill sources are under `internal/assets/skills/`. Load `go-testing` when writing tests. See `docs/operations/sdd-workflow.md` only when the phase map is needed.
- PR CI requires a branch matching `<type>/<lowercase-name>`, a body containing `Closes #N`, `Fixes #N`, or `Resolves #N`, every linked issue labeled `status:approved`, and exactly one `type:*` PR label.
- Commit first lines are enforced only to 10-72 characters by Husky, but repository convention is Conventional Commits; release changelog inclusion depends on `feat`, `fix`, `refactor`, and `perf` prefixes.

The canonical workflow/phase matrix is `internal/assets/skills/_shared/workflow-map.md` (installed as `~/.cortex-ia/opencode/contracts/workflow-map.md`). Use it before routing SDD; do not assume one agent or skill per phase. `orchestrator` routes, `discovery` indexes the agentic environment, `investigate` diagnoses, `planner` owns proposal/spec/design/tasks/archive, `implement` executes, and `reviewer` independently verifies using `code-review-adversary`. Other installed utility skills are discovered from `internal/assets/skills/*/SKILL.md` and loaded only for their actual task trigger.

## Contract Digests (pointer-only)

Authoritative wording lives in the shared contracts under `internal/assets/skills/_shared/` (installed as `~/.cortex-ia/opencode/contracts/`). Each entry below is a digest: it points at the single normative section and never restates its rules.

- **Verbatim provenance** — L1 request captured verbatim with secret redaction, plus RED/GREEN appends carrying commit hashes: `cortex-convention.md` § Verbatim provenance.
- **Per-spec verdicts** — the per-spec verdict round-trip returns one verdict per referenced REQ ID (`verdicts: [{ req_id, verdict, evidence_ref }]`), with executable results overriding worker verdicts on conflict: `cortex-work-protocol.md` §8.1 Per-spec verdict protocol.
- **Escalate-only review tier** — the §2.7a matrix tier is a floor that may be raised with a stated reason and never lowered: `cortex-work-protocol.md` §2.7a Adaptive Review Case Matrix.
- **Resume test** — the resume test requires a task be resumable from the request text plus `git diff` alone: `workflow-map.md` § Task sizing: the resume test.
- **SDD repositioning** — Tier 1/2 spec is a short paragraph plus executable tests; SDD-lite/full is reserved for Tier 3 / multi-session / regulated domains; living specs evolve by delta (propose→apply→archive): `workflow-map.md` § Spec-plane repositioning.
- **Feature-doc Log + mirror read-back** — append-only `## Log` on sdd-lite plan docs; mirrored writes are read back and the authoritative copy is named on mismatch: `workflow-map.md` § Feature-doc Log and mirror reconciliation.
- **Advisory delivery forecast** — per-slice authored LOC from the task DAG, labeled advisory with no gate authority: `workflow-map.md` § Delivery forecast (advisory).
- **Skill Body Budget** — the single governing sizing rule for `SKILL.md` bodies, scoped away from the 4-layer XML anatomy that governs role prompts: `agent-writing-contract.md` § Skill Body Budget.
- **Ratchets (advisory)** — dead-code and refusal-string drift scripts under `scripts/`, baselines under `.cortex-ia/ratchet/`, advisory-only CI in `.github/workflows/ratchets.yml`.
- **Dated audits & evidence** — in-repo reports under `docs/audits/<YYYY-MM-DD>-<topic>.md` and `docs/evidence/`, two-way referenced with their Cortex observations: `docs/audits/README.md`, `docs/evidence/README.md`.
