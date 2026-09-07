
---

# Addendum: proposed P0 retirement of worktree product support

**Document cut:** 2026-09-07. This English addendum is the current planning export requested by the user. It preserves the Spanish historical report above, including its record of prior isolated-worktree activity; it does **not** rewrite history, change live task state, or implement this proposal. As always, the live board, task status, active claim, and bridge-held lease are authoritative.

## Purpose, decision boundary, and current evidence

The proposed P0 direction is to retire Cortex-IA *product support* for isolated worktrees and make `current_workspace` the sole supported implementation workspace strategy. This is a safety and maintenance proposal, not an instruction to delete any existing worktree or historical record.

| Evidence / item | Observed state | Planning implication |
|---|---|---|
| Historical worktree hardening in the report above | Preserved history; not current product approval | Keep it readable and do not claim it was erased or superseded by implementation. |
| `oqh-10a-canonical-protocol-doc` | done / 4 (observed evidence) | Documentation-only tranche completed in its separate isolated worktree. |
| `oqh-10b-align-dispatch-examples` | done / 12 (observed evidence) | Documentation-only tranche completed, uncommitted and uninstalled; it does not implement runtime retirement. |
| Contract snapshot `cortex://observations/342` | exact content pin `184d394e4dd2fc8c3a0b79c29dc5b206482ca9376e08181ed18a3ea12903f64b` | Defines the remaining OQH 11--20 requirements and their bounded scope. |
| `oqh-11` through `oqh-20` | backlog/new contract work needing authoritative readiness reconciliation | Do not mark any as done or dispatch them from this document. Reconcile through supported claim/readiness operations only. |
| Roadmap observation 336 | not found | It is not cited as an existing source or used as evidence. |

No product code, task definition, SQLite row, configuration, installation, synchronization, deployment, package installation, or worktree is changed by this document. The previously authorized future dependency action is limited to web-local lockfile-based installation; it does not authorize global installation, install/sync/deploy, or any action now.

## Proposed target flow and preserved safety invariants

### Current historical flow (for context only)

`request -> isolated_worktree selection/path validation -> implementation worktree -> claim/lease/allowlist checks -> review`

The historical flow is retained above because it explains prior tasks, failures, and evidence. It remains historical even after a future retirement release.

### Proposed supported flow

`request -> explicit current_workspace -> exclusive current-workspace protection -> claim/lease/allowlist/TTL/CAS checks -> bounded implementation -> independent review`

`current_workspace` protections are unchanged and non-negotiable: a live claim is required; writes require a bridge-held, path-normalized lease; edits must remain inside the allowlist; lease and claim TTLs must be live; compare-and-swap revision checks prevent stale transitions; and independent review remains separate from implementation authority. Presentation, prompts, and documentation must never infer authority in place of these checks.

### Proposed rejection flow for legacy requests

`legacy isolated_worktree request -> explicit validation failure -> actionable error -> no claim, workspace creation, remapping, or file mutation`

An old request must fail closed. It must say that `isolated_worktree` is retired and that the caller must submit an explicitly authorized `current_workspace` request; it must not silently reinterpret, downgrade, or redirect the request. A rejected request must leave existing worktrees, their files, task history, receipts, and historical references untouched.

## P0 requirements and acceptance scenarios

### REQ-WORKTREE-RETIRE-001 — Single supported workspace strategy

All supported product entry points accept `current_workspace` only. Removal covers discovered inventory across CLI, TUI, config, bridge, plugins, tools, skills, agents, web, documentation, and install mapping. Inventory must be grounded before edits: enumerate actual registrations, flags, schemas, UI controls, templates, generated assets, help text, and install/copy mappings rather than assuming a surface exists.

- **Given** a valid request explicitly selecting `current_workspace`, **when** its normal authority checks pass, **then** the existing current-workspace path proceeds without weakened claim, lease, allowlist, TTL, CAS, or review protection.
- **Given** a request selecting `isolated_worktree`, **when** it reaches any supported entry point, **then** it fails before success/claim/worktree/file mutation with an actionable retirement error.
- **Given** an unknown or malformed workspace strategy, **when** validation runs, **then** it fails closed rather than falling back to a default.

### REQ-WORKTREE-RETIRE-002 — Surface-complete, truthful migration

The final inventory records each actual surface and its disposition: remove, replace with `current_workspace`, retain as historical/read-only compatibility data, or record as absent. Product-facing text must not advertise creation, selection, routing, or support of isolated worktrees. Compatibility records may be displayed only as history and must be clearly non-actionable.

- **Given** CLI/TUI/config/bridge/plugin/tool/skill/agent/web/install inputs and help, **when** an operator searches for the supported strategy, **then** only `current_workspace` is offered for new work.
- **Given** historical receipts or records that contain isolated-worktree values, **when** they are viewed, **then** they remain readable without activating, deleting, or mutating them.
- **Given** a generated web or installed asset is in the discovered mapping, **when** its source changes, **then** the applicable generated/copy output is updated and verified; nonexistent literal outputs are not fabricated.

