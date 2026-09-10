# Cortex-IA Project Discovery

> Generated: 2026-09-09T00:00:00Z · Repository revision: `6b6b5350a97c841ef1fe8e0826387d8e3060015f`

## Project identity

| Item | Evidence-backed value |
|---|---|
| Repository | `github.com/lleontor705/cortex-ia` (`D:/cortex-ia/go.mod:1`, `package.json:15`) |
| Root | `D:/cortex-ia` |
| Revision | `6b6b5350a97c841ef1fe8e0826387d8e3060015f` (preserved from prior profile; no git mutation) |
| Candidate Cortex project | `cortex-ia` (dispatch and indexed AST evidence) |
| Cortex | Local SQLite, version `2.0.0`; capabilities include FTS5, graph, AST extraction, rules, architecture analysis and blast radius (`cortex_get_status`) |
| Worktree | Prior profile records existing uncommitted product changes; they were not inspected semantically or modified |

## Installed skills

### Filesystem skills

| Scope | Count / evidence |
|---|---|
| Project embedded | 15 `internal/assets/skills/*/SKILL.md`: `ast-impact-analysis`, `code-review-adversary`, `context-distiller`, `discovery`, `fast-tdd`, `grill-me`, `hotfix-triage`, `implement`, `investigate`, `mutation-testing`, `orchestrator`, `parallel-dispatch`, `planner`, `property-based-testing`, `spike-prototype` |
| User installed | `C:/Users/usrLuisLeon/.agents/skills/discovery/SKILL.md` and `.../investigate/SKILL.md` observed; environment also supplied the same named skill inventory |
| OpenCode native role | `C:/Users/usrLuisLeon/.config/opencode/agents/discovery.md` and `investigate.md` |

### Cortex skills

No separate Cortex skill-list endpoint was available. Active Cortex capabilities were observed through `cortex_get_status`; no Cortex skill count was claimable.

## Languages and project types

| Type | Evidence |
|---|---|
| Go CLI/TUI | `D:/cortex-ia/go.mod:1-10`, `cmd/cortex-ia/main.go`, `internal/app`, `internal/tui` |
| Embedded OpenCode assets | `internal/assets/`, `internal/agents/opencode/`; product assets are embedded with `go:embed` per prior profile/AGENTS.md |
| Preact/Vite web console | `web/package.json`, `web/src/`, `web/vite.config.js`; generated output under `internal/cortexiaweb/static/` |
| SQLite persistence | `modernc.org/sqlite` in `go.mod:10`; `internal/delegation` |
| Root Node metadata | `package.json:9-11,25-27`; Husky only and intentionally failing placeholder test |

## Required engines and developer tooling

| Tool | Requirement | Local observation / status |
|---|---|---|
| Go | Required `1.26.1` (`go.mod:3`, `AGENTS.md:13`) | Executable path was previously observed at `C:/Program Files/Go/bin/go.exe`; exact version not re-probed in this refresh. **Unknown version** |
| Node/npm | Required for frontend source changes; `web/package.json` owns frontend scripts | Volta paths were previously observed; exact versions remain **unknown** |
| Vite | Frontend build dependency declared by `web/package.json` | Declared; executable availability not independently probed |
| Git | Repository metadata/version control | Previously observed path; exact version **unknown** |
| OpenCode installed binary | Actual installed package is `opencode-ai` **1.18.29** at `C:/Users/usrLuisLeon/AppData/Local/Volta/tools/image/packages/opencode-ai/node_modules/opencode-ai/package.json:2-9`; executable is `.../bin/opencode.exe:4` and PATH shim is `C:/Users/usrLuisLeon/AppData/Local/Volta/bin/opencode.cmd:1-2` | Installed package version **Observed: 1.18.29**. `where.exe opencode` returned the Volta shim and `Get-Command opencode` resolved `opencode.cmd`; `opencode --version` and `opencode attach --help` were attempted but denied by the active command permission policy, so their exit/output are **Unavailable**, not inferred |
| OpenCode checkout | `D:/opencode/package.json:1-7` and package manifests | Checkout package versions are **1.18.23** (`packages/plugin/package.json:3-5`, `packages/sdk/js/package.json:3-5`, `packages/cli/package.json:3-6`), distinct from installed binary/package **1.18.29**. No `D:/opencode/VERSION` file was found. Checkout HEAD was not obtained in this bounded pass. |
| OpenCode plugin package | Installed `C:/Users/usrLuisLeon/.config/opencode/node_modules/@opencode-ai/plugin/package.json:3-5` | **1.18.29** |
| OpenCode SDK package | Installed `C:/Users/usrLuisLeon/.config/opencode/node_modules/@opencode-ai/sdk/package.json:3-5` | **1.18.29** |
| AGY / Herdr | Delegation config declares `agy` for investigate (`cortex-delegation.json:18-22`) and Herdr preference (`:3-8`) | Runtime executable availability and consent behavior not tested; no job started |

