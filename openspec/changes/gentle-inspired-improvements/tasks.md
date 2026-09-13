# Tasks: Gentle-inspired improvements

## Authority, contract binding and scope

This sdd-full/tasks contract belongs to `gentle-inspired-improvements`, AUTO / HYBRID / current_workspace, session `cortex-gentle-improvements-20260912`. Only this file is writable in this dispatch. No board, task records, product edits, implementation approval or execution readiness is created. The intended sole board remains `gentle-inspired-improvements`. Checkboxes are immutable contract syntax: after hashing, use SQLite for progress, never edit checkboxes without a contract change.

Proposal, five specifications and design remain unchanged. Independent design review: cortex:225 / session `ses_f6864744cffeX8IkKl122RBjQy`. Raw-byte evidence is assigned separately to investigate: session `ses_f685d4e8affeYqJV3Jfmly6Iyj`, terminal job `bfa40333c784f94b75cfa2d17cc322b2`, reported collection 2026-09-12T22:01:39Z. These are evidence pointers, not substituted hashes or proof of current bytes. After this write/validation, an authorized read-only worker must hash all eight complete contract files directly from disk. A separate materialization dispatch must supply versioned `sdd_contract` bindings: version 1, workflow sdd-full, change_id gentle-inspired-improvements, plane hybrid, typed workspace-file transport/project/locator/full-file SHA-256 pins and each task's existing requirement IDs. Never use copied-text hashes, Git blob IDs, mutable topics, invented digests or missing bindings.

### Common execution contract inherited by every candidate

1. Each block's exact ID/title, objective, sorted `Allowed_files`, `Dependencies`, acceptance and verification forms its candidate record. Materialization must include these common clauses, not reduce records to one-line placeholders. Commands are **future** oracles, expected exit **0**; none ran during planning. Names and files marked here as new are proposed interfaces, not claims of existing implementation.
2. Forecasts count source, tests and Markdown separately; JSON/lock/YAML data is declarative and separately bounded. Every JS/TS/document unit is <=350 non-data changed lines; Go units <=500. Source ceilings: <=250 JS/TS, <=350 Go. Tests <=600 per unit and <=250 per new test file. Never append a new suite to an existing file >300 lines. If measured categorized churn exceeds a ceiling, stop for same-board decomposition, not code-golf, scope expansion or weakened acceptance. Markdown migration batches are <=250 changed lines each.
3. Before behavior changes, capture revision, scoped dirty-path raw hashes, exact command/input identity, toolchain/OS, expected/actual result and exit. Add new regression fixture first where needed, reproduce, then run the identical discriminating command after the fix. P1/P2 bootstrap their own evidence using bounded Git/tool reads and isolated Node tests; later units use P2. Static findings remain static until reproduced; correct existing behavior is verification-only.
4. P3's `run-contract-check.mjs` streams bounded Go JSON events, requires every comma-separated `--cases` top-level test to execute and pass, rejects skip/missing/truncation and propagates nonzero exit. It also requires exact Go 1.26.1 with GOTOOLCHAIN=local for actual Go invocation. Missing runners/tools never PASS. Node fixtures assert their named happy/edge/failure cases execute; empty fixture collections fail. Test helpers stay outside embedded plugin trees.
5. Scripts create unique temporary homes/state/output roots, reject empty paths and real-user config targets, and remove only owned scratch. Persistent scripts/assets are explicitly leased in the block that creates/changes them. Go tests use t.TempDir and restore process environment. No repository node_modules or generated sidebar/web writes. Shared process substitutes cannot be mutated by parallel fixtures. Baseline/check scripts accept an explicit evidence path via CORTEX_TEST_BASELINE; `--baseline-from-evidence` MUST require that variable and reject empty/stale/mismatched corpus evidence rather than search arbitrary temp files. PowerShell uses `$env:CORTEX_TEST_BASELINE`; POSIX uses `$CORTEX_TEST_BASELINE`. C0's baseline runner creates/returns the unique path for the controller to set; subsequent oracles reuse it unchanged.
6. **I20 is the mandatory prerequisite for all install-package execution, Windows install tests and full-suite runners.** Before I20 only named non-install Go tests and isolated scripts are permitted. I20's focused test is run only after replacing the real environment process and migrating existing unsafe fixtures. Temp HOME alone cannot isolate Windows registry writes. Audit every ConfigureEnvironment/Install/Sync test route, including env_test.go and service_test.go. Do not wrap or invoke real PowerShell from fixtures; do not run active install/sync commands at any point.
7. Normalize only leased source, verify, freeze, then independent review of identical bytes. Later overlapping authorized changes require orchestrator review-refresh and independent re-review before closure; preserve history. Candidate dependencies are not claims/leases. Actual parallel dispatch requires current SQLite readiness and disjoint per-file reservations.
8. Qualification targets exact Go 1.26.1, GOTOOLCHAIN=local and lint 2.11.4. The observed host Go 1.26.5 is not exact-version evidence. Linux race uses supported compiler and CGO_ENABLED=1. Missing toolchains/OS runners remain unavailable; no automatic download/upgrade or qualification is assumed.
9. No production trust/key packaging, embedded test keys, signing/publication, tag push, deployment, active install sync, credential redesign, new telemetry, old-task mutation or installed-user prompt edits. Protected paths are excluded: internal/tuiassets/cortex-ia-tui.tsx, internal/tuiassets/sidebar-smoke.tsx, internal/assets/tui/cortex-ia-tui.js, internal/delegation/runner.go, internal/delegation/delegation_telemetry_smoke_test.go and historical sidebar outputs. Synthetic assessment may read release/telemetry sources but never access secrets or run release hooks.

## 1. Policy, evidence and qualification foundations

- [ ] gentle-P1 [policy] Permit bounded critical regression contracts
Requirements: REQ-VERIFY-001
Dependencies: []
Allowed_files: ["AGENTS.md", "docs/sdd-workflow.md", "internal/assets/skills/_shared/codebase-design-contract.md", "internal/assets/skills/_shared/diagnosis-loop-contract.md", "internal/assets/skills/implement/SKILL.md", "scripts/check-test-policy.mjs", "scripts/check-test-policy.test.mjs"]
Objective: Reconcile the user-approved critical authority/transport/recovery/updater test exception in source policy without editing installed configuration. Preserve TUI/install-copy coverage and ephemeral unrelated exploration; fail unsafe, oversized or weakened oracles instead of granting blanket persistent-test permission.
Acceptance: Policy copies agree; negative fixtures reject real-home access, missing cases, >250-line new tests, appends to oversized suites and silent invalid-record skipping. No production test/full suite runs here.
Verification: `node --test scripts/check-test-policy.test.mjs && node scripts/check-test-policy.mjs`
Forecast: 90 source + 80 tests + 160 Markdown = 330.

- [ ] gentle-P2 [evidence] Capture comparable local baselines
Requirements: REQ-VERIFY-002
Dependencies: ["gentle-P1"]
Allowed_files: ["scripts/capture-local-baseline.mjs", "scripts/capture-local-baseline.test.mjs"]
Objective: Capture bounded local evidence from exact source/dirty bytes and a specified isolated oracle without remote collection. Compare identical commands/inputs before and after, distinguishing expected/actual result, absence and non-reproduction from PASS.
Acceptance: Temporary self-tests record revision, scoped hashes, OS/tools, command and exits; changed inputs cannot claim comparability. Empty output paths, fabricated savings, telemetry and full-suite execution before I20 evidence fail.
Verification: `node --test scripts/capture-local-baseline.test.mjs && node scripts/capture-local-baseline.mjs --self-test`
Forecast: 160 source + 120 tests = 280.

