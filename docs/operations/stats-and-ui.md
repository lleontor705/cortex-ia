> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# Usage Stats & TUI Cockpit

Cortex-IA ships a read-only view of OpenCode's own usage telemetry plus the interactive terminal cockpit. Neither surface mutates the OpenCode database or Cortex-IA work authority.

## `cortex-ia stats`

```bash
cortex-ia stats          # launch the interactive TUI (Stats panel)
cortex-ia stats --json   # print aggregated statistics as JSON
```

Statistics are derived from OpenCode's local usage database, owned by `internal/ocstats`:

- The database is resolved to `~/.local/share/opencode/opencode.db` by default; `CORTEX_IA_OPENCODE_DB` overrides that path when set to a non-empty value.
- The connection is opened **read-only**. One bounded aggregate pass is performed and every window is reduced in memory, so the command never runs DDL or holds a write lock and cannot contend with a live OpenCode session.
- Three windows are computed: all-time, trailing 30 days, and trailing 7 days.

The `--json` receipt has a stable shape:

| Field | Meaning |
|-------|---------|
| `db_path` | Resolved OpenCode database path. |
| `sessions` | Session count. |
| `messages` | Message count. |
| `tokens` | Total tokens across every class OpenCode pre-aggregates. |
| `active_days` | Distinct active calendar days. |
| `peak_hour`, `peak_hour_messages` | Busiest hour and its message count. |
| `favorite_model`, `favorite_model_share` | Most-used model and its share percentage. |
| `first_day`, `last_day` | First and last activity dates. |
| `top_models` | Up to five `ModelUsage` entries (model, sessions, tokens, share). |

Without `--json`, `cortex-ia stats` launches the interactive TUI.

## TUI cockpit

Running `cortex-ia` with **no arguments** boots directly into the **Stats** screen, so usage analytics are one keystroke from launch. Navigation uses `Esc` to go back, `q` to quit, and `Enter` to confirm.

The cockpit surfaces include:

- **Stats** — read-only usage analytics broken down by day, hour, and model.
- **Models** — resolves each agent's effective model, variant, and source, and lets you assign an agent model through the catalog picker.
- **Providers** — the custom provider catalogs and their transactional install flow (see [`custom-providers.md`](../getting-started/custom-providers.md)).
- **MCP**, **Web**, **Review/Install**, and **Agent Studio** — managed MCP entries, the embedded web console launcher, the install plan/review pipeline, and the agent configuration studio.

## `cortex-ia ui snapshot`

```bash
cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]
```

Prints a structured JSON dashboard snapshot of the current conversation's tasks, delegations, and attention items. It is the read-only data source the web cockpit polls.

The command opens the delegation store **read-only** (`OpenStoreReadOnly`): it never runs DDL or `BEGIN IMMEDIATE`, so serving a snapshot cannot contend with work-authority writers for the database write lock. A missing database or an uninitialized schema yields an honest empty snapshot rather than an error.

## See Also

- [`web-console.md`](web-console.md) — embedded Preact operations console
- [`codebase/dashboard.md`](../architecture/codebase/dashboard.md) — Bubble Tea TUI architecture
- [`configuration.md`](../getting-started/configuration.md) — environment variables and managed state
