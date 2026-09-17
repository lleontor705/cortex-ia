# Lost-job reconciliation closure

Implementation ljr-1.1 is DONE at revision 4. Independent reviewer execution-isolation-reviewer approved review revision 3 on 2026-09-17. The current product fingerprint matches approved SHA-256 b609001e17227072630e3e018aa7a4f20807d9554b7625ee8383d3f75f302431. Original planning pins are unchanged.

## Delivered behavior
The service admits explicit reconciliation only for lost jobs with internally collected Windows boot evidence after the recorded execution start. Active jobs, same-boot missing or reused PIDs, unavailable evidence and ambiguous ordering remain fenced. Schema 13 records an immutable proof for the exact job attempt and identity in the same transaction as its audit event. Historical lost status, error and receipt remain unchanged; neither old authority nor success is fabricated. CLI and orchestrator-only bridge expose the operation, and admission errors identify the blocker and next action. Reconciliation does not kill processes or close Herdr panes.

## Verification evidence
The independent reviewer and orchestrator report Go 1.26.1 full tests and vet PASS, golangci-lint 2.11.4 zero issues, 55/55 harness checks, clean formatting/diff checks and successful CLI build. The implementer additionally exercised the real Windows CIM provider through a disposable CLI smoke with an isolated state root: recover, reconcile, idempotent repeat, preserved failure and fresh synthetic admission. Tests did not target the real incident job. The dedicated authority test is 250 lines. Available direct caller/import analysis substituted for unavailable Cortex AST tools during this review; no broader AST certification is claimed.

The approved candidate D:/cortex-ia/bin/cortex-ia.exe has orchestrator-reported SHA-256 46B8D25F707F97C69569D5A67B327847C769DFAEEEA9D073B791C891A53A0261. This is a build artifact, not evidence of installed activation.

## Operational handoff and limits
Real job 692c01dda373fe1b2a519a99f1c4fb0a remains an operational follow-up at this closure. No claim of its recovery is made here. The operator must first obtain a consistent SQLite API backup and verified binary backup, activate the compatible candidate at the executable actually selected by OpenCode, verify its hash, inspect sync --dry-run --target opencode, then invoke delegate reconcile for the exact job with a reason. Node 24.20 exposes node:sqlite backup(source,destination) for a WAL-consistent snapshot. The bridge tries PATH before fallback installation paths; do not assume a local build replaces its executable. A sync dry-run does not install the new bridge tool.

Keep the compatible binary after schema migration. Do not automatically restore an old database while work is live or roll back only the binary to an incompatible schema reader. Backups support controlled recovery; prefer a forward fix. Do not start replacement work merely to test admission. Non-Windows reconciliation is explicitly unsupported and fails closed; current-boot descendant termination remains unproven. The independent telemetry HTTP 401 investigation and any production service changes are outside this change.

Planner archive uses the currently installed schema-12 CLI so bookkeeping does not prematurely migrate the live database. The durable archive receipt is authoritative for closure; this record is not modified after relocation.
