# Model Configurator — OpenCode v2 agent model + effort

## Why

OpenCode v2 selects per-agent models through the config `agents` map using compact `provider/model#variant` strings, and markdown agents may pin a model in YAML frontmatter. cortex-ia users must currently hand-edit `~/.config/opencode/opencode.jsonc` with no validation, no ownership evidence, no drift detection, no uninstall/rollback integration, and no TUI surface. The V2 session runner ignores `request.body` overlays, so effort must be expressed as the model variant — a subtlety users get wrong — alongside deprecated fields (`temperature`, `top_p`, `maxSteps`, `permission`, `tools`, `disable`) that must never be written.

## What Changes

- New top-level `cortex-ia model` command with subcommands `list|get|set|unset|doctor`, a strict sibling of `mcp`, built in the same three layers: `internal/app` (parse + receipts), `internal/modelmgr` (pure manager, mcpmanager doctrine), `internal/install` (transactional service wiring).
- Flag grammar avoids every retired `--model*` prefix: `model set <agent> <provider/model[#variant]> [--effort <level>] [--json] [--dry-run]` where `--effort` maps to the `#variant` suffix; canonical stored form is the compact string.
- New Bubble Tea **models screen** in the zero-arg TUI (state/action seam like the stats screen) listing agents with their effective model/variant and persisting through the same service path — never ad-hoc file writes.
- Ownership lifecycle: additive `AgentModels` record kind in MetadataV2/LockV2, a domain-separated versioned digest in `internal/installmeta`, namespaced fingerprint-sidecar records (`agent-model/<agent>`), and uninstall/rollback enumeration.
- Free-form shape validation of `provider/model[#variant]` — no embedded model catalog. `model doctor` cross-checks `opencode2 models` output non-fatally.

## Scope

Global configuration only: `~/.config/opencode/opencode.jsonc` (jsonc-first, mirroring mcpmanager `ConfigPath`). `model list`/`get`/`doctor` work honestly on uninstalled homes; `set`/`unset` gate on the agreed v2 installation.

## Non-Goals

- Project-scope configuration writing (deferred by product decision; global only in v1).
- Root-level default `model` key management (per-agent only).
- Embedded provider/model catalog or network lookups (free-form shape validation only; the `opencode2 models` cross-check is informational and non-fatal).
- Web-console mutation surface (`/api/config` stays read-only).
- `request.body` overlays or any deprecated field (`temperature`, `top_p`, `prompt`, `permission`, `tools`, `disable`, `maxSteps`).
- Automated resolution of markdown-frontmatter precedence: frontmatter pins are surfaced as doctor warnings and receipt notes, never silently rewritten.

## Impact

- **Code**: new `internal/modelmgr`, `internal/app/model.go`, `internal/install/model.go`, `internal/installmeta/modeldigest.go`, `internal/tui/models_screen.go`; edits to `internal/state/metadata_v2.go`, `internal/app/app.go`, `internal/install/uninstall.go`, `internal/install/rollback.go`, `internal/tui/model.go`, `AGENTS.md`.
- **Specs**: new `model-command` capability (this delta).
- **Compatibility**: additive metadata JSON (no schema bump); config mutations preserve JSONC comments and unrelated keys via `filemerge.MutateJSONFile`; the embedded `opencode.jsonc` template must never gain `agents`/`model` keys (regression-guarded).
