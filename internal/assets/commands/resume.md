---
description: Recover authoritative state and continue eligible work
agent: orchestrator
subtask: false
---

Resume from `cortex-ia work` state, OpenSpec artifacts, and referenced Cortex evidence. When native child state was lost to restart or compaction, run bounded `background_recover`, then `cortex-ia work recover`; recovered sessions never restore claim or lease tokens. Dispatch only work that is currently ready: $ARGUMENTS
