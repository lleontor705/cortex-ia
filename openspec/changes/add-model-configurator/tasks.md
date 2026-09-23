# Task DAG

Board: `model-configurator` (this change only). Initial ready group: `model-001`. Waves: `model-001` → `model-002` → (`model-003` ∥ `model-004`) → (`model-005` ∥ `model-007`) → `model-006`. Workload policy `flexible`: source ≤ 700 LOC Go per task, persistent tests ≤ 1200 LOC per task, dedicated test files ≤ 250 LOC each, 1–3 files per task. Deeper transactional matrices run as ephemeral smokes per AGENTS.md testing scope.

- [ ] model-001 Add AgentModelV2 record kind with lock agreement and the installmeta agent-model digest domain
  Requirements: REQ-MODEL-006
  Dependencies: none
  Objective: MetadataV2 and LockV2 gain an additive AgentModels []AgentModelV2 record ({agent, config_path, semantic_digest, ownership} mirroring MCPV2) that Normalize sorts, ValidateV2/ValidateLockV2 canonicalize, NewLockFromMetadataV2 copies, and the state/lock agreement check compares exactly like MCPs, so the two documents can never disagree on a managed agent-model entry. internal/installmeta/modeldigest.go adds the stdlib-only versioned digest: a canonical AgentModelIdentity {agent, provider, model, variant} encoded with fixed field order, domain "cortex-ia/installmeta/agent-model-digest\n", prefix "amdv1:", plus parse/valid helpers mirrored from the MCP digest; the leaf must not import repository packages. Documents lacking the field remain valid (nil normalizes to an empty slice), so no schema bump or migration is needed.
  Acceptance:
    - Normalize sorts AgentModels by agent and nil becomes an empty slice for both metadata and lock documents
    - ValidateV2/ValidateLockV2 reject empty agent names, non-relative config paths, and malformed digest encodings on the new records
    - NewLockFromMetadataV2 copies AgentModels; agreement validation fails closed on state/lock divergence and passes on equality
    - The digest is deterministic, version-prefixed amdv1:, hex64, and the parse/valid helpers reject unknown versions and malformed sums
    - Pre-existing documents without agent models validate unchanged
  Allowed files: internal/state/metadata_v2.go, internal/installmeta/modeldigest.go
  Verification: go test ./internal/state/... ./internal/installmeta/... -count=1

