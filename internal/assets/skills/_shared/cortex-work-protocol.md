# Cortex-IA Work Control Protocol

**Version:** 2.0 · **Control plane:** `cortex-ia board`, `cortex-ia work`, `cortex-ia delegate`, `cortex-ia openspec` · **Storage:** `~/.cortex-ia/delegation.db` (SQLite STRICT + WAL)

This is the normative coordination contract for native OpenCode controllers and Cortex-IA-supervised external leaf workers. The CLI owns operational task state, while Cortex MCP remains the epistemic evidence and knowledge plane.

## 1. Command Surface & Signatures

All commands support both **positional** arguments and **named flags**, as well as `--help` / `-h` on all subcommands. Exit code zero is the command-level success signal, and all query commands output structured JSON.

### Task Boards (`cortex-ia board`)
| Command | Syntax Variants | Purpose |
|---|---|---|
| Create | `cortex-ia board create <id> <title> [desc]`<br>`cortex-ia board create --id <id> --title <title> [--description <desc>]` | Create a durable task-board boundary. |
| List | `cortex-ia board list` | List all boards with progress counts. |
| Status / Show | `cortex-ia board status <id>`<br>`cortex-ia board show <id>`<br>`cortex-ia board get <id>` | Query board metadata and full task DAG snapshot. |
| Web Dashboard | `cortex-ia web [--addr 127.0.0.1:7331] [--open]`<br>`cortex-ia board serve [--addr 127.0.0.1:7331]` | Launch embedded real-time operations dashboard. |

### Work Items & Leases (`cortex-ia work`)
| Command | Syntax Variants | Purpose |
|---|---|---|
| Create | `cortex-ia work create <id> <title> [--board <id>] [--depends <id>]...`<br>`cortex-ia work create --id <id> --title <title> [--board <id>] [--depends <id>]...` | Create a task. Tasks with unmet dependencies start in `backlog`, otherwise `ready`. |
| List | `cortex-ia work list [--board <id>]` | List all work items in DAG execution priority. |
| Status / Show | `cortex-ia work status <id>`<br>`cortex-ia work show <id>`<br>`cortex-ia work get <id>` | Query task details, status, revision, claim, and active file leases. |
| Claim | `cortex-ia work claim <id> --owner <owner> [--ttl 15m]` | Atomically transition `ready -> in_progress` and receive an ephemeral `claim_token` and `revision`. |
| Renew Claim | `cortex-ia work renew <id> --claim-token <token> [--ttl 15m]` | Extend active claim TTL before expiry. |
| Reserve Lease | `cortex-ia work lease <id> --claim-token <token> --path <file> [--ttl 15m]` | Acquire an exclusive workspace-relative file lease and receive a `lease_token`. |
| Renew Lease | `cortex-ia work lease-renew --path <file> --lease-token <token> [--ttl 15m]` | Extend file lease TTL before expiry. |
| Release Lease | `cortex-ia work release --path <file> --lease-token <token>` | Explicitly release a file lease upon completion. |
| Transition | `cortex-ia work transition <id> --claim-token <token> [--revision <n>] --to <status>` | Transition task state to `in_review`, `in_progress`, or `blocked`. (`--revision` is optional). |
| Approve | `cortex-ia work approve <id> --reviewer <id> [--revision <n>] --verdict <PASS\|FAIL> [--evidence <ref>]` | Reviewer records verdict. `PASS` atomically sets status to `done` and unlocks dependent tasks. |
| Retry | `cortex-ia work retry <id> [--revision <n>]` | Return a `blocked` task back to `ready`, atomically clearing stale claims/leases. |
| Recover | `cortex-ia work recover` | Sweep expired claims/leases, returning orphaned `in_progress` tasks to `blocked`. |

### OpenSpec SDD Workspace (`cortex-ia openspec` / `openspec`)
| Command | Syntax Variants | Purpose |
|---|---|---|
| Validate | `openspec validate [dir]`<br>`cortex-ia openspec validate [dir]` | Validate `proposal.md`, `specs/`, `design.md`, and `tasks.md` in `openspec/changes/`. |
| List | `openspec list`<br>`cortex-ia openspec list` | List active change proposals in the repository. |
| Status | `openspec status [dir]`<br>`cortex-ia openspec status [dir]` | Inspect completion status of OpenSpec delta specifications. |

### External Worker Delegation (`cortex-ia delegate`)
| Command | Syntax Variants | Purpose |
|---|---|---|
| Create | `cortex-ia delegate create --request-file <path> [--transport direct\|herdr]` | Register a background or multiplexed external worker job. |
| Status | `cortex-ia delegate status <job-id>` | Check job execution lifecycle (`pending`, `running`, `succeeded`, `failed`). |
| Result | `cortex-ia delegate result <job-id>` | Retrieve structured output receipt, token metrics, and SHA-256 digest. |
| Cancel | `cortex-ia delegate cancel <job-id>` | Terminate an active delegation job. |
| Recover | `cortex-ia delegate recover` | Reconcile lost or expired delegation jobs. |

---

## 2. Authority & Role Separation Matrix

- **`orchestrator`**: Creates and queries task DAGs, runs recovery, and retries reconciled blockers. Never claims tasks or holds file leases directly.
- **`planner`**: Writes OpenSpec specs, proposals, design docs, and materializes tasks with `work create`. Never claims implementation tasks.
- **`implement`**: Claims exactly one task, reserves exclusive file leases, executes changes (natively or delegated via Herdr), verifies, and transitions to `in_review`.
- **`reviewer`**: Independently checks diffs and regression suites, then records verdicts with `work approve`. Never writes product code or claims tasks.
- **`External Leaf Worker (CLI)`**: Runs leaf execution in isolated Herdr pane. Does NOT hold direct control plane tokens.

---

## 3. State Machine & Lifecycle Flow

```text
       ┌──────────────┐
       │   backlog    │ (dependencies unresolved)
       └──────┬───────┘
              │ (all dependencies done)
              ▼
       ┌──────────────┐
       │    ready     │ ◄──────────────────────────────┐
       └──────┬───────┘                                │
              │ (work claim --owner ...)               │
              ▼                                        │ (work retry <id>)
       ┌──────────────┐                                │
       │ in_progress  │                                │
       └──────┬───────┘                                │
              │ (work transition --to in_review)       │
              ▼                                        │
       ┌──────────────┐                                │
       │  in_review   │                                │
       └──────┬───────┘                                │
              │                                        │
      ┌───────┴────────┐                               │
      ▼                ▼                               │
┌───────────┐    ┌───────────┐                         │
│   done    │    │  blocked  │ ────────────────────────┘
└───────────┘    └───────────┘
(PASS approval)  (FAIL / timeout / recover)
```

1. **Readiness**: Read from `cortex-ia work status <id>` (never infer from chat or card position).
2. **Authority**: Keep tokens (`claim_token`, `lease_token`) in ephemeral memory only. Never persist tokens in files or git.
3. **Optimistic Locking**: Bumping revisions ensures that conflicting concurrent operations fail cleanly without corrupting state.
4. **Independent Approval**: Implementer cannot self-approve. An independent reviewer verdict is strictly required to reach `done`.
5. **Atomic Unlocking**: When a task reaches `done`, all downstream tasks in `backlog` whose dependencies are fully met automatically transition to `ready`.

