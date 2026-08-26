# Cortex Advanced Module

Load after `cortex-convention.md` when standard retrieval is insufficient.

## Authority and mandatory behavior

OpenSpec owns SDD contracts; `cortex-ia work` owns tasks and readiness; Cortex MCP owns evidence, memory, and lineage. Keep them separate: task state comes from the CLI, contract facts from files, and observations/provenance from Cortex MCP.

Save decisions, fixes, discoveries, config, and patterns immediately. Start with context/search; retrieve focused records and preserve provenance; end with a summary. Destructive administration requires authorization.

Use `cortex_revision_history` for revisions; `cortex_graph`, `cortex_graph_relationships`, and `cortex_graph_path` for lineage; and `cortex_score` to prioritize. Edges and scores are evidence, not authority. Use `cortex_search_hybrid` only after FTS5; `cortex_consolidate` finds repeats and `cortex_project_dna` summarizes decisions.
