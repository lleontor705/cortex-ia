# Canonical workflow map

This is the single routing and phase matrix for Cortex-IA. Installed path: `~/.cortex-ia/opencode/contracts/workflow-map.md`. Runtime authority and receipts are defined in `cortex-work-protocol.md`; selected-plane pins are defined in `cortex-convention.md`. A skill is a procedure used by a role, not a separate agent per SDD phase.

| Workflow | Controller route | Required result and exit gate |
|---|---|---|
| direct-answer / direct-doc | Orchestrator answers from supplied evidence; investigate reads files | Evidence-backed response; filesystem mutations route to a bounded task |
| discovery | Native discovery | Scoped discovery profile, observed capabilities and limitations |
| direct-change | Orchestrator creates one task; implement then reviewer | Live claim/leases, proportional checks, independent approval |
| fast-tdd | Implement with fast-tdd, then reviewer | Causal RED, same oracle GREEN, independent approval |
| hotfix | Implement with hotfix-triage, then reviewer | Containment, regression evidence and explicit structural follow-up |
| spike | Investigate with authorized spike-prototype scratch scope | Reproducible conclusion and cleanup, then choose the next route |
| decision-map | Investigate and user decisions, then planner | Decision map; no implementation board or DAG |
| sdd-lite | Investigate; planner integrated; implement; reviewer; planner archive | Integrated contract, typed SDD task bindings, independent approval and durable closure |
| sdd-full | Investigate; planner propose/spec/design/tasks; implement; reviewer; planner archive | Phase contracts, typed SDD task bindings, independent approval and durable closure |
| review | Independent reviewer | Spec and implementation evidence; only work approval PASS produces done |
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
| Lite/Full / apply | implement | Validated artifacts | Retrieved validated pins | Current authority, scoped change and verification evidence |
| Lite/Full / verify or review | reviewer | Current artifacts and change | Current pins and change | Independent semantic review and current fingerprints |
| Lite/Full / archive | planner | Archived change directory and durable receipt | Logical durable closure; no OpenSpec move | Applicable tasks approved and fingerprints current |

Only the planner materializes the SDD DAG, after integrated/tasks validation. Direct tasks may omit SDD bindings; do not omit a binding to bypass an SDD gate. Decision-map creates no board. One stable board groups a materialized initiative.

## Structural validation contract

For OpenSpec/hybrid use `cortex_ia_openspec_validate` with explicit `relative_directory`, `workflow`, and `phase`. Lite uses one `plan.md`; Full validates only artifacts due at that phase. Decision-map uses `decision-map.md`. The JSON result is structural evidence, not a semantic PASS or product acceptance.

Use `REQ-{DOMAIN}-{NNN}` requirement headings. ADDED/MODIFIED requirements have at least three Scenario blocks with nonempty GIVEN, WHEN and THEN. Reviewers independently judge happy, edge and failure coverage; counting blocks cannot prove this. REMOVED requirements state their reason/migration instead of new implementation scenarios. Task blocks have unique task IDs and an explicit `Requirements:` list of existing requirement IDs. Integrated/final task validation rejects unknown references and uncovered implementable requirements.

## Binding, review and closure

Each new `cortex_ia_work_create` call declares `workflow` (`direct-change`, `fast-tdd`, `hotfix`, `sdd-lite` or `sdd-full`). SDD workflows require a matching `sdd_contract`: `version: 1`, `workflow`, `change_id`, `spec_plane`, `pins` and `requirement_ids`. Each pin contains `transport`, `project`, `locator`, and lowercase SHA-256. Native workspace-file pins are checked against bytes. Local Cortex CLI pins are re-read through a bounded export and compared during runtime checks. Remote MCP pins require independent retrieval and comparison by the controller through the selected transport; runtime does not contact arbitrary remote providers or certify semantic truth. Legacy CLI creation without a declared workflow remains direct-compatible; it does not certify an SDD execution.

Runtime-generated fingerprints bind the task definition and sorted writable-file contents/deletion markers to review and historical approval. A changed fingerprint rejects stale acceptance. Direct and historical tasks remain compatible without fabricated SDD bindings. Shell access is not an OS sandbox; native mutation-tool admission and external baseline checks are specific safeguards with explicit limits.

If later authorized tasks modify files covered by an earlier approval, orchestrator explicitly requests `cortex_ia_work_review_refresh({task_id, revision})` after active work is reconciled. It reopens SDD review with current fingerprints and the original implementation identity, grants no write authority and never approves. An independent reviewer must evaluate the final state and record a fresh verdict before closure. Historical approvals remain unchanged.

After required approvals, planner calls `cortex_ia_change_archive({board_id, change_id, workflow, spec_plane})`. Closure is fail-closed on pending/unbound work, changed fingerprints or conflicting archive state. Existing historical approvals are preserved, never rewritten to make closure pass. Cortex-only closure records a logical receipt. A closed change is not reopened by rerunning archive.

## Delegation and recovery

Orchestrator dispatches native role controllers. The role's delegation gate returns native, direct_cli or herdr_multiplexed; only explicit error-free native authorizes native execution. AGY remains one bounded leaf, never a coordinator. Accepted failures require reconciliation and an explicit new dispatch. Reviewer failure returns work to blocked; expiry requires recovery; excessive scope routes to planner decomposition; repeated causes route to retrospective. Herdr, the TUI and the web are views, not task authority.
