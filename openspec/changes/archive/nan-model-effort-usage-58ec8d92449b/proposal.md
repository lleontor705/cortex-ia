# Proposal: Nan Model Selection, Effort, and Usage Bar UX (nan-model-effort-usage)

## Problem

The nan provider is configured in the OpenCode user config with chat models, but none of those model entries declare variants[]. OpenCode v2 selects reasoning effort as provider/model#variant, so today every effort-qualified reference (for example nan/glm5.3#high) fails closed at model resolution. Users cannot assign effort levels through cortex-ia, have no quota visibility for nan usage inside the TUI, and cannot pick a model/effort pair without hand-editing JSONC.

## User Value

- One command assigns an agent to a nan model with a valid effort level and authors the missing variant automatically.
- An always-on, one-line usage strip in the chat session shows month-to-date quota burn for the current nan model plus last-24h absolute usage.
- Interactive :model and :nan layers let users browse picker-eligible models, choose effort options that respect each model capability matrix, and inspect usage trends without leaving the TUI.

## Approach

Six ratified decisions (approved by the user in the originating session, not re-litigated here):

1. Variant auto-authoring: extend "cortex-ia model set <agent> <ref> --effort <level>" so that, when the referenced nan model has no variants[] entry for a vocabulary-valid effort, modelmgr CREATES it in the OpenCode global config via the comment-preserving filemerge boundary, with ownership-conflict fail-closed behavior and --dry-run support.
2. Usage bar math: official nan plan quotas are encoded as static catalog metadata in internal/modelmgr (per-model monthly quota; glm5.3 premium allowance 3,000M tokens/month with a 400M rolling-4h reference). The TUI strip renders month-to-date percent per model from "nan metrics usage" monthToDate against catalog quota, plus last-24h burn. The rolling-4h window is reference-only in the :nan detail panel because daily-granularity timeSeries cannot compute a rolling window.
3. Placement: always-on compact one-line strip in the currently unused TUI host slot session.composer.top; detail views open through new registerLayer commands :model and :nan.
4. Picker: a :model panel lists picker-eligible nan chat models from "cortex-ia model catalog --json" with effort options constrained by the per-model capability matrix; confirmation assigns per-agent through the existing "model set <agent> provider/model#variant" authority using machine JSON receipts with a dry-run preview first. Read-only until the user confirms in-panel.
5. Web console usage card: explicitly OUT of scope.
6. Execution conditions: auto mode, hybrid spec plane, flexible workload policy.

## Contract Expansion (Ratified, document prominently)

The v1 model surface is global-config-only: it writes agents.<agent>.model in opencode.jsonc and never authors model definitions. This change ratifies a narrow, deliberate expansion: for provider "nan" only, "model set --effort" may append a single variants[] object (id plus settings.reasoningEffort) to the referenced model entry in the same file, using the same atomic, comment-preserving filemerge writer, the same ownership-conflict fail-closed discipline, and the same verified-backup restore path. No variant headers, bodies, or settings beyond reasoningEffort are ever authored. Nothing else in the provider or model definition is modified.

## Non-Goals

- No web console usage card or non-loopback surface changes.
- No changes to nan authentication flows (nan auth/me) and no handling of API keys.
- No reintroduction of platform adapters or session-level model overrides.
- No effort authoring or vocabulary validation for providers other than nan.
- No computed rolling-4h usage window (data granularity does not support it).
- No persistent modelmgr test files (disposable smokes with temp homes only, per AGENTS testing scope).

## Risks

- internal/tuiassets/cortex-ia-tui.tsx is a 2403-LOC hotspot: strip and picker are stacked tasks with a sequential dependency to serialize edits.
- Config mutation safety: authoring is gated on dry-run parity, fail-closed conflicts, and existing restore-on-apply-failure.
- External binary dependence: the nan executable is not on PATH; resolution probes candidates and every failure mode degrades to hiding the usage UI silently.
- Quota metadata is static and can drift from plan changes; doctor and receipts disclose source and vocabulary to limit confusion.

## Success Criteria

All five DAG tasks approved with green verification chains, doctor clean on nan variants, catalog JSON consumed by the plugin bundle, and set --effort producing a config that OpenCode resolves at runtime.
