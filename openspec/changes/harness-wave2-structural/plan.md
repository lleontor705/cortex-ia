# Plan — Harness Wave 2: Structural Single-Sourcing, Permission Hygiene, and Config Tuning

Workflow: sdd-lite (integrated). Board: `harness-wave2`. Spec plane: hybrid. Workload policy: flexible. Source paper: `docs/analysis/harness-improvement-paper.md` (§2 opportunities 1, 2, 3, 6–9, 11–14; §3.2 duplication proposals 13–15; §5.2 Wave 2 roadmap).

## Intent

Land the Wave 2 structural layer of the harness improvement initiative: (a) single-source the Adaptive Review Case Matrix and the workload LOC budget with `internal/assets/skills/_shared/cortex-work-protocol.md` as the normative home; (b) harden agent permission frontmatters (temperature migration, dead v1 action removal, `question` deny for minions, orchestrator `subagent` allowlist, doubled `cortex_*` graph collapse); (c) prune `AGENTS.md` into an on-demand skill; (d) enable `codemode` for the Cortex MCP server and tune compaction/tool-output/warming config; (e) adopt deterministic compaction possession (`event.result`), a `retry` hook, and per-agent `steps` budgets. Nothing in this wave executes runtime behavior changes by itself: all changes are assets, plugins, and the MCP preset, proven by build gates and node harness tests.

User value: policy edits stop drifting (one normative home per policy), the delegation surface is hardened at the permission layer, every session stops paying the 42.7 KB AGENTS.md injection for content needed only on demand, and Cortex MCP round-trips drop via Code Mode.

Non-goals: no Wave 3 enforcement items (E1 gotcha gate, E3 `ctx.permission.rules` file leases, R1, R3, R4); no hq-06 interference (the Go-side speckit + severity-map change is in flight and its files/vocabulary are untouched); no live-config edits by minions (operator-only knobs are listed in Operator Actions); no retired-surface restoration (ForgeSpec, Herdr-as-executor, AGY, external execution leaves, `isolated_worktree`); no behavior-level rewrite of stable inlined contracts (marked digests with normative pointers are permitted for KV-cache stability; full restatements and drifting derived copies are what get eliminated); no context7 codemode change.

Risks: codemode × permission interaction could gate minion Cortex calls differently (mitigation: post-sync live smoke, one-field rollback); compaction possession could lose conversational nuance (mitigation: fallback injection retained + `keep.tokens` tail); AGENTS.md prune could over-trim (mitigation: REQ-W2-007 enumerates mandatory retained sections); `internal/assets/inventory_test.go` pins asset counts (mitigation: keep the suite green or update pins within the same lease).

## Requirements

### Requirement: REQ-W2-001 Normative Adaptive Review Case Matrix (single source)

The Adaptive Review Case Matrix (Cases 1–6 with auto-approval and reviewer-dispatch policy) SHALL be published exactly once, in full, in `internal/assets/skills/_shared/cortex-work-protocol.md` (new §2.7a). All other sites SHALL carry either a compact digest marked `Digest — normative source: cortex-work-protocol.md §2.7a` (permitted only in `AGENTS.md` §2.7 and `agents/orchestrator.md` for KV-cache stability) or a pure pointer. Full restatements outside the protocol are FORBIDDEN.

Test: go build -o bin/cortex-ia ./cmd/cortex-ia

#### Scenario: Reviewer resolves an auto-approval dispute from the protocol

- **GIVEN** a reviewer needs the authoritative approval policy
- **WHEN** it opens `cortex-work-protocol.md` §2.7a
- **THEN** the full Case 1–6 policy is present with the mandatory `cortex_ia_work_approve` wording

#### Scenario: Digest sites do not drift

- **GIVEN** the AGENTS.md §2.7 digest and the protocol normative text
- **WHEN** they are compared
- **THEN** the digest contains no numeric threshold or case rule that contradicts the protocol

