# Cortex Convention

This is the single common authority for persistence, retrieval, lineage, and session guidance. Skills reference this document; they do not copy its rules. Load `cortex-advanced.md` only for low-frequency graph, revision, temporal, or recovery operations.

## Authority and storage

The selected artifact store is configured by the orchestrator. In Cortex mode, read with `mem_search` then `mem_get_observation`; write with `mem_save` using the stable topic key. In OpenSpec mode, read and write the change directory. Hybrid reads Cortex first and falls back to files. Never create OpenSpec directories unless that mode is selected.

ForgeSpec owns contracts, task state, dependencies, readiness, claims, revisions, and audit events. Cortex owns evidence, reflection, lineage, durable memory, sessions, and graph relationships. Evidence cannot override ForgeSpec readiness; ForgeSpec status cannot fabricate evidence.

## Retrieval and handoff

Search results are previews. Always retrieve the full observation before using it:

1. Search the exact topic key.
2. Retrieve the returned observation ID.
3. Use the complete content, not the preview.

Handoffs pass references (Cortex topic keys and ForgeSpec contract IDs), never copied transcripts. Connect new artifacts to upstream observations with `mem_relate` and use `references`, `follows`, or `supersedes` deliberately.

## Artifact keys

Use `sdd/{change}/{artifact}` for explore, proposal, spec, design, tasks, apply-progress, verify-report, and archive-report. Project initialization uses `bootstrap/{project}`. Apply workers report task progress through the task board; the team coordinator owns the merged apply-progress record.

## Sessions and recovery

Start a session with `mem_session_start`, record the user request when available, and close with `mem_session_summary` followed by `mem_session_end`. After restart or compaction, inspect task status, contract revisions, and apply progress; resume only incomplete work and never replay terminal tasks.

## Locks and boundaries

Use `file_reserve`/`file_release` for file conflicts. Use `resource_acquire`/`resource_release` only for external resources such as CI, APIs, or deployment targets. A leaf agent works directly, changes only assigned files, and reports blockers instead of inventing missing policy.

## Contract persistence

Apply contracts use phase `apply`, canonical change and project names, a terminal status, confidence, executive summary, saved artifacts, next recommendations, risks, and task-scoped data. Validate with `sdd_validate`, then persist with `sdd_save`. Do not claim success without command, exit-code, hash, or test evidence where the gate requires it.

## Knowledge graph

Use `mem_relate` to preserve artifact lineage. Use graph traversal and revision history only when the common path needs them; those procedures belong to `cortex-advanced.md`, not to skills or root modules.
