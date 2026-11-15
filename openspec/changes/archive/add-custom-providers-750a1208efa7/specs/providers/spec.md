# Spec Delta: providers (add-custom-providers)

Domain: provider installation into OpenCode config through cortex-ia. RFC 2119 keywords apply. Every requirement lists Given/When/Then scenarios; traceability maps each REQ to tasks in tasks.md.

## ADDED Requirements

### Requirement: REQ-PROV-001 Catalog seeding is idempotent and validation is typed
The system shall maintain one JSON catalog file per provider under the providers/ directory of the Cortex-IA state root (default ~/.cortex-ia/providers, resolved honoring CORTEX_IA_HOME exactly as cortexStateHome does). First access shall materialize the embedded nan.json seed only when the file is absent, with 0644 permissions. Malformed files shall fail closed with InvalidCatalogError naming provider, field, and reason without echoing raw values, and shall never be silently overwritten. Accessors shall return defensive copies.

#### Scenario: Seed an empty state root
- GIVEN no providers directory under an isolated CORTEX_IA_HOME state root
- WHEN the provider catalog is loaded
- THEN providers/nan.json is materialized with 0644 and the Nan provider returns with all eight declared models

#### Scenario: CORTEX_IA_HOME is honored
- GIVEN CORTEX_IA_HOME points to a temporary state root
- WHEN seeding and loading run
- THEN nothing is created under the real user home and the catalog is read from the override root

#### Scenario: Malformed catalog rejects typed
- GIVEN providers/nan.json declares an unknown schema version or a non-array efforts field
- WHEN the catalog loads
- THEN InvalidCatalogError names the provider, field, and reason, echoes no raw value, and the file is left untouched

#### Scenario: Accessors deep-copy
- GIVEN a caller mutates a slice returned by a catalog accessor
- WHEN the provider is read again
- THEN the values match the seeded catalog unchanged

### Requirement: REQ-PROV-002 Provider install materializes the full model set with variants
The installer shall write the provider block into the winning OpenCode config (JSONC wins over JSON per modelmgr ConfigPath and Layout.ResolveConfigRelPath precedence) as provider.id containing npm, display name, options.baseURL, options.apiKey, and every model declared in the catalog. Models with a non-empty effort vocabulary shall carry a variants array of objects with id and settings.reasoningEffort, the exact shape internal/modelmgr authors (planNanVariants and nanVariantObject); models with an empty vocabulary get no variants key. The write shall go through filemerge.MutateJSONFile with a __replace__ overlay on the provider subtree (comments preserved byte-exactly, duplicate members rejected), preceded by a verified backup and followed by a state ownership record with an identity-only semantic digest.

#### Scenario: Install writes every catalog model
- GIVEN the Nan catalog with eight models and a valid token
- WHEN a confirmed provider install runs
- THEN provider.nan in the winning config contains the npm package, name, baseURL, options.apiKey, and all eight model entries

#### Scenario: Effort vocabulary becomes variants
- GIVEN a catalog model declares the efforts low, medium, high, and max
- WHEN the install writes that model entry
- THEN its variants array holds exactly one id-plus-settings.reasoningEffort object per effort in the pinned modelmgr shape

#### Scenario: Comments and unrelated members survive
- GIVEN the winning config contains JSONC comments and unrelated provider or model blocks
- WHEN the install overlay applies
- THEN comments and unrelated members remain byte-identical and only the provider.nan subtree changes

#### Scenario: Ownership record without secrets
- GIVEN an install completes
- WHEN state.json is read back
- THEN a Providers record exists with provider name, config path, model list, backup id, and a semantic digest computed without the API key

#### Scenario: Reinstall converges
- GIVEN a prior installer-owned provider.nan record exists
- WHEN a reinstall with a rotated token completes
- THEN the provider subtree is replaced wholesale with no orphaned models and the record and backup id update

### Requirement: REQ-PROV-003 The token is a transient secret
The API token shall exist only as runes in screen state and as a call argument into the service. Input shall be masked (bullet rendering, fixed width), the token shall be written inline to provider.id.options.apiKey (operator-approved shape), and it shall never appear in receipts, previews text, logs, digests, state records, Cortex observations, verification commands, or test fixtures.

#### Scenario: Token input renders bullets
- GIVEN the providers screen is in the token Input phase
- WHEN the user types the token characters
- THEN the view renders only bullet glyphs of matching width and no raw character appears in any rendered output

#### Scenario: Token travels to config only
- GIVEN a confirmed install carries a token value
- WHEN the service returns its receipt and commits state
- THEN the value appears in exactly one persisted artifact, the winning config options.apiKey, and in no other file or output

#### Scenario: Digest excludes secrets
- GIVEN a semantic digest is computed over a provider entry
- WHEN the entry contains options.apiKey or headers
- THEN those fields are stripped before hashing so token rotation leaves the digest unchanged

