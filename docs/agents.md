# Agent Reference

cortex-ia supports 8 AI coding agents. Each agent has a dedicated adapter that knows its config paths, MCP strategy, and capabilities.

## Claude Code

| Property | Value |
|----------|-------|
| Binary | `claude` |
| Config dir | `~/.claude` |
| System prompt | `~/.claude/CLAUDE.md` |
| Skills dir | `~/.claude/skills/` |
| MCP config | `~/.claude/mcp/<server>.json` (separate file per server) |
| Prompt strategy | Markdown sections (`<!-- cortex-ia:ID -->`) |
| MCP strategy | Separate JSON files |
| Task delegation | Yes (Task tool) |

## OpenCode

| Property | Value |
|----------|-------|
| Binary | `opencode` |
| Config dir | `~/.config/opencode` |
| System prompt | `~/.config/opencode/AGENTS.md` |
| Skills dir | `~/.config/opencode/skills/` |
| Settings | `~/.config/opencode/opencode.json` |
| Commands dir | `~/.config/opencode/commands/` |
| MCP strategy | Merge into `opencode.json` (`"mcp"` key) |
| Task delegation | Yes (task tool) |
| Sub-agents | Yes (agent config in opencode.json) |
| Slash commands | Yes (10 SDD commands) |

## Gemini CLI

| Property | Value |
|----------|-------|
| Binary | `gemini` |
| Config dir | `~/.gemini` |
| System prompt | `~/.gemini/GEMINI.md` |
| Skills dir | `~/.gemini/skills/` |
| Settings | `~/.gemini/settings.json` |
| MCP strategy | Merge into `settings.json` (`"mcpServers"` key) |
| Task delegation | No (single-agent SDD) |

## Cursor

| Property | Value |
|----------|-------|
| Config dir | `~/.cursor` |
| System prompt | `~/.cursor/rules/cortex-ia.mdc` |
| Skills dir | `~/.cursor/skills/` |
| MCP config | `~/.cursor/mcp.json` |
| MCP strategy | MCP config file (merge into mcp.json) |
| Sub-agents | Yes (`~/.cursor/agents/`) |

## VS Code Copilot

| Property | Value |
|----------|-------|
| Config dir | `~/.copilot` |
| System prompt | `{vscode-user}/prompts/cortex-ia.instructions.md` |
| Skills dir | `~/.copilot/skills/` |
| MCP config | `{vscode-user}/mcp.json` (`"servers"` key) |
| MCP strategy | MCP config file |
| Task delegation | Yes |

VS Code User directory varies by platform:
- **macOS**: `~/Library/Application Support/Code/User`
- **Windows**: `%APPDATA%\Code\User`
- **Linux**: `~/.config/Code/User`

## Codex

| Property | Value |
|----------|-------|
| Binary | `codex` |
| Config dir | `~/.codex` |
| System prompt | `~/.codex/agents.md` |
| Skills dir | `~/.codex/skills/` |
| MCP config | `~/.codex/config.toml` (`[mcp_servers.<name>]`) |
| MCP strategy | TOML file |

## Windsurf

| Property | Value |
|----------|-------|
| Config dir | `~/.codeium/windsurf` |
| System prompt | `~/.codeium/windsurf/memories/global_rules.md` |
| Skills dir | `~/.codeium/windsurf/skills/` |
| MCP config | `~/.codeium/windsurf/mcp_config.json` |
| Prompt strategy | Append to file |
| MCP strategy | MCP config file |

## Antigravity

| Property | Value |
|----------|-------|
| Config dir | `~/.gemini/antigravity` |
| System prompt | `~/.gemini/GEMINI.md` (shared with Gemini CLI) |
| Skills dir | `~/.gemini/antigravity/skills/` |
| MCP config | `~/.gemini/antigravity/mcp_config.json` |
| Prompt strategy | Append to file |
| MCP strategy | MCP config file |

## MCP Strategy Details

### Separate JSON Files (Claude Code)
One file per MCP server in `~/.claude/mcp/`:
```json
{"command": "cortex", "args": ["mcp"]}
```

### Merge Into Settings (OpenCode, Gemini)
Deep-merged into the agent's settings file. Existing keys preserved.

OpenCode uses `"mcp"` key with `"type": "local"`:
```json
{"mcp": {"cortex": {"type": "local", "command": ["cortex", "mcp"], "enabled": true}}}
```

Gemini uses `"mcpServers"` key:
```json
{"mcpServers": {"cortex": {"command": "cortex", "args": ["mcp"]}}}
```

### MCP Config File (Cursor, VS Code, Windsurf, Antigravity)
Deep-merged into a dedicated MCP config file (mcp.json or mcp_config.json).

VS Code uses `"servers"` key:
```json
{"servers": {"cortex": {"type": "stdio", "command": "cortex", "args": ["mcp"]}}}
```

### TOML File (Codex)
Upserted as TOML blocks in `~/.codex/config.toml`:
```toml
[mcp_servers.cortex]
command = "cortex"
args = ["mcp"]
```
