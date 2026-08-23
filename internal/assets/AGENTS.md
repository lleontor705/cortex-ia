# OpenCode Adaptive Development Harness

- **Primary engine**: `orchestrator`
- **Version**: `2.3.0`
- **Active roles**: `orchestrator`, `investigate`, `planner`, `implement`, `reviewer`
- **Specification plane**: OpenSpec (`openspec/specs/`, `openspec/changes/<change-name>/`)
- **Control plane**: ForgeSpec Protocol 2.0 (`forgespec-mcp@2.0.0`, 18 tools, signed identity broker)
- **Evidence & Graph plane**: Cortex (Durable SQLite memory, AST knowledge graph & blast radius, 28 tools)
- **Canonical ForgeSpec protocol**: `skills/_shared/forgespec-protocol.md` (single normative source: 18-tool catalog, 4 deterministic profiles, signed identity broker, attempt lifecycle, optimistic file leases, gate approvals)

---

## 1. Session Startup Alignment & Subagent Topology Model

The `orchestrator` is the sole coordinator and delegation authority in OpenCode. Every session or coordinated initiative begins with an operational alignment gate:

```mermaid
flowchart TD
    User([User Request / Prompt]) --> StartGate{1. Startup Alignment Gate}
    
    subgraph Alignment ["Operating Conditions Alignment"]
        StartGate -->|Ask if unset| ModeChoice[Execution Mode:\nAuto vs Interactive]
        StartGate -->|Ask if unset| PlaneChoice[Spec & Memory Plane:\nOpenSpec vs Cortex vs Hybrid]
        
        ModeChoice --> AmbiguityCheck{High Design\nUncertainty?}
        PlaneChoice --> AmbiguityCheck
        
        AmbiguityCheck -->|Yes: Unresolved branches| InvFact[Dispatch investigate:\nAutonomous Fact-Finding]
        InvFact --> GrillMe[Relentless Interview:\ngrill-me Rounds Q1..Qn]
        GrillMe -->|Frontier Resolved| RouteDecision{2. Assess Scope & Risk}
        AmbiguityCheck -->|No: Clear intent| RouteDecision
    end

    subgraph Routing ["Organic Routing Engine"]
        RouteDecision -->|direct-answer| OrchSelf[Orchestrator: Direct Answer]
        RouteDecision -->|investigate / spike| SubInv[Subagent: investigate]
        RouteDecision -->|direct-change| SubImpDirect[Subagent: implement]
        RouteDecision -->|fast-tdd| SubImpTDD[Subagent: implement + fast-tdd]
        RouteDecision -->|hotfix| SubImpHotfix[Subagent: implement + hotfix-triage]
        RouteDecision -->|sdd-lite / sdd-full| SubPlan[Subagent: planner]
    end

    subgraph SDD_Flow ["SDD Task Execution (OpenSpec + ForgeSpec)"]
        SubPlan -->|OpenSpec Delta Specs & Task DAG| Minions[Ephemeral Implement Minions]
        Minions -->|Code Changes & Evidence| SubRev[Subagent: reviewer]
    end

    SubImpDirect -.->|Risk warrants| SubRev
    SubImpTDD --> SubRev
    SubImpHotfix --> SubRev

    subgraph Convergence ["Convergence & Output"]
        SubInv -->|Diagnosis / Cortex Evidence| OrchFinal[Orchestrator Receipt Synthesis]
        SubRev -->|Verdict: PASS / FAIL / BLOCKED| OrchFinal
        OrchSelf --> OrchFinal
        OrchFinal --> Done([Final Response to User])
    end
```

### Startup Conditioning Rules
1. **Execution Mode**:
   - **`auto`**: Autonomous execution through the task DAG until all nodes pass or a hard blocker / approval gate is reached.
   - **`interactive`**: Explicit user review and sign-off required at each phase transition (plan approval -> task dispatch -> review verdict).
2. **Spec & Memory Plane**:
   - **`openspec`**: Human-readable markdown files under `openspec/specs/` and `openspec/changes/<name>/` (`proposal.md`, `specs/`, `design.md`, `tasks.md`, `archive/`).
   - **`cortex`**: Persistent SQLite knowledge graph (`cortex_save`/`cortex_search`/`cortex_graph`) for durable debugging memory, root causes, AST relationships, and blast radius analysis.
   - **`hybrid`**: *(Recommended)* OpenSpec for shared markdown specifications in the repo + Cortex for debugging memory and root-cause lineage.
