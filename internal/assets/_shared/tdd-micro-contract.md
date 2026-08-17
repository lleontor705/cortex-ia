# Shared TDD Micro Contract

Use for Fast-TDD and hotfix work when a bounded behavior has a usable oracle. File count alone neither selects nor rejects this route.

## Minion lifecycle

1. Negotiate ForgeSpec capabilities and claim exactly one task.
2. Retain revision, attempt, claim token, and expiry in live state only.
3. Reserve the exact file scopes and retain lease authority in live state only.
4. For Fast-TDD, demonstrate one focused RED before minimal GREEN, then refactor and run proportional regression.
5. Heartbeat/renew before expiry; stop writes immediately when claim or lease authority is lost.
6. Save summarized evidence in Cortex, update ForgeSpec with CAS/idempotency, and always release leases.

Hotfix may prioritize containment, but requires the smallest safe patch, a regression oracle when feasible, an independent review, and a follow-up task when only the symptom was contained. Do not fabricate RED for documentation, declarative configuration, generated code, or work without a deterministic oracle; route those through direct change or SDD with proportional verification.

## Evidence and status

Commands, exit codes, oracle results, and hashes are proof; narrative is not. Keep these dimensions separate:

- `phase_status`: `success | partial | failed | blocked`
- `task_status`: exact ForgeSpec task state
- `verification_verdict`: `PASS | FAIL | BLOCKED | INCONCLUSIVE`

## Receipt

```json
{
  "schema_version": "2.0",
  "workflow": "fast-tdd | hotfix | direct-change",
  "task_id": "string",
  "phase_status": "success | partial | failed | blocked",
  "task_status": "in_progress | done | blocked",
  "verification_verdict": "PASS | FAIL | BLOCKED | INCONCLUSIVE",
  "files_modified": [],
  "evidence": {
    "red": null,
    "green": null,
    "regression": null
  },
  "evidence_refs": [],
  "risks": [],
  "cleanup": { "leases_released": true }
}
```

Authority tokens must never appear in the receipt or Cortex.
