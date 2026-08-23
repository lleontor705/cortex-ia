---
description: Start continuous live file watcher daemon to automatically re-index AST on code changes
agent: orchestrator
subtask: false
---

Launch the Cortex file watcher daemon in the background (`cortex watch . --project=<project-name>`) with debounced incremental static AST indexing. Files modified or created will automatically update `code_symbols` and `code_relations` without manual re-indexing: $ARGUMENTS
