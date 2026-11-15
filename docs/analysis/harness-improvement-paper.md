# Harness Improvement Paper

**Status**: Wave 1 executed in the current change set; Waves 2–3 proposed.
**Scope**: The `cortex-ia` OpenCode v2 development harness — agent prompts, shared skill
contracts, plugin runtime policy, and the Cortex-IA work/decision pipeline.
**Plane**: `hybrid` (OpenSpec for shared specs + Cortex for durable evidence and lineage).
**Source of truth**: Three parallel read-only investigations (OpenCode v2 capability audit,
harness asset audit, decision-pipeline diagnosis) plus repository evidence. Observation IDs
(`#NNN`) reference Cortex memory; file anchors are repository-relative.

---

## Executive summary

The harness is functionally sound when manually driven, but three seams leak and a large
amount of load-bearing policy is duplicated or stale:

1. **Runtime capabilities are under- or mis-configured.** All six agent prompts carry a
   `temperature` value under `request.body` that OpenCode v2 does not send, five agents hold
   a `diagram_render` grant the plugin gate rejects, and several permission lists still use
   retired v1 actions. Fourteen ranked opportunities (Section 2) recover real capability with
   small-to-structural effort.
2. **The asset surface contradicts itself.** Twelve contradictions (C1–C12, Section 3.1) and
   heavy duplication (the Adaptive Review Case Matrix, the workload LOC budget, and a 69 %
   permission-frontmatter share) make edits error-prone and audit-hostile. Fifteen ranked
   proposals split into quick wins and structural single-sourcing.
3. **The decision pipeline works end-to-end but leaks at three seams** (Section 4): the
   knowledge graph is an island, reviewer failure→gotcha extraction is rare (~0.6 %), and
   interview/alignment decisions leave no durable trace.

Section 5 lays out the three-wave roadmap. **Wave 1 lands in this change set**: enum
unification, the C1–C12 corrections, retired-concept purge, plugin type/code sync, the
`blocked_reason` taxonomy and `related` receipt field, and this paper.

---

## 1. Method and sources

- **OpenCode v2 capability audit** — verified against the v2.0.23 documentation and the live
  configuration shipped under `internal/assets/`.
- **Harness asset audit** — line-level scan of `internal/assets/` (10,229 total asset lines;
  2,072 agent lines) with `file:line` anchors for every contradiction.
- **Decision-pipeline diagnosis** — read-only analysis of 483 Cortex observations and the
  `delegation.db` board/task universe, grounded in prior friction records
  (`#290`, `#291`, `#324`, `#364`, `#381`, `#434`, `#445`, `#448`, `#484`).
- **Grounding rule**: no finding is invented here. Every claim maps either to a Cortex
  observation or to a repository anchor reproduced in the tables below.
- **Limitations**: the paper records the *audited* state. Wave 1 corrections are applied
  alongside this document; where a contradiction is listed below it describes the pre-fix
  state intentionally, and Section 5.1 records what Wave 1 changed.

---

## 2. OpenCode v2 capability audit

Fourteen ranked opportunities, with the v2 mechanism and an implementation effort estimate
(`S` ≤ half a day, `M` ~1–2 days, `L` multi-day / structural).

