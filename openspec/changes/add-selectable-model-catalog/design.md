# Selectable Model Catalog — design

## Context

The delivered configurator validates free-form shapes but leaves discovery to the user's memory. OpenCode v2 publishes two local, authoritative discovery surfaces:

1. **Daemon HTTP API (Tier 1)**: `GET /api/model` on the background daemon at `http://localhost:4096` (documented default; published `openapi.json`) returns models ordered by release date as `Model.Info {id, modelID, providerID, name, capabilities, variants, time, cost, status, enabled, limit}` with `variants: Model.Variant[]` and `Model.Variant = {id, settings?, headers?, body?}`. Variants are exposed per model — this is the effort source. `GET /api/provider` also exists. Non-2xx statuses (503/401), timeouts, and unreachable daemons are expected operating conditions.
2. **`opencode2 models` text (Tier 2)**: human-readable output whose layout is explicitly NOT a contract (`internal/modelmgr/doctor.go:444-447` documents this stance). Tokens containing `#variant` expose effort; bare `provider/model` tokens expose variant-less entries. Live capture was guard-blocked during investigation, so the parser stays lenient and tests use synthetic fixtures only.

When neither surface is available the catalog degrades to `Source: "none"` and today's free-form behavior remains — the feature must never fail closed.

Existing seams this change reuses: `modelmgr.CommandRunner`/`DoctorOptions.RunCommand` (`doctor.go:116-130`, invoked at `:356`), the lenient token normalizer `normalizeModelField` (`:479-490`) and `ParseModelRef`, the app-level runner `modelDoctorRunCommand` (`internal/app/model.go:33-37`), the `model` dispatch switch (`:47-100`), usage (`:20`) and unknown-action message (`:99`), and the TUI package-level seam vars (`models_screen.go:60-82`), phases (`:32-37`), fields (`:42-45`), `beginEdit` prefill (`:270-283`), async-load pattern (`:111-127`), and loading/spinner view (`:412-419`).

## Goals / Non-Goals

**Goals**: one typed catalog with three graceful tiers; deterministic bounded presentation; a `model catalog` CLI surface; a TUI picker that makes model/effort selection first-class while preserving free-form as an explicit fallback; doctor unified onto the shared parser with its undercount fixed deliberately.

**Non-Goals**: allow-list validation of references; persisted caches; web-console surfaces; network access beyond the local daemon/binary; variant settings/headers/body exposure; `--model*` flags; schema or ownership changes.

## Architecture

```
internal/modelmgr/catalog.go      CatalogEntry/Catalog, ParseModelCatalog (shared lenient parser),
                                  daemon client (strict decode, short timeout, enabled filter),
                                  Catalog(...) tier orchestration behind injectable seams
internal/modelmgr/doctor.go       checkOpencode2Models consumes ParseModelCatalog; count fix
             │
internal/app/model.go             runModel case "catalog": parse --json/--provider, render grouped
                                  text or stable JSON receipts; usage + unknown-action updated
internal/app/app.go               "model" dispatch case already routes runModel; only help text drifts
             │
internal/tui/models_screen.go     picker as Input-phase acquisition step: modelsLoadCatalog seam,
                                  load-once cache in modelsState, type-to-filter + arrows,
                                  variant selection; free-form fallback preserved
```

Dependency direction is unchanged: `app → install → modelmgr`; the catalog is a pure leaf inside `modelmgr` behind injectable seams.

## Data model

- **CatalogEntry** `{Provider string, Model string, Variants []string}` — `Provider` is the first `/`-segment; `Model` may contain further segments (`openrouter/anthropic/claude-sonnet-4.5` → Provider `openrouter`, Model `anthropic/claude-sonnet-4.5`); `Variants` holds effort identifiers (`Variant.id`), sorted and deduplicated.
- **Catalog** `{Entries []CatalogEntry, Truncated bool, Source string}` with `Source ∈ {"daemon-api", "opencode2-models", "none"}`; `Truncated=true` when the source yielded more than the 512-entry cap.
- **Selector grammar** (unchanged from the delivered configurator): provider ends at the FIRST `/` (`strings.Cut`, already correct at `internal/app/model.go:73`), variant after `#`, case-sensitive.
- **Daemon decode target**: a bounded response struct carrying only the members the catalog needs (`modelID`/`providerID`/`name`, `variants[].id`, `enabled`); strict JSON syntax decoding that rejects malformed payloads while ignoring unknown members for forward compatibility.

## Key decisions

