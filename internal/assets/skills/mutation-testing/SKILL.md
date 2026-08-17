---
name: mutation-testing
description: Inject syntactic and semantic faults into delivered code to verify whether test suites are genuine verifiers or shallow 'vibe tests'.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# Mutation Testing & Adversarial Verification

Use this skill during independent code review (`reviewer` role) to validate the effectiveness of tests submitted by implementation minions.

## 1. The 'Vibe Test' Problem
LLMs frequently write superficial tests that pass without actually asserting invariants (e.g. tests that assert `true === true`, lack assertions, or test mock objects disconnected from implementation logic). Mutation testing validates tests by verifying they FAIL when bugs are deliberately introduced.

## 2. Mutation Operators to Apply

Apply 1 or 2 minimal, reversible mutations to the target implementation:

1. **Boundary / Operator Mutator:**
   - Change `>` to `>=` or `<`.
   - Change `===` to `!==`.
   - Change `+` to `-`.
2. **Boolean / Inversion Mutator:**
   - Invert boolean return value (`return true` -> `return false`).
   - Negate conditional predicate (`if (isValid)` -> `if (!isValid)`).
3. **Void / Return Statement Mutator:**
   - Remove function call side effect or replace return payload with `null`/`undefined`/`{}`.

## 3. Verification Protocol
1. **Baseline Run:** Execute the test suite on the delivered code. Ensure it passes ($ExitCode = 0$).
2. **Apply Mutation:** Temporarily edit one key logic branch in the implementation file.
3. **Execute Mutated Run:** Rerun the specific test that claims to cover that logic.
   - **KILLED MUTATION (PASS):** Test fails with exit code $\neq 0$. The test is rigorous and genuine.
   - **SURVIVED MUTATION (FAIL):** Test passes ($ExitCode = 0$) despite broken logic. The test is a shallow/vibe test.
4. **Revert Mutation:** Instantly revert the file back to clean git state (`git checkout -- [file]`).
5. **Report:** If any mutation survived, emit `verification_verdict: FAIL` with the exact surviving mutation scenario.
