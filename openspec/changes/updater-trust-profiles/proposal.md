# Proposal — Updater Verification Profiles (updater-trust-profiles)

## Problem

cortex-ia's self-update pipeline is fail-closed on trust: every check/apply gate calls
`RequireTrust()` (`internal/updater/updater.go:107,321`, `internal/updater/download.go:152`,
`internal/app/update.go:106,180`) and the single manifest-verification seam
(`internal/updater/download.go:213`, `VerifyManifest`) requires an Ed25519-signed
`release-manifest.json` + `release-manifest.sig`. Locally built binaries (no ldflags trust
bundle) can therefore never self-update, even to install an official release, and developers
must manually download from GitHub Releases.

## User value

- Dev/local builds can bootstrap onto the official release train with a bounded-integrity
  path (HTTPS host allowlist + per-artifact SHA-256/size + anti-replay floor) without ever
  weakening packaged production builds.
- The active verification profile becomes an explicit, named, observable property of every
  update receipt instead of an implicit "signature verified" assumption.

## Approach

Introduce `VerificationProfile ∈ {strict, checksum, insecure}` resolved at the single
manifest-verification seam behind the existing `RequireTrust` gates:

- **strict** — current Ed25519 path, extracted behind a `ReleaseVerifier` interface;
  semantics byte-identical; selected whenever the trust bundle is present; never weakened.
- **checksum** — fetch `release-manifest.json` over the pinned-HTTPS `SafeHTTPClient`
  (host allowlist `download.go:26-32`), skip only `ed25519.Verify` and the
  `release-manifest.sig` asset requirement; keep repo/tag binding, schema/artifact-table
  validation, per-artifact SHA-256 + size checks (`download.go:133-137,236`), and the
  anti-replay floor (`version.go:147-176`). Selected only when the bundle is absent AND
  explicit consent is given via `--allow-checksum-updates` or
  `CORTEX_IA_ALLOW_CHECKSUM_UPDATES`.
- **insecure** — reserved enum value only. No selector, flag, or code path in this change
  activates it; any resolution attempt fails closed.

## Non-goals

- No changes to release signing tooling (`tools/genmanifest`), CI release workflows, or the
  trust-bundle format; no new `checksums.txt` release asset (it does not exist in this
  repository's pipeline — the structured `release-manifest.json` is the sole source).
- No `UpdateState` JSON schema change; consent and profile are surfaced in stdout receipts
  only.
- No TUI surface changes; TUI update prompts inherit profile behavior through the updater
  gates.
- No activation path for `insecure`; no config-file key (the repository's established
  consent surface is CLI flags + `CORTEX_IA_*` environment variables).

## Risks

- **Integrity dilution**: a stripped-down signed release plus manifest would pass checksum
  verification. Accepted: threat model is transport compromise/CDN misdelivery, not a
  malicious-but-valid GitHub release; strict remains the only profile for bundle-bearing
  builds; consent is explicit and receipt-logged.
- **Refactor drift**: seam extraction could silently alter strict semantics. Mitigated by a
  semantics-preserving gate task whose oracle is the existing authorized persistent updater
  suites (`manifest_test.go`, `trust_test.go`, `download_auth_test.go`,
  `authenticated_update_test.go`) passing unmodified.
- **Dev-build floor semantics**: relaxing the "current version must parse" precondition only
  for the checksum profile; the persisted applied floor is still enforced when present.
