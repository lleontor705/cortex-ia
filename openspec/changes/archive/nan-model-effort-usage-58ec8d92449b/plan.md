# Integrated Plan: Nan Model Selection, Effort, and Usage Bar

## 1. Intent & Non-Goals

Give cortex-ia users (1) a validated, auto-authoring path from "model set --effort" to OpenCode-resolvable nan variant references, (2) an always-on quota strip in the TUI chat session, and (3) interactive :model and :nan panels backed by machine receipts. Non-goals: web console usage card, nan auth or API key handling, non-nan effort authoring, computed rolling-4h windows, platform adapters, session-level model overrides.

## 2. Requirements

Authoritative Given/When/Then text lives in specs/nan-model-integration/spec.md (hybrid plane; OpenSpec is the spec source, Cortex holds decisions/evidence).

| ID | Obligation | Tasks |
| --- | --- | --- |
| REQ-NAN-001 | Static nan catalog metadata in catalog receipts (identifiers only) | task-nan-catalog |
| REQ-NAN-002 | Variant auto-authoring with dry-run parity and fail-closed conflicts | task-nan-variants |
| REQ-NAN-003 | Doctor nan effort validation, WARNING-only | task-nan-variants |
| REQ-NAN-004 | session.composer.top usage strip with silent degradation | task-nan-strip |
| REQ-NAN-005 | :model picker and :nan detail layers, read-only until confirmed | task-nan-picker |

## 3. Design

New static table internal/modelmgr/nan_catalog.go feeds an additive Meta object on CatalogEntry receipts. model set --effort validates the level against the model vocabulary, then authors {id, settings.reasoningEffort} into the model variants array through the existing single-mutation filemerge path (ownership-conflict fail-closed, dry-run preview, unchanged backup/restore). Doctor gains a read-only nan-variants check. The TUI plugin adds a 60s-class nan metrics poller with candidate-path resolution and generation guards, a quota strip in slot session.composer.top, and registerLayer commands :model and :nan backed by catalog JSON receipts and the model set machine receipts. Details, data model, and trade-offs: design.md.

## 4. Tasks

DAG (board nan-model-effort-usage, sdd-lite, flexible, auto):

| Task | Wave | Depends on | Files |
| --- | --- | --- | --- |
| task-nan-catalog | 1 | none | internal/modelmgr/nan_catalog.go, internal/modelmgr/catalog.go, internal/app/model.go |
| task-nan-variants | 2 | task-nan-catalog | internal/modelmgr/manager.go, internal/modelmgr/model.go, internal/modelmgr/doctor.go |
| task-nan-strip | 2 | task-nan-catalog | internal/tuiassets/cortex-ia-tui.tsx, internal/assets/tui/cortex-ia-tui.js |
| task-nan-picker | 3 | task-nan-strip, task-nan-variants | internal/tuiassets/cortex-ia-tui.tsx, internal/assets/tui/cortex-ia-tui.js |
| task-nan-integration | 4 | task-nan-picker | internal/assets/tui/cortex-ia-tui.js |

task-nan-strip and task-nan-variants run in parallel (disjoint files); the two TSX tasks share the hotspot file and are serialized by dependency, each capped at 500 LOC churn per the flexible policy. Full task text: tasks.md.

## 5. Verification and Workload

- Go gates: gofmt -s, go vet, go build chain per task (raw commands in tasks.md).
- TSX gates: npm --prefix internal/tuiassets run build then go build -o bin/cortex-ia ./cmd/cortex-ia per task; node --check against an .mjs copy of the bundle only (the CJS asset oracle is known-broken, obs 43).
- modelmgr oracles: disposable temp-home smokes (CORTEX_IA_HOME), deleted after run; no persistent tests added outside the whitelisted surfaces.
- Review: single independent code-review-adversary pass after integration with config-mutation safety and secret-hygiene lenses.

## 6. Rollback

Revert commits in task order. Config side effects are bounded to opencode.jsonc authored variants plus agent mappings; "model unset" removes the mapping and the verified-backup path restores the file. Static metadata and read-only panels need no migration.
