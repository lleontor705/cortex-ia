---
name: cortex-protocol
description: Cortex persistent-memory and code-graph protocol — Adaptive-RAG/HippoRAG retrieval, incremental AST ingestion, AST delta auditing, rules-vs-observations boundary, closed-loop failure memory, proactive save taxonomy, single stable session/board continuity, and CLI/watcher workflows. Load explicitly when applying the Cortex protocol to memory, AST, graph, or session/board operations.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
  opencode/autoinvoke: false
---

# Cortex memory protocol

Load this skill for the detail of saving to Cortex, searching it, and using the code graph. The Cortex server injects the per-session basics; this skill adds what you need on demand.

## Search

- `cortex_search(query, type, project, scope, limit, graph_expand)`. Omit `graph_expand` for direct search, or set it to `true` to include linked observations.
- `cortex_search_hybrid(query, limit, scope)` fuses lexical and dense results.
- `cortex_graph(observation_id, depth)` walks linked observations. `cortex_relate(from_id, to_id, relation_type)` links records that really relate.
- `cortex_get_rules(project)` returns governance rules. Rules are not memory.

## Save

Call `cortex_save` right after a decision, a bug fix, a gotcha, or a reusable pattern. Take `type` and `topic_key` from the taxonomy table in `cortex-convention.md`, which is authoritative. Gotchas are `type: bugfix` with `gotchas/<issue>`; bug fixes are `type: bugfix` with `bugfix/<issue>` and the root cause; decisions use `architecture/<module>`; patterns use `patterns/<domain>`.

`cortex_save_rule` is for permanent governance directives only, such as a coding standard or an architectural invariant. Never use it for task results or review notes.

Never save claim or lease tokens, lease state, diff hashes, progress notes, secrets, raw stdout, or unverified hypotheses.

## Code graph

- Ingest with the absolute workspace root, never `.`: `cortex_ingest_code(<absolute path>, project)`. Ingest only when the symbol index is empty and no watcher is running.
- `cortex_get_blast_radius` takes an observation ID, not a symbol or path. For code questions use `cortex_get_code_symbols`, `cortex_code_find`, or `cortex_detect_cycles`.
- A reviewer runs `cortex_detect_cycles` before a code task passes.

## Sessions

Only the orchestrator starts, summarises, and ends a session; subagents never call `cortex_session_*`. After context compaction, the orchestrator writes its summary with `cortex_session_summary`, restores context with `cortex_context`, and searches the topics it resumes.
