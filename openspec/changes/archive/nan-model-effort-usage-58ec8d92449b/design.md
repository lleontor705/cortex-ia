# Design: Nan Model Selection, Effort, and Usage Bar

## Context and seams

- Dispatcher internal/app/model.go stays parse-and-render only (hard invariant). All ownership, validation, and mutation decisions remain in internal/modelmgr.
- Persistence seam: filemerge.MutateJSONFile writes agents.<agent>.model today; variant authoring extends the same single mutation plan, not a second writer.
- TUI seam: internal/tuiassets/cortex-ia-tui.tsx is built by tsup into internal/assets/tui/cortex-ia-tui.js (go:embed, installed to tui-plugins). Slot session.composer.top is unused (zero conflict). Polling pattern copies the existing stats poller (execFile of cortexExecutable() every 60s with generation/disposed guards). Reusable primitives: MultiColorProgressBar, formatLargeTokens.
- Nan CLI seam: binary is NOT on PATH; resolver probes candidates like cortexExecutable(): %LOCALAPPDATA%/Programs/nan/nan.exe, then plain "nan". "nan metrics usage" emits pure pretty JSON with no --json flag.

## Data models

internal/modelmgr/nan_catalog.go (new) defines:

- NanModelMeta { Model, Tier, ContextTokens, MonthlyQuotaTokens, Rolling4hRefTokens, EffortVocabulary []string, EffortAdjustable, Modalities, PickerEligible, PreferredOver }
- Static table rows for the seven ratified chat models: glm5.3 (premium, quota 3,000,000,000, rolling4hRef 400,000,000, vocab low/medium/high/max, adjustable), glm5.3-flash (2,000,000,000, adjustable, text+image), deepseek-v4-flash (3,000,000,000, effort ignored adaptive, preferred over qwen3.6), qwen3.8-flash (500,000,000, accepted-but-fixed depth, ctx 262144), mimo-v2.5 (1,000,000,000, accepted-not-adjustable, audio, max_tokens>=300 guidance), gemma4 (vocab none/minimal/low/medium/high/max with token budgets, ctx 262144), qwen3.6 (gemma4 vocabulary, legacy, deprioritized in picker).
- Non-chat nan models (embedding, rerank, kokoro, whisper, flux-2-klein) are NOT picker candidates; they may appear with pickerEligible false for doctor checks only.

CatalogEntry gains Meta *NanModelMeta json:"meta,omitempty". For provider nan, entries merge acquisition variants (ids only) with static meta. JSON and text receipts render meta additively. The daemon tier keeps dropping variant settings/headers/body as secret carriers; static authoring is the only place settings.reasoningEffort is ever written.

## Authoring algorithm (model set)

1. Parse as today: ParseDesired maps --effort to Variant.
2. If Provider == nan and Variant != "": validate Variant against NanModelMeta.EffortVocabulary; unknown level fails closed listing the vocabulary.
3. Read current config (loadConfig). Locate the provider model entry. If absent, behavior is the v1 contract: assignment only, no authoring (OpenCode resolution governs).
4. If the entry variants array lacks the requested id and every existing element is an object, compute the full replacement variants array (existing entries preserved verbatim plus {id: level, settings: {reasoningEffort: level}} appended). Any non-object or structurally malformed variants value aborts as an ownership conflict (fail closed, nothing written).
5. Emit one JSONMutation (overlay including both the provider model update and agents.<agent>.model) through the existing Set/planSet path so the verified backup and restore-on-failure semantics of InstallV2-style atomic apply are unchanged. InspectSet/dry-run previews the identical plan byte-for-byte with zero writes.
6. Ownership metadata (state v2 AgentModels digests) still covers only the agents mapping; authoring is disclosed in the receipt (authoredVariants field).

## Doctor

New check id nan-variants: for every agent whose model is provider nan, compare the effort id against the static vocabulary; unknown id emits a WARNING listing the correct vocabulary (message includes agent, value, vocabulary). Read-only, never rewrites, matching the markdown-pin posture.

## TUI components

1. NanUsageStrip in slot session.composer.top: resolver probe; execFile(nan, ["metrics","usage"]) with 60s cadence, timeout, maxBuffer, disposed+generation guards; catalog snapshot from execFile(cortexExecutable(), ["model","catalog","--json","--provider","nan"]) fetched lazily and cached 10 minutes; monthToDate percent = model tokens / meta.monthlyQuotaTokens (quota 0 suppresses the percent); last-24h absolute burn via formatLargeTokens; single line built from MultiColorProgressBar; hidden silently when the session model is not picker-eligible nan or any data source is unavailable.
2. :model layer (registerLayer, priority 40, category Cortex, dispose via lifecycle.onDispose): opens panel cortex.model inside session.panel; read-only browse of catalog models; effort options filtered by meta.effortVocabulary with adjustable badges; qwen3.6 visually deprioritized in favor of deepseek-v4-flash; agent selector; confirm executes dry-run then model set with --json and renders the machine receipt outcome.
3. :nan layer: panel cortex.nan with daily sparkline from timeSeries (input+output per date, per model), per-model month-to-date vs quota table, and a static rolling-4h reference note for glm5.3 (explicit text: daily granularity cannot express a rolling window).
4. Bundle chain: every TSX task ends with npm --prefix internal/tuiassets run build then go build; the committed .js is regenerated by the owning task. Syntax oracle uses an .mjs copy (node --check on the CJS asset is known-broken, obs 43).

## Security and secret hygiene

- Variant settings/headers/body never surface beyond the authored id+reasoningEffort pair, and catalog receipts expose ids only.
- apiKeys are redacted in every receipt and UI path.
- No new endpoints; network access stays with the nan binary and the loopback daemon read tier.

## Test strategy (project policy)

- No persistent modelmgr tests: behavior oracles are disposable smokes with CORTEX_IA_HOME temp homes, deleted after run.
- Persistent gates stay limited to whitelisted surfaces (TUI package, pipeline install test).
- TSX verification = tsup + go build chain plus .mjs syntax check where needed.

## Migration and rollback

- Additive JSON/meta changes roll forward; removal path is config revert from the verified backup plus "model unset" for the agent mapping. No schema migration.

## Trace-off notes

- Static quotas over daemon-fetched limits: quotas are plan facts the daemon does not publish; drift is handled by doctor warnings and disclosed source.
- Reference-only 4h window over synthesized windows: correctness over illusion.
- Two stacked TSX tasks over one giant diff: hotspot file serialization under the 500-LOC flexible TSX cap.
