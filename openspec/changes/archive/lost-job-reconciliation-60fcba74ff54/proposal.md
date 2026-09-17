# Lost-job reconciliation

## Problem and evidence
An expired external worker remains lost and fences its workspace after cancellation. The attached incident identifies job 692c01dda373fe1b2a519a99f1c4fb0a in ats-inventory. Its recorded start is 2026-09-16T05:49:22Z; the investigator reports a later Windows boot at 16:34:04Z. This is candidate evidence, not permission to modify production state during implementation. PID absence alone does not prove descendant termination.

## Outcome
Provide one explicit, audited reconciliation operation for lost jobs whose recorded execution predates a verified operating-system boot. Preserve historical failure and receipts, release only that job attempt's fence, and require fresh authority for later execution.

## Scope
One cohesive implementation and independent review: service proof ledger and migration, native boot observation, CLI, orchestrator-only bridge tool and permission, actionable admission diagnostics, bounded authority/transport regressions. Use flexible workload policy, not artificial file-count decomposition.

## Non-goals
No same-boot PID heuristics, process kills, automatic pane cleanup, automatic retry, restored claims, broad administrative override, or OS sandbox. The separate HTTP 401/schema-2 hub compatibility incident is not included. No production configuration or state mutations in implementation/tests. Prior archived SDD is unchanged.
