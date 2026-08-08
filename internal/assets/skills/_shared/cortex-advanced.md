# Cortex Advanced Module

Load this when retrieval is insufficient. Authority, trust, transport IDs, and sessions remain governed by `cortex-convention.md`.

## Evolution and graph

- `cortex_revision_history` inspects topic-key revisions.
- `cortex_graph` traverses bounded related observations.
- `cortex_graph_relationships` lists typed edges.
- `cortex_graph_path` finds a bounded observation path.
- `cortex_score` helps prioritize context.

Retrieve the focused observation first. Relationships and scores are evidence, not authority.

## Search and synthesis

Use `cortex_search_hybrid` only when FTS5 is insufficient. `cortex_consolidate` identifies repeated topic-key memories; `cortex_project_dna` summarizes project decisions and gotchas. Neither tool merges, deletes, or promotes facts automatically. Preserve provenance and require authorization for destructive admin operations.
