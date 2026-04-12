---
description: Start a new SDD change — runs exploration then selects pipeline depth
agent: orchestrator
subtask: false
---

Follow the SDD orchestrator workflow for starting a new change named "$ARGUMENTS".

WORKFLOW:
1. Launch investigate sub-agent to investigate the codebase for this change
2. Present the exploration summary to the user

**Fast-track evaluation**: After receiving the investigation output, evaluate complexity to choose the right pipeline depth:

| Complexity | Criteria | Pipeline |
|-----------|----------|----------|
| **Trivial** | confidence >= 0.9, affected_files <= 2, single approach | implement → validate |
| **Simple** | confidence >= 0.8, affected_files <= 5, clear recommendation | propose → implement → validate |
| **Normal** | confidence >= 0.6, multiple approaches or domains | propose → spec → design → tasks → implement → validate |
| **Complex** | confidence < 0.6, high risk, migration required | Full pipeline with finalize |

Tell the user which track was selected and why. User can override with "use full pipeline".
If any phase fails or returns low confidence, escalate to the next deeper track.

**TRIVIAL fast-track** (confidence >= 0.9, affected_files <= 2):
- Skip ALL planning phases (propose, spec, design, tasks, decompose)
- Delegate directly to @implement with the investigation output as context:
  ```
  task(@implement, "
    Implement this change directly.
    Change: $ARGUMENTS | Project: {project}
    INVESTIGATION SUMMARY: {paste the full investigation output}
    AFFECTED FILES: {list from investigation}
    APPROACH: {recommended approach from investigation}
    artifact_store.mode: {mode}
  ")
  ```
- After @implement completes, delegate to @validate
- If @implement returns confidence < 0.8 → escalate to Simple track (run propose first, then re-implement)

**SIMPLE fast-track** (confidence >= 0.8, affected_files <= 5):
3. Launch draft-proposal sub-agent
4. Skip spec/design/tasks — delegate directly to @implement with the proposal as context
5. After @implement completes, delegate to @validate

**NORMAL / COMPLEX track**:
3. Launch draft-proposal sub-agent
4. Present the proposal summary and ask the user if they want to continue with /fast-forward

CONTEXT:
- Working directory: !`echo -n "$(pwd)"`
- Current project: !`echo -n "$(basename $(pwd))"`
- Change name: $ARGUMENTS
- Artifact store mode: {determined by orchestrator — default: cortex if Cortex MCP available, else none}

CORTEX NOTE:
Sub-agents handle persistence automatically. Each phase saves its artifact to Cortex with topic_key "sdd/$ARGUMENTS/{type}".

Read the orchestrator instructions to coordinate this workflow. Do NOT execute phase work inline — delegate to sub-agents.
