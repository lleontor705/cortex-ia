---
agent: orchestrator
description: Activate debate for an SDD topic
subtask: false
---

Activate debate for the supplied topic. Capture the working directory, project, artifact store, requested positions, and all user context.

Reference the executable debate-dispatch handler. It selects the declared evidence-backed method, records independent findings, and returns a canonical decision package. This command does not define debate rounds, agent counts, task-board operations, tie-breakers, or persistence policy.

If a human gate is requested, present the handler's evidence and decision question, then wait for explicit approval or correction. Dispatch only after approval and return the canonical status and references unchanged.
