# Cortex-IA Project Discovery

> Generated: 2026-09-12T21:30:00Z · Repository revision: `684f6332e25e5325fe76a47a9bf58ea3db871a44`

## Project identity

| Item | Evidence |
|---|---|
| Repository/root | `github.com/lleontor705/cortex-ia`, `D:/cortex-ia` (`go.mod:1`, `package.json:13-16`) |
| Cortex project | `cortex-ia` (dispatch and indexed graph evidence) |
| Cortex runtime | Local Zero-CGO SQLite, version `2.0.0`; FTS5, graph, scoring, temporal, DNA, handoff, hybrid search, AST, rules, architecture, repo-map, test-impact, symbol-search and agent-context capabilities (`cortex_get_status`) |
| Initiative context | Session/board identity supplied by dispatch: `cortex-gentle-improvements-20260912` / `gentle-inspired-improvements`; discovery performed no lifecycle or board/task mutations |
| Selected plane | `hybrid`; no implementation task/specification artifacts supplied to discovery |
| Working tree baseline | Clean: `git status --short` returned no paths at discovery time |

## Installed skills

### Filesystem skills

| Scope | Count | Evidence |
|---|---:|---|
| Embedded project skills | 15 | `internal/assets/skills/*/SKILL.md` |
| Names | 15 | `ast-impact-analysis`, `code-review-adversary`, `context-distiller`, `discovery`, `fast-tdd`, `grill-me`, `hotfix-triage`, `implement`, `investigate`, `mutation-testing`, `orchestrator`, `parallel-dispatch`, `planner`, `property-based-testing`, `spike-prototype` |
| User-installed skills | Available in host inventory | OpenCode environment skill list; no project-local `.agents/skills` files found |

### Cortex skills

No separate Cortex skill-list result is exposed by the active schema. Count: **unknown**. Available Cortex capabilities are recorded above and are not interpreted as skill count.

## Languages and project types

| Type | Evidence |
|---|---|
| Go CLI/TUI | `go.mod:1-10`, `cmd/cortex-ia/main.go`, `internal/app`, `internal/tui` |
| SQLite persistence | `modernc.org/sqlite` in `go.mod:10`; `internal/delegation` |
| Embedded OpenCode assets | `internal/assets/assets.go` (`go:embed`); `internal/agents/opencode` |
| Preact/Vite web console | `web/package.json:1-18`, `web/src`, `web/vite.config.js` |
| TypeScript/OpenTUI asset toolchain | `internal/tuiassets/package.json:1-17`, `internal/tuiassets/tsup.config.ts` |
| Root Node metadata | `package.json:9-11`; root `npm test` is intentionally not the product test runner |

## Required engines and developer tooling

| Tool | Requirement | Local state/evidence |
|---|---|---|
| Go | Required target `1.26.1` (`go.mod:3`, `AGENTS.md:13`) | Available: `go version go1.26.5 windows/amd64` |
| Node/npm | Required for web or TUI asset source changes (`AGENTS.md:16`) | Available: Node `v24.20.0`, npm `11.19.0` |
| pnpm | Relevant to TUI lock/workspace metadata | Unknown; not probed |
| TypeScript/tsup/OpenTUI | Required for TUI asset generation; versions declared in `internal/tuiassets/package.json:8-15` | Dependencies/node_modules presence not revalidated; no build run |
| Vite | Required for web source changes (`web/package.json:6-9`) | Declared; executable not independently probed |
| Git | Required repository control | Available; revision/status observed; version not probed |
| golangci-lint | Recommended verification (`AGENTS.md:61`) | Unknown; not probed |

## Data and infrastructure dependencies

- SQLite is in-process and owns durable boards, DAG work, claims, leases, approvals and receipts (`AGENTS.md:35-40,77`; `internal/delegation`).
- Local filesystem/OpenCode home are core installation resources (`AGENTS.md:69-82`).
- Herdr is optional transport/process infrastructure and never task authority (`AGENTS.md:78`).
- Telemetry/reporting spans `internal/telemetry`, `internal/app/report.go`, `internal/delegation/runner.go`, and `cmd/cortex-report-hub/main.go`; plugin bridges are under `internal/assets/plugins/`.
- No database server/container dependency was evidenced. Secrets, credentials and full user configuration were not inspected.

## Cortex governance map

