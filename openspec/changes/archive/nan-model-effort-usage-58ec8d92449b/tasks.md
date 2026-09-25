# Tasks: Nan Model Selection, Effort, and Usage Bar

Task text mirrors the work items on board nan-model-effort-usage. Requirements IDs trace to specs/nan-model-integration/spec.md.

## Wave 1

- [ ] task-nan-catalog [modelmgr] Publish static nan model metadata through model catalog receipts
  - Requirements: REQ-NAN-001
  - Files: internal/modelmgr/nan_catalog.go, internal/modelmgr/catalog.go, internal/app/model.go
  - Depends: none
  - Verification: gofmt -s -w . && go vet ./internal/modelmgr/... ./internal/app/... && go build -o bin/cortex-ia ./cmd/cortex-ia
  - Notes: new static NanModelMeta table for the seven ratified chat models with quotas and effort vocabularies; additive CatalogEntry.meta in JSON and text receipts; identifiers only, no settings or keys; confirm via "bin/cortex-ia model catalog --json --provider nan".

## Wave 2

- [ ] task-nan-variants [modelmgr] Author nan effort variants in model set and validate them in doctor
  - Requirements: REQ-NAN-002, REQ-NAN-003
  - Files: internal/modelmgr/manager.go, internal/modelmgr/model.go, internal/modelmgr/doctor.go
  - Depends: task-nan-catalog
  - Verification: gofmt -s -w . && go vet ./internal/modelmgr/... && go build -o bin/cortex-ia ./cmd/cortex-ia
  - Notes: vocabulary pre-validation, single-mutation authoring of {id, settings.reasoningEffort} appended to existing variants, ownership-conflict fail-closed on malformed arrays, dry-run byte parity, doctor nan-variants WARNING; prove with disposable temp-home smokes (CORTEX_IA_HOME) then delete them.

- [ ] task-nan-strip [tui] Render the always-on nan quota strip in session.composer.top
  - Requirements: REQ-NAN-004
  - Files: internal/tuiassets/cortex-ia-tui.tsx, internal/assets/tui/cortex-ia-tui.js
  - Depends: task-nan-catalog
  - Verification: npm --prefix internal/tuiassets run build && go build -o bin/cortex-ia ./cmd/cortex-ia
  - Notes: nan executable candidate resolver, 60s-class bounded poller with timeout/maxBuffer/disposed+generation guards, monthToDate percent against catalog quota, 24h burn via formatLargeTokens, silent hide states, reuse MultiColorProgressBar, commit regenerated bundle.

## Wave 3

- [ ] task-nan-picker [tui] Add :model picker and :nan detail layers backed by machine receipts
  - Requirements: REQ-NAN-005
  - Files: internal/tuiassets/cortex-ia-tui.tsx, internal/assets/tui/cortex-ia-tui.js
  - Depends: task-nan-strip, task-nan-variants
  - Verification: npm --prefix internal/tuiassets run build && go build -o bin/cortex-ia ./cmd/cortex-ia
  - Notes: registerLayer commands in category Cortex with lifecycle disposal; catalog-driven model list with vocabulary-constrained effort options; qwen3.6 deprioritized; read-only until confirm; dry-run preview then model set --json; :nan daily sparkline, per-model month-to-date breakdown, static rolling-4h reference note; syntax-oracle via .mjs copy only; commit regenerated bundle.

## Wave 4

- [ ] task-nan-integration [assets/app] Rebuild chain and end-to-end verification for the nan initiative
  - Requirements: REQ-NAN-001, REQ-NAN-002, REQ-NAN-003, REQ-NAN-004, REQ-NAN-005
  - Files: internal/assets/tui/cortex-ia-tui.js
  - Depends: task-nan-picker, task-nan-variants
  - Verification: gofmt -s -w . && go vet ./... && go test -count=1 ./... && npm --prefix internal/tuiassets run build && go build -o bin/cortex-ia ./cmd/cortex-ia
  - Notes: install/sync into a CORTEX_IA_HOME temp home, run model catalog --json and confirm the plugin bundle consumes quota metadata shapes, model set --effort --dry-run against a temp config then a real authoring check OpenCode accepts, doctor clean on nan variants, all temp homes deleted after the run.
