# Proposal — Updater Migration to GoReleaser checksums.txt (updater-checksums-migration)

## Problem

cortex-ia currently relies on a custom, asymmetric Ed25519 release verification scheme. Before applying an update, the client verifies `release-manifest.json` against an Ed25519 detached signature `release-manifest.sig` using a compiled-in public trust bundle (`productionTrustBundle`) injected via ldflags.

This architecture introduces substantial friction and operational risks:
1. **Secret & Key Management Overhead**: Every release requires private Ed25519 keys (`CORTEX_IA_RELEASE_SIGNING_KEY`), trust bundle configuration (`CORTEX_IA_TRUST_BUNDLE`), and key IDs (`CORTEX_IA_RELEASE_KEY_ID`) in CI. Key rotations require coordinated releases, interval enforcement (`MinVersion`/`MaxVersion`), and dual-key transitions.
2. **Proprietary Tooling**: A dedicated CI-only tool (`tools/genmanifest`) must crawl the release directory, build `release-manifest.json`, sign it, and upload the artifacts to GitHub Releases.
3. **Ecosystem Redundancy**: GoReleaser (the project's release engine) already produces standard SHA-256 `checksums.txt` (`<sha256>  <filename>`) as part of every official build.
4. **Local & Developer Usability**: Any binary built without a packaged trust bundle fails closed (`ErrNoTrustedKey`), requiring manual overrides or preventing seamless auto-update.

## User Value

- **Operational Simplicity**: Removes all Ed25519 private keys, secrets, and trust bundle management from GitHub Actions CI/CD.
- **Standards Alignment**: Adopts the universal Go standard for release verification (`checksums.txt` generated natively by GoReleaser).
- **Reduced Codebase Complexity**: Eliminates over 1,000 lines of custom signature verification, key rotation logic, manifest generation, and legacy tests.
- **Robust Security**: Retains strict HTTPS transport security (`SafeHTTPClient` with pinned domain allowlists), whole-archive SHA-256 verification against the release digest, anti-traversal safety, and anti-downgrade replay protection via `AppliedFloor`.
- **Zero Friction for Operators**: Binaries can seamlessly update from GitHub Releases without requiring runtime or build-time trust bundle injection.

## Approach

1. **Parser & Format (Layer 1)**: Introduce `internal/updater/checksums.go` to parse GoReleaser `checksums.txt` files (`<64-hex sha256>  <filename>`). Enforce strict shape rules: exactly 64 lowercase hex digits, clean relative filenames (rejecting path traversal `..` or slash separators), size caps (`MaxChecksumsSize = 1 MiB`), and duplicate filename detection.
2. **Core Updater & Download (Layer 2)**: Refactor `internal/updater/download.go` and `updater.go` to fetch `checksums.txt` via `SafeHTTPClient` from allowlisted GitHub endpoints, resolve the platform archive (`FindAsset`), verify its SHA-256 digest against `checksums.txt`, and safely extract the binary. Replace the `ReleaseVerifier` / `strictVerifier` / `checksumVerifier` matrix with a unified checksum verification pipeline while preserving anti-downgrade protection (`AppliedFloor`).
3. **CLI & App Surfaces (Layer 3)**: Update `internal/app/update.go` and `internal/tui/update_prompt.go` to operate unconditionally with checksum verification. Maintain `--allow-checksum-updates` as an accepted deprecated no-op flag to ensure backward compatibility with existing automated scripts. Output explicit receipts noting `verification: checksums.txt`.
4. **CI/CD & Legacy Cleanup (Layer 4)**: Clean `.github/workflows/release.yml` by removing signing secrets and `tools/genmanifest` invocation. Remove `productionTrustBundle` from `.goreleaser.yaml`. Delete `tools/genmanifest`, `trust.go`, `manifest.go`, and obsolete Ed25519 test suites. Update reference documentation.

## Non-goals

- No changes to archive packaging formats (`tar.gz` for Linux/macOS, `zip` for Windows) or binary naming templates (`cortex-ia_{version}_{os}_{arch}`).
- No non-HTTPS or non-allowlisted release downloads: `SafeHTTPClient` host allowlisting remains non-negotiable.
- No removal or weakening of `AppliedFloor` anti-downgrade protection.
- No introduction of external GPG, Minisign, or Sigstore/Cosign verification in this migration phase.

## Risks & Mitigations

- **Transport Trust Model Shift**:
  - *Risk*: Moving away from Ed25519 signatures means integrity relies on GitHub Releases transport and account security rather than author signature keys.
  - *Mitigation*: HTTPS transport is strictly enforced with pinned host allowlisting (`github.com`, `api.github.com`, `objects.githubusercontent.com`, `github-releases.githubusercontent.com`, `raw.githubusercontent.com`). Combined with GoReleaser's immutable release assets and cortex-ia's `AppliedFloor` anti-replay floor, this aligns with the prevailing security posture across Go CLI tooling (e.g., gh, fzf, ripgrep).
- **Backward Compatibility for Automation**:
  - *Risk*: Existing scripts passing `--allow-checksum-updates` or setting `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` could fail if the flags are rejected as unknown.
  - *Mitigation*: The flag and environment variable are accepted as deprecated no-ops with informative stdout notes rather than throwing parse errors.
- **Refactoring Regressions**:
  - *Risk*: Changes to download, parsing, or extraction might break update extraction on Windows (zip) or Unix (tar.gz).
  - *Mitigation*: Existing zip and tar.gz safety extractors (`archive.go`, `archive_safety_test.go`) remain untouched. High-coverage modular unit tests (`checksums_test.go`, `download_checksum_test.go`) test valid and corrupted downloads against an in-memory HTTP server.
