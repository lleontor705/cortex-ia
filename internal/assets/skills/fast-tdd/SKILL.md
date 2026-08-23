---
name: fast-tdd
description: Execute one bounded RED-GREEN-REFACTOR loop where a fast deterministic oracle proves observable behavior.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Fast-TDD minion strategy

Use this strategy only when behavior is localized, observable, reproducible, and covered by a fast deterministic test oracle. File count alone does not decide eligibility. If the oracle is slow, flaky, unavailable, or the change crosses unresolved boundaries, return `blocked` and recommend `direct-change`, `spike`, or right-sized SDD. You are a leaf minion: **NEVER call `cortex_session_start` or `cortex_session_end`** (session lifecycle is owned exclusively by the orchestrator).

## Lifecycle and loop

Canonical protocol: `skills/_shared/forgespec-protocol.md` — when a ForgeSpec task is assigned, run its canonical implementer lifecycle and completion order; authority tokens live only in live context, with writes stopped immediately on expired or stale authority. This file adds only the RED-GREEN-REFACTOR loop:

1. Write one focused test that captures the missing behavior. Run it and prove RED: failure must be caused by the intended missing behavior, not syntax, setup, or an unrelated failure.
2. Implement the minimum production change and rerun the identical focused command to prove GREEN.
3. Refactor only locally, then rerun the focused test and proportional regression suite.
4. Save a bounded Cortex observation containing commands, exit codes, revision, timestamp, oracle, and summarized outcomes; never save tokens or large stdout. Link it from the evidence-bearing `forgespec_task_transition`; cleanup follows the canonical completion order on every exit path.

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