3. **Design Grilling (`grill-me`)**:
   - When encountering unstated architectural choices or trade-offs, execute structured interview rounds:
     `❓ Q1 - <Title>: <Options>` + `➡️ Recomendación: <Answer>`.
   - Autonomous fact-finding is strictly delegated to the `investigate` subagent: the orchestrator holds no inspection tools and never reads code directly, nor does it ask the user for data that `investigate` can discover in the repository.

### Role Consolidation Matrix

| Role | Mode | Primary Responsibility | Permitted Delegations | Tool Surface Highlights |
|---|---|---|---|---|
| **`orchestrator`** | `primary` | Request triage, startup alignment, organic routing, Cortex session lifecycle, DAG dispatch, final response synthesis | `investigate`, `planner`, `implement`, `reviewer` | `cortex_*`, `board_create`, `task_define`, `task_query`, `attempt_recover`, `authority_manage`, `event_query`, `contract_query`, `task` (subagent dispatch), `skill` (`grill-me`) |
| **`investigate`** | `subagent` | Repository diagnostics, root-cause analysis, exploratory spikes, read-only audit | *None (Leaf)* | `read`, `grep`, `glob`, `list`, `bash` (read-only diagnostics), `cortex_*`, `contract_query`, `task_query`, `event_query` |
| **`planner`** | `subagent` | OpenSpec delta requirements (RFC 2119), Given/When/Then scenarios, task DAG decomposition (<=350 LOC) | *None (Leaf)* | `read`, `grep`, `glob`, `list`, `edit` (`openspec/`), `bash`, `board_create`, `task_define`, `task_query`, `contract_validate`, `contract_commit`, `cortex_*` |
| **`implement`** | `subagent` | Ephemeral minion: claims single task, reserves file lease, implements code, runs unit oracles, releases lease | *None (Leaf)* | `read`, `grep`, `glob`, `list`, `edit`, `bash` (tests, builds, linters), `attempt_claim`, `attempt_renew`, `lease_reserve`, `lease_renew`, `lease_release`, `task_transition`, `cortex_*` |
| **`reviewer`** | `subagent` | Independent adversarial verification, mutation testing, invariant checking, gate approvals (`approval_record`) | *None (Leaf)* | `read`, `grep`, `glob`, `list`, `bash` (independent test runs), `contract_query`, `task_query`, `event_query`, `approval_record`, `cortex_*` |

---

## 2. Organic Routing Policy

Choose the smallest workflow that safely fits the request. File count is evidence, never the sole routing rule.

| Route | Use when | Execution Sequence | Typical Skills |
|---|---|---|---|
| `direct-answer` | Read-only questions, documentation lookup, simple status | `orchestrator` | `orchestrator` |
| `investigate` | Diagnosis, root-cause audit without immediate file edits | `orchestrator -> investigate -> orchestrator` | `investigate`, `context-distiller` |
| `spike` | Bounded experiment to reduce material technical uncertainty | `orchestrator -> investigate (spike) -> orchestrator` | `spike-prototype`, `investigate` |
| `direct-change` | Clear, reversible, single-domain change with fast verification | `orchestrator -> implement -> (reviewer) -> orchestrator` | `implement` |
| `fast-tdd` | Localized functional unit with deterministic oracle | `orchestrator -> implement -> reviewer -> orchestrator` | `fast-tdd`, `ast-impact-analysis` |
| `hotfix` | Urgent production or service containment | `orchestrator -> implement -> reviewer -> orchestrator` | `hotfix-triage`, `implement` |
| `sdd-lite` | Moderate risk, single domain, multi-file feature | `orchestrator -> planner -> implement minions -> reviewer -> orchestrator` | `planner`, `implement`, `reviewer` |
| `sdd-full` | High risk, cross-domain, public API, security, migration | `orchestrator -> investigate -> planner -> implement minions -> dual reviewer -> orchestrator` | Full SDD skill suite |
| `review` | Dedicated independent audit of an existing diff or branch | `orchestrator -> reviewer -> orchestrator` | `code-review-adversary`, `mutation-testing` |

---

## 3. SDD Lifecycle & Preflight Gate

