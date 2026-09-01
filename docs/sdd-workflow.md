# Specification-Driven Development (SDD) Workflow

**Cortex-IA** integrates **OpenSpec** contracts with a transactional **SQLite Task DAG** to provide deterministic, verifiable software evolution.

<p align="center">
  <img src="assets/sdd-pipeline.svg" alt="SDD Pipeline" width="100%" />
</p>

---

## 1. The 5-Phase SDD Loop

```text
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  1. PROPOSE  │ ──▶ │   2. SPEC    │ ──▶ │  3. DECOMPOSE│ ──▶ │   4. APPLY   │ ──▶ │  5. VERIFY   │
│ (proposal.md)│     │ (RFC Delta)  │     │  (Tasks DAG) │     │ (Claim/Lease)│     │ (Review/PASS)│
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

### Phase 1: Propose & Align
- The `orchestrator` coordinates the initiative.
- If architectural ambiguity exists, the `orchestrator` runs structured interview rounds using `grill-me`.
- The `planner` initializes the change proposal in `openspec/changes/<change-id>/proposal.md`.

### Phase 2: Delta Specifications
- The `planner` writes formal RFC 2119 delta requirements under `openspec/changes/<change-id>/specs/<domain>/spec.md`.
- Scenarios use deterministic `Given / When / Then` acceptance criteria.
- Validated via `openspec validate`.

### Phase 3: Task DAG Decomposition
- The `planner` breaks down the implementation into discrete, bounded slices (≤350 LOC per task) in `tasks.md`.
- Materializes the tasks in SQLite using `cortex-ia work create`:
  ```bash
  cortex-ia board create <board-id> "<Title>"
  cortex-ia work create task-1.1 "Scaffolding" --board <board-id>
  cortex-ia work create task-1.2 "Domain Logic" --board <board-id> --depends task-1.1
  ```

### Phase 4: Atomic Implementation
- The `implement` minion reads `cortex-ia work status task-1.1`.
- Claims the task and reserves exclusive file locks:
  ```bash
  cortex-ia work claim task-1.1 --owner implement-minion-1
  cortex-ia work lease task-1.1 --claim-token <tok> --path internal/core/handler.go
  ```
- Executes the code changes (natively or delegated via Herdr with live telemetry).
- Runs unit tests and transitions the task to `in_review`:
  ```bash
  cortex-ia work transition task-1.1 --claim-token <tok> --to in_review
  ```

### Phase 5: Adversarial Review & Unlocking
- The `reviewer` independently verifies git diffs, executes test suites, and checks invariants.
- If verification passes, the reviewer approves the task:
  ```bash
  cortex-ia work approve task-1.1 --reviewer reviewer-agent --verdict PASS --evidence "All unit tests green"
  ```
- **Automatic Unlocking**: `task-1.1` becomes `done`, its file leases are purged, and `task-1.2` automatically transitions from `backlog` to `ready`.