| Identifier/title | Source/scope | Applicability |
|---|---|---|
| No active Cortex rules | `cortex_get_rules(project=cortex-ia)`; total 0 | No project-specific Cortex directives available |
| Project contract | `D:/cortex-ia/AGENTS.md` | OpenCode-only scope, SQLite authority, current workspace, security, testing and lifecycle boundaries |
| Workflow map | `internal/assets/skills/_shared/workflow-map.md` | Canonical route and SDD phase/artifact matrix |
| Work protocol v3.0 | `internal/assets/skills/_shared/cortex-work-protocol.md` | Role authority, claims/leases, delegation, workload, receipts and closure |
| Evidence convention | `internal/assets/skills/_shared/cortex-convention.md` | Hybrid contract representation, pins, evidence taxonomy and recovery |
| Design contract | `internal/assets/skills/_shared/codebase-design-contract.md` | Module/interface/seam/adapter vocabulary and task graph boundaries |

## Architecture and patterns

### Modules and interfaces

- `cmd/cortex-ia/main.go` composes into `internal/app`; `internal/app/app.go` separates Bubble Tea TUI from hand-written CLI dispatch (`AGENTS.md:71`).
- `internal/install` is the service facade for install/sync/doctor/rollback/uninstall/MCP, collaborating with `internal/pipeline`, `internal/backup`, `internal/state`, `internal/installmeta` and `internal/mcpmanager` (`AGENTS.md:72-76`).
- `internal/delegation` owns SQLite schema/migrations, jobs, boards, DAG state, claims, TTL leases, recovery, approvals and receipts (`AGENTS.md:77`).
- `internal/herdr` is the optional process/transport adapter; it does not own authority (`AGENTS.md:78`).
- `internal/cortexiaweb` serves the Vite-built, embedded console with loopback-only restrictions (`AGENTS.md:79`).
- Cortex graph currently indexes 1,616 symbols, 5,416 relations and 143 files (`cortex_analyze_architecture`; indexed evidence may not represent future edits).

### Seams and adapters

- Filesystem/home seam: `internal/pipeline`, `internal/backup`, `internal/state`, `internal/agents/opencode`.
- SQLite authority seam: `internal/delegation` and its transactional store.
- Process/transport seam: AGY runner in `internal/delegation`, Herdr adapter in `internal/herdr`.
- HTTP/telemetry seam: `internal/telemetry`, `cmd/cortex-report-hub`, and plugin bridges under `internal/assets/plugins`.
- Embedded static seam: `internal/cortexiaweb/server.go` plus `web/src` generated output.
- TUI plugin seam: `internal/tuiassets/cortex-ia-tui.tsx` and generated `internal/assets/tui/cortex-ia-tui.js`.

### Dependency direction and risks

- Architecture analysis identifies high-coupling hotspots: `internal/delegation/store.go:Store` degree 148; `internal/install/service.go:New` degree 107; `internal/tui/model.go:model` degree 99; `internal/delegation/store.go:Close` degree 73; `internal/install/service.go:Service` degree 79. These are observed hotspots, not redesign directives.
- Cycle detection reports 10 symbol/call cycles, concentrated in `internal/mcpmanager`, `internal/install`, `internal/pipeline`, and `internal/installmeta`. Treat as bounded review evidence; do not assume every reported cycle is an import defect.
- Communities include delegation, TUI, pipeline, install, app and state, with low reported cohesion scores; interpretation belongs to planning/review.
- For the next phase, preserve SQLite authority, current-workspace exclusivity, explicit recovery, and existing module ownership. New seams should be placed at real filesystem/process/protocol variation boundaries per the design contract.

## Domain vocabulary and decision records

- Canonical terms: OpenCode native assets, Cortex contract, AGY execution leaf, Herdr transport, board, DAG/work item, claim, TTL lease, approval, receipt, install/sync/rollback/recovery, managed MCP, ownership fingerprint, current workspace, telemetry, handoff, observation snapshot.
- Decision/context records: `AGENTS.md`, `docs/architecture.md`, `docs/CODEBASE-GUIDE.md`, `docs/codebase/*.md`, `SPEC.md`, `docs/sdd-workflow.md`, `CHANGELOG.md`, Cortex observations `209`, `210`, `211`.
- Dispatch-selected policy changes to plan (user-authorized constraints, not yet implementation decisions): auto execution, hybrid plane, current workspace, authenticated updates, and small persistent regression tests for critical authority/transport/recovery/update-verification seams, in addition to existing TUI/install coverage.
- Comparison findings are intentionally not repeated here; observations 209-211 are evidence references for planner/investigator context.

