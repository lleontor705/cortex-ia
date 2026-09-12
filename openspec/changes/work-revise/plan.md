# Work Revise Safe Contract Update Plan

## Intent

This change defines an official, fail-closed `work revise` operation for Cortex-IA task authority. It exists to safely update stale task definitions and SDD bindings for unclaimed work without bypassing review. The authorized engine feature work is a separate prerequisite initiative implemented in workspace `D:/cortex-ia` by two new implementation tasks on the stable dedicated board `work-revise`; those tasks use `change_id: work-revise` and pin this plan. The separate Pi recovery case stays on the original board `pi-native-adaptive-ux`: its five existing Pi tasks remain workspace `D:/lleontor705/pi-cortex-ia`, keep `change_id: pi-native-adaptive-ux`, keep their existing replacement workflow and `spec_plane`, and pin the corrected Pi contract at `openspec/changes/pi-native-adaptive-ux/plan.md`.

Non-goals: this change does not claim tasks, lease files, approve work, decompose blocked tasks, reopen archived changes, or mutate active/in-review/done/superseded work. It does not silently remove an SDD contract to make a task editable. It does not reuse or alter the original `pi-native-adaptive-ux` board for the engine feature, move Pi tasks to the engine workspace, point Pi recovery at this engine plan, change Pi workflow/change identity/spec plane, or redesign the original Pi task scopes except for the explicitly reviewed correction to stale pins and any already-authorized narrow scope correction.

## Requirements

### Requirement: REQ-WORK-001 bounded task revision identity and lifecycle guard

Cortex-IA MUST provide a bounded task-definition revision path that only accepts unclaimed `ready` or `backlog` tasks at an explicitly observed revision. The operation MUST preserve task ID, board ID, workspace, dependency edges, replacement history, and append an audit event while incrementing the task revision only after all validation succeeds.

#### Scenario: ready task can be revised with CAS
- GIVEN a task is `ready`, unclaimed, has no active review, and its revision equals the supplied expected revision
- WHEN a supported revise payload updates objective, acceptance criteria, verification, allowed files, and a complete SDD contract
- THEN the task definition is atomically replaced, the task revision increments once, and an audit event records the revision action

#### Scenario: backlog task keeps dependencies
- GIVEN a task is `backlog` because dependencies are not done
- WHEN a valid revise payload is applied at the expected revision
- THEN the task remains `backlog`, its dependency set is unchanged, and downstream readiness is not recalculated by the definition update

#### Scenario: stale revision is rejected
- GIVEN a task revision differs from the supplied expected revision
- WHEN the revise operation is requested
- THEN Cortex-IA rejects the request with a work conflict and leaves every task row, definition row, dependency, claim, lease, review, and audit state unchanged

### Requirement: REQ-WORK-002 fail-closed status, claim, review, and unknown-field validation

The revise operation MUST reject active, claimed, review, terminal, superseded, malformed, or ambiguous inputs before writing. Supported statuses are only unclaimed `ready` and unclaimed `backlog`; `in_progress`, `in_review`, `done`, `blocked`, and `superseded` are not revisable through this path.

#### Scenario: active or claimed task is rejected
- GIVEN a task is `in_progress` or has a live claim or active file lease
- WHEN a revise payload is submitted
- THEN Cortex-IA rejects the request and does not change the task definition or revision

#### Scenario: review and terminal states are rejected
- GIVEN a task is `in_review`, `done`, `blocked`, or already superseded by decomposition
- WHEN a revise payload is submitted
- THEN Cortex-IA rejects the request with an actionable conflict and leaves state unchanged

#### Scenario: unknown JSON fields fail
- GIVEN a revise plan JSON contains any unknown top-level field, unknown task field, or unknown SDD contract field
- WHEN the CLI decodes the payload
- THEN decoding fails before store mutation and no partial task update is committed

### Requirement: REQ-WORK-003 SDD contract and workspace-file pin safety

Revising an SDD task MUST validate the complete replacement contract before mutation. New workspace-file pins MUST be read-only validated against current bytes and project identity before the database transaction changes state; the operation MUST reject attempts to drop an existing SDD binding unless the task is explicitly direct-work compatible and unbound by policy.

#### Scenario: valid new workspace pins are accepted
- GIVEN a replacement SDD contract has `version: 1`, preserves the live task's existing `workflow`, `change_id`, and `spec_plane`, carries complete non-null SDD fields, requirement IDs from that task's current contract, and workspace-file pins for current contract artifacts
- WHEN every pin project canonicalizes to the task workspace and every digest matches current bytes
- THEN the revise operation may proceed to the atomic database update

#### Scenario: stale or mismatched pin is rejected
- GIVEN any replacement workspace-file pin has a mismatched project, unsafe locator, symlink traversal, missing file, or changed SHA-256 digest
- WHEN the revise operation is requested
- THEN Cortex-IA rejects the request before mutation and reports that fresh contract review is required

#### Scenario: SDD binding cannot be silently removed
- GIVEN the current task has an SDD contract
- WHEN the revise payload omits `sdd_contract` or supplies null
- THEN Cortex-IA rejects the request instead of converting the task to an unbound direct task

### Requirement: REQ-WORK-004 CLI payload contract and operational protections

The CLI MUST expose the revise operation through a bounded JSON plan file or stdin, using explicit identity protections. The payload MUST name board, task, workspace/project, expected revision, existing workflow/change identity expectations, and replacement definition fields; mismatches fail closed.

#### Scenario: identity mismatch fails
- GIVEN the payload task ID, board ID, workspace/project, existing change ID, or expected workflow does not match live task state
- WHEN the CLI invokes revise
- THEN Cortex-IA rejects the request without mutating state

