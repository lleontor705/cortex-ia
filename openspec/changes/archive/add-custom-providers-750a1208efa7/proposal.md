# Proposal: Install Custom Providers

Change: add-custom-providers | Workflow: sdd-lite | Spec plane: hybrid | Workload: flexible

## Problem
cortex-ia installs OpenCode assets and manages MCPs, but a custom model provider such as Nan (nan.builders) must still be hand-edited into the user's opencode.jsonc. Meanwhile internal/modelmgr already authors and reads the provider.nan per-model reasoning-effort variants but deliberately refuses to create the provider definition itself — "rather than writing a provider definition it cannot trust" (internal/modelmgr/manager.go:443-464). The trustworthy writer half of that contract is missing.

## User value
One TUI flow, "Install custom provider": list the provider catalog JSON files (~/.cortex-ia/providers/*.json, seeded with an embedded Nan catalog), show the provider's models and effort vocabularies, ask for the API token through masked input, preview a dry-run, then write the provider block into the user's OpenCode config with backup and ownership accreditation. First provider shipped: Nan (npm @ai-sdk/openai-compatible, base https://api.nan.builders/v1, 8 chat models verified on nan.docs 2026-09-26).

## Approach
1. New leaf package internal/providermgr: catalog types, embedded seed/nan.json, idempotent seeding into the providers/ folder of the Cortex-IA state root (CORTEX_IA_HOME honored the way cortexStateHome does in internal/app/work.go:883), strict validation with typed InvalidCatalogError that never echoes values, deep-copy accessors (mirrors internal/mcpmanager presets.go + desired.go).
2. install.Service provider transaction (new internal/install/provider.go): config resolution with JSONC-wins-else-JSON precedence, twin-file split-brain conflict detection, backup, filemerge.MutateJSONFile __replace__ overlay writing ALL catalog models with modelmgr-shaped effort variants, ownership committed as a new additive state v2 family Providers, identity-only secret-free SemanticDigest.
3. TUI: providers_screen.go phase machine copied 1:1 from the models_screen.go precedent (List to Input to Preview to Receipt) over package-var seams; masked token input via a small manual rune-masking helper (no masked input exists in the repo today); Home menu registration in the 3 existing places with a new letter hotkey p (numeric keys 1-9 are saturated).
4. Locked by the operator, not reopened here: inline options.apiKey token, all-models install, modelmgr array variants shape canonical, docs variant-shape discrepancy tracked as a risk with a live verification check in review.

## Non-goals (v1 boundaries, anti-cascade-amplification)
- No CLI provider command family; TUI-first (follow-up F1).
- No modelmgr refactor and no removal of internal/modelmgr/nan_catalog.go (its drift is follow-up F2).
- No repo-side custom parser for the object-map variants form shown by opencode.ai/docs/models; the modelmgr-consistent array form is pinned canonical (risk R1, live check required at review).
- No secret storage beyond the inline options.apiKey in the OpenCode config; no keychain, no env indirection.
- No web console provider surface (F3); no compaction block advice (F4).

## Follow-ups (recorded, not built here)
1. CLI provider add/list/remove family reusing the same service methods.
2. Extend model catalog to read the file catalog, fixing nan_catalog.go drift (mimo-v2.6-flash is missing there today).
3. Read-only provider projection in the embedded web console.
4. Advice surface for compaction block variants.

## Risks
- R1 Variants shape discrepancy with official docs: array form (repo canonical, both modelmgr writer and read-side assume it) versus object-map form (docs). Mitigation: pinned canonical shape in design.md plus a mandatory live check of variant visibility via the OpenCode models listing during review; not repo-side parsing.
- R2 Twin-file split-brain: the live user environment has provider.nan defined in BOTH opencode.json and opencode.jsonc. Mitigation: typed conflict, mutate nothing (REQ-PROV-004).
- R3 TUI home numeric-key saturation (1-9 exhausted). Mitigation: letter hotkey scheme; p for providers; numerics frozen (REQ-PROV-006).
- R4 Token leakage: token is inline in config by operator choice; hard invariant that it never reaches receipts, logs, digests, state, Cortex observations, command strings, or test fixtures; masked input required (REQ-PROV-003).

## Success criteria
OpenSpec validation passes (sdd-lite/integrated); board custom-providers DAG (6 tasks) approved by the operator in interactive mode; all TestREQ oracles green with mutation evidence per cortex-work-protocol.md sections 4/8; live OpenCode check confirms installed models and variants are visible.
