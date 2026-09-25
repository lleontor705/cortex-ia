# Task DAG — orchestrator-reconcile

Board: `orchestrator-reconcile`. Waves: 1) `task-recon-delegation` ∥ `task-recon-bridge` (disjoint Go/TS files); 2) `task-recon-tests` ∥ `task-recon-bridge-tests`; 3) `task-recon-integration`. Workload policy `flexible`: source ≤ 700 LOC Go per task; persistent test files ≤ 250 LOC each. Tests touch only temporary homes (`CORTEX_IA_HOME`/`t.TempDir()`), never developer state. Review: one independent reviewer with a security lens (authority transitions, audit completeness, no bypass paths) approves each task via `work approve`; no reviewer task node exists in the DAG.

- [ ] task-recon-delegation Add the ReconcileWork authority service and the work reconcile CLI verb
  Requirements: REQ-RECON-001, REQ-RECON-002
  Dependencies: none
  Objective: New internal/delegation/work_reconcile.go implements Store.ReconcileWork(ctx, ReconcileInput{TaskID, Reason, HostSessionID, ExpectedRevision, To, OwnerSessionInactive}) (ReconcileResult, error) in one BEGIN IMMEDIATE transaction, following the RecoverWork/RetryWork patterns in work.go. The service enforces every fail-closed release condition from REQ-RECON-001 — bounded non-empty reason, current-session owner protection against opencode-session:<HostSessionID>, the 15m staleness window on work_claims.updated_at (package constant workReconcileStaleWindow mirroring the bridge maintenance stale_progress window, with a WHY comment noting the mirror) OR the supplied host-attested inactivity evidence, positive-revision CAS, in_progress status, live (non-expired) claim, and the MaxWorkAttempts guard when To is ready — collecting ALL failed codes into one ErrWorkReconcileRefused whose message lists the failed condition names, mutating nothing on refusal. On release it captures the task's work_leases paths, deletes the task's work_claims and work_leases rows, CAS-transitions the task to blocked (default) or ready, and appends one kind=reconciled work_events row whose JSON detail is the full decision-input snapshot per REQ-RECON-002 (actor, reason, owner, attempt, expiry, last renewal, window, inactivity evidence, revisions before/after, released lease paths) containing no tokens or token hashes. internal/app/work.go adds the reconcile case with grammar reconcile <task-id> --reason <text> --session <host-session-id> --revision <n> [--to ready] [--owner-session-inactive <true|false>] parsed through the existing workIDOptions/oneOption/positiveRevision discipline (dispatcher holds no decision logic), renders the structured JSON receipt via printJSON plus one human summary line, and registers the verb in the runWork usage listing; internal/app/app.go gains the matching one-line work verb help entry.
  Acceptance:
    - ReconcileWork releases only when every condition passes; the refusal error names every failed condition and leaves claims, leases, status, revision, and work_events byte-identical
    - Expired claims refuse with claim_not_live so work recover stays the only expiry path
    - --to ready refuses with attempt_limit at MaxWorkAttempts while the default blocked release still succeeds
    - work_events gains exactly one reconciled row with the full decision-input snapshot and no token material anywhere
    - go build and go vet pass with the dispatcher free of authority logic and comments restricted to non-obvious WHY
  Allowed files: internal/delegation/work_reconcile.go, internal/app/work.go, internal/app/app.go
  Verification: go vet ./internal/delegation/... ./internal/app/...

- [ ] task-recon-bridge Defer stale bridge authority handles to durable state and expose cortex_ia_work_reconcile
  Requirements: REQ-RECON-003, REQ-RECON-004
  Dependencies: none
  Objective: internal/assets/plugins/cortex-work.ts makes durable state authoritative for the claim path: before throwing "authority for <task> is already held by this controller", cortex_ia_work_claim probes durableWorkStatus(task_id) and when the durable claim is not live or its owner no longer matches controllerIdentity(handle.sessionID) it stops the handle's maintenance loop, deletes the stale workAuthority entry, and continues to the fresh claim; the throw remains only for a still-live durable claim owned by the retained handle's own session, and a failed durable probe keeps the refusal fail-closed. The same file adds the cortex_ia_work_reconcile tool ({task_id, reason, revision, to?}) that executes ONLY as orchestrator via a new work_reconcile: ["orchestrator"] classification in the mutations role gate (unclassified tools fail at load), builds argv work reconcile <task> --reason <r> --session <context.sessionID> --revision <n> [--to ready] [--owner-session-inactive <true|false>] with the host session bound exclusively from the execution context, and derives the inactivity evidence bit exclusively from the plugin's own host tracking (the durable claim owner session was a known active host subagent and is no longer tracked → true; otherwise false), returning the durable JSON receipt verbatim.
  Acceptance:
    - cortex_ia_work_claim succeeds with a fresh claim when a retained handle has no matching durable live claim, exactly reproducing the obs-176 unblock
    - The "already held" error still fires while the durable claim is live and owned by the retained handle session
    - A durable status probe failure on a held task refuses without touching durable state
    - cortex_ia_work_reconcile is registered, role-gated to orchestrator, and passes the existing every-declared-tool-instantiates bridge test
    - --session comes only from context.sessionID and the inactivity flag only from host session tracking, never from tool arguments
  Allowed files: internal/assets/plugins/cortex-work.ts
  Verification: node --test scripts/harness-authority.test.mjs

