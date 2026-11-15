# Cortex-IA Work Protocol

**Installed contract:** `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md`

This is the normative contract for native controllers (orchestrator, discovery, investigate, planner, implement, reviewer) on every host. Role files and skills define task-specific behavior. On authority, delegation and completion, this file wins.

## 1. Authority and evidence

| Plane | Owns | Does not own |
|---|---|---|
| Cortex-IA (`~/.cortex-ia/delegation.db`) | Boards, tasks, claims, leases, revisions, approvals, operational events | Requirements or epistemic truth |
| Cortex MCP | Durable evidence, memories, code knowledge, relationships | Claims, leases, transitions, approvals |

The plan file `docs/plans/<board-id>.md` is a repository document: goal, acceptance checks, slices, progress and outcome. It grants no authority. Board views, chat, test output and Cortex observations are evidence. Readiness and completion come only from current `cortex_ia_work_status` state.

## 2. Roles, tiers and delegation

| Role | Work-control authority |
|---|---|
| `orchestrator` | Queries boards and tasks; creates tasks only for Tier 2; recovers expired attempts; retries reconciled blockers; routes decomposition to the planner; approves low-risk work under §2.7a. Never decomposes, claims, leases or edits product code. |
| `discovery` | Writes `./.cortex-ia/discovery.md` through `cortex_ia_discovery_write`, plus the `/.cortex-ia/` .gitignore append. Never mutates work state. |
| `planner` | Writes the plan file, creates the initiative board and its same-board DAG, and atomically applies a decomposition of a blocked task. Never claims implementation work. |
| `investigate` | Reads board and task status and durable evidence. Never mutates work state. |
| `implement` | Owns one live task claim, reserves every writable path, verifies, transitions to `in_review`. |
| `reviewer` | Inspects independently and reruns checks. Its only work mutation is `cortex_ia_work_approve`. Never edits, claims, leases or approves its own work. |

Every role runs natively in the installed host. No role hands work to an external CLI or leaf, and no external surface holds work-control, approval, session or MCP authority. Only the orchestrator starts, summarizes and ends the Cortex session. It keeps one session ID and one board ID per initiative. Dispatched controllers are ephemeral and never call session lifecycle tools.

### Tiers

- **Tier 1, direct answer.** Answers, summaries, chat-composed documentation and read-only lookups need no board or claim. Filesystem reads go to `investigate`, or the orchestrator uses facts already supplied. A file mutation becomes Tier 2.
- **Tier 2, one bounded task.** For localized changes (`direct-change`, `fast-tdd`, `hotfix`, `ops-task`) the orchestrator creates one task with `cortex_ia_work_create` on the `default` board, dispatches one `implement`, and approves under §2.7a. Multi-task DAGs and decomposition belong to the planner.
- **Tier 3, planned initiative.** The planner writes `docs/plans/<board-id>.md` and materializes a same-board DAG. See `workflow-map.md`.

### Delegation

- Choose the smallest route that fits. Scale effort to the question: answer directly when the facts are in hand; dispatch one subagent for one focused question; run several only for independent questions.
- Do not dispatch a subagent for work that takes a couple of tool calls.
- Do not dispatch a second subagent to double-check a first. Review is decided by §2.7a.
- Discovery and investigate are tools for the orchestrator when it lacks facts. Run discovery when `./.cortex-ia/discovery.md` is missing or stale.
- Parallel implementers run only on tasks with distinct claims and disjoint file leases. Coupled work runs sequentially in one task. At most four run at once.
- Read-only work uses `allowed_files: []` and goes to `investigate` or `reviewer`, never `implement`. An implement dispatch always carries non-empty leased paths.
- A targeted check (one procedure, one file, one status) stays within about five tool calls and touches only the direct target.
- Route high-output commands (full test suites, linters, builds) to `reviewer` or an implementer so orchestrator context stays small.
- When the user explicitly authorizes applying something already investigated, go straight to `implement`.
- Routine work runs on the `default` board. Create a board only for an initiative with several dependent tasks.
- Verification commands are raw executable command lines with no comments or parenthetical notes. Explanations go in `acceptance_criteria` or `objective`.
- Operational scripts take credentials and targets from environment variables, never from source. Running a script against an external database or service needs the user's explicit authorization for that target; the orchestrator names it in `allowed_effects`. Without it, the implementer writes and verifies the script locally and reports that execution needs authorization.
- Workspace strategy is `current_workspace` only. `isolated_worktree` is retired and fails closed.
- Step budgets (`max_steps`) are advisory. Reaching one produces a warning, not a stop or a state change.

