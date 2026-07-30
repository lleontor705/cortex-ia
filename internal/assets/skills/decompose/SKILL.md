---
name: decompose
description: >
  Break an approved design into small dependency-ordered work units with
  requirement-linked acceptance criteria and a validated task board.
  Trigger: After design approval, when task decomposition is activated.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Decompose — Dependency-Ordered Work Units

## Objective

Turn design file changes and specification scenarios into executable tasks.
Every task has one logical outcome, a bounded file set, dependencies, acceptance
criteria, and a clear verification command. Decomposition does not implement or
reinterpret approved requirements.

## Activation

Activate only when proposal, specification, and design artifacts are available.
Read the design file-change table as authoritative and cross-reference every
requirement and test strategy entry. Stop if an artifact is missing.

## Method

1. Group files by ownership and logical boundary. Keep a task to one to three
   files where practical; split larger units at a stable interface.
2. Map each task to requirement IDs, scenarios, typed contracts, and tests.
3. Assign dependency-free foundation work first, then core behavior,
   integration, verification, and only explicitly designed cleanup.
4. Build an acyclic graph. Tasks in one parallel group MUST have no dependency
   on each other. Dependency references use stable task IDs, not prose.
5. For test-first projects, make the first work item a failing test, followed
   by minimal implementation and a refactor that preserves green tests.
6. Produce a workload forecast with task count, parallel groups, estimated
   changed lines, and review-risk notes.

## Decision gates

- `tasks/coverage`: every design file and requirement maps to at least one task
  with scenario-derived acceptance criteria.
- `tasks/dag`: no cycles, no later-phase dependency, and no same-group edge.
- `tasks/scope`: no task includes an out-of-scope proposal item or unowned file.
- `tasks/readiness`: only dependency-free tasks are ready; each task names its
  test and rollback boundary.

If a file-change or requirement mapping is ambiguous, return `partial` with the
unmapped item. If the graph is cyclic, return `blocked` and identify the edge.

## Valid example

Types and schema are task `1.1` with no dependencies; service task `2.1`
depends on `1.1`; integration task `3.1` depends on `2.1`; each has a test
command and acceptance criteria linked to a requirement scenario.

## Invalid example

A task titled “finish the feature” touching ten files, depending on a later
phase, and lacking a scenario is invalid. The decomposition gate rejects it and
requires a split at a typed boundary.

## Output checks

Return board ID, task list, phases, parallel groups, coverage map, workload
forecast, risks, rollback boundaries, status, and confidence. Use canonical
status values from the generated contract. Verify the persisted markdown list
and board entries have equal counts and dependency sets.

## Boundary discipline

Decomposition owns sequencing and reviewability, not architectural invention.
Keep each task reversible at a file or interface boundary and make its
acceptance criteria observable. If two tasks share a file, either combine them
or identify a non-overlapping edit boundary before assigning parallel work.
Do not make a downstream task ready merely because its predecessor is likely to
finish; readiness follows the recorded graph. A task that discovers a design
gap should report it and stop rather than encode an assumption into acceptance
criteria. Keep the board, markdown list, and coverage map synchronized so the
next phase can recover without interpreting prose.
Record the design revision and board fingerprint so resumed work can detect
drift before claiming a task is complete.
Unresolved design questions block readiness.
Acceptance criteria remain executable and reviewable.

## References

- `_shared/sdd-phase-contract.md` — result envelope and status vocabulary.
- `tasks/coverage`, `tasks/dag`, `tasks/scope`, `tasks/readiness` — executable
  gate identifiers.
- `internal/components/sdd/phasecontract` — canonical contract definitions.
- `internal/components/sdd/contractgen` — generated reference source.
