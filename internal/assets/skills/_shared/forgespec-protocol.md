# ForgeSpec Protocol 2.0 Normative Reference

**Version**: `2.0.0` / `2.1.0` · **Control Plane**: `forgespec-mcp` · **OpenCode Adapter**: `opencode-forgespec`

This document defines the single normative source of truth for ForgeSpec Protocol 2.0 coordination, role surfaces, lifecycles, and security invariants.

---

## 1. Canonical 18-Tool Catalog

All tools in OpenCode are namespaced with the `forgespec_` prefix and operate under strict cryptographic session identity verification:

| Domain | Canonical Tool Name | Purpose |
|---|---|---|
| **Boards & Contracts** | `forgespec_board_create` | Create isolated project boards with atomic CAS revisions. |
| | `forgespec_contract_commit` | Commit immutable cryptographic spec/design contract digests. |
| | `forgespec_contract_query` | Query contract history, revisions, and lifecycle phases. |
| | `forgespec_contract_validate` | Validate contract schemas against formal RFC 2119 rules. |
| **Tasks & Planning** | `forgespec_task_define` | Define tasks in DAG order with explicit dependencies. |
| | `forgespec_task_query` | Query task board state, status filters, and dependencies. |
| | `forgespec_task_transition` | Transition task state (`ready`, `in_progress`, `in_review`, `done`, `blocked`). |
| **Attempts & Claims** | `forgespec_attempt_claim` | Ephemeral worker claims task execution with bounded TTL. |
| | `forgespec_attempt_renew` | Heartbeat to extend attempt execution lease before expiry. |
| | `forgespec_attempt_recover` | Orchestrator reclaims orphaned or timed-out task attempts. |
| **File Leases** | `forgespec_lease_reserve` | Reserve exclusive/shared optimistic file scopes before editing. |
| | `forgespec_lease_renew` | Extend active file lease reservations. |
| | `forgespec_lease_release` | Release file lease scopes upon task completion or rollback. |
| **Governance & Events** | `forgespec_authority_manage` | Delegate or revoke role authority and capability tokens. |
| | `forgespec_approval_record` | Record formal reviewer/human gate approvals with SHA-256 digests. |
| | `forgespec_event_query` | Query append-only audit event log with HMAC-signed pagination. |
| **Core & Diagnostics** | `forgespec_forge_health` | Safe diagnostics: node runtime, sqlite version, 16 tables check. |
| | `forgespec_forge_negotiate` | Capability handshake and role profile tool filtering. |

---

## 2. Deterministic Role Profiles

When initializing, each agent calls `forgespec_forge_negotiate` with `{"profile": "<role>"}`. The server returns only the permitted tool subset:

### 1. `orchestrator`
* **Permitted Tools**: `forgespec_forge_negotiate`, `forgespec_forge_health`, `forgespec_board_create`, `forgespec_task_define`, `forgespec_task_query`, `forgespec_contract_query`, `forgespec_event_query`, `forgespec_authority_manage`, `forgespec_attempt_recover`.
* **Invariants**: Never claims tasks or takes file leases directly; coordinates leaf roles.

### 2. `planner`
* **Permitted Tools**: `forgespec_forge_negotiate`, `forgespec_forge_health`, `forgespec_board_create`, `forgespec_task_define`, `forgespec_task_query`, `forgespec_contract_query`, `forgespec_contract_commit`, `forgespec_contract_validate`, `forgespec_event_query`.
* **Invariants**: Writes specifications, proposals, and task DAGs; never claims execution attempts or edits production code.

### 3. `worker` (`implement`)
* **Permitted Tools**: `forgespec_forge_negotiate`, `forgespec_forge_health`, `forgespec_attempt_claim`, `forgespec_attempt_renew`, `forgespec_lease_reserve`, `forgespec_lease_renew`, `forgespec_lease_release`, `forgespec_task_transition`, `forgespec_task_query`, `forgespec_contract_query`, `forgespec_event_query`.
* **Invariants**: Owns the complete task attempt lifecycle; releases all leases before marking `done`.

