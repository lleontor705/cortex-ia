---
name: code-review-adversary
description: Independently audit a change for correctness, security, regression, concurrency, performance, and contract compliance without editing. Use when an independent reviewer must verify a completed change.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Independent review lenses

The `reviewer` agent owns the verdict flow (`agents/reviewer.md`). This skill is the checklist for the three lenses. The case matrix (`_shared/cortex-work-protocol.md` §2.7a) decides which lenses apply, and the LOC budget and receipt are in §4 and §8.

Judge the actual diff against the task objective and its acceptance checks. Treat the implementer's receipt as a claim to verify, not as proof.

## Lens 1: Functional and structural

- Rerun the acceptance checks the task names (its `AC-<n>` IDs). Record the command and exit code. Do not rerun the whole plan's checks.
- Run the tests for the changed packages and their direct callers.
- For code tasks, run `cortex_ingest_code` on the absolute workspace root, then `cortex_detect_cycles` for the project. A new cycle is a BLOCKER.
- Check that tests assert what the objective needs: error branches, return payloads, boundaries. A test that would pass under any change to the logic is an oracle gap. Name the assertion that cannot fail.

## Lens 2: Resilience and security

- No secret, credential, or authority token (`claim_token`, `lease_token`) appears in the diff, fixtures, logs, or receipts.
- Locks, goroutines, file handles, and connections are released on every path.
- Errors are not silently swallowed, and no "skip" path is added to make tests pass. Contracts stay as they are.
- Check the code-intelligence and privacy rules in `_shared/cortex-work-protocol.md` §4 only when the diff touches them.

## Lens 3: Architecture and discovery

- The diff respects the boundaries in `./.cortex-ia/discovery.md` (when present) and the design contract in `_shared/codebase-design-contract.md`.
- Interfaces stay narrow, and dependency direction holds.
- Size is checked against the active `workload_policy` (§4). Under `flexible` or `unbounded`, a larger coherent diff is not a finding by itself.

## Severity

- **BLOCKER**: a secret or destructive data risk, an unmet acceptance criterion, a new dependency cycle, or a reproducible regression in a real caller.
- **WARNING**: a real but bounded risk the task can accept.
- **NIT**: style or preference. Hypothetical inputs to test helpers, mocks, or branches no caller reaches are NIT at most.

Each finding names the lens, file and line, the evidence, the impact, and a remediation.

## Non-code tasks

For generated artifacts, documentation, and declarative configuration, check artifact presence, non-zero size, format validity, and secret hygiene. Skip AST, cycle, and linter checks.

## Verdict per lens

Give each lens PASS, FAIL, BLOCKED, or INCONCLUSIVE with its evidence. A tool that is unavailable makes that check INCONCLUSIVE, which is neither a defect nor a PASS.

## Memory

On FAIL, the reviewer saves the root cause as a gotcha and cites its observation id in the findings (see `agents/reviewer.md`). On PASS for an architectural change, record the decision with `cortex_save` (`type: "decision"`, `topic_key: "architecture/<module>"`). Topic keys and observation types are in `_shared/cortex-convention.md`. Never store secrets, raw output, or authority tokens.
