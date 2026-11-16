# Updater Verification & Integrity

`cortex-ia update` verifies every downloaded release before extracting or replacing the binary.

There is one verification model. It has two independent parts, and they answer different questions.

| Question | Mechanism | Who can check it |
|---|---|---|
| Are these the bytes the release published? (integrity) | SHA-256 of the archive must match `checksums.txt` from the same GitHub release | The updater, on every update |
| Was this release built by this repository's release workflow? (authenticity) | Sigstore build provenance attested by `.github/workflows/release.yml` | The user, with `gh attestation verify` |

## Integrity (checked by the updater)

- **Digest ledger**: each release publishes GoReleaser's `checksums.txt` (`<sha256: 64 lowercase hex>  <filename>`) next to the archives.
- **Transport**: `checksums.txt` and release archives are downloaded only over HTTPS from the host allowlist (`github.com`, `api.github.com`, `objects.githubusercontent.com`, `release-assets.githubusercontent.com`, `github-releases.githubusercontent.com`, `raw.githubusercontent.com`). Redirects to other hosts or to non-HTTPS origins are rejected. Loopback origins are never accepted by a shipped binary.
- **In-flight digest**: every archive byte stream is hashed and compared with the digest from `checksums.txt`.
- **Anti-downgrade and replay**: `AppliedFloor` persisted in `~/.cortex-ia/update-state.json` rejects equal or older versions.
- **Atomic replacement**: the binary is replaced through a `.tmp.<rand>` file with an automatic `.old` backup restored on error.

### What the checksum does not prove

`checksums.txt` is served from the same release as the archive it lists. Anyone who can replace the archive on that release can also replace `checksums.txt`, so a matching digest shows that the download is the one the release contains. It does not show that the maintainers produced it. The digest check is integrity against the release, not authenticity of the publisher.

## Authenticity (checked by the user)

Each release archive and `checksums.txt` carries a build provenance attestation created in the `goreleaser` job of `.github/workflows/release.yml` with `actions/attest-build-provenance`. The attestation binds the file's SHA-256 to the workflow run, the repository, and the commit that was tagged.

To verify a downloaded archive:

```bash
gh attestation verify cortex-ia_<version>_<os>_<arch>.tar.gz --repo lleontor705/cortex-ia
```

A successful verification confirms that the file was produced by this repository's release workflow. Run it before trusting a release downloaded by hand. The self-updater does not run this check itself.

## Release source

The release job runs `go mod tidy -diff` before GoReleaser builds. If tidying would change `go.mod` or `go.sum`, the release fails, so the shipped binary is built from the tagged source exactly as committed. Release archives contain `README.md`, `LICENSE` and the binary only.

## Legacy Verification Profiles (Historical Reference)

Before version 0.5.0, cortex-ia used Ed25519 signatures (`release-manifest.json` + `release-manifest.sig`) and build-time trust bundles. That was replaced by GoReleaser `checksums.txt` over HTTPS, which needs no signing key custody. The attestation step above restores a publisher-level check without a long-lived key.

The profile selector and its consent gate were removed; no code path resolves a verification profile any more.

`--allow-checksum-updates` and the environment variable `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` are accepted as deprecated no-ops so existing scripts keep working. They change nothing.
