---
name: investigate
description: >
  Explore a codebase, diagnose an observed failure, or assess a migration with
  evidence-backed analysis and explicit alternatives.
  Trigger: When the operator requests investigation or the exploration phase is activated.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Investigate — Evidence-Backed Exploration

## Objective

Turn an investigation topic into a bounded, citable analysis. The output maps
the affected code, explains the observed behavior, compares viable approaches
when appropriate, and identifies the next decision. Exploration is analysis,
not implementation.

## Activation

Activate with a topic, focus (`ARCHITECTURE`, `INVESTIGATION`, or `MIGRATION`),
project, change name, and constraints. Load the existing project context when
available. If the requested artifact is absent, report the gap before reading
unrelated areas.

## Method

1. Identify entry points with a focused search. Read each entry point fully and
   trace one dependency level deep.
2. Read the relevant tests and configuration. For an incident, read the exact
   error and trace trigger → boundary → failure. For migration, inventory
   versions, coupling, deprecated APIs, and rollback boundaries.
3. Cite every material claim as `path:line` and distinguish observed facts from
   interpretation.
   Every material claim requires a file:line citation.
4. For architecture or migration focus, compare at least two approaches. Give
   each approach concrete pros, cons, effort, and risk; mark exactly one
   recommendation. For investigation focus, provide symptom, root cause,
   evidence chain, suggested fix, and blast radius.
5. Stop at the declared scope. Record unresolved evidence instead of filling
   it with assumptions.

## Decision gates

- `explore/evidence`: every affected file in the result was actually read and
  every material claim has a citation.
- `explore/alternatives`: architecture and migration outputs contain two or
  more viable approaches with one recommendation; incident outputs contain a
  causal evidence chain.
- `explore/scope`: the analysis names limitations and does not imply code
  changes that were not investigated.

If a gate fails, return `partial` or `blocked` with the missing evidence rather
than presenting an unqualified recommendation.

## Valid example

Given a request to assess rate limiting, the result cites the router and
middleware files, compares in-process and shared-store approaches, records
effort and risk for both, and recommends one approach tied to observed project
patterns.

## Invalid example

Given only a directory listing, the result MUST NOT claim that a handler uses a
particular store or that a migration is safe. It returns `blocked` for missing
source evidence and names the file needed to proceed.

## Output checks

Return topic, focus, affected files, evidence citations, approaches or
diagnosis, recommendation, limitations, status, and confidence. Use canonical
phase status values from the generated contract. Keep the investigation within
the 600–1,000 word budget and ensure exactly one recommended approach where
approaches are required.

## Boundary discipline

Investigation owns reasoning from observed evidence. It does not edit files,
select a final implementation contract, or convert a hypothesis into a fact.
Use narrow searches before broad reads, preserve the exact error text, and
separate direct evidence from inferred control flow. If a dependency is
unavailable, name the unavailable observation and reduce confidence. If two
approaches have the same effort, prefer the one that preserves existing
boundaries and provides a simpler rollback, but state that preference rather
than hiding it. Keep the analysis reproducible: another reviewer should be able
to follow each citation and reach the same conclusion without relying on a
private transcript.
Record the command, revision, and timestamp needed to reproduce the evidence
chain, and keep negative findings visible to later reviewers.
Confidence follows evidence coverage, never the amount of prose.

## References

- `_shared/sdd-phase-contract.md` — result envelope and status vocabulary.
- `explore/evidence`, `explore/alternatives`, `explore/scope` — executable gates.
- `internal/components/sdd/phasecontract` — canonical result definitions.
- `internal/components/sdd/contractgen` — generated reference source.
