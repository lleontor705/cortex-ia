# SDD Workflow — Single-Agent Mode

You follow a Spec-Driven Development (SDD) workflow for substantial changes. Since you operate as a single agent (no sub-agent delegation), you execute all phases sequentially yourself.

## Memory Protocol (Cortex v0.2.1)

At session start:
1. Call `mem_session_start(id: "session-{timestamp}", project: "{project}", directory: "{cwd}")` to register session.
2. Call `mem_context` to load previous session context.
3. Review prior decisions, outstanding work, and established patterns.

During work:
- Call `mem_save` after significant decisions, bug fixes, discoveries, or convention changes.
- Use `topic_key` for evolving topics to enable upsert.
- Use `mem_suggest_topic_key(type, title)` when unsure about the right topic_key.
- Use `mem_relate` to connect related observations in the knowledge graph.
- Use `mem_save_prompt(content, project)` to save user prompts for intent tracking.
- Use `mem_capture_passive(content, project)` to extract learnings from large outputs.
- Use `mem_revision_history(observation_id)` to see how an observation evolved across upserts.
- Use `mem_timeline(observation_id)` to see chronological context around an observation.
- Use `mem_search_hybrid(query, project)` for FTS5 + vector combined search when `mem_search` is insufficient.

At session end:
- Call `mem_session_summary` with: goal, accomplishments, decisions, next steps, relevant files.
- Call `mem_session_end(id: "{session_id}", summary: "{brief}")` to close session.

Project hygiene:
- If project name is fragmented: `mem_merge_projects(from: "variant1,variant2", to: "canonical-name")`
- To soft-delete obsolete observations: `mem_archive(observation_id)`
- To permanently delete: `mem_delete(id, hard_delete: true)` (admin only)

## Auto-Bootstrap

Before running any SDD phase, verify the skill registry exists:

1. Call `mem_search(query: "skill-registry", project: "{project}")`
2. If NOT found, check: does `.sdd/skill-registry.md` exist in the project root?
3. If NEITHER exists → run `/sdd-init` first to bootstrap the project (detects stack, builds skill registry).
4. If found → proceed normally.

This check runs ONCE per session. After bootstrap completes (or registry exists), skip for the rest of the session.

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

### Skills Directory

SDD skill instructions are installed at: `{{SKILLS_DIR}}/`
When executing any phase, read the corresponding skill file first (e.g. `{{SKILLS_DIR}}/investigate/SKILL.md` for explore phase).

### Model Assignments

{{MODEL_ASSIGNMENTS}}

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

Additional ForgeSpec tools available:
- `sdd_save(contract)` → persist validated contract to history
- `sdd_get(contract_id)` → retrieve a specific contract by ID
- `sdd_list(project, phase?, limit?)` → query contracts with filters
- `sdd_history(project)` → phase transition timeline
- `sdd_phases()` → get all phase metadata (thresholds, transitions)

### Task Board (ForgeSpec)

During the apply phase, use the task board for tracking:
1. `tb_create_board(project: "{project}", name: "{change}")` — create board
2. `tb_add_task(board_id, title, description, priority, spec_ref, acceptance_criteria, dependencies)` — add tasks from decompose
3. `tb_unblocked(board_id)` — find ready tasks (no unresolved deps)
4. `tb_claim(task_id, agent)` — start working on a task
5. `tb_update(task_id, status: "done", notes?)` — mark complete; auto-unblocks dependents
6. `tb_add_notes(task_id, notes)` — append timestamped notes without changing status
7. `tb_delete_task(task_id)` — remove task (only backlog/done status)
8. `tb_get(task_id)` — get full task details
9. `tb_list(project?)` — list all boards

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

### Adaptive Pipeline

The pipeline depth is NOT fixed. Adjust dynamically:

ESCALATION:
- If apply produces confidence < 0.6 → add verify + archive even if fast-tracked
- If verify finds 3+ spec violations → re-run design → tasks → apply (redesign cycle)
- If the same phase fails validation 2 times → HALT, ask user for guidance

DE-ESCALATION:
- If all phases return confidence > 0.9 → skip archive retrospective
- If change is trivial → remember for next similar change (save to project context)

CHECKPOINT:
- Every 4 tasks during apply: save progress via `mem_save(topic_key: "sdd/{change}/apply-progress")`
- Include: `{ completed: [...], failed: [...], remaining: [...] }`

RUNAWAY PREVENTION:
- Max 2 redesign cycles (verify → design → apply). After 2: HALT
- If any phase has been retried 3 times (network + validation combined): HALT

### Recovery

If you lose context (compaction) or variables seem undefined:
1. IMMEDIATELY call `mem_context` to recover session state
2. Call `mem_search(query: "sdd/{change}/state", project: "{project}")` for pipeline progress
3. Check `tb_status` for any in-progress task board
4. Resume from last completed phase — NEVER restart from scratch
5. If change name is unknown: `mem_search(query: "sdd/", project: "{project}")` to find active changes
