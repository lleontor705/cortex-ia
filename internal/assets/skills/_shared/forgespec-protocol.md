# ForgeSpec 1.6.0 Protocol — Canonical Source

Single normative source for every OpenCode role using the ForgeSpec control plane (server 1.6.0, `direct-v1`, api/schema 1.0.0). Role prompts and skills reference this file; they must not copy normative content out of it. When this file conflicts with tool schemas or live server behavior, the live server wins and this file must be corrected.

## 1. Selection and no-op (when to use ForgeSpec)

- Use ForgeSpec for any coordinated change: SDD workflows, task boards, claims, file leases, approvals, authority grants, or audit.
- Do not force ForgeSpec for `direct-answer` (read-only questions, simple status) or Cortex-only memory work; no capabilities call is needed there.
- A `direct-change` without a board task never invents claims; if coordination is active, check file conflicts before editing.
- No-op updates are safe: a `tb_update` with unchanged status and no notes/evidence returns `ok` with the current revision without mutating.

## 2. Negotiation cache and actor

- Call `forgespec_capabilities` (with a client identity and `requested_mode: direct-v1`) before the first coordinated operation. Proceed only when `compatibility.compatible` is true and `direct-v1` is selected.
- Cache the capabilities result only while the server identity (name/version), transport/session context, and required features are unchanged. Renegotiate on any server, version, context, session, required-feature, or compatibility change, or when an error suggests drift. Never assume a mode: read the advertised modes.
- Use exactly `coordination_mode: "direct-v1"`, `api_version: "1.0.0"`, `schema_version: "1.0.0"` on direct-v1 calls; the file-lease service rejects anything else.
- Actor: one stable identity string per role invocation (for example the dispatching role name). Direct-v1 calls carry identity in `actor`. There is no global rule about the legacy `agent` field: each file-lease tool fixes its own identity fields (§9). `file_reserve` is sent with `actor` only (sending `agent` alongside `actor` was rejected live on 1.6.0 with `request_invalid`), while `file_release` is the documented exception that requires BOTH `agent` and `actor`. No other direct-v1 call carries `agent`. Keep authority tokens (`claim_token`, `lease_token`) in live memory only; never print or persist them.
- Authority-granting operations require the exact negotiated capability string `task-authority@1.0.0` (never cite negotiated strings without their version).
- `forgespec_capabilities` is the single negotiation authority: compatibility, selected mode, schemas, and limits come only from its response. `forgespec_health` is diagnostics (status, database integrity, active leases); it is not a version gate. Its `version` field may lag the capabilities-reported server version (observed live: health `1.5.2` while capabilities reported `1.6.0` with `compatible: true`). Report and investigate such a discrepancy; it does not by itself invalidate an otherwise compatible negotiation.

## 3. Legacy versus direct-v1

- Legacy reads `tb_status`, `tb_get`, `tb_unblocked` (no coordination context) do not see direct-v1 boards or tasks: direct-v1 state reports as absent. They are legacy-only; never use them to judge direct-v1 state.
- Direct-v1 reads are actor-aware: `tb_list_boards` (with `actor` plus the three version fields), `tb_query`, `tb_batch_status`, `tb_events`. Board-scoped reads may return `RESOURCE_NOT_AVAILABLE` for non-owner actors even though `tb_claim` by exact `task_id` still succeeds for the assigned actor.
- Never silently fall back from direct-v1 to legacy (the server rejects legacy mutations on direct-v1 tasks with `legacy_direct_bypass`). If direct-v1 is unavailable, stop and report `blocked`.

## 4. Tool catalog (exactly 30 live tools)

| Group | Tools |
|---|---|
| Core / negotiation (2) | `forgespec_capabilities`, `forgespec_health` |
| File leases (3) | `file_reserve`, `file_renew`, `file_release` |
| SDD contracts (5) | `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history` |
| Task board (20) | `tb_create_board`, `tb_add_task`, `tb_set_dependencies`, `tb_list_boards`, `tb_query`, `tb_status`, `tb_get`, `tb_unblocked`, `tb_batch_status`, `tb_events`, `tb_claim`, `tb_heartbeat`, `tb_update`, `tb_recover_claims`, `tb_requeue`, `tb_approve`, `tb_audit_log`, `tb_grant`, `tb_handoff`, `tb_revoke` |

The live `tools/list` must expose exactly this set. Catalog drift (extra or missing names) is a renegotiation/escalation event, never a silent adaptation.

## 5. CAS and idempotency

Every mutation is compare-and-swap on a revision and carries exactly one fresh idempotency key per logical operation (≤256 bytes). The revision field name differs per family (live 1.6.0 schemas):

