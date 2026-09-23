# Model Command — spec delta

## ADDED Requirements

### Requirement: REQ-MODEL-001 — Top-level `model` command surface
The CLI SHALL provide a top-level `cortex-ia model` command, a strict sibling of `cortex-ia mcp`, with subcommands `list`, `get`, `set`, `unset`, and `doctor`. The grammar SHALL use only `--effort`, `--json`, and `--dry-run` flags; no flag or option name beginning with the retired `--model` prefix SHALL appear in any documented grammar, and `preflightCLI` SHALL continue to reject such arguments app-wide before dispatch.

#### Scenario: set with effort reports the compact form
- **GIVEN** an installed home with an agreed v2 state
- **WHEN** the user runs `cortex-ia model set plan anthropic/claude-sonnet-4-5 --effort high --dry-run`
- **THEN** the receipt reports the canonical compact string `anthropic/claude-sonnet-4-5#high` and no configuration file is written

#### Scenario: retired --model prefix remains rejected
- **GIVEN** any invocation
- **WHEN** the command line contains an argument starting with `--model` (for example `cortex-ia model set plan x --model-y`)
- **THEN** `preflightCLI` fails closed with `RetiredSurfaceError` before any state is read

#### Scenario: unknown subcommand fails with usage
- **GIVEN** the `model` command surface
- **WHEN** the user runs `cortex-ia model frobnicate`
- **THEN** the CLI exits with an error naming the valid subcommands `list, get, set, unset, doctor`

### Requirement: REQ-MODEL-002 — Free-form model reference shape validation
The manager SHALL validate model references as free-form `provider/model` with an optional `#variant` suffix (effort): non-empty `provider` and `model` tokens containing no whitespace, `#`, or control characters, and a variant token that is non-empty and free of whitespace and control characters. The canonical stored form SHALL be the compact string `provider/model#variant`; the expanded object form `{providerID, model, variant}` SHALL be accepted when reading existing config and re-serialized compactly. No embedded provider/model catalog SHALL exist, and no deprecated field (`temperature`, `top_p`, `prompt`, `permission`, `tools`, `disable`, `maxSteps`) or `request.body` overlay SHALL ever be written.

#### Scenario: valid compact reference accepted
- **GIVEN** the set subcommand
- **WHEN** the model argument `anthropic/claude-sonnet-4-5#high` is parsed
- **THEN** the desired reference validates and encodes to the identical compact string

#### Scenario: effort flag overrides the variant suffix
- **GIVEN** the set subcommand with both a `#variant` suffix and `--effort`
- **WHEN** the desired reference is built
- **THEN** the `--effort` value overrides the suffix variant and the receipt states the final compact string

#### Scenario: malformed reference rejected before any access
- **GIVEN** the set subcommand
- **WHEN** the model argument is `anthropic/` or `claude sonnet` or `anthropic/claude#`
- **THEN** the CLI fails closed with a shape-validation error before any config read or state access

#### Scenario: expanded object re-serialized compactly
- **GIVEN** an existing config where `agents.plan.model` is the object `{"providerID": "anthropic", "model": "claude-sonnet-4-5", "variant": "high"}`
- **WHEN** `cortex-ia model get plan` runs
- **THEN** the effective model is reported as the compact string `anthropic/claude-sonnet-4-5#high`

### Requirement: REQ-MODEL-003 — Case-folded agent resolution registry
The manager SHALL resolve agent names against a case-folded registry combining the builtin agents (`build`, `plan`, `general`, `explore`), the keys of the config `agents` object, and markdown agent files under `<configRoot>/agents/*.md`. An operation targeting an agent absent from the registry SHALL fail closed with a typed conflict naming the agent and the resolution sources. A markdown agent whose frontmatter pins a `model` SHALL remain settable: the operation SHALL succeed, the receipt SHALL note the frontmatter value, and `model doctor` SHALL report the pin as a WARNING finding (never a blocker).

#### Scenario: unknown agent fails closed
- **GIVEN** a home with no `agents` entry, no markdown file, and no builtin named `ghost`
- **WHEN** `cortex-ia model set ghost anthropic/claude-sonnet-4-5` runs
- **THEN** the operation fails closed with a typed conflict, nothing is written, and the error lists the resolution sources

