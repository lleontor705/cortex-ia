# SDD coordination and authority

Use the [canonical workflow map](../../internal/assets/skills/_shared/workflow-map.md) for routing and phase ownership, the [runtime protocol](../../internal/assets/skills/_shared/cortex-work-protocol.md) for claims/leases/review, and the [Cortex convention](../../internal/assets/skills/_shared/cortex-convention.md) for selected-plane evidence.

## Work lifecycle

`backlog -> ready -> in_progress -> in_review -> done`

Only independent approval PASS produces `done`. Failure enters `blocked`; recovery reconciles expired authority, and retry requires fresh authority. Decomposition preserves contract bindings and replaces an oversized task with approved-scope dependencies. UI placement never authorizes work.

## SDD definitions

Use `cortex_ia_work_create` with `sdd_contract` containing version 1, workflow, change_id, spec_plane, pins and requirement_ids. Each pin identifies transport, project, locator and exact SHA-256. CLI automation can pass the same bounded JSON with `work create --contract-file <file>`. Direct/legacy definitions may omit this field; SDD controllers must not omit it to bypass verification.

Review fingerprints are computed from the current definition and sorted allowed paths, including deletion markers and file bytes. Approval compares them with current state and retains them in historical approval metadata. A changed file or definition requires a fresh review. Provider-side changes require explicit retrieval of Cortex pins through the selected transport.

## Typed controller operations

- Planner: `cortex_ia_openspec_validate({relative_directory,workflow,phase})` for OpenSpec/hybrid structural checks, `cortex_ia_work_create` for bound tasks, and `cortex_ia_change_archive` for closure.
- Implement: claim one ready task, reserve every writable path, renew authority, verify and transition to review. Use typed tools so tokens remain in controller memory and stdin rather than prompts or command arguments.
- Reviewer: inspect the exact contracts and diff, run relevant checks, and approve using current revision and bounded evidence. The implementation owner's identity cannot serve as reviewer.
- Orchestrator: select routes, dispatch controllers, reconcile failures and deliver the result. It does not claim, implement or approve.

The bridge verifies host roles; the store verifies transactional authority. Neither can infer semantic correctness from an exit code or the presence of an evidence string.
