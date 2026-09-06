# Cortex-IA Project Discovery

> Generated: 2026-09-05 (UTC date; exact clock not probed) · Repository revision: observed from repository state was not safely re-probed in this refresh

Evidence-backed refresh for authorized follow-up after audit observations #44/#45. This profile is a cache, not authority. **Declared** means manifest/checked-in contract; **Observed** means repository/Cortex/tool output; **Inferred** means source-supported reasoning; **Unknown** means not verified. No product files, configuration, dependencies, builds, tests, services, databases, ingestion, delegation, session lifecycle, boards, or tasks were changed by discovery. The only write is this profile.

## Project identity

- **Observed:** repository root is `/home/lleon/Projects/personal/lleontor705/cortex-ia`; module path is `github.com/lleontor705/cortex-ia` from `go.mod`.
- **Declared:** user alignment is `auto`, `spec_plane=cortex`, `workspace_strategy=isolated_worktree`; existing session is `manual-save-cortex-ia`; no lifecycle, board, or task mutation is authorized for this discovery.
- **Observed:** Cortex is local SQLite, version `2.0.0`, with numeric local observation/graph identifiers and AST, graph, rules, hybrid-search, and handoff capabilities. The requested project key `cortex-ia` resolves without ambiguity through Cortex context.
- **Observed:** recent durable audit context includes observations #44 (discovery) and #45 (bugfix), plus #46 session summary. Existing Cortex board/task state is evidence only and was not modified; current board/task authority remains outside this profile.

## Installed skills

### Filesystem skills

**Observed:** no project-local `.agents/skills/*/SKILL.md` files matched. User-level filesystem skills are installed under `/home/lleon/.agents/skills/`; the effective discovery inventory supplied to this role lists 20 available skills, including built-in `customize-opencode` separately. The project embeds runtime skill assets under `internal/assets/skills/`.

| Scope/source | Skill availability | Purpose relevant to future work |
|---|---|---|
| User filesystem | discovery | Profile generation; used for this refresh |
| User filesystem | orchestrator | Routing and authority reconciliation |
| User filesystem | investigate | Read-only diagnosis/audit |
| User filesystem | planner | Cortex/OpenSpec planning and task DAGs |
| User filesystem | implement | One claimed bounded implementation task |
| User filesystem | reviewer | Independent verification/approval via reviewer assets |
| User filesystem | code-review-adversary | Adversarial review |
| User filesystem | ast-impact-analysis | Dependency and targeted-oracle mapping |
| User filesystem | context-distiller | Bounded evidence extraction |
| User filesystem | fast-tdd | Local RED/GREEN/refactor loop |
| User filesystem | grill-me | Resolve material design ambiguity |
| User filesystem | hotfix-triage | Incident containment |
| User filesystem | mutation-testing | Test oracle sensitivity |
| User filesystem | property-based-testing | Invariant-driven tests |
| User filesystem | spike-prototype | Disposable uncertainty experiment |
| User filesystem | workflow-retrospective | Workflow failure analysis |
| User filesystem | using-git-worktrees | Isolation guidance |
| User filesystem | find-skills | Skill discovery; not used to install anything |
| User filesystem | omarchy | Unrelated desktop customization |
| Built-in | customize-opencode | OpenCode configuration only; not applicable to product correction |

**Observed project runtime assets:** `internal/assets/skills/{orchestrator,planner,implement,investigate,code-review-adversary,fast-tdd,ast-impact-analysis,context-distiller,mutation-testing,property-based-testing,spike-prototype,hotfix-triage,workflow-retrospective,grill-me,discovery}/SKILL.md`, plus shared contracts under `internal/assets/skills/_shared/`. [AGENTS.md; internal/assets/]

### Cortex skills

- **Unknown:** no Cortex skill-list tool is exposed in the active tool schema. The Cortex catalog cannot be counted or qualified; this is not evidence that the catalog is empty.

## Languages and project types