#### Scenario: No full restatement remains outside the protocol

- **GIVEN** any harness asset outside `cortex-work-protocol.md`
- **WHEN** it is searched for the full six-case matrix wording
- **THEN** only a marked digest or a pure pointer form exists

### Requirement: REQ-W2-002 Normative workload LOC budget (single source)

The workload LOC budget table SHALL be published exactly once in `cortex-work-protocol.md` §4 with the canonical language sets: source logic Go/Rust/Java/C# and TS/Python (deletions weighted 0.2x); test/fixture budgets per `strict|flexible|unbounded`; declarative data/schemas exempt. Derived sites (`AGENTS.md` rules 4/§3, `agents/implement.md`, `agents/reviewer.md`, `skills/_shared/codebase-design-contract.md`, `skills/_shared/diagnosis-loop-contract.md`, `skills/code-review-adversary/SKILL.md`, `skills/implement/SKILL.md`, `skills/planner/SKILL.md`) SHALL point to the protocol section; `AGENTS.md` may keep a marked digest. The stale `<400 lines` cap SHALL NOT reappear anywhere.

Test: go build -o bin/cortex-ia ./cmd/cortex-ia

#### Scenario: Consistent budgets across all assets

- **GIVEN** an implementer checks its churn budget under `flexible`
- **WHEN** it reads any harness asset
- **THEN** every site states the same 700/500 (Go-family/TS-family) caps or points to the protocol §4

#### Scenario: Stale cap is absent

- **GIVEN** the Wave 2 change set
- **WHEN** the repository is searched for the `<400 lines` cap
- **THEN** zero authoritative-policy matches remain outside the analysis paper's historical record

#### Scenario: Language sets are unified

- **GIVEN** the budget wording in the protocol §4 and every derived pointer
- **WHEN** the language lists are compared
- **THEN** only Go/Rust/Java/C# and TS/Python appear, with no `TS/Python/Ruby` or C#-less variant remaining

### Requirement: REQ-W2-003 Temperature migration to the v2-honored hook

All six `internal/assets/agents/*.md` files SHALL remove the dead `request.body.temperature` block. The transport plugin (`internal/assets/plugins/cortex-subagent-transport.ts`) SHALL, inside its existing `ctx.session.hook("context", …)` registration, set `event.options.temperature` discriminated by `event.agent`: orchestrator 0.2, planner 0.2, implement 0.2, discovery 0.2, investigate 0.3, reviewer 0.1; unknown/synthetic agents SHALL receive no override. The plugin's dead tool-prune deletes for `write_to_file` and `apply_patch` SHALL be removed (v2 exposes neither) while `edit`/`write` deletes are retained. RED-first: `TestREQ_W2T_*` must fail before the hook change and pass after.

Test: node --test scripts/harness-temperature.test.mjs

#### Scenario: Reviewer requests carry its mapped temperature

- **GIVEN** a reviewer child session dispatches a model call
- **WHEN** the context hook runs
- **THEN** `event.options.temperature === 0.1`

#### Scenario: Unknown agents get no override

- **GIVEN** an agent name with no mapping or a synthetic session
- **WHEN** the context hook runs
- **THEN** `event.options.temperature` is unset

#### Scenario: Dead frontmatter block is gone

- **GIVEN** any of the six agent files
- **WHEN** its frontmatter is parsed
- **THEN** no `request.body.temperature` key exists

### Requirement: REQ-W2-004 Permission frontmatter hygiene (all six agents)

