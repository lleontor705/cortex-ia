<p align="center">
  <img src="docs/assets/hero-banner.svg" alt="Cortex-IA Hero Banner" width="100%" />
</p>

<p align="center">
  <a href="https://github.com/lleontor705/cortex-ia/releases/latest"><img src="https://img.shields.io/github/v/release/lleontor705/cortex-ia?color=38BDF8&label=release" alt="Release"></a>
  <a href="https://github.com/lleontor705/cortex-ia/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lleontor705/cortex-ia?color=A855F7" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lleontor705/cortex-ia"><img src="https://goreportcard.com/badge/github.com/lleontor705/cortex-ia" alt="Go Report Card"></a>
  <a href="https://github.com/lleontor705/cortex-ia/actions"><img src="https://img.shields.io/badge/tests-100%25%20passing-10B981" alt="Tests"></a>
  <a href="https://github.com/lleontor705/cortex-ia"><img src="https://img.shields.io/badge/platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue" alt="Platforms"></a>
</p>

---

## ⚡ What is Cortex-IA?

**Cortex-IA** is the enterprise-grade, deterministic **Multi-Agent Control Plane & Orchestration Engine** designed for autonomous software development with **OpenCode** and **Herdr**. 

Built as a single portable Go binary, Cortex-IA solves the fundamental challenges of multi-agent coding: **race conditions**, **conflicting file edits**, **hallucinated task readiness**, **unmonitored background tasks**, and **unstructured coordination**.

```text
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   CORTEX-IA ECOSYSTEM                                       │
│                                                                                             │
│  ┌──────────────────────────┐   ┌─────────────────────────────┐   ┌─────────────────────────┐  │
│  │     OpenCode Agents      │   │   CORTEX-IA Control Plane   │   │  Herdr Multiplexing     │  │
│  │ (Orchestrator, Discovery,│──▶│  (SQLite ACID DAG, Leases,  │──▶│  (Live Stream Terminals,│  │
│  │  Investigate, Planner,   │   │   CAS Revisions, OpenSpec)  │   │   NDJSON Telemetry)     │  │
│  │   Implement, Reviewer)   │   │                             │   │                         │  │
│  └──────────────────────────┘   └─────────────────────────────┘   └─────────────────────────┘  │
│                                             │                                                │
│                                             ▼                                                │
│                               ┌───────────────────────────┐                                  │
│                               │     CORTEX Server (MCP)   │                                  │
│                               │  (AST Graph & Blast Tree) │                                  │
│                               │  (Epistemic Evidence DB)  │                                  │
│                               └───────────────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🌟 Key Superpowers

- 🔒 **Zero-Race Concurrency with Exclusive File Leases (`work lease`)**  
  Prevents agents from overwriting each other's code. Agents must atomically reserve exclusive workspace-relative file paths with TTL leases before editing. Parallel native implementers safely share the workspace via disjoint file reservations (`cortex_ia_file_reserve`).
- 🎯 **Deterministic Task DAG with Optimistic CAS Locking (`work claim` / `transition`)**  
  Tasks transition through strict state machines (`backlog ➔ ready ➔ in_progress ➔ in_review ➔ done`). Downstream dependencies automatically unlock only when prior dependencies receive an independent review approval.
- 🛡️ **Mandatory Independent Review Gates (`work approve`)**  
  Implementers cannot self-approve. An independent reviewer agent must verify test suites and recorded evidence before marking any task complete.
- 🧹 **Zero Raw JSON Chat Hygiene & Typed Tool Authority**  
  Eliminates token bloat and hallucinated text parsing. Structured receipts are passed directly via typed tool calls (`cortex_ia_work_transition` and `cortex_ia_work_approve`) stored atomically in SQLite, while chat displays clean, readable Markdown summaries.
- 📺 **Live Real-time Terminal Telemetry in Herdr Panes (`delegate worker`)**  
  Watch external worker agents think and act in real-time. Features live action humanization, sub-second tool execution telemetry, animated activity spinners, and streamed textual reasoning.
- 📐 **Native OpenSpec SDD Integration (`cortex-ia openspec`)**  
  Built-in support for Specification-Driven Development proposals, RFC 2119 delta specifications, and task decompositions bounded to ≤350 LOC.
- 📊 **Real-time Web Operations Dashboard (`cortex-ia web`)**  
  Embedded, single-binary Web UI with real-time SSE streaming for live board state visualization, task creation, and audit logging.

---

## 🧭 CORTEX (MCP) vs CORTEX-IA (CLI)

<p align="center">
  <img src="docs/assets/cortex-vs-cortexia.svg" alt="Cortex vs Cortex-IA" width="100%" />
</p>

| Dimension | 🧠 **CORTEX** (MCP Server) | ⚙️ **CORTEX-IA** (Control Plane & CLI) |
|---|---|---|
| **Nature** | Standardized MCP Server (32 tools: `cortex_*`) | Standalone native Go binary (`cortex-ia.exe`) |
| **System Plane** | **Epistemic & Evidence Plane** | **Operational Control Plane** |
| **Storage** | Knowledge Graph & AST Symbol DB | ACID Transactional SQLite (`~/.cortex-ia/delegation.db`) |
| **Primary Focus** | • AST code symbols & call graphs<br>• Blast radius impact analysis<br>• Durable bug gotchas & ADR memories<br>• Cross-session project context | • Task DAG & CAS revision state machines<br>• Atomic claim tokens & exclusive file leases<br>• Herdr terminal multiplexing & live NDJSON streaming<br>• OpenSpec SDD validator & Web dashboard |
| **Authority Rule** | **Informative & Advisory Only.** Stored observations never authorize code writes or mark tasks complete. | **Single Source of Truth.** Task readiness, leases, transitions, and approvals exist strictly in SQLite via `cortex-ia work`. |

---

## 🚀 Quick Start

### 1. Interactive Setup (TUI)
Launch the beautiful BubbleTea terminal interface to configure your OpenCode environment:
```bash
cortex-ia
```

### 2. Fast Non-Interactive Installation
```bash
cortex-ia install         # Installs agents, skills, plugins & registers Cortex MCP
cortex-ia sync            # Converges installed home with embedded assets
cortex-ia doctor          # Verifies health, environment paths & tool dependencies
```

### 3. Launch the Real-Time Web Dashboard
```bash
cortex-ia web --open      # Launches the local dashboard at http://127.0.0.1:7331
```

---

## 📦 Installation

### Precompiled Binary (Recommended)
Download the latest prebuilt binary from the [Releases](https://github.com/lleontor705/cortex-ia/releases) page for Windows, macOS, or Linux.

### Go Install
```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