- **Declared:** primary Go CLI/TUI/control-plane module, Go `1.26.1`, module `github.com/lleontor705/cortex-ia` (`go.mod:1-11`).
- **Observed:** Go source exists under `cmd/` and `internal/`; entrypoints include `cmd/cortex-ia/main.go` and `cmd/cortex-report-hub/main.go`.
- **Declared:** isolated Preact/Vite web frontend in `web/`, with Preact `^10.27.2` and Vite `^7.1.5` (`web/package.json`).
- **Declared:** root `package.json` is Husky-only and its `npm test` intentionally exits with failure; it is not the application test runner.
- **Declared/observed:** `internal/tuiassets/` is a separate TypeScript/tsup asset toolchain; embedded/runtime assets live under `internal/assets/` and generated web output is embedded by Go according to `AGENTS.md`.
- **Observed:** Markdown contracts, agent prompts, JSON/config assets, shell E2E scripts, Dockerfiles, and GitHub Actions support the Go product; no separate Python/Rust/Java application manifest was evidenced.

## Required engines and developer tooling

| Capability | Requirement | Local state / verification status | Evidence |
|---|---|---|---|
| Go | Required exact `1.26.1` | **Required; availability not verified in this refresh** | `go.mod:3`; `AGENTS.md:13` |
| Go compatible with CI | CI declares Go `1.26` | **Declared CI requirement; local availability unknown** | `.github/workflows/ci.yml:12-16` |
| Node/npm | Required only for web asset changes and Husky workflows | **Available presence not verified in this refresh** | `package.json`; `web/package.json`; `AGENTS.md:14-16` |
| Vite/Preact | Required when `web/src/` changes | **Project dependency declared; local executable qualification unknown** | `web/package.json` |
| TypeScript/tsup | Required only for `internal/tuiassets/` asset build | **Project dependency declared; local executable qualification unknown** | `internal/tuiassets/package.json` |
| golangci-lint | Required declared local quality gate | **Availability unknown** | `.golangci.yml`; `AGENTS.md:60` |
| govulncheck | Optional/security tooling; not manifested as a canonical local gate | **Availability unknown** | project guidance/CI security job references |
| jq | Optional bounded JSON projection/inspection | **Availability unknown** | prior profile recommendation; not required by `go.mod` |
| OpenCode | Required native host for installed agent assets | **Installed host/skill context observed; executable version unknown** | supplied environment and `internal/assets/` |
| AGY/Herdr | Conditional external execution/transport only | **Not exercised; no delegation allowed in this discovery** | `AGENTS.md`; `cortex-work-protocol.md` |
| Docker | Optional E2E/container tooling | **Project Dockerfiles exist; executable availability unknown** | `Dockerfile`; `e2e/Dockerfile.*` |

**Availability versus verification:** manifests establish requirements and checked-in assets establish intended runners. Discovery did not execute builds/tests, did not restore dependencies, and did not qualify tool versions. Any future PASS must provide executable command, exit code, revision/hash, and bounded result; inspection alone is not verification.

## Data and infrastructure dependencies

- **Declared:** `modernc.org/sqlite` is embedded and avoids a database daemon requirement for normal local operation (`go.mod`; `AGENTS.md`).
- **Declared:** Cortex-IA authority state is normally local SQLite under `~/.cortex-ia/delegation.db`; claims, leases, task transitions, approvals, and operational events belong to Cortex-IA, not Cortex memory or boards (`AGENTS.md`; `cortex-work-protocol.md`).
- **Declared:** local filesystem and process execution are real resource seams; AGY uses argv without a shell, bounded output, temporary home, timeouts, and explicit workspace strategy (`AGENTS.md`; `cortex-work-protocol.md`).
- **Declared:** embedded operations web server is loopback-only, preserves CSP/request limits/timeouts, and must not bypass authority (`AGENTS.md`).
- **Unknown:** no current database, service, live deployment target, effective external runner, or live configuration was connected or qualified.

## Cortex governance map

**Observed Cortex result:** `cortex_get_rules(project="cortex-ia")` returned zero active project/global rules. No stable Cortex rule IDs/titles/scopes were returned. Repository and installed contract directives remain applicable evidence, but are not Cortex rule records.

