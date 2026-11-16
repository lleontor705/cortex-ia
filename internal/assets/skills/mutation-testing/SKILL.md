---
name: mutation-testing
description: Inject syntactic and semantic faults into delivered code to verify whether test suites are genuine verifiers or shallow 'vibe tests'. Use when judging whether tests would catch real behavior changes.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.1.0"
---

# Mutation testing

Use this skill to check that a test suite fails when the behaviour it covers changes. Evidence rules (what a receipt must contain, and when a transition is allowed) are in `_shared/cortex-work-protocol.md` §4 and §8.

## Who runs what

- **Implementer**: applies one or two small, reversible mutations to the code under test, runs the covering test, and reverts. This happens before the task moves to `in_review`.
- **Reviewer**: read-only. It checks the claimed result against the diff and the test source. It does not edit files or run mutations. It may read output from a mutation tool such as `go-mutesting` if one already ran.

## Operators

Pick one or two that fit the change:

1. Boundary and operator: `>` to `>=` or `<`; `==` to `!=`; `+` to `-`.
2. Boolean: invert a returned boolean; negate an `if` predicate.
3. Return value: replace a returned payload with `nil`, `false`, or an empty value.

## Procedure

1. Run the covering test on the unmodified code. It must pass.
2. Apply one mutation. If the code no longer compiles, discard that mutation and choose another. A compile error does not count as a kill.
3. Rerun the covering test.
   - Test fails on an assertion: the mutation is **KILLED**. The test is doing its job.
   - Test still passes: the mutation **SURVIVED**. Strengthen the test and repeat.
4. Revert by applying the inverse edit with the edit tool, then run `git diff` and confirm that only the intended change remains. Never revert with `git checkout -- <file>`: it discards the implementer's uncommitted fix along with the mutation.

If execution is impossible in the environment, record **STATIC-ANALYSIS** instead. Then name the assertions that cover the changed unit's error boundaries, return payloads, and failure branches. A bare claim does not meet that bar.

## Reviewer check

For each claimed result:

- KILLED: confirm the named mutation would flip an assertion in the delivered test. A contradiction, or a claim the test cannot falsify, fails the task.
- STATIC-ANALYSIS: confirm the cited assertions cover the boundaries described above. Accept the evidence if they do. Do not fail the task only because no mutation was executed.
- Any test that would pass under any behaviour change of the logic it covers is an oracle gap, and it fails a code task.
