# Cortex — Persistent Memory & AST Code Intelligence for AI Agents

`cortex` is the high-performance memory and code intelligence layer wired into OpenCode agents by Cortex-IA. Running as a dedicated Go binary exposing 32 tools over MCP, it provides a local-first SQLite + FTS5 + vector store, an associative knowledge graph, and a Zero-CGO Static AST Extractor.

---

## 🌟 Key Superpowers & Benefits of Cortex

### 1. ⚡ Zero-CGO Static AST Code Extractor
- **Polyglot Parsing**: Built-in 2-pass static extractor for Go, TypeScript, JavaScript, Python, and Rust.
- **Symbol & Dependency Graphing**: Extracts functions, structs, classes, interfaces, method receivers, import trees, and call hierarchies (e.g. 1,200+ symbols and 4,000+ relationships indexed in milliseconds).
- **Incremental SHA-256 Cache**: Delta updates re-index changed files in `<50ms` during code reviews.
- **Key Tools**: `cortex_ingest_code(path, project)`, `cortex_get_code_symbols(project)`, `cortex_code_graph(project)`.

### 2. 🔄 Architectural Cycle Detection & Blast Radius
- **Structural Cycle Invariant**: Detects circular imports and cross-package architectural violations across the entire codebase.
- **Refactor Impact**: Traces call graphs and caller dependencies before renaming or extracting components.
- **Key Tools**: `cortex_detect_cycles(project)`, `cortex_analyze_architecture(project)`, `cortex_get_blast_radius(id)`.

### 3. 🕸️ Multi-Hop HippoRAG & Associative Knowledge Graph
- **Connected Memory**: Links bug root causes, ADR decisions, and gotchas with typed relations (`references`, `relates_to`, `follows`, `supersedes`, `contradicts`).
- **Hybrid Retrieval**: Combines SQLite FTS5 full-text search with vector embeddings (RRF rank fusion) and graph expansion (`graph_expand: true`).
- **Key Tools**: `cortex_save`, `cortex_relate`, `cortex_search`, `cortex_search_hybrid`, `cortex_graph`.

### 4. 📜 Behavioral Governance & Project Directives
- **Directives Injection**: Persists permanent team rules, architectural constraints, and safety guidelines that are automatically injected into agent dispatches.
- **Strict Hygiene**: Directives (`cortex_save_rule`) are reserved exclusively for permanent invariants, keeping ephemeral task notes and test logs in SQLite task state.
- **Key Tools**: `cortex_get_rules(project)`, `cortex_save_rule(title, content, topic_key)`.

### 5. 🎯 Single-Initiative Session Idempotency
- **Lifecycle Demarcation**: The root `orchestrator` owns `cortex_session_start` and `cortex_session_end`. Subagents and external leaves remain ephemeral within that session context.
- **Session Continuity**: Upon startup, orchestrators check `cortex_context` to reuse existing active session IDs, preventing fragmented context.

---

## 🛠️ MCP Tool Groups (32 Tools)

| Category | Tools | Description |
|---|---|---|
| **Core Memory & Search** | `cortex_save`, `cortex_update`, `cortex_search`, `cortex_search_hybrid`, `cortex_get_observation`, `cortex_context`, `cortex_suggest_topic_key` | CRUD observations, hybrid FTS+vector search, topic keys |
| **Knowledge Graph** | `cortex_relate`, `cortex_graph`, `cortex_graph_relationships`, `cortex_graph_path`, `cortex_score` | Graph edges, BFS path traversal, PageRank importance |
| **AST & Architecture** | `cortex_ingest_code`, `cortex_get_code_symbols`, `cortex_code_graph`, `cortex_detect_cycles`, `cortex_analyze_architecture`, `cortex_get_blast_radius` | Polyglot AST extraction, import cycles, blast radius |
| **Governance & Rules** | `cortex_get_rules`, `cortex_save_rule`, `cortex_resolve_query`, `cortex_get_status` | Team directives, rule persistence, unified query |
| **Session Lifecycle** | `cortex_session_start`, `cortex_session_end`, `cortex_session_summary`, `cortex_capture_passive` | Initiative boundary demarcation, final session summaries |
| **History & Hygiene** | `cortex_revision_history`, `cortex_consolidate`, `cortex_project_dna`, `cortex_handoff` | Snapshot evolution, near-duplicate consolidation |

---

## 🧭 Separation of Planes: Epistemic vs Operational

```
┌─────────────────────────────────────────────────────────────┐
│                 EPISTEMIC / EVIDENCE PLANE                  │
│                        Cortex MCP                           │
│  • AST Code Symbols & Relations  • Bug Gotchas & ADRs       │
│  • Cycle Detection & Architecture • Project Directives      │
│                [ Advisory & Informative Only ]               │
└─────────────────────────────────────────────────────────────┘
                              │
                    Evidence & Knowledge
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 OPERATIONAL CONTROL PLANE                   │
│                       Cortex-IA CLI                         │
│  • Deterministic Task DAG (CAS)  • Exclusive File Leases    │
│  • Independent Review Approval   • Board Lifecycle (Archive)│
│                 [ Single Source of Authority ]              │
└─────────────────────────────────────────────────────────────┘
```

> [!IMPORTANT]
> **Authority Invariant**: Cortex memory is advisory. Having an observation or high score in Cortex never authorizes code writes or marks tasks as complete. Authority exists strictly in SQLite via `cortex-ia work claim`, `work lease`, and independent `work approve`.

---

## 🔌 How Cortex-IA Registers Cortex MCP

Cortex-IA registers the Cortex MCP server directly into OpenCode settings (`~/.config/opencode/opencode.json`):

```json
{
  "mcp": {
    "cortex": {
      "command": ["cortex", "mcp", "--tools=agent"],
      "enabled": true
    }
  }
}
```

Verify the setup:
```bash
cortex-ia doctor          # Verifies cortex binary, skills, and configuration health
```

---

## 📚 See Also

- [`architecture.md`](architecture.md) — Cortex-IA multi-agent control plane architecture
- [`sdd-workflow.md`](sdd-workflow.md) — Specification-Driven Development lifecycle