### Install Script (Linux / macOS)
```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

### Build from Source
```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia ./cmd/cortex-ia
```

---

## 💻 CLI Command Surface

Commands that emit machine-readable receipts print JSON to stdout; human diagnostic commands (`doctor`, `rollback`, `recover`, `report status`, `help`, `update`) print plain text. Status queries accept `show`/`get` aliases where noted, and each subcommand group prints its usage with `--help`.

### 1. Task Boards (`cortex-ia board`)
| Command | Syntax | Purpose |
|---|---|---|
| **Create** | `cortex-ia board create <id> "<title>" "[desc]"` | Initialize a durable task-board boundary |
| **List** | `cortex-ia board list` | List all boards with completed/total counters |
| **Status** | `cortex-ia board status <id>` *(or `show`, `get`)* | Query board metadata and full task DAG snapshot |
| **Archive** | `cortex-ia board archive <id>` | Mark a completed board as archived |
| **Unarchive** | `cortex-ia board unarchive <id>` | Restore an archived board to active |
| **Delete** | `cortex-ia board delete <id>` | Permanently delete an archived board and its tasks |
| **Serve** | `cortex-ia board serve [--addr 127.0.0.1:7331]` | Run the embedded loopback web dashboard |

### 2. Work Items & Leases (`cortex-ia work`)
| Command | Syntax | Purpose |
|---|---|---|
| **Create** | `cortex-ia work create <id> "<title>" [--board <board>] [--depends <id>]... [--objective <text>] [--acceptance <text>] [--verify <cmd>] [--file <path>]...` | Add a task to the DAG (`backlog`/`ready`) with its definition |
| **Revise** | `cortex-ia work revise --plan <file\|@stdin>` | Safely revise an unclaimed task definition |
| **Review Refresh** | `cortex-ia work review-refresh <id> --revision <n>` | Rebind the review to an observed revision |
| **Archive** | `cortex-ia work archive --board <id> --change <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Close independently approved SDD work |
| **List** | `cortex-ia work list [--board <board-id>]` | List work items, optionally scoped to one board |
| **Status** | `cortex-ia work status <id>` *(or `show`, `get`)* | Query task status, revision, claim, and active leases |
| **Approvals** | `cortex-ia work approvals <id>` | List historical approval records |
| **Fingerprint** | `cortex-ia work fingerprint <id>` | Compute current fingerprints and compare with the approval |
| **Claim** | `cortex-ia work claim <id> --owner <owner> [--path <file> ...] [--ttl 15m]` | Atomically acquire a task (and optional leases); returns `claim_token` |
| **Controller Renew** | `cortex-ia work controller-renew <id> --owner <owner> --authority @stdin` | Renew a live claim and its complete lease set |
| **Renew** | `cortex-ia work renew <id> --claim-token <tok> [--ttl 15m]` | Extend live claim TTL before expiry |
| **Lease** | `cortex-ia work lease <id> --claim-token <tok> --path <file> [--ttl 15m]` | Reserve one exclusive file lease; returns `lease_token` |
| **Reserve** | `cortex-ia work reserve <id> --claim-token <tok> --path <file> [--path <file> ...] [--ttl 15m]` *(or `file-reserve`)* | Reserve one or more files atomically |
| **Lease Renew** | `cortex-ia work lease-renew --path <file> --lease-token <tok> [--ttl 15m]` | Extend a file lease TTL while editing |
| **Release** | `cortex-ia work release --path <file> --lease-token <tok>` | Release one file lease |
| **Release All** | `cortex-ia work release-all <id> --claim-token <tok>` | Release every file lease held by a task |
| **Transition** | `cortex-ia work transition <id> --claim-token <tok> [--revision <n>] --to <in_review\|in_progress\|blocked>` | Shift task state with an optional submission receipt |
| **Approve** | `cortex-ia work approve <id> --reviewer <id> --verdict <PASS\|FAIL\|BLOCKED\|INCONCLUSIVE> [--evidence <ref>]` | Record a review verdict; `PASS` unlocks downstream tasks |
| **Retry** | `cortex-ia work retry <id> [--revision <n>]` | Clear residual locks and return a `blocked` task to `ready` |
| **Decompose** | `cortex-ia work decompose <id> --revision <n> --plan <file\|@stdin> [--contract-file <file>]` | Replace a blocked task with atomic tasks |
| **Recover** | `cortex-ia work recover` | Sweep expired claims/leases |
| **Verify Lease** | `cortex-ia work verify-lease --path <file> [--task <id>] [--owner <owner>]` *(or `check-lease`)* | Verify an active file lease |

