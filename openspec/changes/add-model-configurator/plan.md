# Model Configurator — integrated plan (sdd-lite)

Change: `add-model-configurator` · Board: `model-configurator` · Spec plane: hybrid · Workload policy: flexible

## Intent

Give cortex-ia users a validated, ownership-tracked, TUI-reachable configurator for OpenCode v2 per-agent models and effort (variant): a top-level `cortex-ia model` command (`list|get|set|unset|doctor`) as a strict sibling of `mcp`, plus a Bubble Tea models screen in the zero-arg TUI, both operating one transactional service path against the global config (`~/.config/opencode/opencode.jsonc`, jsonc-first).

Non-goals: project-scope config writing (deferred by product decision); root default `model` key; embedded provider/model catalogs; web-console mutations; deprecated fields (`temperature`, `top_p`, `prompt`, `permission`, `tools`, `disable`, `maxSteps`) and `request.body` overlays; automatic markdown-frontmatter precedence resolution.

## Requirements

Authoritative deltas with RFC 2119 keywords and Given/When/Then scenarios live in `specs/model-command/spec.md`:

- REQ-MODEL-001 Top-level `model` command surface (subcommands list|get|set|unset|doctor; only `--effort`, `--json`, `--dry-run`; `--model*` prefixes stay retired app-wide)
- REQ-MODEL-002 Free-form model reference shape validation (compact `provider/model#variant` canonical; expanded object accepted on read; no catalog; deprecated fields never written)
- REQ-MODEL-003 Case-folded agent resolution registry (builtins build/plan/general/explore + config `agents` keys + markdown agents; unknown agents fail closed; markdown frontmatter pins surface as warnings and receipt notes)
- REQ-MODEL-004 Transactional set/unset mutation semantics (ErrNotInstalled gate, home lock, serviceTxn with verified backup, `filemerge.MutateJSONFile` only, dry-run, previous-value disclosure, `--json` receipts)
- REQ-MODEL-005 Honest list/get reporting (works on uninstalled homes; source-tagged: managed/config/markdown/unset; sanitized; stable `--json`)
- REQ-MODEL-006 Ownership records, drift detection, and lifecycle integration (additive `AgentModels` state/lock records, `amdv1:` installmeta digest, namespaced `agent-model/<agent>` sidecar records, uninstall/rollback enumeration, embedded-template regression guard)
- REQ-MODEL-007 `model doctor` diagnostics (decode health, duplicate members, drift, shape, `default_agent`, template guard, non-fatal `opencode2 models` cross-check)
- REQ-MODEL-008 TUI models configuration screen (self-contained `modelsState` + `modelsAction` seam; persists exclusively through the install service path)

## Design

Concise architecture (full rationale in `design.md`):

- **Layers mirror `mcp`**: `internal/app` (parse + receipts) → `internal/install` (transactional service: canonical home lock, serviceTxn with verified backup, postimages, namespaced sidecar records) → `internal/modelmgr` (pure manager: Desired validation, case-folded registry, `filemerge.MutateJSONFile` overlay mutations, doctor) with `internal/state` (AgentModelV2 + lock agreement) and `internal/installmeta` (`amdv1:` digest; stdlib-only leaf).
- **Data**: config target `agents.<agent>.model` compact string; `Desired {Agent, Provider, Model, Variant}`; `AgentModelV2 {agent, config_path, semantic_digest, ownership}` mirrors `MCPV2`; fingerprint records namespaced `agent-model/<agent>` in the existing sidecar document (no schema bump).
- **Key decisions**: `set` overwrites with previous-value disclosure (documented deviation from MCP accreditation for a single-scalar, user-intended, reversible change); unknown agents fail closed; markdown pins are doctor WARNINGs, never auto-resolved; no `txn.go` edit (`serviceTxn.commitStateModels` lives in `install/model.go`); TUI uses injected service seams, never ad-hoc writes; embedded `opencode.jsonc` template stays free of `agents`/`model` keys (regression-guarded — installer safe-merge makes template keys win).
- **Interfaces**: `install.Service.ModelSet/ModelUnset/ModelList/ModelGet`; `modelmgr.Manager.{ConfigPath, Resolve, Set, Unset, List, Get}`; `modelmgr` Doctor report over injectable fs/exec seams; `app` `runModel` dispatch with text + `--json` receipts; TUI `modelsState`/`modelsAction` with injectable loader/persistence seams.

## Tasks

DAG on board `model-configurator` (initial ready: model-001; waves 001 → 002 → (003 ∥ 004) → (005 ∥ 007) → 006). Full contracts in `tasks.md`:

- model-001 [state+installmeta] AgentModelV2 record kind + lock agreement + amdv1 digest domain — internal/state/metadata_v2.go, internal/installmeta/modeldigest.go — deps: none
- model-002 [modelmgr] core: Desired validation, case-folded registry, JSONC overlay mutations — internal/modelmgr/model.go, manager.go, manager_test.go — deps: model-001
- model-003 [modelmgr] doctor diagnostics + template guard + opencode2 cross-check seam — internal/modelmgr/doctor.go, doctor_test.go — deps: model-002
- model-004 [install] transactional service wiring + uninstall/rollback enumeration — internal/install/model.go, uninstall.go, rollback.go — deps: model-001, model-002
- model-005 [app] CLI parse/receipts/help text + preflight & template regression tests — internal/app/model.go, app.go, model_test.go — deps: model-003, model-004
- model-006 [docs] AGENTS.md built-in surfaces gain `model` — AGENTS.md — deps: model-005
- model-007 [tui] models screen (modelsState/modelsAction seam, service persistence) — internal/tui/models_screen.go, model.go, models_screen_test.go — deps: model-004

**Verification strategy**: each task gates on its raw `go test` command (see tasks.md); parallel waves keep allowed_files disjoint; before archive the full local gate runs (`gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`) and the reviewer performs AST delta ingestion plus a cycle check before any PASS.