### 2.7a Adaptive Review Case Matrix (Normative)

The orchestrator classifies the task before deciding whether a `reviewer` is needed. The matrix tier is a floor.

- **Case 1, data and generated artifacts** (`.xlsx`, `.csv`, `.pdf`, `.parquet`, JSON fixtures, images, charts, reports). Verification is artifact proof: the file exists, is non-empty and has a valid format, or its generator exits 0. Orchestrator auto-approves. No `reviewer`.
- **Case 2, pure documentation and text** (`*.md`, `docs/**`, instructions, comments, specs). Verification is markdown validity, link validity or formatting. Orchestrator auto-approves regardless of size. No `reviewer`.
- **Case 3, declarative configuration and styling** (`.gitignore`, `.dockerignore`, CSS, non-security JSON or YAML). Verification is parser validation. Orchestrator auto-approves on clean syntax. Dispatch `reviewer` only for security-sensitive settings (IAM, firewall, auth keys, CORS).
- **Case 4, operational and database scripts** (`ops-task`). Verification is exit 0 plus target object or query checks. Execution against an external target still needs the user authorization in the fileless-operations rule above. Orchestrator auto-approves read-only or idempotent test-environment scripts. Dispatch `reviewer` for production schema changes or irreversible DDL on shared tables.
- **Case 5, low-risk unitary code** (`direct-change`, `fast-tdd`, `hotfix`). One domain, at most 3 files and 150 LOC of core logic (pure test files up to 250 LOC), deterministic green tests, no regressions. Orchestrator auto-approves.
- **Case 6, high-risk code.** Dispatch an independent `reviewer` for: concurrency, locks or goroutine lifecycles; production schema migrations or irreversible DDL; public APIs, breaking protocol changes, auth, crypto or security boundaries; more than 3 files or more than 150 LOC of core logic; failed, ambiguous or missing tests.

Auto-approval goes only through `cortex_ia_work_approve` with an `evidence` pointer. A receipt, a passing test, a board card or a chat assertion never completes a task.

**Escalate-only tier.** An implementer or reviewer may raise the tier with a stated reason when transitioning to `in_review`. Lowering the tier below the matrix is refused, and the refusal is recorded as evidence. A raised tier travels on the existing `in_review` transition and adds no task state.

## 3. Typed tools and token custody

Controllers use the typed `cortex_ia_board_*` and `cortex_ia_work_*` tools from the active bridge. The current tool schema is authoritative; never invent a tool or argument.

| Tool group | Who may use it |
|---|---|
| `cortex_ia_board_create\|list\|status` | Planner for initiative boards; orchestrator for the bounded Tier 2 bootstrap; reads per role |
| `cortex_ia_work_create` | Planner for DAG tasks; orchestrator for one Tier 2 task |
| `cortex_ia_work_decompose` | Planner only, on an orchestrator-routed blocked task, with its revision |
| `cortex_ia_work_list\|status` | Token-free reads, per role |
| `cortex_ia_work_recover` | Orchestrator only; unscoped sweep of expired claims |
| `cortex_ia_work_retry` | Orchestrator only; retries a reconciled blocked task under revision CAS |
| `cortex_ia_work_recover_own` | Implementer only; recovers its own expired task |
| `cortex_ia_work_review_refresh` | Orchestrator only; reopens review under the current revision. Grants no write authority and never approves |
| `cortex_ia_work_claim\|renew\|release_all\|transition` | Implementer only |
| `cortex_ia_file_reserve\|cortex_ia_file_release` | Implementer only; one path or a batch |
| `cortex_ia_work_approve` | Independent reviewer, or orchestrator auto-approval under §2.7a |