| # | Opportunity | v2 mechanism | Effort |
|---|---|---|---|
| 1 | `temperature` is dead configuration | Every agent sets `request.body.temperature` (`internal/assets/agents/*.md:7`), which OpenCode v2 does not transmit. Remove the block or migrate to a provider option that v2 honors; leaving it is silently misleading. | S |
| 2 | Harden delegation surface | Constrain the `subagent` permission action to an explicit role allowlist and set `general: disabled` so only the six declared controllers are dispatchable. | S |
| 3 | Deny `question` for minions | Minions must not block on interactive prompts. `question: deny` on implement/investigate/planner/reviewer/discovery forces clarification back through the orchestrator. | S |
| 4 | File leases at the runtime layer | Express per-file write leases through `ctx.permission.rules` so a write without a live reservation fails at the permission layer, not only in tool code. | L |
| 5 | Deterministic compaction | Own compaction via `event.result` possession so session summarization is a controlled hook rather than an opaque runtime action. | M |
| 6 | Prune `AGENTS.md` | `internal/assets/AGENTS.md` is 42.7 KB and is injected on every dispatch. Move detail into `autoinvoke: false` skills loaded on demand by task trigger. | M |
| 7 | `codemode: true` for cortex MCP | Enable code-mode for the Cortex MCP tools to cut per-call round-trip overhead in tool-heavy roles. | S |
| 8 | `steps` budgets | Set bounded `steps` per role to cap runaway loops deterministically. | S |
| 9 | `retry` hook for 429s | A `retry` hook with backoff for 429/quota responses keeps controllers alive across transient throttling. | S |
| 10 | `ctx.session.synthetic` guardrails | Use `ctx.session.synthetic` to mark/deny synthetic sessions so they cannot touch durable work state. | M |
| 11 | Compaction tuning | Tune `compaction.keep` and `tool_output` retention to keep evidence while trimming bulk output. | S |
| 12 | Remove dead v1 permission actions | `write`, `write_to_file`, and `apply_patch` are v1 actions still present in four agents (`orchestrator.md:21-29`, `investigate.md:18-21`, `planner.md:18-21`, `reviewer.md:18-21`); v2 uses `write`+`edit`. | S |
| 13 | De-duplicate `use-railway` | The `use-railway` skill is registered more than once across installed skill locations, double-loading guidance. | S |
| 14 | `warming` | Enable `warming` to pre-warm providers and remove first-call latency from interactive gates. | S |

**Reading the table**: opportunities 1, 2, 3, 7–14 are low-risk configuration hygiene;
4, 5, 6, 10 are the structural items that also appear in Wave 2 (Section 5.2).

---

## 3. Harness asset audit

### 3.1 Contradictions C1–C12

Each contradiction has a repository anchor and a Wave 1 resolution (Section 5.1).

| ID | Contradiction | Anchor (file:line) | Wave 1 fix |
|---|---|---|---|
| C1 | `discovery` is declared a *mandatory* session task in the workflow map but is dispatched *on demand* in AGENTS.md rule 0. | `internal/assets/skills/_shared/workflow-map.md:8` vs `internal/assets/AGENTS.md` §1 rule 0 | Rewrite the workflow-map row to on-demand |
| C2 | `speckit` is half-documented: `spec_plane` accepts it and the convention/workflow map describe it, while AGENTS.md declares `openspec|cortex|hybrid`. | `internal/assets/plugins/cortex-work.ts:1044`; `internal/assets/skills/_shared/cortex-convention.md:5,37`; `internal/assets/skills/_shared/workflow-map.md:45` | Purge speckit from contract plane and plugin enum |
| C3 | `next_route` is expressed in seven-plus divergent vocabularies across skill and agent receipts. | canonical intent at `…/cortex-work-protocol.md:164`; divergences at `agents/investigate.md:311`, `skills/code-review-adversary/SKILL.md:105`, `skills/discovery/SKILL.md:83`, `skills/fast-tdd/SKILL.md:47`, `skills/hotfix-triage/SKILL.md:51`, `skills/investigate/SKILL.md:99`, `skills/planner/SKILL.md:181`, `skills/spike-prototype/SKILL.md:37`, `skills/workflow-retrospective/SKILL.md:45` | Publish one canonical enum in the protocol; align all receipts |
| C4 | `cortex_save` guidance recommends `type: "observation"`, which is not in the allowlist. | `internal/assets/agents/investigate.md:312` vs `…/cortex-convention.md:25` | Replace with canonical types |
| C5 | `diagram_render` is granted to five agents the plugin gate rejects (implement-only), so those grants are dead. | grants at `discovery.md:114`, `implement.md:135`, `investigate.md:219`, `orchestrator.md:132`, `planner.md:183`, `reviewer.md:156`; gate in `internal/assets/plugins/cortex-work.ts` | Remove grants outside `implement.md` |
| C6 | `npm install*` is denied and then allowed in the same frontmatter. | `internal/assets/agents/implement.md:191,211` | Delete dead rule; keep the intended install policy |
| C7 | TTL text drifted: the protocol and plugin say 30 m, the agent example and tool schema say 15 m. | `…/cortex-work-protocol.md:114`, `plugins/cortex-work.ts:131` vs `agents/implement.md:265`, `plugins/cortex-work.ts:1237` | Align examples/schema to 30 m |
| C8 | `planner`/`orchestrator` are told to run `cortex-ia` CLI commands although neither has shell. | `orchestrator.md:185,190`; `planner.md:203,232` | Replace with typed `cortex_ia_*` tools |
| C9 | `ops-task` is absent from the protocol's work-approval authority lists while AGENTS.md Case 4 includes it. | `…/cortex-work-protocol.md` orchestrator and approve rows vs `AGENTS.md` Case 4 | Add `ops-task` to the authority lists |
| C10 | `parallel-dispatch` still ships the legacy `<minion-contract>` envelope with retired fields. | `internal/assets/skills/parallel-dispatch/SKILL.md:48-60` (`worktree`, `budget_tier`) | Replace with the canonical v2.0 dispatch schema |
| C11 | `parallel-dispatch` instructs an "Announce at start" banner, contradicting the zero-chatter contract. | `internal/assets/skills/parallel-dispatch/SKILL.md:18` | Delete the instruction |
| C12 | `Recomendación` appears in instructions despite the Language Domain Contract (artifacts in English). | `internal/assets/AGENTS.md:97`; `internal/assets/skills/grill-me/SKILL.md:31,37` | Rename to `Recommendation` |