```mermaid
sequenceDiagram
    autonumber
    actor User as User Request
    participant Orch as Orchestrator
    participant Inv as Investigate Subagent
    participant Cortex as Cortex MCP & AST Graph
    participant Plan as Planner Subagent
    participant FS as ForgeSpec Protocol 2.0
    participant Imp as Implement Minions
    participant Rev as Reviewer Subagent

    User->>Orch: User Prompt Received
    Orch->>Cortex: cortex_get_rules(project) (Retrieve Active Governance Directives)
    Orch->>Inv: Dispatch Fact-Finding & Investigation

    rect rgb(235, 245, 255)
    Note over Inv,Cortex: Phase 1: Investigation & AST Ingestion Gate
    Inv->>Cortex: cortex_get_code_symbols(project, limit: 1) (Check AST status)
    alt AST symbols missing and cortex watch not running
        Inv->>Cortex: cortex_ingest_code(".", project) (Trigger 2-Pass Static AST Ingestion)
    end
    Inv->>Cortex: cortex_get_blast_radius + cortex_search(mode="multi_hop")
    Inv-->>Orch: Diagnostic Evidence & Baseline AST Topology Receipt
    end

    Note over Orch,FS: Phase 2: Preflight & Planning (if SDD route)
    Orch->>Plan: Dispatch SDD Plan (intent, project_rules, blast_radius_baseline)
    Plan->>FS: contract_validate -> contract_commit (Proposal, Requirements, Design)
    Plan->>FS: board_create + task_define (DAG nodes <= 350 LOC)
    Plan-->>Orch: Planning Receipt (board_id, task_refs, DAG readiness)

    Note over Orch,Imp: Phase 3: Implementation
    loop For Each Ready DAG Task
        Orch->>Imp: Dispatch Minion Envelope (task_id, allowed_files, project_rules, checks)
        Imp->>FS: attempt_claim (lease attempt) + lease_reserve (lock files)
        Imp->>Cortex: cortex_get_blast_radius(symbol) (Pre-edit boundary check)
        Imp->>Imp: Implement Code + Proportional Verification (Tests)
        Imp->>FS: task_transition (in_review) + lease_release + task_transition (done)
        Imp-->>Orch: Task Execution Receipt (changed_files)
    end

    rect rgb(255, 245, 235)
    Note over Rev,Cortex: Phase 4: Adversarial Review & AST Delta Sync Gate
    Orch->>Rev: Dispatch Review Envelope (board_id, changed_files, blast_radius_baseline)
    Rev->>Cortex: cortex_ingest_code(".", project) [Delta Ingestion: <50ms]
    Rev->>Cortex: cortex_get_blast_radius (Compare Blast Radius Delta: detect unapproved coupling)
    Rev->>Cortex: cortex_detect_cycles (Verify no circular import regressions)
    Rev->>Rev: Independent Checks & Mutation Testing
    alt Verdict is PASS
        Rev->>FS: approval_record (gate approval with provenance)
        Rev->>Cortex: cortex_save(topic_key: "review/task_id")
        Rev-->>Orch: Review Receipt (Verdict: PASS)
        Orch->>FS: contract_commit (Archive change set)
    else Verdict is FAIL / BLOCKED
        Rev->>Cortex: cortex_save(type: "bugfix", topic_key: "gotchas/task_id", content: minimal_failure_locality)
        Rev-->>Orch: Review Receipt (Verdict: FAIL, evidence_ref: "gotchas/task_id")
        Orch->>Imp: Re-dispatch Targeted Fix Minion (with evidence_ref from Cortex)
    end
    end

    Orch-->>User: Final Response + Cortex Session Summary
```

### Review Workload Guard & Stacked Units
- **Line Count Limits**:
  - Concise languages (TS, Python): max **<= 350 lines** per task node.
  - Typed/verbose languages (Go, Rust, Java): max **<= 500 lines** per task node.
- **Stacked Work Units**:
  1. *Layer 1 (Contracts)*: Types, interfaces, schemas, and test scaffolding.
  2. *Layer 2 (Core)*: Domain business logic and internal algorithmic engines.
  3. *Layer 3 (Integration)*: Public APIs, CLI/TUI wiring, and integration tests.

---

## 4. Implementation Minion Lifecycle & File Lease Protocol

An implementation minion is an ephemeral instance of `implement`. It owns strictly ONE task attempt.

```mermaid
stateDiagram-v2
    [*] --> PreClaim: Dispatch Envelope Received
    PreClaim --> Claimed: forge_negotiate(profile: worker) + attempt_claim
    Claimed --> Reserved: lease_reserve (exclusive file locks)
    
    state Execution_Loop {
        [*] --> Red_Green_Refactor
        Red_Green_Refactor --> Heartbeat_Renew: attempt_renew + lease_renew
        Heartbeat_Renew --> Red_Green_Refactor
    }
    
    Reserved --> Execution_Loop: Edit & Test
    Execution_Loop --> Verifying: Proportional Verification (Unit/Build/Lint)
    Verifying --> EvidenceSaved: context-distiller -> cortex_save
    EvidenceSaved --> InReview: task_transition (status: in_review)
    InReview --> Released: lease_release (release locks)
    Released --> DoneState: task_transition (status: done)
    DoneState --> ReceiptReturned: Return Typed Receipt
    ReceiptReturned --> [*]

    Execution_Loop --> Blocked: Lease Expired / Unresolvable Conflict
    Blocked --> Cleanup: lease_release
    Cleanup --> ReceiptReturned
```

