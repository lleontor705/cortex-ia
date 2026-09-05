# Delegation / Cortex recovery

## Why
Herdr 0.8.2 rejects `version` but accepts `--version`. Its one-line repair has independent functional evidence (Cortex #14–15), but formal approval lacked an OpenSpec contract. Cortex-only adaptation timed out (#16); checkpoint 76baed68 then failed the clean-related-same-HEAD gate before acceptance (#19–20). Neither event proves a general AGY outage.

## What Changes
- Specify the existing Herdr repair retrospectively, without inventing a prior spec or test-first plan.
- Complete Cortex-only planning and independent review using observation ID plus verifiable content digest, not fictional revisions.
- Preserve OpenSpec/hybrid and SQLite authority; decompose the existing blocked parent at CAS6 into three bounded, sequential replacement slices.
- Integrate reviewed assets into one local branch `fix/delegation-cortex-recovery`, then build, inventory safe local installation effects, install with backup, reload, and verify actual delegation.

## Authorization and Scope
This OpenSpec exception applies only to this initial recovery, including present review. General preference remains auto / Cortex / isolated_worktree. Reuse session and board `critical-review-cortex-ia-20260905`. Local branch, installation and E2E are authorized; remote push/PR are not. Existing contracts draft is reference material, not approved product. Herdr review can proceed immediately against `specs/herdr/spec.md`, independently of contract implementation.

## Non-goals
No main ref movement, HEAD gate bypass, retry of the exhausted parent, new initiative board/session, role permission expansion, timeout tuning, telemetry/model/config preference changes, Go hardening, web work, or new persistent tests outside the original TUI/simple installation-copy policy.

## Risks
Work decomposition inherits the root workspace; it does not relocate claims. Installed assets require rebuild and reload. Current Git cleanliness, conforming Go/lint availability, and the complete live installation write manifest require executable checks by an authorized controller. Planner has file reads, not a Git shell; historical clean/diff evidence is not a fresh status result.