## Data and infrastructure dependencies

- SQLite is an in-process deterministic dependency via `modernc.org/sqlite` (`go.mod:10`); `internal/delegation` owns durable task authority per `AGENTS.md:35-40,77`.
- Local filesystem and OpenCode configuration are core resources for install, delegation and asset management (`AGENTS.md:69-82`).
- Optional Herdr is a transport adapter and does not own task authority (`AGENTS.md:78`).
- No remote database, container, or external service dependency was evidenced in the inspected manifests. Secrets, full config contents, and user transcripts were not inspected.

## Cortex governance map

| Identifier/title | Scope/source | Applicability |
|---|---|---|
| No active project rules returned | Cortex local, `cortex_get_rules(project=cortex-ia)` | No additional project directives were available |
| `AGENTS.md` project contract | `D:/cortex-ia/AGENTS.md` | OpenCode-only product, current-workspace strategy, SQLite authority, security and verification invariants |
| `codebase-design-contract.md` | `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/codebase-design-contract.md` | Module/interface/seam/adapter vocabulary; discovery must observe, not redesign |
| `cortex-work-protocol.md` | `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/cortex-work-protocol.md` | Role boundaries, native/external execution modes, receipt requirements |
| `workflow-map.md` | `C:/Users/usrLuisLeon/.cortex-ia/opencode/contracts/workflow-map.md` | Discovery route and phase matrix |
| Installed discovery role | `C:/Users/usrLuisLeon/.config/opencode/agents/discovery.md:118-128` | Explicitly native/non-delegating; no ingestion, jobs, lifecycle calls, or product edits |

No rules or product/runtime configuration were changed.

## Architecture and patterns

### Modules and interfaces

- CLI/TUI composition root: `cmd/cortex-ia/main.go` delegates to `internal/app`; `internal/app/app.go` separates Bubble Tea TUI and hand-written CLI dispatch (`AGENTS.md:69-72`).
- Install service facade: `internal/install` owns install/sync/doctor/rollback/uninstall/MCP operations; pipeline, backup, state and MCP manager are collaborators (`AGENTS.md:71-77`).
- Delegation control plane: `internal/delegation` owns SQLite schema/migrations, AGY jobs, boards, DAG, claims, leases, approvals, recovery and receipts (`AGENTS.md:77`).
- Embedded web console: `web/src` -> Vite output -> `internal/cortexiaweb/static`; loopback server boundary is declared in `AGENTS.md:79`.
- Indexed architecture evidence: 2,003 symbols, 7,790 relations, 136 files (`cortex_analyze_architecture(project=cortex-ia)`).

### Seams and adapters

- Filesystem/home seam for install, backup, state and OpenCode layout (`internal/pipeline`, `internal/backup`, `internal/state`, `internal/agents/opencode`).
- Process seam at `internal/delegation` AGY runner; optional Herdr adapter remains transport-only (`AGENTS.md:77-79`).
- HTTP/embedded-static seam at `internal/cortexiaweb`; loopback-only binding and authority-preserving API boundary are declared (`AGENTS.md:79,88`).
- OpenCode SDK source exposes TUI launch options for `project`, `model`, `session`, and `agent`, and server launch resolves a URL (`D:/opencode/packages/sdk/js/src/server.ts:5-20,22-39,102-123`). This establishes declared SDK fields only; it does **not** prove runtime event or consent behavior.