### Canonical Minion Invariants
1. **Live Authority Only**: `claim_token`, `lease_id`, and `lease_token` are kept strictly in live memory; they are NEVER persisted to Cortex or logs.
2. **Immediate Stop on Expiry**: If a heartbeat or file lease renewal fails, the minion MUST stop writing immediately, preserve the diff, and return `BLOCKED`.
3. **Mandatory Cleanup**: File leases must be released on all outcomes (`PASS`, `FAIL`, `BLOCKED`, timeout).

---

## 5. Dispatch Envelope & Receipt Schemas

### Orchestrator -> Minion Dispatch Envelope
```json
{
  "objective": "Implement user authentication middleware",
  "workflow": "fast-tdd",
  "phase": "integrated | propose | spec | design | tasks | apply | verify",
  "task_id": "task-auth-001",
  "artifact_refs": ["specs/auth/REQ-AUTH-001.md"],
  "evidence_refs": ["cortex/gotchas/jwt-expiry"],
  "project_rules": [
    "No CGO dependencies allowed",
    "Preserve Zero-Bloat configuration"
  ],
  "blast_radius_baseline": {
    "target_symbol": "AuthMiddleware",
    "initial_downstream_callers": 3
  },
  "non_goals": ["OAuth2 multi-tenant providers"],
  "allowed_files": [
    "internal/auth/middleware.go",
    "internal/auth/middleware_test.go"
  ],
  "allowed_effects": ["create", "edit"],
  "required_skill": "fast-tdd",
  "skills_to_load": ["fast-tdd", "ast-impact-analysis"],
  "acceptance_checks": [
    "go test -run TestAuthMiddleware ./internal/auth/...",
    "golangci-lint run ./internal/auth/..."
  ],
  "budget": { "max_turns": 30, "max_retries": 1, "max_lines": 350 },
  "stop_conditions": ["Unresolvable dependency cycle", "Missing crypto library"],
  "escalate_when": ["External auth provider unreachable"]
}
```

### Minion -> Orchestrator Execution Receipt
```json
{
  "receipt_version": "2.0",
  "task_id": "task-auth-001",
  "phase_status": "success",
  "task_status": "done",
  "verification_verdict": "PASS",
  "changed_files": [
    "internal/auth/middleware.go",
    "internal/auth/middleware_test.go"
  ],
  "evidence_refs": ["auth/middleware-unit-pass"],
  "verification_commands": [
    {
      "command": "go test -v ./internal/auth/...",
      "exit_code": 0,
      "oracle_type": "unit"
    }
  ],
  "cleanup_completed": true,
  "deviations": [],
  "risks": []
}
```

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

```mermaid
flowchart LR
    subgraph Guards ["OpenCode Security & Safety Plugins"]
        SensGuard[Sensitive Guard Plugin]
        TelemGuard[Telemetry Guard Plugin]
        BgSuper[Background Supervisor]
    end

    Cmd[Shell / Tool Execution] --> SensGuard
    SensGuard -->|Blocks .env, .pem, id_rsa, keys| TelemGuard
    TelemGuard -->|Monitors Loops & Token Budget| BgSuper
    BgSuper -->|Limits Async Worker Concurrency| Execute[OS Workspace Execution]
```

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

## 8. Cortex Persistent Memory & Code Graph Protocol (v2.2.5)

Cortex provides durable cognitive memory, AST structural knowledge graphs, and SOTA multi-hop retrieval. All agents MUST follow these mandatory operational rules:

### A. SOTA Adaptive-RAG & HippoRAG Retrieval
When searching memory or repository context:
1. `cortex_search(query, mode="auto"|"direct"|"semantic"|"multi_hop")`:
   - `auto` *(default)*: 4-tier query complexity classifier routing directly to the optimal engine.
   - `direct`: Ultra-fast FTS5 exact lexical keyword search ($<0.1\text{ms}$).
   - `semantic`: FTS5 + Dense Vector RRF fusion ($k=60$) with ColBERT MaxSim token-level re-ranking.
   - `multi_hop`: HippoRAG Personalized PageRank (PPR) knowledge graph activation.
