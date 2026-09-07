# Cortex-IA Project Discovery Profile

## Evidence scope
Refreshed from repository layout and authoritative guides: `AGENTS.md`, `internal/assets/AGENTS.md`, shared contracts under `internal/assets/skills/_shared/`, native role assets, `go.mod`, `web/package.json`, `internal/app/delegation.go`, `internal/delegation/{store,work,board,runner,config,ledger}.go`, `internal/cortexiaweb/server.go`, and `web/src/main.jsx`. This is a reviewable cache; current manifests, schemas, rules, and runtime tool output supersede it.

## Stack and engines
- Go CLI/TUI module `github.com/lleontor705/cortex-ia`, Go `1.26.1`; Bubble Tea/Lip Gloss UI; modernc SQLite.
- OpenCode is the only supported host/platform. `cmd/cortex-ia` enters `internal/app`; zero arguments launch the TUI.
- CortexIA Web is isolated Preact `10.27.2` + Vite `7.1.5` (`web/`); Vite output is embedded under `internal/cortexiaweb/static/` via `go:embed`.
- Runtime state is SQLite at `~/.cortex-ia/delegation.db` (with explicit `CORTEX_IA_HOME` isolation override allowed by project guide). Herdr is optional transport/pane integration.

## Native agents and prompt assets
- Native controllers: `orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `reviewer` under `internal/assets/agents/`.
- Skills are `internal/assets/skills/*/SKILL.md`; notable routing/control assets: `orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `parallel-dispatch`, `using-git-worktrees`, `context-distiller`, `code-review-adversary`, `ast-impact-analysis`.
- Commands under `internal/assets/commands/` expose discover/investigate/SDD/TDD/review/work/status/resume and Cortex ingest/watch/code workflows.
- Shared canonical contracts: `cortex-work-protocol.md` (authority, lifecycle, delegation, receipts), `cortex-convention.md` (evidence, lineage, spec-plane), `agent-writing-contract.md` (English technical artifacts, pointers, completion), plus `codebase-design-contract.md` and `diagnosis-loop-contract.md`.
- Assets are embedded runtime source; changes require rebuilding the binary. `discovery` is the sole writer of this profile and cannot delegate.

## Orchestrator flow
`orchestrator` is primary and sole session/routing authority; it has no repository read/write/shell surface. It classifies into Fast Path, bounded Tier 2, or coordinated Tier 3 SDD. It dispatches native controllers only, carries context pointers rather than transcripts, and uses one stable Cortex session and one stable initiative board. Discovery returns this profile; planner owns OpenSpec/Cortex contract planning and DAG creation; implement owns one claim and file leases; reviewer independently verifies and approves.

## Three-plane contract model
- OpenSpec: human-reviewable contracts for `openspec|hybrid`; never runtime authority.
- Cortex MCP: durable evidence/memory/AST graph and pinned contracts for `spec_plane=cortex`; never claims, leases, transitions, or approval.
- Cortex-IA: authoritative boards, same-board dependency DAG, CAS revisions, claims, TTL file leases, approvals, delegation jobs, and operational events in SQLite.

## Delegation control plane
- `internal/app/delegation.go` is CLI intent parsing/rendering for `models`, `create`, `status`, `result`, `cancel`, `recover`, `worker`, and `set-pane`; service logic remains in `internal/delegation`.
- `delegation.Store` owns schema-backed jobs, work boards/items, claims, leases, reviews/approvals, recovery, ledger, worktrees, and dashboard aggregation. Work statuses are `backlog -> ready -> in_progress -> in_review -> done`, with `blocked` and `superseded` paths; PASS requires independent evidence.
- Tokens are random and hashed in SQLite; live claim/lease tokens must not enter prompts, logs, receipts, or Cortex. Multi-step authority transitions use immediate transactions; dependencies remain same-board/project.
- Effective execution mode is returned by the delegation bridge (`native`, `direct_cli`, `herdr_multiplexed`) and is authoritative. External AGY leaves execute bounded envelopes only; they receive no session, task-control, Cortex, approval, or nested-delegation authority. Herdr changes transport/presentation, not authority.

## Plugins and tools
`internal/assets/plugins/` contains Cortex integration, skill discovery, lease guard, task latch, subagent transport, Herdr bridge, and model variants. These are guard/transport/UI integrations; they must not replace SQLite authority. Retired ForgeSpec/task-board MCP surfaces must not be restored.

## CortexIA Web
`internal/cortexiaweb/server.go` serves embedded static assets and read APIs for overview, boards/snapshots, config, and delegations, plus create/archive/unarchive/delete board and create-task writes. It enforces loopback-only host/address, same-origin write checks, CSP/security headers, request-size limits, JSON unknown-field rejection, and server timeouts. The Preact console polls overview every 8s and board snapshots every 5s, presents boards/DAG flow, dependencies, claims, leases, delegation jobs, activity, and config. Web mutations intentionally exclude claim, lease, transition, retry, recovery, and approval authority operations.

## Audit seams and risks
- Canonical seams: `internal/app` dispatcher -> install/delegation services; `internal/delegation.Store` -> SQLite authority; `internal/assets` -> installed OpenCode runtime; `internal/cortexiaweb` -> read-focused console over Store; `web/src` -> embedded UI.
- Audit current schemas/migrations and active Cortex rule inventory before planning. Verify asset mapping/layout safety, token custody, loopback/CSP defenses, same-board dependency invariants, and effective-mode reconciliation. Do not treat UI cards, chat, tests alone, Herdr pane state, or external receipts as authority.

## Discovery constraints
No builds, tests, installs, dependency restoration, services, database connections, Cortex ingestion, or product edits were performed for this refresh.
