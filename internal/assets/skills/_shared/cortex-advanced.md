# Cortex Advanced Module

Load after `cortex-convention.md` when standard retrieval is insufficient.

## Authority and mandatory behavior

ForgeSpec owns SDD contracts, tasks, and readiness; Cortex owns evidence, memory, and lineage. Keep them separate: board/phase facts are ForgeSpec; observations/provenance are Cortex.

Save decisions, fixes, discoveries, config, and patterns immediately. Start with context/search; retrieve focused records and preserve provenance; end with a summary. Destructive administration requires authorization.

Use `cortex_revision_history` for revisions; `cortex_graph`, `cortex_graph_relationships`, and `cortex_graph_path` for lineage; and `cortex_score` to prioritize. Edges and scores are evidence, not authority. Use `cortex_search_hybrid` only after FTS5; `cortex_consolidate` finds repeats and `cortex_project_dna` summarizes decisions.
