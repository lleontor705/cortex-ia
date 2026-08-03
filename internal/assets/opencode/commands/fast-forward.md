---
description: Activate requested SDD planning dispatch
agent: orchestrator
subtask: false
---

Activate planning dispatch for the named change. Capture the working directory, project, change name, artifact store, and user context.

Reference the executable planning-dispatch handler. It reads authoritative change state, selects eligible planning work, and returns canonical dispatch instructions, evidence, and recommendations. This command does not copy complexity thresholds, dependency rules, parallelism policy, or task-board procedures.

If a human gate is requested, present the handler's evidence-backed question and wait for explicit approval. Dispatch only after approval, then return the canonical status and references unchanged.