Each agent frontmatter SHALL (a) drop retired v1 actions `write_to_file` and `apply_patch` (keep v2 `write`/`edit` denies where present); (b) add `{ action: question, resource: "*", effect: deny }` to the five non-orchestrator agents while the orchestrator keeps `question` allowed; (c) collapse the doubled Cortex graph to one `cortex_*` deny umbrella plus explicit allows ONLY in the live normalized form `cortex_cortex_*` for MCP-server tools (bare `cortex_save`-form allow rules are dead duplicates and are removed) while keeping the `cortex_ia_*` plugin-tool graph; (d) constrain the orchestrator with `{ action: subagent, resource: "*", effect: deny }` followed by explicit allows for exactly `discovery`, `investigate`, `planner`, `implement`, `reviewer`; (e) declare a bounded `steps` budget: orchestrator 500, implement 250, reviewer 150, investigate 150, planner 150, discovery 100 (tunable defaults; last matching permission rule wins, so deny-first ordering is mandatory).

Test: node --test scripts/harness-prompt.test.mjs

#### Scenario: Built-in general subagent is not dispatchable

- **GIVEN** the orchestrator tries to launch the built-in `general` subagent
- **WHEN** the permission engine evaluates the ordered rules
- **THEN** the launch is denied

#### Scenario: Step budget stops runaway loops

- **GIVEN** `implement` reaches its declared step budget
- **WHEN** the final step arrives
- **THEN** OpenCode removes tools and asks for a text summary instead of looping indefinitely

#### Scenario: Minions cannot block on interactive prompts

- **GIVEN** any minion agent invokes `question`
- **WHEN** the permission engine evaluates
- **THEN** the call is denied and clarification routes back through the orchestrator

#### Scenario: Cortex permission graph is collapsed to live names

- **GIVEN** any agent frontmatter
- **WHEN** its Cortex tool rules are inspected
- **THEN** exactly one `cortex_*` deny umbrella exists, all MCP allow rules use the `cortex_cortex_*` form, and the `cortex_ia_*` graph is unchanged

### Requirement: REQ-W2-005 codemode for the Cortex MCP preset

`internal/mcpmanager/presets.go` SHALL set `codemode: true` for the `cortex` preset (and the mirrored desired-entry validation in `internal/mcpmanager/desired.go` SHALL be aligned). The `context7` preset and the legacy command flag `--tools=agent` SHALL remain untouched. Existing adopt/qualification tests SHALL stay green.

Test: go test -count=1 ./internal/mcpmanager/...

#### Scenario: Fresh install writes codemode true

- **GIVEN** a fresh `cortex-ia install` or `mcp add cortex`
- **WHEN** the entry is written
- **THEN** the cortex server object carries `codemode: true`

#### Scenario: Qualification stays conflict-free

- **GIVEN** the qualification probe compares the managed preset
- **WHEN** it evaluates the cortex entry
- **THEN** no conflict error is produced and the suite passes

#### Scenario: context7 preset is untouched

- **GIVEN** the context7 preset entry
- **WHEN** the preset catalog is inspected
- **THEN** its command, digest inputs, and `codemode: false` value are unchanged

### Requirement: REQ-W2-006 Deterministic compaction possession and retry hook

In `internal/assets/plugins/cortex-work.ts`, the `compaction` session hook SHALL set `event.result` to a structured summary composed of the work-DAG state snapshot (`cortex-ia work list`) plus a continuation pointer, so summarization is owned rather than an opaque runtime model call; on work-state read failure it SHALL fall back to the existing context/message injection. A `retry` session hook SHALL grant or veto retries for provider failures with exponential backoff for 429/quota classes and a hard veto after attempt 3. RED-first: `TestREQ_W2C_*` and `TestREQ_W2R_*` in a new modular harness test file.

Test: node --test scripts/harness-compaction.test.mjs

#### Scenario: Compaction is possessed

- **GIVEN** a compaction event fires
- **WHEN** the hook completes
- **THEN** `event.result.summary` contains the `[CORTEX-IA STATE SNAPSHOT]` DAG marker

#### Scenario: Transient 429 retries with backoff and caps out

- **GIVEN** a 429 provider failure on attempt 2
- **WHEN** the retry hook runs
- **THEN** `event.decision.retry === true` with a positive delay, and at attempt ≥ 3 the decision is `retry: false`

