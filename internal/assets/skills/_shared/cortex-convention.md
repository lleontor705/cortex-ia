# Cortex Convention for SDD Agents

Shared protocol for all SDD agents. Reference this file instead of duplicating these patterns in individual skills.

## Persistence Modes

The orchestrator sets `artifact_store.mode` per session. Default: `cortex` when Cortex MCP is available, else `none`.

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

All SDD agents work directly with their own tools. Only three coordinator skills (team-lead, debate, parallel-dispatch) may delegate.

**If your SKILL.md does NOT contain a `<delegation>` section: you are a LEAF agent.**

Leaf agent rules:
1. Do all work directly using your own tools (read, write, edit, bash, grep, glob, MCP tools)
2. Return results to the caller — orchestrator or team-lead handles coordination
3. Each agent runs once per delegation

**Only these skills may delegate:**
- `team-lead` → launches `@implement` sub-agents
- `debate` → launches `@investigate` defender agents
- `parallel-dispatch` → launches domain-specific agents

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
- Saving session state: `mem_save(title: "session/cli-selection", topic_key: "session/cli-selection", ...)`

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
| `file_reserve` / `file_check` / `file_release` | ForgeSpec | File glob patterns during apply |
| `resource_acquire` / `resource_release` / `resource_check` / `resource_list` | Agent Mailbox | Deploy, CI, APIs, DB, infrastructure |

**resource_acquire params**: resource_id (string key), agent, lease_type ("exclusive"/"shared"), ttl_seconds (default 300), metadata (optional context).

**Dead-Letter Queue**: `dlq_list()` to find failed deliveries, `dlq_retry(dlq_id)` to replay, `dlq_purge()` to clear. Check after compaction recovery and dependent timeouts.

## Leaf Agent Protocol

You are a leaf agent. The `task` tool is not available to you — do all work directly using your own tools (read, write, edit, bash, grep, glob, MCP tools). Return results to the caller. You cannot launch sub-agents or delegate work.

## Contract Persistence Protocol (ForgeSpec)

After generating your phase contract:
1. `sdd_validate(phase: "{your-phase}", contract: {json})` — validate structure and required fields
2. `sdd_save(contract: {validated_json}, project: "{project}")` — persist to ForgeSpec store
If validation fails: fix errors in the contract and re-validate (max 2 retries before returning with status: "blocked").

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