#### Scenario: supported fields revise atomically
- GIVEN a valid payload names only supported fields and includes objective, acceptance criteria, verification, allowed files, and SDD contract
- WHEN the CLI applies the plan
- THEN all supplied definition fields and the SDD contract change in one transaction or none changes

#### Scenario: structural validation precedes operational mutation
- GIVEN planning artifacts for `work-revise` fail OpenSpec structural validation
- WHEN an operator attempts to use proposed task JSON as an implementation contract
- THEN the workflow stops for semantic/structural review and no existing `pi-native-adaptive-ux` board task is revised

## Design

### Operation shape

Add a store-level method such as `ReviseWorkDefinition(ctx, WorkRevisionPlan)` in a new delegation file rather than expanding existing `internal/delegation/work.go`. The method should:

1. Decode and normalize the plan with `json.Decoder.DisallowUnknownFields` at every level.
2. Resolve/canonicalize the requested workspace/project and allowed files using existing task path rules.
3. Validate replacement SDD contract with the same encode/decode rules used by create/review.
4. Perform read-only pin validation before `BEGIN IMMEDIATE`: `verifyWorkspacePins` for workspace files and `verifyLocalCortexPins` for local Cortex pins.
5. Enter one immediate transaction, reload the live task, and verify original current task identity (task ID, board ID, workspace/project, workflow, change ID, spec plane), expected revision, status in `ready|backlog` only, no active claim, no active file leases, no active review, no decomposition/superseded replacement, and open SDD change. Previously stored old pins may be stale because updating them is the purpose; only the new replacement pins are validated against current bytes.
6. Preserve ID, board, workspace, dependencies, and conversation ownership; update only title if explicitly permitted by the contract, objective, acceptance criteria, verification, allowed files JSON, and contract JSON.
7. Increment revision once, update `updated_at`, and append a `definition_revised` audit event containing bounded old/new definition hashes rather than full prompts.

### CLI shape

Add a `cortex-ia work revise --plan <file|@stdin>` entry in `internal/app/work.go`. The CLI should parse only this supported plan format and call the store method. It should not accept ad-hoc flags that can accidentally omit the SDD contract. The planned future CLI schema is exactly this versioned, identity-protected shape; the current CLI must not accept `proposed-tasks.json` as a recovery payload:

```json
{
  "version": 1,
  "task_id": "pi-native-adaptive-ux-1.4",
  "board_id": "pi-native-adaptive-ux",
  "project": "D:/lleontor705/pi-cortex-ia",
  "expected_revision": 1,
  "expected_status": "backlog",
  "expected_workflow": "sdd-lite",
  "expected_change_id": "pi-native-adaptive-ux",
  "definition": {
    "title": "[rendering] Add compact themed Cortex view helpers",
    "objective": "...",
    "acceptance_criteria": "...",
    "verification": "...",
    "allowed_files": ["..."],
    "sdd_contract": {
      "version": 1,
      "workflow": "sdd-lite",
      "change_id": "pi-native-adaptive-ux",
      "spec_plane": "hybrid",
      "pins": [{"transport": "workspace_file", "project": "D:/lleontor705/pi-cortex-ia", "locator": "openspec/changes/pi-native-adaptive-ux/plan.md", "sha256": "514b397ae490be48d919271f839e433c34751885ca3dfd00e2b65973551deda7"}],
      "requirement_ids": ["REQ-PI-NATIVE-001"]
    }
  }
}
```

For the Pi recovery operation, the semantic review should require one future `work revise` plan per existing task in board `pi-native-adaptive-ux`, not a new board and not a CLI recovery payload today. Each replacement contract must preserve the task's ID, board ID, workspace `D:/lleontor705/pi-cortex-ia`, existing replacement workflow, `change_id: pi-native-adaptive-ux`, `spec_plane`, and dependencies while updating stale old pins to current bytes from the Pi plan. Stale original pins are allowed only as observed old state; every new pin must validate against current bytes. Task `pi-native-adaptive-ux-1.4` may receive only the already-reviewed correction to its stale implementation scope; the current dependency on `pi-native-adaptive-ux-1.3` remains.

### Failure and atomicity

Validation failures, unknown fields, stale revisions, identity mismatches, null/omitted SDD contracts for SDD tasks, workflow/change/spec-plane drops, active claims, active leases, in-review state, done state, blocked state, superseded parents, closed SDD changes, or pin mismatches return errors before committing any state. Multi-record validation uses defensive copies and never mutates caller input in-place. Runtime pin checks are read-only and precede DB mutation; transaction-time checks repeat identity/lifecycle/CAS protections to close stale-observation windows. The existing CLI does not yet have `work revise`; that absence is why the two implementation tasks below exist, not a design blocker.

## Tasks

- [ ] work-revise-1 Implement delegation revise engine and store tests
  Requirements: REQ-WORK-001, REQ-WORK-002, REQ-WORK-003
  Allowed files target: internal/delegation/work_revise.go; internal/delegation/work_revise_test.go
  Verification: go test -count=1 ./internal/delegation -run 'TestReviseWorkDefinition'
  Forecast: <=250 changed lines target, <=500 Go cap.

- [ ] work-revise-2 Add CLI revise plan parsing and app tests
  Requirements: REQ-WORK-002, REQ-WORK-004
  Allowed files target: internal/app/work_revise.go; internal/app/work_revise_test.go; minimal help-line edit in internal/app/work.go only if needed.
  Verification: go test -count=1 ./internal/app -run 'TestWorkRevise'
  Forecast: <=250 changed lines target, <=500 Go cap.
