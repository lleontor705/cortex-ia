# OpenCode Assets & MCP Components

`cortex-ia` deploys a complete, atomic asset set and manages the MCP catalog for **OpenCode** (`~/.config/opencode/`).

---

## 1. Native Workflow Assets

All workflow assets are embedded directly inside the `cortex-ia` binary via `go:embed` and mapped byte-for-byte to OpenCode's native directory structure:

| Asset Kind | Embedded Source | Destination under `~/.config/opencode/` | Purpose |
| :--- | :--- | :--- | :--- |
| **Base Configuration** | `opencode.jsonc` | `opencode.jsonc` | Safe 3-way merge preserving user keys, comments, and permissions. |
| **System Prompt** | `AGENTS.md` | `AGENTS.md` | Core orchestrator system prompt and SDD operational protocol. |
| **Sub-Agents (5)** | `agents/*.md` | `agents/<name>.md` | `orchestrator`, `planner`, `implement`, `investigate`, `reviewer`. |
| **Slash Commands (9)**| `commands/*.md` | `commands/<name>.md` | `/hotfix`, `/sdd`, `/work`, `/review`, `/tdd`, `/spike`, `/status`, `/resume`, `/investigate`. |
| **Native Skills (12)**| `skills/<name>/SKILL.md`| `skills/<name>/SKILL.md` | SDD phase skills & utility skills (`fast-tdd`, `ast-impact-analysis`, `property-based-testing`, etc.). |
| **Plugins** | `plugin/*.ts` | `plugin/*.ts` | Cortex integration, model variants, and optional delegation bridge. |

---

## 2. Managed MCP Server Presets

`cortex-ia` manages two official catalog presets. Task coordination is built into the Go CLI and SQLite store.

### 1. Cortex (`cortex`) — *Default: ON*
- **Execution Vector**: `["cortex", "mcp", "--tools=agent"]`
- **Capabilities**: Cross-session persistent memory, knowledge graph, hybrid search (FTS5 + semantic), temporal evolution history.

### 2. Context7 (`context7`) — *Default: OFF (Optional)*
- **Execution Vector**: `["npx", "-y", "@upstash/context7-mcp"]`
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

## 4. Platform Expansion Roadmap

`cortex-ia` is architected to bring the same SDD and MCP stack to additional platforms in the future:
1. **OpenCode**: Primary active native target (`~/.config/opencode`).
2. **Google Antigravity**: Next target on roadmap (`~/.gemini/antigravity`).
3. **Claude CLI**: Next target on roadmap (`~/.claude`).

