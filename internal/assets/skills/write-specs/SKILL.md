---
name: write-specs
description: >
  Convert an approved proposal into domain-grouped delta specifications with
  unique requirement IDs and testable Given/When/Then scenarios.
  Trigger: After proposal approval, when the specification phase is activated.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Write Specs — Testable Delta Requirements

## Objective

Translate proposal intent into precise requirements that expose behavior,
boundaries, and failure states. Specifications are stakeholder-readable and
implementation-neutral. Each requirement is traceable through a generated ID.

## Activation

Activate only when the complete proposal is available. Read the proposal before
classifying domains. Existing domains receive ADDED, MODIFIED, and REMOVED
delta sections; new domains receive a complete initial contract.

## Method

1. Build a skeleton mapping every proposal scope item and risk to a domain,
   section, requirement ID, and scenario count.
2. Assign unique IDs in the form `REQ-{DOMAIN}-{NNN}`. Use RFC 2119 terms such
   as MUST, SHOULD, MAY, and MUST NOT for normative behavior.
3. For each requirement, write strict scenarios in this order: Given,
   When, Then. Include a happy path, an edge case, and an error state. Use
   Gherkin only for observable workflow, rule, behavior, or gate outcomes;
   place implementation combinatorics in the named test oracle.
4. Add a coverage assessment for happy paths, edge cases, and error states.
   Mark gaps `partial` or `missing` and explain them honestly.
5. Name the primary test oracle for each requirement. Preserve ambiguity as an
   explicit log entry rather than silently choosing a behavior.

## Decision gates

- `spec/traceability`: every proposal scope item and risk maps to one or more
  unique requirement IDs.
- `spec/scenarios`: every requirement has strict Given/When/Then happy, edge,
  and error scenarios.
- `spec/coverage`: coverage labels match the listed scenarios and unresolved
  gaps are disclosed.
- `spec/vocabulary`: scenario terms use canonical phase/status fields and do
  not introduce a competing contract vocabulary.

If the proposal is absent, return `blocked`. If coverage is incomplete, return
`partial`; never convert an unknown behavior into a passing requirement.

## Valid example

For `REQ-AUTH-004`, a happy scenario describes a valid token, an edge scenario
describes concurrent refresh, and an error scenario describes an expired token.
Each uses Given/When/Then and names the test oracle.

## Invalid example

A paragraph saying “the service should work correctly” is invalid: it has no
unique ID, no observable precondition, and no failure behavior. Return a spec
gate failure until the three scenarios are written.

## Output checks

Return domain list, delta sections, requirement IDs, scenarios, oracle links,
coverage, ambiguity log, status, and confidence. Use canonical status from the
generated contract. Verify IDs are unique, scenario counts are complete, and
the concatenated document is internally consistent before handoff.

## Boundary discipline

Specifications describe observable obligations and leave implementation choices
to design. Keep one requirement focused on one behavior, give it a stable ID,
and avoid embedding file names or speculative APIs in scenario text. A negative
scenario is required even when the proposal emphasizes a happy path because it
defines the fail-closed boundary. When a term could mean phase execution,
verification, or an adapter outcome, name the typed field explicitly. Preserve
the original proposal wording in traceability notes when refinement changes
scope. This keeps later test failures actionable and prevents an implementation
from satisfying a weaker interpretation than the approved outcome.
Record the proposal revision and requirement inventory so later design work can
detect additions, removals, and changed scenario obligations.
Names and IDs remain stable across revisions.
Scenario language stays observable and implementation-neutral.
Avoid hidden assumptions in prose.

## References

- `_shared/sdd-phase-contract.md` — result envelope and status vocabulary.
- `spec/traceability`, `spec/scenarios`, `spec/coverage`, `spec/vocabulary` —
  executable gate identifiers.
- `internal/components/sdd/phasecontract` — canonical contract definitions.
- `internal/components/sdd/contractgen` — generated reference source.
