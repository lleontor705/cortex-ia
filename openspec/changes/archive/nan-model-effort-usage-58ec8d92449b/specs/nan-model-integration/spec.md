# Delta: Nan Model Integration (modelmgr, model command, TUI)

## ADDED Requirements

### Requirement: REQ-NAN-001: Static nan catalog metadata in model catalog receipts
The system MUST expose per-model nan metadata (context tokens, monthly quota tokens, effort vocabulary, effort-adjustable flag, tier, modalities, picker-eligible flag) through the "cortex-ia model catalog" command in both JSON and text receipts. Receipts MUST carry variant identifiers only and MUST NOT surface variant settings, headers, bodies, or API keys. Metadata is additive: non-nan entries keep their current shape.

#### Scenario: Catalog exposes nan metadata
- GIVEN internal/modelmgr holds the static nan model table with glm5.3 quota 3,000,000,000 tokens and effort vocabulary low, medium, high, max
- WHEN an operator runs "cortex-ia model catalog --json --provider nan"
- THEN the JSON receipt contains the glm5.3 entry with a meta object carrying contextTokens, monthlyQuotaTokens, effortVocabulary, effortAdjustable, tier, modalities, and pickerEligible
- AND only variant identifiers are exposed, never settings, headers, bodies, or keys
- AND the text receipt renders the same entry with its effort vocabulary

#### Scenario: Additive for non-nan providers
- GIVEN the acquisition tier returns entries for openrouter and other providers alongside nan
- WHEN the catalog is rendered
- THEN non-nan entries have no meta object and their existing fields are unchanged

#### Scenario: Degraded acquisition stays secret-free
- GIVEN the daemon tier is unreachable and the text tier yields nothing
- WHEN the catalog command runs with --json
- THEN the command exits successfully with an empty entries list and source none
- AND no secret, api key, or variant setting value appears anywhere in the output

### Requirement: REQ-NAN-002: Variant auto-authoring for model set --effort on nan models
For provider nan, when the referenced model entry exists in the OpenCode global config and lacks a variants[] entry with the requested id, "cortex-ia model set <agent> <ref> --effort <level>" MUST author a single variant object with the requested id and settings.reasoningEffort equal to that id, append it to the existing variants list without reordering or dropping existing entries, and set agents.<agent>.model to provider/model#variant, preserving comments and unrelated members through the existing filemerge writer. Requests whose effort is outside the model effort vocabulary MUST fail closed with the valid vocabulary listed and nothing written. Malformed or non-object existing variants structures MUST fail closed as ownership conflicts. The --dry-run flag MUST preview the full outcome without writing. Providers other than nan retain the v1 global-config-only behavior.

#### Scenario: Missing variant authored
- GIVEN opencode.jsonc declares nan model glm5.3 without a variants entry for high and comments present in the file
- WHEN the operator runs "cortex-ia model set orchestrator nan/glm5.3 --effort high"
- THEN the glm5.3 model entry gains a variants object with id high and settings.reasoningEffort high appended to any existing variants
- AND agents.orchestrator.model becomes nan/glm5.3#high
- AND pre-existing comments and unrelated members survive byte-for-byte

#### Scenario: Dry-run previews without writing
- GIVEN the same unauthored state for nan/glm5.3 with effort high
- WHEN the operator runs the identical command with --dry-run
- THEN a machine JSON receipt previews both the authored variant and the agent assignment
- AND no byte of the config file changes

#### Scenario: Effort outside vocabulary rejected
- GIVEN the static nan table lists glm5.3 effort vocabulary as low, medium, high, max
- WHEN the operator runs "cortex-ia model set planner nan/glm5.3 --effort ultra"
- THEN the command fails closed before any write
- AND the error lists the valid vocabulary for glm5.3

#### Scenario: Malformed existing variants structure fails closed
- GIVEN the glm5.3 model entry has a variants value that is not an array of objects
- WHEN the operator runs the set command with a valid effort
- THEN the mutation aborts as an ownership conflict and the file is restored unchanged

### Requirement: REQ-NAN-003: Doctor validation of nan variant effort ids
"cortex-ia model doctor" MUST validate effort identifiers of nan agent references against the static effort vocabulary and MUST emit a WARNING naming the agent, the offending identifier, and the correct vocabulary when validation fails. Doctor MUST remain read-only: it never rewrites configuration or markdown frontmatter, and existing markdown-pin WARNING behavior is preserved.

