# Model Configurator — design

## Context

OpenCode v2 configures per-agent models through the global config `agents` map (compact `provider/model#variant` strings; expanded `{providerID, model, variant}` also accepted on load) and through markdown agent frontmatter under `~/.config/opencode/agents/<name>.md`. Builtins are `build`/`plan` (primary) and `general`/`explore` (subagent), overridable by the same ID. Effort is expressed as the model variant — the V2 session runner does not send `request.body` overlays — and the legacy fields (`temperature`, `top_p`, `prompt`, `permission`, `tools`, `disable`, `maxSteps`) are deprecated.

cortex-ia already owns a proven template for managed global-config surfaces: `cortex-ia mcp` (parse/receipts in `internal/app`, pure `mcpmanager`, transactional `internal/install` wiring with verified backups, ownership records, and a fingerprint sidecar). This change replicates that doctrine for agent models and extends it to the TUI.

## Goals / Non-Goals

**Goals**: one command surface (`model`) and one TUI screen operating the same service path; free-form shape validation; ownership, drift, and uninstall/rollback integration; honest read-only behavior on uninstalled homes.

**Non-Goals**: project-scope writing (deferred by product decision); root default `model` key; embedded catalogs; web-console mutations; deprecated fields and `request.body` overlays; automatic markdown-frontmatter precedence resolution.

## Architecture

```
internal/app/model.go          parse + receipts (runModel; list|get|set|unset|doctor)
internal/app/app.go            dispatch case "model" + help text (strict sibling of "mcp")
             │
internal/install/model.go      transactional service: lock, serviceTxn, sidecar, records
internal/install/uninstall.go  lifecycle enumeration of meta.AgentModels
internal/install/rollback.go
             │
internal/modelmgr              pure manager: Desired validation, registry, JSONC overlay mutations, doctor
internal/installmeta           versioned agent-model digest (stdlib-only leaf)
internal/state                 AgentModelV2 record kind + lock agreement
internal/tui/models_screen.go  modelsState + modelsAction (service seams), registered in model.go
```

Dependency direction mirrors `mcpmanager`: `app → install → modelmgr → {filemerge, installmeta}`; `state` and `installmeta` stay leaves.

## Data model

- **Config target**: `agents.<agent>.model` as the compact string `provider/model#variant`. The expanded object form is read-supported and re-serialized compactly. Only the `model` member is ever mutated; all other agent members and comments are preserved.
- **Desired** (modelmgr): `{Agent, Provider, Model, Variant}`. Parse splits at the rightmost `#`; tokens must be non-empty and free of whitespace, `#`, and control characters. `--effort <level>` overrides a `#variant` suffix. Canonical encoding is the compact string.
- **AgentModelV2** (state): `{agent, config_path, semantic_digest, ownership}` mirroring `MCPV2`; additive JSON field `agent_models` on MetadataV2 and LockV2; nil normalizes to an empty slice (no schema bump, no migration).
- **AgentModelIdentity** (installmeta): `{agent, provider, model, variant}` encoded as canonical JSON with fixed field order, version 1, prefix `amdv1:`, domain `cortex-ia/installmeta/agent-model-digest\n`. Model references are non-secret, so a plain versioned SHA-256 suffices — a documented deviation from the mcpv2 keyed postimage (there are no URLs or env values here). The leaf imports nothing from the repository.
- **Fingerprint sidecar**: reuse the existing `FingerprintDocument` with namespaced record names `agent-model/<agent>` (additive; MCP lookups match by exact name, so no collision). No new sidecar file, no schema change.

## Key decisions

