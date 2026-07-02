# SDD Phase Common Patterns

Shared conventions for every SDD phase skill. Reference this file instead of duplicating these patterns in individual phase skills (`investigate`, `draft-proposal`, `write-specs`, `architect`, `decompose`, `team-lead`, `implement`, `validate`, `finalize`, `bootstrap`, `onboard`).

## Sub-Agent Context Protocol

SDD phase sub-agents run with a fresh context and NO memory. The orchestrator controls what each sub-agent can see:

| Aspect | Rule |
|--------|------|
| Read context | Orchestrator passes artifact references (topic keys or file paths), NOT content. Sub-agent retrieves content itself. |
| Write context | Sub-agent persists its artifact via `mem_save` BEFORE returning. Full detail belongs in Cortex, not in the return message. |
| Memory access | Sub-agent does NOT search Cortex for prior context on its own (unless explicitly instructed to read a specific artifact). |

This isolation makes phases composable and compaction-safe: each delegation is self-contained.

## Artifact Read Pattern

Every phase reads its upstream dependencies via the two-step retrieval protocol. Never work with the 300-character preview returned by `mem_search` alone.

```
1. mem_search(query: "{topic-key}", project: "{project}")  → get observation ID
2. mem_get_observation(id: {id})                            → retrieve full content
```

Skipping step 2 means working with truncated data, which leads to wrong conclusions and missed context.

## Artifact Write Pattern

Each phase writes its output via `mem_save` with a stable `topic_key`. The `topic_key` enables idempotent upsert: saving to the same key updates rather than duplicates.

```
mem_save(
  title:     "{topic-key}",
  topic_key: "{topic-key}",
  type:      "architecture",
  scope:     "project",
  project:   "{project}",
  content:   "{artifact markdown}"
)
```

- `type` is always `"architecture"` for SDD artifacts (exception: skill-registry uses `"config"`).
- `scope` is always `"project"`.

## Topic Key Format

```
sdd/{change-name}/{artifact-type}
```

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

Exception: bootstrap uses `bootstrap/{project-name}`.

## Phase Read/Write Matrix

| Phase | Reads (required) | Reads (optional) | Writes |
|-------|------------------|------------------|--------|
| bootstrap | nothing | — | `bootstrap/{project}` |
| investigate | nothing | — | `sdd/{change}/explore` |
| draft-proposal | nothing | explore | `sdd/{change}/proposal` |
| write-specs | proposal | explore | `sdd/{change}/spec` |
| architect | proposal | explore | `sdd/{change}/design` |
| decompose | spec + design | proposal | `sdd/{change}/tasks` |
| team-lead | tasks + spec + design | apply-progress | `sdd/{change}/apply-progress` |
| implement | tasks | spec + design + apply-progress | (via `tb_update` only) |
| validate | spec + tasks | apply-progress | `sdd/{change}/verify-report` |
| finalize | verify-report | all others | `sdd/{change}/archive-report` |

For phases with required dependencies, the sub-agent retrieves full content itself from Cortex using the two-step retrieval pattern above. The orchestrator passes artifact references (topic keys), NOT the content.

## Apply-Progress Continuity

The apply phase may run in batches. Progress is tracked in `sdd/{change}/apply-progress`:

- **First batch**: sub-agent creates the artifact.
- **Subsequent batches**: sub-agent MUST read the existing apply-progress first, MERGE new progress with existing progress, then save the combined result. Do NOT overwrite — MERGE.

The orchestrator tells the implement/team-lead sub-agent when previous apply-progress exists so it can perform the read-merge-write cycle.

## OpenSpec Mode

When `artifact_store.mode` is `openspec` or `hybrid`, artifacts also exist on the filesystem:

| Artifact | Filesystem Path |
|----------|-----------------|
| Proposal | `openspec/changes/{change}/proposal.md` |
| Specs | `openspec/changes/{change}/specs/{domain}/spec.md` |
| Design | `openspec/changes/{change}/design.md` |
| Tasks | `openspec/changes/{change}/tasks.md` |
| Verify Report | `openspec/changes/{change}/verify-report.md` |

For `hybrid` mode: read from Cortex first, fall back to filesystem. Write to both.

## Knowledge Graph Connection

After saving an artifact, connect it to its upstream dependency for traceability:

```
mem_relate(from: {new_obs_id}, to: {upstream_obs_id}, relation: "references")
```

Supported relations: `references`, `relates_to`, `follows`, `supersedes`, `contradicts`.

Without these edges, artifacts are isolated islands that cannot be navigated via `mem_graph`.

## Status Contract

Every phase returns a structured status contract that the orchestrator uses to decide next steps. See `sdd-status-contract.md` (in this directory) for the full field definitions and decision logic.
