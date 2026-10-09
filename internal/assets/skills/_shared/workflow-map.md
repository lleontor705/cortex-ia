# Canonical workflow map

This is the single routing and phase matrix for Cortex-IA. Installed path: `~/.cortex-ia/opencode/contracts/workflow-map.md`. Runtime authority and receipts are defined in `cortex-work-protocol.md`; selected-plane pins are defined in `cortex-convention.md`. A skill is a procedure used by a role, not a separate agent per SDD phase.

| Workflow | Controller route | Required result and exit gate |
|---|---|---|
| direct-answer / direct-doc | Orchestrator answers from supplied evidence; investigate reads files | Evidence-backed response; filesystem mutations route to a bounded task |
| discovery | Native discovery | On-demand: dispatch only when `./.cortex-ia/discovery.md` is absent (initial onboarding), explicitly requested, or stale before a Tier 3 SDD initiative. Performs the `/.cortex-ia/` `.gitignore` hygiene append before profiling. Quick agentic environment index: skills dictionary (local + global installed), run/test info, minimal governance, quick index |
| direct-change | Orchestrator creates one task in default; implement; reviewer only if risk warrants | Live claim/leases, proportional checks, independent reviewer or orchestrator auto-approval on low-risk changes, data/artifact generation (Excel, CSV, reports), docs, and declarative config |
| ops-task | Orchestrator creates one task in default; implement; reviewer only if risk warrants | Script execution exit 0, target DB/infrastructure verification, independent reviewer or orchestrator auto-approval on low-risk/idempotent scripts |
| fast-tdd | Implement with fast-tdd, then reviewer | Causal RED, same oracle GREEN, mutation evidence (`KILLED` or `STATIC-ANALYSIS` per `cortex-work-protocol.md` §4/§8), independent approval |
| hotfix | Implement with hotfix-triage, then reviewer | Containment, regression evidence and explicit structural follow-up |
| spike | Investigate with authorized spike-prototype scratch scope | Reproducible conclusion and cleanup, then choose the next route |
| decision-map | Investigate and user decisions, then planner | Decision map; no implementation board or DAG |
| sdd-lite | Investigate; planner integrated; implement; reviewer; planner archive | Integrated contract, typed SDD task bindings, independent approval and durable closure |
| sdd-full | Investigate; planner propose/spec/design/tasks; implement; reviewer; planner archive | Phase contracts, typed SDD task bindings, independent approval and durable closure |
| review | Independent reviewer | Spec and implementation evidence including mutation evidence (`cortex-work-protocol.md` §8); only work approval PASS produces done |
| retrospective | Investigate with workflow-retrospective | Diagnosis and recommendations, no implementation or state mutation |

## Planning phases and storage planes

| Workflow / phase | Role | OpenSpec or hybrid artifact | Cortex-only artifact | Exit |
|---|---|---|---|---|
| decision-map / chart or resolve | planner | decision-map.md | Pinned decision snapshot | Decisions and unresolved questions; no tasks |
| sdd-lite / integrated | planner | plan.md | Pinned integrated contract | Intent, requirements, design, task traceability and checks |
| sdd-full / propose | planner | proposal.md | Pinned proposal | Scope and non-goals; no future files required |
| sdd-full / spec | planner | proposal.md and specs/**/spec.md | Pinned requirements | Unique requirement IDs and complete scenarios |
| sdd-full / design | planner | Previous phase plus design.md | Pinned design | Interfaces, risks and verification approach |
| sdd-full / tasks | planner | Previous phase plus tasks.md | Pinned tasks and contract references | Task references resolve to requirements before materialization |
| Lite/Full / decompose | planner | Modified plan.md or tasks.md | Pinned decomposed tasks | 2-8 coherent child tasks sized by workload_policy; parent superseded |
| Lite/Full / apply | implement | Validated artifacts | Retrieved validated pins | Current authority, scoped change and verification evidence |
| Lite/Full / verify or review | reviewer | Current artifacts and change | Current pins and change | Independent semantic review and current fingerprints |
| Lite/Full / archive | planner | Archived change directory and durable receipt | Logical durable closure; no OpenSpec move | Applicable tasks approved and fingerprints current |

Only the planner materializes the SDD DAG, after integrated/tasks validation. Direct tasks may omit SDD bindings; do not omit a binding to bypass an SDD gate. Decision-map creates no board. One stable board groups a materialized initiative.

## Spec-plane repositioning

The routing table above defines *which* workflow executes; the table below repositions *what spec form* a workload warrants, so tiers and storage planes do not drift. It is guidance layered on the routing and phase-matrix rows, which remain authoritative for the SDD mechanics themselves.

| Workload | Spec form |
|---|---|
| Tier 1 / Tier 2 bounded task | Short paragraph plus executable tests; no SDD change directory |
| Tier 3 / multi-session / regulated domain | `sdd-lite` (integrated contract) or `sdd-full` (phased gates) |
| Living spec evolution | Delta spec: propose→apply→archive; never in-place base-spec rewrites |
| "Spec fulfilled" judgment | Executable verification first (mutation gate where applicable); narrative review second |

A Tier 1 or Tier 2 bounded task carries its specification as a short paragraph plus executable tests rather than a change directory. An SDD change directory is reserved for Tier 3 initiatives, multi-session work, and regulated domains. When a living spec evolves after archive, the change is recorded as a delta spec (propose→apply→archive) instead of rewriting the base spec in place. Judging a spec fulfilled centers on executable verification — including the mutation gate where applicable — ahead of narrative review. The phase-matrix, binding and closure rules above still govern how SDD artifacts are produced, validated and closed.

