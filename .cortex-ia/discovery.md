# Cortex-IA Project Discovery

> Refreshed: 2026-09-21 (post kill-AGY certification audit) · Repository revision: working tree based on `0216da34beb2ae74574808d88431ce331110924c` (`git rev-parse HEAD`)
>
> **Refresh trigger:** the certification audit (session `ses_f3a11bc56ffe0tQBrGB4O4wr5E`) flagged this profile as describing the retired AGY execution path as canonical. Prior profile was generated 2026-09-12 against `684f6332e25e5325fe76a47a9bf58ea3db871a44`. This profile is a reviewable cache of observations, not authority: current manifests, repository evidence, active Cortex rules and tool output win on conflict.

## Project identity

| Item | Evidence |
|---|---|
| Repository/root | `github.com/lleontor705/cortex-ia`, `D:/cortex-ia` (`go.mod:1`, `package.json:13-16`) |
| Cortex project | `cortex-ia` (dispatch and indexed graph evidence) |
| Cortex runtime | Local Zero-CGO SQLite, version `2.0.0`; capabilities: fts5_search, knowledge_graph, scoring, temporal, dna, handoff, hybrid_search, ast_extraction, rules_directives, blast_radius, architecture_analysis, repo_map, test_impact, symbol_search, agent_context (`cortex_get_status`) |
| Execution doctrine | **Native-only.** No external execution leaf. OpenCode role controllers execute under Cortex-IA work authority (`AGENTS.md:5-6,19-21,43`) |
| In-flight initiative | Board `agent-flow-hardening` (change `agent-flow-hardening`, hybrid plane, sdd-lite): GAP-03 retire AGY, GAP-05 review-FAIL circuit breaker, GAP-06 workload budgets |
| Selected plane | Not supplied by this dispatch; profile records both planes (OpenSpec artifacts under `openspec/changes/`, Cortex evidence/observations) |
| Working tree baseline | **Dirty** (AGY-removal wave): ~60 modified/deleted paths plus untracked `openspec/changes/agent-flow-hardening/` and new test files (`git status --short`). Expected, not a workspace violation |

## Installed skills

### Embedded project skills

| Scope | Count | Evidence |
|---|---:|---|
| Embedded skill sources | 17 | `internal/assets/skills/*/SKILL.md` |
| Names | 17 | `ast-impact-analysis`, `code-review-adversary`, `context-distiller`, `discovery`, `document-reader`, `fast-tdd`, `grill-me`, `hotfix-triage`, `implement`, `investigate`, `mutation-testing`, `parallel-dispatch`, `planner`, `property-based-testing`, `spike-prototype`, `system-diagrams`, `workflow-retrospective` |
| Installed role agents | 6 | `internal/assets/agents/orchestrator.md`, `discovery.md`, `investigate.md`, `planner.md`, `implement.md`, `reviewer.md` (`orchestrator` is an agent role, not a skill; the prior profile miscounted it as one) |
| Shared contracts | — | `internal/assets/skills/_shared/`: workflow-map, cortex-work-protocol, cortex-convention, codebase-design-contract, diagnosis-loop-contract |

### Repository-local skills (`.agents/skills/`)

| Skill | Path |
|---|---|
| `opencode-theme-dev` | `.agents/skills/opencode-theme-dev/SKILL.md` |
| `opencode-plugin-dev` | `.agents/skills/opencode-plugin-dev/SKILL.md` |
| `opencode-installer-dev` | `.agents/skills/opencode-installer-dev/SKILL.md` |
| `prompt-design` | `.agents/skills/prompt-design/SKILL.md` |

### User-scoped skills

