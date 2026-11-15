# Release Signing Keys (Retired)

> **Notice**: Asymmetric Ed25519 signing keys and build-time trust bundles have been retired in cortex-ia.
> The updater now verifies integrity using standard GoReleaser `checksums.txt` over pinned HTTPS connections.

## Historical Summary

Previously, cortex-ia releases required:
1. An offline Ed25519 private key stored in GitHub Secrets (`CORTEX_IA_RELEASE_SIGNING_KEY`).
2. A trust bundle injected into GoReleaser ldflags (`productionTrustBundle`).
3. An internal tool (`tools/genmanifest`) that generated `release-manifest.json` and `release-manifest.sig`.

This required complex offline key ceremony procedures, multi-version rotation intervals, and dual-key transitions.

## Current Architecture

1. **Automated Releases**: GoReleaser natively generates `checksums.txt` during the release workflow without custom tooling or secrets.
2. **Deterministic Verification**: The self-update engine verifies the SHA-256 digest of the downloaded archive against `checksums.txt` fetched over pinned TLS endpoints.
3. **No Private Key Custody**: There are no private signing keys to rotate, back up, or risk losing in CI.
