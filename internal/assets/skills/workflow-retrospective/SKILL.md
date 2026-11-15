---
name: workflow-retrospective
description: Analyze a completed or repeatedly failing Cortex-IA workflow and recommend evidence-backed improvements without editing the environment. Use after revision loops, repeated blockers, or an explicit retrospective request.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Workflow Retrospective

Run this as a read-only `investigate` pass. Read the work revisions, review findings, failure evidence, referenced artifacts, commands, and the agent instructions involved. Change nothing: no skills, code, configuration, task state, or installation. The retrospective recommends a separate, bounded change, and someone else applies it.

## Method

1. Define the subject (the task or session) and its current or terminal state.
2. Build a revision timeline of state transitions, distinct failure causes, oracle results, and corrective attempts. Merge repeats of the same cause into one entry.
3. For each repeated failure, find the earliest boundary that could have prevented it.
4. Classify each candidate as `navigation`, `automated-check`, `review-rule`, `planning-contract`, `tool-economy`, `instruction-no-op`, or `information-access`.
5. Recommend the smallest change that would prevent recurrence, and assign it to one owner: `discovery`, `investigate`, `planner`, `orchestrator`, `implement`, `reviewer`, a shared contract, or product tooling.
6. Rank candidates by repeated cost, preventability, confidence, and risk. Keep observed facts separate from inferred improvements.

One unusual failure is not a universal rule. An executable check usually beats a prompt rule for the same defect. Put evaluative requirements in reviewer guidance, not in implementer context.

## Report

Send the §8 completion receipt, plus `subject`, `revision_count`, `distinct_failure_causes`, `repeated_causes`, a ranked `recommendations` list (each with category, owner, evidence, change, and priority), and `limitations`. Set `next_route` per `cortex-work-protocol.md` §8.
