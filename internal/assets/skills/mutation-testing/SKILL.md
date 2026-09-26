---
name: mutation-testing
description: Inject syntactic and semantic faults into delivered code to verify whether test suites are genuine verifiers or shallow 'vibe tests'.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.1.0"
---

# Mutation Testing & Adversarial Verification

Use this skill to validate the effectiveness of tests submitted by implementation minions and eliminate shallow 'vibe tests'.

## 1. Role Boundaries & Execution Context
- **Invocation Contract**: Mutation execution is **implementer-side only**, performed in `current_workspace` under a live claim and per-file lease BEFORE `transition in_review`. The reviewer never executes mutations.
- **Implementer Role (`implement`)**: Applies 1–2 minimal reversible mutations during TDD and verification to prove test sensitivity, then reverts to clean git state. The normative obligation and exemptions live in `_shared/cortex-work-protocol.md` §4 (lifecycle) and §8 (evidence composition).
- **Reviewer Role (`reviewer`)**: Possesses strictly read-only authority (`edit: false`, `write: false`). **DO NOT** attempt to edit files via bash (`sed`, `cat`) or clone repositories into temporary directories. Verify test resistance read-only via:
  1. **Static Assertion Analysis**: Inspect test source to confirm explicit assertion of error boundaries, return payloads, and failure branches.
  2. **Independent Evidence Verification**: Confirm the receipt's named mutation would flip an actual assertion in the delivered diff and test source; a contradiction or unfalsifiable claim is a FAIL.
  3. **Dedicated AST Mutation Tooling**: Use specialized tooling (e.g. `go-mutesting`) if installed, for reading results only — never to mutate source yourself.

## 2. Mutation Evidence Receipt Field

The `mutation` receipt field records the implementer's evidence outcome; the normative gate is stated once in `_shared/cortex-work-protocol.md` §4/§8.

| Value | Meaning | Consequence |
|---|---|---|
| `KILLED` | covering test failed under the mutation | genuine verifier; transition allowed |
| `SURVIVED` | test passed despite broken logic | shallow test; strengthen and re-run; never ships in a transition |
| `STATIC-ANALYSIS` | environment/tool cannot compile or execute mutations | minimum bar: explicit recorded assertion of the changed unit's error boundaries, return payloads, and failure branches; required, never silently skipped |

`SKIPPED_UNSUPPORTED` is not a valid outcome: when execution is impossible, record `STATIC-ANALYSIS` with the three-part assertion bar instead.

## 3. Mutation Operators to Apply (Implementer / AST Tools)
Apply 1 or 2 minimal, reversible mutations to the target implementation:
1. **Boundary / Operator Mutator:**
   - Change `>` to `>=` or `<`.
   - Change `===` to `!==` or `==` to `!=`.
   - Change `+` to `-`.
2. **Boolean / Inversion Mutator:**
   - Invert boolean return value (`return true` -> `return false`).
   - Negate conditional predicate (`if (isValid)` -> `if (!isValid)`).
3. **Void / Return Statement Mutator:**
   - Replace return payload with `nil`, `false`, or empty struct.

## 4. Strict Verification Protocol
1. **Baseline Run:** Execute test suite on delivered code. Must PASS ($ExitCode = 0$).
2. **Compilation Guardrail:** In compiled languages (Go, Rust, C++), the mutation MUST compile cleanly. A compilation error (e.g. `declared and not used`, type mismatch) is **NOT** a killed mutation. If a mutation fails to compile on the first attempt, discard it immediately—never iterate in open-ended bash debugging.
3. **Execute Mutated Run:** Rerun the specific test covering that logic:
   - **KILLED MUTATION (PASS):** Test fails with exit code $\neq 0$ due to assertion failure. The test is rigorous and genuine.
   - **SURVIVED MUTATION (FAIL):** Test passes ($ExitCode = 0$) despite broken logic. The test is a shallow/vibe test.
4. **Revert Mutation:** Instantly revert the file back to clean git state (`git checkout -- [file]`).
5. **Report:** If any mutation survived, emit `verification_verdict: FAIL` with the exact surviving mutation scenario.