`.cortex-ia/skill-registry.md` records 16 user-scoped skills under `C:\Users\usrLuisLeon\.agents\skills\` (last synchronized 2026-09-12T17:38:44Z; includes a user `orchestrator` skill distinct from the embedded agent role).

### Cortex skills

No Cortex skill-list tool is exposed by the active MCP schema. Count: **unknown**. Cortex capabilities (not skills) are recorded in the identity table above.

## Languages and project types

| Type | Evidence |
|---|---|
| Go CLI/TUI | `go.mod:1-10`, `cmd/cortex-ia/main.go`, `internal/app`, `internal/tui` |
| SQLite persistence | `modernc.org/sqlite` (`go.mod:10`); `internal/delegation` |
| Embedded OpenCode assets | `internal/assets/assets.go` (`go:embed`); subdirs `agents`, `commands`, `plugins`, `skills`, `themes`, `tui` |
| OpenCode native layout/mapping | `internal/agents/opencode` (`layout.go`, `assetmap.go`) |
| TypeScript plugins | 8 sources under `internal/assets/plugins/`: cortex.ts, cortex-work.ts, cortex-task-latch.ts, cortex-subagent-transport.ts, cortex-snapshot.ts, cortex-skill-discovery.ts, cortex-permission-fence.ts, cortex-lease-guard.ts |
| Preact/Vite web console | `web/package.json:1-18`, `web/src`, `web/vite.config.js` |
| TypeScript/OpenTUI asset toolchain | `internal/tuiassets/package.json:1-17`, `internal/tuiassets/tsup.config.ts`, `internal/tuiassets/cortex-ia-tui.tsx` |
| Node probe/scripts | `.cortex-ia/probes/adoption-consent-probe.mjs`, `scripts/*.mjs` |
| Root Node metadata | `package.json:9-11`; root `npm test` intentionally fails and is not the test runner |

## Required engines and developer tooling

| Tool | Requirement | Local state/evidence (probed 2026-09-21) |
|---|---|---|
| Go | Target `1.26.1` (`go.mod:3`, `AGENTS.md:13`) | `go version go1.26.5 windows/amd64` |
| Node/npm | Required for web or TUI asset source changes (`AGENTS.md:14-16`) | Node `v24.21.0`, npm `11.19.0` |
| git | Required repository control | `git version 2.55.0.windows.3` |
| golangci-lint | Required local gate (`AGENTS.md:55`) | `2.12.2` built with go1.26.5 |
| pnpm | Ruled out — npm is the declared package manager | Not probed; not required by any checked-in script |
| TypeScript/tsup/OpenTUI | TUI asset generation (`internal/tuiassets/package.json:8-15`) | Declared; `node_modules` presence not revalidated; no build run |
| Vite | Web source changes (`web/package.json:6-9`) | Declared; not independently probed |
| `go vet` | Local gate | Not executed by discovery (no builds authorized) |

## Data and infrastructure dependencies

- SQLite is in-process and owns durable boards, DAG work, claims, TTL file leases, approvals and receipts (`AGENTS.md:25-29,73`; `internal/delegation`). Single local database: `~/.cortex-ia/delegation.db`; `CORTEX_IA_HOME` is an explicit isolated state-root override for automation/smokes only.
- Local filesystem + OpenCode home are core installation resources (`AGENTS.md:67-78`).
- **Herdr is diagnostics-only** optional tooling: setup/detection helpers in `internal/herdr/setup.go` (`Install`, `Setup`, `Status`, `ResolveHerdr`, `HerdrRunning`); its sole production consumer is the web console status display (`internal/cortexiaweb/server.go:253-254`). Herdr never owns task state or approval (`AGENTS.md:74`).
- Telemetry/reporting spans `internal/telemetry`, `internal/app/report.go`, and `cmd/cortex-report-hub/main.go`; plugin bridges under `internal/assets/plugins/`. The former `internal/delegation/runner.go` telemetry/reporting path no longer exists (see Retired surface).
- No database server or container dependency was evidenced. Secrets, credentials and full user configuration were not inspected.

## Cortex governance map

| Identifier/title | Source/scope | Applicability |
|---|---|---|
| Rule `[132]` `patterns/install-cortex-preflight` | `cortex_get_rules(project=cortex-ia)`, scope `project` | Cortex-IA specific: `Sync()` calls `preflightCortexBinary` once before `pipeline.PlanSync`; warnings are non-fatal; do not duplicate the helper |
| Rule `[125]` `patterns/docs-language-spanish` | same, scope `project` | **Out-of-project**: describes `docs/PERU-COMPLIANCE.md` in an unrelated ats-inventory codebase; not a Cortex-IA constraint |
| Rule `[105]` `patterns/sales-nullable-quant-contract` | same, scope `project` | **Out-of-project**: describes ats-inventory sales tests; not a Cortex-IA constraint |
| Active rule total | `cortex_get_rules` | **3** (prior profile recorded 0) |
| Project contract | `D:/cortex-ia/AGENTS.md` | OpenCode-only scope, native-only execution, SQLite authority, current workspace, security, testing and lifecycle boundaries |
| Workflow map | `internal/assets/skills/_shared/workflow-map.md` | Canonical route and SDD phase/artifact matrix |
| Work protocol | `internal/assets/skills/_shared/cortex-work-protocol.md` | Role authority, claims/leases, delegation, workload, receipts and closure |
| Evidence convention | `internal/assets/skills/_shared/cortex-convention.md` | Hybrid contract representation, pins, evidence taxonomy and recovery |
| Design contract | `internal/assets/skills/_shared/codebase-design-contract.md` | Module/interface/seam/adapter vocabulary and task graph boundaries |

## Architecture and patterns

### Modules and interfaces

- `cmd/cortex-ia/main.go` only sets the release version and calls `internal/app`; `internal/app/app.go` separates the Bubble Tea TUI from hand-written CLI dispatch (`AGENTS.md:67`). Dispatched commands (`internal/app/app.go:47-108`): `install`, `sync`, `snapshot`, `work`, `worktree`, `board`, `ledger`, `ui`, `openspec`, `web`, `doc`, `diagram`, `mcp`, `report`, `doctor`, `rollback`, `recover`, `uninstall`, `update`/`upgrade`, `version`, `help`.
- `internal/install` is the service facade for install/sync/doctor/rollback/uninstall/MCP, collaborating with `internal/pipeline`, `internal/backup`, `internal/state`, `internal/installmeta`, `internal/mcpmanager` (`AGENTS.md:68-72`).
- `internal/pipeline` plans then applies the asset copy transactionally with verified backup and restore-on-failure (`AGENTS.md:69`).
- `internal/delegation` owns the SQLite schema/migrations, task boards, DAG state, claims, TTL file leases, approvals, recovery and structured receipts, plus the **legacy read-only delegation-job projection** served to the web console (`AGENTS.md:73`; `internal/cortexiaweb/server.go:65,265-284` expose `getDelegation`, `delegation.Job`, `delegation.Receipt`).
- `internal/herdr` retains optional Herdr setup/detection helpers for diagnostics only (`AGENTS.md:74`).
- `internal/cortexiaweb` serves the Vite-built, embedded Preact console over a loopback-only HTTP/API server; it may create boards/tasks but exposes no claim/lease/transition/recovery/approval mutations (`AGENTS.md:75`).
- `internal/agents/opencode` declares the native layout and pure asset mapping, failing closed on unsafe/off-surface/colliding destinations (`AGENTS.md:76`).
- `internal/components/filemerge` owns JSONC decode/merge and atomic writes (`AGENTS.md:77`).

### Retired surface: external AGY execution path

The external AGY execution path is **removed and retired fail-closed**. Doctrine: "The external AGY execution path is removed: no external CLI receives a Cortex session lifecycle, `cortex-ia work` claim/lease tokens, approval authority, or nested-delegation capability" (`AGENTS.md:6`; see also `AGENTS.md:43,122`).

| Surface | Current state | Evidence |
|---|---|---|
| AGY execution engine | Removed. `internal/delegation/runner.go` is reduced to a bare `package delegation` clause; `internal/delegation/execution_environment.go` deleted; AGY worker completion/cancellation wrappers removed | `internal/delegation/runner.go`; `git status --short` |
| AGY install target | Rejected. `ParseTargets` accepts only `opencode`, `claude`, `all`; `internal/targets/agy.go` deleted | `internal/targets/targets.go:29-53`; `internal/targets/targets_test.go:21-22` (agy causes an error) |
| CLI detection | Only `opencode` and `claude` are detected | `internal/clidetect/detector.go:18-19,35-45` |
| Hook surface | `internal/app/hook.go` deleted; `hook` is a retired command | `git status --short`; `internal/app/app.go:243` |
| Retired commands | `detect`, `verify`, `repair`, `config`, `list`, `init`, `skill`, `skill-registry`, `memory`, `agent-builder`, `auto-install`, `profiles`, `profile`, `delegate`, `herdr`, `hook` fail closed via `RetiredSurfaceError` | `internal/app/app.go:113-116,227-244,259-270` |
| Retired flags | `--agent`, `--persona`, `--profile`, `--model`, `--sdd` rejected in preflight; scanning stops at `--` | `internal/app/app.go:251-257,276-289` |
| Repository state | No `RunWorker`, `ResolveAGY` or `antigravity` symbol remains in any Go file | grep across `*.go` returns 0 matches |
| Residual legacy tokens | `agy` survives only as a tolerated legacy **config value** for the read-only config display and v1-to-v2 migration path (`RoleConfig.CLI` comment, `Validate` acceptance, `Load` migration branch). No execution engine consumes it; all default roles are `native` | `internal/delegation/config.go:24,54-63,90-98,231` |
| Legacy job projection | Retained read-only by design (no destructive purge of AGY job history), covering `Job`, `Receipt`, `Get`, `List` for the web console | `openspec/changes/agent-flow-hardening/plan.md:9,27`; `internal/cortexiaweb/server.go:265-284` |

Removal tracked on board `agent-flow-hardening`: `T-AGY-1,2,6a,6b,7,8,9,10,12,13,3M,3V,4,5,14,15,16,11` are **done**; `T-AGY-6` and `T-AGY-17` are **superseded**.

### Seams and adapters

- Filesystem/home seam: `internal/pipeline`, `internal/backup`, `internal/state`, `internal/agents/opencode`.
- SQLite authority seam: `internal/delegation` and its transactional store (`BEGIN IMMEDIATE` for multi-step authority transitions; fail closed on unknown future schema versions) (`AGENTS.md:73`).
- **Execution seam: native-only.** There is no process/transport runner and no external execution leaf; role controllers execute natively under claim plus per-file leases. The former "AGY runner in `internal/delegation`" seam no longer exists.
- Diagnostics-only seam: `internal/herdr`, whose sole production consumer is the web console status display; it is never an authority or transport substitute.
- HTTP/telemetry seam: `internal/telemetry`, `cmd/cortex-report-hub`, and plugin bridges under `internal/assets/plugins/`.
- Embedded static seam: `internal/cortexiaweb/server.go` plus generated `web/src` output.
- TUI plugin seam: `internal/tuiassets/cortex-ia-tui.tsx` and generated `internal/assets/tui/cortex-ia-tui.js`.
- Workspace seam: `current_workspace` is the only supported implementation strategy; `isolated_worktree` fails closed. `cortex-ia worktree` keeps read-only `list` and `validate`; `create`, `clean`, `drop`, `delete`, `remove`, `prune` are retired (`internal/app/worktree.go:17,65-66`).

### Dependency direction and risks

- High-coupling hotspots reported by the AST graph: `internal/install/service.go:New` degree 166; `internal/delegation/store.go:Store` degree 165; `internal/tuiassets/cortex-ia-tui.tsx` degree 148; `internal/tui/model.go:model` degree 130; `internal/assets/plugins/cortex-subagent-transport.ts` degree 130; `internal/delegation/store.go:Close` degree 89. Observed hotspots, not redesign directives.
- AST index size: 3,072 symbols, 11,743 relations, 276 files (`cortex_analyze_architecture`, project `cortex-ia`).
- **AST staleness caveat:** cycle detection still reports 10 cycles whose file set includes `internal/delegation/runner.go` (now a bare package clause) and a `Request`/`Validate` chain that no longer exists. The Cortex code graph therefore predates the AGY-removal wave; cycle and coupling evidence must be treated as stale until a reviewer performs delta re-ingestion. Discovery did not ingest (not authorized).
- For the next phase, preserve SQLite authority, native-only execution, current-workspace exclusivity, explicit recovery, and existing module ownership. New seams belong at real filesystem/process/protocol variation boundaries per the design contract.

## Domain vocabulary and decision records

- Canonical terms: OpenCode native assets, Cortex contract, native-only execution (no external leaf), Herdr diagnostics-only, board, DAG/work item, claim, TTL file lease, approval, receipt, install/sync/rollback/recovery, managed MCP, ownership fingerprint, current workspace, telemetry, handoff, observation snapshot.
- **Retired vocabulary** (must not be presented as canonical): "AGY execution leaf", "AGY runner", "Herdr transport/delegation", "isolated worktree".
- Decision/context records: `AGENTS.md`, `docs/architecture.md`, `docs/CODEBASE-GUIDE.md`, `docs/codebase/*.md`, `SPEC.md`, `docs/sdd-workflow.md`, `CHANGELOG.md`, and `openspec/changes/agent-flow-hardening/` (proposal, plan, design, tasks; specs for agy-execution and work-authority).
- In-flight (not landed) work on board `agent-flow-hardening`: `T-WL-1` (workload_policy storage and creation plumbing, additive migration v15, status `ready`) and `T-WL-2` (strict workload-budget enforcement in work transition, status `backlog`). Recorded here as in-flight, not as implemented capability.
- Cortex rule `[132]` is the only Cortex-IA-scoped directive; rules `[125]` and `[105]` belong to an unrelated project.

## Development guardrails

- No secrets, key activation, release publishing, active installation sync or deployment are authorized by this discovery dispatch.
- Preserve OpenCode-only scope; do not restore retired adapters/personas/model routing/external task-board or ForgeSpec surfaces (`AGENTS.md:8-9`).
- **Do not reintroduce an external execution leaf or an AGY surface.** AGY is retired fail-closed; `isolated_worktree` requests fail closed (`AGENTS.md:6,43,122`).
- Native parallel writers may share the workspace only with distinct live claims and per-file `cortex_ia_file_reserve` leases acquired in deterministic sorted order, released on every outcome, stopping on conflict or expiry (`AGENTS.md:46`).
- Claims, leases, transitions and approvals remain SQLite authority; only an independent reviewer `work approve --verdict PASS` produces `done` (`AGENTS.md:28-29`).
- Hybrid SDD uses OpenSpec artifacts plus Cortex evidence; the planner validates phase artifacts, binds requirements to tasks, materializes one stable board/DAG, and archives only after approvals.
- Persistent regression tests are bounded and modular (dedicated files up to 250 LOC; never append to a file above 300 LOC), use temporary homes and synthetic inputs, and must never weaken contracts by skipping invalid records (`AGENTS.md:61-62`).

## Canonical verification commands

Discovery did not build, test, ingest, install, start services, connect to databases, or mutate product files. Commands from checked-in guidance:

```text
gofmt -s -w .
go vet ./...
golangci-lint run ./...
go test -count=1 ./...
go test ./internal/tui/...
go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1
go test ./internal/delegation -count=1
go test ./internal/targets -count=1
go test ./internal/clidetect -count=1
go test ./internal/herdr -count=1
npm --prefix internal/tuiassets run build
npm --prefix web run build
go build -o bin/cortex-ia ./cmd/cortex-ia
```

For TUI source changes, generate assets before the Go build. For web source changes, build web assets before the Go build and retain generated `internal/cortexiaweb/static/**` (`AGENTS.md:16`).

## Unknowns and blockers

- The Cortex code graph is stale relative to the working tree (AGY removal not re-ingested); the cycle and coupling numbers above are not current architectural truth.
- `pnpm`, TypeScript/tsup/`node_modules` presence, Vite executability, and OpenCode runtime behavior were not probed.
- Herdr binary installation state and telemetry endpoint behavior were not probed.
- No Cortex skill-list result is exposed; Cortex skill count remains unknown.
- Exact OpenSpec archive state for `agent-flow-hardening` is unknown (change directory present and unarchived; `T-WL-1` and `T-WL-2` remain non-done).
- No runtime reproduction, performance measurement or exhaustive comparative diagnosis was performed.
- No claim is made about release key possession/activation, secret-derived values, or external update endpoint state.
- No secrets, private keys, credentials, full configuration, transcripts or external services were inspected.

## Evidence index

- `D:/cortex-ia/go.mod:1-10` — Go module, target and SQLite/runtime dependencies.
- `D:/cortex-ia/AGENTS.md:3-9,13-17,21-31,34-48,55-63,65-78,119-125,127-133` — project contract, native-only execution, tooling, authority, architecture, security, verification.
- `D:/cortex-ia/internal/assets/AGENTS.md` — installed harness contract (roles, workload policy, lifecycle).
- `D:/cortex-ia/internal/app/app.go:47-108,113-116,225-257,259-270,276-289` — dispatched commands, retired commands/flags, `RetiredSurfaceError`.
- `D:/cortex-ia/internal/app/worktree.go:11-19,65-70` — read-only worktree inspection, retired mutating subcommands.
- `D:/cortex-ia/internal/targets/targets.go:11-53` and `internal/targets/targets_test.go:21-22` — valid targets, agy rejected.
- `D:/cortex-ia/internal/clidetect/detector.go:14-45,86-118` — opencode/claude detection only.
- `D:/cortex-ia/internal/delegation/runner.go` — reduced to `package delegation` (engine removed).
- `D:/cortex-ia/internal/delegation/config.go:20-29,54-63,86-101,216-245` — residual legacy `agy` config value tolerated for read-only display/migration; native defaults.
- `D:/cortex-ia/internal/herdr/setup.go:30-136` and `internal/cortexiaweb/server.go:253-254` — diagnostics-only Herdr with a single production consumer.
- `D:/cortex-ia/internal/cortexiaweb/server.go:26,45,65,235-284` — read-only delegation-job projection and board/task creation surface.
- Embedded skills `internal/assets/skills/*/SKILL.md` (17), role agents `internal/assets/agents/*.md` (6), repo-local skills `.agents/skills/*/SKILL.md` (4), and `.cortex-ia/skill-registry.md`.
- `D:/cortex-ia/openspec/changes/agent-flow-hardening/` — proposal, plan, design, tasks, and agy-execution / work-authority specs (GAP-03/05/06 contracts and reconciliation notes).
- Cortex results: `cortex_get_status`, `cortex_get_rules` (3 rules), `cortex_analyze_architecture` (3,072 symbols / 11,743 relations / 276 files), `cortex_detect_cycles` (10 stale cycles), `cortex_ia_board_list`, `cortex_ia_board_status(agent-flow-hardening)`, `cortex_ia_work_list`.
- Shell probes: `git rev-parse HEAD`, `git status --short`, `git log --oneline -5`, `go version`, `node --version`, `npm --version`, `git --version`, `golangci-lint --version`.

Profile path: `D:/cortex-ia/.cortex-ia/discovery.md`. This refresh wrote only the discovery profile and mutated no work state.
