---
name: draft-proposal
description: >
  Turn exploration evidence into a bounded, reviewable change proposal with
  measurable acceptance and a reversible rollout.
  Trigger: When exploration is complete and proposal drafting is activated.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Draft Proposal — Bounded Change Intent

## Objective

Transform the exploration artifact into an intent that a product owner,
architect, and implementer can review. The proposal names the problem, outcome,
scope, affected areas, risks, dependencies, acceptance evidence, and rollback
boundary. It does not become a specification or implementation plan.

## Activation

Activate only when `sdd/{change}/explore` is available and the operator supplies
the change name and project. Read the complete exploration artifact before
drafting. If it is missing, return `blocked` and request exploration; do not
invent scope from the title.

## Method

1. Skeleton first: write one sentence each for intent, in-scope, out-of-scope,
   approach, affected areas, risks, rollback, and measurable acceptance.
2. Check the skeleton against exploration citations. Every affected path must
   exist or have a verified parent directory. Every risk must point to a real
   dependency or code pattern.
3. Expand the skeleton into these sections: Intent; Scope In; Scope Out;
   Approach; Affected Areas; Risks; Rollback Plan; Dependencies; Success
   Criteria.
4. Make scope-out explicit and explain why each item is deferred. Make each
   success criterion observable through a test, command, receipt, or behavior.
5. Derive overall risk from the highest justified likelihood/impact pair. If a
   decision is unresolved, state the assumption and its consequence.

## Decision gates

- `proposal/evidence`: intent, affected areas, and risks cite the exploration
  artifact and verified paths.
- `proposal/scope`: at least one concrete scope-in and one justified scope-out
  item exist; no affected area silently expands scope.
- `proposal/rollback`: rollback names the assets to restore, the verification
  command, and data or external-state considerations.
- `proposal/acceptance`: every success criterion has a repeatable observation.

Failure of any gate returns `partial` or `blocked` with the exact missing
section. Do not advance a proposal with an untestable success claim.

## Valid example

Given exploration that identifies a middleware boundary and two alternatives,
the proposal selects one with rationale, lists the middleware files, defers the
dashboard, defines a removal-and-restore rollback, and states the command that
proves the acceptance behavior.

## Invalid example

Given an exploration with no verified affected path, a proposal MUST NOT name
imaginary files or claim rollback is simply “revert.” It returns `blocked` until
the path and restoration boundary are evidenced.

## Output checks

Return change title, intent, scope in/out, approach, affected areas, risk level,
rollback plan, dependencies, success criteria, evidence references, status,
and confidence. Use canonical phase status values and the generated proposal
contract. Keep the proposal within its output budget and ensure the rollback
gate is explicitly true.

## Boundary discipline

The proposal is an agreement about intent and boundaries, not a second design
document. Keep implementation details at the level needed to compare the
chosen approach and its alternatives. Do not hide product decisions inside a
technical risk table. A scope-out item is a deliberate deferral with a reason,
not an omission. A rollback step must describe the managed bundle, receipt, or
configuration that returns the system to its prior observable state. If the
exploration supports several outcomes, expose the decision instead of selecting
one silently. Preserve citations so reviewers can distinguish evidence from
proposal judgment.
Record the proposal revision and evidence fingerprints so a later review can
distinguish an updated decision from an accidental omission.

## References

- `_shared/sdd-phase-contract.md` — result envelope and status vocabulary.
- `proposal/evidence`, `proposal/scope`, `proposal/rollback`,
  `proposal/acceptance` — executable gate identifiers.
- `internal/components/sdd/phasecontract` — canonical contract definitions.
- `internal/components/sdd/contractgen` — generated reference source.
