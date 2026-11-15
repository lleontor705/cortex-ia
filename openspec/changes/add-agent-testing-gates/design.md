# Design — Agent testing-quality gates (Wave 1)

Locked decisions (encoded, not re-opened). Every insertion point below was verified against repository file:line evidence on 2026-09-26.

## D1 — Mutation evidence gate (A + E)

For fast-TDD-eligible code tasks (a persistent test delta in a testable unit), the implementer runs 1–2 reversible mutations per the mutation-testing skill BEFORE `transition in_review`. The fast-tdd receipt gains a `mutation` field:

| Value | Meaning | Consequence |
|---|---|---|
| KILLED | covering test failed under mutation | genuine verifier; transition allowed |
| SURVIVED | test passed despite broken logic | shallow test; strengthen and re-run; never transition with SURVIVED |
| STATIC-ANALYSIS | environment/tool cannot execute mutations | minimum bar: mutation-testing skill Static Assertion Analysis — explicit assertion of error boundaries, return payloads, failure branches; required field, never silently skipped |

The reviewer independently verifies the evidence in Phase 3/4 and applies the perpetually-green lens: a test that would pass under any behavior change is an oracle gap → FAIL.

## D2 — Enforcement surface (single source)

The normative MUST lives once in `_shared/cortex-work-protocol.md`: §4 lifecycle item 5 gains the mutation-evidence clause (line ~108 insertion point: "Run focused checks, then proportional regression"); §8 evidence composition (lines 162-164) gains the receipt field and the reviewer duty. Scope phrase: "fast-TDD-eligible code tasks"; exempt categories mirror `skills/implement/SKILL.md:21` (documentation, declarative configuration, generated output, no reliable fast oracle). `internal/assets/AGENTS.md` mirrors as cross-reference bullets only: one new §2 Pragmatic invariant item ("Mutation Evidence Gate"), one §3 Pre-Transition Workload Preflight bullet, and the Phase-4 diagram caption at :201 ("Independent Checks & Mutation Testing") upgraded from decorative to normative cross-reference.

## D3 — De-duplication

`agents/reviewer.md` keeps its operational 5-phase pipeline but adds a citation making `skills/code-review-adversary/SKILL.md` the elaborated single source; the perpetually-green/test-strength lens wording is written once in the skill and referenced from the agent by pointer. No forked wording.

## D4 — Oracle-strength mandate (B)

implement + fast-tdd skills: every new or changed test demonstrates failure capability — asserts a boundary, a payload, or a failure branch; no test that cannot fail. Skill-local MUST, wording consistent with D1.

## D5 — Coverage ratchet (D)

Advisory pattern only: implementers of test-bearing changes MAY compute `go test -coverprofile` before/after and report the delta in the receipt. Never a hard gate this wave (target repos vary).

## D6 — REQ↔test traceability (C)

Naming convention: tests for REQ-bound work follow `TestREQ_{DOMAIN}_{NNN}_<slug>` (language-conditional adaptation allowed). Planner verification commands for such tasks target those names (`-run TestREQ_...`). Recorded in `skills/planner/SKILL.md`, `_shared/cortex-convention.md` traceability chain, and the `_shared/workflow-map.md` structural-validation note. Convention-level only — no Go validator change this wave.

## D7 — Language and style

All artifacts in English; zero-noise contract text (no narrative filler), comments restricted to non-obvious invariants.

## Wave boundaries

- Wave 1 (this change): 11 markdown assets, 4-task DAG, board agent-testing-quality.
- Wave 2 (deferred): F1 OpenSpec/delegation validator fields for mutation evidence; F2 durable receipt-schema columns in internal/delegation.
- Wave 3 (deferred): F3 web-console test-quality metrics surface.

## File map (disjoint by task)

- T1: internal/assets/skills/_shared/cortex-work-protocol.md, internal/assets/AGENTS.md
- T2: internal/assets/skills/fast-tdd/SKILL.md, internal/assets/skills/implement/SKILL.md, internal/assets/agents/implement.md
- T3: internal/assets/skills/mutation-testing/SKILL.md, internal/assets/skills/code-review-adversary/SKILL.md, internal/assets/agents/reviewer.md
- T4: internal/assets/skills/planner/SKILL.md, internal/assets/skills/_shared/cortex-convention.md, internal/assets/skills/_shared/workflow-map.md