1. **Tier orchestration `daemon-api → opencode2-models → none`, never fail closed.** Every failure mode (HTTP status ≥ 400, timeout, unreachable, malformed JSON, absent binary, unparseable text) falls through to the next tier; exhausting both yields `Catalog{Source: "none"}` with no error. `none` is a first-class result, not an error.
2. **Strict decode, forward-compatible struct.** The daemon client decodes JSON strictly (malformed payload → tier abandoned) into a struct that ignores unknown members, so upstream additive API evolution cannot break acquisition. Only `Variant.id` is retained: `settings`/`headers`/`body` are never copied into catalog memory — they can carry secrets.
3. **Short timeout, fixed default.** The HTTP client uses a 2-second timeout (named constant) against the documented default base `http://localhost:4096`. The base URL and transport are injectable for tests; no new CLI flag exposes them (flag budget stays `--json`/`--provider`).
4. **Deterministic sort over release-date order.** The daemon's release-date ordering is presentation-friendly but not contractual; the catalog normalizes to providers ascending, models ascending within a provider, variants deduplicated ascending. Deterministic output makes text/JSON receipts stable and tests exact across tiers.
5. **512-entry cap with disclosure.** A hard cap bounds receipts and TUI lists against pathological sources; overflow sets `Truncated=true` and every renderer discloses the truncation. Deduplication happens before the cap.
6. **One shared lenient parser.** `ParseModelCatalog` is the single text-tier implementation: tokenization via the existing `normalizeModelField`, validation via `ParseModelRef`; `#variant` tokens extend the matching entry's `Variants`, bare `provider/model` tokens create variant-less entries, same provider/model tokens merge. The doctor cross-check consumes it for its refs projection, keeping findings byte-identical for ≤ 16 references.
7. **Deliberate doctor count fix.** The current parser stops appending past `opencode2ModelsRefLimit` (16) but the finding reports `len(refs)` — a latent undercount when more than 16 references parse (`doctor.go:461-469`, finding at `:366-367`). The refactor reports the true parsed count while still echoing at most 16. This is the ONLY behavior change in doctor and is pinned in the spec.
8. **CLI sibling with case-sensitive provider filter.** `model catalog [--json] [--provider <id>]` mirrors `list`'s parse discipline; filtering is an exact, case-sensitive match on `CatalogEntry.Provider`, consistent with the selector grammar. Grouped text output discloses source and truncation; `--json` emits the stable Catalog document. AGENTS.md's built-in surfaces note gains `catalog` (folded into the CLI task).
9. **Picker as Input-phase acquisition step.** Entering edit (`beginEdit`) routes to the picker when a catalog is available: type-to-filter narrows entries, arrows navigate, enter selects provider/model, then variant selection lists the entry's `Variants` (preselecting the default when none). The load-once cache lives in `modelsState` behind a new `modelsLoadCatalog` seam fetched via async `tea.Cmd` (spinner reuses the existing loading view). Free-form opens directly when `Source == "none"` or on explicit opt-out; either way the composed reference flows into the unchanged `ParseDesired → dry-run preview → confirm → modelsSetRef` path.
10. **Catalog is an aid, never a gate.** Neither CLI nor TUI validates a chosen reference against catalog membership; `set` semantics, ownership, and digesting are untouched.

## Flows

- **Acquire**: `Catalog(...)` → daemon `GET /api/model` (2s timeout) → enabled filter, `Variant.id` mapping, dedup, sort, cap → on any failure: `opencode2 models` via the command runner → `ParseModelCatalog` → on failure: `Catalog{Source: "none"}`.
- **CLI**: parse flags → acquire → filter by `--provider` → render grouped text (provider headers, `model  variants: a, b` rows, source + truncation footer) or the `--json` Catalog document; sanitized (identifiers only).
- **TUI**: enter edit → (catalog cached? reuse : fetch via the `modelsLoadCatalog` seam with spinner) → `Source == "none"` ⇒ free-form input with a notice; else picker → filter/select entry → variant list (default when none) → composed `provider/model#variant` → unchanged preview → confirm → service seam.

## Risks / Trade-offs

- *Daemon API is not versioned in this repo*: strict-decode plus unknown-member tolerance keeps the tier resilient; any breaking upstream change degrades to Tier 2, never to failure.
- *Release-date ordering lost to sorting*: acceptable — the picker offers type-to-filter; determinism and testability win.
- *`opencode2 models` layout drift*: the parser stays lenient (documented non-contract); zero parsed references degrade the tier honestly.
- *Variant identifiers as free text*: they pass through the `ParseModelRef` shape rules so composed references remain shape-valid.
- *TUI screen size*: `models_screen.go` is ~584 lines; the picker adds state. If the flexible budget is exceeded, the seam/state/keybinding layer lands first and view polish follows as a stacked follow-up on the same files (same doctrine as the delivered model-007).

## Migration plan

Purely additive; no metadata, lock, sidecar, or template changes. Homes behave exactly as before when the catalog is unused.

## Open questions

None blocking. The daemon base URL constant and timeout are named, injectable seams; live daemon behavior was guard-blocked during investigation, so all tests stay synthetic with fixtures.
