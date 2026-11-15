# Spec delta — updater verification profiles

Domain: updater | Plane: hybrid | Change: updater-trust-profiles

## ADDED Requirements

### Requirement: REQ-UPD-001 ReleaseVerifier seam preserves strict semantics byte-identically
The manifest-verification path SHALL route exclusively through a `ReleaseVerifier` seam, and the
strict implementation MUST exhibit behavior identical to the current `RequireTrust` +
`VerifyManifest` + `VerifyVersionFloor` chain for every input, including error values and
messages. `DownloadAndVerifyRelease` and `downloadAndVerifyReleaseWithFloor` MUST retain their
current signatures and observable outcomes.

#### Scenario: Strict adapter verifies a signed release through the seam
- **GIVEN** a packaged trust bundle with a trusted key matching repo and tag and a valid signed manifest pair
- **WHEN** the download seam verifies the release via the active strict verifier
- **THEN** the manifest parses, schema/repo/tag/artifact-table validation and ed25519.Verify succeed exactly as the pre-change code path does

#### Scenario: Strict adapter rejects a tampered signature
- **GIVEN** a manifest whose bytes differ from the signed envelope content while the trust bundle is present
- **WHEN** the strict verifier checks the bundle
- **THEN** verification fails with ErrInvalidSignature and no artifact bytes are returned

#### Scenario: Strict adapter keeps rejecting releases without a signature asset
- **GIVEN** a release carrying release-manifest.json but no release-manifest.sig asset under the strict profile
- **WHEN** the seam resolves the verifier's required asset set
- **THEN** verification fails with the same missing-asset error the current code returns before any manifest fetch

#### Scenario: Refactor oracle suites pass unmodified
- **GIVEN** the existing persistent suites manifest_test.go, trust_test.go, download_auth_test.go and authenticated_update_test.go
- **WHEN** the seam extraction task is verified with go test -count=1 ./internal/updater/...
- **THEN** all suites pass with zero assertions modified or deleted

### Requirement: REQ-UPD-002 Checksum profile enforces transport and integrity checks without signature verification
The checksum verifier SHALL validate `release-manifest.json` fetched only through
`SafeHTTPClient` pinned-HTTPS host allowlisting, MUST apply `validateManifestShape`
(schema version, repository/tag binding, artifact table, lowercase-hex SHA-256, size bounds)
and MUST skip only `ed25519.Verify` and signature-envelope fetching, while the
per-artifact SHA-256 and size checks of `DownloadArtifactBytes` remain mandatory for every
profile.

#### Scenario: Checksum verifier accepts a well-formed manifest without signature
- **GIVEN** a release with a valid release-manifest.json asset and consent active under the checksum profile
- **WHEN** the checksum verifier validates the fetched manifest bytes
- **THEN** schema, repo, tag, and artifact-table checks pass and no signature envelope is required or fetched

#### Scenario: Checksum verifier rejects repository binding mismatch
- **GIVEN** a manifest whose repository field differs from the client repo under the checksum profile
- **WHEN** the checksum verifier validates the manifest
- **THEN** verification fails with ErrRepositoryMismatch

#### Scenario: Checksum verifier rejects digest mismatch on artifact download
- **GIVEN** a checksum-verified manifest describing an artifact with SHA-256 X while the served bytes hash to Y
- **WHEN** the pipeline downloads the matched release asset
- **THEN** DownloadArtifactBytes fails with ErrDigestMismatch and nothing is applied

#### Scenario: Checksum fetch is confined to the allowlisted HTTPS hosts
- **GIVEN** a manifest or asset download URL pointing to a non-allowlisted host
- **WHEN** the checksum profile fetches through SafeHTTPClient
- **THEN** the request fails with ErrDisallowedOrigin or ErrInsecureScheme before any body is read

### Requirement: REQ-UPD-003 Profile selection is consent-gated and strict is never downgraded
Profile resolution SHALL select strict whenever the trust bundle is present regardless of
consent; SHALL select checksum only when the bundle is absent and explicit consent is given
via the `--allow-checksum-updates` flag or `CORTEX_IA_ALLOW_CHECKSUM_UPDATES=1|true`; and
MUST keep the existing ErrNoTrustedKey fail-closed behavior when the bundle is absent without
consent. The insecure profile SHALL remain unselectable: no flag, variable, or file may cause
resolution to return it.