### Dependency direction and risks

- Indexed high-centrality nodes include `internal/delegation/store.go` `Store` (degree 219), `internal/install/service.go` `New` (degree 180), `internal/delegation/work.go` module (degree 162), `internal/tui/model.go` model (degree 107), and `internal/app/work.go` `runWork` (degree 112) (`cortex_analyze_architecture`). These are coupling hotspots, not redesign directives.
- Indexed cycle detection reports 10 call/dependency cycles, concentrated in `internal/install/{doctor.go,service.go}`, `internal/pipeline/{journal.go,engine.go,plan.go}`, and `internal/mcpmanager/{presets.go,qualification.go}` (`cortex_detect_cycles`). Discovery does not classify them as defects.
- Community labels include delegation, tui, app and pipeline; cohesion scores are low in returned architecture evidence. Treat this as graph evidence requiring investigator/planner interpretation.

## Domain vocabulary and decision records

- Canonical terms: OpenCode native assets, Cortex contracts, AGY execution leaf, Herdr transport, board, work item/DAG, claim, TTL lease, approval, receipt, install/sync/rollback/recovery, managed MCP, ownership fingerprint, current workspace (`AGENTS.md`, `docs/agents.md`, `docs/security.md`, `docs/mcp.md`).
- Decision/architecture references: `docs/architecture.md`, `docs/CODEBASE-GUIDE.md`, `docs/codebase/*.md`, `SPEC.md`, `docs/sdd-workflow.md`, `CHANGELOG.md`.
- Relevant durable Cortex evidence: observations `#18`, `#19`, `#20`, `#22`, `#24`, and `#138` were returned by bounded search for OpenCode/delegation/version context. They are evidence pointers, not fresh runtime verification.

## Development guardrails

- Discovery role is native and non-delegating: `C:/Users/usrLuisLeon/.config/opencode/agents/discovery.md:122-128`; its tool surface disables delegation, ingestion, session lifecycle, work mutation and Cortex saves (`:18-80`).
- Investigate role is configured as delegable: `C:/Users/usrLuisLeon/.config/opencode/agents/investigate.md:70-78`; it calls the delegation gate and may supervise one external read-only leaf. `cortex-delegation.json:18-22` sets `delegate=true`, `cli=agy`, `mode=plan`, `skip_permissions=true`; `cortex-delegation.json:3-8` enables delegation and Herdr preference.
- Therefore native read requests can still be sent externally because the installed investigate role explicitly mandates `cortex_ia_delegate_start` and the active role configuration enables delegation. This is configuration evidence only; no consent PASS, runtime event behavior, or job acceptance is inferred.
- The OpenCode harness declares that external leaves are executors, not coordinators, and that returned `execution_mode` is authoritative (`C:/Users/usrLuisLeon/.config/opencode/AGENTS.md:93-112`, `cortex-work-protocol.md:105-117`).
- Do not treat SDK type fields or package manifests as runtime consent/event proof.

## Canonical verification commands

Discovery did not execute builds/tests/services. Recommended commands remain those declared by `D:/cortex-ia/AGENTS.md:61-67`:

```text
gofmt -s -w .
go vet ./...
golangci-lint run ./...
go test -count=1 ./...
go test ./internal/tui/...
go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1
npm --prefix web run build
```

The requested bounded probes were authorized by the installed discovery role (`discovery.md:84-115`), but `opencode --version` and `opencode attach --help` were denied by the active command permission policy in this session; no output or exit code is claimed.

## Unknowns and blockers

- Installed OpenCode binary/package and installed `@opencode-ai/plugin`/SDK are verified as **1.18.29** from package manifests, while the `D:/opencode` checkout plugin/SDK/CLI manifests are **1.18.23**. Checkout Git HEAD and binary command output remain unknown.
- Exact `opencode --version` and `opencode attach --help` exit codes/help excerpt are unavailable because command execution was blocked by the active permission policy. No host/service was started.
- Runtime event routing, consent prompts, session behavior, and actual AGY acceptance were not tested and must not be inferred from SDK types. SDK types show event and permission shapes (`C:/Users/usrLuisLeon/.config/opencode/node_modules/@opencode-ai/sdk/dist/gen/types.gen.d.ts:24-26,379-401,385-392`) only.
- No Cortex skill-list endpoint was available; Cortex skill count is unknown.
- AST index is present and architecture/cycle analysis succeeded; discovery did not trigger ingestion. Indexed data may predate current uncommitted changes.
- No secrets, full user config, other projects' sessions, or transcripts were inspected.

