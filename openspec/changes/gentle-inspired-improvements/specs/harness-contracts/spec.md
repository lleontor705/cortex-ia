# Delta for Harness Contracts

Scope: proposal `openspec/changes/gentle-inspired-improvements/proposal.md`; evidence cortex:210,211,213,215 and `.cortex-ia/discovery.md`. These are new traceable requirements, not claims that baseline protections are absent. Static operational-scope, budget and latch mismatches MUST receive isolated reproduction before fixes. This phase authorizes no runtime changes, board or tasks.

## ADDED Requirements

### Requirement: REQ-HARNESS-001: Admit explicit operational effects without file authority
Trace: REC-01.
The harness MUST accept empty `allowed_files` only for explicitly scoped operational/database effects under valid implementation authority. Acceptance MUST NOT grant repository writes, bypass host permissions, or claim shell sandboxing. Review MUST examine the declared target and preserve unrelated pre-existing workspace changes; ordinary file changes still require scoped claims and leases.

#### Scenario: Happy operational admission
- GIVEN a valid operational dispatch, empty file scope and explicit authorized target/effects
- WHEN transport validates and the authorized controller executes an isolated operation
- THEN admission succeeds, target evidence is reviewable, and repository baseline bytes remain unchanged.

#### Scenario: Edge unrelated drift
- GIVEN unrelated pre-existing workspace modifications and a correctly scoped operational result
- WHEN independent review evaluates the operation
- THEN unrelated drift alone does not fail target verification or expand writable scope; the baseline remains preserved.

#### Scenario: Failure missing effect authority
- GIVEN empty file scope with missing or ambiguous operational effects, or an attempted repository write
- WHEN admission or mutation authorization is checked
- THEN the unauthorized action is rejected with a visible reason and no new file authority.

### Requirement: REQ-HARNESS-002: Reconcile documented budgets and runtime semantics
Trace: REC-02.
Documentation and parsing MUST agree on `budget.max_turns` versus `max_steps`, including explicit compatibility and conflict handling. Dispatch limits remain advisory; planner has no step ceiling. Emergency limits, five cleanup calls, initialization rejection and repetition warnings MUST retain canonical work-protocol semantics without automatic SQLite transitions. Reconciliation MUST NOT silently ignore supplied budget values.

#### Scenario: Happy documented budget
- GIVEN a supported documented budget form
- WHEN a verified child reaches its advisory threshold
- THEN one current model-visible warning appears, ordinary tools remain available, and task state is unchanged.

#### Scenario: Edge emergency and repetition
- GIVEN non-planner emergency exhaustion or repeated identical terminal outcomes, and a planner control case
- WHEN boundary calls occur
- THEN emergency exhaustion allows only five cleanup calls, repetition only warns, planner remains uncapped, and none automatically changes task state.

#### Scenario: Failure invalid or conflicting configuration
- GIVEN invalid emergency/repetition configuration or conflicting budget forms
- WHEN initialization or dispatch validation occurs
- THEN a visible rejection identifies the conflict rather than silently defaulting; no work authority is created.

### Requirement: REQ-HARNESS-003: Make latched continuation explicit and role-correct
Trace: REC-03.
Failure guidance MUST identify the actual supported continuation path and responsible role. Orchestrator reconciliation/retry MUST precede fresh execution; recovery MUST NOT recreate claims, leases or approvals. Missing capabilities MUST be reported, not replaced by invented unlatch tools. Accepted external jobs MUST be reconciled before any fresh dispatch.

#### Scenario: Happy supported continuation
- GIVEN a latched failure with reconciled durable state
- WHEN the orchestrator follows the documented supported continuation path
- THEN a fresh authorized attempt can proceed without replaying the failed attempt or inheriting its tokens.

#### Scenario: Edge unavailable continuation
- GIVEN recovery capability is absent or the task identity is unknown
- WHEN continuation is requested
- THEN guidance reports the limitation and required orchestrator action; execution remains blocked without fabricated task state.

#### Scenario: Failure unauthorized recovery
- GIVEN expired authority or an unreconciled accepted external job
- WHEN a leaf attempts recovery, retry or automatic native fallback
- THEN execution is refused, the existing evidence is preserved, and reconciliation remains with the authorized controller/orchestrator.
