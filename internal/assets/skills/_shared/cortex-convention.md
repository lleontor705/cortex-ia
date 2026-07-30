# Cortex Convention for SDD Agents

Shared protocol for all SDD agents. Reference this file instead of duplicating these patterns in individual skills.

## Common Rules (Single Source of Truth)

> **Do not duplicate these rules in skill files.** Reference this section instead.

### File Locks vs Resource Locks
- Use `file_reserve` / `file_release` for **file conflicts** between agents working on the same codebase
- Use `resource_acquire` / `resource_release` for **external resources** (deploy targets, CI, API endpoints)
- Do NOT use `resource_acquire` for file conflicts

### Two-Step Memory Retrieval
- `mem_search` returns 300-char previews only
- ALWAYS follow with `mem_get_observation(id)` to get full content
- Working with truncated search results leads to wrong conclusions

### TDD Mode Detection
- The orchestrator checks `mem_search(query: "sdd-init/{project}")` for `strict_tdd: true`
- If found, all implement/validate agents receive the TDD directive
- If not found, agents use Standard Mode

## Persistence Modes

The orchestrator sets `artifact_store.mode` per session. Default: `cortex` when Cortex MCP is available, else `none`. Note: "engram" is a legacy alias for "cortex" mode. Both refer to the same MCP-based memory backend.

| Mode | Read from | Write to |
|------|-----------|----------|
| `cortex` | Cortex via `mem_search` → `mem_get_observation` | Cortex via `mem_save` |
| `openspec` | Filesystem: `openspec/changes/{change-name}/` | Filesystem |
| `hybrid` | Cortex first, filesystem fallback | Both |
| `none` | Orchestrator prompt context | Return inline only |

Only create `openspec/` directories when mode is explicitly `openspec` or `hybrid`.

## OpenSpec File Paths

When mode is `openspec` or `hybrid`, artifacts map to filesystem:

| Artifact | Path |
|----------|------|
| Proposal | `openspec/changes/{change-name}/proposal.md` |
| Specs | `openspec/changes/{change-name}/specs/{domain}/spec.md` |
| Design | `openspec/changes/{change-name}/design.md` |
| Tasks | `openspec/changes/{change-name}/tasks.md` |
| Verify Report | `openspec/changes/{change-name}/verify-report.md` |

## Topic Key Format

```
sdd/{change-name}/{artifact-type}
```

Exception: bootstrap uses `bootstrap/{project-name}`.

## Standard Artifact Types

| Phase | Agent | Artifact Type | Example Topic Key |
|-------|-------|---------------|-------------------|
| init | bootstrap | (project context) | `bootstrap/auth-service` |
| explore | investigate | `explore` | `sdd/add-auth/explore` |
| propose | draft-proposal | `proposal` | `sdd/add-auth/proposal` |
| spec | write-specs | `spec` | `sdd/add-auth/spec` |
| design | architect | `design` | `sdd/add-auth/design` |
| tasks | decompose | `tasks` | `sdd/add-auth/tasks` |
| apply | implement | `apply-progress` | `sdd/add-auth/apply-progress` |
| verify | validate | `verify-report` | `sdd/add-auth/verify-report` |
| archive | finalize | `archive-report` | `sdd/add-auth/archive-report` |
| archive | finalize | `retrospective` | `sdd/add-auth/retrospective` |

## Phase Read/Write Matrix

| Phase | Reads (required) | Reads (optional) | Writes |
|-------|------------------|------------------|--------|
| bootstrap | nothing | — | `bootstrap/{project}` |
| investigate | nothing | — | `sdd/{change}/explore` |
| draft-proposal | nothing | explore | `sdd/{change}/proposal` |
| write-specs | proposal | explore | `sdd/{change}/spec` |
| architect | proposal | explore | `sdd/{change}/design` |
| decompose | spec + design | proposal | `sdd/{change}/tasks` |
| implement | tasks + spec + design | task-scoped apply-progress | `sdd/{change}/apply-progress/{task-id}` |
| implement | tasks | spec + design + apply-progress | (via `tb_update` only) |
| validate | spec + tasks | apply-progress | `sdd/{change}/verify-report` |
| finalize | verify-report | all others | `sdd/{change}/archive-report` |

For phases with required dependencies, the sub-agent retrieves full content itself from Cortex using the two-step retrieval protocol. The orchestrator passes artifact references (topic keys), NOT the content.

## Apply-Progress Continuity

The apply phase may run in batches. Progress is tracked in `sdd/{change}/apply-progress`:

- **First batch**: sub-agent creates the artifact.
- **Subsequent batches**: sub-agent MUST read the existing apply-progress first, MERGE new progress with existing progress, then save the combined result. Do NOT overwrite — MERGE.

The orchestrator reads task-scoped apply evidence and authoritative ForgeSpec status before consolidating progress. Workers never overwrite peer evidence.

## Phase Name Aliasing

Each SDD phase has a canonical name (the skill directory name), an SDD-command alias, and a short alias. Always use the canonical name in code and configs.

