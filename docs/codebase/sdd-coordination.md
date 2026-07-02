# SDD Multi-Agent Coordination

← [Codebase Guide](../CODEBASE-GUIDE.md)

How the Spec-Driven Development multi-agent infrastructure coordinates execution across parallel workers. This page covers the coordination protocols (messaging, A2A tasks, file reservations, task boards, DLQ) — not the SDD skill content itself (see `internal/assets/skills/`) or the contract validation mechanics (see [mcp-boundaries.md](mcp-boundaries.md)).

## SDD Pipeline Phases

```
investigate → propose → spec ──→ tasks → apply → verify → archive
                          ↑
                        design (parallel with spec)
```

| Phase | Skill | Produces | Writes? |
|-------|-------|----------|---------|
| init | bootstrap | project context | YES |
| explore | investigate | exploration notes | READ+WEB |
| propose | draft-proposal | proposal artifact | NO |
| spec | write-specs | spec (Given/When/Then) | NO |
| design | architect | design artifact | NO |
| tasks | decompose | task board + task list | NO |
| apply | team-lead → @implement | production code | YES |
| verify | validate | verify-report | READ+WEB |
| archive | finalize | archive-report | YES |

"NO write" phases persist artifacts to Cortex (`mem_save`), not to files.

## Ownership: Who Does What

| Role | Owned by | Method |
|------|----------|--------|
| Pipeline orchestration | Principal Orchestrator | Task delegation (not P2P) |
| Apply-phase coordination | @team-lead | Owns `apply-progress` artifact; launches @implement per task |
| Single-task implementation | @implement | Writes code; reports via `tb_update` |
| Phase contracts | ForgeSpec | `sdd_validate` + `sdd_save` |
| Task dependency graph | ForgeSpec | `tb_create_board`, `tb_claim`, `tb_status` |
| File conflict prevention | ForgeSpec | `file_reserve` / `file_release` |
| Completion signals | Agent Mailbox | `msg_broadcast` |
| Blocking clarifications | Agent Mailbox | `msg_request` (sync) |
| Formal tracked delegation | Agent Mailbox | `a2a_submit_task` / `a2a_respond_task` |
| Lost message recovery | Agent Mailbox | `dlq_list` / `dlq_retry` |
| External resource locks | Agent Mailbox | `resource_acquire` / `resource_release` |

## Team-Lead → @implement Delegation

The apply phase is the only phase with internal sub-delegation. The orchestrator delegates the entire apply phase to @team-lead, which in turn launches one @implement per task.

```
Orchestrator
   │  Task delegation (whole apply phase)
   ▼
@team-lead  ──▶  owns sdd/{change}/apply-progress
   │
   ├──▶ Task(@implement, task-1)   ── tb_update(done)
   ├──▶ Task(@implement, task-2)   ── tb_update(done)
   └──▶ Task(@implement, task-3)   ── tb_update(done)
```

Rules:
- @implement is a leaf agent — it never spawns further agents.
- @implement reports progress ONLY via `tb_update(task_id, status, notes)`. It does NOT write `apply-progress` (team-lead owns that to prevent upsert races).
- @team-lead merges progress into `apply-progress` after each group completes.

## Parallel Group Execution

Task dependencies form a DAG. The orchestrator classifies groups:

| Group type | Behavior |
|------------|----------|
| Independent | No task depends on another group → start immediately. `MODE: independent` |
| Dependent | Tasks depend on an upstream group's tasks → wait. `MODE: dependent, WAIT FOR: [groups]` |

All group team-leads are launched in a single turn (parallel). Dependent team-leads self-coordinate:

1. Register with `agent_register(name: "team-lead-{N}")`.
2. Poll `msg_read_inbox` (~30s) for upstream `msg_broadcast` completion signals.
3. Fallback to `tb_status` and `dlq_list` on timeout.

The orchestrator does NOT sequence dependent groups manually — they wait autonomously via P2P messaging.

## A2A Task Lifecycle

