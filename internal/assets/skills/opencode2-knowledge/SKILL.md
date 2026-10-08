---
name: opencode2-knowledge
description: OpenCode v2 (opencode2) knowledge index and harness flow diagrams — official docs mapping, live CLI inspection helpers, project developer skills, and the startup/SDD/minion/guard mermaid diagrams. Load explicitly when researching or debugging OpenCode v2 platform capabilities.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
  opencode/autoinvoke: false
---

# OpenCode v2 Knowledge Index & Harness Flow Diagrams

Relocated from `internal/assets/AGENTS.md` (the OpenCode v2 knowledge index §I and the §1/§3/§4/§7 mermaid flow diagrams) so the per-session harness injection stays lean. The skill registers `metadata.opencode/autoinvoke: false`: load it explicitly by ID when researching or debugging OpenCode v2 platform capabilities or when a harness flow diagram is needed.

## OpenCode v2 (`opencode2`) Knowledge & Search Index

When researching, developing, or debugging capabilities for **OpenCode v2 (`opencode2`)**, use this canonical index to locate authoritative specifications, official documentation, and local inspection commands.

### 1. Official Documentation Mapping

| Topic & URL | Scope & Key Concepts | When to Consult |
| :--- | :--- | :--- |
| **[Core Docs](https://opencode.ai/v2/docs/)** | Core runtime architecture, Daemon/Server model, File hierarchy (`.config/opencode/` vs `.opencode/`), Precedence & merging rules, `opencode.jsonc` schema, Permissions array format (`[{ action, resource, effect }]`). | When designing configuration templates, setting permissions, or understanding directory precedence. |
| **[CLI & TUI](https://opencode.ai/v2/docs/cli/)** | Global CLI commands, TUI navigation (`opencode2`), `cli.json` configuration, Theme switching (`/themes`), Keybindings, Terminal Truecolor requirement (`COLORTERM=truecolor`). | When configuring user TUI preferences, themes, keybindings, or troubleshooting TUI rendering. |
| **[Build & Plugins](https://opencode.ai/v2/docs/build/)** | Plugin architecture (`@opencode/plugin`), Tool hooks (`ctx.tool.hook`), Transforms (`ctx.tool.transform`), Event subscriptions (`ctx.event`), Context extensions (`ctx.agent`, `ctx.provider`, `ctx.model`, `ctx.mcp`, `ctx.command`), Custom tools. | When authoring plugins, guards, telemetry interceptors, or runtime middleware. |
| **[API & Server](https://opencode.ai/v2/docs/api/)** | OpenAPI 3.1.0 specification, Background service daemon, HTTP `/api/*` endpoints, WebSocket event streaming, Session compaction, Snapshot management. | When interacting directly with the local OpenCode daemon via HTTP or building client bridges. |

### 2. Live CLI Inspection Helpers (`opencode2`)

Use the native binary (`opencode2`) directly to inspect live runtime state:

- `opencode2 debug paths`: Print active filesystem locations (`home`, `data`, `cache`, `config`, `state`, `log`, `db`).
- `opencode2 debug config`: Print all resolved configuration sources and the fully merged active configuration tree.
- `opencode2 models`: List all active AI models and provider connectivity.
- `opencode2 --print-logs`: Stream real-time diagnostic server logs to stderr.
- `opencode2 stats`: Output shareable usage statistics.

### 3. Project Skills & Developer Helpers

Project-level skills are located in `.agents/skills/` (ready for use in this repository without asset embedding):

- **`opencode-theme-dev`** (`.agents/skills/opencode-theme-dev/SKILL.md`):
  - Author and migrate v2 themes (`base`, `dark`, `light`, 9-step `hue` scales, `categorical`).
  - Validation helper: `node scripts/validate-theme.mjs <theme.json>` (checks all 16 required tokens).
  - Reference: `.agents/skills/opencode-theme-dev/references/theme-token-spec.md`.
- **`opencode-plugin-dev`** (`.agents/skills/opencode-plugin-dev/SKILL.md`):
  - Develop native plugins using `@opencode/plugin` and the universal dual-mode wrapper (`Plugin.define`).
  - Reference: `.agents/skills/opencode-plugin-dev/references/plugin-api-reference.md`.
  - Examples: `.agents/skills/opencode-plugin-dev/examples/tool-guard-plugin.ts`.
- **`opencode-installer-dev`** (`.agents/skills/opencode-installer-dev/SKILL.md`):
  - Best practices for configuration installers and environment orchestrators targeting `opencode2`.
  - Reference: `.agents/skills/opencode-installer-dev/references/opencode-v2-precedence.md`.

## Harness Flow Diagrams

### 1. Session Startup Alignment & Subagent Topology (startup flowchart)

```mermaid
flowchart TD
    User([User Request / Prompt]) --> StartGate{1. Startup Alignment Gate}
    
    subgraph Alignment ["Operating Conditions Alignment"]
        StartGate -->|Ask if unset| ModeChoice[Execution Mode:\nAuto vs Interactive]
        StartGate -->|Ask if unset| PlaneChoice[Spec & Memory Plane:\nOpenSpec vs Cortex vs Hybrid]
        StartGate -->|Ask if unset| WorkloadChoice[Workload Policy:\nStrict vs Flexible vs Unbounded]
        StartGate -->|Fixed policy| WorkspaceChoice[External Implement Workspace:\nCurrent Workspace]
        
        ModeChoice --> AmbiguityCheck{High Design\nUncertainty?}
        PlaneChoice --> AmbiguityCheck
        WorkloadChoice --> AmbiguityCheck
        WorkspaceChoice --> AmbiguityCheck
        
        AmbiguityCheck -->|Yes: Unresolved branches| InvFact[Dispatch investigate:\nAutonomous Fact-Finding]
        InvFact --> GrillMe[Relentless Interview:\ngrill-me Rounds Q1..Qn]
        GrillMe -->|Frontier Resolved| RouteDecision{2. Assess Scope & Risk}
        AmbiguityCheck -->|No: Clear intent| RouteDecision
    end

    subgraph Routing ["Organic Routing Engine"]
        RouteDecision -->|direct-answer| OrchSelf[Orchestrator: Direct Answer]
        RouteDecision -->|discovery / onboarding| SubDiscovery[Subagent: discovery]
        RouteDecision -->|investigate / spike| SubInv[Subagent: investigate]
        RouteDecision -->|direct-change| SubImpDirect[Subagent: implement]
        RouteDecision -->|fast-tdd| SubImpTDD[Subagent: implement + fast-tdd]
        RouteDecision -->|hotfix| SubImpHotfix[Subagent: implement + hotfix-triage]
        RouteDecision -->|sdd-lite / sdd-full| SubPlan[Subagent: planner]
    end

    subgraph SDD_Flow ["SDD Task Execution (selected spec plane + cortex-ia work)"]
        SubPlan -->|Validated contracts & Task DAG| Minions[Ephemeral Implement Minions]
        Minions -->|Code Changes & Evidence| SubRev[Subagent: reviewer]
    end

    SubImpDirect --> AutoApproveGate{Adaptive Review:\nLow risk, data, docs, config?}
    AutoApproveGate -->|Yes: Auto-Approve| OrchFinal
    AutoApproveGate -->|No: High-risk code| SubRev
    SubImpTDD --> SubRev
    SubImpHotfix --> SubRev

    subgraph Convergence ["Convergence & Output"]
        SubInv -->|Diagnosis / Cortex Evidence| OrchFinal[Orchestrator Receipt Synthesis]
        SubDiscovery -->|.cortex-ia/discovery.md| OrchFinal
        SubRev -->|Verdict: PASS / FAIL / BLOCKED| OrchFinal
        OrchSelf --> OrchFinal
        OrchFinal --> Done([Final Response to User])
    end
```

### 3. SDD Lifecycle & Preflight (sequence diagram)

```mermaid
sequenceDiagram
    autonumber
    actor User as User Request
    participant Orch as Orchestrator
    participant Inv as Investigate Subagent
    participant Cortex as Cortex MCP & AST Graph
    participant Plan as Planner Subagent
    participant Work as cortex-ia work CLI
    participant Imp as Implement Minions
    participant Rev as Reviewer Subagent

    User->>Orch: User Prompt Received
    Orch->>Cortex: cortex_get_rules(project) (Retrieve Active Governance Directives)
    Orch->>Inv: Dispatch Fact-Finding & Investigation

    rect rgb(235, 245, 255)
    Note over Inv,Cortex: Phase 1: Investigation & AST Ingestion Gate
    Inv->>Cortex: cortex_get_code_symbols(project, limit: 1) (Check AST status)
    alt AST symbols missing and cortex watch not running
        Inv->>Cortex: cortex_ingest_code(workspace_root_absolute_path, project) (Trigger 2-Pass Static AST Ingestion)
    end
    Inv->>Cortex: filtered code symbols + cortex_search(graph_expand=true)
    Inv-->>Orch: Diagnostic Evidence & Baseline AST Topology Receipt
    end

    Note over Orch,Work: Phase 2: Preflight & Planning (if SDD route)
    Orch->>Plan: Dispatch SDD Plan (intent, project_rules, blast_radius_baseline)
    Plan->>Plan: Write and validate selected-plane contracts
    Plan->>Work: work create (dependency DAG nodes <= 350 LOC in stable initiative board)
    Plan-->>Orch: Planning Receipt (artifact refs, task refs, DAG readiness)

    Note over Orch,Imp: Phase 3: Implementation
    loop For Each Ready DAG Task
        Orch->>Imp: Dispatch Minion Envelope (task_id, allowed_files, project_rules, checks)
        Imp->>Work: work claim + file_reserve per writable file
        Imp->>Cortex: filtered symbols + bounded caller inspection
        Imp->>Imp: Implement Code + Proportional Verification (Tests)
        Imp->>Work: transition in_review (releases leases)
        Imp-->>Orch: Task Execution Receipt (changed_files)
    end

    rect rgb(255, 245, 235)
    Note over Orch,Rev: Phase 4: Adaptive Review Gate (Auto-Approval vs Independent Reviewer)
    alt Low-Risk / Data Artifacts / Docs / Declarative Config
        Orch->>Work: work approve PASS (orchestrator auto-approval with evidence)
    else High-Risk Code Tasks
        Orch->>Rev: Dispatch Review Envelope (board_id, changed_files, blast_radius_baseline)
        Rev->>Cortex: cortex_ingest_code(workspace_root_absolute_path, project) [Delta Ingestion: <50ms]
        Rev->>Cortex: compare symbols/imports/callers (detect unapproved coupling)
        Rev->>Cortex: cortex_detect_cycles (Verify no circular import regressions)
        Rev->>Rev: Independent Checks & Mutation Testing
        alt Verdict is PASS
            Rev->>Work: work approve PASS (gate approval with evidence)
            Rev->>Cortex: cortex_save(type: "decision", topic_key: "architecture/feature") + cortex_relate
            Rev-->>Orch: Review Receipt (Verdict: PASS)
            Orch->>Plan: Archive selected-plane contract after approval
        else Verdict is FAIL / BLOCKED
            Rev->>Cortex: cortex_save(type: "bugfix", topic_key: "gotchas/task_id", content: minimal_failure_locality) + cortex_relate
            Rev-->>Orch: Review Receipt (Verdict: FAIL, evidence_ref: "gotchas/task_id")
            Orch->>Imp: Re-dispatch Targeted Fix Minion (with evidence_ref from Cortex)
        end
    end
    end

    Orch-->>User: Final Response + Cortex Session Summary
```

### 4. Implementation Minion Lifecycle (state diagram)

```mermaid
stateDiagram-v2
    [*] --> PreClaim: Dispatch Envelope Received
    PreClaim --> Claimed: work status + work claim
    Claimed --> Reserved: cortex_ia_file_reserve (exclusive single file)
    
    state Execution_Loop {
        [*] --> Red_Green_Refactor
        Red_Green_Refactor --> Heartbeat_Renew: work renew + lease-renew
        Heartbeat_Renew --> Red_Green_Refactor
    }
    
    Reserved --> Execution_Loop: Edit & Test
    Execution_Loop --> Verifying: Proportional Verification (Unit/Build/Lint)
    Verifying --> EvidenceSaved: context-distiller -> cortex_save
    EvidenceSaved --> InReview: work transition --to in_review
    InReview --> Released: reviewer verifies and work approve PASS
    Released --> DoneState: CLI atomically releases locks and marks done
    DoneState --> ReceiptReturned: Return Typed Receipt
    ReceiptReturned --> [*]

    Execution_Loop --> Blocked: Lease Expired / Unresolvable Conflict
    Blocked --> Cleanup: work release
    Cleanup --> ReceiptReturned
```

### 7. Safety, Shell Boundaries & Guard Plugins (guard flow)

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
