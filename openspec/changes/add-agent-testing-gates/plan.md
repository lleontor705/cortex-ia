# Plan — Agent testing-quality gates (Wave 1, integrated contract)

Workflow: sdd-lite | spec_plane: openspec | Board: agent-testing-quality | Workload: flexible | Language: English

## Intent

Bake AI-era testing-quality practices — mutation-proven test strength, perpetually-green oracle detection, and REQ-to-test traceability naming — into the installed OpenCode asset set (shared contracts, AGENTS.md, skills, role agents) so a green test run is never sufficient proof without evidence the suite can fail. Wave 1 is instruction-only: markdown asset edits exclusively, zero Go code.

Non-goals: no Go code or persistent tests, no SQLite/delegation schema, no validator changes, no coverage hard gate, no installed-copy edits (sync propagates assets). Waves 2–3 (openspec validator fields F1, delegation evidence fields F2, web metrics F3) are explicitly deferred.

## Requirements

Normative delta lives in `specs/agent-quality/spec.md` under `## ADDED Requirements`; each REQ carries Happy Path / Edge Case / Error-state scenarios plus a `Test:` oracle line:

- REQ-ATG-001 — implementer-side mutation-evidence gate for fast-TDD-eligible tasks; KILLED / SURVIVED / STATIC-ANALYSIS semantics; exemption parity with the existing TDD exemption list.
- REQ-ATG-002 — mandatory `mutation` field in fast-tdd and implement receipts; missing or SURVIVED-bearing evidence never yields PASS; exempt tasks record the reason.
- REQ-ATG-003 — reviewer Phase 3 independent verification of mutation evidence plus the perpetually-green test-strength lens, text single-sourced in the code-review-adversary skill.
- REQ-ATG-004 — one normative MUST in cortex-work-protocol.md (§4 lifecycle + §8 evidence composition); AGENTS.md and reviewer.md mirror via cross-references only; Phase-4 diagram caption upgraded to a normative pointer.
- REQ-ATG-005 — oracle-strength failure-capability mandate, advisory-only coverage delta, and `TestREQ_{DOMAIN}_{NNN}_<slug>` naming recorded in planner skill, cortex-convention.md traceability chain, and workflow-map.md.

## Design

Decisions D1–D7 are user-locked and elaborated in design.md. Enforcement shape: the MUST appears exactly once in the shared protocol (per the single-source rule in agent-writing-contract.md); skills operationalize it at the point of action; agents cite, never fork, skill wording. Mutation execution is implementer-side because the reviewer cannot write or run temporary tests; SURVIVED blocks transition until the test is strengthened; unsupported environments fall back to a mandatory, never-silent STATIC-ANALYSIS field matching the mutation-testing skill's Static Assertion Analysis bar.

## Tasks

T1 blocks T2, T3, T4; T2 ∥ T3 ∥ T4 with mutually disjoint allowed_files. Full machine-readable contracts are in tasks.md and materialized as board work items.

- t1-contracts-core — Requirements: REQ-ATG-001, REQ-ATG-002, REQ-ATG-004 — edit _shared/cortex-work-protocol.md and internal/assets/AGENTS.md.
- t2-implement-loop — Requirements: REQ-ATG-001, REQ-ATG-002, REQ-ATG-005 — edit fast-tdd SKILL.md, implement SKILL.md, agents/implement.md.
- t3-reviewer-side — Requirements: REQ-ATG-003, REQ-ATG-004 — edit mutation-testing SKILL.md, code-review-adversary SKILL.md, agents/reviewer.md.
- t4-planner-traceability — Requirements: REQ-ATG-004, REQ-ATG-005 — edit planner SKILL.md, _shared/cortex-convention.md, _shared/workflow-map.md.

## Verification strategy

Per-task raw greps prove anchor tokens were baked (mutation evidence / STATIC-ANALYSIS / perpetually-green / Mutation Evidence Gate / failure capability / TestREQ_); every task runs `go build ./...` to re-embed assets; T2 additionally runs `go test ./internal/pipeline/ -run TestInstall_DryRun -count=1` as the clean-copy asset smoke. Reviewers audit cross-file wording against the single-source rule; Wave 2–3 items are out of scope by design.
