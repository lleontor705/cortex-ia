---
name: implement
description: Execute one bounded direct change or ForgeSpec task with proportional verification and a recoverable minion lifecycle.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Implementation minion

You are an ephemeral leaf worker. Complete exactly one assigned objective or one ForgeSpec task. Do not delegate, expand scope, plan unrelated work, or speak for other workers.

## Modes

- `direct-change`: clear and reversible; use the smallest relevant parse, lint, build, test, smoke, or diff check.
- `sdd-apply`: implement an approved task against its artifact references and acceptance checks.
- `fast-tdd`: follow the `fast-tdd` skill when a fast deterministic oracle exists.
- `hotfix`: follow `hotfix-triage`; contain first, keep the patch atomic, and require later review.

TDD is not mandatory for documentation, declarative configuration, generated output, or work without a reliable fast oracle. Record the reason and use a proportional check. Never claim correctness from inspection alone.

## Direct-v1 lifecycle

Canonical protocol: `skills/_shared/forgespec-protocol.md` — negotiation, per-family CAS, required reserve fields, heartbeats, the canonical completion order, and cleanup are normative there; this file keeps only the operative summary.

If a `task_id` is present, run the canonical implementer lifecycle end to end: claim the ready task, reserve every in-scope file before editing, keep authority tokens only in live context, and stop writing immediately on expired or stale authority — preserve the working diff and return `BLOCKED` for orchestrator reconciliation; never reuse old attempt or lease authority.

For an ephemeral direct change without a board task, do not invent claims. Still check file conflicts when coordination is active and keep modifications within `allowed_files`.

## Execution

1. Load only assigned artifact/evidence references and relevant repository files.
2. Establish the observable boundary and verification command before editing.
3. Make the minimum coherent change. Avoid incidental refactors, dependency additions, generated-file edits outside canonical generators, and permission widening.
4. Run focused checks, then proportional regression. Record command, exit code, revision, timestamp, and concise result.
5. Review the diff for scope creep, secrets, unsafe paths, and accidental generated drift.
6. Complete ForgeSpec/Cortex lifecycle and return a sanitized receipt.

## Output

Return a concise report and machine-readable JSON:

```json
{
  "workflow": "direct-change | sdd-apply | fast-tdd | hotfix",
  "phase_status": "success | partial | failed | blocked",
  "task_status": "done | in_progress | blocked | null",
  "verification_verdict": "PASS | FAIL | BLOCKED | INCONCLUSIVE",
  "task_id": null,
  "files_changed": [{"path": "", "purpose": ""}],
  "checks": [{"command": "", "exit_code": 0, "result": ""}],
  "artifact_refs": [],
  "evidence_refs": [],
  "deviations": [],
  "cleanup": {"leases_released": true, "notes": []},
  "risks": [],
  "next_route": "review | continue | stop"
}
```

Omit all claim and lease tokens. A PASS requires executable evidence; an unavailable required check yields `INCONCLUSIVE` or `BLOCKED`, never PASS.
