# SDD coordination and authority

← [Codebase Guide](../CODEBASE-GUIDE.md)

Use the [canonical workflow map](../../internal/assets/skills/_shared/workflow-map.md) for routing and phase ownership, the [runtime protocol](../../internal/assets/skills/_shared/cortex-work-protocol.md) for claims/leases/review, and the [Cortex convention](../../internal/assets/skills/_shared/cortex-convention.md) for selected-plane evidence. These shared contracts install as `~/.cortex-ia/opencode/contracts/` and are the single normative source; a skill is a procedure used by a role, not a separate agent per phase.

## Work lifecycle

`backlog -> ready -> in_progress -> in_review -> done`

Only independent approval PASS produces `done`. Failure enters `blocked`; recovery reconciles expired authority, and retry requires fresh authority. Decomposition preserves contract bindings and replaces an oversized task with approved-scope dependencies. UI placement never authorizes work.

## Routing and role ownership

| Workflow | Route | Exit gate |
|---|---|---|
| `direct-answer` / `direct-doc` | Orchestrator answers from evidence | Filesystem mutations route to a bounded task |
| `discovery` | Native discovery (mandatory first task of every session) | Quick agentic-environment index |
| `direct-change` | Orchestrator creates one `default` task; implement; reviewer only if risk warrants | Live claim/leases, proportional checks |
| `fast-tdd` | Implement with fast-tdd, then reviewer | Causal RED, same oracle GREEN, independent approval |
| `hotfix` | Implement with hotfix-triage, then reviewer | Containment, regression evidence, structural follow-up |
| `sdd-lite` | Investigate; planner integrated; implement; reviewer; planner archive | Integrated contract, typed SDD task bindings, independent approval |
| `sdd-full` | Investigate; planner propose/spec/design/tasks; implement; reviewer; planner archive | Phase contracts, typed SDD task bindings, durable closure |
| `review` | Independent reviewer | Only work approval PASS produces `done` |
| `retrospective` | Investigate with workflow-retrospective | Diagnosis and recommendations; no state mutation |

Only the planner materializes the SDD DAG, after integrated/tasks validation. Decision-map creates no board; one stable board groups a materialized initiative.

| Role | SDD responsibility |
|---|---|
| `orchestrator` | Selects routes across the 3-tier model, dispatches controllers, reconciles failures, and delivers the result. It does not claim, implement, or approve. |
| `discovery` | Maintains the agentic-environment profile. Strictly native and read-only with respect to work state. |
| `investigate` | Read-only diagnosis, red-capable reproduction, and root-cause analysis that feeds planning. |
| `planner` | Selected-plane contracts, validated artifacts, the typed task DAG, and durable closure. Never claims implementation work. |
| `implement` | Claims one ready task, reserves every writable path, renews authority, verifies, and transitions to review. |
| `reviewer` | Independently inspects the exact contract and diff, reruns checks, and approves or rejects with bounded evidence. The implementation owner's identity cannot serve as reviewer. |

## SDD definitions

Use `cortex_ia_work_create` with `sdd_contract` containing version 1, workflow, change_id, spec_plane, pins and requirement_ids. Each pin identifies transport, project, locator and exact SHA-256. CLI automation can pass the same bounded JSON with `work create --contract-file <file>`. Direct/legacy definitions may omit this field; SDD controllers must not omit it to bypass verification.

Review fingerprints are computed from the current definition and sorted allowed paths, including deletion markers and file bytes. Approval compares them with current state and retains them in historical approval metadata. A changed file or definition requires a fresh review. Provider-side changes require explicit retrieval of Cortex pins through the selected transport.

## Structural validation

For OpenSpec/hybrid use `cortex_ia_openspec_validate` with explicit `relative_directory`, `workflow`, and `phase`. Lite validates one `plan.md`; Full validates only the artifacts due at that phase; decision-map uses `decision-map.md`. The JSON result is structural evidence, not a semantic PASS. Requirements use `REQ-{DOMAIN}-{NNN}` headings with at least three nonempty GIVEN/WHEN/THEN scenario blocks for ADDED/MODIFIED entries; task blocks declare unique IDs and an explicit `Requirements:` list of existing requirement IDs. Counting blocks cannot prove coverage — reviewers judge happy, edge, and failure paths independently.

## Typed controller operations

- Planner: `cortex_ia_openspec_validate({relative_directory,workflow,phase})` for OpenSpec/hybrid structural checks, `cortex_ia_work_create` for bound tasks, and `cortex_ia_change_archive` for closure.
- Implement: claim one ready task, reserve every writable path, renew authority, verify and transition to review via `cortex_ia_work_transition` with typed parameters (`summary`, `verdict`, `evidence_refs`, `changed_files`). Raw JSON text blocks in chat are strictly forbidden.
- Reviewer: inspect the exact contracts and diff, run relevant checks, and approve using current revision and bounded evidence via `cortex_ia_work_approve` with typed parameters (`verdict`, `reason`, `summary`, `findings`). The implementation owner's identity cannot serve as reviewer.
- Discovery: inspect project skills, stack, engines, and architecture; maintain `.cortex-ia/discovery.md` via `cortex_ia_discovery_write`. Strictly native and read-only.
- Orchestrator: select routes across the 3-tier model, dispatch controllers, reconcile failures and deliver the result. It does not claim, implement or approve.

## Closure and recovery

After required approvals, the planner calls `cortex_ia_change_archive({board_id, change_id, workflow, spec_plane})`. Closure is fail-closed on pending or unbound work, changed fingerprints, or conflicting archive state. Existing historical approvals are preserved and never rewritten to make closure pass; Cortex-only closure records a logical receipt. If later authorized tasks modify files covered by an earlier approval, the orchestrator explicitly requests `cortex_ia_work_review_refresh({task_id, revision})` after active work is reconciled, and a fresh independent verdict is required.

Orchestrator dispatches native role controllers; every role executes natively and no external leaf participates in work authority. Reviewer failure returns work to `blocked`; expiry requires recovery; excessive scope routes to planner decomposition; repeated causes route to retrospective. Herdr, the TUI, and the web are views, not task authority. The bridge verifies host roles; the store verifies transactional authority. Neither can infer semantic correctness from an exit code or the presence of an evidence string.

---

## See Also

- [`sdd-workflow.md`](../sdd-workflow.md) — SDD lifecycle and layered verification
- [`agents.md`](../agents.md) — Role topology and typed receipt contracts
- [`architecture.md`](../architecture.md) — `internal/delegation` engine internals
- [workflow-map.md](../../internal/assets/skills/_shared/workflow-map.md) — canonical routing matrix
