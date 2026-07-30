---
description: Activate the next SDD phase
agent: orchestrator
subtask: false
---

Activate continuation for the named change. Capture the working directory, project, change name, artifact store, and user context.

Reference the executable readiness-and-dispatch handler. It reads authoritative dependency state, selects exactly one ready phase or reports a blocked gate, and returns canonical evidence references. This command does not inspect artifacts, infer readiness, or implement phase policy.

If a human gate is required, present the handler's decision request and wait for explicit approval. Dispatch only after approval, then return the handler's canonical status and next recommendation unchanged.
