# Plan: Install Custom Providers (add-custom-providers)

Integrated sdd-lite contract. Spec plane: hybrid (OpenSpec markdown here is the shared contract; Cortex holds durable evidence). Workload policy: flexible. Control plane: board custom-providers, mirrored 1:1 by tasks.md. Companion artifacts: proposal.md (problem, value, non-goals, risks), specs/providers/spec.md (normative Given/When/Then scenarios), design.md (architecture, data models, interfaces, sequences, risk analysis).

## 1. Intent & Non-Goals

Intent: add an "Install custom provider" TUI flow that lists provider catalog JSON files from ~/.cortex-ia/providers/*.json (embedded Nan seed written on first access), shows each provider's models and effort vocabularies, collects the API token through masked input, previews a dry-run, and writes the provider block into the user's OpenCode config (opencode.jsonc winning over opencode.json) with backup, ownership accreditation, modelmgr-consistent variants, and strict secret hygiene.

Non-goals (v1 boundaries): no CLI provider command family (TUI-first, follow-up F1); no modelmgr refactor and no nan_catalog.go removal (F2); no object-map variants support and no repo-side custom variants parser — the modelmgr-authored array shape is pinned canonical (risk R1 with a live review check); no secret storage beyond inline options.apiKey; no web console surface (F3); no compaction advice (F4).

## 2. Requirements

Normative scenarios live in specs/providers/spec.md under its ADDED Requirements delta section. Traceability per rule patterns/agent-testing-gates (D6 TestREQ naming):

| REQ | Subject | Owning tasks | Test oracle |
|---|---|---|---|
| REQ-PROV-001 | Catalog seed, load, typed validation | cp-01-catalog | TestREQ_PROV_001_* |
| REQ-PROV-002 | Full model set with modelmgr-shape variants | cp-02-state, cp-03-install | TestREQ_PROV_002_* |
| REQ-PROV-003 | Token is transient secret, masked input | cp-03-install, cp-04-screen | TestREQ_PROV_003_* |
| REQ-PROV-004 | Twin and unmanaged conflicts mutate nothing | cp-03-install | TestREQ_PROV_004_* |
| REQ-PROV-005 | TUI List-Input-Preview-Receipt machine | cp-04-screen, cp-05-home, cp-06-flow | TestREQ_PROV_005_* |
| REQ-PROV-006 | 3-place home registration, hotkey p, numerics frozen | cp-05-home, cp-06-flow | TestREQ_PROV_006_* |

## 3. Design

Condensed (full detail, data models, interfaces, sequence, and risks R1-R5 in design.md): a new leaf package internal/providermgr owns catalog types, the embedded seed/nan.json, CORTEX_IA_HOME-aware state-root resolution, idempotent seeding, typed InvalidCatalogError, and deep-copy accessors. internal/state gains an additive, omitempty Providers family (ProviderV2 records, no schema_version bump). internal/install gains provider.go exposing ProviderCatalog, ProviderPreview (dry-run), and ProviderInstall: resolve winning config plus twin conflict, refuse unmanaged blocks, build the provider-subtree __replace__ overlay with all catalog models and effort variants, then run the Backup, Update config, Commit state transaction through filemerge.MutateJSONFile with a secret-free identity-only SemanticDigest. The TUI adds masked_input.go (manual rune masking, bullets, zero new direct dependencies) and providers_screen.go (models_screen phase machine over package-var seams), wired into Home at entry index 9 behind hotkey p with numeric keys 1-9 frozen.

## 4. Tasks & Verification Strategy

Same-board DAG on board custom-providers, six nodes, mirrored 1:1 in tasks.md:

- Wave 1 (parallel, disjoint files): cp-01-catalog (no deps), cp-02-state (no deps)
- Wave 2: cp-03-install (deps cp-01-catalog, cp-02-state)
- Wave 3: cp-04-screen (dep cp-03-install)
- Wave 4: cp-05-home (dep cp-04-screen)
- Wave 5: cp-06-flow (dep cp-05-home) — integration and authority regression oracle

Verification strategy:
- Each task carries raw, standalone focused TestREQ go test commands as listed in tasks.md; implementers run them before transitioning to in_review.
- Mutation Evidence Gate (cortex-work-protocol.md sections 4 and 8; harness rule 9; rule patterns/agent-testing-gates): cp-01 through cp-05 are fast-TDD-eligible code tasks; each receipt must record mutation-probe results over the covering branches; a SURVIVED outcome blocks the in_review transition until the covering test is strengthened. cp-06 is a pure-test task and is never decomposed.
- Workload (flexible): Go source at most 700 LOC and tests/fixtures at most 1200 LOC per task; per-task budgets tighter than the policy cap are recorded in tasks.md; all new test files are dedicated modular files at most 250 LOC and nothing appends to a test file above 300 LOC.
- Temp homes always: every provider test uses a temporary home plus CORTEX_IA_HOME override; the developer's real OpenCode or Cortex state is never targeted; synthetic tokens only in fixtures.
- Review phase additionally runs the live OpenCode variant-visibility check mandated by R1 and the repo full gate: gofmt -s -w ., go vet ./..., golangci-lint run ./..., go test -count=1 ./....
- Token audit invariant for review: grep receipts, state.json, logs, and test fixtures for the sentinel token value; any hit is a blocker (REQ-PROV-003).
