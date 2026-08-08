# SDD Orchestrator Reference

Load this reference only when the current phase needs operational detail. The root orchestrator prompt remains a bounded routing map.

## Apply Phase

1. Read `board_id` from the validated decompose contract.
2. Query ForgeSpec readiness; never infer readiness from prompt text or runtime-local state.
3. Claim and route one ready task directly to `role/implement` with its bounded work reference.
4. Portable sequential executes one work unit at a time without delegation. Portable flat may use qualified runtime-native direct-child dispatch for independent ready tasks.
5. Validate returned apply contracts, persist durable evidence in Cortex, update authoritative status in ForgeSpec, then query readiness again.
6. On failure, preserve returned evidence and terminal status. Retry only when ForgeSpec records the work as retryable.

## Capability Boundaries

- ForgeSpec owns SDD contract revisions, task dependencies, readiness, claims, attempts, status, audit events, and file reservations when negotiated.
- Cortex owns durable evidence payloads, provenance, and relationships.
- Runtime-native dispatch owns child execution transport only.
- Missing or stale required capability evidence blocks the affected operation. Optional capabilities degrade only through a declared safe substitution.
- Current profiles never infer guarantees from package names, documentation, tool presence, or runtime-local state.

## File Coordination

Use ForgeSpec file reservations only when the negotiated capability supports the required enforcement. Without qualified file reservations, serialize overlapping writes under one owner. Deployment, CI, external APIs, and databases remain outside this workflow compiler's coordination authority.

## Recovery

Recover task state from ForgeSpec and durable evidence from Cortex. Do not reconstruct completion from runtime transport state. After context loss, reload the validated phase contract, task reference, and most recent Cortex checkpoint before continuing.

## Tool Categories

- Progress: `tb_status`, `tb_unblocked`, `tb_claim`, `tb_update`, `tb_get`, `tb_list_boards`
- Contracts: `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history`
- File reservations: `file_reserve`, `file_release`
- Memory: `cortex_save`, `cortex_search`, `cortex_get_observation`, `cortex_context`, `cortex_session_summary`, `cortex_relate`
