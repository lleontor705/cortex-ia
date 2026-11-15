# Harness Wave 3 — Task Contract

**Change**: harness-wave3-enforcement · **Board**: harness-wave3 · **Workload policy**: flexible
All code tasks are RED-first (TestREQ). Verification lines are raw commands.

## Tasks

### w3-01 — [delegation] Persist blocked_reason with atomic transitions
- **Requirements:** REQ-W3-003
- **Files:** internal/delegation/work.go, internal/delegation/work_submission.go, internal/delegation/work_blocked_reason_test.go (new)
- **Objective:** Add the durable blocked_reason surface per Decision D2: schema migration (work_items.blocked_reason with STRICT CHECK on the Wave 1 taxonomy authority_expired|upstream|needs_user|env|scope_drift plus '' and 'unclassified'; work_submissions.blocked_reason), TransitionWork writes the reason on to==blocked and clears it when leaving blocked, and validation rejects unknown values fail-closed inside the BEGIN IMMEDIATE boundary. RED-first: the new modular test file (<=250 LOC, temporary home) must fail before the implementation and assert valid persist, invalid reject with no state change, clear-on-unblock, and legacy NULL treated as unclassified.
- **Test:** go test -count=1 ./internal/delegation -run TestBlockedReason
- **Review:** independent reviewer mandatory (authority surface).

### w3-02 — [delegation] Class-aware stale-task TTL degradation (DegradeStaleWork)
- **Requirements:** REQ-W3-004
- **Files:** internal/delegation/work_degrade.go (new), internal/delegation/work_degrade_test.go (new)
- **Objective:** Mirror the PruneWork pattern (work.go:1508) for hygiene: DegradeStaleWorkOptions{BoardID, TTL, DryRun} with default TTL 168h; candidates are status IN (ready, blocked) with updated_at <= cutoff, excluding blocked_reason='needs_user' (legacy NULL = unclassified, degradable) and any row with a live claim; apply phase runs in BEGIN IMMEDIATE, increments revision, sets status=backlog; result receipt carries per-class degraded counts, exempted IDs, and dry-run flag; in_progress/in_review/done are structurally unreachable. RED-first: tests use a temporary home, seed mixed boards, assert dry-run non-mutation, needs_user exemption, class-aware receipt, and revision/readiness behavior.
- **Test:** go test -count=1 ./internal/delegation -run TestDegradeStaleWork
- **Review:** independent reviewer mandatory (authority surface). Depends on w3-01 (blocked_reason column).

### w3-03 — [cli] Wire work degrade command and --blocked-reason flag
- **Requirements:** REQ-W3-003, REQ-W3-004
- **Files:** internal/app/work.go, internal/app/work_degrade_test.go (new)
- **Objective:** Expose the store surfaces on the dispatcher: `cortex-ia work degrade [--board <board-id>] [--ttl <duration>] [--dry-run]` mirroring prune flag handling and printJSON receipts, and `--blocked-reason <enum>` on `work transition` forwarded into the submission input with usage text and shell completion list updated. RED-first in-app test with a temporary home covering degrade flag parsing/dry-run output and blocked-reason pass-through plus invalid-enum usage failure.
- **Test:** go test -count=1 ./internal/app -run TestWorkDegrade
- **Review:** independent reviewer mandatory (authority surface). Depends on w3-01, w3-02.

### w3-04 — [plugin] Harden work_transition: E1 gotcha gate + blocked_reason argument
- **Requirements:** REQ-W3-001, REQ-W3-003
- **Files:** internal/assets/plugins/cortex-work.ts, scripts/harness-gotcha-gate.test.mjs (new)
- **Objective:** In cortex_ia_work_transition execute: (a) E1 gate per Decision D1 — on to==in_review, call cortex(["work","approvals",task_id]); if any FAIL verdict exists, require an evidence_refs entry matching gotchas/<task_id> for the target task; reject fail-closed on unparsable/failed lookups; zero-FAIL histories pass untouched; (b) add optional blocked_reason enum argument (authority_expired|upstream|needs_user|env|scope_drift) forwarded as --blocked-reason only when the CLI advertises it (w3-03 landed). RED-first: dedicated harness test (<=250 LOC) with virtualized child_process covering all four D1 scenarios and the enum forwarding; suites must not append to existing harness files.
- **Test:** node --test scripts/harness-gotcha-gate.test.mjs
- **Verification:** go build -o bin/cortex-ia ./cmd/cortex-ia
- **Review:** independent reviewer mandatory (authority gate). Depends on w3-03.

