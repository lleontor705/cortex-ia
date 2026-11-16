---
name: fast-tdd
description: Execute one bounded RED-GREEN-REFACTOR loop where a fast deterministic oracle proves observable behavior. Use when a localized behavior has a fast, deterministic test oracle.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# Fast-TDD minion strategy

Use this strategy only when behavior is localized, observable, reproducible, and covered by a fast deterministic test oracle. File count alone does not decide eligibility. If the oracle is slow, flaky, unavailable, or the change crosses unresolved boundaries, return `blocked` and recommend `direct-change`, `spike`, or a `plan`. You never call session lifecycle tools; the orchestrator owns them.

## Lifecycle and loop

Canonical protocol: `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md` — when a persistent task is assigned, run its canonical claim/lease lifecycle; authority tokens live only in live context, with writes stopped immediately on expired or stale authority. This file adds only the RED-GREEN-REFACTOR loop:

Read `~/.cortex-ia/opencode/contracts/diagnosis-loop-contract.md` for bug fixes. Respect the repository's persistent-test policy; when that policy excludes the needed test class, use an ephemeral oracle and report the limitation instead of adding a prohibited test.

1. Name the public seam and independent source of the expected result. Write one focused test or approved ephemeral oracle that captures the exact missing behavior. Run it and prove RED: failure must be caused by the intended behavior, not syntax, setup, or an unrelated failure.
2. Implement the minimum production change and rerun the identical focused command to prove GREEN.
3. Run 1–2 minimal reversible mutations per the `mutation-testing` skill and record the outcome: `KILLED` proves the covering test can fail and asserts a boundary, a return payload, or a failure branch; `SURVIVED` marks a shallow test — strengthen the assertions and re-run until `KILLED`, never transitioning with `SURVIVED`; `STATIC-ANALYSIS` is permitted only when mutations cannot execute, naming the explicit assertions covering error boundaries, return payloads, and failure branches. Exempt work kinds (documentation, declarative configuration, generated output, no reliable fast oracle) record the exemption reason instead.
4. Refactor only locally, then rerun the focused test and proportional regression suite.
5. Rerun the original unminimized reproduction when the change fixes a defect. Verify tagged instrumentation and disposable harnesses are absent.
6. Save a bounded Cortex observation containing commands, exit codes, revision, timestamp, oracle, seam, and summarized outcomes; never save tokens or large stdout. Use its reference for review approval; cleanup follows the canonical completion order on every exit path.

## Output

Return the §8 receipt from `cortex-work-protocol.md` as Markdown `field: value` lines (not JSON), with `workflow: fast-tdd`. Fields: `status`, `summary`, `changed_files`, `checks`, `verdicts` (one per acceptance-check ID), `evidence_refs`, `blockers` when any, and `next_route`. Record the TDD evidence as `checks` entries:

- `<red command> -> exit <code>` (red: oracle, seam, expected_source)
- `<green command> -> exit <code>` (green)
- `<refactor command> -> exit <code>` (refactor)
- `<regression command> -> exit <code>` (regression)
- mutation: `KILLED` or `STATIC-ANALYSIS`, with the description, command, and whether it was reverted

For a fast-TDD-eligible code task with a test delta, the mutation entry is required: accepted outcomes are `KILLED` and `STATIC-ANALYSIS`, and `SURVIVED` never ships. A missing mutation entry makes the evidence incomplete (`INCONCLUSIVE` or `BLOCKED`), never `PASS`. Exempt work kinds record the exemption reason instead.

Never include claim or lease authority in the receipt.