### Requirement: REQ-PROV-004 Twin-file duplication reconciles; unmanaged winner blocks refuse
When the winning config is resolved and the provider id is also defined in the non-winning twin file, or is defined with differing content across the twins while the winner copy is installer-owned, the install shall reconcile the split-brain automatically instead of refusing, in this strict order: capture the verified backup FIRST so it protects both config files; remove the stale provider.id block from the non-winning twin atomically via the filemerge MutateJSONFile JSONMutation.RemovePaths edit; apply the __replace__ overlay to the winner; commit ownership. ProviderPreview shall disclose the planned reconciliation (twin path plus block removal plus winner write) while writing nothing, and the receipt shall disclose both mutations under the single backup id. If the twin edit or the winner overlay fails after the backup, the transaction shall restore both config files from the verified backup and leave state.json untouched. An unmanaged provider.id block in the winner (not covered by a Providers ownership record) shall remain a typed refusal, ErrProviderUnmanaged, that mutates nothing.

#### Scenario: Twin copy reconciled backup-first
- GIVEN opencode.jsonc is the winner and lacks provider.nan while the twin opencode.json defines provider.nan
- WHEN the install runs
- THEN the verified backup is captured before any edit, the stale provider.nan is removed from opencode.json via the RemovePaths edit, the winner receives the __replace__ overlay, ownership is committed, and the receipt names both mutations under one backup id

#### Scenario: Divergent twins converge
- GIVEN provider.nan appears in both opencode.json and opencode.jsonc with differing content and the winner copy carries an ownership record
- WHEN the install runs
- THEN the backup-first ordering removes the twin block and converges the winner to the full catalog definition leaving exactly one managed provider.nan

#### Scenario: Dry-run discloses reconciliation without writes
- GIVEN resolution finds provider.nan duplicated in the non-winning twin
- WHEN ProviderPreview runs
- THEN the dry-run reports the planned twin removal, winner write, and backup-first ordering while every file mtime remains unchanged

#### Scenario: Failed reconciliation restores both files
- GIVEN the backup is captured and either the twin RemovePaths edit or the winner overlay fails
- WHEN the transaction unwinds
- THEN both config files are restored byte-identical from the verified backup and state.json remains uncommitted

#### Scenario: Unmanaged winner block still refuses
- GIVEN provider.nan exists in the winner without a covering Providers ownership record
- WHEN an install is attempted
- THEN the typed ErrProviderUnmanaged refusal mutates nothing, because reconciliation never covers winner-side unmanaged content

#### Scenario: Solo twin proceeds untouched
- GIVEN only opencode.json exists and does not define provider.nan
- WHEN an install resolves the config
- THEN opencode.json is selected as the target, no reconciliation is planned, and the normal transaction proceeds

### Requirement: REQ-PROV-005 The TUI drives the install through a phase machine
The providers screen shall replicate the models_screen.go phase machine List then Input then Preview then Receipt (with an Error surface when the catalog fails to load) over package-var seams wrapping the install service so tests run against temporary homes. List shows catalog providers with each model's effort vocabulary; Input collects the masked token; Preview renders a dry-run (target config path, provider id, model and variant counts, planned action, and the planned twin reconciliation when present) with zero filesystem writes; confirmation executes the service flow through the Running display with phases Backup, Update config, Commit state; the Receipt never renders the token.

#### Scenario: List reflects the catalog
- GIVEN the catalog loads with the Nan provider
- WHEN the user opens Install custom provider
- THEN the list shows nan and its eight models with each effort vocabulary displayed

#### Scenario: Preview is inert
- GIVEN a masked token has been entered
- WHEN the Preview phase renders
- THEN no write to config, state, or providers directory occurs and Escape returns to Input

#### Scenario: Confirmation runs the transactional phases
- GIVEN the user confirms the preview
- WHEN the service flow completes
- THEN the Running display passes through Backup, Update config, Commit state and the Receipt reports action, config path, model count, and backup id

#### Scenario: Catalog failure degrades safely
- GIVEN catalog loading fails
- WHEN the screen renders the Error surface
- THEN only Escape to Home is offered and the Input phase never renders with an empty catalog

### Requirement: REQ-PROV-006 Home registration is 3-place with a letter hotkey
The new entry "Install custom provider" shall be registered at homeEntries index 9 across all three places (homeEntries in model.go, homeDescriptions in views.go, the selectHomeEntry switch) and opened by the hotkey p or P in updateHome. Numeric keys 1 through 9 keep their existing mappings to entries 0 through 8; the key 0 stays reserved and unbound; letter hotkeys are the scheme for every entry beyond index 8.

#### Scenario: Hotkey p opens the providers screen
- GIVEN the TUI is on Home
- WHEN the user presses p
- THEN the active screen becomes the providers List phase

#### Scenario: Numeric keys are unchanged
- GIVEN the Home menu renders ten entries
- WHEN keys 1 through 9 are pressed
- THEN each selects the same entry index it selected before this change

#### Scenario: Three-place registration is complete
- GIVEN homeEntries has length ten
- WHEN the Home view renders and the user navigates with up, down, and enter
- THEN every entry including index 9 renders a description and activates without panic or collision

#### Scenario: Existing models hotkey survives
- GIVEN the TUI is on Home
- WHEN the user presses m
- THEN the models screen opens exactly as before the change

## Traceability
Each REQ maps to at least one task in tasks.md with an exact Requirements reference; every task verification command targets TestREQ names.
