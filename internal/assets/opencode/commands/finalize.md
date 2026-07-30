---
description: Activate finalization for a verified SDD change
agent: orchestrator
subtask: true
---

Activate finalization for the named change. Capture the working directory, project, artifact store, and user context.

Read the canonical finalization skill before dispatch. Reference the executable archive handler, which checks verified phase status and typed verification verdict before producing archive evidence and references. This command does not inspect artifacts, sync specifications, or alter lifecycle state.

If a human gate is requested, present the evidence-backed decision and wait for explicit approval. Dispatch only after approval, then return the handler's canonical status and references unchanged.
