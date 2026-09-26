# Proposal — Agent testing-quality gates (add-agent-testing-gates, Wave 1)

## Problem

The installed agent flow accepts a green test suite as proof of correctness without any evidence that the suite can fail. Repository evidence (verified 2026-09-26):

- The mutation-testing skill (`internal/assets/skills/mutation-testing/SKILL.md`, v1.1.0) is unreachable in practice: it is referenced only in `internal/assets/AGENTS.md:134,201` (routing table and a sequence-diagram caption) and has zero occurrences in `internal/assets/agents/reviewer.md`, `_shared/cortex-work-protocol.md`, `_shared/workflow-map.md`, or the fast-tdd loop.
- No check anywhere proves a test can fail: reviewer Phase 3 (`agents/reviewer.md:370-378`) verifies oracle presence and greenness only. The sole test-strength instrument is the voluntary skill with a `SKIPPED_UNSUPPORTED` fallback (`mutation-testing/SKILL.md:19`).
- `agents/reviewer.md:346-421` carries a parallel inline copy of the 5-phase/3-lens pipeline that also lives in `skills/code-review-adversary/SKILL.md:16-48` — a duplication hazard against the single-source rule in `_shared/agent-writing-contract.md`.
- `skills/fast-tdd/SKILL.md:35-40` receipt schema has red/green/refactor/regression only — no mutation or coverage field; `skills/implement/SKILL.md:48-63` requires command+exit+revision evidence but nothing about test strength.
- The reviewer is barred from writing temporary tests (`agents/reviewer.md:337-341`), so any mutation execution must be implementer-side, pre-transition.

## User value

- A shallow "vibe test" can no longer ride a green run through implementation and review: fast-TDD-eligible tasks must present mutation evidence (or a mandatory static-analysis fallback), and reviewers apply an explicit perpetually-green lens.
- Wave-1 gates land as durable instruction text in the exact files agents already load (protocol, AGENTS.md, skills, role agents), so behavior changes on the next `cortex-ia sync` + rebuild with zero runtime-code risk.
- REQ↔test traceability becomes searchable: REQ-bound tests follow a naming convention that planner verification commands target directly.

## Approach

Instruction-only Wave 1: edit eleven markdown assets across four DAG tasks in one board. The normative MUST lives once in `_shared/cortex-work-protocol.md`; `AGENTS.md` mirrors it via cross-reference bullets; skills operationalize (implement/fast-tdd mutation receipt, mutation-testing invocation contract, code-review-adversary perpetually-green lens as single source, planner TestREQ naming); role agents cite skill sources instead of duplicating them. All decisions D1–D7 are recorded in design.md and already user-locked; this change encodes, not re-opens, them.

## Non-goals

- Zero Go code, zero Go test additions, zero schema changes this wave (assets are go:embed markdown; `go build ./...` re-embeds).
- Deferred to Waves 2–3: OpenSpec/delegation validator enforcement fields for mutation evidence (F1), durable receipt-schema mutation/coverage fields in SQLite work authority (F2), web-console test-quality metrics display (F3).
- No coverage hard gate: coverage delta is advisory only.
- No changes to installed `~/.cortex-ia/opencode/` copies; sync propagates assets.
- The reviewer's prohibition on writing/executing tests stays unchanged.

## Risks

- **Instruction overload**: adding MUSTs to already-long contracts risks agent non-compliance. Mitigated by single-source placement (one normative clause, mirrors are pointers), exemption mirroring the existing TDD exemption precedent (`skills/implement/SKILL.md:21`), and STATIC-ANALYSIS as a bounded fallback.
- **Cross-file drift**: three tasks edit mutually referenced files. Mitigated by anchor-token acceptance criteria and per-task consistency greps in the DAG verification fields.
- **False rigor**: mutation evidence can be fabricated in prose. Mitigated by REQ-ATG-003 requiring reviewer independent verification against the actual diff and test source (double-blind rule already in place).