| Stable ID/title | Scope/source | Applicability |
|---|---|---|
| None returned | Cortex local rules | No active Cortex rule records observed |
| `cortex-work-protocol.md` v3.0 | Installed shared contract | Normative authority, delegation, claims/leases, completion, bootstrap and workspace strategy |
| `cortex-convention.md` | Installed shared contract | Normative Cortex-only specification snapshots, pins, retrieval, lineage and sessions |
| `codebase-design-contract.md` | Installed shared contract | Shared module/interface/seam/adapter vocabulary; discovery observes, does not redesign |
| Repository `AGENTS.md` | Project directive | Go version, architecture, testing policy, security, product scope, installed asset constraints |
| Harness `AGENTS.md` | Global role directive | Discovery boundary, no lifecycle/delegation, artifact ownership and receipt dimensions |

## Architecture and patterns

### Modules and interfaces

- **Observed entry/composition root:** `cmd/cortex-ia/main.go` delegates to `internal/app`; `internal/app` owns handwritten CLI/TUI dispatch and receipt rendering, not installation/ownership logic (`AGENTS.md`; `cmd/cortex-ia/main.go`; `internal/app/`).
- **Observed service facade:** `internal/install` owns install/sync/doctor/rollback/uninstall/MCP operations; `internal/pipeline` plans/applies transactional asset copies; `internal/backup` owns snapshots/manifests/verification/retention; `internal/mcpmanager` owns managed MCP catalog and qualification; `internal/state` and `internal/installmeta` own metadata/locks/digests (`AGENTS.md`; matching source directories).
- **Observed authority module:** `internal/delegation` contains SQLite schema/migrations, jobs, boards, DAG work, claims, TTL leases, approvals, recovery, and receipts (`AGENTS.md`; `internal/delegation/*.go`).
- **Observed transport/adapter modules:** `internal/herdr` owns optional Herdr setup/pane transport; `internal/cortexiaweb` owns loopback operations console; `internal/agents/opencode` owns layout and asset mapping; `internal/components/filemerge` owns JSONC/TOML merge and atomic writes.
- **Observed interfaces:** `internal/tui/tui.go` declares `ServiceAPI`; delegation types include `Store`, `WorkItem`, `WorkDefinition`, `WorkClaim`, `WorkLease`, and `WorkApproval`; install and pipeline APIs are exposed through concrete service structures and functions (`internal/tui/tui.go`; `internal/delegation/*.go`).
- **Cortex indexed evidence:** 1,283 symbols across 116 files and 4,266 relations are indexed for project `cortex-ia`; current filtered symbol query returned `cmd/cortex-ia/main.go` and report-hub symbols.

### Seams and adapters

- **Filesystem seam:** configuration, install destinations, backups, state metadata, and OpenCode asset roots are local substitutable resources; acquisition/replacement must occur at narrow boundaries (`internal/state`, `internal/install`, `internal/agents/opencode`, `codebase-design-contract.md`).
- **Process seam:** AGY/Herdr and report-hub processes are external process boundaries; use argv/temporary homes/bounded receipts, never shell interpolation (`AGENTS.md`; `cortex-work-protocol.md`).
- **Protocol adapter:** typed OpenCode bridge calls translate to local CLI/process operations; transport presentation never owns task authority (`AGENTS.md`; installed agent assets/contracts).
- **Persistence seam:** SQLite store owns operational authority; Cortex observations provide evidence/specification in `spec_plane=cortex` but cannot claim, lease, transition, or approve (`cortex-work-protocol.md`; `cortex-convention.md`).
- **Design-contract interpretation:** prefer deep modules and real seams; do not introduce interfaces solely to mock internal code or speculative adapters (`codebase-design-contract.md`).

### Dependency direction and risks