| Mutation family | CAS revision field(s) | Idempotency field |
|---|---|---|
| Task state: `tb_claim`, `tb_heartbeat`, `tb_update`, `tb_approve`, `tb_requeue` | `expected_revision` (current task revision) | `idempotency_key` |
| Board: `tb_add_task`, `tb_recover_claims` | `expected_board_revision` | `idempotency_key` |
| Dependencies: `tb_set_dependencies` | `expected_board_revision` + `expected_task_revision` | `idempotency_key` |
| SDD: `sdd_save` | `expected_head_revision` (plus `submitted_digest` as `sha256:` hex) | `idempotency_key` |
| Authority: `tb_grant`, `tb_handoff`, `tb_revoke` | `expectedBoardRevision` (camelCase) | `idempotencyKey` (camelCase) |
| File leases: `file_reserve` / `file_renew`, `file_release` | `file_reserve`: `expected_task_revision` (the schema also accepts `expected_revision` as its alias); `file_renew`/`file_release`: `expected_revision` = current lease revision | `idempotency_key` |

- `tb_create_board` creates state and carries no CAS revision.
- Take each expected revision from the most recent read, claim, heartbeat, or mutation response; the task revision advances on transitions (for example, the claim's `ready -> in_progress` bump), so use the latest response value for the next CAS call.
- On `stale_revision`/CAS conflict: re-query the current revision and retry with a new key; never reuse a key across logical operations.
- Idempotent replay returns `replayed: true` with the original result; treat it as success, not as a new mutation.

## 6. Implementer lifecycle (minimum sequence)

`tb_query (ready) -> tb_claim -> file_reserve -> execute -> verify -> save sanitized evidence -> tb_update (in_review, evidence) -> file_release -> tb_update (done, status-only) -> receipt`

- Claim only `ready` tasks by exact `task_id` with current revision, a unique idempotency key, and `lease_seconds` (15–3600). The response returns the live `task_revision`, `attempt_id`, and expiry; the `ready -> in_progress` transition advances the revision, so use the latest response value for the next CAS call.
- Reserve every file in scope before editing. Direct-v1 `file_reserve` (verified on 1.6.0) requires: the coordination triplet, `actor` (send `actor` only — adding the legacy `agent` field is rejected with `request_invalid`), `workspace_id` (forward slashes on Windows), `case_policy`, `task_id`, `attempt_id`, `claim_token`, `expected_task_revision` (the `expected_revision` alias is schema-accepted), a unique `idempotency_key`, patterns (≤100 scopes), and `ttl_minutes` (server-bound 1–60). Omitting `workspace_id` or `case_policy` yields `request_invalid`; a missing authority field yields `file_lease_invalid`. On scope overlap, do not edit.
- Keep authority alive: `tb_heartbeat` (attempt lease) and `file_renew` (file lease) before expiry. If a lease or claim expires, or authority is stale/superseded, stop writing immediately, preserve the diff, and return `blocked`.
- Canonical completion order (mandatory, unambiguous): (1) verify — run the proportional oracle and record exit codes; (2) save sanitized evidence to Cortex; (3) `tb_update` to `in_review` carrying the evidence links; (4) `file_release` every lease while attempt authority is still live (release requires BOTH `agent` and `actor` — the reserve/release asymmetry documented in §9); (5) `tb_update` to `done` as a status-only transition. The `done` transition closes the attempt and revokes its claim authority (verified on 1.6.0), which is why release must precede it. The SDD contract's older `release -> update` summary is corrected here.
- Failure handling: on FAIL or BLOCKED, append the failure evidence (`tb_update` with notes, status unchanged or `in_review`), release every lease, and return the failed/blocked receipt — never self-mark `done`. TTL expiry is the sanctioned fallback only when attempt authority is already closed; record it as a deviation.

## 7. Failure and recovery

- Renew with `tb_heartbeat` before the attempt lease expires; `file_renew` before the file lease expires (extend 15–3600 s, on the lease revision).
- Expired attempts are recovered only by the orchestrator: query state/events, then `tb_recover_claims` for expired attempts, then explicit `tb_requeue` (with `recover_active_dependents` for cascades). Never reuse an old attempt or claim authority.
- `tb_events` returns authorized immutable deltas in stable revision order (cursor-paged); use it plus `tb_audit_log` to reconstruct state after crashes.

## 8. Approvals, authority, and audit

- `tb_approve` records an immutable decision only against an existing gate, from an allowed actor, with explicit asserted provenance (`approval_ref`: provider, kind, external_id, `sha256:` digest). Asserted provenance is not authentication.
- `tb_grant` creates attenuated, expiring task/board authority for named operations (requires exact `task-authority@1.0.0` negotiation). `tb_handoff` is a reference-only attenuated handoff: it carries references (provider/kind/external_id/digest), never copied transcripts. `tb_revoke` appends a revocation without changing board ownership.
- Grants, revocations, and approvals are auditable via `tb_audit_log`; `tb_events` exposes the immutable board event stream.

## 9. File leases

- One lease per reserve: patterns (≤100 scopes), `case_policy` (explicitly required in the direct-v1 envelope, consistent within a workspace; Windows workspaces use `insensitive` in practice), TTL 1–60 minutes.
- Identity-field asymmetry between reserve and release (verified on live 1.6.0 schema and behavior): `file_reserve` is `actor`-only (its schema exposes `agent` as legacy-advisory, but sending `agent` alongside `actor` was rejected live with `request_invalid`; only an equal-valued pair is schema-tolerated, so `actor`-only is the canonical form). `file_release` requires BOTH fields: the tool schema makes `agent` mandatory and the service layer rejects a missing `actor` with `file_lease_invalid` ("File lease authority fields are required") — send `agent` and `actor` with the same value. `file_renew` has no `agent` parameter at all (`actor` only).
- `file_renew`/`file_release` require the lease id/token plus matching task-attempt authority (`task_id`, `attempt_id`, `claim_token`) and the current lease revision.
- On Windows, pass `workspace_id` with forward slashes (backslashes break the transport bridge). Release is mandatory on PASS, FAIL, BLOCKED, interruption, or timeout; the TTL fallback applies only when attempt authority is already closed.

## 10. SDD contracts and task DAGs

- Validate before saving: `sdd_validate` then `sdd_save` with parent chain and digest; `sdd_history` for lineage; free-form planning content must live under the top-level `data` field (unknown sibling fields validate but are not persisted).
- Lite: `explore -> integrated plan -> task DAG -> apply -> verify`; one integrated plan contract.
- Full: `bootstrap -> explore -> proposal -> spec + design -> planning join -> task DAG -> apply -> verify -> archive`. Writes against one contract revision are serialized; spec and design revisions are joined before task creation.
- Planner owns proposal/spec/design/tasks. DAG readiness reconciles only on `done`; gates (`required_for`, `allowed_actors`) are enforced by `tb_update` transitions.

## 11. Evidence and secrets

- Persist sanitized, bounded evidence in Cortex: commands, exit codes, failure locality, counts, durations, content digests. Never raw stdout dumps, claim/lease tokens, bearer/credential material, or long-lived identifiers.
- Evidence links use provider/kind/external_id plus a `sha256:` digest binding the referenced artifact.
- Cortex evidence never determines task readiness; ForgeSpec does.

## 12. Role permission matrix (least privilege)

Exact allowed tool sets, identical to the active agent frontmatter allowlists (the machine check compares these sets verbatim; `tb_approve` for reviewer is ask-gated):

| Role | Exact ForgeSpec surface |
|---|---|
| orchestrator | `forgespec_capabilities`, `forgespec_health`, `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history`, `tb_create_board`, `tb_add_task`, `tb_set_dependencies`, `tb_list_boards`, `tb_query`, `tb_batch_status`, `tb_events`, `tb_recover_claims`, `tb_requeue`, `tb_approve`, `tb_grant`, `tb_handoff`, `tb_revoke`, `tb_audit_log` — no `tb_claim`/`tb_heartbeat`/`tb_update`, no file leases |
| planner | `forgespec_capabilities`, `forgespec_health`, `sdd_validate`, `sdd_save`, `sdd_get`, `sdd_list`, `sdd_history`, `tb_list_boards`, `tb_query`, `tb_create_board`, `tb_add_task`, `tb_set_dependencies` — no execution, recovery, approvals, or leases |
| implement | `forgespec_capabilities`, `forgespec_health`, `sdd_get`, `tb_query`, `tb_events`, `tb_claim`, `tb_heartbeat`, `tb_update`, `file_reserve`, `file_renew`, `file_release` — nothing else |
| investigate | `forgespec_capabilities`, `forgespec_health`, `sdd_get`, `sdd_list`, `sdd_history`, `tb_list_boards`, `tb_query`, `tb_batch_status`, `tb_events`, `tb_audit_log` — reads only, no mutations |
| reviewer | `forgespec_capabilities`, `forgespec_health`, `sdd_get`, `sdd_list`, `sdd_history`, `tb_list_boards`, `tb_query`, `tb_batch_status`, `tb_events`, `tb_audit_log`, plus `tb_approve` (ask-gated; configured gate naming reviewer only) — read-only otherwise |

Legacy-only readers (`tb_status`, `tb_get`, `tb_unblocked`) are granted to no active role; they never judge direct-v1 state (§3). Catalog drift between this matrix and any frontmatter allowlist is a defect in whichever side diverges from the live 30-tool catalog (§4).