- [ ] gentle-P3 [verification] Require executed named Go cases
Requirements: REQ-VERIFY-001, REQ-VERIFY-002
Dependencies: ["gentle-P2"]
Allowed_files: ["scripts/run-contract-check.mjs", "scripts/run-contract-check.test.mjs"]
Objective: Provide the bounded subprocess/event boundary preventing Go's successful no-tests-to-run result from authorizing work. Preserve nonzero exits, timeouts and unavailable OS/tools, using synthetic child output for its own tests rather than install code.
Acceptance: Every requested test must run/pass, not skip; malformed/truncated/missing events, absent case, nonzero child or wrong compiler fail. Bound output and reject install/full-suite execution without I20 isolated-fixture evidence.
Verification: `node --test scripts/run-contract-check.test.mjs`
Forecast: 130 source + 120 tests = 250.

- [ ] gentle-K1 [harness] Qualify the locked SDK and test loader
Requirements: REQ-HARNESS-002, REQ-CONTEXT-001, REQ-MCP-001
Dependencies: ["gentle-P2"]
Allowed_files: ["docs/sdk-qualification.md", "scripts/harness-plugin-loader.mjs", "scripts/harness-plugin-loader.test.mjs", "scripts/qualify-harness-sdk.mjs"]
Objective: Qualify actual plugin/SDK 1.18.18 and TypeScript 5.9.3 using existing package/lock inputs rather than the transport's 1.18.29 comment. Load actual TS through a pinned transpiler and contract-faithful host/fs/process substitutes in a temporary copied-lock installation; never build protected sidebar assets.
Acceptance: Record manifest/npm/pnpm lock identities, resolved transitive SDK/plugin integrities and exercised hook/metadata/resume shapes; lock disagreement or unsupported shape blocks dependents. Loader proves real source hooks execute with replaced boundaries and cannot resolve ambient globals. Missing package/cache/network remains blocked; no dependency upgrade or repo node_modules write.
Verification: `node --test scripts/harness-plugin-loader.test.mjs && node scripts/qualify-harness-sdk.mjs --locked --isolate --require-plugin 1.18.18 --require-typescript 5.9.3`
Forecast: 170 source + 120 tests + 40 Markdown = 330; existing package/locks are read-only.

- [ ] gentle-Q0 [qualification] Resolve authentic immutable CI inputs
Requirements: REQ-DISTRIBUTION-001
Dependencies: ["gentle-P2"]
Allowed_files: ["docs/qualification-inputs.md", "scripts/check-qualification.mjs", "scripts/check-qualification.test.mjs", "scripts/qualification-inputs.json"]
Objective: Obtain authentic upstream commit provenance for existing action majors/reusable workflows and a verified compatible GoReleaser 2.x patch before consumer edits. Build an input/workflow checker distinguishing qualified identities from pending candidates without speculating pins or changing external settings.
Acceptance: Resolve checkout@v4, setup-go@v5, golangci-lint-action@v8, upload-artifact@v4, goreleaser-action@v6, github-script@v7 and stale@v9 at their current upstream repositories plus both lleontor705/ats-deploy-public workflows@main. Include recursively invoked dependencies, source URLs/time, peeled tags, exact 40-hex commits, tool release/checksum provenance, Go 1.26.1 and lint 2.11.4. Verify exact GoReleaser patch accepts schema 2 through isolated config checking without hooks/publishing. Missing authentic evidence, mutable transitive dependency, wrong repo/ref/digest, 39/41-hex or missing gate edges fail. Unfixable upstream mutability blocks Q11/Q12; no external repo writes.
Verification: `node --test scripts/check-qualification.test.mjs && node scripts/check-qualification.mjs --mode inputs --record scripts/qualification-inputs.json --require-authentic-provenance`
Forecast: 170 source + 120 tests + 40 Markdown = 330; declarative record <=250 lines.

- [ ] gentle-M0 [mcp] Qualify frozen integration inputs and schemas
Requirements: REQ-MCP-001
Dependencies: ["gentle-P2"]
Allowed_files: ["docs/mcp-qualification.md", "scripts/fixtures/mcp/capabilities.json", "scripts/fixtures/mcp/package-lock.json", "scripts/fixtures/mcp/package.json", "scripts/qualify-mcp.mjs", "scripts/qualify-mcp.test.mjs"]
Objective: Qualify candidate Context7 4.1.0 with an authentic complete transitive lock and actual local Cortex executable identity before preset edits. Use isolated initialize/tools-list exchanges and versioned fixtures; no active config, real documentation query or secret-bearing environment.
Acceptance: Verify official npm integrity, Node >=20.18.1 and resolved lock inputs; record Cortex version, raw executable SHA-256 and actual agent schema. At most two protocol requests per server, ten seconds total per run and 1 MiB aggregate output. Optional absent capability is reported; missing required capability, malformed/unmanaged input or changed lock fails. Offline fixtures alone cannot qualify live compatibility; missing actual input blocks M1, not fixture-only approval.
Verification: `node --test scripts/qualify-mcp.test.mjs && node scripts/qualify-mcp.mjs --locked --isolate --timeout-ms 10000 --max-output-bytes 1048576 --max-requests 2 --fixture-root scripts/fixtures/mcp --require-live-schema-evidence`
Forecast: 170 source + 120 tests + 40 Markdown = 330; locks/schema are declarative, no invented entries.

## 2. Runtime harness and precise context

- [ ] gentle-H1 [transport] Admit native operational effects without file authority
Requirements: REQ-HARNESS-001
Dependencies: ["gentle-K1"]
Allowed_files: ["internal/assets/plugins/cortex-subagent-transport.ts", "scripts/harness-operational.test.mjs"]
Objective: Reproduce empty operational scope rejection and admit only native verified implement operational/database requests with explicit target/effects and valid task authority. Preserve repository-write guards and unrelated baseline bytes; external operational execution remains unavailable without touching runner.go or leased-file admission.
Acceptance: Actual-plugin fixtures admit scoped native effects and preserve dirty baseline; reject missing effects, ordinary empty file scopes, spoofed roles, repository writes and unavailable authority. External guidance requires reconciled fresh orchestrator-native dispatch, never automatic fallback.
Verification: `node --test scripts/harness-operational.test.mjs`
Forecast: 110 source + 190 tests = 300.

- [ ] gentle-H2 [transport] Parse one budget form without identity spoofing
Requirements: REQ-HARNESS-002
Dependencies: ["gentle-H1"]
Allowed_files: ["internal/assets/plugins/cortex-subagent-transport.ts", "internal/assets/skills/_shared/cortex-work-protocol.md", "scripts/harness-budget.test.mjs"]
Objective: Canonicalize max_steps and accept deprecated budget.max_turns plus existing args.steps/args.max_steps as mutually exclusive aliases for one advisory count. Reject duplicate keys before JSON collapse and derive planner exemption/fallback role only from verified host/resume identity, not envelope or args.agent claims.
Acceptance: Reject duplicate decoded keys including escaped spellings, null/array/non-object/malformed budgets, missing/invalid budget integer, multiple forms even equal, multiple envelopes and role conflicts. Host planner/resume uncapped; spoofing cannot elevate. One current advisory notice leaves tools usable; non-planner emergency permits exactly five cleanup calls, invalid init config rejects and repetition only warns; no SQLite transition. Operational regression still passes.
Verification: `node --test scripts/harness-budget.test.mjs scripts/harness-operational.test.mjs`
Forecast: 110 source + 210 tests + 25 Markdown = 345.

