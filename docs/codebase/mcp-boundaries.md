# Service Boundaries

## ForgeSpec

ForgeSpec owns versioned SDD contracts and authoritative task dependency, readiness, claim, status, attempt, and audit state. `direct-v1` is capability-qualified; ForgeSpec 1.2.x is `legacy-sequential`. Missing required P0 evidence blocks. Optional P1 file reservations may degrade to sequential/no-concurrent-write execution.

## Cortex

Cortex owns durable evidence payloads, provenance, memory, and relationships. References between ForgeSpec and Cortex are stable IDs, not duplicated mutable state.

## Runtime transport

The orchestrator dispatches one ForgeSpec-ready reference directly to `implement` through a runtime-native primitive. Transport state is never task authority.

## Retired provider boundary

The historical Agent Mailbox provider and its tools are retired and removed from current configuration, permissions, prompts, and counts. Provider-neutral remote A2A is unsupported and unbound. External Mailbox data and repositories are never automatically changed; operator-controlled cleanup requires preservation checks.
