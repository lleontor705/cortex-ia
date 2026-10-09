> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# Agent Topology & Coordination Contracts

**Cortex-IA** embeds an enterprise multi-agent topology tailored for **OpenCode**. Execution is native-only: there is no external execution leaf, and every role controller runs inside OpenCode under Cortex-IA work authority.

<p align="center">
  <img src="assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

---

## 1. The 6 Native Roles

| Role | Execution Mode | Scope & Responsibility | Permitted Delegations |
|---|---|---|---|
| **`orchestrator`** | Primary / Interactive Coordinator | Request intake, startup alignment, Cortex session management, DAG dispatch, and final receipt synthesis. | `discovery`, `investigate`, `planner`, `implement`, `reviewer` |
| **`discovery`** | Subagent / Discovery Controller | Project onboarding, skills inventory, environment readiness, engine requirements, and maintains `.cortex-ia/discovery.md`. | None (strictly native) |
| **`investigate`** | Subagent / Read-Only Controller | Diagnostic audits, root-cause identification, exploratory spikes, and AST blast radius inspection. | None (native-only) |
| **`planner`** | Subagent / Spec Controller | OpenSpec delta specifications (RFC 2119), Given/When/Then scenarios, and task DAG decomposition sized by the active `workload_policy` tier (`strict` / `flexible` / `unbounded`) and the resume test. | None (native-only) |
| **`implement`** | Subagent / Mutating Controller | Single task claim, exclusive file leases, TDD oracle execution, and review transition. | None (native-only) |
| **`reviewer`** | Subagent / Adversarial Gate | Independent test verification, mutation checks, invariant auditing, and `PASS` gate approval. | None (native-only) |

The external `agy` execution leaf and its `delegate` / `herdr` delegation bridge are retired. Every role controller is native: no external CLI receives work-control tokens, approvals, session lifecycle, or MCP authority. `internal/herdr` remains in the tree as an optional diagnostics-only helper whose sole production consumer is the web console status display.

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
    ],
    verdicts: [
      { req_id: "REQ-AUTH-001", verdict: "PASS", evidence_ref: "cortex_topic_or_id" }
    ]
  })
  ```

  When an envelope references S#/REQ IDs verbatim, the receipt returns exactly one verdict per referenced ID as `verdicts: [{ req_id, verdict, evidence_ref }]`, reusing the verbatim ID string; executable results override worker verdicts on conflict.

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

---

## 4. Adaptive Review & Authority Deltas (pointer digests)

- **Adaptive Review Case Matrix**: non-code kinds — generated data/artifacts, pure documentation/text, declarative config, and operational/DB scripts — are orchestrator auto-approved with an evidence pointer; an independent `reviewer` is mandatory only for high-risk code (concurrency/locks, production schema or irreversible DDL, public APIs/auth/crypto/security boundaries, more than 3 files or more than 150 LOC of core logic, or failed/ambiguous/missing tests). Auto-approval runs exclusively through `cortex_ia_work_approve`.
  - Digest — normative source: `cortex-work-protocol.md` §2.7a Adaptive Review Case Matrix.
- **Escalate-only tier**: the review tier assigned by the matrix is a floor, never a ceiling; an implementer or reviewer MAY raise it with a stated reason, and lowering below the assigned tier is refused fail-closed.
- **Per-spec verdict receipt shape**: when a dispatch carries S#/REQ IDs verbatim, the receipt returns exactly one verdict per referenced ID as `verdicts: [{ req_id, verdict, evidence_ref }]`; executable results override worker verdicts on conflict.
  - Digest — normative source: `cortex-work-protocol.md` §8.1 Per-spec verdict protocol.
- **Verbatim L1 provenance**: the original request is captured verbatim with secret redaction, and RED/GREEN appends carry commit hashes.
  - Pointer: `cortex-convention.md` § Verbatim provenance.

---

## 5. Workflow & Repository Evidence Pointers

- **SDD repositioning**: Tier 1/2 specs are a short paragraph plus executable tests; SDD-lite/full is reserved for Tier 3 / multi-session / regulated domains; living specs evolve by delta (propose → apply → archive), never by an in-place base-spec rewrite.
  - Pointer: `workflow-map.md` § Spec-plane repositioning.
- **Resume test**: a task must be resumable from the request text plus `git diff` alone; when it conflicts with a tier or LOC budget, the stricter outcome governs.
  - Pointer: `workflow-map.md` § Task sizing: the resume test.
- **Ratchets (advisory)**: dead-code and refusal-string drift are pinned to baselines under `.cortex-ia/ratchet/` and compared by `scripts/ratchet-deadcode.sh` and `scripts/ratchet-refusals.sh`; both run advisory-only in `.github/workflows/ratchets.yml` (`continue-on-error: true`) and never block a merge.
- **Dated audits & evidence**: in-repo reports live under `docs/audits/<YYYY-MM-DD>-<topic>.md` and `docs/evidence/`, carrying a two-way reference rule against their Cortex observations; see `docs/audits/README.md` and `docs/evidence/README.md`.