The bridge keeps claim and lease tokens in process memory and sends them to the CLI over stdin. Tokens never appear in prompts, argv, receipts, logs, files, Cortex observations or chat. A human may use the literal-token CLI form in a protected terminal when it is necessary.

## 4. Task lifecycle

```text
backlog --dependencies done--> ready --claim--> in_progress
in_progress --verified implementation--> in_review
in_review --PASS (reviewer or orchestrator auto-approval)--> done
in_review --FAIL--> blocked --explicit retry--> ready
in_progress --expired authority or recovery--> blocked
```

1. Read readiness from `cortex_ia_work_status`. Dependencies stay inside one board.
2. Claim one `ready` task and keep its current revision.
3. Reserve every writable file before the first write, in sorted order, with `cortex_ia_work_claim({ task_id, paths })` or `cortex_ia_file_reserve({ task_id, paths })` for mid-flight additions. If a file conflicts, release what you acquired, write nothing to that file, and report `blocked`.
4. The bridge renews the claim and leases while the host makes progress. Renew manually only when a tool reports maintenance inactive or a lease is near expiry. When authority expires or becomes uncertain, stop writing, keep the diff, and report `blocked`.
5. Run focused checks, then proportional regression. Fast-TDD code tasks (a persistent test delta in a testable unit) also record mutation evidence before `in_review`: apply one or two minimal reversible mutations to the changed logic, rerun the covering test, and revert. `KILLED` means the test failed under the mutation. `SURVIVED` means the test passed anyway; strengthen the assertions and repeat until `KILLED`. `STATIC-ANALYSIS` is the fallback when mutations cannot run, with a recorded assertion of the unit's error boundaries, return values and failure branches. Documentation, declarative config, generated output and tasks without a reliable fast oracle are exempt; record the reason. Store bounded evidence, never full stdout.
6. Transition to `in_review` with the current revision (`cortex_ia_work_transition({ to: "in_review", revision: <n> })`). The bridge releases leases on transition and records the review authority and implementation owner. A task in `in_review` stays valid for approval even if the claim TTL elapses.
7. Only an approved PASS produces `done` and unlocks eligible dependents. On failure, transition to `blocked` (which releases leases and the claim) and record a `blocked_reason` from §8.

Every transition, approval and retry passes the current `revision`. On a stale-revision conflict, re-read status and decide again; do not retry blindly.

A claim lasts 30 minutes per successful renewal. A host that stops making progress for 15 minutes stops renewing, so the claim expires at most 30 minutes later and stays detectable. The owning implementer reconciles its own expiry with `cortex_ia_work_recover_own`. Unscoped recovery and retry stay with the orchestrator.

Review approval fingerprints the task definition and its writable files. Drift requires a fresh review. Historical approvals are never rewritten.

Known bridge edge cases:
- One claim per session. An in-memory handle can outlive a dead durable claim and block new claims with `BRIDGE_AUTHORITY_UNUSABLE`. A claim for the owned task that fails on status clears the handle.
- The review binding fingerprints working-tree bytes. A checkout or branch switch between `in_review` and approval needs a fresh review.
- A claim that expires before transition moves the task to `blocked`. Its owner recovers it, or a verified diff is re-claimed and transitioned without rerunning full suites when the evidence is already durable.

### Workload LOC Budget (Normative)

Before `in_review`, run `cortex_ia_workload_audit` (git diff numstat, categorized by source, test and schema/doc). Source logic covers Go, Rust, Java and C# and TS or Python (deletions in TS and Python count 0.2x). Declarative data and schemas are exempt from every tier.

| `workload_policy` | Source logic, Go/Rust/Java/C# | Source logic, TS/Python | Tests and fixtures | Over budget |
|---|---|---|---|---|
| `strict` | <= 350 LOC | <= 250 LOC | <= 600 LOC total; modular files <= 250 LOC | `blocked` with `blocked_reason: "scope_drift"` and `WORKLOAD_SOURCE_BUDGET_EXCEEDED` (or `WORKLOAD_TEST_BUDGET_EXCEEDED`) at the start of the summary; route to planner |
| `flexible` (default) | <= 700 LOC | <= 500 LOC | <= 1200 LOC total | Advisory `WORKLOAD_ADVISORY` in the summary; may transition |
| `unbounded` | No ceiling | No ceiling | No ceiling | Diff checks bypassed |