| Canonical (skill dir) | SDD-Command | Short | Phase # |
|----------------------|-------------|-------|---------|
| bootstrap | sdd-init | init | 0 |
| investigate | sdd-explore | explore | 1 |
| draft-proposal | sdd-propose | propose | 2 |
| write-specs | sdd-spec | spec | 3 |
| architect | sdd-design | design | 4 |
| decompose | sdd-tasks | tasks | 5 |
| implement | sdd-apply | apply | 6 |
| validate | sdd-verify | verify | 7 |
| finalize | sdd-archive | archive | 8 |

Init (phase 0) is a prerequisite, not part of the main pipeline. The main pipeline is phases 1-8.

## mem_save Parameters

```
mem_save(
  title: "{topic-key}",
  topic_key: "{topic-key}",
  type: "architecture",
  scope: "project",
  project: "{project-name}",
  content: "{artifact markdown}"
)
```

- `topic_key` enables idempotent upsert: saving to the same key updates rather than duplicates.
- `type` is always `"architecture"` for SDD artifacts (except skill-registry which uses `"config"`).
- `scope` is always `"project"`.

## Two-Step Retrieval Protocol

`mem_search` returns 300-character previews only. Always follow this pattern:

```
1. mem_search(query: "{topic-key}", project: "{project}") → get observation ID
2. mem_get_observation(id: {id}) → retrieve full content
```

Skipping step 2 means working with truncated data.

## Knowledge Graph (mem_relate)

After saving artifacts, connect them for traceability:

```
mem_relate(from: {new_obs_id}, to: {upstream_obs_id}, relation: "references")
```

Supported relations:
- `references` — this artifact references another (most common in SDD)
- `relates_to` — general association
- `follows` — sequential dependency (e.g., spec follows proposal)
- `supersedes` — new version replaces old
- `contradicts` — conflicting information (flag for review)

## Delegation Boundary

All SDD agents work directly with their own tools. Only the `debate` and `parallel-dispatch` coordinator skills may delegate.

**If your SKILL.md does NOT contain a `<delegation>` section: you are a LEAF agent.**

Leaf agent rules:
1. Do all work directly using your own tools (read, write, edit, bash, grep, glob, MCP tools)
2. Return results to the caller — the orchestrator handles phase and ready-work routing
3. Each agent runs once per delegation

**Only these skills may delegate:**
- `debate` → launches `@investigate` defender agents
- `parallel-dispatch` → launches domain-specific agents

## Sub-Agent Context Protocol

SDD phase sub-agents run with a fresh context and NO memory. The orchestrator controls what each sub-agent can see:

| Aspect | Rule |
|--------|------|
| Read context | Orchestrator passes artifact references (topic keys or file paths), NOT content. Sub-agent retrieves content itself. |
| Write context | Sub-agent persists its artifact via `mem_save` BEFORE returning. Full detail belongs in Cortex, not in the return message. |
| Memory access | Sub-agent does NOT search Cortex for prior context on its own (unless explicitly instructed to read a specific artifact). |

This isolation makes phases composable and compaction-safe: each delegation is self-contained.

## Skill Loading Protocol (Canonical Version)

Every SDD agent MUST follow this exact protocol at startup. Do NOT deviate.

```
1. mem_search(query: "skill-registry", project: "{project}") → get observation ID
2. mem_get_observation(id: {id}) → read full skill registry
3. Fallback: read .sdd/skill-registry.md from the project root
4. If neither exists: proceed without skills (not an error — log a note recommending /bootstrap)
5. If a loaded skill has `requires` in frontmatter, load those dependency skills first
6. Load project context: mem_search(query: "bootstrap/{project}", project: "{project}")
   - If found: mem_get_observation(id) → store as project context (tech stack, conventions)
   - If not found: proceed without it — note the gap
```

`mem_search` returns 300-char previews. Call `mem_get_observation(id)` for full content. Working with previews leads to wrong conclusions.

## Exploration with mem_graph

To explore connections from any observation:

```
mem_graph(id: {obs_id}, depth: 2)
```

Useful for: recovering context after compaction, understanding artifact lineage, finding related work.

## Advanced Tools

For revision history, timeline context, project consolidation, hybrid search, session lifecycle, observation management, and temporal tools: read `cortex-advanced.md` in this directory.

## mem_save vs mem_update — When to Use Each

### mem_save (create or upsert)
Use `mem_save` with `topic_key` to create a new observation or update an existing one:
- Creating a new artifact: `mem_save(title: "sdd/{change}/spec", topic_key: "sdd/{change}/spec", ...)`
- Updating an evolving artifact: same call — `topic_key` triggers upsert (replaces content if key exists)
- Saving session state: `mem_save(title: "session/preferences", topic_key: "session/preferences", ...)`