### 3. OpenSpec SDD Workspace (`cortex-ia openspec`)
| Command | Syntax | Purpose |
|---|---|---|
| **Validate** | `cortex-ia openspec validate <change> --workflow <sdd-lite\|sdd-full> --phase <phase> [--json]` | Structurally validate planning artifacts. `--workflow` and `--phase` are **required** |
| **List** | `cortex-ia openspec list` | List active change proposals in `openspec/changes/` |
| **Status** | `cortex-ia openspec status [change-name]` | Inspect task progress and status of changes |
| **Archive** | `cortex-ia openspec archive <change-name> --board <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Close independently approved SDD work |
| **New** | `cortex-ia openspec new <change-name> [domain]` | Scaffold a new OpenSpec change directory |

### 4. Cortex Snapshots (`cortex-ia snapshot`)
| Command | Syntax | Purpose |
|---|---|---|
| **Read** | `cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]` | Read and verify one bounded local Cortex observation |

### 5. Git Worktrees (`cortex-ia worktree`)
| Command | Syntax | Purpose |
|---|---|---|
| **List** | `cortex-ia worktree list [--repo <repo-path>]` | List authoritative Git worktrees |
| **Validate** | `cortex-ia worktree validate <worktree-path> [--repo <repo-path>] [--head <commit>]` | Validate a worktree contract against git porcelain |

`current_workspace` is the only supported execution strategy. `worktree create`, `clean`, `drop`, `delete`, `remove`, and `prune` are retired and fail closed; existing worktrees are preserved.

### 6. Dual Ledger (`cortex-ia ledger`)
| Command | Syntax | Purpose |
|---|---|---|
| **Fact Add** | `cortex-ia ledger fact add <text> [--board <id>] [--source <src>] [--sync-cortex]` | Record a verified fact (optionally synced to Cortex memory) |
| **Fact List** | `cortex-ia ledger fact list [--board <board-id>] [--json]` | List verified facts in chronological order |
| **Progress** | `cortex-ia ledger progress record --summary <text> [--drift] [--action <act>]` | Record an orchestrator progress evaluation |
| **Status** | `cortex-ia ledger status [--board <board-id>] [--json]` | Display the full dual-ledger report (facts + progress) |

### 7. UI Snapshot (`cortex-ia ui`)
| Command | Syntax | Purpose |
|---|---|---|
| **Snapshot** | `cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]` | Print a bounded read-only TUI snapshot |

### 8. Documents & Diagrams (`cortex-ia doc` / `cortex-ia diagram`)
| Command | Syntax | Purpose |
|---|---|---|
| **Doc Convert** | `cortex-ia doc convert <file> [-o <out.md>] [--standalone] [--format <fmt>] [--max-lines <n>] [--ocr <hosted\|reject>] [--json]` | Convert office/PDF documents to Markdown |
| **Doc Inspect** | `cortex-ia doc inspect <file> [--json]` | Inspect document metadata |
| **Diagram Validate** | `cortex-ia diagram validate <type> <spec.json> [--quality <standard\|showcase>] [--json]` | Validate diagram topology |
| **Diagram Render** | `cortex-ia diagram render <type> <spec.json> [output.html] [--quality <standard\|showcase>] [--json]` | Render an interactive diagram HTML file |
| **Diagram Compare** | `cortex-ia diagram compare <base.json> <head.json> [output.html] [--json]` | Compare two architecture snapshots |
| **Diagram Reach** | `cortex-ia diagram reach <type> <spec.json> --from <node-id> [--direction <upstream\|downstream\|both>] [--json]` | Trace graph reachability from a node |

### 9. MCP Management (`cortex-ia mcp`)
| Command | Syntax | Purpose |
|---|---|---|
| **Add (preset)** | `cortex-ia mcp add <name> --preset [--dry-run]` | Register a managed catalog MCP preset |
| **Add (local)** | `cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]` | Register a managed custom local MCP server |
| **Add (remote)** | `cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]` | Register a managed custom remote MCP server |
| **List** | `cortex-ia mcp list [--json]` | List managed MCP entries and ownership |
| **Remove** | `cortex-ia mcp remove <name> [--dry-run]` | Deregister a managed MCP entry |

`--preset`, `--local`, and `--remote` are mutually exclusive: exactly one is required per `add`.

### 10. Reporting & Hooks (`cortex-ia report` / `cortex-ia hook`)
| Command | Syntax | Purpose |
|---|---|---|
| **Report Error** | `cortex-ia report error --code <code> --message <msg> [--details <text\|@stdin>]` *(or `send`)* | Generate and send a signed error report |
| **Report Config** | `cortex-ia report config [--endpoint <url>] [--secret <key>] [--enable\|--disable]` | Configure the reporting endpoint |
| **Report Flush** | `cortex-ia report flush` | Retry bounded queued reports |
| **Report Status** | `cortex-ia report status` | Show the current reporting configuration |
| **Hook Pre-Tool** | `cortex-ia hook pre-tool` | Execute the Antigravity pre-tool lifecycle hook |
| **Hook Stop** | `cortex-ia hook stop` | Execute the Antigravity stop lifecycle hook |

### 11. Maintenance & Lifecycle (`install` / `sync` / `doctor` / `rollback` / `recover` / `uninstall` / `update`)
| Command | Syntax | Purpose |
|---|---|---|
| **Install** | `cortex-ia install [--target <list>] [--dry-run] [--overwrite]` | Install assets and plugins (default target: `opencode`) |
| **Sync** | `cortex-ia sync [--target <list>] [--dry-run] [--overwrite]` | Reconcile the installed home with the current asset set |
| **Doctor** | `cortex-ia doctor` | Read-only installation health report |
| **Rollback** | `cortex-ia rollback [backup-id]` / `cortex-ia rollback list` | Restore a backup or list available backups |
| **Recover** | `cortex-ia recover [list]` / `cortex-ia recover <journal-id>` | List or restore pending recovery journals |
| **Uninstall** | `cortex-ia uninstall [--target <list>] [--dry-run]` | Remove the accredited installation |
| **Update** | `cortex-ia update [--check]` *(or `upgrade`)* | Check for / install the latest release |

---

## 🤖 Multi-Agent Coordination Topology

<p align="center">
  <img src="docs/assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

1. **`orchestrator` (Primary)**: Triage, startup alignment, Cortex session lifecycle, and DAG dispatch. Never claims tasks or holds file leases.
2. **`discovery` (Subagent)**: Inspects skills, toolchains, engines, and project architecture into the durable `.cortex-ia/discovery.md` profile.
3. **`investigate` (Subagent)**: Root-cause diagnosis, AST blast radius inspection, spikes, and read-only diagnostic audits.
4. **`planner` (Subagent)**: Writes OpenSpec delta specifications (RFC 2119), Given/When/Then contracts, and decomposes task DAGs (≤350 LOC).
5. **`implement` (Subagent)**: Atomically claims one task, reserves exclusive file leases, runs fast TDD loops, and transitions to review via typed tools.
6. **`reviewer` (Subagent)**: Independently verifies git diffs, executes test oracles, and grants `PASS` approval to unlock downstream dependencies.

---

## 🚦 3-Tier Organic Routing Model

Cortex-IA matches user requests to the smallest, safest workflow using a three-tier model:

| Tier | Workflows | Characteristics | Execution Model |
|---|---|---|---|
| **Tier 1: Fast Path** | `direct-answer`, `discovery`, `investigate`, `spike`, `hotfix`, `fast-tdd`, `ops-task` | Direct execution without task DAG overhead. Specialized for Q&A, onboarding, root-cause diagnosis, or fast unit TDD. | Single-turn dispatch via `orchestrator ➔ subagent ➔ orchestrator`. |
| **Tier 2: Bounded Unitary Task** | `direct-change` | Single-domain, low-risk changes with fast verification. Uses `board_id: "default"`. | Claim task ➔ exclusive file lease ➔ edit & test ➔ `cortex_ia_work_transition` ➔ independent review gate. |
| **Tier 3: Coordinated SDD** | `sdd-lite`, `sdd-full` | High-complexity, multi-file features or cross-domain architectural changes. | Stable initiative board ➔ OpenSpec delta specs ➔ DAG decomposition (≤350 LOC) ➔ parallel implementation minions ➔ adversarial review. |

---

## 🛡️ Transactional Safety Guarantees

- **Dry-Run Determinism**: `--dry-run` calculates the exact execution plan without making any disk writes.
- **Cross-Process File Locking**: Every mutating command holds a robust cross-process file lock (`LockFileEx` on Windows, `flock` on Unix) preventing concurrent installer races.
- **Verified Backups & Rollbacks**: Snapshots affected configuration files under `~/.cortex-ia/backups/` and automatically rolls back if an apply phase encounters an error.
- **Strict Path Sandboxing**: Leases and workspace operations reject directory traversal (`..`) and absolute path escape attempts.

---

## 📚 Documentation Reference

- 📖 [Quickstart Guide](docs/quickstart.md) — Guided first-time setup and onboarding
- 🏛️ [Architecture Deep-Dive](docs/architecture.md) — Internal engine layers, models, and SQLite concurrency
- 🤖 [Agent Roles & Contracts](docs/agents.md) — 6-role coordination topology and typed receipt contracts
- 🧠 [Cortex Memory & Graph](docs/cortex-memory.md) — AST symbol graph, blast radius, and durable observations
- 📑 [SDD Workflow Guide](docs/sdd-workflow.md) — Specification-Driven Development lifecycle with OpenSpec
- 🔒 [MCP & Security Boundaries](docs/codebase/mcp-boundaries.md) — Separation of authority and security rules

---

## 📄 License

MIT License · Built with ❤️ by [Luis Leon](https://github.com/lleontor705) and contributors.