- **Declared direction:** CLI dispatch → service facades → pipeline/backup/state/MCP collaborators; delegation owns task authority; Herdr is transport only; web is an observational operations surface and may create boards/tasks but not mutate authority operations (`AGENTS.md`).
- **Observed graph risks:** Cortex architecture analysis reports `Store` in `internal/delegation/store.go` as the highest-degree node (degree 109, score 137), followed by `New` and `Service` in `internal/install/service.go`, and TUI `model`. These are coupling hotspots, not automatically defects.
- **Observed cycle detector result:** 10 symbol/call cycles, including `Service`↔`backupsRoot`/`PendingJournals`, longer `Service`/pipeline journal/planning paths, and `Preset`↔`RemoteURL`. The tool reports symbol/call cycles; they are not independently classified here as import cycles or authorization to redesign.
- **Observed community evidence:** indexed communities include delegation, tui, pipeline, install, state, backup, app, cortex-ia-tui, and filemerge. Cohesion scores reported by Cortex are low-to-moderate (~0.047–0.065 for sampled large communities); use as risk evidence only.
- **Inferred (medium):** future audit corrections should preserve authority boundaries around `internal/delegation.Store`, `internal/install.Service`, and `internal/app` rather than broadening interfaces or moving ownership without an explicit contract decision.

## Domain vocabulary and decision records

- **Declared canonical terms:** `native`, `direct_cli`, `herdr_multiplexed`, `spec_plane`, `isolated_worktree`, `current_workspace`, board, DAG task, claim, lease, revision/CAS, in-review, independent approval, PASS/FAIL/BLOCKED/INCONCLUSIVE, pinned Cortex snapshot, transport/project identity, observation ID, exact UTF-8 SHA-256 digest (`cortex-work-protocol.md`; `cortex-convention.md`).
- **Declared spec-plane rule:** with `spec_plane=cortex`, Cortex observations are the authoritative specification plane; OpenSpec files/tools are neither written nor required for Cortex phases. A full snapshot must include requirements, three Given/When/Then cases, design/interfaces, acceptance/oracles, risks/non-goals, and task traceability (`cortex-convention.md:39-46`).
- **Declared pin rule:** retrieve the full observation before validation; pin real transport/project identity, observation ID, exact content, and SHA-256 of exact UTF-8 content; missing/truncated/drifted content fails closed; new contract content requires a new snapshot/pin and fresh review (`cortex-convention.md:47-53`).
- **Declared audit context:** observations #44/#45 are the durable starting context for authorized corrections; their full contents were not reproduced here, but recent Cortex context labels them discovery and bugfix observations. Orchestrator should retrieve them with `cortex_get_observation` before planning if their exact findings are required.
- **Repository decision records:** current OpenSpec draft `openspec/changes/delegation-cortex-recovery/` contains proposal/design/tasks/specs, but under the user-selected `spec_plane=cortex` it is reference evidence, not normative contract authority. No OpenSpec validation was run.

## Development guardrails

- **Role boundaries:** orchestrator routes and reconciles but never claims, leases, edits, or approves; planner owns SDD DAG/decomposition; implementer owns one live claim and file reservations; reviewer independently verifies and approves; external leaves have no Cortex/session/task authority (`cortex-work-protocol.md:17-32`).
- **Bootstrap restriction:** only explicit user authorization permits one bounded `direct-change` task creation by orchestrator, after duplicate check and complete objective/files/effects/checks/review definition. It does not authorize SDD DAG creation, decomposition, claims, leases, edits, or approval; SDD DAG creation/decomposition remains planner-only (`cortex-work-protocol.md:30-32`).
- **Workspace:** user selected `isolated_worktree`; before external implementation, use an existing clean related worktree or provision one under the controller protocol. Do not infer strategy from Git/Herdr/config. Existing worktree cleanliness, relationship, accreditation and source equivalence remain **unknown/not verified** by discovery.
- **Authority:** task readiness comes from `cortex_work_status`; board position/chat/receipts/tests do not prove readiness or completion. Only independent reviewer PASS produces done (`cortex-work-protocol.md:52-70`).
- **No token leakage:** claims/lease tokens remain live process memory and must not appear in prompts, argv, receipts, logs, files, Cortex observations or chat (`cortex-work-protocol.md:34-50`).
- **Testing policy:** persistent tests are limited to TUI and simple pipeline install/copy checks; deeper SQLite/delegation/CLI/web transactional or process oracles are ephemeral and removed after execution (`AGENTS.md:58-66`). Use temporary homes and never real developer OpenCode/Cortex state.
- **Generated/embedded assets:** changes under `internal/assets/` require rebuilding the binary; web source changes require `npm --prefix web run build` and committing generated output before Go build (`AGENTS.md`).
- **Source restrictions:** do not restore retired ForgeSpec/external board MCP/platform adapters/personas/model routing/SDD compiler surfaces; preserve fail-closed retired commands and OpenCode-only scope (`AGENTS.md`).
- **PR/repository policy:** branch naming is `<type>/<lowercase-name>`; PR body must link an approved issue with `Closes/Fixes/Resolves #N`; exactly one `type:*` label; Conventional Commit first line 10–72 chars (`AGENTS.md`). These are future repository workflow constraints, not verification results.
- **Audit #44/#45 implication:** future implementation must not equate static inspection or external receipts with PASS, must keep runner authority and telemetry/integrity concerns bounded to their authorized scope, and must require independent current-revision verification. Exact remediation scope remains to be recovered from full observations #44/#45.

