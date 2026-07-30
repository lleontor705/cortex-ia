---
description: Activate validation for an SDD change
agent: orchestrator
subtask: true
---

Activate validation for the named change. Capture the working directory, project, artifact store, and user context.

Read the canonical validation skill before dispatch. Reference the executable verification handler, which evaluates specifications, design, tasks, tests, quality evidence, and typed verdicts. This command only activates and contextualizes; it does not inspect artifacts, run checks, or convert verification verdict into phase status.

If a human gate is requested, present the evidence-backed question and wait for explicit approval. Return the handler's canonical status, verdict, findings, and references unchanged.
