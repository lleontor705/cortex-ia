---
name: fast-tdd
description: Execute one bounded RED-GREEN-REFACTOR loop where a fast deterministic oracle proves observable behavior.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Fast-TDD minion strategy

Use this strategy only when behavior is localized, observable, reproducible, and covered by a fast deterministic test oracle. File count alone does not decide eligibility. If the oracle is slow, flaky, unavailable, or the change crosses unresolved boundaries, return `blocked` and recommend `direct-change`, `spike`, or right-sized SDD.

## Lifecycle and loop

1. If assigned a ForgeSpec task, negotiate compatible `direct-v1`, query its current revision, claim it with `tb_claim`, and retain the returned attempt authority only in live context.
2. Reserve every edit scope with `file_reserve` bound to that task and attempt. Do not edit on conflict.
3. Write one focused test that captures the missing behavior. Run it and prove RED: failure must be caused by the intended missing behavior, not syntax, setup, or an unrelated failure.
4. Implement the minimum production change and rerun the identical focused command to prove GREEN.
5. Refactor only locally, then rerun the focused test and proportional regression suite.
6. Renew the task attempt and file leases before expiry. If authority expires or a CAS/lease conflict occurs, stop writing.
7. Save a bounded Cortex observation containing commands, exit codes, revision, timestamp, oracle, and summarized outcomes. Never save tokens or large stdout.
8. Update ForgeSpec with latest task revision, attempt authority, and evidence links. Mark done only after GREEN and regression pass. Release all leases on every exit path.

## Output

```json
{
  "workflow": "fast-tdd",
  "phase_status": "success | partial | failed | blocked",
  "task_status": "done | in_progress | blocked | null",
  "verification_verdict": "PASS | FAIL | BLOCKED | INCONCLUSIVE",
  "task_id": null,
  "evidence": {
    "red": {"command": "", "exit_code": 1, "oracle": ""},
    "green": {"command": "", "exit_code": 0},
    "refactor": {"command": "", "exit_code": 0},
    "regression": {"command": "", "exit_code": 0}
  },
  "files_changed": [],
  "evidence_refs": [],
  "cleanup": {"leases_released": true, "notes": []},
  "risks": [],
  "next_route": "review | continue | sdd-lite | stop"
}
```

Never include claim or lease authority in the receipt.