### 3.2 Duplication

| Duplicated policy | Copies / anchors | Problem |
|---|---|---|
| Adaptive Review Case Matrix | 2 full restatements (`internal/assets/AGENTS.md` §2.7, `internal/assets/agents/orchestrator.md`) plus derived summaries in `…/cortex-work-protocol.md` and `skills/parallel-dispatch/SKILL.md` | Four copies drift independently; the approval policy is security-relevant |
| Workload LOC budget | Restated 8+ times: `AGENTS.md:91-92,213,257-258,266,270`; `agents/implement.md:275`; `agents/reviewer.md:421`; `…/cortex-work-protocol.md:116`; `…/codebase-design-contract.md:68`; `…/diagnosis-loop-contract.md:31`; `skills/code-review-adversary/SKILL.md:52`; `skills/implement/SKILL.md:63-64`; `skills/planner/SKILL.md:139-140`; `commands/sdd.md:7` | Inconsistent language sets (`Go/Rust/Java` vs `Go/Rust/Java/C#` vs `TS/Python/Ruby`) and a **stale `<400 lines` cap** at `commands/sdd.md:7` |
| Agent permission frontmatter | 69 % of agent lines are frontmatter (1,421 of 2,072); each agent repeats a deny-all `cortex_*` / `cortex_cortex_*` / `cortex_ia_*` graph followed by per-tool allows | ~300 redundant lines per dispatch; the doubled tool graph is error-prone (see C5, C12) |

### 3.3 Ranked proposals

| Rank | Proposal | Class | Contradiction(s) |
|---|---|---|---|
| 1 | Purge retired concepts (speckit, Herdr, AGY, worktree, `cortex setup claude-code`) to single retirement notes | Quick win | C2 |
| 2 | Unify `next_route` to one canonical enum in the protocol and all receipts | Quick win | C3 |
| 3 | Correct `cortex_save` type guidance to the allowlist | Quick win | C4 |
| 4 | Remove `diagram_render` grants outside `implement.md` | Quick win | C5 |
| 5 | Delete dead v1 permission actions (`write`, `write_to_file`, `apply_patch`) | Quick win | C12/opp 12 |
| 6 | Resolve the npm deny/allow contradiction | Quick win | C6 |
| 7 | Align TTL examples and tool schema to 30 m | Quick win | C7 |
| 8 | Replace CLI-where-no-shell guidance with typed tools | Quick win | C8 |
| 9 | Add `ops-task` to the protocol approval authority lists | Quick win | C9 |
| 10 | Canonicalize the `parallel-dispatch` envelope (drop `worktree`/`budget_tier`) | Quick win | C10 |
| 11 | Remove "Announce at start"; enforce zero-chatter | Quick win | C11 |
| 12 | Rename `Recomendación` → `Recommendation` | Quick win | C12 |
| 13 | Single-source the Adaptive Review Case Matrix (one contract + pointers) | Structural | Duplication |
| 14 | Single-source the workload LOC budget; delete the stale 400-line cap | Structural | Duplication |
| 15 | Split permission frontmatter from role prose; collapse the doubled `cortex_*` graph | Structural | Duplication |

