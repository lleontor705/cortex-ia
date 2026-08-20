# MCP Server Boundaries

← [Codebase Guide](../CODEBASE-GUIDE.md)

cortex-ia configures 4 MCP (Model Context Protocol) servers into supported agents. This page documents which servers exist, what each owns, and when each is used at runtime. It does not cover the injection mechanics (see [repository-map.md](repository-map.md)) or the multi-agent coordination protocols built on top of them (see [sdd-coordination.md](sdd-coordination.md)).

> **Correction note:** GGA is documented here as the Guardian Angel pre-commit hook, NOT as an MCP server. The 4 MCP servers cortex-ia integrates with are Cortex, Agent Mailbox, ForgeSpec, and Context7. GGA has no MCP transport — it writes a config file and an `AGENTS.md` review template.

## MCP Server Overview

| Server | Component package | Transport | Installed by | Tools (count) |
|--------|-------------------|-----------|--------------|---------------|
| **Cortex** | `internal/components/cortex` | Go binary (`cortex mcp`) | `cortex` component | ~19 |
| **Agent Mailbox** | `internal/components/mailbox` | npm process | `agent-mailbox` component | ~26 |
| **ForgeSpec** | `internal/components/forgespec` | npm process | `forgespec` component | ~15 |
| **Context7** | `internal/components/context7` | npm/remote | `context7` component | docs lookup |

Injection is performed by each component delegating to `internal/components/mcpinject` with per-strategy `ServerTemplates`. The adapter's `MCPStrategy()` selects which template is rendered — never branched in component code.

## Cortex — Persistent Memory

| Property | Value |
|----------|-------|
| Purpose | Cross-session persistent memory + knowledge graph + session tracking |
| Binary | `cortex mcp` |
| Component | `model.ComponentCortex` |
| Key tools | `mem_save`, `mem_search`, `mem_get_observation`, `mem_context`, `mem_relate`, `mem_graph`, `mem_timeline`, `mem_session_start/end`, `mem_score` |
| When used | Persisting SDD artifacts, decisions, bug fixes; recovering context after compaction; tracing artifact lineage via the knowledge graph |

Ownership boundaries:
- Cortex owns **artifact persistence** (SDD proposal/spec/design/tasks/apply-progress/verify-report/archive-report).
- Cortex owns **topic-key upserts** for evolving topics (same `topic_key` merges; different keys never overwrite).
- Cortex does NOT own task state — that is ForgeSpec.

## Agent Mailbox — Messaging & A2A

| Property | Value |
|----------|-------|
| Purpose | Inter-agent messaging, A2A task lifecycle, resource leases, dead-letter queue |
| Transport | npm process |
| Component | `model.ComponentMailbox` |
| Key tools | `msg_send`, `msg_request`, `msg_broadcast`, `msg_read_inbox`, `agent_register`, `a2a_submit_task`, `a2a_respond_task`, `a2a_get_task`, `dlq_list`, `dlq_retry`, `resource_acquire`, `resource_release` |
| When used | Team-lead → @implement delegation, parallel-group completion signals, blocking clarifications, deployment resource locks |

Ownership boundaries:
- Mailbox owns **message delivery** and the **dead-letter queue** for lost/expired messages.
- Mailbox owns **advisory resource leases** (`resource_acquire`) for deploy/CI/external-API coordination — NOT for file conflicts (use ForgeSpec `file_reserve`).
- A2A tasks carry lifecycle status (`working`, `input-required`, `completed`, `failed`); Mailbox tracks it.
- Mailbox does NOT own agent-filesystem layout — that is ForgeSpec file reservations.

## ForgeSpec — SDD Contracts & Boards

| Property | Value |
|----------|-------|
| Purpose | SDD contract validation/persistence, task boards, file reservations |
| Transport | npm process |
| Component | `model.ComponentForgeSpec` |
| Key tools | `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history`, `tb_create_board`, `tb_add_task`, `tb_claim`, `tb_update`, `tb_status`, `tb_unblocked`, `file_reserve`, `file_release` |
| When used | Validating phase contracts, tracking task dependencies and status, preventing multi-agent file-write conflicts |

Ownership boundaries:
- ForgeSpec owns **SDD contract lifecycle** (`sdd_validate` + `sdd_save` record phase transitions; `sdd_history` shows the trail).
- ForgeSpec owns **task boards** with dependency tracking — `tb_claim` enforces ready-status and resolved dependencies.
- ForgeSpec owns **file reservations** (`file_reserve`) — the ONLY mechanism for multi-agent file-conflict prevention. Not a filesystem lock; an advisory reservation with TTL.
- ForgeSpec does NOT own message delivery — that is Mailbox.

## Context7 — Live Documentation

| Property | Value |
|----------|-------|
| Purpose | Live framework/library documentation lookup via MCP |
| Transport | npm/remote |
| Component | `model.ComponentContext7` |
| Key tools | `resolve-library-id`, `get-library-docs` |
| When used | Looking up current API signatures before implementing against frameworks; preventing stale/deprecated API usage |

Ownership boundaries:
- Context7 owns **live doc retrieval only**. It writes nothing to disk and stores no state.
- Context7 is consulted, not persisted — results are ephemeral.

## When Each Server Is Used

| Lifecycle moment | Servers involved |
|------------------|------------------|
| User installs ecosystem | All 4 injected via component adapters |
| SDD phase produces artifact | Cortex (`mem_save`) + ForgeSpec (`sdd_validate`, `sdd_save`) |
| Team-lead delegates to @implement | Mailbox (`msg_send` / `a2a_submit_task`) + ForgeSpec (`tb_claim`, `tb_update`) |
| Parallel agents write files | ForgeSpec (`file_reserve` / `file_release`) |
| Implementation needs API docs | Context7 (`resolve-library-id`, `get-library-docs`) |
| Message delivery fails | Mailbox (`dlq_list`, `dlq_retry`) |
| Deployment / CI coordination | Mailbox (`resource_acquire` / `resource_release`) |

## Invariants

- Exactly 4 MCP servers are supported: Cortex, Agent Mailbox, ForgeSpec, Context7.
- GGA is a pre-commit hook component, NOT an MCP server — it writes config + templates, no MCP transport.
- MCP server selection is adapter-driven via `MCPStrategy()`; components never switch on `AgentID`.
- File-reservation ownership belongs exclusively to ForgeSpec; resource-lease ownership belongs exclusively to Mailbox. Do not cross them.
- Context7 is read-only and stateless; never treat it as a persistence layer.

## Contributor Checklist

- [ ] Adding an MCP server? Add `ServerTemplates` per strategy in a `config.go`, write an `inject.go` delegating to `mcpinject.Inject()`, register a `ComponentID`, and add it to `pipeline.buildInjectors()`.
- [ ] Changing tool counts in descriptions? Update the catalog `ComponentInfo` strings in `internal/catalog/components.go`.
- [ ] Debugging lost inter-agent messages? Check Mailbox `dlq_list()` — do not assume Cortex dropped them.
- [ ] File-write conflicts between parallel agents? Use ForgeSpec `file_reserve()` — do NOT use Mailbox `resource_acquire` (advisory leases are for external resources, not filesystem paths).
- [ ] Verifying phase transitions persisted? Query ForgeSpec `sdd_history(project)` for the full trail.

---

← Prev: [Repository Map](repository-map.md) · Next: [SDD Coordination](sdd-coordination.md) →
