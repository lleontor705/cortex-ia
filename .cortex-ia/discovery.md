# Cortex-IA Project Discovery

> Generated: 2026-09-10T21:05:00Z · Repository revision: `02af69a7ff722f2c781cc3188af9025b50a05f46`

## Project identity

| Item | Evidence |
|---|---|
| Repository/root | `github.com/lleontor705/cortex-ia`, `D:/cortex-ia` (`go.mod:1`, `package.json:15`) |
| Cortex project | `cortex-ia` (dispatch and indexed graph evidence) |
| Cortex runtime | Local Zero-CGO SQLite, version `2.0.0`; capabilities returned: FTS5, knowledge graph, scoring, temporal, DNA, handoff, hybrid search, AST extraction, rules, blast radius, architecture analysis, repo map, test impact, symbol search, agent context (`cortex_get_status`) |
| Selected plane | `cortex`; no task/spec artifacts supplied |
| Current session | Root session ID supplied by orchestrator: `ses_f730d1fd4ffeF0jvNf7E3zKxYh`; discovery did not invoke lifecycle tools |

### Dirty baseline (exact paths; preserved)

- `internal/assets/agents/discovery.md`
- `internal/assets/agents/implement.md`
- `internal/assets/agents/investigate.md`
- `internal/assets/agents/orchestrator.md`
- `internal/assets/agents/planner.md`
- `internal/assets/agents/reviewer.md`
- `internal/assets/plugins/cortex.ts`

The only permitted write in this refresh was this profile. The seven pre-existing dirty paths were not edited or reset; their semantic diff was not inspected because the active policy denied the bounded `git diff` command. Treat them as user changes and preserve them.

## Installed skills

### Filesystem skills

| Scope | Count | Evidence |
|---|---:|---|
| Embedded project skills | 15 | `internal/assets/skills/*/SKILL.md` |
| Names | 15 | `ast-impact-analysis`, `code-review-adversary`, `context-distiller`, `discovery`, `fast-tdd`, `grill-me`, `hotfix-triage`, `implement`, `investigate`, `mutation-testing`, `orchestrator`, `parallel-dispatch`, `planner`, `property-based-testing`, `spike-prototype` |
| User-installed inventory | Available in environment | `C:/Users/usrLuisLeon/.agents/skills/*`; active environment listed the same named skills, including `discovery` |

### Cortex skills

No separate Cortex skill-list result was available through the active schema. Do not interpret this as zero skills: Cortex status confirms capabilities including `handoff`, `hybrid_search`, `agent_context`, and AST/architecture tools. **Count unknown.**

## Languages and project types

| Type | Evidence |
|---|---|
| Go CLI/TUI | `go.mod:1-10`, `cmd/cortex-ia/main.go`, `internal/app`, `internal/tui` |
| SQLite persistence | `modernc.org/sqlite` in `go.mod:10`; `internal/delegation` |
| OpenCode embedded assets | `internal/assets/assets.go` (`go:embed`); `internal/agents/opencode` |
| Solid/OpenTUI plugin asset | `internal/tuiassets/cortex-ia-tui.tsx`, `internal/tuiassets/package.json` |
| Preact/Vite web console | `web/package.json`, `web/src`, `web/vite.config.js`; output `internal/cortexiaweb/static` |
| Root Node metadata | `package.json`; Husky only and intentionally failing placeholder `test` script |

## Required engines and developer tooling

| Tool | Requirement | Bounded observation |
|---|---|---|
| Go | Required `1.26.1` by `go.mod:3` and `AGENTS.md:13` | `go version go1.26.5 windows/amd64` observed; newer patch release, compatible target needs confirmation by maintainers |
| Node/npm | Required for TUI asset or web source changes | Node `v24.20.0`, npm `11.19.0` observed |
| pnpm | Required by `internal/tuiassets` lock/workspace metadata | Executable not probed; local `node_modules` and `pnpm-lock.yaml` are present |
| TypeScript/tsup/OpenTUI | Required for sidebar source generation | Declared in `internal/tuiassets/package.json`: TypeScript 5.9.3, tsup 8.5.1, `@opencode-ai/plugin` 1.18.18, OpenTUI 0.4.5; installed node_modules observed, no build run |
| Vite | Required only for web source changes | Declared by `web/package.json`; no build run |
| Git | Required repository control | Revision and status observed; git version not probed |
| golangci-lint | Recommended verification tool | Not probed |

## Data and infrastructure dependencies