## Task sizing: the resume test

Alongside the tier rules and LOC budgets, task sizing MUST apply the resume test as a criterion:

> **resume test**: Could a fresh agent resume this task from the request text plus `git diff` alone, without asking a question?

- A task whose request text plus `git diff` lets a fresh agent resume unaided **passes** the oracle.
- A task whose diff cannot be interpreted without conversation context **fails** the oracle and MUST be re-scoped or split.
- When the resume test conflicts with a tier or LOC budget conclusion, the **stricter outcome governs** and the conflict is recorded in the plan.

## Feature-doc Log and mirror reconciliation

sdd-lite plan docs carry a fixed, append-only `## Log` section. Its shape and write rules are:

- **L1 — request**: the original user request stored verbatim, guarded so secrets are never persisted (secret-shaped content is replaced with a typed placeholder per the guard in `cortex-convention.md`). L1 is written once and is never rewritten.
- **Dated entries**: every subsequent decision and phase transition appends one dated entry, in chronological order. Entries are only appended; existing entries are never edited, reordered, or deleted.

When a plan doc is mirrored to a second store (for example a repo `plan.md` plus a pinned Cortex snapshot), the mirror is a dual write: both copies are read back and compared before success is claimed. If the copies disagree after read-back, the mismatch is reported and the authoritative copy is explicitly named — never silently chosen. A mirror write that is not read back is an incomplete write.

## Delivery forecast (advisory)

Planning records a delivery forecast authored from the materialized task DAG: estimated authored LOC per slice plus a total, together with the slice boundaries. The forecast is a labeled advisory artifact with no gate authority:

- It may flag that a slice exceeds a budget, but it records the breach as advisory only.
- It never relaxes `strict`, `flexible`, or `unbounded`, never changes the workload-policy tier, and never blocks a task.
- It carries no gate authority over the §4 workload table in `cortex-work-protocol.md`; enforcement authority stays exclusively with the pre-transition preflight.

## Structural validation contract

For OpenSpec/hybrid use `cortex_ia_openspec_validate` with explicit `relative_directory`, `workflow`, and `phase`. Lite uses one `plan.md`; Full validates only artifacts due at that phase. Decision-map uses `decision-map.md`. The JSON result is structural evidence, not a semantic PASS or product acceptance.

Use `REQ-{DOMAIN}-{NNN}` requirement headings. ADDED/MODIFIED requirements have at least three Scenario blocks with nonempty GIVEN, WHEN and THEN. Reviewers independently judge happy, edge and failure coverage; counting blocks cannot prove this. REMOVED requirements state their reason/migration instead of new implementation scenarios. Task blocks have unique task IDs and an explicit `Requirements:` list of existing requirement IDs. Integrated/final task validation rejects unknown references and uncovered implementable requirements. REQ-bound tasks delivering a persistent test name it `TestREQ_{DOMAIN}_{NNN}_<slug>` (language-conditional adaptation allowed) and carry a verification command targeting that name; the naming convention is recorded in `cortex-convention.md` and remains convention-level — no runtime validator enforces it this wave.

## Binding, review and closure

Each new `cortex_ia_work_create` call declares `workflow` (`direct-change`, `fast-tdd`, `hotfix`, `ops-task`, `sdd-lite` or `sdd-full`). SDD workflows require a matching `sdd_contract`: `version: 1`, `workflow`, `change_id`, `spec_plane` (`openspec`, `hybrid` or `cortex`), `pins` and `requirement_ids`. Each pin contains `transport`, `project`, `locator`, and lowercase SHA-256. Native workspace-file pins are checked against bytes. Local Cortex CLI pins are re-read through a bounded export and compared during runtime checks. Remote MCP pins require independent retrieval and comparison by the controller through the selected transport; runtime does not contact arbitrary remote providers or certify semantic truth. Legacy CLI creation without a declared workflow remains direct-compatible; it does not certify an SDD execution.

Runtime-generated fingerprints bind the task definition and sorted writable-file contents/deletion markers to review and historical approval. A changed fingerprint rejects stale acceptance. Direct and historical tasks remain compatible without fabricated SDD bindings. Shell access is not an OS sandbox; native mutation-tool admission and per-file lease reservations are specific safeguards with explicit limits.

If later authorized tasks modify files covered by an earlier approval, orchestrator explicitly requests `cortex_ia_work_review_refresh({task_id, revision})` after active work is reconciled. It reopens SDD review with current fingerprints and the original implementation identity, grants no write authority and never approves. An independent reviewer must evaluate the final state and record a fresh verdict before closure. Historical approvals remain unchanged.

After required approvals, planner calls `cortex_ia_change_archive({board_id, change_id, workflow, spec_plane})`. Closure is fail-closed on pending/unbound work, changed fingerprints or conflicting archive state. Existing historical approvals are preserved, never rewritten to make closure pass. Cortex-only closure records a logical receipt. A closed change is not reopened by rerunning archive.

## Delegation and recovery

Orchestrator dispatches native role controllers; every role executes natively and no external leaf participates in work authority. Accepted failures require reconciliation and an explicit new dispatch. Reviewer failure returns work to blocked; expiry requires recovery; excessive scope routes to planner decomposition; repeated causes route to retrospective. Herdr, the TUI and the web are views, not task authority.
