---
name: hotfix-triage
description: Contain an active incident, apply the smallest safe patch, prove regression protection, and create structural follow-up when needed.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Hotfix strategy

Optimize for safe service restoration, not architectural completeness. Do not refactor, optimize speculatively, add unrelated dependencies, widen permissions, or call `cortex_session_start`/`cortex_session_end` (session lifecycle is owned exclusively by the orchestrator). Define the containment boundary, rollback checkpoint, and stop conditions before editing. Cortex-IA work norms live in `skills/_shared/cortex-work-protocol.md`; this file adds only the containment delta.

## Procedure

1. **Preserve Symptom & Triage Prior Incidents:**
   - Search historical incidents: `cortex_search(query: "<symptom>", type: "bugfix")` and check `gotchas/<module>`.
   - Identify the smallest reproducible failing boundary. Distinguish immediate cause from deeper structural cause.
2. **Task & Lease Reservation:**
   - When a Cortex-IA work task exists, run the canonical implementer lifecycle (claim, task-bound file leases, heartbeat, review, mandatory cleanup). Keep authority tokens only in live context.
3. **Pre-Edit Blast Radius & Minimal Patch:**
   - Run `cortex_get_blast_radius` on target symbols to verify no unexpected secondary modules are impacted.
   - Apply the smallest coherent patch. Patch size is a risk signal, not an arbitrary correctness rule; stop and escalate when the change crosses unresolved architectural, data, or security boundaries.
4. **Focused Regression & Smoke Check:**
   - Add or run a targeted regression test, then run a focused smoke check. If no automated oracle exists, state the limitation and use the safest observable check; do not label it PASS beyond its evidence.
5. **Durable Incident & Gotcha Memory:**
   - Save a sanitized Cortex incident observation (`cortex_save` with `type: "bugfix"`, `topic_key: "hotfix/<incident_id>"` or `gotchas/<incident_id>`): symptom, root cause confidence, patch rationale, commands, outcomes, revision, and follow-up. Do not store tokens, secrets, or raw logs.
6. **Complete Lifecycle & Mandatory Review:**
   - Complete the canonical update/release order with sanitized evidence attached. Require independent `review` after containment.

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