## Development guardrails

- No secrets, key activation, release publishing, active installation sync or deployment are authorized by the dispatch.
- Preserve OpenCode-only scope; do not restore retired adapters/personas/model routing/external task-board or ForgeSpec surfaces (`AGENTS.md:7-9`).
- Current workspace is the only supported external implementation strategy; external leaves remain exclusive and must preserve the clean baseline (`AGENTS.md:56-57`; work protocol).
- Claims, leases, transitions and approvals remain SQLite authority; only independent reviewer PASS produces `done` (work protocol:87-107).
- Hybrid SDD uses OpenSpec artifacts plus Cortex evidence. Planner must validate phase artifacts, bind requirements to tasks, then materialize one stable board/DAG; planner archives only after approvals (`workflow-map.md:19-49`).
- Persistent regression tests authorized for critical seams must remain bounded/modular, use temporary homes and avoid secrets/real developer configuration (`AGENTS.md:62-67`; design contract:63-68). This is an implementation planning constraint, not a discovery policy edit.
- Authenticated update work must preserve fail-closed verification and must not activate keys or publish releases under this authorization.

## Canonical verification commands

Discovery did not build, test, ingest, install, start services, connect to databases, or mutate product files. Recommended commands from checked-in guidance:

```text
gofmt -s -w .
go vet ./...
golangci-lint run ./...
go test -count=1 ./...
go test ./internal/tui/...
go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1
npm --prefix internal/tuiassets run build
npm --prefix web run build
go build -o bin/cortex-ia ./cmd/cortex-ia
```

For TUI source changes, generate assets before Go build and commit source plus generated output. For web source changes, build web assets before Go build and retain generated `internal/cortexiaweb/static/**` (`AGENTS.md:16`, `internal/assets/assets.go`).

## Unknowns and blockers

- `pnpm --version`, `golangci-lint`, OpenCode runtime behavior, delegation configuration, Herdr state and telemetry endpoint behavior were not probed.
- AST evidence is indexed and current symbols were available, but discovery did not ingest; future dirty changes may not be reflected until reviewer delta ingestion.
- No Cortex skill-list endpoint/result is exposed; Cortex skill count remains unknown.
- No runtime reproduction, performance measurement or exhaustive comparative diagnosis was performed; observations 209-211 explicitly remain bounded/static evidence.
- Exact OpenSpec change directory and requirement IDs for the pending initiative are not yet supplied; planner must establish them during `sdd-full` planning.
- No claim is made about release key possession/activation, secret-derived values, or external update endpoint state.
- No secrets, private keys, credentials, full configuration, transcripts or external services were inspected.

## Evidence index

- `D:/cortex-ia/go.mod:1-10` — Go module, target and SQLite/runtime dependencies.
- `D:/cortex-ia/package.json:9-27` — root Node/Husky metadata and intentionally failing test script.
- `D:/cortex-ia/web/package.json:1-18` — Preact/Vite toolchain.
- `D:/cortex-ia/internal/tuiassets/package.json:1-17` — TUI asset build and declared versions.
- `D:/cortex-ia/AGENTS.md:3-17,33-40,59-90,92-98` — project contract, tooling, authority, architecture, security and verification.
- `D:/cortex-ia/internal/assets/skills/_shared/workflow-map.md:1-53` — routing, SDD-full phases, validation, bindings and closure.
- `D:/cortex-ia/internal/assets/skills/_shared/cortex-work-protocol.md:9-17,19-30,36-45,66-109,111-154` — three planes, role boundaries, task lifecycle, workload and receipts.
- `D:/cortex-ia/internal/assets/skills/_shared/cortex-convention.md:5-23,25-64` — evidence, hybrid contracts, pins and recovery.
- `D:/cortex-ia/internal/assets/skills/_shared/codebase-design-contract.md:7-30,32-52,54-68` — architecture vocabulary, seams and task boundaries.
- Cortex results: `cortex_get_status`, `cortex_get_rules`, `cortex_get_code_symbols`, `cortex_analyze_architecture`, `cortex_detect_cycles`.
- Cortex observations `209`, `210`, `211` — supplied comparison corrections and bounded recommendations; not reanalyzed here.

Profile path: `D:/cortex-ia/.cortex-ia/discovery.md`. This refresh wrote only the discovery profile.
