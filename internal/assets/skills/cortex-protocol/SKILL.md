---
name: cortex-protocol
description: Cortex persistent-memory and code-graph protocol — Adaptive-RAG/HippoRAG retrieval, incremental AST ingestion, AST delta auditing, rules-vs-observations boundary, closed-loop failure memory, proactive save taxonomy, single stable session/board continuity, and CLI/watcher workflows. Load explicitly when applying the Cortex protocol to memory, AST, graph, or session/board operations.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
  opencode/autoinvoke: false
---

# Cortex Persistent Memory & Code Graph Protocol (v2.2.5)

Relocated from `internal/assets/AGENTS.md` §8 so the per-session harness injection stays lean. The section is predominantly reference material: the Cortex MCP server injects an equivalent per-session protocol block into every subagent prompt, and the per-session invariants remain normative in `cortex-work-protocol.md`. The skill registers `metadata.opencode/autoinvoke: false`: load it explicitly by ID when applying the Cortex protocol to memory, AST, graph, or session/board operations.

Cortex provides durable cognitive memory, AST structural knowledge graphs, and SOTA multi-hop retrieval. All agents MUST follow these mandatory operational rules:

## A. SOTA Adaptive-RAG & HippoRAG Retrieval

When searching memory or repository context:
1. `cortex_search(query, type, project, scope, limit, graph_expand)`:
   - Use `graph_expand: false` or omit it for direct memory search.
   - Use `graph_expand: true` to include graph-connected observations.
   - Never pass a `mode` argument; it is not part of the current tool schema.
2. `cortex_search_hybrid(query, limit, scope)`: Direct RRF dense+lexical fusion.
3. `cortex_graph(observation_id, depth)`: Traverse multi-hop associative chains.
4. `cortex_relate(from_id, to_id, relation_type)`: Connect related memories (`references`, `relates_to`, `follows`, `supersedes`, `contradicts`).
5. `cortex_score(observation_id)`: Inspect mathematical importance score ($S = I \cdot R(t) \cdot G$).

## B. Incremental Delta AST Ingestion & Watcher Synergy

1. **Absolute Workspace Root**: Always pass the absolute project directory path (e.g. `d:/cortex-ia` or `D:/ITC/APIs_Externos`) to `cortex_ingest_code(path, project)`. NEVER pass relative `.` because the Cortex MCP server runs in an isolated process directory.
2. **Startup Check**: `investigate` queries `cortex_get_code_symbols(project, limit: 1)`. If empty and `cortex watch` is not running, run `cortex_ingest_code(workspace_root_absolute_path, project)` once to establish the AST baseline.
3. **Review Delta Ingestion (<50ms)**: `reviewer` executes `cortex_ingest_code(workspace_root_absolute_path, project)` upon receiving edited files, utilizing SHA-256 incremental caching to re-index only the modified files without full repository scan penalty.
4. **Watcher Daemon**: When `cortex watch` is running in background, all file edits are indexed continuously in <500ms debounce.

## C. AST Delta Auditing (No Coupling Spikes)

1. **Baseline**: During `investigate` / `planner`, capture filtered symbol definitions, imports, source callers, and relevant test packages.
2. **Review Comparison**: `reviewer` compares the same bounded evidence after editing. `cortex_get_blast_radius` requires a numeric observation ID and must not be called with a code symbol or path.
3. **Cycle Regression**: `reviewer` MUST run `cortex_detect_cycles(project)` before emitting `PASS`.

## D. Automated Project Directives (`cortex_get_rules`) vs Memory Observations

1. **Directives vs Observations Boundary**: `cortex_save_rule` is STRICTLY reserved for permanent, persistent governance directives, coding standards, and architectural invariants (e.g. `rules/go-version`, `rules/auth-discipline`). NEVER use `cortex_save_rule` or prefix `rules/` for ephemeral task completions, git worktree creation, test outputs, or PR reviews.
2. **Orchestrator Injection**: `orchestrator` pulls `cortex_get_rules(project)` at session startup and injects genuine governance constraints into the `project_rules` array of minion dispatch envelopes.
3. **Minion Compliance**: `implement` minions must treat `project_rules` as hard invariants alongside acceptance tests.

