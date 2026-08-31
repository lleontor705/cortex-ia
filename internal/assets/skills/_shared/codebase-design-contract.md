# Codebase Design Contract

**Installed contract:** `~/.cortex-ia/opencode/contracts/codebase-design-contract.md`

This contract gives discovery, investigation, planning, implementation, and review one precise vocabulary for architecture and task boundaries. It does not authorize a role to cross its control-plane boundary: the orchestrator routes, the planner designs and decomposes, implementers edit one claimed task, and reviewers audit independently.

## Shared vocabulary

- **Module:** a cohesive unit that hides decisions behind a bounded surface.
- **Interface:** the smallest caller-visible contract; it is also the preferred test surface.
- **Implementation:** internal choices hidden behind the interface.
- **Depth:** useful behavior divided by interface complexity. Prefer deep modules that provide substantial behavior through a small contract; flag shallow pass-through layers that add ceremony without hiding complexity.
- **Locality:** code that changes together should remain easy to find and reason about together.
- **Seam:** a deliberate substitution boundary at a real source of variation, ownership, process, or protocol.
- **Adapter:** a translation at a seam. One implementation alone normally does not justify a speculative adapter; a second real implementation, remote protocol, or required deterministic substitute makes the seam concrete.

Use the **deletion test** as evidence: if deleting a wrapper leaves callers almost unchanged, the wrapper is probably shallow. Do not treat this heuristic as permission to bypass a repository's declared architecture.

## Dependency classification

Classify a dependency before proposing a seam:

| Category | Default treatment |
|---|---|
| In-process deterministic code | Use the real implementation in focused tests. |
| Local substitutable resource such as filesystem, clock, or process boundary | Substitute at the narrow acquisition boundary when determinism requires it. |
| Remote but project-owned service | Define a port at the protocol boundary and keep transport translation in an adapter. |
| True external service | Isolate the protocol boundary and use a contract-faithful fake or mock only there. |

Prefer **replace, do not layer**: a test substitute replaces the real boundary; it must not wrap the real dependency and accidentally retain its side effects. Avoid interfaces created only to mock internal code.

## Architecture decision protocol

Use Design It Twice only when the orchestrator identifies a named architectural or public-interface decision with material ambiguity. The planner produces two or three contract-level alternatives without product-code edits. At least one alternative should optimize for a minimal interface and another for the most important known variation or caller. Compare them by:

1. interface size and depth;
2. caller locality and cognitive load;
3. dependency direction and seam placement;
4. blast radius, reversibility, and operational risk;
5. fit with confirmed project architecture and constraints.

The planner selects or recommends one design and records why. Implementers receive only the selected contract. Do not launch competing implementation tasks merely to explore architecture.

## Task graph contract

Each planned task states its objective, exact writable files, requirements or interface contract, observable acceptance, exact verification command, dependencies, and explicit out-of-scope work. Dependencies must reflect executable prerequisites rather than presentation order.

For user-observable behavior, default to tracer-bullet vertical slices that deliver a narrow complete path and can be verified independently. Use horizontal foundation tasks only for genuine shared prerequisites. For a wide mechanical or contract refactor that cannot land green slice by slice, use `expand -> disjoint migrate batches -> contract`; removal depends on every migration.

Use independent, sequential, diamond, fork-join, or pipeline shapes only when supported by real dependency direction. Minimize unnecessary chain depth and identify the critical path. Parallel writers require distinct ready task claims and disjoint per-file leases; apparent board position never proves readiness or ownership.

When a task is blocked because its scope is too broad, the orchestrator decides to route decomposition and supplies the evidence. The planner alone designs and atomically applies the smaller replacement DAG. The orchestrator never performs the split and an implementer never self-expands its task.

## Role application

- **Discovery:** record observed modules, interfaces, dependency categories, seams, adapters, and risks with evidence; never redesign.
- **Investigate:** assess depth, locality, dependency direction, and seam quality; compare alternatives only when a real choice exists.
- **Planner:** own architecture alternatives, selected design contracts, critical-path DAG construction, and blocked-task decomposition.
- **Orchestrator:** decide when investigation, design comparison, or decomposition is needed; dispatch the responsible role and monitor authoritative DAG state.
- **Implement:** preserve the selected interface and dependency direction; do not add speculative abstraction or unapproved architecture variants.
- **Reviewer:** detect architectural regression, including widened interfaces, shallow wrappers, misplaced seams, new cycles, and tests coupled to implementation details.
