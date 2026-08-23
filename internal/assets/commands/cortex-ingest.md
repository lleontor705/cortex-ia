---
description: Scan codebase with Cortex Native Polyglot AST Extractor and index symbols & relationships into knowledge graph
agent: orchestrator
subtask: false
---

Call the `cortex_ingest_code` tool on the current project repository (`.` or specified path) using the Zero-CGO Native Polyglot AST Extractor to index all code symbols (functions, structs, classes, interfaces) and dependencies into the Cortex knowledge graph. Report summary of files scanned, symbols indexed, and blast radius connections: $ARGUMENTS
