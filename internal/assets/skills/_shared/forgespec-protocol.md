# ForgeSpec Protocol 2.0 — Canonical Specification

Single normative source for every OpenCode role using the ForgeSpec control plane (`forgespec-mcp@2.0.0`, protocol `2.0`). Role prompts and skills reference this specification; they must not copy normative protocol text out of it. When this document conflicts with live server behavior, the live server wins and this file must be corrected.

## 1. Selection & Protocol Handshake

- Use ForgeSpec for all multi-agent coordination: boards, tasks, attempt claims, file leases, gate approvals, delegated authority, and immutable audit trails.
- Every role starts by negotiating capabilities via `forge_negotiate` with its assigned profile:
  - `profile: "orchestrator"` (Orchestration & pipeline supervision)
  - `profile: "planner"` (Contract authoring, delta specifications & task DAG decomposition)
  - `profile: "worker"` (Implementation minion: attempt claims, file reservations, execution transitions)
  - `profile: "reviewer"` (Independent adversarial review & gate approvals)
- `forge_health` provides safe runtime and storage qualification without revealing filesystem paths or credentials.

## 2. Canonical Tool Catalog (Exactly 18 Tools)

| Domain Module | Canonical Tools | Purpose |
|---|---|---|
| **Boards & Contracts (4)** | `board_create`<br>`contract_commit`<br>`contract_query`<br>`contract_validate` | Workspaces, SDD phase progression, RFC 2119 validation, cryptographic revision digests (`sha256:`). |
| **Tasks & Planning (3)** | `task_define`<br>`task_query`<br>`task_transition` | Dependency-ordered DAG definition, state machine transitions (`ready` ➔ `in_progress` ➔ `in_review` ➔ `done`). |
| **Execution & Attempts (3)** | `attempt_claim`<br>`attempt_recover`<br>`attempt_renew` | Worker assignment, attempt TTLs, heartbeat renewals, and crash recovery. |
| **File Leases (3)** | `lease_reserve`<br>`lease_renew`<br>`lease_release` | Scoped optimistic file reservations preventing write collisions across concurrent workers. |
| **Governance & Events (3)** | `authority_manage`<br>`approval_record`<br>`event_query` | Attenuated delegation, verified gate approvals (`approval_ref`), and HMAC-paginated audit trail. |
| **Core & Diagnostics (2)** | `forge_health`<br>`forge_negotiate` | Capability handshake, profile qualification, and SQLite integrity validation. |

## 3. Role Permission Matrix (Deterministic Profiles)

| Tool Name | `orchestrator` | `planner` | `worker` (`implement`) | `reviewer` |
|---|:---:|:---:|:---:|:---:|
| `forge_negotiate` | ✅ | ✅ | ✅ | ✅ |
| `forge_health` | ✅ | ✅ | ✅ | ✅ |
| `contract_query` | ✅ | ✅ | ✅ | ✅ |
| `event_query` | ✅ | ✅ | ✅ | ✅ |
| `task_query` | ✅ | ✅ | ✅ | ✅ |
| `board_create` | ✅ | ✅ | ❌ | ❌ |
| `task_define` | ✅ | ✅ | ❌ | ❌ |
| `contract_validate`| ❌ | ✅ | ❌ | ❌ |
| `contract_commit`  | ❌ | ✅ | ❌ | ❌ |
| `attempt_claim`    | ❌ | ❌ | ✅ | ❌ |
| `attempt_renew`    | ❌ | ❌ | ✅ | ❌ |
| `lease_reserve`    | ❌ | ❌ | ✅ | ❌ |
| `lease_renew`      | ❌ | ❌ | ✅ | ❌ |
| `lease_release`    | ❌ | ❌ | ✅ | ❌ |
| `task_transition`  | ❌ | ❌ | ✅ | ❌ |
| `approval_record`  | ❌ | ❌ | ❌ | ✅ |
| `attempt_recover`  | ✅ | ❌ | ❌ | ❌ |
| `authority_manage` | ✅ | ❌ | ❌ | ❌ |

## 4. Identity Broker & OpenCode Plugin Boundary

- The OpenCode plugin `opencode-forgespec` automates identity injection.
- Callers do NOT manually construct or pass identity fields (`actor`, `agent`, `caller_id`); the plugin strips alias fields and attaches cryptographic `_identity` envelopes authenticated by the local identity broker.
- Authority tokens (`claim_token`, `lease_token`) MUST remain strictly in live worker memory; they are NEVER persisted to Cortex, transcripts, or commit logs.

## 5. Implementer Lifecycle (Worker Minimum Sequence)

```text
task_query (ready)
       ↓
attempt_claim (task_id, expected_revision)
       ↓
lease_reserve (workspace_id, patterns, ttl_minutes)
       ↓
[ Execute Code Changes & Deterministic Oracles (fast-tdd) ]
       ↓
[ Save Sanitized Evidence to Cortex (cortex_save) ]
       ↓
task_transition (state: in_review, evidence_refs)
       ↓
lease_release (lease_id, lease_token)
       ↓
task_transition (state: done)
       ↓
[ Return Typed Execution Receipt to Orchestrator ]
```

1. **Claim Task**: Claim only tasks in `ready` state using `attempt_claim` with the current revision and unique idempotency key.
2. **File Reservation**: Acquire exclusive file leases with `lease_reserve` before touching files.
3. **Heartbeat Maintenance**: Renew attempt with `attempt_renew` and leases with `lease_renew` before TTL expiry. On expiration or authority conflict, stop writing immediately, preserve diff, and return `BLOCKED`.
4. **Completion Order**: (1) Verify via test oracles; (2) Save evidence in Cortex; (3) `task_transition` to `in_review`; (4) `lease_release`; (5) `task_transition` to `done`.
5. **Failure Protocol**: On `FAIL` or `BLOCKED`, update task with failure notes (`in_review` or unchanged), release all file leases, and return the execution receipt. Never self-mark `done` on failed tests.

## 6. Approvals, Authority & Governance

- **Gate Approvals (`approval_record`)**: Reviewers record immutable sign-offs against configured gates. Requires asserted provenance (`provider`, `kind`, `external_id`, `sha256:` digest).
- **Delegated Authority (`authority_manage`)**: Orchestrators grant attenuated, expiring capabilities to specific roles or subagents without transferring global board ownership.
- **Audit Lineage (`event_query`)**: Provides an immutable, HMAC-paginated log of all lifecycle transitions, contract commits, and lease acquisitions.