## E. Closed-Loop Failure Memory & Knowledge Graph

1. **Failure Extraction**: When `reviewer` or tests detect a failure, `reviewer` persists the minimal failure locality in Cortex (`cortex_save` with `type: "bugfix"`, `topic_key: "gotchas/<task_id>"`).
2. **Graph Linking**: Always call `cortex_relate(from_id, to_id, relation_type)` to connect the bugfix/decision to the relevant entity, task, or previous observation.
3. **Targeted Fix Minion**: `orchestrator` includes `evidence_refs: ["gotchas/<task_id>"]` in the fix minion envelope so the next minion avoids repeating the same root cause.

## F. Proactive Save & Topic Taxonomy (MANDATORY)

Call `cortex_save` IMMEDIATELY after:
- Any architectural or design decision made (`type: decision`, `topic_key: architecture/<module>`).
- Any bug fixed (`type: bugfix`, `topic_key: bugfix/<issue>` — include root cause).
- Any gotcha or non-obvious learning (`type: discovery`, `topic_key: gotchas/<feature>`).
- Any pattern or convention established (`type: pattern`, `topic_key: patterns/<domain>`).
Never save ephemeral SQLite claim tokens, file lease states, diff hashes, or routine progress notes into Cortex.

## G. Single Stable Session & Board Continuity

1. **One Session per Initiative**:
   - The `orchestrator` owns the session lifecycle. It MUST maintain **EXACTLY ONE stable session ID and ONE stable board ID** throughout the entire initiative.
   - At startup, check if an active session already exists for the project via `cortex_context`. If active, bind to the existing `session_id`. DO NOT call `cortex_session_start` with new IDs mid-flow or across conversational turns in the same initiative.
   - **SUBAGENTS MUST NEVER CALL `cortex_session_start`, `cortex_session_summary`, OR `cortex_session_end`**.
2. **One Authoritative Board per Initiative**:
   - The board ID created by `planner`/`orchestrator` represents the initiative. Never spawn derivative successor boards (`-v2`, `-v3`, `-run2`). Blocked tasks must be decomposed in place with `cortex_ia_work_decompose`.
3. **Close (Orchestrator Only, MANDATORY before final turn)**: Call `cortex_session_summary` with:
   - `project`: `"<project_name>"` (e.g. `"ats-inventory"` or `"cortex-ia"`)
   - `content`: Single Markdown string containing all sections (`## Goal`, `## Discoveries`, `## Accomplished`, `## Next Steps`, `## Relevant Files`). Never pass `goal` or `discoveries` as separate top-level parameters.
4. **Compaction Recovery**: When context reset/compaction occurs:
   - Call `cortex_session_summary` with the compacted text immediately.
   - Call `cortex_context` to restore session continuity.
   - Call `cortex_search` for specific topics before resuming work.

## H. Cortex CLI & Continuous Watcher Workflows

Agents with terminal / bash capabilities can invoke the Cortex CLI for macro project operations:

```bash
# 1. Full AST Code Ingestion:
cortex ingest . --project=<project-name>
# Scans Go, TS, JS, Python, Rust, C++ using Zero-CGO 2-Pass Static Extractor

# 2. Continuous Live File Watcher Daemon:
cortex watch . --project=<project-name> --debounce=500ms
# Runs in background, automatically re-indexing modified files incrementally

# 3. Structural Code & Graph CLI Inspection:
cortex code graph --project=<project-name>
cortex code blast-radius <symbol-or-path> --project=<project-name>
cortex code cycles --project=<project-name>
cortex code architecture --project=<project-name>
cortex code search "<symbol-query>" --project=<project-name>

# 4. SOTA Multi-Mode Search:
cortex search "auth tokens" --mode=auto
cortex search "distributed consensus" --mode=multi_hop --limit=15

# 5. Diagnostics & Agent Setup:
cortex doctor
cortex setup opencode
```
