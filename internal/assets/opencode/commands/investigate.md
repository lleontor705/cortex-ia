---
description: Activate investigation for a supplied topic
agent: orchestrator
subtask: true
---

Activate investigation for the supplied topic and capture the working directory, project name, artifact store, and user context.

Read the canonical investigation skill, then dispatch the executable investigation handler. The handler examines the requested topic, affected areas, alternatives, risks, and recommendation, while preserving the canonical contract and evidence references. This command only activates and contextualizes; it does not inspect files or perform investigation itself.

If a human gate is requested, show the question and wait for an explicit decision. Return the handler's status, findings, artifacts, and next recommendation unchanged.
