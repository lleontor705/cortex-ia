# Cortex-only contract lifecycle

## ADDED Requirements

### Requirement: REQ-CORTEX-001 — Verifiable specification pin
Cortex-only MUST carry identified requirements, three Given/When/Then cases each, design/interfaces, acceptance commands, risks/non-goals and task traceability in a full snapshot observation. Pin MUST include transport/project identity, real observation ID and SHA-256 of exact UTF-8 content bytes, without newline or Unicode normalization. Retain the exact content for comparison; hash the content field, not display metadata. Mutable topic keys, timestamps and invented revisions MUST NOT be pins.

#### Scenario: Happy path
- GIVEN a saved snapshot and full retrieval
- WHEN planner validates completeness and records its ID/content/digest
- THEN implement and independent reviewer retrieve that ID and verify identical bytes before relying on it.

#### Scenario: Edge case
- GIVEN `cortex_revision_history` returns an empty array
- WHEN pinning the initial snapshot
- THEN ID plus content/digest remains valid; no revision is fabricated and history is optional.

#### Scenario: Error state
- GIVEN a missing observation, truncated retrieval or digest mismatch
- WHEN validation or review runs
- THEN it fails closed and cannot authorize acceptance; a changed contract requires a new snapshot/pin and fresh review.

### Requirement: REQ-CORTEX-002 — Equivalent routing and independent acceptance
`spec_plane=cortex` MUST omit OpenSpec writes/validation/archival for decision-map, Lite and every Full phase while retaining equivalent contract validation. OpenSpec/hybrid MUST retain their current artifact gates. Only independent SQLite approval completes work; changed scope, content or delivered diff invalidates earlier acceptance applicability, without rewriting historical approval records.

#### Scenario: Happy path
- GIVEN Cortex-only and a verified complete pin
- WHEN planner creates the bounded DAG and reviewer audits delivery
- THEN no OpenSpec is required and separate Spec/Standards axes and current-revision approval remain mandatory.

#### Scenario: Edge case
- GIVEN OpenSpec or hybrid selection
- WHEN planning/review or archival executes
- THEN OpenSpec requirements remain; hybrid links evidence without replacing its source contract.

#### Scenario: Error state
- GIVEN no authoritative contract, a stale pin or only implementer/AGY success
- WHEN review is requested
- THEN no PASS or done results; missing specification is INCONCLUSIVE and identified drift is FAIL/BLOCKED pending replanning.

## Verification
Each consumer must reference the shared `cortex-convention.md`, without duplicating pin rules. Review walks decision-map, Lite, Full propose/spec/design/tasks/review/archive across all three planes and checks missing/drifted pins. Use ephemeral exports of real full retrievals: `python3 -c 'import hashlib,pathlib,sys; assert hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest()==sys.argv[2]' CONTENT_FILE PIN_DIGEST` must exit0 for matching bytes and nonzero after a one-byte mutation of a disposable copy. Uppercase arguments are receipt-bound paths/digest, never fabricated values. Review verifies returned content boundaries before export. No new get-by-revision API, schema migration or persistent smoke suite.
