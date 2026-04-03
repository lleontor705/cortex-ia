# Cortex Advanced Tools Reference

Low-frequency tools and protocols. Load only when needed by specific phases (investigate, validate, finalize, bootstrap).

## Revision History (mem_revision_history)

Track how an observation evolved across `topic_key` upserts:

```
mem_revision_history(observation_id: {id}, limit: 20)
```

Returns structured snapshots showing each revision with timestamps. Use when:
- A spec or design changed mid-pipeline and you need to see what changed
- Investigating why a previous phase produced different output
- Auditing artifact evolution during finalize/retrospective

## Timeline Context (mem_timeline)

See chronological context around a specific observation:

```
mem_timeline(observation_id: {id}, before: 5, after: 5)
```

Returns observations created before and after the target. Useful for understanding what was happening in the session when an artifact was created.

## Project Consolidation (mem_merge_projects)

If project names fragmented (common with git remote variations):

```
mem_merge_projects(from: "my_project,myproject", to: "my-project")
```

Merges all observations from variant names into the canonical name. Run during bootstrap or when `mem_search` returns fewer results than expected.

## Hybrid Search (mem_search_hybrid)

When `mem_search` (FTS5) returns insufficient results:

```
mem_search_hybrid(query: "{topic}", project: "{project}", limit: 10)
```

Combines FTS5 full-text search with vector similarity (when available) via Reciprocal Rank Fusion. Falls back to FTS5-only if vectors are disabled.

## Session Lifecycle

```
mem_session_start(id: "{session-id}", project: "{project}", directory: "{cwd}")
mem_save_prompt(content: "{user_request}", project: "{project}")
... work ...
mem_session_summary(content: "{summary}", project: "{project}")
mem_session_end(id: "{session-id}", summary: "{brief}")
```

## Observation Management

- `mem_archive(observation_id)` — soft-delete (still findable with include_archived flag)
- `mem_delete(id, hard_delete: false)` — soft-delete by ID
- `mem_delete(id, hard_delete: true)` — permanent deletion (admin only)
- `mem_suggest_topic_key(type, title)` — get recommended topic_key for new observations

## Temporal Tools (Experimental — Cortex v0.2.1+)

Available when Cortex is configured with temporal/metrics repositories:

| Tool | Purpose |
|------|---------|
| `temporal_create_edge` | Create edge with temporal validity and evolution tracking |
| `temporal_get_edges` | Retrieve edges valid at specific time point |
| `temporal_get_relevant` | Get observations relevant at specific time |
| `temporal_create_snapshot` | Point-in-time snapshot of knowledge graph |
| `temporal_record_operation` | Record operation with performance metrics |
| `temporal_evaluate_quality` | Evaluate memory system quality (relevance, completeness, consistency) |
| `temporal_system_metrics` | System-wide performance metrics |
| `temporal_health_check` | System health status |
| `temporal_evolution_path` | Edge evolution history |
| `temporal_fact_state` | Current state of facts for observation |

Use temporal tools for: long-running changes spanning multiple sessions, auditing knowledge graph evolution, system observability.
