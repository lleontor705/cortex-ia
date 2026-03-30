# SDD Workflow — Single-Agent Mode

You follow a Spec-Driven Development (SDD) workflow for substantial changes. Since you operate as a single agent (no sub-agent delegation), you execute all phases sequentially yourself.

## Memory Protocol (Cortex)

At session start:
1. Call `mem_context` to load previous session context.
2. Review prior decisions, outstanding work, and established patterns.

During work:
- Call `mem_save` after significant decisions, bug fixes, discoveries, or convention changes.
- Use `topic_key` for evolving topics to enable upsert.
- Use `mem_relate` to connect related observations in the knowledge graph.

At session end:
- Call `mem_session_summary` with: goal, accomplishments, decisions, next steps, relevant files.

## SDD Pipeline (9 Phases)

```
init → explore → propose → spec → design → tasks → apply → verify → archive
```

### Phase Dependencies
```
proposal → spec ──┐
         ↘        ├→ tasks → apply → verify → archive
         design ──┘
```

Spec and design can run in parallel (but since you're a single agent, run them sequentially).

### Commands

- `/sdd-init` — Detect project stack, bootstrap persistence, build skill registry
- `/sdd-explore <topic>` — Investigate an idea; read codebase, compare approaches
- `/sdd-new <change>` — Start new change: explore → propose
- `/sdd-ff <name>` — Fast-forward planning: propose → spec → design → tasks
- `/sdd-continue [change]` — Run next dependency-ready phase
- `/sdd-apply [change]` — Implement tasks following specs and design
- `/sdd-verify [change]` — Validate implementation against specs
- `/sdd-archive [change]` — Close change and persist final state

### Artifact Persistence

Store artifacts via Cortex memory:

| Phase | Artifact Type | Topic Key |
|-------|--------------|-----------|
| init | project context | `bootstrap/{project}` |
| explore | exploration | `sdd/{change}/explore` |
| propose | proposal | `sdd/{change}/proposal` |
| spec | specifications | `sdd/{change}/spec` |
| design | architecture | `sdd/{change}/design` |
| tasks | task breakdown | `sdd/{change}/tasks` |
| apply | progress | `sdd/{change}/apply-progress` |
| verify | report | `sdd/{change}/verify-report` |
| archive | archive | `sdd/{change}/archive-report` |

Save with: `mem_save(title: "{topic-key}", topic_key: "{topic-key}", type: "architecture", scope: "project", project: "{project}", content: "{artifact}")`

Read with two steps (CRITICAL — search returns 300-char previews only):
1. `mem_search(query: "{topic-key}", project: "{project}")` → get observation ID
2. `mem_get_observation(id: {id})` → full content

### Contract Validation (ForgeSpec)

After completing each phase, validate your output:
```
sdd_validate(phase: "{phase}", contract: {JSON contract})
```

Contract format:
```json
{
  "schema_version": "1.0",
  "phase": "{phase}",
  "change_name": "{change}",
  "project": "{project}",
  "status": "success",
  "confidence": 0.85,
  "executive_summary": "...",
  "artifacts_saved": [{"topic_key": "...", "type": "cortex"}],
  "next_recommended": ["{next-phase}"],
  "risks": []
}
```

Confidence thresholds per phase:
- init/explore: 0.5 (exploratory)
- propose/design: 0.7 (planning)
- spec/tasks: 0.8 (critical)
- apply: 0.6 (partial OK)
- verify/archive: 0.9 (quality gates)

### Task Board (ForgeSpec)

During the apply phase, use the task board for tracking:
1. `tb_create_board(project: "{project}", name: "{change}")` — create board
2. `tb_add_task(board_id, title, description, priority, dependencies)` — add tasks from decompose output
3. `tb_unblocked(board_id)` — find ready tasks
4. `tb_claim(task_id)` — start working on a task
5. `tb_update(task_id, status: "done")` — mark complete; auto-unblocks dependents

### File Reservation (ForgeSpec)

Before modifying files, reserve them to prevent conflicts in multi-agent setups:
```
file_reserve(patterns: ["src/auth/**/*.ts"], agent: "{your-agent-name}")
```

After completing work:
```
file_release(agent: "{your-agent-name}")
```

### Execution Mode

When starting a new change, decide based on complexity:

| Complexity | Confidence | Files | Pipeline |
|-----------|-----------|-------|----------|
| Trivial | >= 0.9 | <= 2 | apply → verify |
| Simple | >= 0.8 | <= 5 | propose → apply → verify |
| Normal | >= 0.6 | any | Full pipeline |
| Complex | < 0.6 | any | Full pipeline + archive |

### Recovery

If you lose context (compaction):
1. Call `mem_context` to recover session state
2. Check `mem_search(query: "sdd/{change}/state")` for pipeline progress
3. Resume from last completed phase
