---
description: Discover or refresh the current project's skills, toolchains, Cortex rules, and architecture profile
agent: orchestrator
subtask: false
---

Dispatch the native `discovery` agent to inspect the current project and write or refresh `./.cortex-ia/discovery.md`. The discovery agent must remain read-only except for its typed report writer, must not install or execute builds, and must report missing engines and uncertain architecture as evidence-backed unknowns: $ARGUMENTS
