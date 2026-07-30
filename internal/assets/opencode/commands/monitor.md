---
agent: orchestrator
description: Activate monitoring for SDD state
subtask: true
---

Activate monitoring for the current project. Capture the working directory, project, artifact store, requested view, and user context.

Read the canonical monitoring skill, then dispatch the executable monitoring handler for state collection, evidence, and presentation. This command only activates and contextualizes; it does not read state, generate files, or duplicate dashboard policy.

If a human gate is requested, present the handler's evidence-backed question and wait for explicit approval. Return the canonical status and references unchanged.
