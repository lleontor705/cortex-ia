# Operational follow-up: lost-job reconciliation

This additive note records operator-reported outcomes after archive 60fcba74ff54ac0572ed2ac57c0a523ae77e669d2ca4dc3e59650d03c9390b88. It does not modify archived artifacts, contract pins or the closure manifest.

## Confirmed recovery

The orchestrator reports a consistent SQLite API backup at C:/Users/usrLuisLeon/.codex/backups/cortex-ia/reconcile-20260917T045414056Z, with integrity verification and a verified installed-binary backup. The candidate was installed with SHA-256 46B8D25F707F97C69569D5A67B327847C769DFAEEEA9D073B791C891A53A0261 and version display v0.4.38+dirty.

Public CLI reconciliation of job 692c01dda373fe1b2a519a99f1c4fb0a succeeded at 2026-09-17T04:54:54.946767Z using observed Windows boot time 2026-09-16T16:34:04.896294Z. Repeating the command returned the same recorded timestamp. This records prior-boot termination, not successful execution of the failed job.

The orchestrator compared row hashes across all eighteen pre-existing tables: delegation_events gained one event and schema_migrations advanced from 12 to 13; all other previous tables, including delegation_jobs and task/claim/lease tables, were unchanged. The new proof table contains one proof. All jobs in the affected workspace are terminal, and its sole lost job is reconciled. No old authority was restored. Final independent verification by bridge_guard confirmed schema 13, exactly one proof, one termination_reconciled event and zero remaining workspace blockers; lost status and LEASE_EXPIRED remain preserved.

## Installation and remaining investigation

OpenCode sync completed with transaction txn-20260917T045546-8dc9f6ee and verified backup install-20260917T045546-642623800. Context7 was restored through the supported command `cortex-ia mcp add context7 --preset`: configured, qualified and installed are true, with backup mcp-20260917T045725-657554200. Final independent verification by bridge_guard confirmed semantic equality with the original backup, enabled=true, a present MCP ownership record and CLI list status managed. The identified cause of the collateral removal is runSync using DefaultOptions with Context7=false, which treats the managed entry as selected for removal. selection.context7=false still persists: preserving the existing selection remains a pending bug fix, and no further generic sync is planned for this rollout.

The orchestrator reports that the installed bridge and orchestrator asset hashes exactly match the reviewed source, and the installed binary matches the reviewed candidate. The reconcile tool is exposed only to the orchestrator. This rollout exercised Windows; other operating systems reject reconciliation until a supported evidence provider exists.

The separate telemetry HTTP 401 remains pending. No remote deployment is claimed. Recovery evidence above was supplied by the operating orchestrator; the planner did not independently rerun the real mutation.
