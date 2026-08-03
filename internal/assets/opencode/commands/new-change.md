---
description: Activate a new SDD change request
agent: orchestrator
subtask: false
---

Activate a new SDD change named by the user. Capture the working directory, project, change name, artifact store, and all supplied context.

Reference the executable change-dispatch handler. It evaluates investigation evidence, selects the dependency route, and returns the canonical next action. This command does not copy thresholds, choose phases, launch work, or write artifacts.

When the handler requests a human gate, present its question, explain the recorded evidence references, and wait for explicit approval or correction. Dispatch only after that decision. Return the canonical status, route, and references exactly as emitted.
