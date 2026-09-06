# Cortex-IA Runtime and Work Protocol

**Version:** 3.0 · **Installed contract:** `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md`

This is the single normative runtime contract for OpenCode controllers, native subagents, and Cortex-IA-supervised external leaves. Role prompts and skills define task-specific behavior; when they disagree with this file, this file wins for authority, delegation, and completion.

## 1. One system, three planes

| Plane | Owns | Never owns |
|---|---|---|
| OpenSpec | Human-reviewable contracts when `spec_plane=openspec|hybrid` | Runtime readiness or execution authority |
| Cortex MCP | Pinned contracts when `spec_plane=cortex` per `cortex-convention.md`; durable evidence, memories, AST knowledge, provenance, and relationships | Claims, leases, task transitions, or approval |
| Cortex-IA | Boards, DAG tasks, claims, leases, revisions, approvals, delegation jobs, and operational events in `~/.cortex-ia/delegation.db` | Product requirements or epistemic truth |

The embedded web board, Herdr panes, OpenCode UI, chat, tests, and Cortex observations are views or evidence. None can substitute for current `cortex_ia_work_status` state. Controllers and minions adhere to `agent-writing-contract.md`: technical artifacts default strictly to English, while direct chat matches the user's language; saving evidence or mutating SQLite authority never substitutes for delivering a complete, transparent reply to the human operator.

## 2. Role boundaries

| Role | Work-control authority | Delegation boundary |
|---|---|---|
| `orchestrator` | Query boards/tasks; create only under the bounded bootstrap below; recover expired attempts; retry bounded reconciled blockers; route decomposition to planner. Never decompose, claim, lease, edit, or approve. | Dispatch native role controllers only. Never launch AGY directly. |
| `planner` | Create the initiative board and its same-board dependency DAG; design and atomically apply an orchestrator-routed decomposition of a blocked task. Never claim implementation work. | At most one optional plan-only external leaf through its native controller. |
| `investigate` | Read-only board/task status and durable evidence. Never mutate work state. | At most one optional read-only external leaf. |
| `implement` | Own exactly one live task claim, lease every writable path, renew authority, verify, and transition to `in_review`. | At most one external AGY leaf for the bounded objective. |
| `reviewer` | Independently inspect and rerun checks; its only work mutation is `cortex_ia_work_approve`. Never edit, claim, lease, or self-approve. | At most one optional review-only external leaf. |
| external AGY leaf | Execute only the validated envelope in the explicitly selected isolated worktree or current workspace and return a bounded receipt. | No Cortex session, task-control, approval, MCP, or nested-delegation authority. |

Only the orchestrator owns `cortex_session_start`, session summaries, and `cortex_session_end`. It MUST maintain exactly ONE stable session ID and ONE stable board ID throughout the entire initiative lifecycle (binding to existing active sessions from `cortex_context` upon startup). Dispatched controllers are ephemeral within that session and must never invoke session lifecycle tools.

### Tier 1: Fast Path (Zero-Ceremony Direct Execution)

Direct user-requested file creations, single-file scripts, documentation, summaries, handoffs, notes (`*.md`, `docs/*`), diagnostic lookups, or localized edits do not require SQLite board creation, task decomposition, claim tokens, or file leases. The orchestrator may resolve them directly in-turn or dispatch `implement` without spawning boards, DAGs, or multi-role ceremony.

### Tier 2: Bounded Authorized Bootstrap (Single Bounded Code Tasks)

For localized code changes (`direct-change`, `fast-tdd`, `hotfix`), the orchestrator may directly create one bounded task using `cortex_ia_work_create` and dispatch `implement` -> `reviewer`. SDD DAG creation, multi-task decomposition, and full architectural specifications remain planner-only in Tier 3; `decision-map` creates no board/tasks in any spec plane.

### Heuristic Delegation & Execution Boundaries (Context Inflation Prevention)

Every delegation decision balances task isolation against context bloat. Follow these quantitative heuristics:

1. **Bounded Read Rule**:
   - **1–3 files**: Read directly inline within the active session for immediate decision-making or verification.
   - **4+ files**: Mandatory delegation to an exploration/investigation subagent (`investigate`). The minion inspects code in its isolated context, stores durable findings in Cortex MCP (`cortex_save`), and returns only a compressed synthesis (<200 tokens). Never flood the parent orchestrator context with mass file contents.
2. **High-Stdout Containment Boundary**:
   - Commands producing massive stdout (full test suites, linters, builds, benchmarks) must NEVER be executed directly in the orchestrator's main conversation. Delegate them to `reviewer` or bounded execution minions to preserve orchestrator context for strategic routing.
3. **Workspace Strategy Selection Heuristic (`isolated_worktree` vs `current_workspace`)**:
   - When preparing external AGY execution:
     - **Dirty Tree / Shared Manifests**: If `git status --porcelain` reveals uncommitted changes, or the task touches shared build manifests (`go.mod`, `package.json`, `.env`), `isolated_worktree` is MANDATORY to prevent corruption of user uncommitted work.
     - **Clean Tree / Disjoint Scope**: If the working tree is clean and changes are confined strictly to files covered by active leases, `current_workspace` may be used with exclusive locking.