1. **Strict sibling of `mcp`, no `--model*` flags.** The command name `model` is free (`retiredCommands` does not list it); the `--model` flag prefix is retired app-wide and `preflightCLI` keeps rejecting it. The grammar uses `--effort`, `--json`, `--dry-run` only. A preflight regression test pins this forever.
2. **`set` overwrites with full disclosure.** Unlike MCP custom entries (accreditation-required removals of complex unmanaged bodies), a model reference is a single scalar the user explicitly asked to change; the receipt always reports the previous value, and the change is reversible via verified backup and `unset`. Fail-closed stays for malformed config, unresolvable agents, and `ErrNotInstalled`. This is a deliberate, documented deviation from the `ConflictUnaccredited` doctrine.
3. **Markdown frontmatter pins are surfaced, not resolved.** Doctor reports them as WARNING; `set` succeeds with a receipt note. v1 does not guess precedence between frontmatter and config entries and never rewrites frontmatter (non-goal).
4. **Unknown agents fail closed** with a typed conflict listing the resolution sources (builtins, config keys, markdown files), mirroring the fail-closed manager doctrine.
5. **JSONC safety.** Mutations go exclusively through `filemerge.MutateJSONFile` (hujson Pack preserves comments; duplicate members rejected; atomic write). `MergeJSONObjects` is banned on user config (strips comments; violates the installer-dev skill contract).
6. **No `txn.go` edit.** `serviceTxn` gains a `commitStateModels` method defined in `internal/install/model.go` (same package), mirroring `commitState` but writing `meta.AgentModels` plus the derived lock — identical journaling, backup, and reverse-restore semantics.
7. **TUI through service seams.** `modelsState`/`modelsAction` mirror the stats screen (self-contained state, screen-level action seam, injectable loader). Loading and persistence use injected function variables wrapping the install service — the same mutations the CLI uses — so the TUI never writes config directly and tests never touch real user state. If the screen exceeds the flexible source budget, the state/action seam lands first and the view/keybindings follow as a stacked follow-up on the same files.
8. **Template regression guard.** The embedded `internal/assets/opencode.jsonc` must never gain `agents`/`model` keys: the installer applies the template as a safe-merge in which template keys win, so a template key would silently pin models on every install/sync. A persistent test in `internal/app` asserts the guard.
9. **Honest uninstalled behavior.** `list`/`get`/`doctor` read config and registry without demanding v2 metadata (ownership shown as unavailable); `set`/`unset` gate on `ErrNotInstalled` like MCP mutations.

## Doctor checks

| Check | Severity | Notes |
|---|---|---|
| JSONC decode health / duplicate members | ERROR | names the offending member, never values |
| Shape of every `agents.*.model` | ERROR | compact and expanded forms |
| Managed-entry drift (recorded vs observed digest) | WARNING+ | expected/observed digests disclosed |
| Markdown frontmatter model pin | WARNING | informational, settable |
| `default_agent` references an existing visible primary | WARNING | per v2 docs |
| Embedded template free of `agents`/`model` keys | ERROR | regression guard |
| `opencode2 models` cross-check | INFO/SKIPPED | non-fatal when binary/output unavailable |

## Flows

- **set**: parse → Desired.Validate → registry resolve (case-folded) → service.ModelSet → (dry-run? report only) → lock → reload context+sidecar → serviceTxn(verified backup) → manager mutates via MutateJSONFile → commitStateModels + namespaced sidecar record → commit → receipt (action, previous value, compact value, config path, backup ID, warnings incl. markdown pins).
- **unset**: same recipe; removes the `model` member; prunes an emptied entry cortex-ia created.
- **doctor**: load config honestly → run all checks with injectable seams → typed findings; never mutates.
- **TUI**: modelsState loads the registry + effective models through the loader seam → user edits → dry-run preview → confirm → persistence seam (same service call) → in-screen receipt/conflict.

## Risks / Trade-offs

- *Read-convert of expanded model objects* rewrites stored form on the next `set`; the receipt discloses it. Acceptable: compact is the documented canonical form.
- *Builtin drift* if OpenCode adds builtins: the registry pins the four documented v1 builtins; the doctor cross-check informs; adding builtins is a small additive change.
- *Case-fold collisions* on Windows/macOS mirrors assetmap doctrine (fold on windows/darwin).
- *Sidecar namespace* reuses one document; exact-name matching keeps MCP and agent-model records independent.

## Migration plan

Additive JSON only. Documents without `agent_models` validate unchanged (nil → empty slice); state/lock agreement compares the new records like MCPs.

## Open questions

None blocking. The `opencode2 models` output parser is defensive: any parse failure degrades to a SKIPPED finding.
