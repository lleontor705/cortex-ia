# Delta for Verification Policy

Authority: `openspec/changes/gentle-inspired-improvements/proposal.md`; cortex:209,210,211,213,215 and `.cortex-ia/discovery.md`. AUTO/HYBRID/current_workspace applies. Specification coverage is not runtime proof; no tasks or design commitments are created here.

## ADDED Requirements

### Requirement: REQ-VERIFY-001: Reconcile bounded critical regression policy
Trace: REC-13.
Source test policy and documentation MUST consistently permit the user-authorized small persistent regressions for authority, transport, recovery and update verification, alongside existing TUI and simple install-copy coverage. Tests MUST protect observable contracts, use temporary homes and synthetic inputs, and remain modular: dedicated test files at most 250 lines, no suite appended to files already exceeding 300 lines, and canonical task workload caps. Deeper unrelated transactional exploration remains ephemeral; tests MUST NOT weaken contracts or use real developer state.

#### Scenario: Happy critical regression
- GIVEN an authorized critical boundary and isolated fixture
- WHEN a bounded persistent test is proposed and executed
- THEN policy permits that test, it detects the negative condition and passes for the correct behavior, with a reproducible command and expected exit zero for the accepted implementation.

#### Scenario: Edge existing coverage
- GIVEN existing TUI/install coverage or deeper unrelated exploratory smokes
- WHEN policy is reconciled
- THEN existing permitted coverage remains valid, exploratory checks remain ephemeral, and new suites use bounded dedicated files instead of enlarging oversized files.

#### Scenario: Failure unsafe or weakened oracle
- GIVEN a test accesses real user state, exceeds scope/budgets or changes expected rejection into silent skipping
- WHEN reviewed
- THEN acceptance fails, no unsafe test is executed, and the original behavioral contract is retained.

### Requirement: REQ-VERIFY-002: Establish deterministic local baselines without collection
Trace: REC-14.
Before behavioral changes, implementers MUST capture bounded local baseline evidence with source revision, relevant dirty-state boundaries, exact command, toolchain/OS, expected/actual result and exit code. The same discriminating oracle MUST evaluate before/after behavior. Static pinning, operational-budget and latch findings MUST remain static until isolated reproduction; security exposure MUST NOT be inferred from inspection alone. No new telemetry, collection endpoint, benchmark promise or unmeasured savings is authorized.

#### Scenario: Happy comparable baseline
- GIVEN a bounded isolated oracle and fixed inputs
- WHEN baseline and changed implementation are evaluated
- THEN local evidence identifies comparable conditions and actual outcomes; claims are limited to what the oracle demonstrates.

#### Scenario: Edge unavailable environment
- GIVEN a required tool/OS is absent or the oracle cannot reproduce a suspected issue
- WHEN evidence is summarized
- THEN the limitation or non-reproduction is explicit, no runtime PASS is inferred, and unsupported comparisons remain inconclusive.

#### Scenario: Failure misleading metrics
- GIVEN a summary invents speed/token savings, treats static inspection as runtime proof or adds telemetry collection
- WHEN acceptance is reviewed
- THEN the summary/change is rejected until unsupported claims and collection are removed or separately authorized.

## Recommendation index

| Recommendation | Requirement | Domain file under this change's specs/ |
|---|---|---|
| REC-01 | REQ-HARNESS-001 | harness-contracts/spec.md |
| REC-02 | REQ-HARNESS-002 | harness-contracts/spec.md |
| REC-03 | REQ-HARNESS-003 | harness-contracts/spec.md |
| REC-04 | REQ-CONTEXT-001 | context-review/spec.md |
| REC-05 | REQ-CONTEXT-002 | context-review/spec.md |
| REC-06 | REQ-REVIEW-001 | context-review/spec.md |
| REC-07 | REQ-STATE-001 | context-review/spec.md |
| REC-08 | REQ-DISTRIBUTION-001 | distribution-security/spec.md |
| REC-09 | REQ-MCP-001 | distribution-security/spec.md |
| REC-10 | REQ-UPDATE-001 | distribution-security/spec.md |
| REC-11 | REQ-SECRET-001 | distribution-security/spec.md |
| REC-12 | REQ-PLATFORM-001 | platform-installation/spec.md |
| REC-13 | REQ-VERIFY-001 | verification-policy/spec.md |
| REC-14 | REQ-VERIFY-002 | verification-policy/spec.md |
| REC-15 | REQ-INSTALL-001 | platform-installation/spec.md |

## Next-phase gates

Design must resolve trust bootstrap/rotation/version policy, projection placement, delegation-config transaction coverage, reproducible CI/MCP version qualification and residual-duplication metric/threshold. These are not permission to weaken requirements. Structural validation checks IDs/scenario syntax; semantic review separately checks each happy/edge/failure outcome, preservation boundaries and completeness. Neither gate certifies implementation. No production activation or protected older-board write is authorized.