## Evidence index

- `D:/cortex-ia/go.mod:1-10` — module, Go version and dependencies.
- `D:/cortex-ia/package.json:9-27` — root Node/Husky metadata.
- `D:/cortex-ia/AGENTS.md:11-17,33-90` — project toolchain, architecture, authority and security contract.
- `D:/cortex-ia/internal/assets/skills/*/SKILL.md` — embedded skill inventory.
- `C:/Users/usrLuisLeon/.config/opencode/AGENTS.md:84-100` — installed role capabilities and native discovery boundary.
- `C:/Users/usrLuisLeon/.config/opencode/agents/discovery.md:7-128` — discovery permissions and non-delegating instructions.
- `C:/Users/usrLuisLeon/.config/opencode/agents/investigate.md:64-100` — investigate delegation behavior.
- `C:/Users/usrLuisLeon/.config/opencode/cortex-delegation.json:1-31` — active investigate external delegation configuration.
- `C:/Users/usrLuisLeon/.config/opencode/package.json:1-4` — configured plugin version.
- `C:/Users/usrLuisLeon/.config/opencode/node_modules/@opencode-ai/plugin/package.json:3-5,44-49` — installed plugin/SDK versions.
- `C:/Users/usrLuisLeon/.config/opencode/node_modules/@opencode-ai/sdk/package.json:3-5` — installed SDK version.
- `C:/Users/usrLuisLeon/AppData/Local/Volta/tools/image/packages/opencode-ai/node_modules/opencode-ai/package.json:2-9,20-32` — installed binary package/version and platform binaries.
- `C:/Users/usrLuisLeon/AppData/Local/Volta/bin/opencode.cmd:1-2` and `where.exe opencode` output — PATH shim.
- `D:/opencode/package.json:1-7,112-118` — checkout package manager and workspace dependency declarations.
- `D:/opencode/packages/plugin/package.json:3-5` and `packages/sdk/js/package.json:3-5` — checkout version distinction, 1.18.23.
- `D:/opencode/packages/sdk/js/src/server.ts:5-20,22-39,102-123` — declared SDK session/project/URL fields.
- Cortex results: `cortex_get_status`, `cortex_get_rules`, `cortex_get_code_symbols`, `cortex_analyze_architecture`, `cortex_detect_cycles`, bounded `cortex_search`.

## Verified / Unverified evidence refresh

### Verified

- Installed OpenCode package, installed plugin, and installed SDK are all manifest version **1.18.29**.
- `D:/opencode` checkout package manifests for plugin, SDK and CLI are **1.18.23**; this checkout is not the installed binary source by assumption.
- `where.exe opencode` resolves the Volta shim `C:/Users/usrLuisLeon/AppData/Local/Volta/bin/opencode` and `.cmd`; the shim delegates to Volta.
- Investigate remains configured for external execution: role instructions require the delegation gate, and `cortex-delegation.json` has `delegate=true`, `cli=agy`, `mode=plan`, `skip_permissions=true`.
- Discovery itself remains native/non-delegating and has only the single profile write capability.
- SDK declarations expose `session`, `project`, `agent`, URL and permission/event type fields, but these are schema/type evidence only.

### Unverified / blocked

- `opencode --version`: command attempted; active permission policy denied execution, therefore no exit code/output.
- `opencode attach --help`: command attempted; active permission policy denied execution, therefore no exit code or help excerpt.
- Actual runtime event routing, consent behavior, AGY availability/acceptance, Herdr transport state, and OpenCode checkout HEAD.

Profile write target: `D:/cortex-ia/.cortex-ia/discovery.md`. This refresh intentionally changes only the discovery profile and does not change product/runtime/config state.
