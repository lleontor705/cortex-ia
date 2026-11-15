# Harness Wave 3 — Enforcement & Residual Cleanup

**Workflow**: sdd-lite (integrated) · **Spec plane**: hybrid · **Workload policy**: flexible
**Source**: docs/analysis/harness-improvement-paper.md (§4.3 friction archaeology, §4.4 proposals, §5.3 roadmap) · Waves 1–2 executed; hq-06 landed
**Board**: harness-wave3 (created by this plan; same-board DAG w3-01..w3-08)

## Intent & Non-Goals

Convert the three decision-pipeline leaks diagnosed in the paper into runtime enforcement — E1 gotcha-evidence transition gate, E3 read-only abort escalation, R3 board TTL hygiene with class-aware structured blocked_reason — and land the residual cleanups (dead speckit branch, AGENTS.md §8 relocation to an on-demand skill, asset-permission oracle). Nothing outside the repository is executed; hub-side work is flagged as operator/external actions.

Non-goals:
- NG1: No R1 (hub-side ingest-time incident classification) or R4 (decision-pipeline metrics on the console) — external operator actions, listed in §Operator Actions.
- NG2: No E3-leases (runtime file leases via ctx.permission.rules) — deferred with rationale (Decision D3).
- NG3: No reopening of Wave 1–2 surfaces beyond what the listed items require: Case Matrix/LOC digest pointers, the temperature map, compaction/retry hooks, and the w2-06/07 permission collapse are untouched.
- NG4: No restoration of ForgeSpec, the Herdr execution path, AGY, external CLI leaves, or isolated worktrees.
- NG5: No weakening of fail-closed authority semantics: every new rejection path fails closed; BEGIN IMMEDIATE remains the multi-step authority boundary in the delegation store.

## Decisions

- **D1 — E1 evidence contract (string form, fail-closed)**: The plugin bridge has no Cortex observation-read tool (the gated reader list at internal/assets/plugins/cortex-work.ts:1670-1673 exposes no observation lookup), so the gate cannot verify observation existence directly. The contract is: on `cortex_ia_work_transition({ to: "in_review" })`, the plugin (a) requires at least one `evidence_refs` entry matching `gotchas/<task_id>` (exact prefix form, task_id matching the transition target), and (b) verifies via `cortex-ia work approvals <task_id>` (ListWorkApprovals, internal/app/work.go:263) that the approval history contains at least one FAIL verdict. If the approvals query fails or returns unparsable output, the transition is rejected (fail-closed). Tasks with zero FAIL verdicts in history are unaffected — preserving the Adaptive Review pure-test/artifact exemptions.
- **D2 — R3 storage model**: Wave 1 shipped the blocked_reason taxonomy only as asset text (internal/assets/agents/implement.md:249, skills/_shared/cortex-work-protocol.md:208). Wave 3 makes it durable: a `blocked_reason` column on `work_items` (CHECK against the taxonomy plus legacy 'unclassified') is the authoritative query surface for class-aware degradation, and the blocked `work_submissions` record also carries `blocked_reason` for the append-only audit trail. The column is set on every transition to `blocked` and cleared when the task leaves blocked.
- **D3 — E3-leases deferred**: v2 `ctx.permission.rules({ sessionID, permissions })` REPLACES (not merges) session-scoped rules, and child sessions inherit rules at creation time. An allow-list of leased paths would leak authority to minions spawned while a lease is live (inheritance persists after lease release/expiry), full-snapshot rewrites race with concurrent claim/lease changes, and permission-layer denials bypass the lease-guard hook's structured LEASE_REQUIRED recovery guidance. This weakens fail-closed authority semantics (NG5), so the item is deferred to a future wave pending a bounded spike on: rule replace semantics under concurrent sessions, non-inherited scoping, and interaction with cortex-lease-guard.ts / cortex-permission-fence.ts.
- **D4 — §8 classification**: AGENTS.md §8 (lines 236-321, 86 lines, ~6.9KB) is predominantly reference material — the Cortex MCP server injects an equivalent per-session protocol block into every subagent prompt, and the per-session-invariant subset (failure extraction, save taxonomy, session/board continuity) is already normative in cortex-work-protocol.md. The w2-11 reviewer kept it inlined before the MCP-injection equivalence was established; reconsidering with that evidence, relocation is safe. It moves to a new `skills/cortex-protocol` skill (autoinvoke: false) with a short digest + pointer retained in AGENTS.md, advancing the ≤24KB injection target.
- **D5 — Pruning pattern**: The dispatch referenced obs #110, which is actually the TUI menu/upgrade FSM decision. The real pattern to extend is `Store.PruneWork` (internal/delegation/work.go:1508; CLI `work prune --board --older-than --dry-run`): candidate query → dry-run result → BEGIN IMMEDIATE batch apply. Degradation follows the same shape and never touches in_progress/in_review/done rows or rows with live claims.
- **D6 — E3 delta scope**: ERR_SUBAGENT_READONLY_REPEATED_ABORT telemetry already fires once per streak at MAX_CIRCUIT_ATTEMPTS (cortex-task-latch.ts:385-389). The Wave 3 delta is a second, structured recovery-request signal (ERR_SUBAGENT_READONLY_RECOVERY_REQUEST) emitted once per streak with machine-readable details; reportSignal calls the CLI directly so no resolveErrorCode/cortex-work.ts coupling is introduced. Read-only roles keep zero mutation authority: no latch throw, no work-state writes.

