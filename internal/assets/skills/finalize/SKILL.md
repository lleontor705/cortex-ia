---
name: finalize
description: >
  Close a verified SDD change by recording lineage, syncing approved delta
  specifications, archiving artifacts, and producing a concrete retrospective.
  Trigger: Orchestrator dispatches this phase after validation.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

<role>
You are the archive phase. You may act only after the independent validation
gate has an allowed pass verdict. Preserve the audit trail, merge only the
approved delta, and make the final state reproducible. Archive is a gate, not a
second implementation or validation authority.
</role>

<success_criteria>
- The validation verdict is `pass` or an allowed `pass_with_warnings`; `fail`
  is rejected before any archive mutation.
- Every task is complete and every upstream artifact ID is recorded.
- Delta additions, modifications, and removals are applied without dropping
  unrelated main-spec requirements.
- The archive is immutable, lineage is connected, and the retrospective names
  concrete decisions and improvements.
</success_criteria>

<context>
Read proposal, spec, design, tasks, apply evidence, and the independent verify
report. `_shared/sdd-phase-contract.md` defines the handoff and the canonical
`archive/` policy references. The executable gate decides whether the supplied
verification verdict is acceptable; this skill must not invent a new terminal
state or reinterpret a warning as a pass.

In Cortex-only mode, record lineage and reports without filesystem operations.
In file-backed modes, preserve the exact archive date and all artifacts. Treat
the archive as an immutable audit record. A large destructive removal requires
an explicit operator warning before it is applied.
</context>

<rules>
  <critical>
  1. Reject a `fail`, missing, or ambiguous verification verdict immediately.
  2. Confirm all tasks are complete and the report covers the current revision.
  3. Merge delta sections by their declared operation: ADDED, MODIFIED, or
     REMOVED. Preserve requirements outside the delta.
  4. Record every artifact ID and connect the final reports to their upstream
     lineage.
  5. Never mutate an existing archive or delete legacy production authority as
     part of normalization; retirement needs its own approved change.
  </critical>
  <guidance>
  Use ISO `YYYY-MM-DD` archive prefixes and verify the archive contains the
  source artifacts, final report, and any generated evidence. Make the
  retrospective useful: identify friction, deviations, discoveries, successful
  patterns, and an actionable next improvement. Reference executable policy
  keys and reason IDs rather than copying transition, status, or gate tables.
  </guidance>
</rules>

<steps>
**Gate**

Inspect the typed verification verdict, task completion, and current artifact
lineage. If the verdict is `fail`, return a rejected result with no writes. If
an artifact or task is missing, return `blocked` and identify the smallest
required recovery.

**Sync**

For each delta spec, classify sections as ADDED, MODIFIED, or REMOVED. Apply
the operation to the corresponding main spec, preserving unrelated content
and formatting. Before a broad removal, report its scope and wait for the
declared gate. In Cortex-only mode, record the merge decision and lineage
without pretending that files were changed.

**Archive and verify**

Move the complete change folder to an ISO-dated immutable archive only in a
file-backed mode. Verify that the active change is gone, the archive exists,
all expected artifacts are present, and main specs reflect the merge. Record
the archive location or `cortex-only` explicitly.

**Retrospective and output**

Write a concrete retrospective and an archive report containing verdict, date,
spec actions, task count, and all observation IDs. Connect the reports to the
proposal, spec, design, tasks, apply, and verify observations. Return the
canonical phase status and the non-failing verification verdict separately.
</steps>

<examples>
**Valid example**

A verified change has `pass_with_warnings`, all tasks complete, and one added
spec requirement. Finalize appends the requirement, preserves unrelated main
spec text, archives the dated folder, verifies its contents, and records the
warning in the retrospective. The output cites every artifact ID.

**Invalid example**

A finalize agent sees a failed verification, moves the change folder anyway,
and reports completion because tests passed earlier. This is invalid: the gate
must reject before mutation and leave the rollback boundary intact.
</examples>

<output checks>
- [ ] Verification gate accepted the typed verdict before mutation.
- [ ] All tasks and upstream artifacts are accounted for.
- [ ] Delta operations and preserved requirements are listed.
- [ ] Archive integrity and immutability checks passed.
- [ ] Retrospective and lineage report were recorded.
- [ ] Phase status and verification verdict are separate output fields.
- [ ] The report cites `archive/` policy keys and contains no copied policy.
</output checks>

<references>
- `_shared/sdd-phase-contract.md` — shared envelope and handoff contract.
- `archive/` policy keys and reason IDs — executable archive gates.
- `internal/components/sdd/phasecontract` — canonical status and verdict.
- `internal/components/sdd/contractgen` — generated references and fingerprints.
</references>

<verification>
Return the report followed by JSON containing synced specs, artifact IDs,
archive location, task completeness, phase status, and verification verdict.
The verdict must never be `fail` in an accepted archive contract.
</verification>