### w3-05 — [plugin] Read-only abort recovery-request escalation
- **Requirements:** REQ-W3-002
- **Files:** internal/assets/plugins/cortex-task-latch.ts, scripts/harness-latch-recovery.test.mjs (new)
- **Objective:** In the READONLY_ROLES branch of executeAfter: keep existing streak telemetry; at attempts === MAX_CIRCUIT_ATTEMPTS emit ERR_SUBAGENT_READONLY_RECOVERY_REQUEST once per streak via reportSignal with structured details {role, task_id, reason, attempts, objective digest, dispatch_blocked:false, recovery_hint}; clear streak state on success (existing readonlyStreaks cleanup) so the signal re-arms. No latch, no throw, no durable work mutation, no mutation authority for read-only roles. RED-first: dedicated harness test (<=250 LOC) asserting threshold emission, once-per-streak, reset/re-arm, and zero state mutation.
- **Test:** node --test scripts/harness-latch-recovery.test.mjs
- **Verification:** go build -o bin/cortex-ia ./cmd/cortex-ia
- **Review:** independent reviewer mandatory (runtime gate). No dependencies (Wave A).

### w3-06 — [delegation] Remove unreachable speckit projection branch
- **Requirements:** REQ-W3-005
- **Files:** internal/delegation/work_reservations.go
- **Objective:** Delete the hq-06-residue branch at lines 64-66 (GetWork + SpecPlane=="speckit" + ProjectSpecKitState call) from ClaimWorkWithLeases; the branch is unreachable because decodeContract rejects speckit (see speckit_projector.go). Keep the fail-closed stub and its tests untouched. No behavior change for openspec/cortex/hybrid claims; full delegation suite must stay green.
- **Test:** go test -count=1 ./internal/delegation -run TestProjectSpecKitStateIsFailClosedStub
- **Review:** independent reviewer (bundled with the w3-01/w3-02 delegation review session). No dependencies (Wave A).

### w3-07 — [assets] Relocate AGENTS.md §8 into cortex-protocol skill
- **Requirements:** REQ-W3-006
- **Files:** internal/assets/AGENTS.md, internal/assets/skills/cortex-protocol/SKILL.md (new)
- **Objective:** Move AGENTS.md lines 236-321 (§8 Cortex Persistent Memory & Code Graph Protocol, ~6.9KB) into new internal/assets/skills/cortex-protocol/SKILL.md with frontmatter (name: cortex-protocol, opencode/autoinvoke: false) following the opencode2-knowledge relocation pattern (obs #511); replace §8 in AGENTS.md with a short digest (mandatory save taxonomy + session/board continuity summary) plus an explicit load pointer; advance the <=24KB embedded injection target. Honest assessment recorded in plan Decision D4: predominantly reference material duplicating the per-session Cortex MCP injected protocol; per-session invariants remain normative in cortex-work-protocol.md.
- **Test:** go build -o bin/cortex-ia ./cmd/cortex-ia
- **Verification:** go test -count=1 ./internal/pipeline
- **Review:** orchestrator auto-approval (Case 2 pure documentation). No dependencies (Wave A).

### w3-08 — [oracle] Agent frontmatter permission-invariant test
- **Requirements:** REQ-W3-007
- **Files:** scripts/harness-permission-oracle.test.mjs (new)
- **Objective:** Dedicated node:test suite (<=250 LOC) parsing the six internal/assets/agents/*.md frontmatters and asserting the w2-06 oracle-coverage invariants: question deny on the five minions; orchestrator subagent allowlist with no general dispatch; exactly one cortex_* deny umbrella plus cortex_cortex_*-form allows; no retired v1 actions (write, write_to_file, apply_patch); steps present. Failures must name the agent file and violated invariant. Pure-test task: on failure, fix assertions directly; MUST NOT be decomposed.
- **Test:** node --test scripts/harness-permission-oracle.test.mjs
- **Review:** orchestrator auto-approval (pure test). No dependencies (Wave A).
