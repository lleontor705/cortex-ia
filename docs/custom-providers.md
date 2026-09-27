# Custom Providers

Cortex-IA can install a **custom OpenCode provider** — an npm provider package, its API base URL, and its chat models — into the managed OpenCode configuration. Providers are declared as JSON catalogs, and the install runs through the same transactional pipeline as the rest of install/sync (verified backup, atomic config merge, state commit).

## Provider catalogs

Each provider is one declarative JSON file under the Cortex-IA state root:

```
~/.cortex-ia/providers/
└── nan.json
```

The directory and the embedded `nan` seed are materialized idempotently by `internal/providermgr` when absent. An existing file — valid or malformed — is **never overwritten**, and a malformed catalog fails closed with a typed error that names the provider and field without echoing the offending value.

### Catalog schema

```json
{
  "schema": "cortex-ia/providers/v1",
  "id": "nan",
  "name": "Nan",
  "npm": "@ai-sdk/openai-compatible",
  "package": "@opencode/ai/providers/openai-compatible",
  "baseURL": "https://api.nan.builders/v1",
  "effort_posture": "A value a model cannot apply is never an error.",
  "models": [
    {
      "id": "glm5.3",
      "name": "GLM 5.3",
      "efforts": ["low", "medium", "high", "max"],
      "context": "1M",
      "premium": true,
      "limit": { "context": 1048576, "output": 32768 },
      "modalities": { "input": ["text"], "output": ["text"] }
    }
  ]
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `schema` | yes | Must be `cortex-ia/providers/v1`; any other value fails closed. |
| `id` | yes | Provider id; also the catalog file's base name. |
| `name` | yes | Display name. |
| `npm` | yes | OpenCode provider npm package. |
| `package` | no | OpenCode v2 runtime provider package; falls back to `npm`. |
| `baseURL` | yes | Provider API base URL. |
| `docs` | no | Documentation pointer. |
| `effort_posture` | no | Prose describing how the provider maps reasoning effort. |
| `models` | yes | Non-empty array of model definitions (`id`, `name`, `efforts` required; `context`, `premium`, `limit`, `modalities` optional). |

Unknown catalog or model members are rejected, model ids must be unique, and every value is validated with typed errors. Accessors return defensive copies, so a caller can never mutate shared catalog state.

## Installing a provider from the TUI

The interactive dashboard exposes a **Home → Install custom provider** screen:

1. **List** — providers are read from the `~/.cortex-ia/providers/` catalogs; press enter to select one, `r` to reload.
2. **Token** — enter the provider API token (rendered masked).
3. **Preview** — a dry-run shows the exact entry, target config container, runtime member/key, and models that will be written.
4. **Commit** — a transactional install runs in three phases: **Backup → Update config → Commit state**.

The token is a transient argument: it travels only into the provider entry's `options.apiKey` and is never representable on any receipt. Installs write managed entries into the OpenCode configuration and record them in `state.json`/`cortex-ia.lock`, so `cortex-ia doctor` can detect drift and `cortex-ia rollback` can restore the pre-install state.

## Listing the selectable catalog

```bash
cortex-ia model catalog [--json] [--provider <id>]
```

`model catalog` lists the provider/model references (with their effort variants and the acquisition source) that the current OpenCode setup exposes. It acquires the catalog in tiers — the OpenCode daemon API (`GET /api/model`), then the `opencode models` text output — and never fails: an exhausted acquisition yields an honest empty receipt. Static capability metadata is attached only to the bundled `nan` entries. Once a custom provider is installed into the OpenCode configuration, its models become selectable through this catalog.

## See Also

- [`configuration.md`](configuration.md) — install/sync flags and managed state
- [`codebase/mcp-boundaries.md`](codebase/mcp-boundaries.md) — ownership and fail-closed boundaries
- [`codebase/reference-map.md`](codebase/reference-map.md) — CLI and package lookup