### REQ-WORKTREE-RETIRE-003 — Non-destructive retirement and rollback

Retirement removes product support only. It must not automatically run `git worktree remove`, delete directories or files, reset or clean repositories, delete history, perform direct SQL migrations, install, sync, publish, deploy, or modify user configuration. Rollback is a narrowly reviewed product revert that restores the prior support code/documentation only; it never reconstructs or deletes user worktrees or history.

- **Given** existing worktrees and historical records, **when** retirement is delivered, **then** none are removed, reset, cleaned, or rewritten.
- **Given** a failed verification or hard blocker, **when** the tranche cannot safely proceed, **then** it stops with evidence and a bounded blocker report; it does not bypass checks or silently ship partial retirement.

## Finite dependency-aware execution plan

Completion means the entire accepted retirement scope has passed its required verification and independent review; it is not a per-tranche pause or an assertion that a partial surface removal is complete. Each phase may proceed only after its dependencies and live authority conditions are satisfied.

| Priority / phase | Dependency and bounded work | Exit evidence |
|---|---|---|
| **P0.1: reconcile and inventory** | Re-read live board/task readiness, reconcile OQH 11--20 through supported operations, and produce a grounded cross-surface inventory with owners and generated-asset mappings. No task is inferred ready from this report. | Live statuses/revisions recorded; every actual surface dispositioned; missing/blocked surfaces explicitly listed. |
| **P0.2: contract and validation** | Specify a single validation seam for the strategy, exact legacy rejection text/behavior, compatibility handling, and deterministic positive/negative cases. Keep `current_workspace` authority protections unchanged. | Deterministic cases prove valid current-workspace behavior, old isolated rejection, malformed rejection, and no mutation on failures. |
| **P0.3: entry points and user surfaces** | In dependency order, remove/replace actual CLI, TUI, config, bridge, plugin, tool, skill, agent, web, documentation, and install mappings identified in P0.1. Generated outputs are handled only where sources and authorized build prerequisites exist. | Allowlisted diffs and surface inventory show no supported isolated-worktree product path. |
| **P0.4: integration, review, closure** | Run targeted and full applicable oracles, verify no destructive side effects, inspect generated/copy mappings, and obtain independent review for each required lifecycle transition. | PASS evidence, current revisions, reviewer approval, and a closure report that distinguishes completed work from any hard blocker. |

Hard blockers are handled finitely: record the exact unavailable prerequisite, affected phase, safe non-action taken, and required authorized recovery; do not create fake fast-path tasks, retry a live writer, mutate SQLite directly, or keep looping. For example, absent `web/node_modules` blocks a web build until an explicitly authorized **web-local lockfile** installation is performed; it does not authorize global installation or a substitute build claim.

## Verification matrix and non-goals

| Oracle | Required result |
|---|---|
| Inventory oracle | Grounded search/registration inventory covers every listed surface or records it absent, with no assumed mappings. |
| Strategy oracle | Valid `current_workspace` preserves all authority checks; `isolated_worktree`, unknown, and malformed strategies fail closed before mutation. |
| Safety oracle | Existing worktrees/files/history remain present and unchanged; no direct SQL, reset, clean, deletion, install, sync, publish, or deploy occurred. |
| Build/asset oracle | Only applicable authorized builds/copies run; web changes include real generated `static/**` output when prerequisites exist, never guessed filenames. |
| Scope oracle | `git diff --check` passes and each task changes only its allowlisted files. |
| Review oracle | Independent reviewer verifies the current revision, negative cases, authority invariants, and evidence before approval. |

**Non-goals:** This plan does not implement retirement; promise deletion of existing worktrees; retroactively alter historical claims or receipts; silently remap legacy requests; add remote APIs; use direct SQL migration; or authorize dependency, global, installation, synchronization, deployment, publication, or user-config changes. It also does not mark OQH 11--20 complete: their current state and dependencies must be reconciled from the live task system.

## Traceability

The remaining OQH contract pin (`cortex://observations/342`) maps `oqh-11`/`oqh-12` to traceability and prompt-reference work, `oqh-13`/`oqh-14` to dispatch/receipt compatibility, `oqh-15` to canonical tool clarity, `oqh-16`--`oqh-18` to observational web authority/freshness, `oqh-19` to accessibility, and `oqh-20` to generated asset/install-sync verification. This retirement proposal must be decomposed only after the P0.1 inventory and live readiness reconciliation establishes which existing contract tasks can safely host a change and which require an explicitly approved follow-up. It must not claim that the missing roadmap observation 336 supplied that mapping.