**Split**: proposals 1–12 are Wave 1 quick wins; 13–15 are Wave 2 structural work
(Section 5.2).

---

## 4. Decision-pipeline diagnosis

### 4.1 The loop works when manually driven

The v0.5.8 chain is a clean end-to-end example: triage (`#434`) → decisions
(`#444`/`#445`/`#446`) → release (`#448`, commit `7aa0227`) → linked issues closed. Recovery
friction that used to require the orchestrator (`#246`, `#214`, `#324`) was reduced by
scoped self-recovery (`#445`). The pipeline is *correct*; the leaks are systemic, not
localized bugs.

### 4.2 Three structural leaks

| Leak | Evidence | Consequence |
|---|---|---|
| **The knowledge graph is an island** | `cortex_relate` is effectively unused: decision hubs `#434` and `#448` carry **zero** graph edges despite being a direct causal chain (`#484`) | Retrieval falls back to FTS/vector only; multi-hop HippoRAG value is lost |
| **Failure→gotcha extraction is rare** | `type=bugfix` observations are ~3 of 483 (`#5`, `#20`, `#291`); the protocol's reviewer FAIL → `gotchas/<task_id>` step is under-applied (`#484`) | Root causes are not durably captured, so later tasks re-derive them |
| **Interview/alignment decisions are ephemeral** | `grill-me` interview rounds appear **zero** times across 483 observations (`#484`) | Design rationale from grilling is unmeasurable and unrecoverable |

### 4.3 Friction archaeology

- **TTL defect latency (≈4 days, 3 releases)**: signal on v0.5.5 (2026-10-03, `#324`) →
  root cause diagnosed as claim TTL coupled to `stale_progress_ms` (`#434`) → fixed in v0.5.8
  (`7aa0227`, `#448`, 2026-10-07).
- **Report-hub duplicate noise (5 issues in 26 h)**: `#122`, `#129`, `#130`, `#131`, `#132`
  minted by a task-scoped dedup signature; root-caused in `#290` and fixed by coalescing
  (`71bd808`, `#291`).
- **Vocabulary drift misclassifies severity**: `ERR_DELEGATION_FAIL` (emitter) vs
  `ERR_DELEGATION_FAILURE` (severity table) left a report low/unclassified instead of
  high/needs-investigation (`#364`). The classifier later treats both identically
  (`c4fb4ae`, `#381`), but the dual vocabulary and the absence of inline error-code
  validation remain — motivating E2.
- **Zombie blocked tasks**: 10 blocked tasks across 5 boards, two stale >1 week — the oldest
  (`pi-canvas-release-v011`, 2026-09-12) is 26 days old — with no machine-readable
  `blocked_reason` in the projection (`#484`).
- **Read-only aborts never trip the breaker**: repeated read-only-role aborts emit
  `ERR_SUBAGENT_READONLY_REPEATED_ABORT` telemetry once per streak (`#381`,
  `cortex-task-latch.ts:370-388`) but never trip the subagent circuit breaker
  (`cortex-task-latch.ts:345-349`, `#364`), so a failing diagnostic loop stays manual.
- **Bridge authority friction**: one claim per session even when the durable claim is dead
  (`#252`); review-binding drift after a branch switch requires a fresh review (`#284`);
  claim-TTL expiry on verified work needs diff-only reconciliation (`#214`).

### 4.4 Proposals

**Prompt-level (P)**

- **P1** — Make FAIL→gotcha extraction mandatory: the reviewer receipt must carry the
  `related: <obs-ids> | none` line, and every FAIL must emit a `gotchas/<task_id>`
  observation.
- **P2** — Create graph edges at write time: require `cortex_relate` for every decision and
  gotcha to kill the island graph.
- **P3** — Persist interview/alignment outcomes: have `grill-me` close with a durable
  observation so design rationale is measurable.

**Protocol-level (R)**

