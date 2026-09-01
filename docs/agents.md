# Agent Topology & Coordination Contracts

**Cortex-IA** embeds an enterprise multi-agent topology tailored for **OpenCode** and **Herdr**.

<p align="center">
  <img src="assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

---

## 1. The 5 Native Roles

| Role | Execution Mode | Scope & Responsibility | Permitted Delegations |
|---|---|---|---|
| **`orchestrator`** | Primary / Interactive Coordinator | Request intake, startup alignment, Cortex session management, DAG dispatch, and final receipt synthesis. | `investigate`, `planner`, `implement`, `reviewer` |
| **`investigate`** | Subagent / Read-Only Controller | Diagnostic audits, root-cause identification, exploratory spikes, and AST blast radius inspection. | Optional leaf worker (`agy` plan mode) |
| **`planner`** | Subagent / Spec Controller | OpenSpec delta specifications (RFC 2119), Given/When/Then scenarios, and task DAG decomposition (≤350 LOC). | Optional leaf worker (`agy` plan mode) |
| **`implement`** | Subagent / Mutating Controller | Single task claim, exclusive file leases, TDD oracle execution, and review transition. | Optional leaf worker (`agy` accept-edits) |
| **`reviewer`** | Subagent / Adversarial Gate | Independent test verification, mutation checks, invariant auditing, and `PASS` gate approval. | Optional read-only leaf worker |

---

## 2. Hard Security & Authority Invariants

1. **Role Separation**:
   - `orchestrator` never claims work items or holds file leases directly.
   - `planner` never claims implementation tasks.
   - `implement` must hold an active claim and exclusive file lease before touching any file.
   - `reviewer` cannot approve their own implementation changes (`reviewer_id != claim_owner`).
2. **Ephemeral Authority Tokens**:
   - `claim_token` and `lease_token` are kept strictly in ephemeral memory during execution.
   - Tokens must **never** be written to disk, committed to Git, or stored in Cortex observations.
3. **Fail-Closed Execution**:
   - If a claim or file lease expires before completion, the implementing agent must **immediately stop writing**, preserve the diff, and report `BLOCKED`.

---

## 3. Typed Receipt Output Contract

Every leaf controller must return a structured typed receipt conforming to the following JSON schema:

```json
{
  "receipt_version": "2.0",
  "task_id": "activeboard-1.1",
  "phase_status": "success",
  "task_status": "done",
  "verification_verdict": "PASS",
  "changed_files": [
    "internal/domain/contracts/mutation_test.go"
  ],
  "evidence_refs": [
    "cortex_topic_or_id"
  ],
  "verification_commands": [
    {
      "command": "go test ./internal/domain/contracts/...",
      "exit_code": 0,
      "oracle_type": "unit"
    }
  ],
  "token_usage": {
    "input_tokens": 28565,
    "output_tokens": 1182,
    "thinking_tokens": 343,
    "total_tokens": 29747
  },
  "cleanup_completed": true,
  "deviations": [],
  "risks": []
}
```
