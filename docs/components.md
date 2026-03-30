# Components

cortex-ia configures 8 components. Each is independently selectable (with automatic dependency resolution).

## MCP Server Components

### Cortex (19 MCP tools)

Persistent cross-session memory with knowledge graph.

**Key tools**: `mem_save`, `mem_search`, `mem_get_observation`, `mem_context`, `mem_session_summary`, `mem_relate`, `mem_graph`, `mem_score`, `mem_archive`, `mem_search_hybrid`

**What gets injected**: MCP server config pointing to `cortex mcp` binary.

**Dependency**: None (but required by SDD and Conventions).

### ForgeSpec (15 MCP tools)

SDD contract validation with task board and file reservation.

**Contract tools**: `sdd_validate`, `sdd_save`, `sdd_history`, `sdd_phases`
**Task board**: `tb_create_board`, `tb_add_task`, `tb_status`, `tb_claim`, `tb_update`, `tb_unblocked`, `tb_get`, `tb_list`
**File locks**: `file_reserve`, `file_check`, `file_release`

**What gets injected**: MCP server config pointing to `npx -y forgespec-mcp`.

### Agent Mailbox (9 MCP tools)

Inter-agent messaging for P2P communication.

**Tools**: `msg_send`, `msg_read_inbox`, `msg_acknowledge`, `msg_broadcast`, `msg_search`, `msg_request`, `msg_list_threads`, `agent_register`, `msg_list_agents`

**What gets injected**: MCP server config pointing to `npx -y agent-mailbox-mcp`.

### CLI Orchestrator (4 MCP tools)

Multi-CLI routing with circuit breaker and automatic fallback.

**Tools**: `cli_execute`, `cli_stats`, `cli_list`, `cli_route`

**What gets injected**: MCP server config pointing to `npx -y cli-orchestrator-mcp`.

### Context7 (2 MCP tools)

Live framework and library documentation.

**Tools**: `resolve-library-id`, `get-library-docs`

**What gets injected**: MCP server config. Uses remote HTTP for OpenCode/VS Code, npx for others.

## Content Components

### SDD Workflow

The largest component. Injects:
1. **Orchestrator prompt** — Multi-agent (Claude/OpenCode) or single-agent (others)
2. **19 skill files** — One SKILL.md per SDD phase + utility skills
3. **Shared conventions** — `_shared/cortex-convention.md`
4. **Slash commands** — 10 command files (OpenCode only)
5. **Sub-agent definitions** — Agent config files (OpenCode, Cursor)

**Dependencies**: Cortex, ForgeSpec, Agent Mailbox.

### Conventions

Injects cortex memory protocol and naming conventions:
1. `cortex-convention.md` → skills/_shared/ directory
2. `cortex-protocol.md` → system prompt (markdown section or append)

**Dependency**: Cortex.

### Extra Skills

Non-SDD utility skills. Injected separately to avoid conflicts with SDD component.

## Dependency Graph

```
cortex ─────────┐
cli-orchestrator │  (no deps, injected independently)
agent-mailbox ──┤
forgespec ──────┤
context7        │
                ├──▶ sdd (requires cortex + forgespec + mailbox)
conventions ◄────── cortex
skills          (no deps)
```

## Preset Resolution

### Full Preset
All 8 components: cortex, cli-orchestrator, agent-mailbox, forgespec, context7, conventions, sdd, skills.

### Minimal Preset
Selected: cortex, forgespec, context7, sdd.
After dependency resolution: **+ agent-mailbox** (pulled by SDD).

### Custom Preset
User selects via TUI. Dependencies auto-resolved.
