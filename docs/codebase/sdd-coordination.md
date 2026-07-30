# Direct SDD Coordination

1. ForgeSpec returns authoritative dependency readiness.
2. The orchestrator selects one bounded ready work reference.
3. Runtime-native dispatch invokes `implement` directly.
4. `implement` reserves exact files only when ForgeSpec advertises qualified reservation support; otherwise work remains sequential.
5. `implement` records evidence and updates ForgeSpec using its claim/CAS contract.
6. `validate` independently checks behavior and evidence.

The historical `team-lead` role is retired and absent from every current profile. There is no local scheduler, message bus, inbox, dead-letter queue, heartbeat engine, or second task authority.

ForgeSpec `direct-v1` requires fresh P0 evidence. ForgeSpec 1.2.x may run only as visible `legacy-sequential`. Provider-neutral remote A2A is unsupported and unbound.

The historical Agent Mailbox database, WAL/SHM files, caches, archives, and repository checkout are never automatically mutated or deleted. Cleanup is operator-controlled after preservation checks.