## Canonical verification commands

Recommendations from checked-in guidance only; discovery did not execute them.

| Command | Purpose / condition |
|---|---|
| `gofmt -s -w .` | Declared formatting gate; writes source and therefore not a discovery command |
| `go vet ./...` | Declared Go static gate |
| `golangci-lint run ./...` | Declared lint gate; availability/version unknown |
| `go test -count=1 ./...` | Declared full regression gate; persistent-test policy limits what new tests may be added |
| `go test ./internal/tui/...` | Focused persistent TUI oracle |
| `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1` | Focused simple install/copy oracle; exact test presence must be checked before relying on it |
| `go build -o bin/cortex-ia ./cmd/cortex-ia` | Product build; required after embedded asset changes, not run here |
| `npm --prefix web run build` | Frontend asset generation when `web/src/` changes; writes generated output |
| `npm test` | Intentionally fails; do not use as application test runner (`package.json:9-11`) |
| `git diff --check` | Recommended diff hygiene before review; not run in discovery due tool boundary |
| `git worktree list --porcelain` / `git status --porcelain=v1` | Required future isolation/baseline evidence; not verified in this refresh |

## Unknowns and blockers

1. **Exact source state is not fully verified:** current revision, branch, dirty/clean status, untracked files, and pre-existing diffs require a future authorized Git read before implementation. This is a readiness gap, not a product defect.
2. **Related isolated worktree availability/cleanliness is unknown:** no authoritative current worktree listing was obtained in this refresh. Orchestrator must not infer that a clean related worktree exists.
3. **Required Go `1.26.1` and local runner availability are unknown:** `go.mod` declares the version, but discovery did not verify local executables. Do not substitute a different Go version or claim readiness.
4. **Lint/security/frontend executable qualification is unknown:** `golangci-lint`, `govulncheck`, Node/npm/Vite/tsup, AGY, Herdr, and OpenCode versions were not verified.
5. **Cortex contract content for authorized corrections is absent from this profile:** planner must retrieve full observations #44/#45 if their requirements/risks are in scope; previews/labels are insufficient for pinning.
6. **Current work/board status is not authority for this discovery:** existing board/task rows were read as evidence only; no boards/tasks were created or changed. The user explicitly requested no board/task operations.
7. **No verification result exists:** no build, tests, lint, static analysis command, installation, service, database, delegation, or E2E execution was performed.
8. **Potentially normative blockage:** the installed contracts require Cortex-only pinned snapshot contracts with full content/digest/traceability before planning/review; they also require one stable session and board for an initiative, while this user request explicitly forbids lifecycle/boards/tasks. Therefore future correction planning can proceed only as read-only evidence recovery or after the orchestrator reconciles the initiative-authority conflict; discovery cannot resolve it.
9. **Telemetry/reporting contract tension:** `cortex-work-protocol.md` declares signed error reports sent to a centralized Railway hub, while repository guardrails emphasize local bounded evidence and the user forbids delegation/external operations here. No report was emitted; future orchestrator must establish applicable authority before any reporting side effect.

