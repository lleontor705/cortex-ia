# Canonical Workflow Map

**Installed contract:** `~/.cortex-ia/opencode/contracts/workflow-map.md`

The routing and phase matrix for Cortex-IA. Authority and receipts are defined in `cortex-work-protocol.md`. A skill is a procedure a role uses, not an agent per stage.

| Workflow | Controller route | Result and exit gate |
|---|---|---|
| direct-answer | Orchestrator answers from supplied evidence; investigate reads files if needed | Evidence-backed answer; any file mutation becomes a bounded task |
| discovery | Native discovery | Runs when `./.cortex-ia/discovery.md` is missing, stale, or explicitly requested. Appends the `/.cortex-ia/` .gitignore entry before profiling |
| investigate | Native investigate | Bounded diagnosis with cited evidence. No mutation |
| spike | Investigate with spike-prototype in an authorized scratch scope | Reproducible conclusion and cleanup, then choose the next route |
| direct-change | Orchestrator creates one task on `default`; implement; reviewer only if §2.7a requires it | Live claim and leases, proportional checks, approval under the review matrix. Covers data artifacts, docs and declarative config |
| ops-task | Orchestrator creates one task on `default`; implement | Script exit 0, target verification, approval under the review matrix |
| fast-tdd | Implement with fast-tdd; approval under the review matrix | Red test first, same oracle green, mutation evidence (`KILLED` or `STATIC-ANALYSIS`, see protocol §4), approval |
| hotfix | Implement with hotfix-triage; approval under the review matrix | Containment, regression evidence, and an explicit structural follow-up |
| plan | Planner writes the plan file and materializes the DAG; implement per slice; reviewer or auto-approval | Plan with runnable checks, same-board DAG, approvals, Outcome filled at close |
| review | Independent reviewer | Verdict against the plan's checks and the diff. Only an approved PASS produces `done` |
| retrospective | Investigate with workflow-retrospective | Diagnosis and recommendations. No implementation or state mutation |

Use the smallest route that fits. Direct tasks need no plan. The planner skill (`skills/planner/SKILL.md`, section 1) states when a plan is needed.

## Plan phases

| Phase | Role | Artifact | Exit |
|---|---|---|---|
| plan / plan | planner | `docs/plans/<board-id>.md` and the same-board DAG | Checks are runnable, slices are valid, DAG is materialized |
| plan / decompose | planner | Revised slices for one blocked task | 2-8 child tasks on the same board; parent superseded |
| plan / apply | implement | Progress, Decisions and Surprises appended before `in_review` | Slice checks pass; change stays in scope |
| plan / review | reviewer | Plan's acceptance checks, rerun on the current tree | Independent verdict per check |
| plan / close | last implementer | Outcome section filled | All tasks approved; the file stays as history |

The plan is the only spec artifact. Implementation and review read it, and repository tests are the living spec. Durable decisions go to `AGENTS.md` or an ADR, not into the plan.

## Task sizing: the resume test

> **Resume test:** could a fresh agent resume this task from the request text plus `git diff` alone, without asking a question?

A task that passes is correctly sized. A task whose diff cannot be read without the conversation must be re-scoped or split. If the resume test and a budget conclusion disagree, the stricter outcome governs and the conflict is recorded in the plan's Decisions.

## Delivery forecast (advisory)

A plan's Slices section may estimate authored LOC per slice. The estimate can flag a slice that exceeds its budget. It never changes the workload policy or blocks a task. Enforcement stays with the pre-transition diff check in `cortex-work-protocol.md` §4.

## Binding, review and closure

Each `cortex_ia_work_create` call declares `workflow` (`direct-change`, `fast-tdd`, `hotfix` or `ops-task`). A task created from a plan uses the workflow that fits its slice.

Fingerprints of the task definition and the sorted writable-file contents bind a review and its approval. A changed fingerprint rejects stale acceptance. Shell access is not an OS sandbox. Native mutation-tool admission and per-file reservations are the safeguards, with their limits.

When later authorized tasks change files covered by an earlier approval, the orchestrator calls `cortex_ia_work_review_refresh({task_id, revision})` after active work is reconciled. That reopens review under current fingerprints, grants no write authority and never approves. An independent reviewer evaluates the final state and records a fresh verdict. Historical approvals stay unchanged.

Plan closure has no archive step. When the last approval is recorded, the last implementer fills in the Outcome section.

## Delegation and recovery

The orchestrator dispatches native role controllers, and no external leaf takes part in work authority. A failed dispatch needs reconciliation and an explicit new dispatch. Reviewer failure returns work to `blocked`. Expired authority needs recovery. Excess scope routes to planner decomposition. Repeated causes route to retrospective. The TUI and the web console are views, not task authority.