- [ ] task-recon-tests Add the persistent fail-closed reconcile regression matrix
  Requirements: REQ-RECON-001, REQ-RECON-002
  Dependencies: task-recon-delegation
  Objective: New dedicated modular test file internal/delegation/work_reconcile_test.go (≤ 250 LOC, store fixtures on temporary homes following the newRecoveryFixture pattern with synthetic owners) locks the authority-boundary behavior that reviewers and future refactors must never regress: the full fail-closed refusal matrix (live current-session claim → current_session_owner; fresh renewed foreign claim without evidence → claim_fresh_no_inactivity_evidence; stale foreign claim → released to blocked with lease and claim rows deleted and the reconciled event written; zero or mismatched --revision → revision_mismatch; empty or whitespace-only reason → reason_missing; expired claim → claim_not_live; non-in_progress task → status_not_in_progress), the multi-condition refusal message listing every failed code with byte-identical durable state afterward, the --to ready happy path plus its attempt_limit guard, and REQ-RECON-002 audit assertions that the event detail contains the decision-input snapshot fields while neither event detail nor result JSON contains any plaintext token or its hash (assert against the claim token captured at fixture creation).
  Acceptance:
    - Every refusal condition has its own named subtest asserting the specific failed-condition code and zero mutation
    - The stale-release path asserts lease rows deleted, claim row deleted, status blocked, revision incremented once, exactly one reconciled event
    - --to ready succeeds below the attempt limit and refuses with attempt_limit at it
    - Token-absence assertions scan the event detail and receipt for the real fixture tokens
    - go test -count=1 ./internal/delegation -run '^TestReconcileWork' passes with only temporary-home fixtures
  Allowed files: internal/delegation/work_reconcile_test.go
  Verification: go test -count=1 ./internal/delegation -run '^TestReconcileWork'

- [ ] task-recon-bridge-tests Add the persistent bridge deference and reconcile-tool harness matrix
  Requirements: REQ-RECON-003, REQ-RECON-004
  Dependencies: task-recon-bridge
  Objective: New dedicated harness test scripts/harness-work-reconcile.test.mjs (node:test + the existing harness-plugin-loader.mjs mock pattern from harness-authority.test.mjs, fully synthetic execFileSync fixtures — no real database, no developer state) proves the transport-boundary semantics: a retained handle whose durable status reports no live claim (or an owner that no longer matches the handle) is dropped and the fresh claim proceeds, so the obs-176 "authority already held" deadlock cannot recur; a genuinely live durable claim owned by the retained handle still refuses with the already-held error; a failing durable probe keeps the refusal; cortex_ia_work_reconcile exists, instantiates in the tool map, denies non-orchestrator agents with BRIDGE_ROLE_DENIED before any CLI call, and for the orchestrator builds argv containing work reconcile, the context-derived --session, --reason, --revision, optional --to ready, and --owner-session-inactive values derived only from host tracking, returning the durable JSON receipt unmodified.
  Acceptance:
    - node --test scripts/harness-work-reconcile.test.mjs passes
    - The stale-handle deference test asserts the CLI claim call actually happens after the drop
    - The genuinely-held test asserts no second claim call is made
    - The role-denial test asserts zero CLI invocations for implement/planner/reviewer contexts
    - The argv matrix asserts session identity never originates from tool arguments
  Allowed files: scripts/harness-work-reconcile.test.mjs
  Verification: node --test scripts/harness-work-reconcile.test.mjs

- [ ] task-recon-integration Add the incident end-to-end smoke through CLI and bridge seams
  Requirements: REQ-RECON-001, REQ-RECON-002, REQ-RECON-003, REQ-RECON-004
  Dependencies: task-recon-tests, task-recon-bridge-tests
  Objective: Two synthetic oracles replay the exact obs-184/obs-176 incident chain claim → owner session dies → reconcile → retry → fresh claim. internal/app/work_reconcile_smoke_test.go drives the real dispatcher runWork against an isolated CORTEX_IA_HOME temporary state root (never the developer's delegation.db): create a task in a temp board, claim with a synthetic owner, reconcile from a different host session (fresh-renewal protection first refuses, then the host-attested inactivity evidence or aged renewal releases it), verify the reconciled audit row, run work retry, claim again with a new owner, and prove the first owner's tokens are dead for renew/transition — closing the token-loss loop; scripts/harness-reconcile-incidentsmoke.test.mjs replays the same chain through the bridge seam with mocked execFileSync, proving the stale-handle deference unblocks the fresh cortex_ia_work_claim and that cortex_ia_work_reconcile forwards identity and evidence end to end. Both files stay within the modular-test budget and clean up their temporary homes.
  Acceptance:
    - The CLI smoke reproduces the full incident chain and ends with a working fresh claim owned by a new session
    - Protection is proven inside the same smoke: an immediate reconcile of a foreign claim renewed now must refuse before the evidence path releases it
    - The audit ledger after release contains exactly one reconciled row for the task
    - The bridge smoke proves claim succeeds after deference and reconcile forwards --session from context
    - Both verification commands pass and touch only temporary state
  Allowed files: internal/app/work_reconcile_smoke_test.go, scripts/harness-reconcile-incidentsmoke.test.mjs
  Verification: go test -count=1 ./internal/app -run '^TestWorkReconcileIncidentSmoke$' && node --test scripts/harness-reconcile-incidentsmoke.test.mjs
