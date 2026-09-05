# Herdr executable qualification

## ADDED Requirements

### Requirement: REQ-HERDR-001 — Tool-specific probe
The bridge MUST probe Herdr using argv `["--version"]` and Cortex-IA using `["version"]`; resolution failures MUST preserve safe preacceptance transport fallback. This is a retrospective specification of the existing repair, not evidence of a prior plan.

#### Scenario: Happy path
- GIVEN a controlled Herdr rejecting `version` and accepting `--version`
- WHEN `firstExecutable("herdr")` qualifies its PATH candidate
- THEN it resolves Herdr, and an intercepted argv is exactly `["--version"]`.

#### Scenario: Edge case
- GIVEN a controlled Cortex-IA accepting both flags
- WHEN `firstExecutable("cortex-ia")` qualifies its PATH candidate
- THEN intercepted argv is exactly `["version"]`; a mutant changing both probes to `--version` is rejected by the oracle.

#### Scenario: Error state
- GIVEN no Herdr candidate exists in the isolated lookup environment
- WHEN qualification runs
- THEN it reports executable-not-found and the caller can choose direct transport before acceptance, never false-positive Herdr success.

## Verification and Review
Existing task: `critical-review-herdr-probe`, in_review revision3 at planning. No dependency on Cortex-only contract approval. Reviewer SHALL fetch current state, independently verify this specification, rerun the oracle and exact-argv negative control, synchronize bounded AST evidence, and approve only at current revision if both Spec and Standards pass.

From the hardening worktree: `TARGET_FILE="$PWD/internal/assets/plugins/herdr-bridge.ts" node /tmp/opencode/critical-review-herdr-probe/oracle-herdr-probe.mjs --mode=green` and `node --experimental-strip-types --check internal/assets/plugins/herdr-bridge.ts` and `git diff --check`, each exit0. The same green oracle against root original currently must exit1; this negative control is not a product PASS. Reinspect fixtures to prevent host candidate leakage. Use an ephemeral in-memory VM argv interceptor as in #15; no persistent regression file.

Cortex #14–15 attest earlier runs, not runs performed by this planner. Syntax checking is not full type checking. Probe success is not transport/pane E2E success. No extra product change is required solely to satisfy retrospective documentation.