### mem_update (modify by ID)
Use `mem_update` when you have the exact observation ID and want to modify specific content:
- Updating tasks.md with [x] marks: `mem_update(id: {tasks_id}, content: "{updated markdown}")`
- Correcting a typo in a saved observation: `mem_update(id: {obs_id}, content: "{fixed content}")`

### Rules
1. Prefer `mem_save` with `topic_key` for all SDD artifacts — it's idempotent and self-healing
2. Use `mem_update` only when you already hold the observation ID from a prior `mem_get_observation` call
3. Never call `mem_update` with a guessed ID — always retrieve it via `mem_search` first
4. After `mem_update`, the observation retains its original ID but content changes — downstream agents using `mem_search` will find the updated version

## Memory Quick Reference

| Operation | Tool | When |
|-----------|------|------|
| Save artifact | `mem_save(title, topic_key, type: "architecture", scope: "project", project, content)` | After completing phase work |
| Load artifact | `mem_search(query, project)` → ID, then `mem_get_observation(id)` → full content | Before starting phase work |
| Connect artifacts | `mem_relate(from, to, relation: "references")` | After saving new artifact |
| Update by ID | `mem_update(id, content)` | When you already hold the observation ID |
| Explore graph | `mem_graph(id, depth: 2)` | Recovering context or tracing lineage |

## A2A Task Delegation

For formal work requests with lifecycle tracking (alternative to msg_send for delegation):

| Tool | Purpose |
|------|---------|
| `a2a_submit_task(from_agent, to_agent, message)` | Submit work request |
| `a2a_get_task(task_id)` | Check status: submitted/working/completed/failed/canceled |
| `a2a_respond_task(task_id, message, status)` | Return structured result |
| `a2a_list_tasks(agent)` | Audit trail of delegations |
| `a2a_cancel_task(task_id)` | Cancel unresponsive task |

**When A2A vs msg_send**: Use `msg_send`/`msg_request` for quick clarifications. Use `a2a_submit_task` when you need status tracking, structured responses, or audit trail.

## Resource Coordination Protocol

| Mechanism | Source | Use For |
|-----------|--------|---------|
| `file_reserve` / `file_release` | ForgeSpec | File glob patterns during apply (use `check_only: true` to check without reserving) |
| `resource_acquire` / `resource_release` / `resource_check` | Agent Mailbox | Deploy, CI, APIs, DB, infrastructure |

**resource_acquire params**: resource_id (string key), agent, lease_type ("exclusive"/"shared"), ttl_seconds (default 300), metadata (optional context).

**Dead-Letter Queue**: `dlq_list()` to find failed deliveries, `dlq_retry(dlq_id)` to replay, `dlq_purge()` to clear. Check after compaction recovery and dependent timeouts.

## Leaf Agent Protocol

You are a leaf agent. The `task` tool is not available to you — do all work directly using your own tools (read, write, edit, bash, grep, glob, MCP tools). Return results to the caller. You cannot launch sub-agents or delegate work.

## Contract Persistence Protocol (ForgeSpec)

After generating your phase contract, self-validate and persist:
1. `sdd_validate(contract: {json_string})` — validate structure. The contract MUST have these exact top-level fields:
   ```json
   {
     "schema_version": "1.0",
     "phase": "{your-phase}",
     "change_name": "{change-name}",
     "project": "{project}",
     "status": "success|partial|failed|blocked",
     "confidence": 0.0-1.0,
     "executive_summary": "10+ chars describing outcome",
     "artifacts_saved": [{"topic_key": "...", "type": "cortex|openspec|inline"}],
     "next_recommended": ["next-phase"],
     "risks": [{"description": "...", "level": "low|medium|high|critical"}],
     "data": { ... phase-specific output ... }
   }
   ```
   Common validation errors: `status` must be one of the 4 values (NOT "complete"), `risks` items need `description` + `level` (NOT `risk` + `severity`), `confidence` must be 0-1 number.
2. `sdd_save(contract: {validated_json_string})` — persist to ForgeSpec store
If validation fails: read the error paths, fix the contract fields, and re-validate (max 2 retries before returning with status: "blocked").

## Status Contract Reference

Every phase returns a structured status contract that the orchestrator uses to decide next steps. See `sdd-status-contract.md` (in this directory) for the full field definitions and decision logic.

## Standard Pre-Return Checklist

Before returning results to the caller, verify:
1. All artifacts loaded via full Two-Step Retrieval (mem_get_observation), not 300-char previews
2. Contract JSON includes all required fields: status, executive_summary, artifacts, next_recommended, risks
3. Artifacts persisted with correct topic_key: `sdd/{change-name}/{artifact-type}`
4. `mem_relate` called connecting new artifact to upstream dependency

## Peer Communication Protocol

- `msg_request(to, subject, body, timeout)`: synchronous query (timeout 1-300s) — use for quick clarifications
- `msg_send(to, subject, body)`: async notification — use for status updates
- `msg_broadcast(sender, subject, body, priority)`: announce to all agents — use for completion/discovery
- Escalate scope changes or blockers to the orchestrator, not peers
