# Task DAG

Initial ready group: tfo-1.1 and tfo-3.2. Main chain: tfo-1.1 -> tfo-1.2 -> tfo-2.1 -> tfo-2.2 -> tfo-3.1. Snapshot runs independently. Seven requested optimizations are represented: renewal and batch release share their controller lifecycle slice.

- [x] tfo-1.1 Persist attributable submissions and atomically deliver work
  Requirements: REQ-TOOLS-001
  Dependencies: none
  Objective: The bridge currently discards declared delivery evidence and hides release failures. Add a dedicated submission relation and service-owned atomic delivery, expose attributable current-attempt receipts, and transmit the existing public fields through CLI without granting approval authority.
  Acceptance: Versioned STRICT append-only submissions persist bounded summary/verdict/refs/normalized changed paths for the current task attempt and submission revision. Transition, submission and lease release commit together or leave no effects; stale authority, out-of-scope paths and oversized receipts reject. New attempt never projects previous submission as current. Reviewer approval remains separate, links applicable submission identity without fabricating legacy evidence; implementer PASS never completes work. Legacy calls remain supported. Actual bridge harness proves all declared evidence arrives without token disclosure. Migration preserves existing jobs, approvals, claims, leases and reconciliation proofs.
  Allowed files: internal/delegation/store.go, internal/delegation/work.go, internal/delegation/work_submission.go, internal/delegation/work_submission_test.go, internal/app/work.go, internal/assets/plugins/herdr-bridge.ts, scripts/harness-submission.test.mjs
  Verification: go test -count=1 ./internal/delegation ./internal/app

- [x] tfo-1.2 Require live artifact authority for document and diagram writes
  Requirements: REQ-TOOLS-002
  Dependencies: tfo-1.1
  Objective: Document conversion and diagram rendering currently bypass native edit guards through output paths. Separate inline reading from mutation and reuse canonical lease checks at host admission and immediately before service writes, preserving standalone intentional CLI utility behavior only through an explicit non-controller boundary.
  Acceptance: Inline document conversion remains available without write authority. Host-context implement identity and active path lease are required for output and diagram writes, with lexical and physical workspace containment and service revalidation immediately before every write or write-producing renderer launch. Expiry during conversion produces no later write. Reviewer or caller-spoofed identity cannot authorize writes. Role permissions expose authorized implement rendering while preserving aliases and all other role inventory. No claim of atomic filesystem/SQLite or OS sandbox.
  Allowed files: internal/assets/plugins/herdr-bridge.ts, internal/assets/agents/implement.md, internal/assets/plugins/cortex-lease-guard.ts, internal/app/doc.go, internal/app/diagram.go, internal/docconv/docconv.go, internal/diagram/render.go, internal/delegation/artifact_authority.go, internal/delegation/artifact_authority_test.go, scripts/harness-authority.test.mjs
  Verification: go test -count=1 ./internal/delegation ./internal/docconv ./internal/diagram ./internal/app

- [x] tfo-2.1 Renew live controller authority and release reservations in batches
  Requirements: REQ-TOOLS-003
  Dependencies: tfo-1.2
  Objective: Native controllers currently depend on manual per-file heartbeat and release calls. Move maintenance into a controller whose host liveness and live tokens authenticate a bounded transactional renewal, and reuse existing batch release while retaining TTL as the authority backstop.
  Acceptance: Authenticated batch renewal validates live claim, in_progress state and each owned live lease in one transaction, never revives expiry or exposes worker-only extension. One nonoverlapping bounded timer uses fresh host session busy/retry status; idle/error/deleted/dispose/delivery stops it. Bounded stale-progress budget prevents orphan renewal; timeout/unknown/expiry fails closed. Release-all uses one existing transactional CLI command; clear local authority only after matching success acknowledgement, preserve actionable failure. Cache executable discovery with invalidation and bounded subprocess deadlines without changing selected AGY/model/auth.
  Allowed files: internal/assets/plugins/herdr-bridge.ts, internal/delegation/work.go, internal/app/work.go, internal/delegation/work_controller_authority.go, internal/delegation/work_controller_authority_test.go, scripts/harness-controller-authority.test.mjs, scripts/harness-reservations.test.mjs
  Verification: go test -count=1 ./internal/delegation ./internal/app