## Requirements

### Requirement: REQ-W3-001 E1 gotcha-evidence transition gate
The work-transition path enforces that a FAIL-driven rework cannot reach in_review without gotcha evidence, without affecting tasks that were never failed.

#### Scenario: FAIL history without gotcha evidence is rejected
- **GIVEN** a task whose `work approvals` history contains at least one FAIL verdict
- **WHEN** an implement session calls `cortex_ia_work_transition({ task_id, to: "in_review" })` with no `gotchas/<task_id>` entry in `evidence_refs`
- **THEN** the tool throws before invoking the CLI, the task remains in_progress, and the error names the missing gotcha-evidence contract

#### Scenario: FAIL history with valid gotcha evidence proceeds
- **GIVEN** a task whose approval history contains at least one FAIL verdict
- **WHEN** the transition supplies an `evidence_refs` entry exactly of the form `gotchas/<task_id>` matching the target task
- **THEN** the transition executes through the normal authorized CLI path and the durable acknowledgement checks are unchanged

#### Scenario: Zero-FAIL history is unaffected
- **GIVEN** a task with no FAIL verdict in its approval history (first delivery, pure-test or generated-artifact work)
- **WHEN** the session transitions to in_review with or without evidence refs
- **THEN** the gate does not reject, preserving the Adaptive Review exemptions

#### Scenario: Unavailable approval history fails closed
- **GIVEN** the `cortex-ia work approvals` lookup fails, times out, or returns unparsable output for a task with a FAIL verdict
- **WHEN** the session attempts the in_review transition
- **THEN** the gate rejects the transition instead of allowing it, and the error states that approval history could not be verified

### Requirement: REQ-W3-002 E3 read-only abort escalation
Repeated read-only-role aborts surface a structured recovery request without granting those roles any mutation authority.

#### Scenario: Recovery request after threshold aborts
- **GIVEN** a read-only role (investigate, reviewer, discovery) has aborted MAX_CIRCUIT_ATTEMPTS (4) consecutive times on the same objective in one session
- **WHEN** the last abort is processed by the latch plugin
- **THEN** the plugin emits ERR_SUBAGENT_READONLY_RECOVERY_REQUEST telemetry with structured details (role, task_id, reason, attempts, objective digest, dispatch_blocked=false) exactly once for that streak, in addition to the existing ERR_SUBAGENT_READONLY_REPEATED_ABORT signal

#### Scenario: No mutation authority is granted
- **GIVEN** the recovery request has been emitted for a read-only role
- **WHEN** subsequent dispatches of that role run
- **THEN** the plugin neither latches, throws, nor mutates any durable work state; dispatch remains available and only telemetry escalates

#### Scenario: Streak resets on success and re-arms
- **GIVEN** a read-only streak that previously reached the recovery-request threshold
- **WHEN** the same role/objective later completes successfully
- **THEN** the streak counter is cleared and a later abort sequence re-arms the one-per-streak recovery request from zero

### Requirement: REQ-W3-003 R3 blocked_reason persistence and plumbing
The blocked_reason taxonomy becomes durable, validated state across the store, CLI, and plugin transition surface.

#### Scenario: Blocked transition persists a valid reason
- **GIVEN** a claimed task in_progress
- **WHEN** a transition to blocked is submitted with blocked_reason `authority_expired` (or `upstream`, `needs_user`, `env`, `scope_drift`) via CLI flag or plugin argument
- **THEN** the work_items row stores the reason, the blocked work_submission records it for audit, and the append-only event stream reflects the class

#### Scenario: Invalid reason is rejected fail-closed
- **GIVEN** any blocked transition
- **WHEN** the submitted blocked_reason is not in the taxonomy
- **THEN** the store rejects the transition with ErrWorkConflict-class validation error and no state changes commit (BEGIN IMMEDIATE atomicity)

#### Scenario: Reason clears on re-activation and legacy rows stay queryable
- **GIVEN** a blocked task with a persisted reason, plus legacy blocked rows predating the column (NULL)
- **WHEN** the blocked task transitions onward, or degradation queries run over legacy rows
- **THEN** leaving blocked clears the reason, and NULL reasons are treated as class `unclassified` (degradable, never needs_user)

