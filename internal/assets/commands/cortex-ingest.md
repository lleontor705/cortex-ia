---
description: Scan codebase with Cortex Native Polyglot AST Extractor and index symbols & relationships into knowledge graph
agent: orchestrator
subagent: false
---

Dispatch the native `investigate` subagent to call the `cortex_ingest_code` tool on the current project repository (specifying its absolute workspace root path) using the Zero-CGO Native Polyglot AST Extractor to index all code symbols (functions, structs, classes, interfaces) and dependencies into the Cortex knowledge graph. Have it report the summary of files scanned, symbols indexed, and blast radius connections: $ARGUMENTS