Circuit breaker. Two consecutive review FAIL verdicts, or a diff over budget under `strict`, mean the orchestrator does not retry the monolithic task. It routes the task to the planner with `phase: "decompose"`. The planner designs 2 to 8 fully specified tasks and calls `cortex_ia_work_decompose` once. Cortex applies the plan atomically: it keeps board membership and upstream dependencies, chains the children, redirects downstream dependencies to the last child, and marks the parent `superseded`. Orchestrators, implementers and reviewers never decompose directly.

Other rules for this budget:
- Tasks whose `allowed_files` are only tests, fixtures or harnesses are never decomposed. Fix or simplify the assertion, prune unrealistic mocks, or use a standard CLI oracle.
- Implementers do not weaken contracts to make tests pass. Batch validation works on copies and never mutates caller input before the whole request is validated.
- New test scenarios go in dedicated files of 250 LOC or less. Do not add to a test file over 300 LOC.
- Code-intelligence fields (doc summaries, reasoning, symbols, callers, relationships) are never redacted or altered by privacy or sanitization steps.
- Recovery reconciles expired authority only; it does not recreate claims or leases. Retry uses a fresh attempt with fresh tokens. A task gets at most five durable claim attempts.
- On a transient quota error, reconcile durable state and resume under fresh authority. Do not mark the task permanently blocked.
- When a dispatch ends empty, corrupt or aborted, reconcile durable state in SQLite before a fresh attempt. Never reuse tokens from an expired or terminal attempt. Read-only roles are never latched; repeated aborts are reported as `ERR_SUBAGENT_READONLY_REPEATED_ABORT`.

## 5. Dispatch

Every dispatch is one `<minion-dispatch>` JSON object. Fields:

- Required: `contract_version` (`"2.0"`), `role`, `workflow`, `phase`, `task_id` (a string, or `null` for read-only roles), `objective`, `allowed_files`, `acceptance_checks`, `artifact_refs`.
- Optional: `non_goals`; `context` (what is already known, with `file:line` anchors); `evidence_refs` (a retry after FAIL includes `gotchas/<task_id>`); `allowed_effects` (only with user authorization); `workload_policy` (default `flexible`); `max_steps` (advisory).
- Implement requires non-empty `allowed_files`. Planner uses workflow `plan` with phase `plan` or `decompose`.

Envelopes are context-complete: they carry anchors, verified evidence with its command and result, user decisions, non-goals and the verification oracle, so a subagent does not need to ask mid-run. Envelopes never include tokens or credentials. An envelope describes routing and never grants authority.

Native background dispatch needs `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true`. Only the orchestrator launches role controllers. Completion notifications are the join signal; do not poll in loops.

## 6. Orchestrator and subagent communication

1. **Context-complete envelopes.** See §5. Asking the orchestrator for context it already held is an envelope defect.
2. **Open questions in receipts.** Unresolved questions travel as `open_questions` in the receipt (§8). The orchestrator resolves each through a user gate, a follow-up dispatch or steering.
3. **Steering.** The orchestrator keeps each child's `sessionID` and steers a live child through `task` session continuation: `steer` injects into the current run, `queue` waits for the next step. Steering never changes authority; tokens never appear in steering payloads.
4. **Synthetic context injection.** Shell-capable roles may post context or evidence to their own session with `opencode api post /api/session/<sessionID>/synthetic --data '<json>'`. Payloads never carry work-state mutations, tokens or secrets. This is context only, not generated test data.

## 8. Completion receipt

A controller reports its result through the typed tools (`cortex_ia_work_transition`, `cortex_ia_work_approve`) or as its final message. Use this schema:

