# Delta for Distribution Security

Authority: `openspec/changes/gentle-inspired-improvements/proposal.md`; cortex:209,211,213,215; discovery. Production keys/secrets, release publishing, deployment and active installation sync are NOT authorized. Trust bootstrap/rotation mechanisms remain open for design; authentication itself is mandatory.

## ADDED Requirements

### Requirement: REQ-DISTRIBUTION-001: Make build qualification reproducible
Trace: REC-08.
CI actions MUST use verified immutable commit SHA references. Build tool versions, manifests and lockfiles MUST agree with an explicit reproducible toolchain selection. Provenance MUST identify source, dependencies, commands and actual toolchain/OS outcomes. Existing unit, lint, Linux CGO race and production approval gates MUST be preserved; required OS checks MUST gate qualification, not merely appear in a matrix.

#### Scenario: Happy reproducible qualification
- GIVEN fixed source, dependency inputs and supported OS/toolchain declarations
- WHEN isolated qualification runs
- THEN recorded commands and versions are reproducible, required checks exit zero, and action references resolve to verified immutable commits.

#### Scenario: Edge unavailable OS runner
- GIVEN a required OS runner or compiler is unavailable
- WHEN qualification is assessed
- THEN that OS remains unverified and release readiness is withheld; cross-compilation alone is not runtime verification.

#### Scenario: Failure mutable or failing input
- GIVEN mutable action references, manifest/toolchain mismatch or a failed required check
- WHEN qualification is evaluated
- THEN acceptance fails with the specific input/check; existing gates are not bypassed.

### Requirement: REQ-MCP-001: Qualify versioned integration inputs
Trace: REC-09.
Managed MCP integration MUST use versioned reproducible inputs and explicit compatibility evidence against the actual exposed schema. Qualification MUST be bounded, isolated and secret-free; missing capabilities MUST remain unknown or incompatible rather than fabricated. Ownership and malformed-config rejection MUST remain intact.

#### Scenario: Happy compatible fixture
- GIVEN a versioned integration and contract-faithful isolated fixture
- WHEN documented bounded qualification runs
- THEN actual capabilities and versions are recorded and required exchanges succeed reproducibly.

#### Scenario: Edge missing optional capability
- GIVEN the actual schema omits an optional capability
- WHEN compatibility is assessed
- THEN the limitation is reported without invented tools, parameters or unsupported feature claims.

#### Scenario: Failure incompatible input
- GIVEN malformed managed configuration or a missing required capability
- WHEN validation runs
- THEN qualification fails visibly without modifying active configuration or overwriting unmanaged ownership.

### Requirement: REQ-UPDATE-001: Authenticate the entire update chain
Trace: REC-10.
Updates MUST authenticate release metadata against independently established trust, bind repository/version and artifact identity, verify artifact bytes before use, and reject unsafe archive paths/types before installation. Missing/invalid signatures, unknown or unauthorized rotated trust, replay/downgrade and binding/digest failures MUST fail closed without unsigned fallback or replacing the working binary. Design MUST specify bootstrap, rotation and version policy. Producer-to-consumer fixture evidence MUST cover signed metadata, digest and extraction safety; production activation requires separate explicit authorization.

#### Scenario: Happy authenticated fixture
- GIVEN trusted non-production signing fixtures and matching repository/version/artifact metadata
- WHEN an isolated update follows metadata verification, byte verification and safe extraction
- THEN only the authenticated intended binary is installed in the temporary target.

#### Scenario: Edge rotation and replay
- GIVEN authorized rotation fixtures plus unknown-key and old-version variants
- WHEN trust/version checks run
- THEN only rotation authorized by the selected trust policy succeeds; unknown trust and replay/downgrade are rejected.

#### Scenario: Failure broken chain
- GIVEN missing/bad signatures, mismatched bindings/digests or unsafe archive members
- WHEN each negative fixture is processed
- THEN installation fails visibly, the working binary remains unchanged, and no unsigned fallback occurs.

### Requirement: REQ-SECRET-001: Assess distributed secret-derived privilege safely
Trace: REC-11.
Binary-secret risk MUST be assessed from source and synthetic builds without reading secrets or asserting unverified credential privilege. Evidence MUST distinguish public identifiers from authority-bearing material; confirmed risk reduction MUST preserve intended behavior. Credential redesign/activation requires separate authorization.

#### Scenario: Happy synthetic assessment
- GIVEN synthetic build values and source consumers
- WHEN distribution exposure is inspected
- THEN evidence identifies observable embedding and usage without retrieving production secrets.

#### Scenario: Edge unknown privilege
- GIVEN privilege cannot be established without external evidence
- WHEN risk is reported
- THEN privilege remains explicitly unverified; neither compromise nor harmlessness is asserted.

#### Scenario: Failure disclosure
- GIVEN a proposed check requires real credentials or records sensitive values
- WHEN reviewed
- THEN the check is rejected and replaced by secret-free evidence or an explicit blocker.
