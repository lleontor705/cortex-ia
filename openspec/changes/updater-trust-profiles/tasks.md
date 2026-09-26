# Tasks — Updater Verification Profiles

Board: `updater-trust-profiles` | Workflow: sdd-lite | Workload: flexible | Sequential DAG (each node depends on its predecessor).
Full acceptance detail is carried in the work-item contracts; this file is the canonical
requirement-traced checklist.

### Task: task-utp-001 Extract ReleaseVerifier seam with byte-identical strict adapter
- Requirements: REQ-UPD-001
- Depends: none
- Allowed files: internal/updater/verifier.go, internal/updater/manifest.go, internal/updater/download.go
- Objective: Introduce the ReleaseVerifier interface, strictVerifier delegating verbatim to RequireTrust/VerifyVersionFloor/CheckUpdateCandidate/VerifyManifest, defaultVerifier() hardcoded to strict, and rewire downloadAndVerifyReleaseWithFloor's authority, eligibility, required-asset, and bundle-verification blocks through the seam. Extract validateManifestShape from VerifyManifest so the checksum profile can reuse one canonical validator without drift. This is a semantics-preserving refactor: strict behavior, error values, and messages MUST remain identical and are proven by the existing authorized persistent suites running unmodified.
- Acceptance: Given the existing updater suites when go test -count=1 ./internal/updater/... runs then every suite passes with zero edits; Given a release missing release-manifest.sig under strict when the seam resolves RequiredAssets and verifies then the same missing-asset error occurs before any artifact is applied; Given a tampered signature when VerifyBundle runs under strict then ErrInvalidSignature is returned unchanged.
- Verification: go build ./... && go test -count=1 ./internal/updater/... && golangci-lint run ./internal/updater/...

### Task: task-utp-002 Add checksum verifier profile plus profile and consent resolution core with unit tests
- Requirements: REQ-UPD-002, REQ-UPD-003, REQ-UPD-004
- Depends: task-utp-001
- Allowed files: internal/updater/verifier_checksum.go, internal/updater/profile.go, internal/updater/verifier_test.go
- Objective: Implement checksumVerifier (consent-gated RequireAuthority returning ErrNoTrustedKey without consent; validateManifestShape-based VerifyBundle that skips ed25519.Verify and tolerates absent signature bytes; RequiredAssets without release-manifest.sig; CheckEligibility/UpdateCandidate preserving the applied floor for dev builds while still rejecting non-canonical identifiers) and the profile.go core: VerificationProfile constants with insecure reserved-unselectable, ResolveProfile(bundlePresent, consent), mutex-guarded SetChecksumConsent getter/setter, consentFromEnvironment parsing CORTEX_IA_ALLOW_CHECKSUM_UPDATES (1|true), and RequireProfileAuthority. Ship dedicated unit tests in verifier_test.go (max 250 lines) covering both adapters, the resolution table, floor protection, and allowlist-boundary failures; never append suites to files above 300 lines.
- Acceptance: Given bundle absent and consent set when ResolveProfile runs then checksum is selected and RequireAuthority passes; Given bundle present and consent set when ResolveProfile runs then strict is selected; Given bundle absent without consent when RequireProfileAuthority runs then ErrNoTrustedKey is returned verbatim; Given dev current version and applied floor equal to the candidate tag when checksumVerifier.CheckEligibility runs then ErrDowngradeOrReplay results; Given manifest bytes with mismatched repository field when checksumVerifier.VerifyBundle runs then ErrRepositoryMismatch results.
- Verification: go build ./... && go test -count=1 ./internal/updater/... && golangci-lint run ./internal/updater/...

### Task: task-utp-003 Wire profile selection, dev-build routing, consent flag, and profile-named update receipts
- Requirements: REQ-UPD-003, REQ-UPD-004, REQ-UPD-005
- Depends: task-utp-002
- Allowed files: internal/updater/verifier.go, internal/updater/updater.go, internal/app/update.go
- Objective: Replace defaultVerifier()'s hardcoded strict with ResolveProfile-driven selection, convert the remaining authority gates (checkLatest and ApplyUpdateToTarget) to defaultVerifier().RequireAuthority()/CheckEligibility() preserving strict semantics, make checkLatest report an update candidate for dev/unknown current versions only under the checksum profile, and extend internal/app/update.go with the --allow-checksum-updates flag (parsed in runUpdate, documented in printUpdateHelp), SetChecksumConsent wiring, RequireProfileAuthority at both entry gates, the single consent-source warning line, and (verification: <profile>) appended to the apply-success receipt. Keep the strict path and every existing error contract byte-identical; scheduler and TUI callers inherit behavior unchanged.
- Acceptance: Given a bundle-less dev build with --allow-checksum-updates when cortex-ia update --check runs then a checksum consent warning is printed once and the newest release is reported as a candidate; Given the same build without flag or env when any update surface runs then ErrNoTrustedKey is returned and no network fetch occurs; Given a successful apply under checksum when the completion line prints then it includes verification: checksum; Given a strict bundle-bearing release build when update check runs then output and behavior match pre-change behavior apart from the appended profile name on apply success.
- Verification: go build ./... && go test -count=1 ./internal/updater/... ./internal/app && golangci-lint run ./internal/updater/... ./internal/app/...

### Task: task-utp-004 Document updater verification profiles and cross-link release key provisioning
- Requirements: REQ-UPD-005
- Depends: task-utp-003
- Allowed files: docs/updater-verification-profiles.md, docs/configuration.md, docs/release-keys.md
- Objective: Author docs/updater-verification-profiles.md describing strict, checksum, and the reserved-unselectable insecure profile, the fail-closed default, both consent surfaces (--allow-checksum-updates, CORTEX_IA_ALLOW_CHECKSUM_UPDATES), what the checksum profile does and does not protect against, and the dev-build bootstrap flow. Add an Environment Variables and Updates flag entry in docs/configuration.md linking the new page, and add a cross-reference in docs/release-keys.md section 6 pointing operators to the profile documentation for the key-pair provisioning fix. Keep bilingual suffix conventions in mind: links to pages without an ES twin point at the English canonical.
- Acceptance: Given the docs tree when grep checks run then docs/release-keys.md and docs/configuration.md both reference updater-verification-profiles and docs/configuration.md documents allow-checksum-updates; Given the new page when reviewed then all three profile names, the consent variable, and the reserved insecure statement are present.
- Verification: grep -q 'updater-verification-profiles' docs/release-keys.md && grep -q 'updater-verification-profiles' docs/configuration.md && grep -q 'allow-checksum-updates' docs/configuration.md