- **R1** — Hub-side ingest-time classification: classify recurring incident classes at
  ingestion and auto-coalesce/auto-resolve, replacing post-hoc cron triage.
- **R2** — Machine-readable `blocked_reason`
  (`authority_expired | upstream | needs_user | env | scope_drift`) in blocked transitions
  and the work projection.
- **R3** — Board/TTL hygiene: sweep stale blocked tasks and codify TTL/review-binding
  reconciliation so zombie state is visible and recoverable.
- **R4** — Decision-pipeline metrics: instrument graph-edge coverage on decision hubs,
  bugfix-extraction rate, and interview capture; surface them on the console.

**Enforcement-level (E)**

- **E1** — Gotcha-evidence transition gate: a FAIL-driven rework cannot reach `in_review`
  without a linked gotcha observation.
- **E2** — Error-code validation gate: emitted codes are validated against one map; unknown
  codes are downgraded and logged, never misclassified (`#364`).
- **E3** — Runtime file leases via `ctx.permission.rules`, so unauthorised writes fail at the
  permission layer (audit opportunity 4).

---

## 5. Three-wave roadmap

### 5.1 Wave 1 — executed in this change

| Item | Delivered |
|---|---|
| Enum unification | Canonical `next_route` enum published once in `cortex-work-protocol.md`; all receipts aligned (C3) |
| C1–C12 corrections | On-demand discovery (C1); speckit purged (C2); save-type fixed (C4); `diagram_render` grants trimmed to `implement.md` (C5); npm rules de-duplicated (C6); TTL examples to 30 m (C7); typed tools replace CLI (C8); `ops-task` added to authority lists (C9); canonical envelope (C10); no announce banner (C11); `Recommendation` (C12) |
| Retired-concept purge | Herdr, AGY, `isolated_worktree`, speckit reduced to single retirement notes |
| Plugin type/code sync | `cortex_save` type list aligned to the convention allowlist; speckit rejected by the `spec_plane` enum; E2 error-code validation gate |
| Receipt fields | `blocked_reason` taxonomy (R2) and the `related: <obs-ids> | none` reviewer line added |
| Analysis paper | This document |

### 5.2 Wave 2 — structural single-sourcing

- Single-source the Adaptive Review Case Matrix (proposal 13).
- Single-source the workload LOC budget and delete the stale `<400 lines` cap (proposal 14).
- Permission hygiene: split permission frontmatter from prose, collapse the doubled
  `cortex_*` graph (proposal 15), and remove dead v1 actions (opportunity 12).
- Migrate the dead `temperature` config to a v2-honored mechanism (opportunity 1).
- Prune `AGENTS.md` into `autoinvoke: false` skills (opportunity 6), enable `codemode` for
  Cortex MCP (opportunity 7), and adopt deterministic compaction via `event.result`
  (opportunity 5).

### 5.3 Wave 3 — enforcement

- E3: runtime file leases through `ctx.permission.rules` (opportunity 4).
- E1: gotcha-evidence transition gate.
- R1: hub-side ingest-time incident classification.
- R3: board/TTL hygiene — stale blocked-task sweep and reconciliation.
- R4: decision-pipeline metrics on the console.

---

## Appendix A — Linked observations

`#214` (TTL recovery reconciliation) · `#246` (claim-TTL block) · `#252` (bridge single-claim
handle) · `#284` (review-binding drift) · `#290` / `#291` (report-hub duplicate root cause and
coalescing fix) · `#324` (lease-required triage) · `#364` (vocabulary drift severity) · `#381`
(read-only abort telemetry) · `#434` (TTL root-cause triage) · `#445` (scoped recover
authority) · `#448` (v0.5.8 release) · `#484` (decision-pipeline diagnosis).

## Appendix B — Audit scope and limitations

- Asset audit counted `internal/assets/**` markdown and TypeScript sources (10,229 lines);
  agent frontmatter share computed across the six `internal/assets/agents/*.md` files
  (1,421 / 2,072 lines).
- Contradiction anchors reflect the pre-Wave-1 state; Wave 1 corrections are listed in
  Section 5.1.
- The paper records findings only; no new empirical claims beyond the investigations and the
  repository anchors reproduced above.
