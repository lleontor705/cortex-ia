# Spec delta — agent testing-quality gates (Wave 1)

Domain: agent-quality | Plane: openspec | Change: add-agent-testing-gates

## ADDED Requirements

### Requirement: REQ-ATG-001 Implementer-side mutation evidence gate for fast-TDD-eligible tasks
The canonical work protocol SHALL require, for fast-TDD-eligible code tasks (a persistent test delta in a testable unit), that the implementer executes 1–2 minimal reversible mutations per the mutation-testing skill and records their outcome BEFORE calling the `in_review` transition. The TDD exemptions (documentation, declarative configuration, generated output, work without a reliable fast oracle) MUST exempt this evidence too, mirroring the existing exemption list, with the reason recorded. Outcome semantics: KILLED proves the covering test can fail; SURVIVED marks a shallow test and the implementer MUST strengthen assertions and re-run to KILLED before transitioning; STATIC-ANALYSIS is permitted only when the environment cannot execute mutations and its minimum bar — explicit assertion of error boundaries, return payloads, and failure branches — MUST be a required field, never a silent skip.

- **Test:** grep -q "mutation evidence" internal/assets/skills/_shared/cortex-work-protocol.md

#### Scenario: Killed mutation proves a genuine verifier
- **GIVEN** a fast-TDD-eligible task whose implementation is complete and focused oracle is green
- **WHEN** the implementer applies one boundary-operator mutation to the changed logic and reruns the covering test
- **THEN** the test fails with a non-zero exit code, the mutation is reverted to clean git state, and the receipt records mutation outcome KILLED with the mutation description and the covering command

#### Scenario: Survived mutation forces test strengthening before transition
- **GIVEN** a mutation run where the covering test still passes despite the broken logic
- **WHEN** the implementer evaluates transition preconditions
- **THEN** transition to `in_review` is forbidden until the assertions are strengthened and the rerun mutation yields KILLED

#### Scenario: Unsupported tooling falls back to static assertion analysis
- **GIVEN** an environment where mutated code cannot compile or execute
- **WHEN** the implementer records mutation evidence
- **THEN** the receipt carries a STATIC-ANALYSIS outcome naming the explicit assertions covering error boundaries, return payloads, and failure branches of the changed unit, never an omitted field

#### Scenario: Exempt work kinds skip the mutation gate
- **GIVEN** a documentation, declarative-configuration, generated-output, or no-oracle task
- **WHEN** the implementer completes verification
- **THEN** no mutation evidence is required and the receipt records the exemption reason

### Requirement: REQ-ATG-002 Mandatory mutation field in fast-TDD and implement receipts
The fast-tdd skill receipt schema SHALL carry a `mutation` evidence attribute alongside red, green, refactor, and regression, and the implement skill verification step MUST reference the same attribute for fast-TDD-eligible work. Accepted final values are KILLED and STATIC-ANALYSIS with bounded mutation or analysis descriptions; SURVIVED MUST never appear in a receipt transitioning a task to review; a missing or empty `mutation` field on an eligible task makes the completion evidence incomplete and MUST produce INCONCLUSIVE or BLOCKED, never PASS.

- **Test:** grep -q "mutation" internal/assets/skills/fast-tdd/SKILL.md

#### Scenario: Receipt carries killed mutation evidence
- **GIVEN** a fast-TDD-eligible task whose mutation run killed the covering test
- **WHEN** the implementer emits the completion receipt
- **THEN** the evidence object includes the mutation field with outcome KILLED and a bounded description of mutation, command, and revert

#### Scenario: Missing field blocks acceptance on eligible tasks
- **GIVEN** an eligible-task receipt whose evidence lacks the mutation field
- **WHEN** the receiving controller or reviewer checks required attributes
- **THEN** the verdict is INCONCLUSIVE or BLOCKED citing incomplete evidence, never PASS

#### Scenario: Survived status is transient and never ships
- **GIVEN** a mutation run that survived
- **WHEN** the implementer strengthens the test and reruns the mutation
- **THEN** only the post-strengthening outcome is persisted in the receipt and no transition to `in_review` carries SURVIVED

### Requirement: REQ-ATG-003 Reviewer test-strength verification and perpetually-green lens
The code-review-adversary skill SHALL define once, as single source, the test-strength lens: a perpetually-green test — one that would pass under any behavior change of the logic it claims to cover — is an oracle gap and MUST FAIL a code task under Lens 1. The reviewer Phase 3 SHALL independently verify the implementer's mutation evidence against the actual diff and test source while retaining the existing prohibition on writing or executing temporary tests; where mutation execution was impossible, the STATIC-ANALYSIS minimum bar is acceptable sufficient evidence.

