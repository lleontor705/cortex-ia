# Native Background Supervisor Protocol

This protocol defines the runtime overlay for asynchronous OpenCode delegation. It does not replace or restate `forgespec-protocol.md`.

## Authority

- ForgeSpec remains authoritative for readiness, tasks, attempts, claims, leases, revisions, joins, and completion.
- Cortex stores sanitized durable evidence and summaries. It never determines readiness.
- The supervisor exposes advisory, reconstructible native-session state only. It never stores authority tokens or creates a third control plane.

## Prerequisite and transport

- Start OpenCode with `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true`.
- Use only the native `task` tool with `background=true`; the plugin must not create or prompt child sessions itself.
- Only `orchestrator` may launch background subagents. Nested delegation is denied.
- OpenCode completion notifications are the normal join signal. Do not sleep or poll repeatedly.

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
- Raise writer concurrency only when ForgeSpec readiness, non-overlapping file leases, effects, and worktree isolation prove independence.
- A minion still owns exactly one ForgeSpec task/attempt and follows the canonical claim/lease lifecycle.
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
- Native idle does not prove ForgeSpec completion. Apply the canonical completion order in `forgespec-protocol.md`.
- Parent interruption does not imply child cancellation. Use `background_cancel` when cancellation is intended; termination requires native abort acknowledgement and idle reconciliation.
- Recovery can rediscover session identity and native status, but not ForgeSpec authority tokens. Reconcile ForgeSpec before resuming a recovered writer.
- Failed launches expire from admission through a bounded opportunistic TTL; no timer or polling loop is used.
- Advisory records are capped by pruning the oldest terminal records; active records are never discarded to satisfy the cap.
- Compaction context contains only bounded sanitized identifiers, state, and verdict; never prompts, transcripts, or authority material.
