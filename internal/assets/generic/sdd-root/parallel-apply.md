# Parallel Apply

ForgeSpec readiness and CAS are the sole task authorities. Concurrent dispatch requires fresh readiness, runtime-enforced child execution, isolated worktrees, independent file/dependency scopes, and a bounded concurrency budget.

If any proof is missing or stale, record the named degradation and dispatch one ready task at a time as a sequential fallback. Re-query readiness after each terminal result.

Each child claims one task with expected revision and idempotency, loads the assigned references, follows TDD, runs focused checks, reviews scope and generated files, updates status and evidence, and releases isolation. Shared-worktree writes, competing authorities, and unqualified concurrency are forbidden.
