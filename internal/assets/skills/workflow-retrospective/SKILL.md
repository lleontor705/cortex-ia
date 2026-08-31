---
name: workflow-retrospective
description: Analyze a completed or repeatedly failing Cortex-IA workflow and recommend evidence-backed improvements without editing the environment. Use after revision loops, repeated blockers, or an explicit retrospective request.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Workflow retrospective

Run as a read-only `investigate` mode. Inspect authoritative work revisions, review findings, failure evidence, referenced artifacts, commands, and relevant agent instructions. Do not edit skills, code, configuration, task state, or installation. A retrospective recommends a separate bounded change; it never applies one automatically.

## Method

1. Define the reviewed task or session and its terminal or current state.
2. Build a revision timeline containing only state transitions, distinct failure causes, oracle results, and corrective attempts. Collapse repeated instances of the same cause.
3. Identify the earliest boundary that could have prevented each repeated failure.
4. Classify each candidate as `navigation`, `automated-check`, `review-rule`, `planning-contract`, `tool-economy`, `instruction-no-op`, or `information-access`.
5. Recommend the smallest change that would prevent recurrence. Assign it to `discovery`, `investigate`, `planner`, `orchestrator`, `implement`, `reviewer`, a shared contract, or product tooling.
6. Rank candidates by repeated cost, preventability, confidence, and implementation risk. Separate observed facts from inferred improvements.

Do not convert one unusual failure into a universal rule. Prefer an executable check over a prompt rule when both can prevent the same defect. Prefer reviewer guidance over implementer context when the requirement is evaluative rather than necessary to produce the change.

## Output

```json
{
  "workflow": "retrospective",
  "phase_status": "success | partial | blocked",
  "subject": "",
  "revision_count": 0,
  "distinct_failure_causes": [],
  "repeated_causes": [],
  "recommendations": [
    {
      "category": "",
      "owner": "",
      "evidence": [],
      "change": "",
      "priority": "high | medium | low"
    }
  ],
  "limitations": [],
  "next_route": "stop | direct-change | sdd-lite"
}
```