### 4. `reviewer`
* **Permitted Tools**: `forgespec_forge_negotiate`, `forgespec_forge_health`, `forgespec_approval_record`, `forgespec_task_query`, `forgespec_contract_query`, `forgespec_event_query`.
* **Invariants**: Read-only audit and formal gate decision recording; never claims tasks or modifies code.

---

### Governance: `forgespec_authority_manage` Usage
`forgespec_authority_manage` supports 4 discriminated actions (`grant`, `handoff`, `revoke`, `query`):
1. **`grant`**: Delegate board or task operations to another worker:
   ```json
   {
     "action": "grant",
     "resource": { "kind": "board", "board_id": "<board_id>" },
     "operations": ["read_board", "read_task", "add", "update"],
     "grantee_handle": "lineage:sha256:<grantee_worker_digest>",
     "expires_at": 1819047044976,
     "idempotency_key": "grant-planner-<timestamp>"
   }
   ```
2. **`handoff`**: Transfer task ownership to a new worker:
   ```json
   {
     "action": "handoff",
     "resource": { "kind": "task", "board_id": "<board_id>", "task_id": "<task_id>" },
     "operations": ["read_task", "update"],
     "to_handle": "lineage:sha256:<target_worker_digest>",
     "expires_at": 1819047044976,
     "idempotency_key": "handoff-task-<timestamp>"
   }
   ```
3. **`revoke`**: Cancel delegated authority:
   ```json
   {
     "action": "revoke",
     "board_id": "<board_id>",
     "authority_id": "<authority_id>",
     "reason": "Task completed",
     "idempotency_key": "revoke-<authority_id>-<timestamp>"
   }
   ```
4. **`query`**: Inspect active authorities:
   ```json
   {
     "action": "query",
     "resource": { "kind": "board", "board_id": "<board_id>" },
     "operation": "read_board"
   }
   ```

---

## 3. SDD 2.0 Lifecycle Pipeline

ForgeSpec contracts strictly progress through 8 deterministic phases:
```text
init ➔ explore ➔ proposal ➔ spec ➔ design ➔ tasks ➔ apply ➔ verify
```

* **Cryptographic Digests**: Each contract revision produces a deterministic `sha256:` digest bound to parent contracts and board revisions.
* **Attempt Gating**: Transitions to `apply` and `verify` require active attempt claims and scoped file leases.

---

## 4. Canonical Implementer Completion Order

Every `implement` minion must follow this exact sequence:

1. **Empirical Verification**: Run the specific test oracle, linter, or build command to prove `PASS` or `FAIL`.
2. **Sanitize Evidence**: Save concise command exit codes, diff hashes, and root causes in Cortex (`cortex_save`).
3. **Transition to In-Review**: Call `forgespec_task_transition` with `status: "in_review"` and attached evidence links.
4. **Release File Leases**: Call `forgespec_lease_release` while attempt authority is active.
5. **Final Transition**: If `PASS`, transition task to `status: "done"`. If `FAIL` or `BLOCKED`, transition to `blocked` or retain `in_review` for orchestrator triage.
6. **Mandatory Cleanup**: Leases must be released on all exit paths (`PASS`, `FAIL`, or `BLOCKED`).

---

## 5. Security & Threat Invariants

1. **Fail-Closed Identity**: Models never supply `actor`, `caller_id`, or `session` fields. The plugin automatically generates and signs `_identity` cryptographic envelopes.
2. **In-Memory Tokens**: `claim_token` and `lease_token` are ephemeral, held exclusively in live memory, and never persisted in logs, Cortex, or task descriptions.
3. **Optimistic File Leases**: Disjoint scopes prevent write collisions across parallel workers. Stale or expired leases abort write operations immediately.
4. **Storage Qualification**: Uses SQLite STRICT mode across 16 canonical `fs_*` tables with WAL mode and foreign keys enabled.
