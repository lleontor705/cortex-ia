# Components

cortex-ia configures 7 components. Each is independently selectable (with automatic dependency resolution).

## MCP Server Components

### Cortex (31 MCP tools)

Persistent cross-session memory with knowledge graph.

**Core** (9): `mem_save`, `mem_search`, `mem_get_observation`, `mem_context`, `mem_session_summary`, `mem_update`, `mem_capture_passive`, `mem_save_prompt`, `mem_suggest_topic_key`
**Session** (4): `mem_session_start`, `mem_session_end`, `mem_stats`, `mem_delete`
**Knowledge graph** (8): `mem_relate`, `mem_graph`, `mem_score`, `mem_search_hybrid`, `mem_archive`, `mem_timeline`, `mem_revision_history`, `mem_merge_projects`
**Temporal** (10): `temporal_create_edge`, `temporal_get_edges`, `temporal_get_relevant`, `temporal_create_snapshot`, `temporal_record_operation`, `temporal_evaluate_quality`, `temporal_system_metrics`, `temporal_health_check`, `temporal_evolution_path`, `temporal_fact_state`

**What gets injected**: MCP server config pointing to `cortex mcp` binary.

**Dependency**: None (but required by SDD and Conventions).

### ForgeSpec (19 MCP tools)

SDD contract validation with task board and file reservation.

**Contract tools** (6): `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history`, `sdd_phases`
**Task board** (10): `tb_create_board`, `tb_add_task`, `tb_status`, `tb_claim`, `tb_update`, `tb_unblocked`, `tb_get`, `tb_list`, `tb_add_notes`, `tb_delete_task`
**File locks** (3): `file_reserve`, `file_check`, `file_release`

**What gets injected**: MCP server config pointing to `npx -y forgespec-mcp`.

### Agent Mailbox (26 MCP tools)

Inter-agent messaging, A2A task delegation, resource coordination, and dead-letter queue.

**Messaging** (9): `msg_send`, `msg_read_inbox`, `msg_acknowledge`, `msg_broadcast`, `msg_search`, `msg_request`, `msg_get`, `msg_delete`, `msg_count`
**Threads & agents** (4): `msg_list_threads`, `msg_activity_feed`, `msg_list_agents`, `agent_register`
**Status** (1): `msg_update_status`
**A2A Tasks** (5): `a2a_submit_task`, `a2a_get_task`, `a2a_cancel_task`, `a2a_list_tasks`, `a2a_respond_task`
**Resources** (4): `resource_acquire`, `resource_release`, `resource_check`, `resource_list`
**Dead-Letter Queue** (3): `dlq_list`, `dlq_retry`, `dlq_purge`

**What gets injected**: MCP server config pointing to `npx -y agent-mailbox-mcp`.

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
agent-mailbox ──┤
forgespec ──────┤
context7        │
                ├──▶ sdd (requires cortex + forgespec + mailbox)
conventions ◄────── cortex
skills          (no deps)
```

## Preset Resolution

### Full Preset
All 7 components: cortex, agent-mailbox, forgespec, context7, conventions, sdd, skills.

### Minimal Preset
Selected: cortex, forgespec, context7, sdd.
After dependency resolution: **+ agent-mailbox** (pulled by SDD).

### Custom Preset
User selects via TUI. Dependencies auto-resolved.