#### Scenario: Read failure degrades safely

- **GIVEN** the work-state read fails during compaction
- **WHEN** the hook runs
- **THEN** the legacy context/message injection path is used and no exception escapes the hook

### Requirement: REQ-W2-007 AGENTS.md prune into an on-demand skill

A new skill `internal/assets/skills/opencode2-knowledge/SKILL.md` (frontmatter `autoinvoke: false`) SHALL receive the OpenCode v2 knowledge index (§I) and the mermaid flow diagrams (§1 startup flowchart, §3 sequence, §4 state diagram, §7 guard flow). Both `AGENTS.md` surfaces (repo root and `internal/assets/AGENTS.md`) SHALL replace those sections with one-line pointers, retain all shared invariants (role matrix, authority boundaries, anti-overengineering rules, mutation gate pointer, status dimensions, shell boundaries), and target ≤ ~24 KB. The two surfaces SHALL be reconciled (the root copy still carries the retired `cortex setup claude-code` line — remove it).

Test: go build -o bin/cortex-ia ./cmd/cortex-ia

#### Scenario: Session injection is slimmed

- **GIVEN** a session starts
- **WHEN** AGENTS.md is injected
- **THEN** the v2 doc-index and diagram bodies are absent and a pointer to the skill exists

#### Scenario: Knowledge remains reachable on demand

- **GIVEN** an agent needs the v2 doc index
- **WHEN** it loads `opencode2-knowledge`
- **THEN** the full index is present in the skill body

#### Scenario: Surfaces are reconciled

- **GIVEN** the repo-root AGENTS.md and the embedded internal/assets/AGENTS.md
- **WHEN** both are inspected after the prune
- **THEN** neither contains the retired `cortex setup claude-code` line and both are ≤ ~24 KB with identical section structure

### Requirement: REQ-W2-008 Harness config tuning in the repo-owned asset

`internal/assets/opencode.jsonc` SHALL add: `compaction: { auto: true, keep: { tokens: 15000 }, buffer: 20000 }`, `tool_output: { max_lines: 2000, max_bytes: 51200 }`, `warming: true`, and `agents: { general: { disabled: true } }`. These keys are v2-schema-confirmed; the file remains a managed KindConfig asset and every existing permission entry is preserved byte-for-byte.

Test: go build -o bin/cortex-ia ./cmd/cortex-ia

#### Scenario: Config parses with all tuning blocks

- **GIVEN** the asset installs
- **WHEN** the config is parsed
- **THEN** all four blocks are present and JSONC-valid

#### Scenario: Existing permissions are preserved

- **GIVEN** the permission array before and after the change
- **WHEN** the entries are compared
- **THEN** every pre-existing permission entry is present in the same order

#### Scenario: Installer mapping is unaffected

- **GIVEN** the KindConfig asset mapping
- **WHEN** the binary is rebuilt and assets tests run
- **THEN** the destination resolution and managed-asset checks stay green

## Design

- **Normative home**: `cortex-work-protocol.md` gains `§2.7a Adaptive Review Case Matrix (Normative)` and extends §4 with the budget table; both are the only full statements. Digest sites keep a version-stable one-paragraph/table digest plus pointer (KV-cache stability preserved: digests change only when the normative policy changes).
- **Permission graph**: deny-umbrella-first ordering (`cortex_*` deny → `cortex_cortex_*` allows; `subagent *` deny → five allows). Tool names verified against the live server: MCP tools register as `cortex_cortex_*` (`<server=cortex>_<tool=cortex_*>`); plugin tools are unprefixed `cortex_ia_*`. Under `codemode: true` tools are grouped under Code Mode, and normalized-name permission actions remain the documented gating surface.
- **Temperature**: reuse the existing `session.hook("context")` handler in the transport plugin (it already discriminates `event.agent`); a small `AGENT_TEMPERATURE` map applies `event.options.temperature`. Hook request overrides take precedence over model defaults per v2 docs.
- **Compaction**: possession replaces snapshot-push as the primary path; `compaction.keep.tokens` retains the recent tail mechanically, so possession + config tuning compose.
- **Sequencing**: the `codemode` flip (REQ-W2-005) precedes frontmatter collapse (REQ-W2-004) so the collapsed graph is reviewed against the final tool-exposure mode.