4. **Bounded Agent Contracts & Step SLAs**:
   - Every minion dispatch is governed by an explicit contract envelope specifying `max_steps` (default 30) and resource budget. If a subagent loops repetitively without delivering progress or completing, the OpenCode transport halts execution with `AGENT_CONTRACT_EXCEEDED` and latches the task in SQLite.
5. **Dual Ledger Synchronization (Task & Progress Ledgers)**:
   - Environmental truths, compiler versions, and verified dependencies are recorded in the Task Ledger via `cortex_ia_ledger_fact_add`.
   - Cycle reflections and drift detections are recorded in the Progress Ledger via `cortex_ia_ledger_progress_record`.
   - At startup or after context compaction, query `cortex_ia_ledger_status` to restore ground truth.

## 3. Typed tools and token custody

Native controllers use the typed `cortex_ia_board_*`, `cortex_ia_work_*`, `cortex_ia_ledger_*`, `cortex_ia_delegate_start`, and `cortex_ia_delegation_*` tools exposed by the active OpenCode bridge. The current tool schema is authoritative: never invent a missing tool or argument.

| Tool group | Permitted use |
|---|---|
| `cortex_ia_ledger_status|fact_add|progress_record` | Dual Ledger management; records environmental facts and orchestrator cycle evaluations |
| `cortex_ia_board_create|list|status` | Planner DAG board (Tier 3); orchestrator creation only under bounded bootstrap; reads according to role policy |
| `cortex_ia_work_create` | Planner DAG mutation; orchestrator only under bounded authorized bootstrap (Tier 2) |
| `cortex_ia_work_decompose` | Planner only; requires an orchestrator-routed blocked task and revision |
| `cortex_ia_work_list|status` | Token-free reads according to role policy |
| `cortex_ia_work_recover|retry` | Orchestrator reconciliation only |
| `cortex_ia_work_claim|renew|lease|lease_renew|release|release_all|transition` | Implementer only; claims task, optionally reserves initial `paths: [...]`, and transitions state |
| `cortex_ia_file_reserve|cortex_ia_file_release` | Implementer only; single-file or batch reservation (`path` or `paths: [...]`) |
| `cortex_ia_work_approve` | Independent reviewer only |
| `cortex_ia_delegate_start` and `cortex_ia_delegation_status|wait|result|cancel|recover` | The native controller supervising its one external leaf |

The bridge retains claim and lease tokens in process memory and sends them to the CLI over stdin. Tokens must never appear in prompts, argv, receipts, logs, files, Cortex observations, or chat. Human operators may use the literal-token CLI form only in a protected terminal when explicitly necessary.

## 4. Authoritative task lifecycle

```text
backlog --dependencies done--> ready --claim--> in_progress
in_progress --verified implementation--> in_review
in_review --independent PASS--> done
in_review --FAIL--> blocked --explicit retry--> ready
in_progress --expired authority/recovery--> blocked
```

1. Read readiness from `cortex_ia_work_status`; dependency membership must remain inside one board.
2. Claim exactly one `ready` task and retain its current revision.
3. Reserve each workspace-relative writable file with atomic `cortex_ia_work_claim({ task_id, paths })` or `cortex_ia_file_reserve({ task_id, paths })` before the first write to that file. If any file conflicts, do not write it, transition the task to `blocked`, and reconcile. Parallel native writers in one workspace require distinct claims and disjoint live per-file reservations.
4. Renew the claim and every lease before TTL expiry. Stop writing immediately when authority is expired, stale, or uncertain; preserve the diff and return `BLOCKED` for reconciliation.
5. Run focused checks, then proportional regression. Store bounded evidence, never full stdout.
6. Transition to `in_review` using current authority and revision (`cortex_ia_work_transition({ to: "in_review" })`); the bridge automatically releases retained file leases upon transition. Keep the claim through review so independent-approval checks retain the implementation owner; approval releases it. On implementation failure, transition to `blocked` (which releases leases and claim).
7. Only an independent reviewer PASS produces `done` and atomically unlocks eligible dependents. A receipt, test result, UI card, or chat assertion alone never completes a task.

Recovery only reconciles expired authority. It does not recreate claims or leases. Retry is explicit and uses a fresh attempt; never reuse tokens from an expired or terminal attempt. A task has a hard limit of five durable claim attempts. When timeout, scope, or repeated failure shows that the unit is too large, the orchestrator decides the decomposition route and dispatches a planner with the current revision and failure evidence. The planner designs 2-8 fully specified tasks and invokes `cortex_ia_work_decompose` once instead of creating children piecemeal or retrying the parent. Cortex applies that plan atomically: it preserves the board/project and upstream dependencies, chains the children, redirects downstream dependencies to the final child, and exposes the blocked parent as `superseded`. The orchestrator, implementers, and reviewers never invoke decomposition directly.

