# Tool flow delta

## ADDED Requirements

### Requirement: REQ-TOOLS-001: Durable attributable delivery
The service MUST persist bounded current-attempt submissions and release authority atomically without granting approval.

#### Scenario: Happy
- GIVEN live owner submits evidence
- WHEN delivery commits
- THEN receipt, state and lease release persist together

#### Scenario: Edge
- GIVEN retry follows an earlier submission
- WHEN current receipt is queried
- THEN earlier attempt evidence is not presented as current

#### Scenario: Failure
- GIVEN claim, revision or changed path is invalid
- WHEN delivery is requested
- THEN all writes reject without partial effects

### Requirement: REQ-TOOLS-002: Leased artifact output
Artifact writes MUST require host-bound live lease authority immediately before output.

#### Scenario: Happy
- GIVEN implement owns output path
- WHEN conversion writes
- THEN authorized output succeeds

#### Scenario: Edge
- GIVEN conversion outlives its lease
- WHEN write begins
- THEN output is refused

#### Scenario: Failure
- GIVEN reviewer requests an external output path
- WHEN tool executes
- THEN write and renderer launch reject

### Requirement: REQ-TOOLS-003: Live controller maintenance
Renewal MUST authenticate live controller authority; release MUST use acknowledged transactional batching.

#### Scenario: Happy
- GIVEN busy session owns live authority
- WHEN timer ticks
- THEN claim and leases renew together

#### Scenario: Edge
- GIVEN session becomes idle or unknown
- WHEN next maintenance runs
- THEN renewal stops without revival

#### Scenario: Failure
- GIVEN batch release fails
- WHEN controller processes result
- THEN failure remains visible and authority is not falsely cleared

### Requirement: REQ-TOOLS-004: Integrated byte verification
Operations MUST return exact-byte digests while retaining backend verification and compatibility tools.

#### Scenario: Happy
- GIVEN artifact bytes are committed
- WHEN write returns
- THEN digest and length match committed bytes

#### Scenario: Edge
- GIVEN snapshot has an expected pin
- WHEN read returns
- THEN pin is verified without a second manual hash

#### Scenario: Failure
- GIVEN content mismatches a pin
- WHEN verification runs
- THEN operation fails closed

### Requirement: REQ-TOOLS-005: Coherent job observation
Job queries MUST distinguish acceptance, cancellation request, confirmed termination and receipt availability.

#### Scenario: Happy
- GIVEN terminal job has receipt
- WHEN query runs
- THEN one coherent view includes receipt

#### Scenario: Edge
- GIVEN cancellation is pending
- WHEN bounded wait expires
- THEN request is not reported as termination

#### Scenario: Failure
- GIVEN job is lost without proof
- WHEN query runs
- THEN fence and required reconciliation remain explicit

### Requirement: REQ-TOOLS-006: Bounded exact snapshots
Snapshot readers MUST stream existing export with exact identity checks and bounded resources.

#### Scenario: Happy
- GIVEN export contains one matching observation
- WHEN reader completes successfully
- THEN exact content and digest return

#### Scenario: Edge
- GIVEN duplicate ID occurs after a match
- WHEN stream ends
- THEN reader rejects ambiguity

#### Scenario: Failure
- GIVEN producer fails or output exceeds bounds
- WHEN read runs
- THEN verification rejects without fallback
