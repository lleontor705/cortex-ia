# Cortex Convention

This is the common authority for persistence, retrieval, lineage, and session guidance. Skills reference this document; they do not copy its rules. Load `cortex-advanced.md` only for low-frequency graph, revision, temporal, or recovery operations.

## Authority and storage

The artifact store is configured by the orchestrator. In Cortex mode, read with `mem_search_hybrid` (FTS5 + Vector RRF) or `mem_search`, then `mem_get_observation`; write with `mem_save` using the stable topic key and `mem_save_prompt` for prompt tracking. In OpenSpec mode, read and write the change directory. Hybrid reads Cortex first and falls back to files.

ForgeSpec owns contracts, task state, dependencies, readiness, claims, revisions, and audit events. Cortex owns evidence, reflection, lineage, durable memory, sessions, and graph relationships. Evidence cannot override ForgeSpec readiness; ForgeSpec status cannot fabricate evidence.

## Retrieval and handoff

Search results are previews. Always retrieve the full observation before using it:

1. Use `mem_search_hybrid` for semantic and keyword retrieval, or search exact topic key.
2. Retrieve returned observation ID with `mem_get_observation`.
3. Use complete content, not preview.

Handoffs pass references (Cortex topic keys and ForgeSpec contract IDs). Connect new artifacts to upstream observations with `mem_relate` and use `graph_neighbors` to traverse dependency edges.

## Session context and git synchronization

Start sessions with `mem_context` to absorb project history, then `mem_session_start`. Capture prompt intent with `mem_save_prompt`. Close with `mem_session_summary` then `mem_session_end`.

Use `cortex-ia memory sync --export` to export project observations to `.cortex/` for Git team sharing, and `cortex-ia memory sync --import` upon cloning.

## Artifact keys

Use `sdd/{change}/{artifact}` for explore, proposal, spec, design, tasks, apply-progress, verify-report, and archive-report. Project initialization uses `bootstrap/{project}`.

## Locks and boundaries

Use `file_reserve`/`file_release` for file conflicts. Use `resource_acquire`/`resource_release` for external targets. Leaf agents work directly and report blockers.

## Contract persistence

Apply contracts use phase `apply`, canonical change and project names, terminal status, confidence, executive summary, saved artifacts, recommendations, risks, and task data. Validate with `sdd_validate`, then persist with `sdd_save`. Success requires command, exit-code, hash, or test evidence.

## Knowledge graph and lineage

Use `mem_relate` and `graph_edge_add` to record architectural dependencies. Use `graph_neighbors` (BFS depth up to 10) to inspect component relationships before design decisions.
