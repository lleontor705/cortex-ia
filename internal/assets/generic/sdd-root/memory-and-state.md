# Memory and State

ForgeSpec owns contracts, revisions, task dependencies, readiness, claims, status, and audit events. Cortex owns evidence, reflection, lineage, durable memory, sessions, and graph relationships. Neither store may impersonate the other.

Handoffs carry `sdd/{change}/{artifact}` topic keys and ForgeSpec IDs. The receiver uses a reference lookup, retrieves the full observation, and never receives copied transcripts.

After compaction, inspect session context, task status, and artifact keys; traverse the graph only when needed. Resume the last incomplete task and preserve terminal evidence.

Start and end sessions explicitly. Save significant discoveries and connect them to upstream observations; session summaries are the recovery boundary.