### Requirement: REQ-W3-004 R3 stale-task TTL degradation
Stale ready/blocked tasks degrade to backlog after a configurable TTL, class-aware and exempting needs_user.

#### Scenario: Stale tasks degrade to backlog after TTL
- **GIVEN** ready and blocked tasks whose updated_at is older than the configured TTL (default 168h) on a board
- **WHEN** `cortex-ia work degrade` runs (store DegradeStaleWork, BEGIN IMMEDIATE)
- **THEN** those tasks transition to backlog with revision increments and dependency readiness re-evaluated on promotion, and in_progress/in_review/done rows are never touched

#### Scenario: needs_user tasks are exempt
- **GIVEN** a blocked task whose blocked_reason is `needs_user`, older than the TTL
- **WHEN** degradation runs
- **THEN** the task remains blocked and is reported in the result receipt as exempted

#### Scenario: Class-aware receipt and dry-run safety
- **GIVEN** a mixed board of degradable classes, needs_user exemptions, and fresh tasks
- **WHEN** degradation runs with --dry-run, then for real
- **THEN** the dry-run reports per-class counts without mutating, the real run returns a receipt with degraded task IDs grouped by blocked_reason class, and tasks with live claims (if any) are skipped

### Requirement: REQ-W3-005 Speckit dead-branch cleanup
The unreachable speckit projection branch left by hq-06 is removed while the fail-closed retirement stub stays.

#### Scenario: Dead branch removed without behavior change
- **GIVEN** ClaimWorkWithLeases in internal/delegation/work_reservations.go
- **WHEN** the `SpecPlane == "speckit"` branch (lines 64-66) is removed
- **THEN** claim/reservation behavior for openspec, cortex, and hybrid contracts is byte-identical and the existing delegation suites pass

#### Scenario: Retirement stub remains fail-closed
- **GIVEN** the retired speckit projector stub
- **WHEN** the delegation suites run including TestProjectSpecKitStateIsFailClosedStub and the encode/decode rejection tests
- **THEN** the stub still returns ErrSpecKitRetired and contracts with spec_plane speckit are still rejected

#### Scenario: Claim atomicity is unchanged after cleanup
- **GIVEN** ClaimWorkWithLeases after the branch removal
- **WHEN** a claim with file reservations succeeds or fails partway
- **THEN** claim, attempt, event, and lease rollback semantics are identical to the pre-cleanup behavior and no projection side effects run on any spec plane

### Requirement: REQ-W3-006 AGENTS.md §8 relocation
The Cortex protocol section moves from the per-session harness injection into an on-demand skill with a digest pointer.

#### Scenario: §8 moves to cortex-protocol skill
- **GIVEN** internal/assets/AGENTS.md §8 (lines 236-321, ~6.9KB)
- **WHEN** the section is relocated to internal/assets/skills/cortex-protocol/SKILL.md (autoinvoke: false)
- **THEN** AGENTS.md retains a short digest + load pointer in its place, advancing the ≤24KB embedded injection target

#### Scenario: Embedded build proves relocation
- **GIVEN** assets are go:embed sources
- **WHEN** the binary is rebuilt after the move
- **THEN** the build succeeds and the installer copy tests (internal/pipeline) still pass, proving the new skill is part of the asset map

#### Scenario: Per-session invariants stay visible
- **GIVEN** the relocated skill content
- **WHEN** an agent needs the mandatory save taxonomy or session/board continuity rules
- **THEN** the retained AGENTS.md digest names them and points to the skill, and no normative rule from cortex-work-protocol.md is lost

### Requirement: REQ-W3-007 Asset-permission oracle
A dedicated test asserts the permission invariants of all six agent frontmatters, closing the w2-06 oracle-coverage WARNING.

