---
name: hotfix-triage
description: Contain an active incident, apply the smallest safe patch, prove regression protection, and create structural follow-up when needed. Use when a live incident needs containment before root-cause work.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Hotfix Triage

Restore service with the smallest change that is safe to ship. Do not refactor, optimise, add dependencies, or widen permissions while containing an incident. The work rules live in `cortex-work-protocol.md`; this file adds only the containment steps.

Before editing, name the containment boundary, the rollback checkpoint, and the conditions that would stop the work.

## Procedure

1. **Triage the symptom.**
   - Read prior incidents with `cortex_context` and `cortex_get_observation` for the `gotchas/<module>` and `hotfix/<incident_id>` topics.
   - Establish the tightest oracle for the reported symptom before editing (`diagnosis-loop-contract.md`). If containment cannot wait for a full diagnosis, label the causal verdict `INCONCLUSIVE` and keep the containment reversible.
   - Find the smallest failing boundary. A nearby failure does not stand in for the reported one.
2. **Reserve the work.** If a Cortex-IA task exists, follow the implementer lifecycle in `cortex-work-protocol.md` §4: claim, file leases, review hand-off, cleanup.
3. **Check blast radius, then patch.** Find callers with filtered `cortex_get_code_symbols`, `cortex_code_find` and source reads. Apply the smallest coherent patch. Stop and escalate when the fix crosses an unresolved architecture, data, or security boundary; patch size is a signal, not a rule.
4. **Prove it.** Add a targeted regression test where repository policy allows, then rerun the original oracle and a focused smoke check. With no automated oracle, state the limitation and claim no more than the evidence shows. Find the covering tests with `cortex_code_tests`.
5. **Record the incident.** `cortex_save` with `type: "bugfix"` and topic `hotfix/<incident_id>` (or `gotchas/<incident_id>`): symptom, root-cause confidence, patch rationale, commands, outcome, follow-up. Leave out tokens, secrets, and raw logs.
6. **Close.** Review follows the matrix in `cortex-work-protocol.md` §2.7a: an independent reviewer for high-risk triggers, otherwise orchestrator approval with an evidence pointer.

If the patch contains only the incident, hand the structural fix to a separate `plan` follow-up. If authority is lost, tests regress, or rollback is unsafe, stop and return `BLOCKED`. Do not improvise a destructive rollback.

## Report

Send the §8 completion receipt, plus `incident_id`, `containment`, `root_cause` with `root_cause_confidence`, `files_changed`, `cleanup`, and `risks`. Set `next_route` per `cortex-work-protocol.md` §8.
