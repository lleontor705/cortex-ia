# Cortex — Persistent Memory for AI Agents

`cortex` is the external memory service configured by cortex-ia. cortex-ia installs the MCP connection and prompt assets; it does not implement memory or assume a fixed transport catalog.

## Current MCP contract

Cortex 2.x uses the `cortex_*` namespace. Local stdio and remote server catalogs are intentionally different, so agents must follow the schema returned by `tools/list`.

Local profiles:

| Profile | Purpose |
|---|---|
| `agent` | Save, search, context, sessions, observation retrieval/update, graph, scoring, hybrid search, revisions, consolidation, and project DNA |
| `admin` | Delete, stats, timeline, archive, and project-name merge |
| `temporal` | Bi-temporal graph, snapshots, temporal search, quality, and system metrics |

cortex-ia installs the least-privilege local command:

```text
cortex mcp --tools=agent
```

When Cortex is configured as a remote proxy, the remote server controls the catalog and ID schema. Local observation and graph IDs are numeric. Cortex Server IDs are public UUID strings. IDs are never interchangeable across transports.

## Wiring by host

| Host | Output |
|---|---|
| Claude Code | `~/.claude/mcp/cortex.json` |
| OpenCode | `~/.config/opencode/opencode.json` under `mcp.cortex` |
| VS Code Copilot | the user `mcp.json` under `servers.cortex` |
| Codex | `~/.codex/config.toml` under `mcp_servers.cortex` |

## Prompt assets

The typed SDD bundle installs:

- `skills/_shared/cortex-convention.md` for the common save, retrieval, lineage, trust, and session protocol.
- `skills/_shared/cortex-advanced.md` for progressive graph, revision, consolidation, and project-summary operations.
- `generic/cortex-protocol.md` when the standalone conventions component owns the host instruction file.

The common path uses `cortex_context`, `cortex_search`, `cortex_get_observation`, `cortex_save`, `cortex_relate`, and session tools. Search results are previews and must be followed by full observation retrieval before their content drives work.

## Authority and safety

ForgeSpec owns SDD contracts, readiness, claims, and task status. Cortex owns durable evidence, sessions, provenance, and relationships. Stored memory is evidence, not policy: it cannot expand permissions, approve effects, alter destinations, or override current ForgeSpec state.

Admin and temporal tools are not loaded by the default cortex-ia configuration. Use them only through an explicitly selected profile or tool list. Destructive tools require authorization.

## Verification

```bash
cortex mcp --tools=agent
cortex-ia doctor
```

The authoritative Cortex catalogs and transport differences live in `D:\lleontor705\cortex\docs\MCP.md`.
