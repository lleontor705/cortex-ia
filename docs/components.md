# Components

cortex-ia exposes seven current components: Cortex, Agent Mailbox, ForgeSpec, Context7, conventions, direct SDD workflow assets, and extra skills. Component and tool counts are derived from the catalog and negotiated service schemas rather than duplicated in documentation.

## Current service boundaries

| Service | Authority |
|---|---|
| ForgeSpec | Versioned SDD contracts, task DAG/readiness/claim/status, and file reservations when negotiated |
| Cortex | Durable evidence, provenance, memory, and relationships |
| Agent Mailbox | Optional messaging, A2A transport, resource coordination, and dead-letter handling; never SDD task authority |
| Runtime-native dispatch | Bounded child execution transport only |
| cortex-ia | Compile, diagnose, install, back up, receipt, and restore generated assets |

ForgeSpec `direct-v1` requires fresh compatible P0 evidence. ForgeSpec 1.2.x may run only as visible `legacy-sequential`. Optional P1 omissions are disclosed and may force sequential/no-concurrent-write execution.

## Presets and dependencies

The full preset contains all seven components. The minimal preset selects Cortex, ForgeSpec, Context7, and SDD; dependency resolution also includes Agent Mailbox because SDD uses its optional coordination transport. Conventions depend on Cortex.

Installation updates only managed configuration and assets. It never deletes user Mailbox data, WAL/SHM files, caches, archives, or repository checkouts.
