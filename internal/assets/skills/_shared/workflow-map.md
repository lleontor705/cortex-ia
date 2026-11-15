# Canonical workflow map

This is the single routing and phase matrix for Cortex-IA. Installed path: `~/.cortex-ia/opencode/contracts/workflow-map.md`. Runtime authority and receipts are defined in `cortex-work-protocol.md`. A skill is a procedure used by a role, not a separate agent per stage.

| Workflow | Controller route | Required result and exit gate |
|---|---|---|
| direct-answer / direct-doc | Orchestrator answers from supplied evidence; investigate reads files | Evidence-backed response; filesystem mutations route to a bounded task |
| discovery | Native discovery | On-demand: dispatch only when `./.cortex-ia/discovery.md` is absent, explicitly requested, or stale before a planned initiative. Performs the `/.cortex-ia/` `.gitignore` hygiene append before profiling |
| direct-change | Orchestrator creates one task in default; implement; reviewer only if risk warrants | Live claim and leases, proportional checks, approval per the review matrix, including data/artifact generation, docs, and declarative config |
| ops-task | Orchestrator creates one task in default; implement; reviewer only if risk warrants | Script execution exit 0, target verification, approval per the review matrix |
| fast-tdd | Implement with fast-tdd, then approval per the review matrix | Causal RED, same oracle GREEN, mutation evidence (`KILLED` or `STATIC-ANALYSIS` per `cortex-work-protocol.md` §4/§8), approval per the review matrix |
| hotfix | Implement with hotfix-triage, then approval per the review matrix | Containment, regression evidence and an explicit structural follow-up |
| spike | Investigate with authorized spike-prototype scratch scope | Reproducible conclusion and cleanup, then choose the next route |
| plan | Planner writes the plan file and materializes the DAG; implement per slice; reviewer or auto-approval | Plan file with executable checks, same-board DAG, approvals per the review matrix, Outcome filled at close |
| review | Independent reviewer | Evidence against the plan's checks and the diff; only work approval PASS produces done |
| retrospective | Investigate with workflow-retrospective | Diagnosis and recommendations, no implementation or state mutation |

Use the smallest route that fits. Direct tasks need no plan; a plan is needed under the rules in the planner skill (`skills/planner/SKILL.md` section 1).

## Plan phases

| Phase | Role | Artifact | Exit |
|---|---|---|---|
| plan / plan | planner | `docs/plans/<board-id>.md` and the same-board DAG | Checks are runnable, slices are valid, DAG is materialized |
| plan / decompose | planner | Revised slices for one blocked task | 2-8 child tasks on the same board; parent superseded |
| plan / apply | implement | Progress and Decisions and Surprises appended before `in_review` | Checks for the slice pass; scoped change |
| plan / review | reviewer | Plan's Acceptance Checks, rerun against the current tree | Independent verdict per check |
| plan / close | last implementer | Outcome section filled | All tasks approved; the file stays as history |

The plan is the only spec artifact. Planning produces it; implementation and review read it. Repository tests are the living spec. Durable decisions go to `AGENTS.md` or an ADR, not into the plan.

## Task sizing: the resume test

Task sizing MUST apply the resume test:

> **resume test**: Could a fresh agent resume this task from the request text plus `git diff` alone, without asking a question?

- A task that passes the test is correctly sized.
- A task whose diff cannot be interpreted without conversation context fails and must be re-scoped or split.
- When the resume test conflicts with a tier or LOC budget conclusion, the stricter outcome governs and the conflict is recorded in the plan's Decisions.

## Delivery forecast (advisory)

A plan's Slices section may carry an estimated authored LOC per slice. The estimate is advisory. It may flag that a slice exceeds its budget, but it never relaxes or tightens the workload policy and never blocks a task. Enforcement stays with the pre-transition diff check in `cortex-work-protocol.md` §4.

## Binding, review and closure

Each `cortex_ia_work_create` call declares `workflow` (`direct-change`, `fast-tdd`, `hotfix` or `ops-task`). A task created from a plan uses the workflow that fits its slice.

Runtime-generated fingerprints bind the task definition and the sorted writable-file contents and deletion markers to review and to historical approval. A changed fingerprint rejects stale acceptance. Shell access is not an OS sandbox; native mutation-tool admission and per-file lease reservations are the specific safeguards, with their explicit limits.

If later authorized tasks modify files covered by an earlier approval, the orchestrator requests `cortex_ia_work_review_refresh({task_id, revision})` after active work is reconciled. It reopens review with current fingerprints, grants no write authority and never approves. An independent reviewer must evaluate the final state and record a fresh verdict. Historical approvals remain unchanged.

Plan closure has no archive step. When the last approval is recorded, the last implementer fills the plan's Outcome section, and the file stays in `docs/plans/` as history.

## Delegation and recovery

The orchestrator dispatches native role controllers; every role executes natively and no external leaf participates in work authority. Accepted failures require reconciliation and an explicit new dispatch. Reviewer failure returns work to blocked; expiry requires recovery; excessive scope routes to planner decomposition; repeated causes route to retrospective. Herdr, the TUI and the web are views, not task authority.