#### Scenario: case-insensitive resolution preserves config spelling
- **GIVEN** a config with an `agents` key `Backend`
- **WHEN** `cortex-ia model get backend` runs
- **THEN** the agent resolves case-folded to `Backend` and the receipt preserves the config key spelling

#### Scenario: markdown-pinned agent is settable with a visible note
- **GIVEN** a markdown agent `reviewer.md` whose frontmatter pins `model: openai/gpt-5.2`
- **WHEN** `cortex-ia model set reviewer anthropic/claude-sonnet-4-5` runs
- **THEN** the config `agents.reviewer.model` entry is written and the receipt includes a note that the markdown frontmatter also pins `openai/gpt-5.2`

### Requirement: REQ-MODEL-004 — Transactional set/unset mutation semantics
`model set` and `model unset` SHALL mutate `agents.<agent>.model` exclusively through the install service transactional recipe: real runs gate on `ErrNotInstalled`, acquire the canonical home lock, reload state and sidecar under the lock, run inside a service transaction with a verified backup, and commit ownership records and namespaced fingerprint sidecar records together. Config writes SHALL go through `filemerge.MutateJSONFile` only (comment-preserving, atomic, duplicate-member rejecting) — never `MergeJSONObjects`. `--dry-run` SHALL report the planned action and resulting compact value without locking or writing. Receipts SHALL disclose the previous effective value, the action taken, the config path, the backup ID, and warnings; `--json` SHALL emit the same facts as a machine-readable document. On `unset`, only the `model` member SHALL be removed, and an emptied agent entry that cortex-ia created SHALL be pruned.

#### Scenario: set writes only the model member preserving comments
- **GIVEN** an installed home whose `opencode.jsonc` contains comments and an `agents.plan` entry with a `description` member
- **WHEN** `cortex-ia model set plan anthropic/claude-sonnet-4-5#high` commits
- **THEN** the file differs only in the `agents.plan.model` member, all comments and other members survive, and a verified backup ID is reported

#### Scenario: dry-run never writes or locks
- **GIVEN** an installed home
- **WHEN** `cortex-ia model set build openai/gpt-5.2 --dry-run` runs
- **THEN** the receipt reports the planned action and compact value, the config bytes and mtime are unchanged, and the home lock is not acquired

#### Scenario: unset on an uninstalled home fails closed
- **GIVEN** a home without agreed v2 installation metadata
- **WHEN** `cortex-ia model unset plan` runs
- **THEN** the operation fails with the `ErrNotInstalled` guidance to run install first and nothing is written

#### Scenario: set overwrites a previous value with full disclosure
- **GIVEN** a managed `agents.build.model` equal to `openai/gpt-5.2`
- **WHEN** `cortex-ia model set build anthropic/claude-sonnet-4-5` runs
- **THEN** the mutation succeeds, the receipt reports the previous value `openai/gpt-5.2`, and the ownership record is updated

### Requirement: REQ-MODEL-005 — Honest list/get reporting
`model list` and `model get` SHALL work on any home, installed or not, and SHALL report for each agent the effective model reference, its variant (effort), and its source: managed config entry (with ownership state), plain config entry, markdown frontmatter pin, or unset. Output SHALL be sanitized (model references and agent names only, never secrets) and `--json` SHALL produce a stable machine-readable document.

#### Scenario: list on an uninstalled home is honest
- **GIVEN** a home with a config but no agreed v2 installation metadata
- **WHEN** `cortex-ia model list` runs
- **THEN** the listing renders the agents with their effective models and marks ownership as unavailable instead of failing

#### Scenario: get reports source and variant
- **GIVEN** a managed `agents.plan.model` equal to `anthropic/claude-sonnet-4-5#high`
- **WHEN** `cortex-ia model get plan --json` runs
- **THEN** the JSON document names agent `plan`, model `anthropic/claude-sonnet-4-5`, variant `high`, and source `managed`

#### Scenario: unset agent is reported without error
- **GIVEN** a registry agent with no config model entry and no markdown pin
- **WHEN** `cortex-ia model get plan --json` runs
- **THEN** the JSON document reports agent `plan` with an unset model, source `unset`, and a successful exit

#### Scenario: output is sanitized
- **GIVEN** any `model list` or `model get` invocation
- **WHEN** the report is rendered in text or JSON form
- **THEN** only agent names, model references, variants, sources, and ownership states appear — never secrets, tokens, or unrelated config values

