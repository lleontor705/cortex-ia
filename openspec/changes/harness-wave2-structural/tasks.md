# Tasks — Harness Wave 2 (board `harness-wave2`)

Dependency-safe DAG, 11 tasks, 4 waves. Wave A = T1–T5 (independent), Wave B = T6–T7 (depends on T2), Wave C = T8–T10 (depends on T1), Wave D = T11 (depends on T8). Parallel waves own disjoint `allowed_files`. Review policy: T3, T4, T6 → mandatory independent reviewer (plugin/authority changes, RED-first TestREQ); all other tasks → orchestrator auto-approval (markdown/config per the Adaptive Review Policy).

### T1 w2-01 [assets] Publish normative Case Matrix and LOC budget in work-protocol

Requirements: REQ-W2-001, REQ-W2-002

Files: `internal/assets/skills/_shared/cortex-work-protocol.md`

Add `§2.7a Adaptive Review Case Matrix (Normative)` (full Cases 1–6, mandatory `cortex_ia_work_approve` wording) and extend §4 with the canonical LOC budget table (Go/Rust/Java/C# + TS/Python, 0.2x deletions; strict/flexible/unbounded rows; declarative data/schemas exempt). Replace the protocol's derived matrix summary with the normative statement. No stale `<400 lines` cap anywhere. Depends on: none.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T2 w2-02 [mcpmanager] Enable codemode for the cortex MCP preset

Requirements: REQ-W2-005

Files: `internal/mcpmanager/presets.go`, `internal/mcpmanager/desired.go`, `internal/mcpmanager/adopt_test.go`

Flip `codemode: false` → `codemode: true` for the `cortex` preset and align desired-entry validation. `context7` and the `--tools=agent` command flag stay untouched. Update any test fixtures that pin the old value; adopt/qualification semantics (digests derive from the preset) must stay green. Depends on: none.

Verification: go test -count=1 ./internal/mcpmanager/...

### T3 w2-03 [plugin] Temperature override via session.hook("context") (RED-first)

Requirements: REQ-W2-003

Files: `internal/assets/plugins/cortex-subagent-transport.ts`, `scripts/harness-temperature.test.mjs`

RED-first `TestREQ_W2T_001` (temperature per agent map: orchestrator/planner/implement/discovery 0.2, investigate 0.3, reviewer 0.1; no override for unknown agents) and `TestREQ_W2T_002` (dead `write_to_file`/`apply_patch` tool-prune deletes removed; `edit`/`write` deletes retained). Implement inside the existing `ctx.session.hook("context", …)` handler using an `AGENT_TEMPERATURE` map keyed on `event.agent`. The `request.body` blocks are removed from agent frontmatters by T6/T7. Modular test file ≤ 250 LOC. Depends on: none.

Verification: node --test scripts/harness-temperature.test.mjs scripts/harness-prompt.test.mjs

### T4 w2-04 [plugin] Deterministic compaction possession and retry hook (RED-first)

Requirements: REQ-W2-006

Files: `internal/assets/plugins/cortex-work.ts`, `scripts/harness-compaction.test.mjs`

RED-first `TestREQ_W2C_001`: the compaction hook sets `event.result` with a summary containing the `[CORTEX-IA STATE SNAPSHOT]` DAG marker + continuation pointer; work-state read failure falls back to existing context/message injection. `TestREQ_W2R_001`: a `retry` hook grants `{ retry: true, delay }` (exponential backoff) for 429/quota classes and vetoes at attempt ≥ 3; non-retryable errors pass through unchanged. Modular test file ≤ 250 LOC. Depends on: none.

Verification: node --test scripts/harness-compaction.test.mjs scripts/harness-work-reconcile.test.mjs

### T5 w2-05 [config] Harness config tuning in opencode.jsonc asset

Requirements: REQ-W2-008

Files: `internal/assets/opencode.jsonc`

Add `compaction: { auto: true, keep: { tokens: 15000 }, buffer: 20000 }`, `tool_output: { max_lines: 2000, max_bytes: 51200 }`, `warming: true`, `agents: { general: { disabled: true } }`. Preserve every existing permission entry byte-for-byte. JSONC-valid, v2-schema keys only. Depends on: none.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T6 w2-06 [agents] Authority surface: orchestrator, investigate, planner frontmatter

Requirements: REQ-W2-003, REQ-W2-004

Files: `internal/assets/agents/orchestrator.md`, `internal/assets/agents/investigate.md`, `internal/assets/agents/planner.md`

Remove `request.body.temperature`; drop `write_to_file`/`apply_patch` deny rules (keep `edit`/`write` denies); add `question: deny` to investigate and planner (orchestrator keeps `question: allow`); orchestrator: `subagent` deny `*` then explicit allows for exactly discovery/investigate/planner/implement/reviewer; collapse Cortex graph to one `cortex_*` deny umbrella + `cortex_cortex_*`-form allows only (delete bare `cortex_<tool>` duplicates; keep the `cortex_ia_*` graph); add `steps` budgets (orchestrator 500, investigate 150, planner 150). Deny-first rule ordering is mandatory. Depends on: T2.

Verification: node --test scripts/harness-prompt.test.mjs

### T7 w2-07 [agents] Minion surface: reviewer, implement, discovery frontmatter

Requirements: REQ-W2-003, REQ-W2-004

Files: `internal/assets/agents/reviewer.md`, `internal/assets/agents/implement.md`, `internal/assets/agents/discovery.md`

Same hygiene minus the subagent allowlist: temperature removal, dead v1 actions, `question: deny` on all three, Cortex graph collapse, `steps` budgets (reviewer 150, implement 250, discovery 100). In reviewer.md and implement.md, replace LOC-budget restatements with a pointer to `cortex-work-protocol.md §4`. Depends on: T2.

Verification: node --test scripts/harness-prompt.test.mjs

### T8 w2-08 [contracts] AGENTS.md single-source alignment (both surfaces)

Requirements: REQ-W2-001, REQ-W2-002

Files: `AGENTS.md`, `internal/assets/AGENTS.md`

§2.7 full Case Matrix → compact digest + `Digest — normative source: cortex-work-protocol.md §2.7a` pointer; rule 4 and §3 LOC budget restatements → digest/pointer to protocol §4. Both surfaces stay content-equivalent; unify language sets. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T9 w2-09 [contracts] Derived-copy sweep (skills set 1)

Requirements: REQ-W2-001, REQ-W2-002

Files: `internal/assets/skills/parallel-dispatch/SKILL.md`, `internal/assets/skills/code-review-adversary/SKILL.md`, `internal/assets/skills/implement/SKILL.md`

Replace derived Case-Matrix and LOC-budget copies with pointers to the protocol normative sections; no numeric drift may remain. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T10 w2-10 [contracts] Derived-copy sweep (skills set 2 and shared contracts)

Requirements: REQ-W2-002

Files: `internal/assets/skills/planner/SKILL.md`, `internal/assets/skills/_shared/codebase-design-contract.md`, `internal/assets/skills/_shared/diagnosis-loop-contract.md`

Replace LOC-budget restatements with pointers to protocol §4; correct `TS/Python/Ruby` and `Go/Rust/Java` variants if present. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T11 w2-11 [assets] AGENTS.md prune into autoinvoke-false skill

Requirements: REQ-W2-007

Files: `AGENTS.md`, `internal/assets/AGENTS.md`, `internal/assets/skills/opencode2-knowledge/SKILL.md`

Move §I (OpenCode v2 knowledge index) and the mermaid diagrams (§1 flowchart, §3 sequence, §4 state, §7 guard flow) into the new skill with `autoinvoke: false`; replace with one-line pointers; retain all shared invariants (role matrix, authority boundaries, anti-overengineering rules, mutation gate pointer, status dimensions, shell boundaries); remove the retired `cortex setup claude-code` line from the root copy; target ≤ ~24 KB per AGENTS.md. Keep `internal/assets/inventory_test.go` green (update its pinned counts within this lease if the new skill changes them). Depends on: T8.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

## Post-wave operator actions (not tasks)

Sync install + daemon restart; dedupe `use-railway` across `~/.config/opencode/skills` and `~/.agents/skills`; optional live-config overrides (compaction/tool_output/warming/provider temperature) — see plan §Operator Actions.