- [ ] gentle-H3 [latch] Explain reconciled restart continuation
Requirements: REQ-HARNESS-003
Dependencies: ["gentle-H2"]
Allowed_files: ["internal/assets/plugins/cortex-task-latch.ts", "internal/assets/skills/_shared/cortex-work-protocol.md", "scripts/harness-latch.test.mjs"]
Objective: Reproduce failure/latch/disposal behavior and document the actual supported continuation. Orchestrator reconciles durable state, operator restarts the plugin host if still latched, and the same initiative receives a fresh authorized attempt without stale tokens.
Acceptance: Fixtures demonstrate latch persistence and new-instance continuation; unknown identity/capability stays blocked. No conversation deletion, invented unlatch API, leaf retry, arbitrary success-string clear or native automatic fallback.
Verification: `node --test scripts/harness-latch.test.mjs`
Forecast: 70 source + 180 tests + 35 Markdown = 285.

- [ ] gentle-C1 [context] Resolve exact permitted locators with provenance
Requirements: REQ-CONTEXT-001
Dependencies: ["gentle-K1"]
Allowed_files: ["internal/assets/plugins/cortex-skill-discovery.ts", "scripts/harness-context.test.mjs"]
Objective: Resolve authorized explicit locators before inventory fallback, exposing path/origin/precedence without content logging. Distinguish host-installed, repository and embedded-source inventories, preserving host policy and existing hooks rather than treating discovery as permission.
Acceptance: Fixtures exercise explicit host precedence, OS-canonical identity, differing inventories, same-precedence collisions, missing/unreadable/invalid paths and visible errors. No silent substitution; read loads exact permitted contracts and skill uses actual host names only.
Verification: `node --test scripts/harness-context.test.mjs`
Forecast: 150 source + 180 tests = 330.

## 3. Progressive-disclosure batches

