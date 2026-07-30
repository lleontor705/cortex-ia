# Components

cortex-ia currently configures Cortex, ForgeSpec, Context7, conventions, direct SDD workflow assets, and skills. Component and tool counts are derived from the catalog and negotiated service schemas rather than duplicated in documentation.

## Current service boundaries

| Service | Authority |
|---|---|
| ForgeSpec | Versioned SDD contracts, task DAG/readiness/claim/status, and file reservations when negotiated |
| Cortex | Durable evidence, provenance, memory, and relationships |
| Runtime-native dispatch | Bounded child execution transport only |
| cortex-ia | Compile, diagnose, install, back up, receipt, and restore generated assets |

ForgeSpec `direct-v1` requires fresh compatible P0 evidence. ForgeSpec 1.2.x may run only as visible `legacy-sequential`. Optional P1 omissions are disclosed and may force sequential/no-concurrent-write execution.

## Retired compatibility

The historical Agent Mailbox built-in is retired. Its identifier remains only for bounded legacy decode, exact owned-registration migration, rollback, and operator cleanup guidance. Provider-neutral remote A2A is unsupported and unbound.

External Mailbox database, WAL/SHM files, caches, archives, and repository checkouts are never automatically mutated or deleted. Cleanup is operator-controlled after preservation checks.