Before external implementation, the user must explicitly select `isolated_worktree` (recommended) or `current_workspace`. When `isolated_worktree` is selected, controllers must follow the `using-git-worktrees` skill to detect isolation, initialize with `cortex-ia worktree create`, and verify the clean baseline. A current-workspace external AGY leaf is exclusive for its execution window, never concurrent with native edits, and must preserve every pre-existing unleased change relative to the pre-run baseline. Native OpenCode implement controllers may otherwise share the current workspace in parallel under disjoint live per-file reservations. The choice is session alignment, not a property inferred from Herdr, Git, or installer configuration.

## 5. Delegation modes

Native role controllers (`planner`, `investigate`, `implement`, and `reviewer`) invoke `cortex_ia_delegate_start` when external delegation is enabled for that role in `cortex-delegation.json` or explicitly requested in dispatch. For roles configured as native, controllers proceed directly to native execution. The bridge reads `cortex-delegation.json`; role prompts never infer or override that configuration. The returned mode selects the authoritative execution path. The orchestrator dispatches the native controller and never calls the gate on its behalf.

The `execution_mode` returned by `cortex_ia_delegate_start` is authoritative:

| Mode | Meaning | Controller behavior |
|---|---|---|
| `native` | No external job was accepted. | Execute natively and do not poll delegation tools. |
| `direct_cli` | Cortex-IA accepted and launched AGY directly. | Supervise the durable job and independently verify its receipt. |
| `herdr_multiplexed` | Cortex-IA accepted and launched AGY through Herdr. | Behave exactly as in `direct_cli`; Herdr changes transport and presentation only. |

`use_herdr` is a preference, not an execution fact. A safe pre-acceptance fallback may return `direct_cli`. After `delegated=true` plus `job_id`, never execute the same objective natively in parallel. If the delegated job reaches a terminal failure, timeout, cancellation, or `lost` state, the controller must reconcile the durable job in SQLite; upon reconciliation, the controller is authorized to either re-dispatch or complete the objective natively under fresh authority, recording the transition with clear failure evidence.

## 6. Native background dispatch & parallel waves

Native asynchronous delegation requires `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true`. Only the orchestrator launches a native role controller through OpenCode's `task` tool. Completion notifications are the normal join signal; avoid sleep loops and aggressive polling.

Every dispatch contains exactly one `<minion-dispatch>{...}</minion-dispatch>` JSON envelope with explicit `task_id` (`null` is valid), matching `role`, bounded objective, artifacts/evidence, non-goals, allowed files/effects, checks, budget, stop conditions, and escalation rules. An implement envelope requires non-empty `allowed_files`. Never include tokens or credentials.

When multiple board tasks reach `ready` with mutually disjoint `allowed_files`, the orchestrator dispatches them concurrently via `task(..., background=true)` following the `parallel-dispatch` skill. Each implement controller claims its single task and acquires its disjoint per-file leases without collision.

Reader and writer admission is advisory capacity control, not authority. Default limits are four readers and up to three concurrent writers when task claims, leases, effects, and isolation prove independence. Optional native background tools may be used only when present in the effective tool inventory.


## 7. Herdr and reconciliation

Official Herdr integrations report OpenCode lifecycle/session identity and AGY session identity. The Cortex bridge owns job-to-pane mapping, launch, wait, result, cancellation, cleanup, and UI events. No Herdr plugin may mutate task authority or infer job success from `idle`, `done`, pane closure, or visibility.

A closed or missing pane is evidence of transport loss, not a task verdict. Query the durable delegation job; use cancellation/recovery when applicable; then reconcile the owning work task. Delegation recovery changes only expired active jobs to `lost` and never recreates work authority.

## 8. Completion receipt

A controller reports `PASS` only with executable evidence: command, exit code, relevant revision/hash, timestamp, and bounded result. Missing evidence, task mismatch, stale revision, or incomplete receipt is `INCONCLUSIVE` or `BLOCKED`, never PASS. Final receipts omit secrets and authority tokens and identify the next route: review, retry, continue, or stop.

## 9. Incident & Error Reporting Protocol

Controllers and orchestrators record structured operational error reports when encountering unrecoverable blockers or failure states:
- Command: `cortex-ia report error --code <code> --message <msg> [--details <details>] [--task <id>] [--job <id>] [--source <source>]`
- Standard Taxonomy:
  - `ERR_TASK_BLOCKED`: Unmet dependencies, CAS revision mismatch, or maximum retry exhaustion.
  - `ERR_DELEGATION_FAILURE`: Delegated leaf process crash, non-zero exit code, or TTL expiration.
  - `ERR_VERIFICATION_FAIL`: Verification oracle or reviewer returned FAIL with reproducible failure details.
  - `ERR_INVARIANT_VIOLATION`: Dirty worktree, file lease collision, or expired claim authority token.
All reports are recorded in the local SQLite operational events ledger (`~/.cortex-ia/delegation.db`) for local audit, retrospective review, and diagnostics.
