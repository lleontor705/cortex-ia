# Native Background Supervisor Protocol

This protocol defines the runtime overlay for native OpenCode subagents and Cortex-IA-supervised external leaves. It does not replace or restate `cortex-work-protocol.md`.

## Authority

- `cortex-ia work` remains authoritative for readiness, tasks, claims, leases, revisions, and completion.
- Cortex stores sanitized durable evidence and summaries. It never determines readiness.
- The supervisor exposes advisory, reconstructible native-session state only. It never stores authority tokens or creates a third control plane.
- Cortex-IA stores external execution state in `~/.cortex-ia/delegation.db` (SQLite WAL). That state is operational and reconstructible; it never grants task readiness, claims, leases, or completion.

## Prerequisite and transport

- Start OpenCode with `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true`.
- The orchestrator uses only the native `task` tool to launch a role controller. The plugin never bypasses that controller by creating an OpenCode child session.
- A native role controller may call `cortex_delegate_start` for one external AGY leaf. Cortex-IA, not the plugin or AGY, owns process launch, timeout, cancellation, recovery, and receipts.
- Only `orchestrator` launches native background subagents. Only a native role controller may request its external leaf. Nested delegation is denied at both layers.
- OpenCode completion notifications are the normal join signal. Do not sleep or poll repeatedly.

## Execution mode contract

The controller MUST obey the `execution_mode` returned by `cortex_delegate_start`; it MUST NOT infer the effective mode from installer choices, `use_herdr`, binary availability, or pane state.

| `execution_mode` | Meaning | Required controller behavior |
|---|---|---|
| `native` | No external job was accepted. | Execute the objective with the native role controller. Do not poll delegation tools. |
| `direct_cli` | Cortex accepted an external AGY job and launched it directly. | Retain CLI task authority, monitor the job, consume its receipt, and do not execute the same objective concurrently. |
| `herdr_multiplexed` | Cortex accepted an external AGY job and launched it in a Herdr pane. | Behave exactly as in `direct_cli`; the pane adds visibility and multiplexing, never authority. |

- `use_herdr=true` is a preference, not an execution fact. Cortex may safely fall back from Herdr to `direct_cli` before returning the accepted job.
- `delegated=false` MUST carry `execution_mode=native` and means that no external job remains accepted by that call.
- Once a response contains `delegated=true` and a `job_id`, the controller MUST NOT silently fall back to native execution after `failed`, `timed_out`, `cancelled`, `lost`, a missing pane, or a transient status error. It first reconciles durable status, requests cancellation or recovery when applicable, and reports or explicitly retries under fresh authority.
- Mode selection never transfers Cortex-IA work-control or Cortex session authority. External AGY remains a non-delegating execution leaf in both external modes.

Every new background task prompt contains exactly one marker:

```text
<minion-dispatch>{...JSON envelope...}</minion-dispatch>
```

The JSON uses the orchestrator dispatch contract and adds `role` plus optional `worktree_isolated`:

```json
{
  "objective": "...",
  "workflow": "sdd-full",
  "task_id": "task-id-or-null",
  "artifact_refs": [],
  "evidence_refs": [],
  "non_goals": [],
  "allowed_files": [],
  "allowed_effects": [],
  "required_skill": null,
  "acceptance_checks": [],
  "budget": { "max_turns": 30, "max_retries": 1 },
  "stop_conditions": [],
  "escalate_when": [],
  "role": "investigate",
  "worktree_isolated": false
}
```

The envelope must contain `task_id` explicitly (`null` is valid for direct work) and no claim token, lease token, bearer credential, secret, password, or private key. `role` must match `task.subagent_type`. An `implement` envelope requires at least one `allowed_files` entry.

## Admission and ownership

- `investigate`, `planner`, and `reviewer` are readers. Default limit: `OPENCODE_BG_MAX_READERS=4`.
- `implement` is a writer. Default limit: `OPENCODE_BG_MAX_WRITERS=1`.
- Raise writer concurrency only when CLI readiness, non-overlapping file leases, effects, and worktree isolation prove independence.
- A minion still owns exactly one Cortex-IA work task/claim and follows the canonical claim/lease lifecycle.
- A rejected admission is not queued by the plugin. The orchestrator may dispatch later when capacity is available.

## Runtime tools

- `background_doctor`: report flag, observed capability, limits, and counts.
- `background_status`: reconcile native session status and sanitized receipts.
- `background_tail`: read a bounded child-session tail without prompting or interrupting it.
- `background_cancel`: abort one owned child session; never delete it.
- `background_recover`: inspect only the caller's child tree and adopt a session only when its user message contains a valid supervisor envelope whose role matches the native child title.

Use these tools for exceptional diagnosis and explicit user operations, not aggressive polling.

## Reconciliation and completion

- Native `cancelled` is advisory. If the session is still `busy`, effective state remains running.
- A missing/incomplete receipt, invalid task status, or `task_id` mismatch remains `INCONCLUSIVE`; it is never promoted to `PASS`.
- Native idle does not prove task completion. Apply the canonical completion order in `cortex-work-protocol.md`.
- Parent interruption does not imply child cancellation. Use `background_cancel` when cancellation is intended; termination requires native abort acknowledgement and idle reconciliation.
- Recovery can rediscover session identity and native status, but not work-control authority tokens. Reconcile with `cortex-ia work status` before resuming a recovered writer.
- External recovery changes expired `starting`, `running`, or `blocked` jobs to `lost`; it never recreates a work claim or lease. An implement controller must reconcile and reacquire authority before any retry.
- Failed launches expire from admission through a bounded opportunistic TTL; no timer or polling loop is used.
- Advisory records are capped by pruning the oldest terminal records; active records are never discarded to satisfy the cap.
- Compaction context contains only bounded sanitized identifiers, state, and verdict; never prompts, transcripts, or authority material.
