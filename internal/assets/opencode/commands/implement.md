---
description: Activate implementation for ready SDD work
agent: orchestrator
subtask: true
---

Activate implementation for the named change. Capture the working directory, project, task or change identifier, artifact store, and user context.

Reference the executable apply-dispatch handler. It selects ready work, applies required gates, and returns canonical task, contract, evidence, and handoff references. This command does not claim readiness, duplicate retry policy, or implement tasks.

If a human gate is requested, present the evidence-backed decision and wait for explicit approval. Dispatch only after approval. Return the handler's canonical status and next recommendation unchanged.
