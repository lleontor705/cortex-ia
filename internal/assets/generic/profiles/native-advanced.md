# Profile Overlay: native-advanced

Applies when the runtime provides qualified native hooks, skills, models, and isolation.

## Qualified Capabilities

Each capability MUST be proven with fresh enforced evidence before use:
- Native skill preload: verified or fall back to mandatory first-action read.
- Native model field: verified adapter supports it or omit.
- Worktree isolation: verified or degrade to sequential.
- Direct-child dispatch: verified or degrade to sequential.

## Never Assume

Optional capabilities are never assumed. If evidence is stale or absent:
1. Record the degradation explicitly.
2. Fall back to the safe path defined by portable-flat or portable-sequential.
3. Never silently use an unqualified capability.

## Parallel Apply

Qualified parallel apply follows the same ForgeSpec readiness/CAS/direct-child/worktree requirements as portable-flat. The native-advanced overlay does not waive any safety gate.
