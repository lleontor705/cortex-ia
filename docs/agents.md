# Agent & Sub-Agent Reference

`cortex-ia` configures the AI agent ecosystem for **OpenCode** as its primary, fully-supported platform, with planned roadmap support for **Google Antigravity** and **Claude CLI (Claude Code)**.

---

## 1. Supported Platform Matrix

| Platform / Agent | Status | Target Location | Description |
| :--- | :---: | :--- | :--- |
| **OpenCode** | **Active (Native)** | `~/.config/opencode/` | Full SDD stack: system prompt, 5 native sub-agents, 9 commands, 12 skills, plugins, and managed MCPs (`cortex`, `forgespec`, `context7`). |
| **Google Antigravity** | *Roadmap* | `~/.gemini/antigravity/` | Native rules, skills, and sidecar tools configuration. |
| **Claude CLI / Code** | *Roadmap* | `~/.claude/` | Native prompts, tools, and MCP JSON configurations. |

> [!NOTE]
> All legacy adapters (Cursor, VS Code Copilot, Windsurf, Gemini CLI, Kilocode, Kimi, Kiro, Qwen) were retired to ensure strict transactional integrity, atomic installation, and zero-drift verification.

---

## 2. OpenCode Native Layout & Model Mapping

OpenCode uses a structure-preserving native layout under `~/.config/opencode/`:

```text
~/.config/opencode/
├── opencode.jsonc              # Base configuration & MCP server registrations
├── AGENTS.md                   # Global orchestrator system prompt
├── agents/                     # Sub-agent role definitions (Markdown)
│   ├── orchestrator.md         # Primary coordinator
│   ├── planner.md              # Requirements, Architecture, DAG
│   ├── implement.md            # Ephemeral implementation worker
│   ├── investigate.md          # Diagnostic & root-cause analyst
│   └── reviewer.md             # Adversarial quality gatekeeper
├── commands/                   # Interactive slash commands
│   ├── sdd.md, hotfix.md, work.md, tdd.md, review.md, spike.md, ...
├── plugin/                     # Background supervisors & plugins
│   ├── background-supervisor.ts
│   ├── cortex.ts
│   └── model-variants.ts
└── skills/                     # Native skills with SKILL.md specs
    ├── implement/SKILL.md
    ├── fast-tdd/SKILL.md
    ├── context-distiller/SKILL.md
    └── ...
```

### Model Assignments (`opencode.jsonc`)

| Agent | Default Model | Reasoning Tier | Role Specialization |
| :--- | :--- | :---: | :--- |
| **`@orchestrator`** | `openai/gpt-5.6-sol` | High-Reasoning | State-machine transitions, risk routing, minion dispatch. |
| **`@planner`** | `openai/gpt-5.6-sol` | High-Reasoning | Requirement specs, DAG decomposition, review budget guard. |
| **`@implement`** | `zai-coding-plan/glm-5.3` | Fast-Coding | Bounded task execution, atomic edits, Fast-TDD loop. |
| **`@investigate`** | `zai-coding-plan/glm-5.3` | Fast-Coding | Root cause diagnosis, exploratory spikes, read-only audit. |
| **`@reviewer`** | `openai/gpt-5.6-sol` | High-Reasoning | Adversarial review, mutation testing, dual review gates. |

---

## 3. Sub-Agent Detailed Specifications

### 1. Orchestrator (`agents/orchestrator.md`)
- **Mode**: Primary (`mode: primary`, `temperature: 0.2`, `color: #4A90D9`).
- **Core Policy**: Sole routing, state-management, and delegation authority. **NEVER writes product code directly** and never delegates to legacy roles.
- **Workflow Scoring**: Evaluates requests across 6 axes: `[Risk, Ambiguity, Coupling, Testability, Reversibility, Parallelism]` to select routes (`direct-answer`, `direct-change`, `fast-tdd`, `hotfix`, `spike`, `sdd-lite`, `sdd-full`, `review`).
- **Dual Review Gate**: Automatically triggers dual independent review passes for high-risk or security-sensitive changes.

### 2. Planner (`agents/planner.md`)
- **Mode**: Sub-agent (`mode: subagent`, `temperature: 0.2`, `color: #546E7A`).
- **Review Budget Guard**: Enforces that every task node in the DAG forecasts **<= 350 changed lines** (or <= 500 lines for verbose languages).
- **Stacked Work Units**: Decomposes large changes (>400 lines) into hierarchical layers (Layer 1: Contracts/Types -> Layer 2: Domain Logic -> Layer 3: Integration/UI).
- **Fallback Policy**: Uses symbolic navigation (LSP) when present; immediately falls back to `grep`, `glob`, and targeted `read` without blocking.

### 3. Implement (`agents/implement.md`)
- **Mode**: Ephemeral Minion (`mode: subagent`, `temperature: 0.2`, `color: #2E7D32`).
- **Mandatory Lifecycle**:
  1. `forgespec_capabilities` -> Resolve pre-claim state -> `tb_claim` (acquire attempt lease).
  2. `file_reserve` -> Exclusive advisory lease for every target file before touching code.
  3. Execute Fast-TDD loop (`RED` -> `GREEN` -> `Refactor`) or targeted change.
  4. `cortex_save` -> Persist test exit codes and minimal diff evidence via `context-distiller`.
  5. `file_release` -> Release file leases while claim authority is live.
  6. `tb_update` -> Transition task to `in_review` or `done`.
