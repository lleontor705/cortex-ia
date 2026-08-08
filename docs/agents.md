# Agent Reference

cortex-ia supports exactly four AI coding agent targets: Claude Code, OpenCode, VS Code Copilot, and Codex. Each adapter reports evidence-backed capability facts and renders only the profile that the compiler qualifies. Configuration surface does not by itself prove runtime enforcement.

## Manifest-Backed Support Matrix

This matrix mirrors `internal/components/sdd/renderers/testdata/conformance/index.golden.json`. “Tested” means the repository has a deterministic renderer/conformance fixture with the same portable semantic digest. It does not claim universal behavior across every runtime version, model, host, or account tier.

| Target | Sequential | Flat | Native advanced | Qualification note |
|--------|:----------:|:----:|:---------------:|--------------------|
| Claude Code | ✅ | ✅ | qualified fixture* | Flat proves direct-child only; native adds only manifested qualified capabilities |
| Codex | ✅ | ✅ | qualified fixture* | Stronger profiles remain version/evidence bounded |
| OpenCode | ✅ | ✅ | qualified fixture* | Experimental native extension remains default-off |
| VS Code Copilot | ✅ | advisory only | — | Direct-child preview is documentation/prompt-backed and stays sequential |

Profile rules:

- `portable-sequential` requires no delegation.
- `portable-flat` requires fresh runtime-enforced direct-child evidence and never assumes nested delegation.
- `native-advanced` requires all requested native capabilities to be qualified. Every experimental native capability additionally requires **explicit operator opt-in** and is never selected implicitly.
- Missing, stale, documentation-only, or prompt-only evidence degrades conservatively and appears in machine/human manifests.

## What the Manifests Guarantee

Generated bundles expose semantic, security, and degradation information in runtime-appropriate paths. The manifests record the target/profile, schema/compiler/workflow/catalog versions, generation fingerprint, evidence freshness and experimental status, four-state resolutions, enforcement class, substitutions, requested/effective permissions, approval/isolation intent, trust boundaries, opaque secret references, external services/version intervals, hashes, validation findings, and visible degradations.

Capability state and enforcement are separate:

| Capability state | Meaning |
|------------------|---------|
| `native` | The selected binding is target-native for the qualified interval |
| `emulated` | A named substitute preserves the declared outcome and consequence is disclosed |
| `advisory` | Guidance exists, but cortex-ia does not claim enforcement |
| `unsupported` | Required use blocks; optional use needs a declared degradation |

Enforcement is classified as `runtime`, `hook`, `mcp`, `prompt`, or `none`. A prompt is advisory; manifest validation rejects prompt-only “enforced” claims and silent permission widening.

## Service Ownership and Runtime Limits

- **ForgeSpec** is the upstream authority for SDD contracts and task dependencies/readiness/claim/status. Its transactional task capability is a versioned external dependency, not cortex-ia code.
- **Cortex** owns durable memory, evidence, provenance, and relationships.
- **Agent Mailbox** provides optional messaging and coordination tools but does not replace ForgeSpec task authority.
- **cortex-ia** configures these services and compiles/installs their calls. It does not schedule tasks, launch workers, own live runs, or migrate runtime state.

Runtime-native state may optimize execution but remains non-authoritative unless an explicit ForgeSpec binding maps it. Credentialed runtime qualification is opt-in, isolated, budgeted, redacted, and attributable; fewer than three valid trials or any flaky/insufficient evidence remains inconclusive.

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
| Settings | `~/.config/opencode/opencode.jsonc` when present, otherwise `opencode.json` |
| Commands dir | `~/.config/opencode/commands/` |
| MCP strategy | Patch the effective JSONC/JSON settings (`"mcp"` key) |
| Task delegation | Yes (task tool) |
| Sub-agents | Yes (agent config in effective JSONC/JSON settings) |
| Slash commands | Yes (10 SDD commands) |

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

## MCP Strategy Details

### Separate JSON Files (Claude Code)
One file per MCP server in `~/.claude/mcp/`:
```json
{"command": "cortex", "args": ["mcp", "--tools=agent"]}
```

### Merge Into Settings (OpenCode)
Patched into the effective settings file. Existing keys are preserved; JSONC comments and trailing commas survive managed mutations.

OpenCode uses `"mcp"` key with `"type": "local"`:
```json
{"mcp": {"cortex": {"type": "local", "command": ["cortex", "mcp", "--tools=agent"], "enabled": true}}}
```

### MCP Config File (VS Code Copilot)
Deep-merged into a dedicated MCP config file (mcp.json or mcp_config.json).

VS Code uses `"servers"` key:
```json
{"servers": {"cortex": {"type": "stdio", "command": "cortex", "args": ["mcp", "--tools=agent"]}}}
```

### TOML File (Codex)
Upserted as TOML blocks in `~/.codex/config.toml`:
```toml
[mcp_servers.cortex]
command = "cortex"
args = ["mcp", "--tools=agent"]
```
