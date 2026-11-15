# 2026-10-09 — Workflow Improvements Grounding Audit

**Scope**: Compare the sibling `gentle-ai` AI-development workflow with
`cortex-ia`'s, identify what to adopt, and decide how SDD is repositioned.

**Cortex counterpart**: `architecture/workflow-comparison-gentle-ai` (#625).
This report and that observation reference each other; neither is the sole source.

## Evidence

- `gentle-ai`: `AGENTS.md` (skills index), `docs/usage.md` (ODD protocol),
  `docs/trigger-rules.md`, `docs/intended-usage.md`, `docs/engram.md`,
  `docs/skill-style-guide.md`, `odd/tasks/task-size-canon.md`,
  `.github/workflows/ci.yml`.
- `cortex-ia` baseline: `internal/assets/skills/_shared/workflow-map.md`,
  `internal/assets/skills/_shared/cortex-work-protocol.md` (case matrix, LOC
  budget, evidence composition), and the root `AGENTS.md`.

## Findings — ODD/RDD patterns

- **ODD (recoverable feature doc)**: `gentle-ai` retired SDD in v4.0.0 and
  replaced it with one recoverable feature doc per unit carrying a header, a
  `## Specs` section with verbatim `S#` quotes, `## Tasks` lines linked to `S#`
  with commit hashes, and a `## Log` whose L1 is the user request verbatim. A
  memory mirror is reconciled by the parent on resume, and non-atomic dual writes
  must be read back.
- **RDD (risk-driven review)**: a candidate is frozen to lineage before review;
  a native risk tier selects review depth (passive → medium → high); agents may
  only *raise* the tier, never lower it; at most one bounded correction; delivery
  stays human-owned.
- **Deterministic state machine**: `Working/Checking/Ready/NeedsDecision` with
  binary-owned transitions; models never vote.
- **Evidence culture**: dated audits under `docs/audits/`, qualifications under
  `docs/evidence/`, and verbatim quotes plus RED/GREEN runs with commit hashes.

## Adoptions (10)

1. Dead-code ratchet baseline over a committed count file.
2. CLI refusal/error-string ratchet over a committed baseline.
3. Advisory CI wiring for both ratchets that never blocks merges.
4. Verbatim provenance (L1) with a typed-placeholder secrets guard.
5. Per-spec (`S#`/`REQ`) verdict protocol, one verdict per referenced id.
6. Escalate-only review tier: raise with a stated reason, never lower.
7. Resume-test sizing oracle: resumable from request + git diff alone.
8. Skill-body token budget (180–450 target, 1000 hard max), trigger-first.
9. Feature-doc Log structure with mirror read-back reconciliation.
10. Advisory per-slice delivery forecast with no gate authority.

Tracked as `REQ-WF-001` … `REQ-WF-010`; the dated audit/evidence convention
(`REQ-WF-011`) and the SDD repositioning (`REQ-WF-012`) are recorded here, and the
documentation-only gentle-ai authority backlog (`REQ-WF-013`) is SECONDARY and
out of scope for cortex-ia product code.

## Verdict — SDD repositioning

**pivoting/absorbed.** `gentle-ai` shipped SDD, lived it, then deliberately
retired it in favor of ODD; that retirement is evidence-backed, not accidental.
`cortex-ia` does not import ODD wholesale — ODD assumes a single-owner,
human-gated flow with no durable claim/lease/approval authority plane, which is
`cortex-ia`'s structural advantage. Instead SDD is **pivoted** to Tier 3,
multi-session, and regulated domains, while the lightweight ODD-derived forms
(short paragraph plus executable tests, delta specs, feature-doc Log) are
**absorbed** for Tier 1/2. Executable verification, including the mutation gate
where applicable, becomes the central spec-fulfilled criterion.

## Follow-ups

- Reconcile skill-body token budgets with the existing 4-layer XML anatomy in the
  agent-writing contract rather than stacking two rules.
- Keep ratchet baselines advisory or script-updatable; per-PR maintenance
  friction is real.
- Detect staleness of `gentle-ai`-targeted proposals against observation
  `architecture/workflow-comparison-gentle-ai` (#625).
