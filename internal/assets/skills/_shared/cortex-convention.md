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

## Two-Step Read Pattern (CRITICAL)

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