For formal, auditable delegation (vs. fire-and-forget Task calls):

| Stage | Tool | Status |
|-------|------|--------|
| Submit | `a2a_submit_task` | created |
| Assignee works | — | `working` |
| Needs input | `a2a_respond_task(status: "input-required")` | input-required |
| Done | `a2a_respond_task(status: "completed")` | completed |
| Failed | `a2a_respond_task(status: "failed")` | failed |
| Cancel | `a2a_cancel_task` | canceled |

- Check status: `a2a_get_task(task_id)`.
- Cancel is only valid for non-terminal tasks.
- Prefer A2A over `msg_request` when the clarification blocks work AND needs an audit trail.

## File Reservations (Conflict Prevention)

Multi-agent file writes are the primary source of corruption. ForgeSpec file reservations are the ONLY sanctioned mechanism.

| Action | Tool |
|--------|------|
| Reserve files/globs | `file_reserve(patterns, agent, ttl_minutes)` |
| Check for conflicts without reserving | `file_reserve(check_only: true)` |
| Release reservations | `file_release(agent, patterns?)` |
| Inspect current lease | `file_check` (Mailbox, for external resources only) |

Rules:
- Reservations are advisory with a TTL (default 15 min). They do NOT lock the filesystem.
- Reservations expire automatically; release explicitly when work is done.
- Do NOT use Mailbox `resource_acquire` for file paths — it is for external resources (deploy targets, CI, APIs).

## Dead-Letter Queue (Lost Messages)

Messages that expire or fail delivery land in the Mailbox dead-letter queue.

| Action | Tool |
|--------|------|
| List lost messages | `dlq_list()` |
| Retry a message | `dlq_retry(dlq_id)` |
| Clear all | `dlq_purge()` |

Recovery protocol when an expected response never arrives:
1. `a2a_get_task` / `msg_read_inbox` to confirm absence.
2. `dlq_list` to find the dropped message.
3. `dlq_retry` to re-insert, or re-send if the original is stale.

## Task Board Dependency Tracking

| Tool | Purpose |
|------|---------|
| `tb_create_board` | Create board; optionally inline tasks for atomic creation |
| `tb_add_task` | Add a task with `dependencies` referencing other task IDs |
| `tb_claim` | Claim a `ready` task whose dependencies are resolved |
| `tb_unblocked` | List tasks with no unresolved dependencies |
| `tb_update` | Change status / append timestamped notes |
| `tb_status` | Grouped status snapshot for a board |

`tb_claim` is guarded: only `ready` tasks with all dependencies resolved can be claimed. This is the runtime enforcement of the dependency DAG.

## Invariants

- The orchestrator NEVER does implementation work inline — it delegates all code changes to sub-agents.
- Only the apply phase has sub-delegation (@team-lead → @implement). All other phases are direct orchestrator → phase-agent.
- `apply-progress` is owned by exactly one writer (@team-lead). @implement never writes it.
- File reservations are the only multi-agent write-conflict mechanism. Advisory resource leases are for external resources only.
- A2A tasks can only be canceled while non-terminal.
- Completion signals for dependent groups flow through Mailbox `msg_broadcast`, not through ForgeSpec.

## Contributor Checklist

- [ ] Adding a coordination flow? Prefer A2A tasks (`a2a_submit_task`) when an audit trail matters; use `msg_request` for quick sync clarifications.
- [ ] Launching parallel writers? Have each one `file_reserve` its paths first with `check_only` if you only need to detect conflicts.
- [ ] @implement task? Report via `tb_update` only — never touch `apply-progress`.
- [ ] @team-lead? Write `apply-progress` after each group; poll `msg_read_inbox` if your group is dependent.
- [ ] Missing response? Always check `dlq_list` before assuming an agent is stuck.
- [ ] New task dependency? Declare it via `dependencies` on `tb_add_task` — `tb_claim` will enforce it automatically.

---

← Prev: [MCP Boundaries](mcp-boundaries.md) · Next: [Interfaces](interfaces.md) →
