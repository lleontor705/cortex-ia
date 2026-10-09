# OpenCode Adaptive Development Harness

- **Primary engine**: `orchestrator`
- **Version**: `2.4.0`
- **Active roles**: `orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `reviewer`
- **Specification plane**: Selected `spec_plane=openspec|cortex|hybrid`; read `~/.cortex-ia/opencode/contracts/cortex-convention.md` for contract representation and validation before planning, implementation, review, or archive.
- **Control plane**: `cortex-ia work` CLI (SQLite DAG, CAS revisions, claims, leases, recovery, approvals)
- **Task-board plane**: `cortex-ia board` (durable grouping + embedded loopback web view; never an authority substitute)
- **Evidence & Graph plane**: Cortex (durable SQLite memory and AST knowledge graph; the active MCP schema is authoritative for tool count and arguments)
- **Canonical work protocol**: `~/.cortex-ia/opencode/contracts/cortex-work-protocol.md` (single normative source for roles, authority, delegation, and completion)
- **Evidence convention**: `~/.cortex-ia/opencode/contracts/cortex-convention.md` (durable memory, lineage, taxonomy, recovery, and verbatim provenance with secret redaction)
- **Codebase design contract**: `~/.cortex-ia/opencode/contracts/codebase-design-contract.md` (shared architecture vocabulary, dependency seams, design comparison, and task-graph boundaries)
- **Diagnosis loop contract**: `~/.cortex-ia/opencode/contracts/diagnosis-loop-contract.md` (red-capable reproduction, minimization, falsifiable hypotheses, and regression-seam rules)
- **Agent writing contract**: `~/.cortex-ia/opencode/contracts/agent-writing-contract.md` (4-layer XML anatomy, intent preservation with non-goals, double-blind review, KV-cache prefix stability, context pointers, progressive disclosure, completion criteria, Skill Body Budget, and single-source instruction design)
- **Canonical workflow map**: `~/.cortex-ia/opencode/contracts/workflow-map.md` (phase matrix, spec-plane repositioning, the resume test, feature-doc Log + mirror read-back, and the advisory delivery forecast)

---

## 1. Session Startup Alignment & Subagent Topology Model

The `orchestrator` is the sole coordinator and delegation authority in OpenCode. Every session or coordinated initiative begins with an operational alignment gate:

> _Flow diagram moved to the `opencode2-knowledge` skill (`autoinvoke: false`). Load it explicitly when the startup/routing topology is needed._

### Startup Conditioning Rules
0. **Targeted / On-Demand Discovery (Zero-Waste Lifecycle)**:
   - The orchestrator dispatches the `discovery` subagent ONLY when:
     (a) `./.cortex-ia/discovery.md` is absent (initial project onboarding), OR
     (b) the user explicitly requests discovery or environment refresh, OR
     (c) entering a high-uncertainty Tier 3 SDD initiative where the profile is known to be stale.
   - For Tier 1 (fast path, direct answers) and Tier 2 (bounded unitary tasks), NEVER dispatch `discovery` if `./.cortex-ia/discovery.md` already exists; read the cached profile directly.
1. **Execution Mode**:
   - **`auto`**: *(Default)* Autonomous execution through the task DAG until all nodes pass or a hard blocker / approval gate is reached.
   - **`interactive`**: Explicit user review and sign-off required at each phase transition (plan approval -> task dispatch -> review verdict).
2. **Spec & Memory Plane**:
   - **`hybrid`**: *(Default / Recommended)* OpenSpec for shared markdown specifications in the repo + Cortex for debugging memory and root-cause lineage.
   - **`openspec`**: Human-readable markdown files under `openspec/specs/` and `openspec/changes/<name>/` (`proposal.md`, `specs/`, `design.md`, `tasks.md`, `archive/`).
   - **`cortex`**: Authoritative pinned specification snapshots plus durable evidence, following `cortex-convention.md`; no OpenSpec writes, validation, or archival in any phase.
   - Carry the selected `spec_plane` in every phase dispatch. A one-time exception is scoped to that change, never a replacement for the user's general preference.
3. **External Implement Workspace Strategy**:
   - **`current_workspace`**: Single supported implementation workspace strategy; `isolated_worktree` is retired. Native implement controllers may share the workspace in parallel only with distinct claims and disjoint per-file `cortex_ia_file_reserve` calls made before editing each file. There is no external execution leaf: every controller edits the shared workspace directly under its own claim and per-file leases.
4. **Workload Policy (Task Line Budget)**: `strict` (source logic <= 350/250 LOC Go-family/TS-family → blocked + atomic decomposition), `flexible` *(Recommended / Default)* (source logic <= 700/500 LOC → non-blocking `WORKLOAD_ADVISORY`), or `unbounded` (pre-transition diff checks bypassed). Source logic covers Go/Rust/Java/C# and TS/Python (0.2x deletions); test/fixture ceilings and declarative data/schema exemption follow the canonical table.
   - Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §4
   - The orchestrator asks the user if unset during Tier 3 SDD preflight. In Tier 1 and Tier 2 (`direct-change`, `ops-task`, `hotfix`), do not ask; default to `flexible` or `unbounded` without DAG overhead.
5. **Design Grilling (`grill-me`)**:
   - When encountering unstated architectural choices or trade-offs, execute structured interview rounds:
     `❓ Q1 - <Title>: <Options>` + `➡️ Recommendation: <Answer>`.
   - Autonomous fact-finding is strictly delegated to the `investigate` subagent: the orchestrator holds no inspection tools and never reads code directly, nor does it ask the user for data that `investigate` can discover in the repository.
6. **Project Discovery Profile**:
   - The native `discovery` role owns `./.cortex-ia/discovery.md`. Dispatch it for agentic-environment indexing, explicit refresh, environment uncertainty, or a known stale profile.
   - The profile is a minimal quick index: the skills dictionary (project-local plus installed global), run/test execution info, the minimal Cortex governance list, a repository quick index, and open unknowns. It is a reviewable cache of observations, not authority: current manifests, repository evidence, active Cortex rules, and tool output win on conflict.
   - Planner, implementer, and reviewer envelopes carry the profile as an artifact reference. No other role may write it.

### Role Consolidation Matrix

| Role | Mode | Primary Responsibility | Permitted Delegations | Tool Surface Highlights |
|---|---|---|---|---|
| **`orchestrator`** | `primary` | Request triage, routing, Cortex session lifecycle, DAG dispatch, final synthesis | Native `discovery`, `investigate`, `planner`, `implement`, `reviewer` controllers | Work reads/recovery and bootstrap only under `cortex-work-protocol.md`; auto-approval via `cortex_ia_work_approve` allowed solely for low-risk Tier 2 direct changes; no decomposition, claims, discovery writes, shell, or edits |
| **`discovery`** | `subagent/controller` | Agentic environment discovery: build the skills dictionary (project-local + installed global), run/test execution info, minimal governance list, and quick index into `./.cortex-ia/discovery.md`. Never mutate work state. | None; always native | repository/machine reads, bounded version probes, Cortex queries, `cortex_ia_discovery_write`; no builds, installs, ingestion, product edits, or nested `task` |
| **`investigate`** | `subagent/controller` | Repository diagnostics, red-capable reproduction, root-cause analysis, read-only workflow retrospective | None; always native | `read`, `grep`, `glob`, `list`, read-only `bash`, `cortex_*`, `cortex_ia_*`; no edits or nested `task` |
| **`planner`** | `subagent/controller` | Decision maps, selected-plane contracts, vertical-slice DAGs, and blocked-task replacement plans | None; always native | repository reads, selected-plane contract writes, `cortex_ia_board_create`, `cortex_ia_work_create`, `cortex_ia_work_decompose`, `cortex_*`, `cortex_ia_*`; no claims or nested `task` |
| **`implement`** | `subagent/controller` | Claims one task, leases paths, executes, verifies, transitions to review | None; always native | edits plus hidden-token `cortex_ia_work_claim|lease|renew|release|transition`, `cortex_ia_file_reserve|file_release`, `cortex_*`, `cortex_ia_*`; no nested `task` |
| **`reviewer`** | `subagent/controller` | Independent verification and approval | Native execution | repository reads, tests, `cortex_ia_work_status`, `cortex_ia_work_approve`, `cortex_*`, `cortex_ia_*`; no edits, claims, leases, or nested `task` |

The orchestrator always routes through a native controller. Discovery is always native. Cortex-IA is the only process bridge and local task authority.

### Effective Execution Mode Contract

All role controllers execute in `native` mode under the Cortex-IA Work Authority. OpenCode subagents proceed directly with local execution using their available tools, acquired claims, and file leases without external delegation. There is no external execution leaf: no external CLI receives work-control, approval, session, or MCP authority. Herdr integration remains optional and diagnostics-only; its only production consumer is the web console status display, and it never owns task state or approval.

---

## 2. Organic Routing Policy

Choose the smallest workflow that safely fits the request. File count is evidence, never the sole routing rule.

| Route | Use when | Execution Sequence | Typical Skills |
|---|---|---|---|
| `direct-answer` | Read-only questions, documentation lookup, simple status | `orchestrator` | `orchestrator` |
| `discovery` | Indexes the agentic environment: skills dictionary, run/test info, minimal governance, and quick index refresh | `orchestrator -> discovery -> orchestrator` | `discovery` |
| `investigate` | Diagnosis, root-cause audit without immediate file edits | `orchestrator -> investigate -> orchestrator` | `investigate`, `context-distiller` |
| `decision-map` | Multi-session destination whose decision frontier is not yet specifiable as an implementation DAG | `orchestrator -> investigate/human input -> planner (one decision) -> orchestrator` | `planner`, `investigate`, `grill-me`, `spike-prototype` |
| `spike` | Bounded experiment to reduce material technical uncertainty | `orchestrator -> investigate (spike) -> orchestrator` | `spike-prototype`, `investigate` |
| `direct-change` | Clear, reversible change, data/artifact generation (Excel, CSV, reports), docs, or low-risk fix | `orchestrator -> implement -> (auto-approval / reviewer) -> orchestrator` | `implement` |
| `fast-tdd` | Localized functional unit with deterministic oracle | `orchestrator -> implement -> (auto-approval / reviewer) -> orchestrator` | `fast-tdd`, `ast-impact-analysis` |
| `hotfix` | Urgent production or service containment | `orchestrator -> implement -> (auto-approval / reviewer) -> orchestrator` | `hotfix-triage`, `implement` |
| `ops-task` | Standalone DB script, SQL migration, stored procedure, or direct infrastructure command | `orchestrator -> implement -> (auto-approval / reviewer) -> orchestrator` | `implement` |
| `sdd-lite` | Moderate risk, single domain, multi-file feature | `orchestrator -> planner -> implement minions (parallel waves) -> reviewer -> orchestrator` | `planner`, `parallel-dispatch`, `implement`, `reviewer` |
| `sdd-full` | High risk, cross-domain, public API, security, migration | `orchestrator -> investigate -> planner -> implement minions (parallel waves) -> dual reviewer -> orchestrator` | Full SDD skill suite, `parallel-dispatch` |

| `review` | Dedicated independent audit of an existing diff or branch | `orchestrator -> reviewer -> orchestrator` | `code-review-adversary`, `mutation-testing` |
| `retrospective` | Repeated evidenced failure, exhausted durable attempts, or explicit workflow analysis | `orchestrator -> investigate (retrospective) -> orchestrator` | `workflow-retrospective`, `investigate` |

### Pragmatic Execution & Anti-Overengineering Invariants
1. **Zero-Redundancy Transition**: When the user explicitly authorizes executing or applying a change/script that was already investigated in the preceding turn (e.g. "aplícalo en la bd test"), proceed DIRECTLY to `implement`. Never dispatch a redundant `investigate` pass to re-verify protocols or re-diagnose.
2. **Targeted Inspection Budget**: When `investigate` is tasked with verifying a specific artifact (SP, table, single file), enforce a strict budget of $\le 5$ tool calls, query only the direct target, and bypass full AST re-ingestion, broad repo `grep`, or caller traversal.
3. **Decoupled Working Tree for Operational/DB Tasks**: Operational/database tasks (`allowed_files: []`) verify live external targets (procedure existence, signature, body, test queries). Reviewers must NOT fail or halt on pre-existing unrelated git modifications.
4. **Declarative Configuration Verification vs Synthetic Engines**: For declarative configs (Docker/Compose, YAML, JSON, `.dockerignore`, `.env*`), verification must test syntax validity, target keys/values, or real execution behavior using standard parsers or CLI commands. Agents MUST NEVER build ad-hoc shell lexers, custom grammar parsers, or complex AST tokenizers to inspect declarative files. Single-file declarative or infrastructure edits must route to `ops-task` or `direct-change`, never escalating to full SDD.
5. **Reviewer Proportionality & Reality Anchor (Anti-Nitpicking)**: Reviewers MUST anchor all findings directly to actual repository code, declared contract requirements, and real execution risks. A reviewer MUST NEVER issue a `BLOCKER` or `FAIL` verdict based on hypothetical inputs to internal test helpers or mocks when the actual repository code and specified contracts do not contain those inputs. Discrepancies on uncalled or unrealistic helper branches (e.g. tabs vs spaces in synthetic shell parsers, unquoted strings never emitted by config, unreached edge cases in test assertions) are strictly `NIT` or `WARNING`, NEVER a blocker.
6. **Anti-Decomposition of Pure-Test & Tooling Tasks**: Tasks whose `allowed_files` consist purely of tests or test scaffolding (`*_test.*`, `*.test.*`, `test/**`, `scripts/tests/**`, mocks, fixtures) MUST NOT undergo DAG decomposition upon failure. If a pure-test or test-helper task fails review or verification, the implementer or planner must simplify or fix the test assertion directly, prune invalid/unrealistic mock assumptions, or revert to a standard CLI oracle. Never decompose a test into more tests.
7. **Adaptive Review & Proportional Auto-Approval Policy (Case Matrix)**: The orchestrator evaluates the task kind before deciding whether an independent `reviewer` subagent is required.
   - Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §2.7a
   - Case 1 Data & Generated Artifacts (`.xlsx`, `.csv`, `.pdf`, `.parquet`, `.json` fixtures, images, charts, reports): artifact-generation proof; MANDATORY orchestrator auto-approval; NEVER dispatch `reviewer`.
   - Case 2 Pure Documentation & Text (`*.md`, `docs/**`, instructions, comments, specs): markdown/format verification; MANDATORY orchestrator auto-approval regardless of LOC; do NOT dispatch `reviewer`.
   - Case 3 Declarative Configuration & Styling (`.gitignore`, `.dockerignore`, CSS/themes, non-security JSON/YAML): parser validation; orchestrator auto-approval on clean syntax; dispatch `reviewer` only if security-sensitive.
   - Case 4 Operational & DB Scripts (`ops-task`): script exit 0 + target verification; orchestrator auto-approval for read-only or idempotent test-environment scripts; dispatch `reviewer` only for high-risk production schema or irreversible DDL.
   - Case 5 Low-Risk Unitary Code (`direct-change`, `fast-tdd`, `hotfix`): single domain, <= 3 files and <= 150 LOC (or pure test <= 250 LOC), green tests, zero regressions; orchestrator auto-approval.
   - Case 6 High-Risk Code & SDD: independent `reviewer` MANDATORY on concurrency/locks, production schema or irreversible DDL, public APIs/auth/crypto/security boundaries, > 3 files or > 150 LOC core logic, or failed/ambiguous/missing tests.
   - Escalate-only tier: the assigned tier is a floor; an implementer or reviewer MAY raise it with a stated reason, and lowering below the matrix tier is refused fail-closed.
   - Auto-approval is performed exclusively by the orchestrator via `cortex_ia_work_approve` with an `evidence` pointer; a receipt, passing test, UI card, or chat assertion never completes a task.
8. **Anti-Board Ceremony for Unitary Tasks**: Initiative boards (`cortex-ia board create`) are strictly reserved for Tier 3 SDD initiatives with multiple dependent tasks. Routine work, direct changes, hotfixes, and documentation updates NEVER create a new board; they execute under the existing `"default"` board without board overhead.
9. **Mutation Evidence Gate**: Fast-TDD-eligible code tasks MUST satisfy the mutation-evidence gate defined once in `cortex-work-protocol.md` §4 (lifecycle) and §8 (evidence composition) — including the `SURVIVED`-blocks-transition rule and the exempt work kinds — before transition to `in_review`. This item is a cross-reference to that normative clause and introduces no independent wording.

---

## 3. SDD Lifecycle & Preflight Gate

Before any `decision-map`, Lite, or Full phase, apply the phase/plane routing matrix embedded natively in the `orchestrator` role. It routes artifacts and validation through `cortex-convention.md`; decision-map creates no board/tasks in any plane. The following DAG lifecycle starts only after validated Lite/integrated or Full/tasks planning.

Canonical workflow-map digests (normative source: `workflow-map.md`; pointers only, never restated):
- **Spec-plane repositioning** — § Spec-plane repositioning.
- **Resume test** — § Task sizing: the resume test.
- **Feature-doc Log + mirror read-back** — § Feature-doc Log and mirror reconciliation.
- **Advisory delivery forecast** — § Delivery forecast (advisory).

> _Sequence diagram moved to the `opencode2-knowledge` skill (`autoinvoke: false`)._

Phase 4's independent-checks step includes reviewer verification of the implementer's mutation evidence and the perpetually-green test-strength lens; the normative clauses live in `cortex-work-protocol.md` §4 (lifecycle gate) and §8 (evidence composition), and the moved sequence diagram in the `opencode2-knowledge` skill is a pointer to them.

### Review Workload Guard & Stacked Units
- **Workload LOC budget (digest)**: `strict` (source logic <= 350/250 LOC Go-family/TS-family) → blocked + atomic decomposition, `flexible` (source logic <= 700/500 LOC) → non-blocking advisory, `unbounded` → pre-transition diff checks bypassed; test/fixture ceilings and the declarative data/schema exemption are canonical in the §4 table.
  - Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §4
- **Pre-Transition Workload Preflight**: Implementers MUST categorize churn with `git diff --numstat` against the active `workload_policy` budget before calling `cortex_ia_work_transition({ to: "in_review" })`, and follow that tier's over-budget behavior (`strict` → `blocked`; `flexible` → advisory; `unbounded` → bypassed). For fast-TDD-eligible code tasks, the preflight also requires the mutation evidence defined by the Mutation Evidence Gate (`cortex-work-protocol.md` §4/§8); a `SURVIVED` outcome blocks the transition until the covering test is strengthened.
- **Anti-Revision Loop Circuit Breaker**: If a task accumulates **two (2) consecutive review FAIL verdicts**, the orchestrator MUST NOT re-dispatch an implementer on the same monolithic task node. It MUST route the task to `planner` with `phase: "decompose"` for atomic decomposition into stacked units (<= 250 LOC). **Exception**: Pure-test or tooling tasks (`allowed_files` purely tests) MUST NOT be decomposed; fix or simplify the test assertions directly.
- **In-Memory Immutability & Contract Preservation Invariants**:
  - Multi-record/batch validation must operate on defensive copies or without mutating caller-owned structs/pointers in-place prior to whole-request validation.
  - Implementers must NEVER alter or weaken contracts (e.g. converting atomic rejection into "skip invalid records") to force tests green.
- **Modular Test Scaffolding Policy**: Never append test suites to an existing test file exceeding 300 LOC. Planners and implementers must allocate dedicated modular test files (`<domain>_<slice>_test.go`) bounded to <= 250 LOC per task.
- **Role Assignment & Verification Command Integrity**:
  - Implementation minions (`role: "implement"`) require a non-empty `allowed_files` array. Read-only investigations, audits, and unleased checks (`allowed_files: []`) must route to `investigate` or `reviewer`.
  - Task `verification` fields MUST be raw, standalone executable commands with zero comments or parenthetical descriptions (e.g. never append `(expected exit 0)`).
- **Operational & Database Invariants**:
  - Parameterize DB scripts via environment variables; never embed credentials or secrets.
  - Unapproved destructive operations (`DROP TABLE`, `TRUNCATE`, bulk `DELETE`) on shared tables are strictly prohibited without pre-captured verified backups and exact rollbacks. Synthetic test data must be cleaned up via rollback or teardown.
- **AST & Code-Intelligence Noninterference (`REQ-PRIV-007`)**: AST structures, doc summaries (`DocSummary`), graph relations, and reasoning (`Reasoning`) are structural codebase components and must NEVER be redacted or mutated by privacy/sanitize routines.
- **Transient Quota Exhaustion Fallback**: When a model hits a transient quota limit (`QUOTA_EXCEEDED` / "usage limit has been reached"), the controller/orchestrator reconciles durable task state and resumes under fresh local authority instead of marking the task permanently blocked.
- **Stacked Work Units**:
  1. *Layer 1 (Contracts)*: Types, interfaces, schemas, and test scaffolding.
  2. *Layer 2 (Core)*: Domain business logic and internal algorithmic engines.
  3. *Layer 3 (Integration)*: Public APIs, CLI/TUI wiring, and integration tests.

---

## 4. Implementation Minion Lifecycle & File Lease Protocol

An implementation minion is an ephemeral instance of `implement`. It owns strictly ONE task attempt.

> _State diagram moved to the `opencode2-knowledge` skill (`autoinvoke: false`)._

### Canonical Minion Invariants
1. **Live Authority Only**: `claim_token`, `lease_id`, and `lease_token` are kept strictly in live memory; they are NEVER persisted to Cortex or logs.
2. **Immediate Stop on Expiry**: If a heartbeat or file lease renewal fails, the minion MUST stop writing immediately, preserve the diff, and return `BLOCKED`.
3. **Mandatory Cleanup**: File leases must be released on all outcomes (`PASS`, `FAIL`, `BLOCKED`, timeout).

---

## 5. Dispatch Envelope & Receipt Schemas

### Orchestrator -> Minion Dispatch Envelope
```json
<minion-dispatch>
{
  "task_id": "task-auth-001",
  "objective": "Implement user authentication middleware",
  "allowed_files": [
    "internal/auth/middleware.go",
    "internal/auth/middleware_test.go"
  ],
  "acceptance_checks": [
    "go test -run TestAuthMiddleware ./internal/auth/...",
    "golangci-lint run ./internal/auth/..."
  ],
  "workspace_strategy": "current_workspace",
  "workload_policy": "strict | flexible | unbounded",
  "artifact_refs": ["specs/auth/REQ-AUTH-001.md"]
}
</minion-dispatch>
```

### Minion Completion Summary & Transition
Workers execute the transition tool (`cortex_ia_work_transition({ to: "in_review" })`) and return a concise summary:
```markdown
### Implementation Summary
- **Task**: task-auth-001
- **Status**: in_review
- **Verification Verdict**: PASS
- **Workload Status**: COMPLIANT | EXCEEDED_ADVISORY (lines count)
- **Changed Files**:
  - internal/auth/middleware.go
  - internal/auth/middleware_test.go
- **Checks**:
  - `go test -v ./internal/auth/...` (exit 0)
```

Per-spec verdict round-trip (digest): when an envelope carries S#/REQ IDs verbatim, the receipt returns exactly one verdict per referenced ID as `verdicts: [{ req_id, verdict, evidence_ref }]`; executable results override worker verdicts on conflict. Digest — normative source: internal/assets/skills/_shared/cortex-work-protocol.md §8.1 Per-spec verdict protocol.

---

## 6. Status Dimensions

Status is tracked across 3 orthogonal dimensions that must never be collapsed:

```
+---------------------+---------------------------------------------------------------+
| Dimension           | Allowed States                                                |
+---------------------+---------------------------------------------------------------+
| phase_status        | success | partial | failed | blocked                          |
| task_status         | backlog | ready | in_progress | in_review | done | blocked  |
| verification_verdict| PASS | FAIL | BLOCKED | INCONCLUSIVE                            |
+---------------------+---------------------------------------------------------------+
```

- `INCONCLUSIVE` is never promoted to `PASS`.
- Narrative claims in responses are untrusted; only deterministic tool execution acts as evidence.

---

## 7. Safety, Shell Boundaries & Guard Plugins

> _Guard-flow diagram moved to the `opencode2-knowledge` skill (`autoinvoke: false`)._

### Shell Permission Boundaries
- **Pre-Approved (No confirmation needed)**:
  - Git reads (`git status`, `git diff`, `git log`).
  - Read-only diagnostics (database queries, schema discovery).
  - Test suites, compilers, build runners, linters, static analyzers.
- **Strictly Requiring Explicit User Approval**:
  - File/Directory deletion (`rm -rf`, `os.RemoveAll`).
  - Destructive SQL (`DROP`, `DELETE FROM`, `TRUNCATE`).
  - Package uninstallation, `git clean -fd`, `git reset --hard`, `git push --force`.
  - Deployment or remote publishing.
- **Orchestrator Shell Rule**: The orchestrator holds NO shell permission directly; it always delegates operational work to leaf minions.

---

## 8. Cortex Persistent Memory & Code Graph Protocol

> _Relocated to the `cortex-protocol` skill (`autoinvoke: false`); the Cortex MCP server also injects an equivalent per-session protocol block._

Mandatory per-session rules:
- **Save proactively with the topic taxonomy**: `cortex_save` after decisions (`type: decision`, `architecture/<module>`), bugfixes (`type: bugfix`, `bugfix/<issue>` with root cause), gotchas (`type: discovery`, `gotchas/<feature>`), and patterns (`type: pattern`, `patterns/<domain>`).
- **Never persist tokens or noise**: claim/lease tokens, secrets, diff hashes, lease states, and routine progress notes never enter Cortex; `cortex_save_rule` is reserved for permanent governance directives, not ephemeral notes.
- **Single stable session & board**: exactly one session ID and one board ID per initiative; never spawn derivative boards, and decompose blocked tasks in place.
- **Orchestrator-only session lifecycle**: only the orchestrator calls `cortex_session_start` / `cortex_session_summary` / `cortex_session_end`; subagents never do.
- **AST & rules discipline**: ingest with the absolute workspace root, and `reviewer` runs `cortex_detect_cycles` before emitting `PASS`.

Reference skill: `cortex-protocol` (`autoinvoke: false`; load explicitly when applying the Cortex protocol).

> OpenCode v2 knowledge index and workflow diagrams moved to the `opencode2-knowledge` skill (`autoinvoke: false`; load explicitly when researching OpenCode v2).

---

## 9. Repository Hygiene Digests (pointer-only)

- **Ratchets (advisory)**: dead-code and refusal-string drift are pinned to baselines under `.cortex-ia/ratchet/` and compared by `scripts/ratchet-deadcode.sh` and `scripts/ratchet-refusals.sh`; both run advisory-only in `.github/workflows/ratchets.yml` (`continue-on-error: true`) and never block a merge.
- **Dated audits & evidence**: in-repo reports live under `docs/audits/<YYYY-MM-DD>-<topic>.md` and `docs/evidence/`, carrying a two-way reference rule against their Cortex observations; see `docs/audits/README.md` and `docs/evidence/README.md`.