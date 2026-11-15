> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# OpenCode Assets & MCP Components

`cortex-ia` deploys a complete, atomic asset set and manages the MCP catalog for **OpenCode** (`~/.config/opencode/`).

---

## 1. Native Workflow Assets

All workflow assets are embedded directly inside the `cortex-ia` binary via `go:embed` and mapped byte-for-byte to OpenCode's native directory structure:

| Asset Kind | Embedded Source | Destination (home-relative) | Purpose |
| :--- | :--- | :--- | :--- |
| **Base Configuration** | `opencode.jsonc` | `~/.config/opencode/opencode.jsonc` | Safe 3-way merge preserving user keys, comments, and permissions. |
| **System Prompt** | `AGENTS.md` | `~/.config/opencode/AGENTS.md` | Core orchestrator system prompt and operational protocol. |
| **Agents (6)** | `agents/*.md` | `~/.config/opencode/agents/<name>.md` | `orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `reviewer`. |
| **Slash Commands (13)**| `commands/*.md` | `~/.config/opencode/commands/<name>.md` | `/cortex-code`, `/cortex-ingest`, `/cortex-watch`, `/discover`, `/hotfix`, `/investigate`, `/resume`, `/review`, `/spike`, `/status`, `/tdd`, `/work`. |
| **Native Skills (17 + `_shared`)**| `skills/<name>/SKILL.md`| `~/.agents/skills/<name>/SKILL.md` | Role and utility skills (`fast-tdd`, `ast-impact-analysis`, `property-based-testing`, etc.). |
| **Shared Contracts** | `skills/_shared/*.md` | `~/.cortex-ia/opencode/contracts/*.md` | Canonical role, workflow, and protocol contracts. |
| **Plugins (8)** | `plugins/*.ts` | `~/.config/opencode/plugins/<name>.ts` | Cortex memory integration, lease and permission guards, skill discovery, snapshot, subagent transport, and work-state plugins. |
| **Theme** | `themes/cortex.json` | `~/.config/opencode/themes/cortex.json` | Bundled OpenCode theme (opt-in via `--theme`). |
| **TUI Plugin** | `tui/*.js` | `~/.config/opencode/tui-plugins/<name>.js` | Compiled terminal UI extension. |

---

## 2. Managed MCP Server Presets

`cortex-ia` manages two official catalog presets. Task coordination is built into the Go CLI and SQLite store.

### 1. Cortex (`cortex`) — *Default: ON*
- **Execution Vector**: `["cortex", "mcp", "--tools=agent"]`
- **Capabilities**: Cross-session persistent memory, knowledge graph, hybrid search (FTS5 + semantic), temporal evolution history.

### 2. Context7 (`context7`) — *Default: OFF (Optional)*
- **Execution Vector**: `["npx", "-y", "@upstash/context7-mcp@4.1.0"]`
- **Capabilities**: Live framework and library documentation lookup via MCP.

### Built-in Work Control

`cortex-ia work` provides the task DAG, optimistic revisions, TTL claims, exclusive file leases, recovery, approvals, and append-only events in `~/.cortex-ia/delegation.db`. It requires no MCP server.

---

## 3. Custom MCP Servers

Users can register custom local and remote MCP servers through the CLI:

```bash
# Add a custom local server
cortex-ia mcp add my-tool --local --env API_KEY=secret -- npx -y my-tool-mcp

# Add a custom remote SSE endpoint
cortex-ia mcp add remote-docs --remote https://mcp.example.com/sse --header "Authorization=Bearer secret"

# List accredited and unmanaged MCP servers
cortex-ia mcp list
```

---

## 4. Target Support Contract

OpenCode is the only target that receives the full Cortex-IA native asset set (`~/.config/opencode/`). The Claude Code and Antigravity CLI (agy) targets install host bundles, described in [platforms](../getting-started/platforms.md). The external execution leaf, the platform adapter registry, and their configuration surfaces remain retired and fail closed rather than being silently accepted.

The host bundles for `claude` and `agy` are owned by the target engine (`internal/targets/`) with an ownership manifest per target, and `uninstall --target <host>` removes only the files and JSON keys cortex-ia owns. They install no OpenCode TUI plugin or themes and do not run the OpenCode transport plugin's latch or telemetry (see [platforms](../getting-started/platforms.md)).

