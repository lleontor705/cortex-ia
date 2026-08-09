# Profile Overlay: portable-flat

Applies when direct children are available for read/planning but worktree isolation is unproved for writes.

## Read and Planning

Direct children MAY be used for read-only investigation and planning phases (bootstrap, investigate, propose, spec, design, tasks).

## Parallel Apply

Parallel apply is permitted ONLY when ALL are proven with fresh evidence:
1. Fresh ForgeSpec readiness and CAS.
2. Fresh runtime-enforced direct-child capability.
3. Fresh runtime-enforced worktree isolation.
4. Independent dependency and file scopes.

Without worktree proof: degrade to sequential. Record the degradation explicitly.

## Degradation

Missing/stale capability: one ready task at a time, sequential readiness loop, re-query after each terminal state.

## Prohibitions

Shared-worktree parallel writes are forbidden. Each parallel child must create and verify its own worktree before mutation.
