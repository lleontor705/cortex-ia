# Cortex — Persistent Memory for AI Agents

`cortex` is the memory layer cortex-ia wires into every agent. It's a separate Go binary that exposes ~31 tools over MCP, backed by a local-first SQLite + FTS5 + vector + knowledge-graph store. The cortex-ia `cortex` component is the injector that registers that MCP with each agent.

## Tool groups

### Memory CRUD (Engram-compatible — 14 tools)

`mem_save`, `mem_search`, `mem_search_session`, `mem_observation`, `mem_observation_update`, `mem_observation_delete`, `mem_session_start`, `mem_session_end`, `mem_session_list`, `mem_summary`, `mem_revisions`, `mem_timeline`, `mem_context`, `mem_stats`.

### Cortex-exclusive (8 tools)

`mem_search_hybrid` (FTS5 + vector with RRF fusion), `mem_search_temporal` (as-of date queries), `mem_dna` (project DNA digest), `mem_consolidate` (merge near-duplicates), `mem_score` (importance), `mem_reindex`, `mem_archive`, `mem_health`.

### Knowledge graph (4 tools)

`graph_edge_add`, `graph_edge_remove`, `graph_neighbors` (typed BFS up to depth 10), `graph_temporal_neighbors` (with valid-from / invalid-at windows).

### Project & lifecycle (5 tools)

`project_list`, `project_merge`, `obs_score_update`, `obs_archive`, `obs_restore`.

The full reference lives in the `cortex` repo's `llms-full.txt`.

## How cortex-ia wires it

The `cortex` cortex-ia component (`internal/components/cortex/`) emits per-agent server config:

| Agent | Strategy | Output |
|---|---|---|
| Claude Code | `StrategySeparateMCPFiles` | `~/.claude/mcp/cortex.json` |
| OpenCode | `StrategyMergeIntoSettings` | `~/.config/opencode/opencode.json` (`mcp.cortex` block) |
| Cursor / Windsurf / Antigravity / Kiro | `StrategyMCPConfigFile` | `mcp.json` (`mcpServers.cortex`) |
| VS Code Copilot | `StrategyMCPConfigFile` (vscode overlay) | `mcp.json` (`servers.cortex`, `type: stdio`) |
| Codex | `StrategyTOMLFile` | `config.toml` (`[[mcp_servers]]`) |
| Gemini CLI / Qwen / Kilocode / Kimi | `StrategyMergeIntoSettings` | per-agent settings file |

In every case the command is `cortex mcp` (the binary stays on `PATH`, no `npx` indirection).

## Verifying the wiring

```bash
cortex mcp --help          # confirm the binary is on PATH
cortex-ia doctor            # verifies cortex callable + skills + state
```

`doctor`'s "cortex" check runs `cortex --version` to ensure the binary works. If the binary is missing, the cortex component is silently skipped — cortex-ia does not hard-depend on it.

## Convention file

The `conventions` component installs `~/.cortex-ia/skills/_shared/cortex-convention.md` which every SDD skill references. That file teaches the agent **when** to call `mem_save`, `mem_search`, and `forgespec_*` — without it, agents tend to either over-save or never persist anything.

## Compatibility

cortex's first 14 tools match Engram's API one-to-one. Skills written against Engram (e.g. legacy `mem_search(query, project)`) work without modification. cortex-exclusive tools (`mem_search_temporal`, `mem_dna`, etc.) gracefully degrade — skills that don't reference them keep working.

## See also

- [`components.md`](components.md) — how `cortex` fits among the other components
- [`sdd-workflow.md`](sdd-workflow.md) — the SDD loop that uses memory + forgespec
- The `cortex` repo for the full tool reference and HTTP/CLI/TUI alternatives
