---
name: hotfix-triage
description: Contain an active incident, apply the smallest safe patch, prove regression protection, and create structural follow-up when needed.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Hotfix strategy

Optimize for safe service restoration, not architectural completeness. Do not refactor, optimize speculatively, add unrelated dependencies, or widen permissions. Define the containment boundary, rollback checkpoint, and stop conditions before editing. ForgeSpec norms live in `skills/_shared/forgespec-protocol.md`; this file adds only the containment delta.

## Procedure

1. Preserve the exact symptom and identify the smallest reproducible failing boundary. Distinguish immediate cause from deeper structural cause.
2. When a ForgeSpec task exists, run the canonical implementer lifecycle from `skills/_shared/forgespec-protocol.md` (claim, attempt-bound file leases, heartbeat, completion order, mandatory cleanup). Keep all authority tokens only in live context.
3. Apply the smallest coherent patch. Patch size is a risk signal, not an arbitrary correctness rule; stop and escalate when the change crosses unresolved architectural, data, or security boundaries.
4. Add or run a regression test when technically possible, then run a focused smoke check. If no automated oracle exists, state the limitation and use the safest observable check; do not label it PASS beyond its evidence.
5. Save a sanitized Cortex incident observation: symptom, root cause confidence, patch rationale, commands, outcomes, revision, and follow-up. Do not store tokens, secrets, or raw logs.
6. Complete the canonical update/release order with sanitized evidence attached. Require independent `review` after containment.

If the patch only contains the incident, return a separate `sdd-lite` or `sdd-full` follow-up. If authority is lost per the canonical rules, tests regress, or rollback cannot be made safe, stop and return `BLOCKED`; do not issue an improvised destructive rollback.

## Output

```json
{
  "workflow": "hotfix",
  "phase_status": "success | partial | failed | blocked",
  "task_status": "done | in_progress | blocked | null",
  "verification_verdict": "PASS | FAIL | BLOCKED | INCONCLUSIVE",
  "incident_id": "",
  "containment": "",
  "root_cause": "",
  "root_cause_confidence": "high | medium | low",
  "files_changed": [],
  "checks": [],
  "evidence_refs": [],
  "cleanup": {"leases_released": true, "notes": []},
  "risks": [],
  "next_route": "review | sdd-lite | sdd-full | stop"
}
```