- **Cleanup Rule**: Resource and lease cleanup is **mandatory on PASS, FAIL, or BLOCKED**.

### 4. Investigate (`agents/investigate.md`)
- **Mode**: Sub-agent (`mode: subagent`, `temperature: 0.3`, `color: #78909C`).
- **Policy**: Ground findings with exact paths, commands, exit codes, and limitations.
- **Spike Protocol**: Prototype writes are confined strictly to an approved disposable scratch path (`.cortex-ia/scratch/`). Product code is never mutated.

### 5. Reviewer (`agents/reviewer.md`)
- **Mode**: Sub-agent (`mode: subagent`, `temperature: 0.1`, `color: #D32F2F`).
- **Adversarial Protocol**: Blind evaluation of security, invariants, edge cases, and regressions.
- **Mutation Testing**: Uses `mutation-testing` to verify test robustness and eliminate false-positive test assertions.
- **Best-of-N Selection**: Evaluates competing candidate receipts from parallel implementers and arbitrates based on mutation score, diff compactness, and performance benchmarks.

---

## 4. Minion Dispatch & Receipt Contract

### Dispatch Envelope (`@orchestrator` ──▶ Minions)
```json
{
  "objective": "Implement deterministic SHA256 digest in plan finalizer",
  "workflow": "fast-tdd",
  "task_id": "TASK-102",
  "artifact_refs": ["SPEC-001#sec-2"],
  "evidence_refs": ["cortex://bugfix/digest-drift"],
  "non_goals": ["Do not modify CLI flag parsing"],
  "allowed_files": ["internal/pipeline/plan.go", "internal/pipeline/engine.go"],
  "allowed_effects": ["EffectMCPAdd", "EffectMCPRemove", "EffectMCPNoop"],
  "required_skill": "fast-tdd",
  "skills_to_load": ["fast-tdd", "ast-impact-analysis"],
  "acceptance_checks": ["go test -count=1 ./internal/pipeline/..."],
  "budget": { "max_turns": 30, "max_retries": 1, "max_lines": 350 },
  "stop_conditions": ["Lease timeout", "CAS conflict"],
  "escalate_when": ["Unmanaged file collision detected"]
}
```

### Typed Receipt v2.0 (Minions ──▶ `@orchestrator`)
```json
{
  "receipt_version": "2.0",
  "task_id": "TASK-102",
  "phase_status": "success",
  "task_status": "done",
  "verification_verdict": "PASS",
  "changed_files": ["internal/pipeline/plan.go", "internal/pipeline/engine.go"],
  "evidence_refs": ["cortex://evidence/task-102"],
  "verification_commands": [
    { "command": "go test -count=1 ./internal/pipeline/...", "exit_code": 0, "oracle_type": "unit" }
  ],
  "cleanup_completed": true,
  "deviations": [],
  "risks": []
}
```

---

## 5. Dual-Plane Coordination Protocol

```text
┌────────────────────────────────────────────────────────┐
│                   OpenCode Session                     │
│               (@orchestrator Entrypoint)               │
└───────────────┬────────────────────────┬───────────────┘
                │                        │
                ▼                        ▼
┌───────────────────────────────┐ ┌───────────────────────────────┐
│ ForgeSpec MCP (Control Plane) │ │  Cortex MCP (Evidence Plane)  │
├───────────────────────────────┤ ├───────────────────────────────┤
│ • Board & DAG Revisions (CAS) │ │ • Cross-Session Long Memory   │
│ • tb_claim / tb_heartbeat     │ │ • Knowledge Graph Relations   │
│ • file_reserve / file_release │ │ • Bug Lineage & Gotchas       │
│ • Gate Approvals (tb_approve) │ │ • context-distiller Summaries │
└───────────────────────────────┘ └───────────────────────────────┘
```

1. **Control Plane (ForgeSpec)**:
   - Atomic state transitions using Compare-And-Swap (CAS) revisions.
   - File reservation leases prevent conflicting concurrent edits across parallel sub-agents.
2. **Evidence Plane (Cortex)**:
   - Durable memory across multiple agent sessions.
   - Saves empirical proof (test outputs, exit codes, diff hashes) without cluttering context windows.
3. **Supervisor Runtime (`plugin/background-supervisor.ts`)**:
   - Asynchronous background task execution with reader/writer admission limits.
   - Automatic timeout detection and session recovery.

---

## 6. OpenCode Slash Commands

| Slash Command | Primary Route | Dispatched Workflow |
| :--- | :--- | :--- |
| **`/sdd`** | `@orchestrator` | Full Spec-Driven Development (Investigate → Plan → DAG → Implement → Review). |
| **`/hotfix`** | `@orchestrator` | Incident triage & hotfix loop (`hotfix-triage` → `@implement` → `@reviewer`). |
| **`/work`** | `@orchestrator` | Direct change execution for bounded single tasks. |
| **`/tdd`** | `@orchestrator` | Fast Test-Driven Development with deterministic unit test oracle. |
| **`/review`** | `@orchestrator` | Adversarial code review, security audit, and mutation testing. |
| **`/spike`** | `@orchestrator` | Disposable exploratory prototype in scratch directory. |
| **`/investigate`**| `@orchestrator` | Read-only codebase exploration and root-cause analysis. |
| **`/status`** | `@orchestrator` | Displays active task board and ForgeSpec DAG state. |
| **`/resume`** | `@orchestrator` | Reconnects to in-flight SDD session and reconciles CAS state. |