#### Scenario: Bundle present overrides consent toward strict
- **GIVEN** a build with a packaged trust bundle and checksum consent explicitly granted
- **WHEN** the profile resolver runs
- **THEN** it selects strict and consent has no effect on verification behavior

#### Scenario: Bundle absent with consent selects checksum
- **GIVEN** a bundle-less build and CORTEX_IA_ALLOW_CHECKSUM_UPDATES set to true
- **WHEN** the profile resolver runs
- **THEN** it selects checksum and the authority gate passes

#### Scenario: Bundle absent without consent preserves fail-closed behavior
- **GIVEN** a bundle-less build with no flag and no environment consent variable set
- **WHEN** any update gate evaluates authority
- **THEN** it returns ErrNoTrustedKey verbatim and no network call is made

#### Scenario: Insecure resolution fails closed
- **GIVEN** any request or configuration attempting the insecure profile
- **WHEN** profile resolution is invoked
- **THEN** resolution never yields insecure and the pipeline remains on strict or checksum or the fail-closed error

### Requirement: REQ-UPD-004 Dev builds route to the checksum path with applied-floor protection preserved
Under the checksum profile, development builds (empty, dev, unknown, or development version
identifiers per IsDevOrUnknown) SHALL be allowed to check and apply a strictly newer official
release; the persisted applied floor MUST still reject candidates at or below it;
non-canonical identifiers other than dev/unknown (for example git-describe strings) MUST
remain rejected; and the strict profile's existing dev-build rejections MUST NOT change.

#### Scenario: Dev build with consent sees an update candidate
- **GIVEN** a dev build with no applied floor and checksum consent granted
- **WHEN** checkLatest evaluates the latest release tag
- **THEN** an update candidate is reported instead of the current silent "no update" behavior

#### Scenario: Applied floor blocks replay onto a dev build
- **GIVEN** a dev build with checksum consent whose persisted applied floor equals the candidate tag
- **WHEN** eligibility or apply runs through the checksum verifier
- **THEN** it fails with ErrDowngradeOrReplay and no bytes are downloaded

#### Scenario: Strict profile still rejects dev builds unchanged
- **GIVEN** a bundle-bearing strict build reporting a dev current version
- **WHEN** any apply or download eligibility check runs
- **THEN** it fails with ErrDevUnknownVersion exactly as before the change

#### Scenario: Non-canonical git-describe builds stay excluded from checksum routing
- **GIVEN** a bundle-less build with a git-describe identifier and checksum consent granted
- **WHEN** the checksum verifier checks eligibility for the candidate tag
- **THEN** the candidate is rejected because only ReleaseBuild and DevelopmentBuild classes participate

### Requirement: REQ-UPD-005 Update receipts name the active profile and documentation covers every consent surface
Every completed or applied update check under the app surfaces SHALL print the active
verification profile in its receipt line, checksum activations SHALL additionally print a
single consent warning naming the consent source, and user-facing documentation SHALL describe
strict, checksum, insecure (reserved, unselectable), the flag, the variable, and cross-link
the key-provisioning procedure in docs/release-keys.md section 6.

#### Scenario: Apply success receipt names the profile
- **GIVEN** a successful self-update run under any profile
- **WHEN** the success line is printed by internal/app/update.go
- **THEN** the line includes verification: strict or verification: checksum matching the active verifier name

#### Scenario: Checksum activation logs a consent warning with its source
- **GIVEN** a bundle-less build activating checksum through the command-line flag
- **WHEN** an update check or apply begins under the checksum profile
- **THEN** exactly one warning line identifies the checksum profile and the flag as the consent source

#### Scenario: Documentation enumerates profiles and links release key provisioning
- **GIVEN** the merged documentation set for this change
- **WHEN** an operator reads docs/updater-verification-profiles.md with cross-links from docs/configuration.md and docs/release-keys.md
- **THEN** all three profiles, both consent surfaces, the fail-closed default, and the docs/release-keys.md section 6 bundle-provisioning link are present
