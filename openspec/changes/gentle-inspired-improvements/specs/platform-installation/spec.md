# Delta for Platform and Installation

Authority: `openspec/changes/gentle-inspired-improvements/proposal.md`; cortex:209,211,213,215; `.cortex-ia/discovery.md`. Existing explicit-home behavior, verified backups, ownership, journaling and malformed-config rejection are preservation baselines, not missing functionality. No active install/sync is authorized.

## ADDED Requirements

### Requirement: REQ-PLATFORM-001: Gate Windows hardening on isolated reproduction
Trace: REC-12.
Windows handle/open hardening MUST address a demonstrated local failure at the narrow affected boundary. Before a fix, an isolated temporary-home oracle MUST distinguish safe behavior from the suspected race, recording operations, OS/toolchain and expected versus actual results. Unreproduced findings MUST remain hypotheses; broad rewrites, speculative retries and reboot cleanup are not authorized. A justified change MUST preserve non-Windows behavior and fail-closed installation guarantees.

#### Scenario: Happy reproduced boundary
- GIVEN a deterministic Windows fixture reproduces the suspected handle/open failure
- WHEN a narrowly scoped fix is verified with the same oracle
- THEN the original failing condition passes, errors remain visible, and the relevant non-Windows regression checks pass without weakened ownership or backup verification.

#### Scenario: Edge no reproduction
- GIVEN the isolated oracle does not reproduce the suspected defect
- WHEN implementation scope is evaluated
- THEN the finding remains unconfirmed, the result is recorded, and no speculative Windows hardening is accepted as a demonstrated fix.

#### Scenario: Failure concurrent interference
- GIVEN isolated concurrent access prevents a safe operation
- WHEN the affected filesystem boundary is exercised
- THEN the operation fails visibly without corrupting the existing target, claiming completion or introducing unapproved retries/cleanup.

### Requirement: REQ-INSTALL-001: Account for delegation configuration effects and writer outcomes
Trace: REC-15.
Installation results MUST account explicitly for delegation-configuration effects occurring after the pipeline. Unchanged, successfully changed and failed writes MUST be distinguishable without inventing existing API fields. Writers MUST report actual content change and propagate failures; malformed configuration and unmanaged drift MUST remain rejected without overwrite. Pipeline ownership, verified backups and journaling MUST be preserved. Design MUST choose between separately reported effects and expanded transactional integration; until then no universal rollback guarantee is assumed.

#### Scenario: Happy changed configuration
- GIVEN a valid owned temporary installation and changed delegation configuration
- WHEN the authorized isolated operation completes
- THEN resulting bytes match the requested configuration, change is reported accurately, and the result identifies which effects were covered by the pipeline transaction.

#### Scenario: Edge unchanged configuration
- GIVEN existing valid configuration already matches the requested content
- WHEN the writer is invoked in a temporary home
- THEN it reports no content change, does not falsely claim a mutation, and preserves ownership/journal semantics.

#### Scenario: Failure after pipeline or malformed input
- GIVEN pipeline success followed by an injected configuration write failure, or malformed/unmanaged configuration
- WHEN the operation is evaluated
- THEN overall success is not falsely reported, the failed boundary and surviving effects are visible, malformed/unmanaged bytes remain untouched, and any rollback claim is limited to verified coverage under the selected design.

## Scope preservation

These requirements grant no writes to `cortex-ia-quality-20260910` tasks or their protected paths: `internal/tuiassets/cortex-ia-tui.tsx`, `internal/assets/tui/cortex-ia-tui.js`, `internal/delegation/runner.go`, and associated sidebar/telemetry smokes. Later overlap requires explicit authoritative reconciliation and applicable review-refresh, not assumed cleanup authority. No production activation, publishing, deployment, secret access or active user configuration mutation is authorized.