### Requirement: REQ-MODEL-006 — Ownership records, drift detection, and lifecycle integration
Managed agent-model entries SHALL be recorded additively in MetadataV2/LockV2 (`AgentModels`: agent, config path, versioned semantic digest, ownership) with a domain-separated versioned digest in `internal/installmeta` (`amdv1:` prefix over a canonical secret-free identity), agreement-checked between state and lock exactly like MCPs. Fingerprint sidecar records SHALL be namespaced `agent-model/<agent>` inside the existing sidecar document. Uninstall and rollback SHALL enumerate managed agent-model entries alongside MCPs with the same verified-backup guarantees. The embedded `internal/assets/opencode.jsonc` template SHALL never declare `agents` or `model` keys, enforced by a persistent regression test.

#### Scenario: state and lock agree on managed entries
- **GIVEN** a committed `model set` transaction
- **WHEN** the v2 state and lock documents are validated
- **THEN** both contain the same normalized `AgentModels` records and the agreement validation passes

#### Scenario: uninstall removes managed model entries
- **GIVEN** an installed home with a managed `agents.plan.model` entry
- **WHEN** `cortex-ia uninstall` runs with confirmation
- **THEN** the managed model entry is removed or restored per the transactional recipe together with its ownership and sidecar records, and unrelated user config survives

#### Scenario: template regression guard holds
- **GIVEN** the repository asset set
- **WHEN** the persistent regression test inspects the embedded `opencode.jsonc`
- **THEN** it fails if the template declares `agents` or `model` keys, because installer safe-merge makes template keys win and would silently pin models

### Requirement: REQ-MODEL-007 — `model doctor` diagnostics
`model doctor` SHALL report, with typed severities and `--json` output: JSONC decode health and duplicate-member detection of the config; shape validation of every `agents.*.model` value; drift between recorded managed digests and observed values; markdown-frontmatter model pins and conflicts (WARNING); `default_agent` referencing an existing visible primary agent (WARNING when violated); the embedded-template regression check result; and an optional cross-check against `opencode2 models` output that SHALL be skipped non-fatally when the binary or provider is unavailable. Doctor SHALL be honest on uninstalled homes and SHALL never mutate anything.

#### Scenario: drifted managed entry is reported
- **GIVEN** a managed entry whose observed config value differs from the recorded digest
- **WHEN** `model doctor` runs
- **THEN** a drift finding names the agent with the expected and observed digests at WARNING severity or higher, and nothing is mutated

#### Scenario: opencode2 unavailable is non-fatal
- **GIVEN** a home where the `opencode2` binary is not on PATH
- **WHEN** `model doctor` runs
- **THEN** the cross-check is reported as skipped, the remaining findings are still produced, and the command does not fail because of the skip

#### Scenario: malformed config blocks nothing else
- **GIVEN** a config with a duplicate `agents` member
- **WHEN** `model doctor` runs
- **THEN** the decode-health finding names the duplicate member as an ERROR while every other check still reports its status honestly

### Requirement: REQ-MODEL-008 — TUI models configuration screen
The zero-arg Bubble Tea TUI SHALL provide a models configuration screen built as a self-contained `modelsState` with a screen-level `modelsAction` seam (mirroring the stats screen), listing agents with effective model/variant and source, supporting free-form model input and effort/variant selection with a dry-run preview, and persisting exclusively through the install service path used by the CLI — never ad-hoc file writes. Screen loading and persistence SHALL flow through injectable seams so tests run against temporary homes and synthetic inputs only; persistent TUI tests SHALL keep dedicated files ≤ 250 LOC.

#### Scenario: screen lists agents with effective models
- **GIVEN** a TUI session over a home with builtin, markdown, and config agents
- **WHEN** the models screen renders
- **THEN** every registry agent appears with its effective compact reference, variant, and source, and unset agents show their unset status

#### Scenario: persistence flows through the service path
- **GIVEN** the models screen with a selected agent and a free-form model input
- **WHEN** the user confirms the change
- **THEN** the screen invokes the same install service mutation as the CLI with a dry-run preview first, renders the resulting receipt or typed conflict in-screen, and never writes the config directly

#### Scenario: screen seams keep tests off real state
- **GIVEN** the persistent TUI tests
- **WHEN** the models screen suite runs
- **THEN** loading and persistence go through injected seam functions over temporary homes, and no test reads or writes the developer's real OpenCode configuration