## Tasks

Dependency-safe DAG on board `harness-wave2`; workload policy `flexible`; all tasks ≤ ~500 LOC. Review policy: T3, T4 are plugin changes → RED-first TestREQ + mandatory independent reviewer; T6 touches the delegation authority surface → mandatory independent reviewer; all other tasks are markdown/config → orchestrator auto-approval per the Adaptive Review Policy. Parallel waves own disjoint `allowed_files`. Wave A = T1–T5 (independent), Wave B = T6–T7, Wave C = T8–T10, Wave D = T11.

### T1 w2-01 [assets] Publish normative Case Matrix and LOC budget in work-protocol

Requirements: REQ-W2-001, REQ-W2-002

Files: `internal/assets/skills/_shared/cortex-work-protocol.md`. Add §2.7a (full Cases 1–6 with mandatory `cortex_ia_work_approve` wording) and the §4 canonical LOC budget table; replace the protocol's derived matrix summary. Depends on: none.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T2 w2-02 [mcpmanager] Enable codemode for the cortex MCP preset

Requirements: REQ-W2-005

Files: `internal/mcpmanager/presets.go`, `internal/mcpmanager/desired.go`, `internal/mcpmanager/adopt_test.go`. Flip `codemode: false` → `true` for cortex and align desired-entry validation; context7 and `--tools=agent` untouched; update fixtures pinning the old value. Depends on: none.

Verification: go test -count=1 ./internal/mcpmanager/...

### T3 w2-03 [plugin] Temperature override via session.hook("context") (RED-first)

Requirements: REQ-W2-003

Files: `internal/assets/plugins/cortex-subagent-transport.ts`, `scripts/harness-temperature.test.mjs`. RED-first `TestREQ_W2T_001` (per-agent temperature map; no override for unknown agents) and `TestREQ_W2T_002` (dead `write_to_file`/`apply_patch` tool-prune deletes removed, `edit`/`write` retained). Modular test file ≤ 250 LOC. Depends on: none.

Verification: node --test scripts/harness-temperature.test.mjs scripts/harness-prompt.test.mjs

### T4 w2-04 [plugin] Deterministic compaction possession and retry hook (RED-first)

Requirements: REQ-W2-006

Files: `internal/assets/plugins/cortex-work.ts`, `scripts/harness-compaction.test.mjs`. RED-first `TestREQ_W2C_001` (`event.result` possession with DAG snapshot marker + fallback) and `TestREQ_W2R_001` (429 backoff, attempt ≥ 3 veto). Modular test file ≤ 250 LOC. Depends on: none.

Verification: node --test scripts/harness-compaction.test.mjs scripts/harness-work-reconcile.test.mjs

### T5 w2-05 [config] Harness config tuning in opencode.jsonc asset

Requirements: REQ-W2-008

Files: `internal/assets/opencode.jsonc`. Add compaction, tool_output, warming, and `agents.general.disabled` blocks; preserve every existing permission entry byte-for-byte. Depends on: none.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T6 w2-06 [agents] Authority surface: orchestrator, investigate, planner frontmatter

Requirements: REQ-W2-003, REQ-W2-004

Files: `internal/assets/agents/orchestrator.md`, `internal/assets/agents/investigate.md`, `internal/assets/agents/planner.md`. Temperature removal, dead v1 actions, `question: deny` (investigate/planner), orchestrator subagent allowlist (deny `*` + five allows), Cortex graph collapse, `steps` budgets (500/150/150); deny-first ordering mandatory. Depends on: T2.