## Evidence index

- **E1:** `/home/lleon/Projects/personal/lleontor705/cortex-ia/go.mod`, `package.json`, `web/package.json`, `internal/tuiassets/package.json`.
- **E2:** `/home/lleon/Projects/personal/lleontor705/cortex-ia/AGENTS.md`, especially toolchain, architecture, testing, security and workflow sections.
- **E3:** `/home/lleon/Projects/personal/lleontor705/cortex-ia/.github/workflows/ci.yml`, `.github/workflows/pr-check.yml`, `.golangci.yml`.
- **E4:** `/home/lleon/Projects/personal/lleontor705/cortex-ia/internal/` module directories and Go entrypoints; `/internal/tui/tui.go`; `/internal/delegation/*.go`.
- **E5:** `/home/lleon/Projects/personal/lleontor705/cortex-ia/internal/assets/` runtime skills/agents/shared contracts; `internal/assets/AGENTS.md`.
- **E6:** Installed `/home/lleon/.cortex-ia/opencode/contracts/cortex-work-protocol.md`, version 3.0.
- **E7:** Installed `/home/lleon/.cortex-ia/opencode/contracts/cortex-convention.md`.
- **E8:** Installed `/home/lleon/.cortex-ia/opencode/contracts/codebase-design-contract.md`.
- **E9:** Cortex `get_status`: local SQLite, version 2.0.0, numeric local IDs and listed capabilities.
- **E10:** Cortex `get_rules(project="cortex-ia")`: zero active rules returned.
- **E11:** Cortex `context(project="cortex-ia")`: active `manual-save-cortex-ia`; recent observations #44, #45, #46; prior ended audit session.
- **E12:** Cortex code-symbol query: indexed Go symbols including `cmd/cortex-ia/main.go` and `cmd/cortex-report-hub/main.go`.
- **E13:** Cortex architecture analysis: 1,283 symbols, 116 files, 4,266 relations; high-degree `internal/delegation.Store`, `internal/install.New/Service`, TUI model; communities and reported symbol/call cycles.
- **E14:** Cortex cycle detection: 10 reported symbol/call cycles; tool output explicitly treated as evidence, not automatic import-cycle proof.
- **E15:** Existing OpenSpec reference artifacts under `openspec/changes/delegation-cortex-recovery/`; not authoritative under selected `spec_plane=cortex`.
- **E16:** User request and active role restrictions: auto, Cortex, isolated worktree; session `manual-save-cortex-ia`; no lifecycle/boards/tasks, no build/test/install/delegation/product edits.

## Readiness summary for orchestrator

- **Readiness:** `partial / blocked for implementation`, because identity and architecture evidence are available, but required Go/tool runners, source/worktree baseline, exact #44/#45 content, and current authority are not verified.
- **Normative blocker:** under `spec_plane=cortex`, recover full observations and create/pin a complete Cortex contract snapshot (requirements, three G/W/T cases, design/interfaces, acceptance/oracles, risks/non-goals, traceability, exact content SHA-256) before planner/reviewer can authorize implementation. Missing/truncated/drifted pin is fail-closed. The user’s no-boards/tasks constraint means orchestrator must not silently create operational authority during this discovery; reconcile that constraint before implementation routing.
- **Bootstrap contract summary:** only explicit user authorization permits one bounded orchestrator direct-change task after duplicate check and complete allowlist/effects/checks/review definition. It never permits orchestrator claim/lease/edit/approve, SDD DAG creation, or decomposition. Planner owns DAG creation/decomposition; implementer claims/reserves; reviewer independently approves.
- **Workspace contract summary:** external implementation requires explicit `isolated_worktree`; controller must verify clean related worktree, same relevant baseline, allowlist, and pre-existing changes. Accepted external failure cannot silently fall back to native execution; reconcile first. Native writers require claims and per-file reservations.
- **Receipt dimensions:** `phase_status` describes discovery execution; `task_status` is not changed or promoted by this role; `verification_verdict` remains `INCONCLUSIVE` because no executable verification ran.
