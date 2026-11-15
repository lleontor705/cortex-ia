# Updater Verification & Integrity

`cortex-ia update` verifies every downloaded release before extracting or replacing the binary.

Integrity verification follows the standard Go ecosystem pattern:
- **Standard Digest Ledger**: Releases publish a standard GoReleaser `checksums.txt` file containing SHA-256 digests (`<sha256: 64 lowercase hex>  <filename>`).
- **Transport Security**: `checksums.txt` and release archives are downloaded exclusively via HTTPS with a strict host allowlist (`github.com`, `api.github.com`, `objects.githubusercontent.com`, `github-releases.githubusercontent.com`, `raw.githubusercontent.com`). Any redirect to an untrusted domain or non-HTTPS scheme is rejected.
- **In-flight Integrity**: Every downloaded artifact byte sequence is hashed and verified against the expected digest from `checksums.txt`.
- **Anti-Downgrade & Replay**: `AppliedFloor` persisted in `~/.cortex-ia/update-state.json` prevents installing or rolling back to equal or older versions.
- **Atomic Rollback**: Binary replacements are performed atomically (`.tmp.<rand>`) with an automatic `.old` backup restoration in case of unexpected errors.

## Legacy Verification Profiles (Historical Reference)

Prior to version 0.5.0, cortex-ia used asymmetric Ed25519 signatures (`release-manifest.json` + `release-manifest.sig`) and required build-time trust bundles. This was replaced by standard GoReleaser `checksums.txt` over HTTPS to remove private key custody overhead and make releases completely automated without manual signing keys.

The flag `--allow-checksum-updates` and environment variable `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` are retained as deprecated no-ops for backward compatibility with existing scripts.
