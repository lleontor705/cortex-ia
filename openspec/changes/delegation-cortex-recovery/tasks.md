# Recovery task DAG

Board/session: `critical-review-cortex-ia-20260905`. Workflow: sdd-lite/integrated under one-time OpenSpec exception. Durable task definitions contain exact allowlists, commands and acceptance. All commands run in the task's verified checkout; exit0 required unless explicitly a negative control. See design.md for mandatory branch, baseline, claim, gate, migration and stop protocol. No product writes on main; no parent retry.

## Replacement chain (CAS6)
- [ ] 1.1 `critical-review-cortex-pin-planning` — 110 lines. Establish feature branch at unchanged original root HEAD; complete canonical snapshot validation and planner production path (REQ-CORTEX-001/002). Writable: `internal/assets/skills/_shared/cortex-convention.md`, `internal/assets/agents/planner.md`, `internal/assets/skills/planner/SKILL.md`. No reviewer/routing/install work. Check: `git diff --check` plus durable inline Python contract probe; independent real-content digest/drift exercise.
- [ ] 1.2 `critical-review-cortex-independent-review` — 65 lines. Depends 1.1. Writable: `internal/assets/agents/reviewer.md`, `internal/assets/skills/code-review-adversary/SKILL.md`. Implement separate verdict axes and pin validation without OpenSpec in cortex mode. No approval/authority API changes. Check: `git diff --check` plus durable inline Python probe and independent scenario walk.
- [ ] 1.3 `critical-review-cortex-routing-bootstrap` — 125 lines. Depends 1.2. Writable: `internal/assets/AGENTS.md`, `internal/assets/agents/orchestrator.md`, `internal/assets/skills/orchestrator/SKILL.md`, `internal/assets/skills/_shared/cortex-work-protocol.md`, `internal/assets/commands/sdd.md`. Complete all-plane routing, decision-map and bounded bootstrap; split create/decompose permissions. No model/telemetry/default changes. Check: `git diff --check` plus durable inline Python probe and three-plane phase matrix.

## Existing independent review
- [ ] `critical-review-herdr-probe` — existing in_review3, not recreated. Review immediately against specs/herdr/spec.md and #14–15. Can run alongside 1.1; no dependency on replacement chain.

## Delivery join
- [ ] 2.1 `critical-review-recovery-integrate` — <=300 newly changed lines. Depends 1.3 and Herdr done. Preserve child commits/hashes; integrate only approved Herdr diff and these six unchanged planning documents into the same feature branch. Durable allowlist enumerates files. Check branch/main invariant, `git diff --check`, syntax and Herdr green oracle, all exit0. No fixes or installation.
- [ ] 2.2 `critical-review-recovery-build-readiness` — zero source lines. Depends 2.1. Writable only `bin/cortex-ia`; temporary caches/isolated smoke homes are operational outputs, never source scope. Exact Go1.26.1 vet/lint/test/build commands in durable definition. Produce exact local installation and E2E effect manifest before further task creation. No live installation, configuration changes or remote publication.

## Subsequent materialization
Local-install depends 2.2; real-E2E depends approved installation. Their exact writable operational manifests/checks require 2.2 evidence; they are deliberately NOT executable DAG nodes yet. Same board, fresh bounded tasks, no generic approval question. Forecast materialized nodes: 600 lines total; each <=350. Operational source forecast: zero. Evidence/approval, installation and real transport remain distinct verdicts.
