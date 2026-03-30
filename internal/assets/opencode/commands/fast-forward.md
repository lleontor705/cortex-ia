---
description: Fast-forward all SDD planning phases — proposal through tasks
agent: orchestrator
subtask: false
---

Follow the SDD orchestrator workflow to fast-forward all planning phases for change "$ARGUMENTS".

WORKFLOW:
Run sub-agents respecting the dependency graph — parallelize where possible:

1. draft-proposal — create the proposal
2. write-specs AND architect — launch BOTH simultaneously (both read the proposal output, neither depends on the other). Wait for both to complete before proceeding.
3. decompose — break down into implementation tasks (depends on specs + design). After decompose completes, use tb_create_board to initialize the task board from the decomposed tasks for parallel execution tracking.

Present a combined summary after ALL phases complete (not between each one).

PARALLEL EXECUTION NOTE:
Use the `task` tool to launch write-specs and architect concurrently after draft-proposal finishes. Do NOT wait for one before starting the other — they are independent consumers of the proposal artifact.

CONTEXT:
- Working directory: !`echo -n "$(pwd)"`
- Current project: !`echo -n "$(basename $(pwd))"`
- Change name: $ARGUMENTS
- Artifact store mode: {determined by orchestrator — default: cortex if Cortex MCP available, else none}

CORTEX NOTE:
Sub-agents handle persistence automatically. Each phase saves its artifact to Cortex with topic_key "sdd/$ARGUMENTS/{type}" where type is: proposal, spec, design, tasks.

Read the orchestrator instructions to coordinate this workflow. Do NOT execute phase work inline — delegate to sub-agents.