```text
status: done | blocked | failed              (one line)
summary: 1-3 sentences, result first
changed_files: [...]                          (implement only)
checks: ["<command> -> exit <code>", ...]       (commands actually run)
verdicts: [{ check_id, verdict, evidence_ref }]   (one per acceptance-check ID in the envelope)
evidence_refs: [...]
open_questions / blockers: [...]              (only if any)
next_route: stop | review | retry | continue | direct-change | fast-tdd | hotfix | spike | plan | retrospective
```

- `verdict` is `PASS`, `FAIL`, `BLOCKED` or `INCONCLUSIVE`. A `status` of `done` is the controller's own outcome and does not approve anything.
- A `blocked` transition also records `blocked_reason` from `authority_expired | upstream | needs_user | env | scope_drift`, so continuation is deterministic.
- A PASS needs executable evidence: command, exit code, revision or hash, timestamp and a bounded result. Missing evidence, task mismatch, a stale revision or an incomplete receipt yields `INCONCLUSIVE` or `BLOCKED`, never PASS.
- A diagnostic phase may end `INCONCLUSIVE` legitimately. Planning PASS attests only to the planning artifact checks, never to product behavior or approval.
- A fast-TDD code task without mutation evidence (`KILLED`, or `STATIC-ANALYSIS` with its assertion description) is `INCONCLUSIVE` or `BLOCKED`. Reviewers verify the evidence against the diff. A perpetually green test, one that would pass under any change to the logic it covers, is an oracle gap and fails a code task.
- The receiving controller checks required fields, routing identity, evidence and the task state in SQLite before accepting. For a historical text envelope, normalize the explicit fields, request the missing evidence, or return `INCONCLUSIVE`. Never invent a PASS.
- Receipts never contain secrets or tokens.

Verification matches the objective: document and contract checks for planning, cited observations and discriminating probes for diagnosis, executable acceptance checks for implementation and review.

### 8.1 Per-check verdict protocol

When a dispatch names acceptance checks, its envelope carries their IDs verbatim (for example `AC-2`, from the plan's Acceptance Checks table). The receipt returns exactly one `verdicts` entry per referenced ID, reusing the same spelling. `evidence_ref` points to the provenance record defined in `cortex-convention.md` § Verbatim provenance.

An ID that is missing from the receipt, unknown to the envelope or altered from its spelling leaves the completion `INCONCLUSIVE` or `BLOCKED`. When a worker's verdict disagrees with executable verification, the executable result wins and the conflict is recorded as evidence. An absent or failing oracle is never overridden by a worker verdict.

## 9. Error reporting

Record unrecoverable blockers with `cortex_ia_report_error({ code, message, details?, task_id? })`. Reports land in the SQLite operational events ledger for audit.

- `ERR_TASK_BLOCKED`: unmet dependencies, revision conflict, or retries exhausted.
- `ERR_VERIFICATION_FAIL`: a verification oracle or reviewer returned FAIL with reproducible details.
- `ERR_INVARIANT_VIOLATION`: dirty worktree, file-lease collision, or an expired claim token.
- `ERR_DELEGATION_FAIL`: a dispatch or subagent session aborted, crashed or exited non-zero mid-flight.
- `ERR_TOOL_EXECUTION_FAILED`: a tool failed with an unhandled exception or error response.
- `ERR_TOOL_LEASE_REQUIRED`: a mutating file tool ran without a claim or reservation.
- `ERR_TOOL_AUTHORITY_UNUSABLE`: the bridge refused a call because live authority was missing or stale (`WORK_STATUS_UNAVAILABLE`, `BRIDGE_AUTHORITY_UNUSABLE`, `BRIDGE_WRITE_AUTHORITY_UNUSABLE`, `BRIDGE_LEASE_MISSING`). Authority existed and was lost, so reconcile work before retrying with fresh authority.
- `ERR_TOOL_MCP_FAILED`: a Cortex MCP call failed to communicate or persist.
- `ERR_TOOL_INVALID_ARGS`: invalid, missing or malformed parameters.
- `ERR_TOOL_TIMEOUT`: a tool or child process exceeded a deadline. Report it when the ceiling repeats, not once.
- `ERR_SUBAGENT_READONLY_REPEATED_ABORT`: a read-only role aborted terminally on the same objective at the circuit-breaker threshold. Telemetry only; it never blocks dispatch.
