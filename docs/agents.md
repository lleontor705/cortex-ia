# Agent Topology & Coordination Contracts

**Cortex-IA** embeds an enterprise multi-agent topology tailored for **OpenCode** and **Herdr**.

<p align="center">
  <img src="assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

---

## 1. The 6 Native Roles

| Role | Execution Mode | Scope & Responsibility | Permitted Delegations |
|---|---|---|---|
| **`orchestrator`** | Primary / Interactive Coordinator | Request intake, startup alignment, Cortex session management, DAG dispatch, and final receipt synthesis. | `discovery`, `investigate`, `planner`, `implement`, `reviewer` |
| **`discovery`** | Subagent / Discovery Controller | Project onboarding, skills inventory, environment readiness, engine requirements, and maintains `.cortex-ia/discovery.md`. | None (strictly native) |
| **`investigate`** | Subagent / Read-Only Controller | Diagnostic audits, root-cause identification, exploratory spikes, and AST blast radius inspection. | Optional leaf worker (`agy` plan mode) |
| **`planner`** | Subagent / Spec Controller | OpenSpec delta specifications (RFC 2119), Given/When/Then scenarios, and task DAG decomposition (≤350 LOC). | Optional leaf worker (`agy` plan mode) |
| **`implement`** | Subagent / Mutating Controller | Single task claim, exclusive file leases, TDD oracle execution, and review transition. | Optional leaf worker (`agy` current_workspace) |
| **`reviewer`** | Subagent / Adversarial Gate | Independent test verification, mutation checks, invariant auditing, and `PASS` gate approval. | Optional read-only leaf worker |

---

## 2. Hard Security & Authority Invariants

1. **Role Separation**:
   - `orchestrator` never claims work items or holds file leases directly.
   - `discovery` never mutates code or claims tasks; writes strictly to `.cortex-ia/discovery.md`.
   - `planner` creates board DAGs and specifications; never claims implementation tasks.
   - `implement` must hold an active claim and exclusive file leases before editing any file.
   - `reviewer` cannot approve their own implementation changes (`reviewer_id != claim_owner`).
2. **Ephemeral Authority Tokens**:
   - `claim_token` and `lease_token` are kept strictly in ephemeral controller memory during execution.
   - Tokens must **never** be written to disk, committed to Git, or stored in Cortex observations.
3. **Workspace Strategy**:
   - `current_workspace` is the single supported implementation workspace strategy (`isolated_worktree` is retired).
   - Parallel native writers may share the workspace only with distinct task claims and disjoint per-file `cortex_ia_file_reserve` calls made before editing each file.
4. **Fail-Closed Execution**:
   - If a claim or file lease expires before completion, the implementing agent must **immediately stop writing**, preserve the diff, and report `BLOCKED`.

---

## 3. Typed Receipt & Delivery Contract

### A. Zero Raw JSON in Chat Invariant
Agents and subagents must **NEVER** output raw JSON blocks, JSON schemas, or serialized receipt envelopes as visible chat text. All machine-readable receipt data is delivered via **typed tool calls**, and human-readable explanations are rendered in clean, concise Markdown.

### B. Typed Tool Transitions
Instead of printing JSON in chat, workers and reviewers invoke typed tools whose arguments are recorded atomically in SQLite (`~/.cortex-ia/delegation.db`):

- **Implementation Minions** call `cortex_ia_work_transition`:
  ```typescript
  cortex_ia_work_transition({
    task_id: "task-auth-001",
    to: "in_review",
    summary: "Scaffolded JWT middleware and added deterministic unit tests",
    verdict: "PASS",
    evidence_refs: ["cortex_topic_or_id"],
    changed_files: [
      "internal/auth/middleware.go",
      "internal/auth/middleware_test.go"
    ]
  })
  ```

- **Reviewers** call `cortex_ia_work_approve`:
  ```typescript
  cortex_ia_work_approve({
    task_id: "task-auth-001",
    verdict: "PASS", // or "FAIL"
    reason: "All tests pass; AST analysis confirms clean decoupling",
    summary: "Independent oracle run successful with zero coupling regressions",
    findings: [
      "Zero cycle regressions detected",
      "Coverage meets proportional threshold (91%)"
    ]
  })
  ```

### C. Delivery Guarantee
Saving observations or evidence to Cortex (`cortex_save`) or updating SQLite status is an internal side-effect and **NOT** a reply to the user or orchestrator. Agents must always conclude their turn by emitting a concise visible Markdown summary in chat:

```markdown
### Implementation Summary
- **Task**: task-auth-001
- **Status**: in_review
- **Verification Verdict**: PASS
- **Changed Files**:
  - `internal/auth/middleware.go`
  - `internal/auth/middleware_test.go`
- **Checks**:
  - `go test -v ./internal/auth/...` (exit 0)
```