Verification: node --test scripts/harness-prompt.test.mjs

### T7 w2-07 [agents] Minion surface: reviewer, implement, discovery frontmatter

Requirements: REQ-W2-003, REQ-W2-004

Files: `internal/assets/agents/reviewer.md`, `internal/assets/agents/implement.md`, `internal/assets/agents/discovery.md`. Same hygiene minus the subagent allowlist; `question: deny` on all three; `steps` budgets (150/250/100); LOC-budget pointers in reviewer/implement. Depends on: T2.

Verification: node --test scripts/harness-prompt.test.mjs

### T8 w2-08 [contracts] AGENTS.md single-source alignment (both surfaces)

Requirements: REQ-W2-001, REQ-W2-002

Files: `AGENTS.md`, `internal/assets/AGENTS.md`. §2.7 full matrix → marked digest + pointer; rule 4 and §3 budget restatements → digest/pointer; unify language sets; both surfaces content-equivalent. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T9 w2-09 [contracts] Derived-copy sweep (skills set 1)

Requirements: REQ-W2-001, REQ-W2-002

Files: `internal/assets/skills/parallel-dispatch/SKILL.md`, `internal/assets/skills/code-review-adversary/SKILL.md`, `internal/assets/skills/implement/SKILL.md`. Replace derived Case-Matrix and LOC-budget copies with protocol pointers. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T10 w2-10 [contracts] Derived-copy sweep (skills set 2 and shared contracts)

Requirements: REQ-W2-002

Files: `internal/assets/skills/planner/SKILL.md`, `internal/assets/skills/_shared/codebase-design-contract.md`, `internal/assets/skills/_shared/diagnosis-loop-contract.md`. Replace LOC-budget restatements with protocol §4 pointers; correct language-set variants. Depends on: T1.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

### T11 w2-11 [assets] AGENTS.md prune into autoinvoke-false skill

Requirements: REQ-W2-007

Files: `AGENTS.md`, `internal/assets/AGENTS.md`, `internal/assets/skills/opencode2-knowledge/SKILL.md`. Move §I and the four mermaid diagrams into the new skill (`autoinvoke: false`), one-line pointers in both AGENTS.md surfaces, retain the mandatory invariant sections, remove the retired `cortex setup claude-code` line, target ≤ ~24 KB; keep `internal/assets/inventory_test.go` green (update pinned counts within this lease if needed). Depends on: T8.

Verification: go build -o bin/cortex-ia ./cmd/cortex-ia

## Operator Actions (live-config surfaces, NOT repo tasks)

1. Run `cortex-ia install` (or `sync`) after this wave lands and restart the OpenCode daemon so new assets, the `codemode: true` preset, and plugin changes take effect; reconcile the MCP digest if prompted.
2. `use-railway` skill is duplicated in `~/.config/opencode/skills/use-railway` and `~/.agents/skills/use-railway` (precedence conflict; not repo-owned). Remove one copy — recommended: keep `~/.config/opencode/skills/use-railway` and delete `~/.agents/skills/use-railway`.
3. Optional provider-level temperature: the plugin hook now owns per-agent temperature; if the operator prefers config-level control instead, set `providers.<id>.options.temperature` in the live `opencode.jsonc` and the hook map can be cleared in a follow-up.
4. If the operator prefers different retention values than the asset ships, override `compaction.keep.tokens`, `compaction.buffer`, `tool_output.*`, or `warming` in the live `~/.config/opencode/opencode.jsonc` (project config wins over the global asset).

## Deferred to Wave 3 (explicit)

E1 gotcha-evidence transition gate · E3 runtime file leases via `ctx.permission.rules` · R1 hub-side ingest-time classification · R3 board/TTL hygiene sweep · R4 decision-pipeline metrics · `ctx.session.synthetic` guardrails (opportunity 10) · context7 codemode evaluation.
