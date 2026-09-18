# KV-Cache Prefix Optimization for LLM Agents

A technical guide to maximizing Key-Value (KV) cache reuse in agentic systems, minimizing Time-To-First-Token (TTFT), and reducing operational inference costs.

---

## 1. How KV Caching Works in Modern Inference Engines

Modern frontier LLMs and high-throughput inference engines (vLLM with PagedAttention, SGLang with RadixAttention, Anthropic Prompt Caching, Google Vertex Context Caching) store the computed Key and Value matrices of past tokens in GPU High-Bandwidth Memory (HBM).

### The Radix Tree Prefix Match
When an inference request arrives, the server hashes token sequences in blocks (typically 16 to 64 tokens per block) and performs a longest-prefix match against a radix tree in memory:

```
[Root: Model Weights]
       |
       v
[Block 0..63: Static System Prompt Header]  <--- HASH MATCH! (Cache Hit: 0 ms)
       |
       v
[Block 64..127: Tool Definitions & Invariants] <--- HASH MATCH! (Cache Hit: 0 ms)
       |
       v
[Block 128..191: Global Architecture Contracts] <--- HASH MATCH! (Cache Hit: 0 ms)
       |
       +------------------------------------+
       |                                    |
       v                                    v
[Task A Envelope] (Miss)            [Task B Envelope] (Miss)
       |                                    |
[Evaluate Tokens] (Slow)            [Evaluate Tokens] (Slow)
```

If even a **single character or token** changes at token position 100 (e.g., injecting an ephemeral timestamp `2026-09-18T14:48:00Z`), the hash mismatch **invalidates all subsequent blocks** (tokens 101 to $N$).

---

## 2. Quantitative Economic and Latency Impact

### Performance Metrics
- **Time-to-First-Token (TTFT)**:
  - Cache Miss on 4,000 tokens: $\approx 1,200 \text{ ms} - 2,500 \text{ ms}$.
  - Cache Hit on 4,000 tokens: $\approx 50 \text{ ms} - 150 \text{ ms}$ (up to **20x faster**).
- **Cost Reduction**:
  - Frontier APIs (Anthropic Claude 3.5/3.7, Google Gemini 1.5/2.0 Flash/Pro) discount cached tokens by **75% to 90%**.
  - Across a 20-step multi-agent task, stabilizing the prompt prefix saves **70%+ of total session inference cost**.

---

## 3. The 4-Zone Canonical Token Layout

To maximize cache hits across diverse agents, organize all prompts into 4 discrete zones:

| Zone | Content | Lifetime / Churn | Cache Target |
|---|---|---|---|
| **Zone 1: Global Static** | `<identity>`, `<capabilities_and_tools>`, `<hard_invariants>` | Permanent across all sessions & users | 100% |
| **Zone 2: Project Static** | Project architecture, coding standards, `.cortex-ia/discovery.md` | Semi-permanent; changes only on codebase evolution | 90%+ |
| **Zone 3: Task Dynamic** | `<minion-dispatch>`, `<task_envelope>`, `<non_goals>` | Stable for the duration of a single task node | 80%+ across retries |
| **Zone 4: Turn Ephemeral** | Tool call inputs/outputs, `<failure_trace>`, user chat messages | Appended strictly at the end of context | 0% (Fresh tokens) |

---

## 4. Antipatterns That Destroy Cache Reusability

### ❌ Antipattern 1: Header Timestamps
```markdown
# Bad Prompt Layout
You are the Cortex-IA Implementer.
Current Time: 2026-09-18 14:48:12 UTC  <--- KILLS CACHE ON EVERY REQUEST!
Rules:
1. Always write unit tests...
```
**Fix**: If the agent requires temporal awareness, provide it via a dedicated `get_system_time()` tool call or append the time in Zone 4 at the very end of the user turn.

### ❌ Antipattern 2: Non-Deterministic JSON Key Ordering
```json
// Request 1
{"intent": "fix bug", "task_id": "T123"}
// Request 2
{"task_id": "T124", "intent": "fix bug"}
```
**Fix**: Use canonical JSON serialization with sorted keys (`sort_keys=True` in Python, `sortKeys()` in JS/TS) or static XML tags.

### ❌ Antipattern 3: Dynamic UUIDs in System Identity
```markdown
# Bad Prompt Layout
You are Agent instance: 9b2d8e41-6a3f-4e08-bc21-c48f2b57e93a  <--- KILLS CACHE!
```
**Fix**: Keep `<identity>` strictly generic. Pass conversation and task IDs inside the dynamic execution envelope in Zone 3.

---

## 5. Verification Checklist for Cache Efficiency

Run this prompt audit before saving any agent role file:
1. Does the first 2,000 tokens contain any dynamically generated string (date, random hash, process ID)?
2. Are tool descriptions completely identical across all agents sharing the toolset?
3. Are multi-agent contracts ordered deterministically (`identity` $\to$ `tools` $\to$ `invariants` $\to$ `protocols`)?
4. Is new turn evidence appended to the bottom rather than prepended to the top?