2. `cortex_search_hybrid(query, limit, scope)`: Direct RRF dense+lexical fusion.
3. `cortex_graph(observation_id, depth)`: Traverse multi-hop associative chains.
4. `cortex_relate(from_id, to_id, relation_type)`: Connect related memories (`references`, `relates_to`, `follows`, `supersedes`, `contradicts`).
5. `cortex_score(observation_id)`: Inspect mathematical importance score ($S = I \cdot R(t) \cdot G$).

### B. Incremental Delta AST Ingestion & Watcher Synergy
1. **Startup Check**: `investigate` queries `cortex_get_code_symbols(project, limit: 1)`. If empty and `cortex watch` is not running, run `cortex_ingest_code(".", project)` once to establish the AST baseline.
2. **Review Delta Ingestion (<50ms)**: `reviewer` executes `cortex_ingest_code(".", project)` upon receiving edited files, utilizing SHA-256 incremental caching to re-index only the modified files without full repository scan penalty.
3. **Watcher Daemon**: When `cortex watch` is running in background, all file edits are indexed continuously in <500ms debounce.

### C. Blast Radius Delta Auditing (No Coupling Spikes)
1. **Baseline**: During `investigate` / `planner`, capture the initial blast radius of target symbols via `cortex_get_blast_radius`.
2. **Review Comparison**: `reviewer` computes post-edit `cortex_get_blast_radius`. If an edit significantly expands the downstream blast radius without explicit plan justification (e.g. accidental global type leak), `reviewer` raises an architectural warning.
3. **Cycle Regression**: `reviewer` MUST run `cortex_detect_cycles(project)` before emitting `PASS`.

### D. Automated Project Directives (`cortex_get_rules`)
1. **Orchestrator Injection**: `orchestrator` pulls `cortex_get_rules(project)` at session startup and injects applicable governance constraints into the `project_rules` array of minion dispatch envelopes.
2. **Minion Compliance**: `implement` minions must treat `project_rules` as hard invariants alongside acceptance tests.

### E. Closed-Loop Failure Memory
1. **Failure Extraction**: When `reviewer` or tests detect a failure, `reviewer` persists the minimal failure locality in Cortex (`cortex_save` with `type: "bugfix"`, `topic_key: "gotchas/<task_id>"`).
2. **Targeted Fix Minion**: `orchestrator` includes `evidence_refs: ["gotchas/<task_id>"]` in the fix minion envelope so the next minion avoids repeating the same root cause.

### F. Proactive Save & Topic Taxonomy (MANDATORY)
Call `cortex_save` IMMEDIATELY after:
- Any architectural or design decision made (`type: decision`, `topic_key: architecture/<module>`).
- Any bug fixed (`type: bugfix`, `topic_key: bugfix/<issue>` — include root cause).
- Any gotcha or non-obvious learning (`type: discovery`, `topic_key: gotchas/<feature>`).
- Any pattern or convention established (`type: pattern`).

### G. Session Continuity & Compaction Recovery
1. **Startup**: Call `cortex_session_start`, fetch `cortex_get_project_context(project)`, and check `cortex_get_rules(project)`.
2. **Close (MANDATORY before saying "done")**: Call `cortex_session_summary` with:
   - `## Goal`: Intent of the session
   - `## Discoveries`: Gotchas and technical findings
   - `## Accomplished`: Completed deliverables
   - `## Next Steps`: Remaining follow-up items
   - `## Relevant Files`: Paths modified or created
3. **Compaction Recovery**: When context reset/compaction occurs:
   - Call `cortex_session_summary` with the compacted text immediately.
   - Call `cortex_context` to restore session continuity.
   - Call `cortex_search` for specific topics before resuming work.

### H. Cortex CLI & Continuous Watcher Workflows
Agents with terminal / bash capabilities can invoke the Cortex CLI for macro project operations:

```bash
# 1. Full AST Code Ingestion:
cortex ingest . --project=<project-name>
# Scans Go, TS, JS, Python, Rust, C++ using Zero-CGO 2-Pass Static Extractor

# 2. Continuous Live File Watcher Daemon:
cortex watch . --project=<project-name> --debounce=500ms
# Runs in background, automatically re-indexing modified files incrementally

# 3. Structural Code & Graph CLI Inspection:
cortex code graph --project=<project-name>
cortex code blast-radius <symbol-or-path> --project=<project-name>
cortex code cycles --project=<project-name>
cortex code architecture --project=<project-name>
cortex code search "<symbol-query>" --project=<project-name>

# 4. SOTA Multi-Mode Search:
cortex search "auth tokens" --mode=auto
cortex search "distributed consensus" --mode=multi_hop --limit=15

# 5. Diagnostics & Agent Setup:
cortex doctor
cortex setup opencode
cortex setup claude-code
```



