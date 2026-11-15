> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# Custom Providers

**This feature was removed.** Cortex-IA no longer installs custom OpenCode providers. Provider installation is native in OpenCode: the `nan` provider ships native OpenCode provider support, so providers are configured directly through OpenCode's own configuration.

The retired installer spanned the `internal/providermgr` catalog seeder, the `install.Service` provider transaction, the state v2 `Providers` family, and the TUI **Install custom provider** screen. Existing `state.json` provider rows are passively retired — the schema version stays pinned at 2 and the stale key drops on the next state commit — and provider blocks already written into `opencode.json(c)` remain valid native OpenCode configuration, so no user action is required.

## See Also

- [`configuration.md`](configuration.md) — install/sync flags and managed state
- [`codebase/mcp-boundaries.md`](../architecture/codebase/mcp-boundaries.md) — ownership and fail-closed boundaries
- [`codebase/reference-map.md`](../architecture/codebase/reference-map.md) — CLI and package lookup
