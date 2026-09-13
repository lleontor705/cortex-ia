# Delta for Context and Review

Authority: `openspec/changes/gentle-inspired-improvements/proposal.md`; cortex:210,211,213,215; `.cortex-ia/discovery.md`. Requirements preserve host instructions and SQLite authority. Consumer-local versus shared read-only projection remains a design decision; no new API/status fields are prescribed.

## ADDED Requirements

### Requirement: REQ-CONTEXT-001: Resolve exact context with provenance
Trace: REC-04.
Context loading MUST expose exact resolved skill/artifact paths, source provenance and applicable precedence. Valid explicit dispatch locators take precedence over inventory fallback, never over host policy. Host-installed and embedded inventories MUST remain distinguishable; unresolved names, invalid locators and ambiguous same-precedence collisions MUST fail visibly, without silent substitution.

#### Scenario: Happy explicit resolution
- GIVEN an authorized exact skill path and artifact locator
- WHEN context is loaded
- THEN the selected paths, provenance and precedence are visible and match the supplied artifacts.

#### Scenario: Edge inventory difference
- GIVEN host and embedded inventories differ with a valid explicit host locator
- WHEN resolution occurs
- THEN the host artifact is selected transparently and the embedded inventory is not presented as installed availability.

#### Scenario: Failure collision
- GIVEN a missing artifact, invalid locator or unresolved same-precedence collision
- WHEN loading is attempted
- THEN a visible error names the failed reference or candidates and dependent work does not proceed with guessed content.

### Requirement: REQ-CONTEXT-002: Deduplicate instructions without losing obligations
Trace: REC-05.
Normative concepts MUST have one canonical source and triggered, exact pointers usable through permitted loading mechanisms. Always-loaded text MUST retain universally required invariants; conditional detail MUST be discoverable when needed. Design MUST define a deterministic residual-duplication metric and numeric acceptance threshold before edits; savings MUST NOT be asserted without baseline evidence. Routing MUST remain proportional, with no prompt compiler or host-system replacement.

#### Scenario: Happy canonical loading
- GIVEN a conditional workflow and accessible canonical contract
- WHEN its documented trigger occurs
- THEN the controller loads the exact contract and can identify ordered steps and checkable completion criteria without duplicate normative copies.

#### Scenario: Edge simple route
- GIVEN a bounded direct-change objective
- WHEN routing and prompt composition are evaluated
- THEN unnecessary SDD phases are absent and mandatory authority boundaries remain available.

#### Scenario: Failure inaccessible obligation
- GIVEN a broken pointer, conflicting duplicate rule or exceeded approved residual threshold
- WHEN instruction checks run
- THEN acceptance fails with the offending reference/count; no unsupported savings claim passes.

### Requirement: REQ-REVIEW-001: Review normalized current evidence independently
Trace: REC-06.
Normalization MUST precede verification, evidence freeze and review. Changed files/contracts MUST invalidate acceptance applicability while preserving fingerprints and approval history; authorized review-refresh and fresh independent review are required where applicable. Units MUST respect canonical workload budgets without code-golf. Risk-based blind review MUST assess evidence independently, not impose consensus vetoes.

#### Scenario: Happy normalized review
- GIVEN a bounded normalized change
- WHEN checks, freeze and independent review occur in order
- THEN evidence identifies the same current content and only authorized reviewer PASS can produce done.

#### Scenario: Edge later change
- GIVEN previously approved files change under later authorized work
- WHEN closure is considered
- THEN stale acceptance is rejected until orchestrator review-refresh and independent re-review; historical approval remains unchanged.

#### Scenario: Failure misleading readiness
- GIVEN post-freeze edits, oversized compressed code or reviewer disagreement
- WHEN acceptance is evaluated
- THEN stale/oversized work fails, while disagreement is resolved by evidence and authorized review rather than consensus voting.

### Requirement: REQ-STATE-001: Distinguish information from authority
Trace: REC-07.
Views MUST distinguish informative notes, blockers and role-admissible next actions, retaining phase/task/verdict distinctions and provenance. Stale memory MUST NOT override current SQLite state.

#### Scenario: Happy actionable state
- GIVEN current authoritative state and an informative note
- WHEN displayed
- THEN the note does not become a blocker and next actions identify the authorized role.

#### Scenario: Edge stale evidence
- GIVEN memory conflicts with current work state
- WHEN projected
- THEN stale evidence is visibly distinguished and current authority governs actions.

#### Scenario: Failure unavailable authority
- GIVEN authoritative state cannot be retrieved
- WHEN a view recommends continuation
- THEN uncertainty is visible and no readiness, approval or writable authority is inferred.
