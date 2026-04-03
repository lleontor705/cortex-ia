# SDD Orchestrator Reference

Detailed protocols loaded on demand. For the main orchestrator prompt, see the primary orchestrator file.

---
## Apply Phase (Team-Lead Pattern)

After @decompose produces task breakdown:

**Step 1 — Persist plan**: Call `tb_create_board` with the JSON from decompose output.

**Step 2 — Analyze groups**: Classify parallel groups as independent (no cross-group deps) or dependent.

**Step 3 — Launch team-leads**: Launch all team-leads in a single turn.

For independent groups:
```
task(@team-lead, prompt: "
  Execute apply phase for Group {N}.
  Change: {change-name} | Project: {project} | Board: {board_id}
  YOUR TASKS: {task IDs} | MODE: independent — execute immediately.
  artifact_store.mode: {mode} | ENABLED CLIs: {list}
  After completion, broadcast: msg_broadcast(sender: 'team-lead-{N}', subject: 'Group {N} complete', body: '{completed/failed IDs}', priority: 'high')
")
```

For dependent groups:
```
task(@team-lead, prompt: "
  Execute apply phase for Group {N}.
  Change: {change-name} | Project: {project} | Board: {board_id}
  YOUR TASKS: {task IDs} | MODE: dependent — WAIT FOR Groups {X, Y}.
  WAITING: 1. agent_register(name: 'team-lead-{N}', role: 'apply-coordinator')
  2. Poll msg_read_inbox every 30s until all required group completions received (max 10 min)
  3. If timeout → report blocked via msg_send
  After completion, broadcast same as independent groups.
  artifact_store.mode: {mode} | ENABLED CLIs: {list}
")
```

Dependent team-leads self-coordinate via messaging; you do not sequence them.

**If all groups are sequential**: fall back to a single @team-lead executing groups in order.

**If only 1 task**: skip team-lead, delegate directly to @implement.

**Step 4 — Process reports**: Collect all reports, merge, validate with `sdd_validate(phase: "apply", ...)`. On partial failure, ask user whether to retry failed tasks or proceed. On blocked, report and ask for guidance.

**Step 4b — DLQ check on failures**: If partial failure reported:
1. `dlq_list()` — check for messages that failed delivery during apply
2. If DLQ has entries from current change: `dlq_retry(dlq_id)` to replay lost coordination
3. If >= 30% tasks failed: present DLQ + tb_status to user before deciding retry vs proceed

**Step 5 — Recovery after compaction**: `tb_status` for board state, `msg_activity_feed(minutes: 60)` for completion messages, `dlq_list()` for failed deliveries during compaction, `dlq_retry(dlq_id)` for recoverable entries, re-launch only incomplete team-leads.

---
## Debate Mode

Triggers: user requests /debate, investigate returns 2+ approaches within 0.1 confidence, or contentious architectural decision.

Protocol:
1. Create task board entries per position via `tb_create_board`
2. Spawn N @investigate agents in parallel, each defending one position
3. Prompt: "ADVERSARIAL ROLE: Defend position '{X}'. Challenge opponents via msg_send. 3 rounds, 1 message each."
4. Skip non-responsive agents after 2 timeouts
5. After round 3: read threads via `msg_activity_feed(limit: 50)`
6. Tie-breaker: fewest unaddressed counterarguments > codebase alignment > lower risk
7. Persist: `mem_save(title: "sdd/{change-name}/debate", topic_key: "sdd/{change-name}/debate", type: "architecture", ...)`
8. Present summary with winner and rationale

### A2A-Based Debate (preferred for 3+ positions or audit trail)

1. For each position: `a2a_submit_task(from_agent: "orchestrator", to_agent: "{defender}", message: "ADVERSARIAL ROLE: Defend '{X}'. Context: {evidence}")`
2. Defenders respond: `a2a_respond_task(task_id, message: "{argument}", status: "completed")`
3. Track rounds: `a2a_get_task(task_id)` — states: submitted → working → completed
4. Non-responsive: `a2a_cancel_task(task_id)` after 2 polls (replaces skip heuristic)
5. Full audit: `a2a_list_tasks(agent: "orchestrator")` — machine-parseable debate history
6. Tie-breaker and persist: same as P2P protocol (steps 6-8 above)

Use A2A for round lifecycle. Use msg_send for ad-hoc challenges within rounds.

---
## Resource Coordination

### File Locks vs Resource Locks

| Resource Type | Tool | Example |
|--------------|------|---------|
| Source files during apply | `file_reserve` / `file_check` / `file_release` (ForgeSpec) | `patterns: ["src/auth/**"]` |
| Deploy environments | `resource_acquire` / `resource_release` (Mailbox) | `resource_id: "deploy-staging"` |
| CI pipeline slots | `resource_acquire` (shared) | `resource_id: "ci-pipeline", lease_type: "shared"` |
| External API rate limits | `resource_acquire` (shared, short TTL) | `resource_id: "github-api", ttl_seconds: 60` |
| Database migrations | `resource_acquire` (exclusive) | `resource_id: "db-migration"` |

**Rule**: `file_reserve` for file-pattern conflicts in apply phase. `resource_acquire` for everything else.

### Dead-Letter Queue

| Scenario | Action |
|----------|--------|
| Compaction recovery | `dlq_list()` → `dlq_retry(dlq_id)` for current-change messages |
| Dependent team-lead timeout | Check `dlq_list()` — upstream broadcast may be in DLQ |
| Persistent task failures | Failures with messaging component land in DLQ automatically |
| Between changes | `dlq_purge()` to clear stale entries |

---
## Tools Reference

- **Coordination**: task, question, skill
- **Progress**: todowrite, todoread
- **Task Board**: tb_create_board, tb_status, tb_unblocked, tb_claim, tb_update, tb_get, tb_add_task, tb_add_notes, tb_delete_task, tb_list
- **Validation**: sdd_validate, sdd_save, sdd_get, sdd_list, sdd_history, sdd_phases
- **Communication**: msg_send, msg_read_inbox, msg_broadcast, msg_acknowledge, msg_search, msg_request, msg_list_threads, msg_get, msg_delete, msg_count, msg_update_status, msg_list_agents, msg_activity_feed, agent_register
- **File Locks**: file_reserve, file_check, file_release
- **A2A Tasks**: a2a_submit_task, a2a_get_task, a2a_cancel_task, a2a_list_tasks, a2a_respond_task
- **Resource Locks**: resource_acquire, resource_release, resource_check, resource_list
- **Dead-Letter Queue**: dlq_list, dlq_retry, dlq_purge
- **CLI Routing**: cli_execute, cli_route, cli_stats, cli_list
- **Memory (core)**: mem_save, mem_search, mem_get_observation, mem_context, mem_session_summary, mem_update, mem_capture_passive, mem_save_prompt, mem_suggest_topic_key
- **Memory (session)**: mem_session_start, mem_session_end, mem_stats, mem_delete
- **Memory (graph)**: mem_relate, mem_graph, mem_score, mem_search_hybrid, mem_archive, mem_timeline, mem_revision_history, mem_merge_projects
- **Memory (temporal)**: temporal_create_edge, temporal_get_edges, temporal_get_relevant, temporal_create_snapshot, temporal_record_operation, temporal_evaluate_quality, temporal_system_metrics, temporal_health_check, temporal_evolution_path, temporal_fact_state
