# Design

## Decisions

Dedicated submissions versus event payload reuse: event JSON avoids migration but conflates audit log and current-attempt evidence, lacks relational attribution and invites truncation. Select a versioned STRICT append-only submissions relation, foreign-keyed to work task with attempt and transition revision identity, bounded fields, uniqueness and lookup index. The implementation should use the actual attempt representation already owned by work authority. Review linkage records the applicable submission identity separately from implementer verdict; historical approvals without submissions remain valid historical records, never backfilled or resurrected. Migrate existing schema 13 forward without changing unrelated rows. Existing append-only events still record transitions.

Service transaction validates claim/revision, inserts submission, transitions state and releases leases together. A submitted PASS is an assertion, not approval. Preserve the legacy transition API through a compatible overload or options form; CLI forwards optional fields. Public token values stay live-memory/stdin only. Project current-attempt evidence, not stale retry data.

Utilities reuse lease authority and physical path containment. Admission and prewrite checks cover conversion delay and renderer launches; filesystem mutation and SQLite are not one atomic transaction. Host-supplied identity cannot be overridden by tool arguments. No claim that shell tools are sandboxed.

Native maintenance uses authenticated batch renewal, fresh SDK session liveness, serialized bounded ticks and an explicit stale-progress ceiling. It stops on idle/error/deletion/disposal/submission or uncertainty. Never expose unauthenticated worker extension or reacquire expired authority. Existing batch release is reused. Executable discovery caching and deadlines belong to this subprocess-maintenance slice.

Byte-owning writes and reads return verification receipts. Backend token hashing, approval fingerprints, pin checks, backup integrity and HMAC remain unchanged. Compatibility tools remain callable; prompts omit redundant manual hashing and require relate only for a meaningful relationship.

A coherent job query composes status and receipt in one read snapshot; waits occur outside transactions. Existing APIs are adapters. Preserve lost fences, proof-based reconciliation and request-versus-confirmed cancellation.

Snapshot alternatives: upstream raw-ID support is unavailable; formatted MCP output cannot prove exact content. Select standard JSON streaming of existing export, shared by plugin and pin verifier through a new Cortex-IA command. Validate the entire stream including later duplicates and producer exit. Bound time/bytes/record content; retain no project array. This is not a direct-ID backend lookup or producer performance improvement.

## Planning join and dependencies

### Submission relation details and migration discipline

Schema 14 uses a stable submission ID, item_id FK with RESTRICT, attempt, implementation owner, transition revision, from/to status, optional review identity, optional verification verdict, summary, bounded JSON arrays for refs and changed paths, and timestamp. UNIQUE(item_id, transition_revision), current-attempt lookup index, and append-only UPDATE/DELETE guards preserve identity. Review and claim slots are mutable: do not FK to them or create artificial history; derive their identifying values inside the transaction. Verdict remains NULL when omitted; legacy delivery without receipt creates no invented submission. Suggested limits: summary 8 KiB, references 64 entries of at most 1 KiB, changed files 128 entries of at most 1 KiB, with JSON byte/count constraints and normalized allowed-path checks. Reject overflow, never truncate structured evidence.

During implementation and review, all real board/work coordination MUST use the absolute installed schema-13 CLI C:/Users/usrLuisLeon/go/bin/cortex-ia.exe. Never use go run or a newly built schema-14 candidate against real state before rollout. Candidate tests use isolated CORTEX_IA_HOME. Final rollout captures a consistent SQLite API backup and binary backup, installs the compatible new binary before the first database access, then migrates. Rollback after migration requires a compatible binary or controlled recovery; never automatically restore an old database over live work.

Tasks 1.1 through 3.1 share herdr-bridge.ts and later work.go/store.go; serialize their verified interface evolution. Task 3.2 is independently ready: all paths are disjoint from that chain, including its new CLI dispatch in app.go rather than work.go/delegation.go. Do not widen either scope silently. Flexible coherent tasks take precedence over artificial file-count caps.

## Verification and closure

Use existing harness loader with mocked subprocesses and synthetic homes. New persistent critical authority/transport tests are at most 250 lines; deeper migration/CLI transaction oracles are disposable isolated smokes. No real user state, production AGY or network reports in tests. Each task receives independent semantic review. Final stable snapshot runs Go 1.26.1 vet/tests, pinned golangci-lint 2.11.4, all harness tests, diff check and CLI build; refresh earlier approvals whose fingerprints changed, sequentially after all writers stop, reusing stable global evidence.

Install only after review through supported service plan/apply with verified backup and explicit saved MCP selections; preserve Context7, user config, AGY settings and asset ownership. New schema requires a compatible installed binary before migration; do not restore an old binary or database over live work. No deployment. Archive only after all current fingerprints are approved.
