# Spec Delta: release-pipeline (auto-update-pipeline)

## ADDED Requirements

### Requirement: REQ-RP-001 — GoReleaser builds the canonical release matrix
The repository SHALL contain a `.goreleaser.yaml` that builds `./cmd/cortex-ia` for windows/amd64/arm64, linux/amd64/arm64, and darwin/amd64/arm64; injects `-X main.version={{ .Version }}` (per `cmd/cortex-ia/main.go:10`) and the trust-bundle ldflags variable; and archives artifacts as `cortex-ia_{{ .Version }}_{{ .Os }}_{{ .Arch }}` with `.zip` on windows and `.tar.gz` elsewhere — the exact names `updater.FindAsset` and `ValidateAndExtractBinary` (binary name `cortex-ia`/`cortex-ia.exe`) already expect.

#### Scenario: artifact naming matches the updater expectations
- **GIVEN** GoReleaser runs for tag `v0.5.0`
- **WHEN** the windows/amd64 archive is produced
- **THEN** it is named `cortex-ia_0.5.0_windows_amd64.zip` and contains `cortex-ia.exe`

#### Scenario: version is canonical in official builds
- **GIVEN** the release build injects `-X main.version=v0.5.0`
- **WHEN** the built binary reports its version
- **THEN** it is exactly `v0.5.0` and passes `ParseCanonicalVersion`

#### Scenario: non-windows archives use tar.gz
- **GIVEN** GoReleaser runs for tag `v0.5.0`
- **WHEN** the linux/amd64 and darwin/arm64 archives are produced
- **THEN** they are named `cortex-ia_0.5.0_linux_amd64.tar.gz` and `cortex-ia_0.5.0_darwin_arm64.tar.gz`

### Requirement: REQ-RP-002 — Manifest generation and signing reuse the shipped verifier
A release tool (`tools/genmanifest`) SHALL walk the GoReleaser dist output, compute SHA-256 and size per archive, and emit `release-manifest.json` with `schema_version` 1, `repository` `lleontor705/cortex-ia`, `tag`, and the artifact list — byte-compatible with `updater.VerifyManifest` expectations. It SHALL sign the exact manifest bytes with Ed25519 via `updater.SignManifest` using a private key supplied through an environment variable and produce `release-manifest.sig`. The tool SHALL fail closed on missing or short keys, unreadable artifacts, or a non-canonical tag.

#### Scenario: generated manifest verifies with the in-repo verifier
- **GIVEN** fixture archives and a test Ed25519 key pair with the public key injected in the trust store via `SetTrustedKeysForTesting`
- **WHEN** `genmanifest` produces the manifest and signature
- **THEN** `updater.VerifyManifest(rawManifest, rawSig, repo, tag)` succeeds and `FindManifestArtifact` resolves every target

#### Scenario: missing signing key fails closed
- **GIVEN** the signing environment variable is unset
- **WHEN** `genmanifest` runs
- **THEN** it exits non-zero before writing any manifest or signature file

#### Scenario: non-canonical tag fails closed
- **GIVEN** a tag argument of `v0.5.0-rc1`
- **WHEN** `genmanifest` runs with a valid key
- **THEN** it exits non-zero with a typed canonical-version error and writes no artifacts

### Requirement: REQ-RP-003 — Tag-triggered workflow publishes the signed release
A GitHub Actions workflow (`.github/workflows/release.yml`) SHALL trigger on `v*` tags, run the GoReleaser build, generate and sign the manifest with the private key from a GitHub secret, and publish the GitHub release with archives plus `release-manifest.json` and `release-manifest.sig`. The workflow SHALL fail closed when the secret is absent and SHALL NOT echo key material into logs. Local development and test builds SHALL remain valid unsigned builds.

#### Scenario: tag push produces a complete signed release
- **GIVEN** the workflow runs for tag `v0.5.0` with the signing secret configured
- **WHEN** the release is published
- **THEN** its assets include every platform archive plus `release-manifest.json` and `release-manifest.sig`

#### Scenario: absent secret fails the workflow closed
- **GIVEN** the signing secret is not configured
- **WHEN** the workflow reaches the manifest step
- **THEN** the step fails and no release is published

#### Scenario: key material never reaches logs
- **GIVEN** the signing secret is configured
- **WHEN** the workflow executes with standard GitHub Actions log masking
- **THEN** no step prints the private key or trust bundle beyond masked-secret references