#### Scenario: All six frontmatters parsed and invariants asserted
- **GIVEN** the six internal/assets/agents/*.md files
- **WHEN** the oracle test parses each frontmatter
- **THEN** it asserts: question deny on the five minions, subagent allowlist (no general dispatch) on orchestrator, exactly one cortex_* deny umbrella with cortex_cortex_*-form allows, no v1 actions (write, write_to_file, apply_patch), and steps present

#### Scenario: Violations fail with a named agent
- **GIVEN** a frontmatter that violates any invariant
- **WHEN** the oracle runs
- **THEN** it fails naming the agent file and the violated invariant

#### Scenario: Harness suite is modular and green
- **GIVEN** the new scripts/harness-permission-oracle.test.mjs (≤250 LOC, dedicated file)
- **WHEN** the node test suite runs
- **THEN** it passes against the current assets and does not append to any existing harness test file

## Technical Design (condensed)

- **E1** lives entirely in the cortex_ia_work_transition execute path (internal/assets/plugins/cortex-work.ts): pre-flight approvals check via `cortex(["work","approvals", task_id])`, FAIL detection, evidence_refs shape validation `^gotchas/<task_id>$`, fail-closed on query error. No new tool, no schema change.
- **E3** extends the READONLY_ROLES branch in internal/assets/plugins/cortex-task-latch.ts: keep the existing repeated-abort telemetry, add a one-per-streak ERR_SUBAGENT_READONLY_RECOVERY_REQUEST reportSignal with structured details JSON. No latch state, no throw.
- **R3 store**: migration adds work_items.blocked_reason (STRICT CHECK: taxonomy ∪ {'', 'unclassified'}) and work_submissions.blocked_reason; TransitionWork writes both on blocked, clears on leaving blocked; new Store.DegradeStaleWork(ctx, DegradeStaleWorkOptions{BoardID, TTL, DryRun}) in internal/delegation/work_degrade.go mirrors PruneWork: candidate query (status IN ready,blocked; updated_at <= cutoff; needs_user excluded; live-claim guard) → dry-run receipt → BEGIN IMMEDIATE batch UPDATE with revision increment.
- **R3 CLI**: `cortex-ia work transition <id> ... --blocked-reason <enum>` and `cortex-ia work degrade [--board <board-id>] [--ttl <duration>] [--dry-run]` in internal/app/work.go, mirroring prune flags and printJSON receipts.
- **R3 plugin**: `blocked_reason` enum argument on cortex_ia_work_transition, forwarded as --blocked-reason; the E1 gate ships in the same tool execute path.
- **Cleanup**: delete work_reservations.go:64-66; keep speckit_projector.go stub + tests.
- **Docs**: new internal/assets/skills/cortex-protocol/SKILL.md (frontmatter with opencode/autoinvoke: false), AGENTS.md §8 replaced by digest + pointer; root-asset AGENTS.md stays the single embedded harness surface.

## Task DAG (waves)

| Wave | Task | Files | Depends on | Review |
|---|---|---|---|---|
| A | w3-01 delegation blocked_reason persistence | work.go, work_submission.go, work_blocked_reason_test.go (new) | — | reviewer (RED-first) |
| A | w3-05 plugin read-only abort recovery request | cortex-task-latch.ts, harness-latch-recovery.test.mjs (new) | — | reviewer (RED-first) |
| A | w3-06 speckit dead-branch removal | work_reservations.go | — | reviewer (bundled with w3-01/02) |
| A | w3-07 §8 relocation | AGENTS.md, skills/cortex-protocol/SKILL.md (new) | — | auto-approve (Case 2) |
| A | w3-08 asset-permission oracle | harness-permission-oracle.test.mjs (new) | — | auto-approve (pure test) |
| B | w3-02 TTL degradation store | work_degrade.go (new), work_degrade_test.go (new) | w3-01 | reviewer (RED-first) |
| C | w3-03 CLI wiring | internal/app/work.go, internal/app/work_degrade_test.go (new) | w3-01, w3-02 | reviewer (RED-first) |
| D | w3-04 plugin transition hardening (E1 + blocked_reason) | cortex-work.ts, harness-gotcha-gate.test.mjs (new) | w3-03 | reviewer (RED-first) |

Disjoint allowed_files within every wave; plugin files of w3-04 and w3-05 are distinct. All code tasks are RED-first (TestREQ) with independent reviewers (authority surfaces: Go store, CLI dispatch, plugin gates). w3-07 is Case 2 documentation; w3-08 is a pure-test task and MUST NOT be decomposed on failure (fix assertions directly).

## Verification Strategy

- Go: `gofmt -s -w .`, `go vet ./...`, `go test -count=1 ./internal/delegation/... ./internal/app/...`; per-task focused runs in each task's Test line.
- Node: `node --test scripts/harness-gotcha-gate.test.mjs`, `node --test scripts/harness-latch-recovery.test.mjs`, `node --test scripts/harness-permission-oracle.test.mjs` (harness suites load plugin sources via scripts/harness-plugin-loader.mjs with virtualized child_process).
- Embed proof: `go build -o bin/cortex-ia ./cmd/cortex-ia` after any internal/assets change (w3-04, w3-05, w3-07).
- Full gates before review transition: gofmt/vet/go test -count=1 ./... and the touched node suites.

## Operator & External Actions (out of repo scope)

- R1: implement hub-side ingest-time incident classification + auto-coalesce on the report-hub (operator/external).
- R4: surface decision-pipeline metrics (graph-edge coverage, bugfix-extraction rate, interview capture) on the console (operator/external).
- After asset changes land: run `cortex-ia install sync` and restart the OpenCode daemon so plugins/skills reload.
- Optional: revisit D3 (E3-leases) behind the bounded spike listed in Decision D3.