- [ ] model-002 Implement the modelmgr core: Desired validation, case-folded agent registry, and JSONC overlay mutations
  Requirements: REQ-MODEL-002, REQ-MODEL-003, REQ-MODEL-004
  Dependencies: model-001
  Objective: New pure manager package mirroring the mcpmanager doctrine (home-rooted Manager, jsonc-first ConfigPath precedence, typed ConflictError taxonomy, ownership records). Desired {Agent, Provider, Model, Variant} validates the free-form shape (non-empty provider/model tokens without whitespace, '#', or control characters; optional non-empty variant) and encodes the canonical compact string; the expanded {providerID, model, variant} object is accepted on read and re-serialized compactly. The registry resolves agents case-folded across builtins (build, plan, general, explore), config agents keys, and markdown agents under <configRoot>/agents/*.md with a bounded frontmatter scan (model/description lines only). Set/unset/list/get classify through the status taxonomy and fail closed on malformed config (agents not an object, non-string/non-object model values, duplicate members) or unresolvable agents; config mutations go exclusively through filemerge.MutateJSONFile (never MergeJSONObjects), ownership digests come from installmeta, and set discloses the previous effective value. Keep the persistent suite focused (dedicated test file ≤ 250 LOC, t.TempDir homes, synthetic configs); deeper matrices run as ephemeral smokes.
  Acceptance:
    - The shape-validation matrix accepts compact and expanded forms and rejects empty/whitespace/fragment tokens with typed errors before any file access
    - Encode/decode round-trips compactly for both stored forms
    - Registry resolution is case-folded and unknown-agent conflicts list the resolution sources
    - set writes only agents.<agent>.model preserving JSONC comments and all unrelated members; unset removes the model member and prunes an emptied entry cortex-ia created
    - No deprecated field or request overlay is ever emitted; digests are stable across runs
  Allowed files: internal/modelmgr/model.go, internal/modelmgr/manager.go, internal/modelmgr/manager_test.go
  Verification: go test ./internal/modelmgr/... -count=1

- [ ] model-003 Implement modelmgr doctor diagnostics with the template regression check and opencode2 cross-check seam
  Requirements: REQ-MODEL-007, REQ-MODEL-006
  Dependencies: model-002
  Objective: A pure DoctorReport over injectable filesystem and command seams covering: JSONC decode health and duplicate-member detection; shape validation of every agents.*.model value; drift between recorded managed digests and observed values; markdown-frontmatter model pins reported as WARNING findings; default_agent referencing an existing visible primary agent (WARNING when violated); the embedded-template regression check (internal/assets/opencode.jsonc must declare no agents or model keys, because installer safe-merge makes template keys win); and an optional opencode2 models cross-check that skips non-fatally when the binary is unavailable or its output is unparseable. Findings are typed with stable severities so the CLI and TUI render them uniformly; doctor never mutates anything and works honestly on uninstalled homes. Tests use temporary homes and synthetic asset fixtures (dedicated test file ≤ 250 LOC).
  Acceptance:
    - Every check emits a typed finding with severity and never aborts the remaining checks
    - Drift findings name the agent with expected and observed digests
    - opencode2 absence or unparseable output yields a skipped finding, not an error verdict
    - Uninstalled homes produce honest findings without ErrNotInstalled failures
  Allowed files: internal/modelmgr/doctor.go, internal/modelmgr/doctor_test.go
  Verification: go test ./internal/modelmgr/... -count=1

- [ ] model-004 Wire modelmgr through the install service: transactional mutations, namespaced sidecar records, uninstall and rollback enumeration
  Requirements: REQ-MODEL-004, REQ-MODEL-006
  Dependencies: model-001, model-002
  Objective: internal/install/model.go exposes ModelSet/ModelUnset/ModelList/ModelGet mirroring the MCPAddDesired recipe (internal/install/mcp.go:291-392): real runs gate on ErrNotInstalled, acquire the canonical home lock, reload the v2 context and fingerprint sidecar under the lock, run the manager mutation inside beginServiceTxn with a verified backup, record verified postimages, persist namespaced fingerprint records (agent-model/<agent>) in the existing sidecar document, and commit through a serviceTxn.commitStateModels method defined in model.go (same package; txn.go stays untouched) writing meta.AgentModels plus the derived lock. Dry-runs never lock or write and report the planned action and compact value. uninstall.go and rollback.go enumerate meta.AgentModels alongside MCPs so managed agent-model entries are removed or restored with the same verified-backup and reverse-restore guarantees. MCP behavior is unchanged.
  Acceptance:
    - Mutating calls on an uninstalled home fail with ErrNotInstalled guidance and zero writes
    - A committed set persists config, ownership record, and namespaced sidecar record atomically; any later failure restores exact preimages in reverse order
    - Dry-run reports the planned action and compact value without acquiring the lock or changing bytes
    - Uninstall/rollback enumerate managed model entries with verified backups; unrelated user config and MCP handling are untouched
  Allowed files: internal/install/model.go, internal/install/uninstall.go, internal/install/rollback.go
  Verification: go test ./internal/install/... -count=1

- [ ] model-005 Add the cortex-ia model CLI: parse, receipts, help text, and preflight regression tests
  Requirements: REQ-MODEL-001, REQ-MODEL-002, REQ-MODEL-005, REQ-MODEL-007
  Dependencies: model-003, model-004
  Objective: internal/app/model.go dispatches list|get|set|unset|doctor following the runMCP/parseMCPAdd precedent (internal/app/cli.go:341-447): parse --effort, --json, --dry-run (no --model* flags anywhere in the grammar), build modelmgr Desired values, call the install service, and render text plus --json receipts disclosing action, previous value, compact value, config path, backup ID, and warnings, with typed conflicts failing closed. app.go gains the model dispatch case as a strict sibling of mcp (internal/app/app.go:46-117 switch) and help text lines mirroring the mcp block (app.go:291-388). model_test.go covers the parse matrix, a preflight regression proving the documented grammar passes while any --model-prefixed argument still returns RetiredSurfaceError (app.go:251-257,276-289), receipt JSON shape, and the embedded-template regression guard asserting the internal/assets opencode.jsonc declares no agents/model keys. Keep the dedicated test file ≤ 250 LOC with focused persistent cases.
  Acceptance:
    - All five subcommands parse per the documented grammar; unknown subcommands and malformed references fail with usage or typed validation errors
    - The preflight regression passes for the documented grammar and rejects --model* arguments before dispatch
    - Text and --json receipts disclose previous value, action, config path, backup ID, and warnings; conflicts never partially write
    - The template regression guard fails if the embedded template gains agents or model keys
  Allowed files: internal/app/model.go, internal/app/app.go, internal/app/model_test.go
  Verification: go test ./internal/app/... -count=1

- [ ] model-006 Update AGENTS.md built-in surfaces for the model command
  Requirements: REQ-MODEL-001
  Dependencies: model-005
  Objective: The AGENTS.md "Important built-in surfaces" list gains `model` alongside `install`, `sync`, `mcp`, and the other surfaces, with one-line scope notes: global-config-only v1 and markdown-frontmatter pins surfaced as warnings, never silently rewritten. Documentation-only change; no product code.
  Acceptance:
    - The surfaces list names `model` and its subcommands list|get|set|unset|doctor
    - The scope note states global-config-only v1 and the doctor warning semantics for markdown frontmatter pins
    - No other AGENTS.md section is reworded
  Allowed files: AGENTS.md
  Verification: go build ./...

- [ ] model-007 Add the TUI models configuration screen with a modelsState/modelsAction seam persisting through the service path
  Requirements: REQ-MODEL-008, REQ-MODEL-003, REQ-MODEL-004
  Dependencies: model-004
  Objective: internal/tui/models_screen.go mirrors the stats screen architecture (internal/tui/stats_screen.go: self-contained state, screen-level action seam, injectable loader seam): modelsState lists registry agents with effective compact reference, variant, and source; free-form model input plus effort/variant selection produce a dry-run preview, and confirmation persists through the same install service mutation the CLI uses (injected seam functions wrapping the service — never ad-hoc file writes), rendering receipts and typed conflicts in-screen. model.go registers the screen and navigation exactly like the stats wiring (state field, constructor, action switch). models_screen_test.go drives the seams over temporary homes with synthetic inputs; dedicated test file ≤ 250 LOC. If the screen exceeds the flexible source budget, the state/action seam lands first and the view/keybinding polish follows as a stacked follow-up on the same files.
  Acceptance:
    - The screen renders every registry agent with effective model/variant and source, including unset builtins
    - The confirm path performs a dry-run preview then persists via the injected service seam; typed conflicts render in-screen without crashing the TUI
    - No TUI code path writes config files directly; all persistence goes through the service seam
    - Tests use injected seams and temporary homes only and stay within the modular test budget
  Allowed files: internal/tui/models_screen.go, internal/tui/model.go, internal/tui/models_screen_test.go
  Verification: go test ./internal/tui/... -count=1