C0 freezes revision, relevant dirty raw hashes and exact sorted corpus before C2 edits: assets AGENTS, six role files, all current skills/*/SKILL.md (including workflow-retrospective), six _shared contracts, all commands and the transport literal supplemental text. Host/installed-user instructions and generated assets are excluded. P1/H2/H3 changes retain their own pre-change baselines; C0 is the dedicated deduplication baseline, not a retroactive savings baseline for those changes.

Metric: design's CRLF->LF/intra-prose whitespace normalization; ignore blank/code/pointer-only blocks; count exact repeated eligible blocks >=20 words, excess (n-1)*L, D=sum(excess), W=eligible words. Only five universal summaries are exempt: host precedence, role authority, live claims/leases, accepted-job reconciliation, no secrets. At most one summary per always-loaded surface, <=80 words/surface; exempt D separately reported. Final acceptance: zero broken pointers, zero conflicting normative copies, one canonical definition/concept, zero non-exempt repeated normative blocks, W <= original C0 baseline. No new exemptions or claimed token savings.

C0 owns metric tests and the exact concept/corpus/batch manifest. Each batch replaces duplicated normative detail with exact triggered pointers while retaining ordered/checkable procedures and universal safety. Canonical paths remain stable; ownership follows the manifest. Batch mode checks actual selected concepts, pointers, no introduced conflicts/duplicates and W nonincrease, explicitly reporting unfinished other batches. It is not final-corpus PASS. Final C32 uses the same frozen baseline and all-corpus mode. Independent semantic review detects paraphrased conflicts beyond exact matching.

- [ ] gentle-C0 [prompts] Freeze the corpus and enforce its approved metric
Requirements: REQ-CONTEXT-002, REQ-VERIFY-002
Dependencies: ["gentle-P2", "gentle-H3", "gentle-C1"]
Allowed_files: ["scripts/check-harness-contracts.mjs", "scripts/check-harness-contracts.test.mjs", "scripts/harness-contracts.json"]
Objective: Implement the exact design metric, canonical ownership registry and explicit batch selectors before deduplicating prompts. Freeze local evidence in an isolated output, retaining a reproducible baseline path rather than a mutable benchmark or host-text rewrite.
Acceptance: Tests cover duplicate/paraphrase distinction, code/pointer exclusion, 19/20-word boundary, 80/81-word summary, broken pointer/trigger and W increase. Enumerate full current corpus including workflow-retrospective; baseline reports violations, final rejects them. Self-test cannot claim actual corpus clean; controller retains the baseline path for later commands.
Verification: `node --test scripts/check-harness-contracts.test.mjs && node scripts/check-harness-contracts.mjs --mode baseline --create-isolated-baseline`
Forecast: 180 source + 150 tests = 330; declarative manifest <=350 lines.

- [ ] gentle-C21 [prompts] Migrate asset guide startup and routing
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/AGENTS.md"]
Objective: Replace redundant startup/topology/routing detail with triggered canonical workflow/work-protocol pointers. Preserve operating choices, proportional direct routes and universal boundaries; reserve later sections for C22.
Acceptance: Startup/routing concepts pass batch checks; direct work avoids SDD and no host/role authority moves to a weaker source.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C21 --baseline-from-evidence`
Forecast: 0 source + 0 tests + 240 Markdown = 240.

- [ ] gentle-C22 [prompts] Migrate asset guide lifecycle and memory detail
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C21"]
Allowed_files: ["internal/assets/AGENTS.md"]
Objective: Replace remaining duplicated lifecycle/verification/memory procedures with exact canonical pointers. Retain bounded always-loaded safety and usable work/evidence triggers without replacing the host system.
Acceptance: Remaining guide concepts have one owner and reachable completion gates; use original C0 baseline, not a reset after C21.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C22 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C23 [prompts] Migrate read-only role controllers
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/agents/discovery.md", "internal/assets/agents/investigate.md"]
Objective: Keep discovery ownership and bounded diagnosis local while pointing shared evidence/work/safety definitions to canonical sources. Preserve always-native discovery versus optional read-only investigation delegation without lifecycle/product-write grants.
Acceptance: Distinct role procedures remain executable through exact triggers and <=80-word universal summaries; no copied normative procedures or invented tools.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C23 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C24 [prompts] Migrate implementation and review controllers
Requirements: REQ-CONTEXT-002, REQ-REVIEW-001
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/agents/implement.md", "internal/assets/agents/reviewer.md"]
Objective: Preserve separate implementation/review authority while referring lifecycle, scope, workload and freeze/refresh procedures to canonical owners. Retain normalize->verify->freeze->review and evidence-based disagreement resolution, not consensus veto.
Acceptance: Reviewer cannot edit/self-approve; implementer cannot self-complete; changed bytes require authorized refresh and independent review. Batch negatives catch missing triggers and premature completion.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C24 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C25 [prompts] Migrate coordination and planning controllers
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/agents/orchestrator.md", "internal/assets/agents/planner.md"]
Objective: Retain phase/routing ownership and selected-plane authority while removing duplicate protocol bodies. Keep delegation gate, semantic-before-structural admission, board continuity and complete human delivery reachable through precise triggers.
Acceptance: Simple route avoids SDD, planner stays within authorized contracts, and phase/pin obligations remain accessible without invented skills or host replacement.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C25 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C26 [prompts] Reconcile shared canonical ownership
Requirements: REQ-CONTEXT-002, REQ-REVIEW-001
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/skills/_shared/agent-writing-contract.md", "internal/assets/skills/_shared/codebase-design-contract.md", "internal/assets/skills/_shared/cortex-convention.md", "internal/assets/skills/_shared/cortex-work-protocol.md", "internal/assets/skills/_shared/diagnosis-loop-contract.md", "internal/assets/skills/_shared/workflow-map.md"]
Objective: Remove only cross-contract duplicates according to C0, retaining definitions at their registered canonical owners. Preserve H2/H3 semantics, P1 exception, AST/input immutability and fresh-review ordering; no canonical path rename/removal.
Acceptance: Stable pointers retain every conditional obligation; no circular pointer chain without a definition or weakening of budgets/authority.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C26 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C27 [prompts] Migrate implementation and review procedures
Requirements: REQ-CONTEXT-002, REQ-VERIFY-001
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/skills/code-review-adversary/SKILL.md", "internal/assets/skills/fast-tdd/SKILL.md", "internal/assets/skills/hotfix-triage/SKILL.md", "internal/assets/skills/implement/SKILL.md"]
Objective: Preserve distinct RED/GREEN, containment and adversarial procedures while replacing repeated authority/test-policy paragraphs with triggers. Retain critical-boundary permission and discriminating negative oracles rather than generic guidance.
Acceptance: Ordered outputs and policy loading remain explicit; no production/cleanup expansion or consensus approval rule appears.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C27 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C28 [prompts] Migrate evidence and investigation procedures
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/skills/ast-impact-analysis/SKILL.md", "internal/assets/skills/context-distiller/SKILL.md", "internal/assets/skills/discovery/SKILL.md", "internal/assets/skills/investigate/SKILL.md"]
Objective: Preserve evidence acquisition/compression procedures while replacing copied memory/lifecycle rules with exact references. Retain discovery-only profile ownership, bounded diagnosis and static-versus-executed proof distinction.
Acceptance: Inventory/provenance and absent-tool cases stay explicit; stale tool names never become assumed capability or duplicate memory authority.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C28 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C29 [prompts] Migrate planning and dispatch procedures
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/skills/orchestrator/SKILL.md", "internal/assets/skills/parallel-dispatch/SKILL.md", "internal/assets/skills/planner/SKILL.md"]
Objective: Retain routing, bounded DAG design and disjoint-dispatch procedures while centralizing shared phase/pin/claim rules. Preserve genuine prerequisites and exact file authority without prompt compiler or artificial sequencing.
Acceptance: Direct work stays proportionate and unavailable authority cannot become readiness; parallel paths require current SQLite state and disjoint claims/leases.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C29 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C30 [prompts] Migrate specialist procedure pointers
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/skills/grill-me/SKILL.md", "internal/assets/skills/mutation-testing/SKILL.md", "internal/assets/skills/property-based-testing/SKILL.md", "internal/assets/skills/spike-prototype/SKILL.md", "internal/assets/skills/workflow-retrospective/SKILL.md"]
Objective: Preserve interview, disposable experiment, mutation/generative and retrospective procedures while removing shared authority duplication. Retain complete human option envelopes and explicit scratch/test scope; skills never grant execution authority themselves.
Acceptance: Each actual source skill has valid trigger/completion examples; no narrowed choices, invented capabilities or hidden persistent-test expansion.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C30 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C31 [prompts] Migrate command entrypoint pointers
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C0"]
Allowed_files: ["internal/assets/commands/cortex-code.md", "internal/assets/commands/cortex-ingest.md", "internal/assets/commands/cortex-watch.md", "internal/assets/commands/discover.md", "internal/assets/commands/hotfix.md", "internal/assets/commands/investigate.md", "internal/assets/commands/resume.md", "internal/assets/commands/review.md", "internal/assets/commands/sdd.md", "internal/assets/commands/spike.md", "internal/assets/commands/status.md", "internal/assets/commands/tdd.md", "internal/assets/commands/work.md"]
Objective: Make existing commands concise entrypoints to their distinct procedures rather than normative copies. Preserve intent/arguments, reconciliation guidance and least-cost routing without new commands or active execution.
Acceptance: All command triggers resolve; missing context is visible and simple status/direct work avoids unnecessary SDD and speculative tools.
Verification: `node scripts/check-harness-contracts.mjs --mode batch --batch C31 --baseline-from-evidence`
Forecast: 0 + 0 + 250 Markdown = 250.

- [ ] gentle-C32 [prompts] Minimize transport text and close corpus verification
Requirements: REQ-CONTEXT-002
Dependencies: ["gentle-C22", "gentle-C23", "gentle-C24", "gentle-C25", "gentle-C26", "gentle-C27", "gentle-C28", "gentle-C29", "gentle-C30", "gentle-C31"]
Allowed_files: ["internal/assets/plugins/cortex-subagent-transport.ts", "scripts/harness-prompt.test.mjs"]
Objective: Minimize only literal supplemental transport text, preserving parsing/state logic and host instructions. Join all migrations against original C0 baseline with the absolute thresholds, never an intermediate baseline or increased exemptions.
Acceptance: Actual-hook tests retain role boundaries, <=80-word summary and direct routing; full corpus has zero broken/conflicting/non-exempt copies and W nonincrease. H1/H2 regressions still pass; no telemetry/savings claim.
Verification: `node --test scripts/harness-prompt.test.mjs scripts/harness-operational.test.mjs scripts/harness-budget.test.mjs && node scripts/check-harness-contracts.mjs --mode check --baseline-from-evidence`
Forecast: 50 source + 160 tests = 210.

## 4. Fresh review and immutable projection

- [ ] gentle-R1 [delegation] Verify normalized current review fingerprints
Requirements: REQ-REVIEW-001
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/delegation/review_freshness_test.go", "internal/delegation/spec_contract.go"]
Objective: Exercise existing fingerprint/refresh behavior through temporary SQLite, distinguishing protected existing behavior from reproduced defects. Keep mutation transaction ownership and historical approvals; any narrow fix retains normalized-current-byte and independent-reviewer requirements.
Acceptance: TestReviewFreshness covers normalize/freeze, changed file/contract stale rejection, authorized refresh/history, self-approval denial and workload admission. If baseline passes, spec_contract.go remains unchanged; prompt procedure ordering is C24/C26, not new writable scope here.
Verification: `node scripts/run-contract-check.mjs --cases TestReviewFreshness -- go test -json -count=1 ./internal/delegation -run '^TestReviewFreshness$'`
Forecast: 60 Go source + 220 tests = 280.

- [ ] gentle-S1 [delegation] Project coherent token-free work state
Requirements: REQ-STATE-001
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/delegation/work_projection.go", "internal/delegation/work_projection_read.go", "internal/delegation/work_projection_read_test.go", "internal/delegation/work_projection_test.go"]
Objective: Add immutable v1 projection logic over one coherent SQLite read transaction, separating notes/blockers/status/role actions. Use defensive copies and a narrow snapshot seam without schema, cache, mutation API or multi-read GetWork assumptions.
Acceptance: TestWorkProjection/TestWorkProjectionRead cover missing/unknown role, dependencies, owner/expiry, independent review, blocked/expired/done/superseded, stale notes and concurrent coherent reads. Nested output mutation cannot affect input/subsequent output; no tokens escape and actions require live revalidation/claims/leases.
Verification: `node scripts/run-contract-check.mjs --cases TestWorkProjection,TestWorkProjectionRead -- go test -json -count=1 ./internal/delegation -run '^TestWorkProjection(Read)?$'`
Forecast: 240 Go source + 240 tests (120/file) = 480.

- [ ] gentle-S2 [app] Expose additive work-status projection
Requirements: REQ-STATE-001
Dependencies: ["gentle-S1"]
Allowed_files: ["internal/app/work.go", "internal/app/work_projection_test.go"]
Objective: Add the projection to existing work-status output without removing fields/aliases. Treat CLI role as presentation only, with unknown default suppressing action suggestions rather than authenticating anything.
Acceptance: TestWorkProjectionCLI covers legacy compatibility, token-free field, separate phase/task/verdict, unknown role and failed retrieval; no HTTP/mutation/schema expansion.
Verification: `node scripts/run-contract-check.mjs --cases TestWorkProjectionCLI -- go test -json -count=1 ./internal/app -run '^TestWorkProjectionCLI$'`
Forecast: 100 Go source + 180 tests = 280.

- [ ] gentle-S3 [bridge] Freeze host-role projections and reject stale responses
Requirements: REQ-STATE-001
Dependencies: ["gentle-S2", "gentle-K1"]
Allowed_files: ["internal/assets/plugins/herdr-bridge.ts", "scripts/harness-projection.test.mjs"]
Objective: Consume additive projections with host-derived presentation role and deep-copy/freeze serialization. Discard older revision or wrong task/session responses; failed retrieval replaces cached readiness with unknown, never memory-derived authority.
Acceptance: Real-bridge fixtures cover nested mutation, response ordering/mismatch, missing authority and model-selected privileged role rejection. Preserve existing control tools; no runner/sidebar migration, new approval/recovery endpoint or token logging.
Verification: `node --test scripts/harness-projection.test.mjs`
Forecast: 110 TS source + 200 tests = 310.

## 5. Install isolation, effects and exact consumers

Source inspection fixes I3 consumers as internal/app/cli.go (previewAndApply, printInstallReceipt) and internal/tui/actions.go (onInstallDone, installDetail). Internal install/service.go owns recovery, not the dispatcher. No protected sidebar file is involved.

- [ ] gentle-I1 [delegation] Return validated configuration write outcomes
Requirements: REQ-INSTALL-001
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/delegation/config.go", "internal/delegation/config_write_result_test.go"]
Objective: Add save-with-result using existing atomic writing and explicit authorization/expected preimage, not filename/template equality. Validate requested/existing configuration, preserve compatibility wrappers and expose actual create/change/no-op/failure to the locked service effect.
Acceptance: TestConfigWriteResult covers creation, identical bytes, authorized change, malformed request/existing content, unmanaged/stale preimage and writer failure. No pre-validation mutation, silent skip or caller-owned struct mutation; result is actual writer outcome rather than guess.
Verification: `node scripts/run-contract-check.mjs --cases TestConfigWriteResult -- go test -json -count=1 ./internal/delegation -run '^TestConfigWriteResult$'`
Forecast: 150 Go source + 220 tests = 370.

- [ ] gentle-I20 [install] Replace environment effects in every install fixture
Requirements: REQ-INSTALL-001, REQ-VERIFY-001
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/install/env.go", "internal/install/env_isolation_test.go", "internal/install/env_test.go", "internal/install/service.go", "internal/install/service_test.go"]
Objective: Introduce the narrow environment process/resource substitute preventing real Windows registry writes during tests. Migrate existing ConfigureEnvironment and service Install/Sync fixtures, restore process environment and keep production behavior distinct from fixture substitution.
Acceptance: TestEnvironmentIsolation proves Windows runner is replaced, records intended argv/error and never launches PowerShell; env_test.go/service_test.go use it too. Audit every current test route before first execution; missing substitute blocks. Retain real production-path logic through replacement (not skipped/no-op tests), Unix temp-home effects and visible errors. No full suite until independent I20 PASS.
Verification: `node scripts/run-contract-check.mjs --cases TestEnvironmentIsolation,TestConfigureEnvironment -- go test -json -count=1 ./internal/install -run '^(TestEnvironmentIsolation|TestConfigureEnvironment)$'`
Forecast: 120 Go source + 230 tests (existing edits included, new file <=180) = 350.

- [ ] gentle-I21 [install] Report separate post-pipeline effect outcomes
Requirements: REQ-INSTALL-001
Dependencies: ["gentle-I1", "gentle-I20"]
Allowed_files: ["internal/install/env.go", "internal/install/post_pipeline_effects_test.go", "internal/install/receipt.go", "internal/install/service.go", "internal/install/tui_plugin.go"]
Objective: After confirmed pipeline success under the existing home lock, account for environment, TUI registration and delegation config in current order. Add bounded v1 post_pipeline_effects/partial_success with transaction_covered=false, actual writer outcomes and propagated errors without widening pipeline transaction coverage.
Acceptance: TestPostPipelineEffects covers not_requested/unchanged/changed/failed/not_attempted, converged/confirmed paths, stop-on-failure, surviving earlier effects and accurate Changed. Preserve pipeline digest/journal/backup and malformed TUI rejection; propagate Unix/Windows-substitute errors. Dry-run calls no writer/process and reports no applied/qualified effects. Existing cleanup stays bounded and isolated, no expansion.
Verification: `node scripts/run-contract-check.mjs --cases TestPostPipelineEffects -- go test -json -count=1 ./internal/install -run '^TestPostPipelineEffects$'`
Forecast: 230 Go source + 240 tests = 470.

- [ ] gentle-I30 [install] Recover failed effects with fresh preimage confirmation
Requirements: REQ-INSTALL-001
Dependencies: ["gentle-I21"]
Allowed_files: ["internal/install/effect_recovery_test.go", "internal/install/service.go"]
Objective: Reinspect/replan a failed separate effect with fresh explicit authorization bound to its preimage, not pipeline PlanDigest. Preserve completed pipeline/earlier effects and reject unmanaged drift without inventing accreditation or another ownership database.
Acceptance: TestEffectRecovery covers partial failure/retry, fresh and stale preimages, no-op retry, unchanged pipeline journal/digest/backup and truthful recovery failure. No universal rollback or implicit undo of earlier separate effects.
Verification: `node scripts/run-contract-check.mjs --cases TestEffectRecovery -- go test -json -count=1 ./internal/install -run '^TestEffectRecovery$'`
Forecast: 170 Go source + 220 tests = 390.

- [ ] gentle-I31 [app] Render surviving effects without full-success claims
Requirements: REQ-INSTALL-001
Dependencies: ["gentle-I30"]
Allowed_files: ["internal/app/cli.go", "internal/app/install_effects_test.go"]
Objective: Update actual CLI receipt consumers to distinguish partial separate effects from pipeline restoration. Keep ownership in the service and rendering in the dispatcher, preserving nonzero error while exposing bounded safe recovery codes.
Acceptance: TestInstallEffectsCLI uses fake apply receipts for changed/no-op/dry-run/partial/stale-confirmation. Surviving changes cannot render 'nothing was written'; required failure suppresses success and guidance requests fresh reconciliation, never active install.
Verification: `node scripts/run-contract-check.mjs --cases TestInstallEffectsCLI -- go test -json -count=1 ./internal/app -run '^TestInstallEffectsCLI$'`
Forecast: 90 Go source + 180 tests = 270.

- [ ] gentle-I32 [tui] Render separate effect failures from service receipts
Requirements: REQ-INSTALL-001
Dependencies: ["gentle-I30"]
Allowed_files: ["internal/tui/actions.go", "internal/tui/install_effects_test.go"]
Objective: Update onInstallDone/installDetail for separate effects/partial success without weakening MCP qualification/error gates. Use ServiceAPI fakes in a dedicated test file; native installer output is distinct from protected sidebar assets.
Acceptance: TestInstallEffectsTUI covers partial/dry-run/no-op, nil/non-nil receipts, qualified/unqualified MCPs and required-effect errors. No false PASS, blanket restored/nothing-written claim or universal rollback implication; existing oversized tests stay untouched.
Verification: `node scripts/run-contract-check.mjs --cases TestInstallEffectsTUI -- go test -json -count=1 ./internal/tui -run '^TestInstallEffectsTUI$'`
Forecast: 90 Go source + 180 tests = 270.

## 6. Authenticated updater, unavailable in production throughout

Every updater slice inherits exact design bounds and denies unsigned execution from U1 onward. Production trust stays empty after all fixtures pass; no public constructor/mutable Release bypass or environment/URL root. Test keys are generated at runtime and never committed. Intermediate public paths stay fail-closed, not partially authenticated. Only U4 connects the complete fixture chain through unexported test seams.

- [ ] gentle-U1 [updater] Disable unsigned check and apply immediately
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/app/update.go", "internal/app/update_auth_test.go", "internal/updater/trust.go", "internal/updater/trust_test.go", "internal/updater/updater.go"]
Objective: Package empty production trust and reject CheckLatest/ApplyUpdate before network/download/replacement when trusted roots are absent. Change ApplyUpdate to require currentVersion and pass app.Version from the sole inspected caller runUpdate; trust absence is unavailable, never already-up-to-date.
Acceptance: TestUpdateTrust/TestUpdateCLIAuth prove exact message 'Authenticated update unavailable: no trusted release key is packaged', no effects, nil/mutated Release rejection and no constructor bypass. Both unsigned public paths disabled in U1, help usable, unknown/dev version cannot auto-apply; no deferred security gap.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateTrust,TestUpdateCLIAuth -- go test -json -count=1 ./internal/updater ./internal/app -run '^(TestUpdateTrust|TestUpdateCLIAuth)$'`
Forecast: 180 Go source + 220 tests (110/file) = 400.

- [ ] gentle-U20 [updater] Enforce pinned key intervals and stable-version floors
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U1"]
Allowed_files: ["internal/updater/trust.go", "internal/updater/trust_rotation_test.go", "internal/updater/version.go", "internal/updater/version_test.go"]
Objective: Implement maximum-two-record Ed25519 trust and strict canonical stable versions independently of permissive legacy comparison. Bind repository/inclusive intervals; application must exceed known running and successful client-applied floors and downloaded keys never authorize rotation.
Acceptance: TestUpdateRotation/TestUpdateVersion cover predecessor, bounded overlap/successor, unknown/retired/malformed keys/repository, length validation and vMAJOR.MINOR.PATCH. Reject leading zeros, suffixes, overflow, dev/unknown floors, equal/old/replayed apply; authenticated equal-version check may report no update. Production roots stay empty.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateRotation,TestUpdateVersion -- go test -json -count=1 ./internal/updater -run '^(TestUpdateRotation|TestUpdateVersion)$'`
Forecast: 180 Go source + 220 tests (110/file) = 400.

- [ ] gentle-U21 [updater] Verify exact signed manifest bytes and bindings
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U20"]
Allowed_files: ["internal/updater/manifest.go", "internal/updater/manifest_test.go"]
Objective: Verify v1 manifest/signature envelopes over exact received bytes with duplicate-key/trailing-content rejection. Bind repository/tag/unique artifact name/OS/arch/size/SHA-256 to trust policy, never normalize signed bytes or treat checksums.txt as authentication.
Acceptance: TestUpdateManifest covers runtime-signed producer, unknown schema, duplicate decoded keys/artifact identities, trailing JSON, malformed encoding/hash and wrong repository/tag/key/interval. Enforce 1 MiB manifest and 16 KiB signature envelope limits including limit+1; no case-fold/substring fallback.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateManifest,TestUpdateRotation,TestUpdateVersion -- go test -json -count=1 ./internal/updater -run '^(TestUpdateManifest|TestUpdateRotation|TestUpdateVersion)$'`
Forecast: 210 Go source + 230 tests = 440.

- [ ] gentle-U22 [updater] Bind authenticated metadata to bounded downloads
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U21"]
Allowed_files: ["internal/updater/download.go", "internal/updater/download_auth_test.go", "internal/updater/updater.go"]
Objective: Retrieve bounded GitHub hints/metadata/artifacts using HTTPS and explicit fixed GitHub-owned redirect hosts, verifying exact signed artifact and stored-byte digest/size before extraction. Recheck mutable Release and current-version floors at apply; derive locations from fixed repo/tag, not caller URL.
Acceptance: TestUpdateDownload substitutes loopback HTTP and proves authentication before archive fetch and byte checks before extraction. Reject arbitrary origins/redirects, mutated Release, case/substring assets, truncation/misleading lengths, archive 128 MiB+1; honor timeout/context. Public trust remains empty and legacy apply cannot become reachable before U4.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateDownload,TestUpdateTrust -- go test -json -count=1 ./internal/updater -run '^(TestUpdateDownload|TestUpdateTrust)$'`
Forecast: 210 Go source + 230 tests = 440.

- [ ] gentle-U30 [updater] Validate every archive member before extraction
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U22"]
Allowed_files: ["internal/updater/archive.go", "internal/updater/archive_safety_test.go", "internal/updater/updater.go"]
Objective: Replace basename extraction with complete bounded ZIP/tar validation after authenticated bytes. Validate even entries after the target, allow inert regular release docs/scripts, extract only one exact root cortex-ia/cortex-ia.exe and execute none.
Acceptance: TestUpdateArchive covers 1024/1025 members, 128 MiB extracted limit+1, absolute/drive/UNC/backslash/NUL/traversal, links/devices, duplicate/case-colliding paths and multiple binaries. Late malicious members fail before replacement; helper path cannot bypass U1 trust.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateArchive,TestUpdateTrust -- go test -json -count=1 ./internal/updater -run '^(TestUpdateArchive|TestUpdateTrust)$'`
Forecast: 220 Go source + 240 tests = 460.

- [ ] gentle-U31 [updater] Stage replacement and verify recovery bytes
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U30"]
Allowed_files: ["internal/updater/replacement.go", "internal/updater/replacement_safety_test.go", "internal/updater/updater.go"]
Objective: Stage verified bytes beside the destination and propagate write/close/rename/rollback failures while preserving a working backup when recovery is incomplete. Never overwrite unrelated .old files, and require final digest before success without promising universal atomic reversibility.
Acceptance: TestUpdateReplacement covers writes/close/rename/final digest/restore faults, unrelated .old preservation and success on actual OS using temporary executable targets. Original bytes survive or explicit recovery failure retains backup; no retry/reboot cleanup/real executable replacement. Missing Windows runtime remains unverified for Q3.
Verification: `node scripts/run-contract-check.mjs --cases TestUpdateReplacement,TestUpdateTrust -- go test -json -count=1 ./internal/updater -run '^(TestUpdateReplacement|TestUpdateTrust)$'`
Forecast: 200 Go source + 240 tests = 440.

- [ ] gentle-U4 [updater] Prove the complete non-production authentication chain
Requirements: REQ-UPDATE-001
Dependencies: ["gentle-U31"]
Allowed_files: ["internal/app/update_auth_test.go", "internal/updater/authenticated_update_test.go", "internal/updater/updater.go"]
Objective: Connect only verified metadata/download/archive/staged replacement and exercise generated test trust against a temporary target. Keep explicit currentVersion and empty packaged roots: passing signed fixtures never activates production release signing.
Acceptance: TestAuthenticatedUpdate proves producer/consumer success, authenticated equal check, successful-applied floor replay protection and broken-chain unchanged/recoverable target. Run every preceding updater/CLI case; no public test-root constructor, environment root override, signing job or unsigned path.
Verification: `node scripts/run-contract-check.mjs --cases TestAuthenticatedUpdate,TestUpdateTrust,TestUpdateCLIAuth,TestUpdateRotation,TestUpdateVersion,TestUpdateManifest,TestUpdateDownload,TestUpdateArchive,TestUpdateReplacement -- go test -json -count=1 ./internal/updater ./internal/app -run '^(TestAuthenticatedUpdate|TestUpdateTrust|TestUpdateCLIAuth|TestUpdateRotation|TestUpdateVersion|TestUpdateManifest|TestUpdateDownload|TestUpdateArchive|TestUpdateReplacement)$'`
Forecast: 100 Go source + 240 tests (new <=200, CLI edits <=40) = 340.

## 7. Qualified inputs and final target evidence

- [ ] gentle-M1 [mcp] Consume qualified versioned preset evidence offline
Requirements: REQ-MCP-001
Dependencies: ["gentle-M0", "gentle-P3"]
Allowed_files: ["internal/mcpmanager/presets.go", "internal/mcpmanager/qualification.go", "internal/mcpmanager/qualification_contract_test.go"]
Objective: Pin Context7 4.1.0 only after M0 authentic locked/schema qualification, accepting bounded caller-supplied identity/schema evidence at the offline probe seam. Preserve PATH/URL checks as usability evidence, not protocol compatibility; never add install-time network probing or a dependency upgrade.
Acceptance: TestMCPQualificationContract rejects mismatched package/executable/lock, missing required schema, malformed input and unmanaged overwrite, reporting optional absence. Root npx pin cannot imply frozen transitive resolution; changed actual dependencies remain unqualified. Offline runner exercises all fixture cases without real documentation calls.
Verification: `node scripts/run-contract-check.mjs --cases TestMCPQualificationContract -- go test -json -count=1 ./internal/mcpmanager -run '^TestMCPQualificationContract$' && node scripts/qualify-mcp.mjs --offline --timeout-ms 10000 --max-output-bytes 1048576 --fixture-root scripts/fixtures/mcp`
Forecast: 150 Go source + 220 tests = 370.

- [ ] gentle-Q11 [ci] Apply immutable CI and maintenance inputs
Requirements: REQ-DISTRIBUTION-001
Dependencies: ["gentle-Q0", "gentle-I20"]
Allowed_files: [".github/workflows/ci.yml", ".github/workflows/pr-check.yml", ".github/workflows/stale.yml"]
Objective: Replace existing refs with Q0 verified commits without major upgrades or weakened CI/security/PR/maintenance behavior. Align to Go 1.26.1/GOTOOLCHAIN=local and qualified reusable inputs; incompatibility is a blocker, not permission to rewrite upstream workflows.
Acceptance: No mutable direct/transitive refs; repo/ref/commit provenance and accepted reusable inputs match. Checker catches missing gates, skip/continue-on-error bypass and drift. Do not launch workflows or modify external repos/settings.
Verification: `node scripts/check-qualification.mjs --mode check --scope ci --record scripts/qualification-inputs.json`
Forecast: 0 source + 0 tests; <=100 declarative YAML lines, existing Q0 negative oracles reused.

- [ ] gentle-Q12 [release] Gate six targets on reproducible checks
Requirements: REQ-DISTRIBUTION-001
Dependencies: ["gentle-Q11"]
Allowed_files: [".github/workflows/release.yml", ".goreleaser.yaml"]
Objective: Apply authentic SHAs/exact GoReleaser patch while preserving six CGO=0 builds and unit/lint/Linux CGO race/production approval. Make required native target checks actual approval/publication prerequisites, not decorative matrix or successful skipped jobs.
Acceptance: Check exact Go 1.26.1/GOTOOLCHAIN=local, lint 2.11.4, schema-2-compatible patch and all six linux/darwin/windows x amd64/arm64 gate edges; missing runners withhold readiness. Preserve `CGO_ENABLED=1 go test -race -count=1 ./...` and production environment. No release --clean/hooks/tag push/publication; telemetry linker flags and secrets unchanged. go.mod already 1.26.1 is read-only; mismatch requires separate review.
Verification: `node scripts/check-qualification.mjs --mode check --record scripts/qualification-inputs.json`
Forecast: 0 source + 0 tests; <=180 declarative YAML lines.

- [ ] gentle-A1 [security] Assess binary inputs with synthetic values only
Requirements: REQ-SECRET-001
Dependencies: ["gentle-P2"]
Allowed_files: ["scripts/assess-release-inputs.mjs", "scripts/assess-release-inputs.test.mjs"]
Objective: Assess observed release linker inputs/consumers through synthetic temporary builds without secret reads or telemetry edits. Distinguish visible embedding/usage from unverified privilege, neither claiming compromise nor harmlessness.
Acceptance: Secret-free self-tests inspect synthetic embedding/usage with scrubbed environment and no network; real-secret checks reject. No release hook/endpoint call/credential redesign; privilege unknown and any out-of-scope reduction returns for authorization.
Verification: `node --test scripts/assess-release-inputs.test.mjs && node scripts/assess-release-inputs.mjs --synthetic-only --isolated-output`
Forecast: 120 source + 150 tests = 270.

- [ ] gentle-W1 [windows] Reproduce handle boundaries without speculative fixes
Requirements: REQ-PLATFORM-001, REQ-VERIFY-002
Dependencies: ["gentle-P3"]
Allowed_files: ["internal/delegation/windows_boundary_smoke_test.go", "internal/homelock/windows_boundary_smoke_test.go", "scripts/windows-boundary.mjs"]
Objective: Run owned ephemeral fixtures against Windows homelock and workspace canonicalization with controlled contention/open/close/rename in temporary state. Keep these boundaries distinct and identify causal locality before hardening; all platform/filemerge product sources remain read-only.
Acceptance: TestWindowsBoundarySmoke must execute in BOTH packages and record operations/toolchain/OS; safe failure must not corrupt target. No-reproduction is explicitly unconfirmed assessment, not fix PASS; reproduced defect/interference exits nonzero and blocks for orchestrator-routed narrow corrective scope. Missing runner/test is unavailable. Runner refuses pre-existing fixture paths, verifies owned creation and removes only these newly created ephemeral files after bounded evidence; no broad persistent tests/retries/reboot cleanup.
Verification: `node scripts/windows-boundary.mjs --require-os windows -- go test -json -count=1 ./internal/homelock ./internal/delegation -run '^TestWindowsBoundarySmoke$'`
Forecast: 90 JS source + 220 Go ephemeral tests (110/file) = 310.

- [ ] gentle-Q3 [qualification] Verify final source on actual target runners
Requirements: REQ-DISTRIBUTION-001, REQ-VERIFY-001, REQ-VERIFY-002, REQ-PLATFORM-001, REQ-UPDATE-001, REQ-INSTALL-001
Dependencies: ["gentle-A1", "gentle-C32", "gentle-I31", "gentle-I32", "gentle-M1", "gentle-Q12", "gentle-R1", "gentle-S3", "gentle-U4", "gentle-W1"]
Allowed_files: ["docs/qualification-inputs.md", "scripts/run-target-qualification.mjs", "scripts/run-target-qualification.test.mjs"]
Objective: Join final source/dependency identities and actual isolated target outcomes without publishing or equating cross-compilation with runtime proof. Verify normalized final bytes only after I20 isolation and retain unavailable targets as blockers to aggregate qualification.
Acceptance: Before execution verify exact versions and I20 substitute coverage; run `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`, `go build -o <owned-temp-output>/cortex-ia ./cmd/cortex-ia`, all persistent Node contract oracles and six CGO=0 builds with recorded command/exit. Linux/amd64 additionally runs `CGO_ENABLED=1 go test -race -count=1 ./...`; Windows race is additional only with supported compiler evidence. Require actual native runtime cases including updater replacement/install effects for all six targets and matching final source/lock/compiler receipts. Missing runner/case/input, skip, drift or failed gate cannot exit 0 in aggregate mode. No real install/signing/publishing or global gofmt; final overlapping approvals require independent refresh before closure.
Verification: `node --test scripts/run-target-qualification.test.mjs && node scripts/run-target-qualification.mjs --isolated --require-go 1.26.1 --require-all-targets --record scripts/qualification-inputs.json && node scripts/check-qualification.mjs --mode check --record scripts/qualification-inputs.json && node scripts/check-test-policy.mjs && node scripts/check-harness-contracts.mjs --mode check --baseline-from-evidence`
Forecast: 150 JS source + 130 tests + 50 Markdown = 330.

## Traceability

All abbreviated task names in this table have prefix gentle-. Ranges are summary notation, not additional records or writable globs.

| REC / Requirement | Exact candidate coverage |
|---|---|
| 01 / REQ-HARNESS-001 | H1 |
| 02 / REQ-HARNESS-002 | K1, H2 |
| 03 / REQ-HARNESS-003 | H3 |
| 04 / REQ-CONTEXT-001 | K1, C1 |
| 05 / REQ-CONTEXT-002 | C0, C21, C22, C23, C24, C25, C26, C27, C28, C29, C30, C31, C32 |
| 06 / REQ-REVIEW-001 | R1, C24, C26 |
| 07 / REQ-STATE-001 | S1, S2, S3 |
| 08 / REQ-DISTRIBUTION-001 | Q0, Q11, Q12, Q3 |
| 09 / REQ-MCP-001 | K1, M0, M1 |
| 10 / REQ-UPDATE-001 | U1, U20, U21, U22, U30, U31, U4, Q3 |
| 11 / REQ-SECRET-001 | A1 |
| 12 / REQ-PLATFORM-001 | W1, Q3; fix only after causal evidence |
| 13 / REQ-VERIFY-001 | P1, P3, C27, I20, Q3 |
| 14 / REQ-VERIFY-002 | P2, P3, C0, W1, Q3 |
| 15 / REQ-INSTALL-001 | I1, I20, I21, I30, I31, I32, Q3 |

## Readiness, concurrency and blocking

There are **46 exact candidate blocks**. No task is currently ready/materialized. Following a separate hash/admission/materialization dispatch only P1 is initially eligible. Then P2; after P2, P3/K1/Q0/M0/A1 have disjoint paths. After P3, R1/S1/I1/I20/U1/W1 are disjoint and may proceed when tools are present; none authorizes unsafe install tests. After K1, H1/C1 are disjoint. M1 joins M0/P3. I21 joins I1/I20, I30 follows I21, and I31/I32 are disjoint after I30. S2 follows S1; S3 joins S2/K1. Updater units serialize on shared updater/trust files. Q11 waits on Q0 and I20, Q12 follows its CI gate contract, Q3 joins all feature/qualification outputs.

C0 waits for P1-derived policy, H3 protocol semantics and C1 context behavior before deduplication freeze. C21/C23/C24/C25/C26/C27/C28/C29/C30/C31 are disjoint after C0 and use stable canonical paths; C22 follows C21 on asset AGENTS. C32 joins all batches and serializes transport edits after H1/H2. C26 can remove cross-contract duplication without making other stable-path migrations depend artificially on it; batch mode validates pending canonical content, final C32 validates everything. R1 has no prompt overlap. Q3's qualification-inputs.md change follows Q0 through Q12. Every overlapping writable file has an executable dependency path.

Likely longest chains: P1->P2->P3->U1->U20->U21->U22->U30->U31->U4->Q3 and P1->P2->K1->H1->H2->H3->C0->C21->C22->C32->Q3. Duration is not inferred from node count. Aggregate forecast is **14,060 source/test/Markdown changed lines**, excluding declarative inputs; no one unit owns the aggregate. Measure per-task churn before review.

Q0/M0/K1 are bounded qualification prerequisites, not proven inputs. No action SHA, exact GoReleaser patch, executable hash or transitive lock is fabricated. Missing provenance/network/package, incompatible schema or mutable upstream workflow keeps prerequisite commands failing and consumers backlog; no fixture-only success, silent upgrades or upstream writes. Q3 withholds qualification for any absent native runner; cross-builds are not runtime evidence. Production roots remain empty regardless of qualification.

W1 is assessment-only: non-reproduction may complete with explicit unconfirmed evidence, never a hardening claim. A reproduced defect blocks completion and requires an exact narrow same-board correction plan under fresh authority; no unlisted product edit is permitted. Oversized implementation/prompt batches likewise route decomposition instead of weakening budgets, thresholds or scope.

## Semantic admission before structural validation

Controller assessment of this task contract: **ADMITTED FOR STRUCTURAL VALIDATION**, not independent implementation/product approval. All 15 unchanged requirements and design-selected positive/edge/failure behaviors map to explicit candidates. Exact C2 batches cover the observed full corpus; exact I3 consumers are source-grounded. I20 migrates both existing unsafe callers and gates I21/recovery/consumer/full-suite work. Every plugin edit waits for locked SDK evidence; CI/MCP pin edits wait for authentic prerequisites. U1 disables unsigned check/apply immediately and passes currentVersion from the actual caller; all subsequent slices retain empty production trust. H2 preserves host/resume identity and rejects duplicated/malformed/multiple budget forms. Dependencies serialize shared files while preserving disjoint groups. No reviewed requirement/design amendment or protected old-board write is needed for these candidate scopes.

This semantic comparison occurs before the separate `cortex_ia_openspec_validate(relative_directory=openspec/changes/gentle-inspired-improvements, workflow=sdd-full, phase=tasks)` call. Its actual structural result belongs in the receipt, not a post-pin checkbox update. Structural coverage is not semantic or product proof. Before materialization, revalidate all eight raw-file hashes and independent review applicability under the same initiative; current dispatch authorizes no board/work creation.
