---
description: Query the Cortex code graph for call hierarchy, blast radius, cycles, and architecture metrics
agent: orchestrator
subagent: false
---

Dispatch the native `investigate` subagent to run the Cortex code analysis the request needs: `cortex code graph`, `cortex code blast-radius <target>`, `cortex code cycles`, `cortex code architecture`, or `cortex code search <query>`. Have it report the call hierarchy and the impact. Request: $ARGUMENTS
