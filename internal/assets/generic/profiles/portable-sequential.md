# Profile Overlay: portable-sequential

Applies when direct-child and worktree isolation are unproved or unavailable.

## Behavior

- No direct children. All work runs in the current context.
- Sequential readiness loop: query ForgeSpec readiness, claim one ready task, execute, await terminal state, re-query.
- No parallel dispatch. Ever.

## Degradation

When a capability is unproved:
1. Record the degradation explicitly (which capability, why).
2. Proceed sequentially.
3. Never assume the capability is available.

## Apply

One ready task at a time. No worktree isolation needed because there is no parallelism. Each task still CAS-claims via ForgeSpec before mutation.

## Recovery

After compaction or restart: reconcile ForgeSpec state and Cortex evidence, then resume from the last incomplete task sequentially.
