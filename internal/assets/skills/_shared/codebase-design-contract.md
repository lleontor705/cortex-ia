# Codebase Design Contract

**Installed contract:** `~/.cortex-ia/opencode/contracts/codebase-design-contract.md`

A shared vocabulary for architecture and task boundaries, used by discovery, investigate, planner, implement and reviewer. It does not give any role authority beyond its own: the orchestrator routes, the planner designs and decomposes, implementers edit one claimed task, and reviewers audit independently.

## Shared vocabulary

- **Module:** a cohesive unit that hides decisions behind a bounded surface.
- **Interface:** the smallest caller-visible contract. It is also the preferred test surface.
- **Implementation:** the internal choices hidden behind the interface.
- **Depth:** useful behavior divided by interface complexity. Prefer deep modules. Flag shallow pass-through layers that add ceremony without hiding complexity.
- **Locality:** code that changes together stays easy to find and reason about together.
- **Seam:** a deliberate substitution point at a real source of variation, ownership, process or protocol.
- **Adapter:** a translation at a seam. One implementation alone rarely justifies an adapter. A second real implementation, a remote protocol or a required deterministic substitute makes the seam real.

Use the deletion test as evidence: if deleting a wrapper leaves callers nearly unchanged, the wrapper is shallow. This heuristic does not override the repository's declared architecture.

## Dependency classification

| Category | Default treatment |
|---|---|
| In-process deterministic code | Use the real implementation in focused tests |
| Local substitutable resource (filesystem, clock, process) | Substitute at the narrow acquisition point when determinism needs it |
| Remote but project-owned service | Define a port at the protocol boundary; keep transport translation in an adapter |
| True external service | Isolate the protocol boundary; use a contract-faithful fake only there |

Replace, do not layer: a test substitute replaces the real boundary and does not wrap it, so the real side effects do not leak through. Avoid interfaces created only to mock internal code.

## Architecture decisions

Use Design It Twice only when the orchestrator identifies a named architectural or public-interface decision with real ambiguity. The planner produces two or three contract-level alternatives without product-code edits. One should favor a minimal interface and another the most important known variation or caller. Compare them on:

1. interface size and depth;
2. caller locality and cognitive load;
3. dependency direction and seam placement;
4. blast radius, reversibility and operational risk;
5. fit with the confirmed project architecture.

The planner records the chosen design and why. Implementers receive only the selected contract. Do not launch competing implementations to explore architecture.

## Task graph

Each planned task states its objective, exact writable files, interface contract, observable acceptance, exact verification command, dependencies, and out-of-scope work. Dependencies follow executable prerequisites, not presentation order.

For user-visible behavior, prefer tracer-bullet vertical slices: a narrow, complete path that can be verified on its own. Use horizontal foundation tasks only for real shared prerequisites. For a wide mechanical refactor that cannot land green slice by slice, use `expand -> disjoint migrate batches -> contract`, where removal depends on every migration.

Choose independent, sequential, diamond, fork-join or pipeline shapes from real dependency direction. Keep chains short and name the critical path. Parallel writers need distinct ready claims and disjoint file leases. Board position never proves readiness or ownership.

When a task is blocked because its scope is too broad, the orchestrator decides to route decomposition and supplies the evidence. The planner alone designs and applies the smaller replacement DAG. Implementers never split their own task.

## Role application

- **Discovery:** record observed modules, interfaces, dependency categories, seams, adapters and risks with evidence. Do not redesign.
- **Investigate:** assess depth, locality, dependency direction and seam quality. Compare alternatives only when a real choice exists.
- **Planner:** owns alternatives, selected design contracts, critical-path DAGs and blocked-task decomposition.
- **Orchestrator:** decides when investigation, design comparison or decomposition is needed. Dispatches the responsible role and watches the authoritative DAG state.
- **Implement:** keep the selected interface and dependency direction. Add no speculative abstraction.
- **Reviewer:** detects architectural regression: widened interfaces, shallow wrappers, misplaced seams, new cycles, tests coupled to implementation details.

## Invariants

- Code-intelligence structures (symbols, doc summaries, graph relations, call reasoning) are codebase structure, not user data. Privacy and sanitization steps never redact or alter them. See `cortex-work-protocol.md` §4.
- Test fixtures stay bounded and modular under the workload budget in `cortex-work-protocol.md` §4. Follow the target repository's own test policy from its root `AGENTS.md` or `./.cortex-ia/discovery.md`. If neither declares one, add tests proportional to the change. Tests never weaken a contract (for example by silently skipping invalid records) and never touch real developer state.
