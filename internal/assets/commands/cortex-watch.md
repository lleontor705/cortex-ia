---
description: Check the Cortex code index and explain how to run the live file watcher
agent: orchestrator
subagent: false
---

The file watcher is a long-running process, so you start it yourself in a terminal you keep open, from the repository root:

```
cortex watch . --project=<project-name> --debounce=500ms
```

While it runs, edits re-index `code_symbols` and `code_relations` automatically.

Then dispatch the native `investigate` subagent to check whether the index is current with `cortex_get_code_symbols(project, limit: 1)`. It reports whether symbols are indexed. If the index is empty, it recommends `/cortex-ingest`. Project name, if given: $ARGUMENTS