- SQLite is an in-process deterministic dependency; `internal/delegation` owns durable boards, DAG work, claims, leases, approvals and receipts (`AGENTS.md:35-40,77`).
- Local filesystem and OpenCode home/config are core install and asset resources (`AGENTS.md:69-82`).
- Optional Herdr is a process/transport adapter and never owns task authority (`AGENTS.md:78`).
- Telemetry ownership is split: `internal/telemetry/report.go` owns config, report creation, endpoint normalization, sending and non-blocking auto-report; `internal/app/report.go` exposes CLI/config behavior; `internal/delegation/runner.go` reports delegated failures; `cmd/cortex-report-hub/main.go` persists/serves reports. Current source also shows telemetry-related plugin bridges in `internal/assets/plugins/cortex.ts` and `herdr-bridge.ts`.
- Cortex durable handoff is distinct from generic `cortex_save`: `cortex.ts` documents that `cortex_handoff` is delivered by MCP and is not interpreted or fabricated by the plugin (`internal/assets/plugins/cortex.ts:30-32,694-698,863-866`).
- Local snapshot export is a separate bounded read path: `internal/assets/plugins/cortex-snapshot.ts` invokes local Cortex export, requires a numeric observation ID/project match, hashes exact UTF-8 content, bounds export to 8 MiB and content to 1 MiB, and writes no files. This is evidence of a full-read snapshot capability, not a substitute for remote MCP (`cortex-snapshot.ts:8-50`).
- No database server/container dependency was evidenced. Secrets and full user configuration were not inspected.

## Cortex governance map

| Identifier/title | Source/scope | Applicability |
|---|---|---|
| No active Cortex rules | `cortex_get_rules(project=cortex-ia)` returned total 0 | No project-specific Cortex directives available |
| Project contract | `D:/cortex-ia/AGENTS.md` | OpenCode-only product, current workspace, SQLite authority, security and verification invariants |
| Codebase design contract | `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/codebase-design-contract.md` | Module/interface/seam/adapter vocabulary; discovery observes and does not redesign |
| Workflow map | `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/workflow-map.md` | Discovery route, Cortex-plane phase matrix and task binding vocabulary |
| Work protocol | Installed contract path referenced by `AGENTS.md` | Role boundaries, authority, receipts and execution modes; exact file was not reread in this bounded refresh |

Installed contracts are governance evidence for future controllers; this profile remains advisory and current manifests/source/rules win on conflict.

## Architecture and patterns

### Modules and interfaces

- Composition: `cmd/cortex-ia/main.go` calls `internal/app`; `internal/app/app.go` separates no-argument Bubble Tea TUI from hand-written CLI dispatch (`AGENTS.md:69-72`).
- Install facade: `internal/install` coordinates install/sync/doctor/rollback/uninstall/MCP; `internal/pipeline`, `internal/backup`, `internal/state`, `internal/installmeta`, and `internal/mcpmanager` are collaborators.
- Delegation authority: `internal/delegation` owns SQLite schema/migrations, jobs, boards, DAG state, claims, TTL leases, recovery, approvals and receipts.
- Web console: `web/src` -> Vite -> `internal/cortexiaweb/static`; Go serves embedded static files through `internal/cortexiaweb/server.go` (`go:embed`).
- OpenCode plugin bridge: `internal/assets/plugins/cortex.ts` translates OpenCode hooks/tool surfaces to bounded HTTP calls and local Cortex CLI integration; it explicitly bounds payloads/logs and separates handoff semantics.
- Indexed graph snapshot: 1,530 symbols, 5,164 relations and 142 files (`cortex_analyze_architecture`); graph and cycle evidence may predate current dirty changes because discovery did not ingest.

### Seams and adapters

- Filesystem/home seam: install, backup, state and OpenCode layout (`internal/pipeline`, `internal/backup`, `internal/state`, `internal/agents/opencode`).
- Process seam: AGY runner in `internal/delegation`; optional Herdr transport adapter in `internal/herdr`.
- HTTP persistence seam: telemetry/report hub and Cortex local HTTP/CLI bridge (`cmd/cortex-report-hub`, `internal/telemetry`, `internal/assets/plugins/cortex.ts`).
- Embedded static seam: `internal/cortexiaweb/server.go` embeds generated web output and enforces loopback/server boundaries per `AGENTS.md:79,88`.
- TUI plugin seam: OpenCode TUI imports `@opencode-ai/plugin/tui`, OpenTUI Solid and Node process/filesystem APIs (`internal/tuiassets/cortex-ia-tui.tsx:1-17`).

### Dependency direction and risks

- Architecture analysis identifies high-coupling nodes: `internal/delegation/store.go:Store` degree 148, `internal/install/service.go:New` degree 104, `internal/tui/model.go:model` degree 99, `internal/delegation/store.go:Close` degree 71, and `internal/install/service.go:Service` degree 79. These are hotspots, not redesign instructions.
- Cycle detector returned 10 symbol/call cycles, concentrated in `internal/mcpmanager/{presets.go,qualification.go}`, `internal/install/{doctor.go,service.go}`, `internal/pipeline/{journal.go,engine.go,plan.go}`, and `internal/installmeta/mcpdigest.go`. Preserve as observed evidence; do not assume each is an import defect.
- Community labels include delegation, tui, pipeline, install, app, state and backup. Returned cohesion scores are low; interpretation belongs to investigation/planning.

## Domain vocabulary and decision records

- Canonical terms: OpenCode native assets, Cortex contract, AGY execution leaf, Herdr transport, board, DAG/work item, claim, TTL lease, approval, receipt, install/sync/rollback/recovery, managed MCP, ownership fingerprint, current workspace, telemetry, handoff, observation snapshot.
- Decision and architecture references: `AGENTS.md`, `docs/architecture.md`, `docs/CODEBASE-GUIDE.md`, `docs/codebase/*.md`, `SPEC.md`, `docs/sdd-workflow.md`, `CHANGELOG.md`.
- Current tranche supplied by dispatch: telemetry minimization and truthful adaptive OpenCode sidebar. Roadmap mentions receipts/continuity/web; this profile does not redesign or turn roadmap items into requirements.

