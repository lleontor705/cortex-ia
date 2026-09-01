# MCP and Local Control Boundaries

Cortex-IA cleanly separates the **Epistemic & Evidence Plane** (Cortex MCP) from the **Operational Control Plane** (`cortex-ia work` CLI).

| Surface | Runtime | Owns | Must NOT Own |
|---|---|---|---|
| **Cortex MCP** | `cortex mcp --tools=agent` | AST symbol graph, blast radius impact trees, durable observations, gotchas, session context | Task readiness, claims, file leases, DAG transitions, approvals |
| **Context7 MCP** | optional `npx -y @upstash/context7-mcp` | Read-only library & framework documentation | Persistence, execution authority, task state |
| **Cortex-IA Work Control** | Native Go CLI + SQLite WAL | Task DAG, CAS revisions, TTL claims, exclusive file leases, recovery, review approvals | Long-form evidence or raw agent transcripts |
| **Cortex-IA Task Board** | Embedded HTTP UI (`cortex-ia web`) | Board grouping, real-time SSE Kanban visualization, task intake | Authorization grants, remote exposure |
| **OpenSpec SDD** | Repository Markdown (`openspec/`) | Human-readable proposals, delta requirements, designs, task decompositions | Runtime locks, process supervision |
| **Delegation Bridge** | Built-in Go CLI + Herdr plugin | Background/multiplexed worker lifecycle, real-time NDJSON stream telemetry, receipts | Task approval, Cortex session ownership |

---

## Invariant Rules

1. **`cortex-ia work status` is strictly authoritative**: Task readiness and active authority are read solely from SQLite. Browser card positions or chat messages never substitute for `work status`.
2. **Cortex observations are advisory**: Knowledge graph nodes or stored gotchas provide context but never grant permission to touch files or bypass review gates.
3. **Tokens stay in memory**: `claim_token` and `lease_token` reside in live memory only. SQLite stores only their SHA-256 digests.
4. **External workers are execution-only**: Workers delegated through Herdr receive neither work control tokens nor session ownership capabilities.