- [x] tfo-2.2 Integrate content verification into semantic artifact operations
  Requirements: REQ-TOOLS-004
  Dependencies: tfo-2.1
  Objective: Controllers repeat mechanical digest calculations despite byte-aware operations and backend approval binding. Return byte-exact verification from the operation that owns the bytes and align prompts to those receipts, preserving cryptographic checks and diagnostic compatibility rather than treating a hash as semantic correctness.
  Acceptance: Artifact write returns SHA256 and byte length for exact committed UTF8 bytes; verified snapshot retrieval checks expected pin and exposes digest without mandatory second manual hash. Service still computes approval fingerprints and validates pins; compatibility hash/fingerprint tools remain available. Runtime prompts stop requiring duplicate mechanical hashing or unconditional relate for every outcome; meaningful relationships remain supported. Preserve independent approval, all aliases, AGY account temporary home, configured model and SkipPermissions true.
  Allowed files: internal/assets/plugins/herdr-bridge.ts, internal/assets/agents/reviewer.md, internal/assets/skills/_shared/cortex-convention.md, internal/assets/skills/_shared/cortex-work-protocol.md, scripts/harness-artifact-hash.test.mjs
  Verification: node --test scripts/harness-artifact-hash.test.mjs scripts/harness-prompt.test.mjs

- [x] tfo-3.1 Consolidate coherent delegation queries with compatible adapters
  Requirements: REQ-TOOLS-005
  Dependencies: tfo-2.2
  Objective: Status, wait and result repeatedly query the same job through inconsistent formatting paths. Add one typed coherent service query with bounded wait and make existing public adapters share it, preserving cancellation and proof-based reconciliation semantics.
  Acceptance: One service-owned coherent job view includes status, terminal receipt availability, cancellation request/ack and reconciliation facts. Bounded wait polls outside transactions with timeout/unknown explicit; legacy status/wait/result CLI/tools delegate compatibly without deleting aliases. Acceptance is not completion; lost remains fenced unless proof says reconciled, receipt_missing is explicit, no implicit fallback or pane close. Actual harness counts reduced redundant process calls for terminal retrieval without claiming latency benchmark.
  Allowed files: internal/delegation/store.go, internal/delegation/job_query.go, internal/delegation/job_query_test.go, internal/app/delegation.go, internal/assets/plugins/herdr-bridge.ts, scripts/harness-cancellation.test.mjs, scripts/harness-job-query.test.mjs
  Verification: go test -count=1 ./internal/delegation ./internal/app

- [x] tfo-3.2 Stream bounded Cortex exports for exact snapshot verification
  Requirements: REQ-TOOLS-006
  Dependencies: none
  Objective: Snapshot consumers buffer complete project exports although Cortex v2.3.9 has no raw per-ID CLI. Implement a shared bounded streaming export reader and a Cortex-IA command consumed by the plugin and pin verifier, improving retained memory and exact verification without pretending to change the upstream query.
  Acceptance: Shared standard Go JSON streaming reads the existing cortex export protocol and retains only requested observations; plugin and local pin verifier use it. Validate unique ID, exact project, valid UTF8/content, size and expected digest, including duplicate records after a match. Bounded elapsed time, total bytes, selected content and stderr; consume and verify producer success, reject malformed/truncated/oversized output with no silent fallback. No full export buffer or project-sized object array. No claim of upstream per-ID retrieval or reduced producer CPU; no module-cache edits, SQL reads or invented upstream flags.
  Allowed files: internal/delegation/cortex_snapshot.go, internal/delegation/cortex_snapshot_test.go, internal/delegation/spec_contract.go, internal/app/app.go, internal/app/cortex_snapshot.go, internal/assets/plugins/cortex-snapshot.ts, scripts/harness-snapshot.test.mjs
  Verification: go test -count=1 ./internal/delegation ./internal/app