## Development guardrails

- Preserve the seven dirty paths exactly; no product/config edits were made.
- Sidebar source of truth is `internal/tuiassets/cortex-ia-tui.tsx`; generated runtime asset is `internal/assets/tui/cortex-ia-tui.js`. `internal/tuiassets/tsup.config.ts:4-12` defines entry, ESM bundle, Node 22 target and output `../assets/tui`.
- Sidebar generation command (declared by `internal/tuiassets/package.json:5`): `npm --prefix internal/tuiassets run build` (or equivalent local package-manager invocation); this was not executed.
- Go embedding includes `all:tui` through `internal/assets/assets.go`; changing generated/sidebar assets requires rebuilding the binary for runtime delivery (`AGENTS.md:82`, `internal/assets/assets.go`).
- TUI plugin reads bounded board/delegation snapshots and renders sidebar sections; source constants show 2.5-second polling, 10-second staleness and three persisted expansion keys (`internal/tuiassets/cortex-ia-tui.tsx:19-27`; generated asset lines 14-21).
- Telemetry minimization must preserve bounded output, no tokens/payloads/response bodies in logs, UTF-8 truncation metadata, deadline-bounded hooks and handoff neutrality as declared in `internal/assets/plugins/cortex.ts:14-32`.
- Root `npm test` is intentionally not a product test runner (`package.json:9-11`).

## Canonical verification commands

Discovery did not build, test, ingest, start services, install, or connect to databases. Recommended commands from checked-in guidance:

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

For sidebar source changes, run the TUI asset generation first, then the Go build; commit both `internal/tuiassets/cortex-ia-tui.tsx` and generated `internal/assets/tui/cortex-ia-tui.js` when authorized. For web source changes, run the web build before Go build and retain generated `internal/cortexiaweb/static/**`.

## Unknowns and blockers

- `git diff` semantic scope could not be inspected: active command policy denied the bounded diff request. The exact dirty path list above is from successful `git status --short` and must be treated as the preservation baseline.
- No current Cortex rule IDs exist; the rule count is exactly zero. Installed contracts are governance context, not Cortex rule records.
- No separate Cortex skill-list endpoint/result was available; Cortex capability count is not a skill count.
- Indexed AST evidence is present but may predate dirty files; discovery did not trigger ingestion.
- `pnpm --version`, `golangci-lint`, OpenCode runtime version/behavior, runtime consent, event delivery, AGY acceptance, Herdr state and actual telemetry endpoint behavior were not probed.
- No claim is made that generic save and durable handoff are interchangeable: source explicitly separates them. Full-read local snapshot export is evidenced by `cortex-snapshot.ts`; remote snapshot semantics remain unknown.
- No secrets, full user config, transcripts, database contents, or external services were inspected.

## Evidence index

- `D:/cortex-ia/go.mod:1-10` — module, Go target and dependencies.
- `D:/cortex-ia/package.json:9-27` — root Node/Husky metadata.
- `D:/cortex-ia/web/package.json:1-18` — Preact/Vite scripts and dependencies.
- `D:/cortex-ia/internal/tuiassets/package.json:1-17` — sidebar build script and versions.
- `D:/cortex-ia/internal/tuiassets/tsup.config.ts:1-25` — actual sidebar entry/output pipeline.
- `D:/cortex-ia/internal/tuiassets/cortex-ia-tui.tsx:1-27,1061` — TUI imports, polling and sidebar entry.
- `D:/cortex-ia/internal/assets/tui/cortex-ia-tui.js:1-21,1349` — generated runtime sidebar asset.
- `D:/cortex-ia/internal/assets/assets.go:16` — embedded asset inventory.
- `D:/cortex-ia/internal/assets/plugins/cortex.ts:14-32,205-267,694-698,851-941` — telemetry minimization, tool classification, handoff neutrality and hook delivery.
- `D:/cortex-ia/internal/assets/plugins/cortex-snapshot.ts:8-50` — bounded full-read local snapshot export/hash capability.
- `D:/cortex-ia/internal/telemetry/report.go`, `internal/app/report.go`, `internal/delegation/runner.go`, `cmd/cortex-report-hub/main.go` — telemetry ownership.
- `D:/cortex-ia/AGENTS.md:11-17,69-90` — toolchain, architecture, security and verification guardrails.
- `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/codebase-design-contract.md` — shared architecture vocabulary.
- `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/workflow-map.md` — workflow and Cortex-plane matrix.
- Cortex results: `cortex_get_status`, `cortex_get_rules`, `cortex_get_code_symbols`, `cortex_get_code_graph`, `cortex_analyze_architecture`, `cortex_detect_cycles`.

Profile path: `D:/cortex-ia/.cortex-ia/discovery.md`. This refresh intentionally wrote only the discovery profile.