#### Scenario: Valid effort passes
- GIVEN agents map to nan/glm5.3#high and nan/gemma4#minimal with variants authored
- WHEN the operator runs "cortex-ia model doctor --json"
- THEN a nan-variants check reports severity ok with no rewrite attempted

#### Scenario: Unknown effort identifier warned
- GIVEN an agent reference names nan/glm5.3#ultra
- WHEN doctor runs
- THEN the JSON findings include a warning that names the agent, the identifier ultra, and lists low, medium, high, max as the vocabulary
- AND the exit status remains advisory and the config file bytes are unchanged

#### Scenario: Markdown pin behavior preserved
- GIVEN a markdown agent frontmatter pins a nan model with an invalid effort id and no config entry exists
- WHEN doctor runs
- THEN the existing markdown-pin WARNING is emitted and the frontmatter is never rewritten

### Requirement: REQ-NAN-004: Always-on nan usage strip in session.composer.top
The cortex-ia TUI plugin MUST render a compact one-line usage strip in the session.composer.top slot whenever the current session model is a picker-eligible nan model with known quota. The strip MUST show month-to-date percentage against the catalog monthly quota and the last-24h absolute token burn. The plugin MUST locate the nan executable by probing candidates (it is not on PATH), poll "nan metrics usage" on a 60-second-class cadence with bounded timeout and maxBuffer, honor disposed and generation guards, and MUST hide silently whenever nan is absent, unauthenticated, timed out, or returns malformed JSON.

#### Scenario: Strip renders quota burn
- GIVEN the nan executable resolves and returns valid metrics where monthToDate for the session model totals 1,500,000,000 tokens of a 3,000,000,000 quota
- WHEN the session renders
- THEN the strip shows approximately 50 percent month-to-date plus the last-24h token burn on one line
- AND the strip updates within the poll cadence without blocking input

#### Scenario: Silent degradation when nan is unavailable
- GIVEN the nan executable cannot be resolved or the metrics call fails or times out
- WHEN polls run
- THEN the strip renders nothing and no error banner appears
- AND polling stops cleanly when the plugin disposes

#### Scenario: Unknown quota suppresses percent only
- GIVEN the session model has usage data but the catalog reports monthlyQuotaTokens 0
- WHEN the strip renders
- THEN the month-to-date percent is suppressed instead of dividing by zero
- AND the absolute 24h burn still displays

### Requirement: REQ-NAN-005: Model picker and nan detail layers via registerLayer
The plugin MUST register :model and :nan keymap-layer commands. The :model panel lists picker-eligible nan chat models from "cortex-ia model catalog --json" with effort options constrained to each model effort vocabulary, visually deprioritizing legacy qwen3.6 in favor of deepseek-v4-flash, and it MUST be read-only until the user confirms a target agent plus model plus effort; confirmation MUST show the dry-run preview first and then execute "cortex-ia model set" consuming machine JSON receipts. The :nan panel MUST show a daily-granularity sparkline from metrics timeSeries, per-model month-to-date breakdown against quota, and a static reference note for the glm5.3 rolling-4h allowance stating that a rolling window cannot be computed from daily data.

#### Scenario: Confirmed assignment path
- GIVEN the :model panel lists glm5.3-flash with vocabulary low, medium, high, max
- WHEN the user selects implement plus glm5.3-flash plus medium and confirms
- THEN the panel shows the dry-run receipt then applies "model set implement nan/glm5.3-flash --effort medium" and displays the final machine receipt status

#### Scenario: Detail panel reference note
- GIVEN metrics timeSeries contains daily points for several models
- WHEN the user opens :nan
- THEN the sparkline renders daily totals per model with month-to-date percent against quota
- AND the rolling-4h allowance appears as a static reference note, never as a computed window

#### Scenario: Apply failure keeps state unchanged
- GIVEN the model set command exits nonzero after a confirmed assignment
- WHEN the panel receives the failure
- THEN the panel surfaces the error text and leaves the previous effective mapping displayed

#### Scenario: Catalog unavailable in panel
- GIVEN "cortex-ia model catalog --json" fails inside the plugin environment
- WHEN the user opens :model
- THEN the panel shows an empty-state message and performs no assignment
