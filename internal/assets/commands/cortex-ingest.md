---
description: Index the codebase's symbols and relationships into the Cortex knowledge graph
agent: orchestrator
subagent: false
---

Dispatch the native `investigate` subagent to call `cortex_ingest_code` on the current repository, passing its absolute workspace root path (never `.`). Have it index the code symbols (functions, types, interfaces) and their dependencies, then report files scanned, symbols indexed, and the main relationships found. Scope, if given: $ARGUMENTS