- **Test:** grep -q "perpetually-green" internal/assets/skills/code-review-adversary/SKILL.md

#### Scenario: Perpetually-green oracle fails the review
- **GIVEN** a diff whose new tests only assert non-error outcomes without boundary, payload, or failure-branch assertions
- **WHEN** the reviewer applies the test-strength lens
- **THEN** the review verdict is FAIL citing Lens 1 oracle gap with the specific unfailable assertion identified

#### Scenario: Independent verification of claimed mutation evidence
- **GIVEN** a receipt claiming a KILLED mutation for a named mutation scenario
- **WHEN** the reviewer inspects the diff, the test source, and the claimed covering assertions
- **THEN** the reviewer confirms the named mutation would flip an actual assertion; a contradiction or unfalsifiable claim yields FAIL citing the mismatch, without the reviewer writing any test

#### Scenario: Static analysis bar accepted when execution is unsupported
- **GIVEN** an eligible-task receipt recording STATIC-ANALYSIS due to environment limits
- **WHEN** the reviewer inspects the cited assertions for error boundaries, return payloads, and failure branches
- **THEN** the review accepts the evidence when the static bar is met and must not FAIL solely because no executed mutation was available

### Requirement: REQ-ATG-004 Single normative source with mirrored cross-references
The normative mutation-evidence MUST SHALL live exactly once in `_shared/cortex-work-protocol.md` (lifecycle gate clause plus evidence-composition clause); `internal/assets/AGENTS.md` SHALL mirror it only as cross-reference bullets in the Pragmatic Invariants and Pre-Transition Workload Preflight sections, with the Phase-4 review diagram caption upgraded from decorative to normative cross-reference; `agents/reviewer.md` SHALL cite `skills/code-review-adversary/SKILL.md` as the elaborated single source for the 5-phase/3-lens pipeline and MUST not fork normative wording of the skill or protocol.

- **Test:** grep -q "Mutation Evidence Gate" internal/assets/AGENTS.md

#### Scenario: Protocol holds the normative clause
- **GIVEN** the post-change shared protocol file
- **WHEN** the lifecycle and completion-receipt sections are read
- **THEN** exactly one normative mutation-evidence clause and one perpetually-green reviewer duty exist there

#### Scenario: AGENTS.md mirrors without restating
- **GIVEN** the post-change AGENTS.md
- **WHEN** the invariants, preflight, and Phase-4 diagram text are audited
- **THEN** each testing-gate mention is a pointer to the canonical protocol section and introduces no independent normative wording

#### Scenario: Reviewer agent cites the adversary skill
- **GIVEN** the post-change agents/reviewer.md pipeline text
- **WHEN** an auditor locates the test-strength lens wording
- **THEN** the agent text is a pointer to the code-review-adversary skill as elaborated single source and no duplicated lens definition exists

### Requirement: REQ-ATG-005 Oracle-strength mandate, advisory coverage delta, and TestREQ traceability naming
The implement and fast-tdd skills SHALL mandate that every new or changed test demonstrates failure capability by asserting at least one boundary, return payload, or failure branch; tests that cannot fail MUST not ship for eligible tasks. Implementers of test-bearing changes MAY compute a before/after coverage delta via `go test -coverprofile` and report it in the receipt as an advisory-only signal that MUST never gate this wave. REQ-bound tests SHALL follow the naming convention `TestREQ_{DOMAIN}_{NNN}_<slug>` with language-conditional adaptation allowed; the planner skill, the cortex-convention traceability chain, and the workflow-map structural-validation note SHALL record the convention, and planner verification commands for such tasks target these names. Convention-level only: no Go validator change in this wave.

- **Test:** grep -q "TestREQ_" internal/assets/skills/planner/SKILL.md

#### Scenario: Failable test satisfies the oracle-strength mandate
- **GIVEN** a new test asserting an error boundary and a specific failure branch
- **WHEN** the implementer reviews test strength before transition
- **THEN** the test meets the failure-capability requirement and the receipt names its discriminating assertions

#### Scenario: Coverage delta is advisory only
- **GIVEN** a test-bearing change whose receipt reports a before/after coverage delta
- **WHEN** the reviewer evaluates the transition
- **THEN** the delta informs findings only and a low or unmeasured delta MUST not FAIL the task on that basis alone

#### Scenario: Planner verification targets REQ test names
- **GIVEN** a planned task bound to a requirement ID that delivers a persistent test
- **WHEN** the planner writes the task verification command
- **THEN** the command targets the matching TestREQ-style test name via the runner selection flag where the language supports it

#### Scenario: Non-REQ work keeps standard naming
- **GIVEN** an exploratory or non-REQ-bound change
- **WHEN** tests are authored
- **THEN** the TestREQ naming convention does not apply and standard repository test naming stands
