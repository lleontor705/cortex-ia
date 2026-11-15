# Design — Updater Verification Profiles

## Component boundaries

All new logic lives inside `package updater` at the single manifest-verification seam;
`internal/app/update.go` gains consent parsing and receipt text only.

### Verifier interface (new file `internal/updater/verifier.go`)

```go
type VerificationProfile string // "strict" | "checksum" | "insecure" (reserved)

type ReleaseVerifier interface {
    Name() VerificationProfile
    RequireAuthority() error
    CheckEligibility(current, tag, appliedFloor string) error
    UpdateCandidate(current, tag, appliedFloor string) (bool, error)
    RequiredAssets() []string
    VerifyBundle(rawManifest, rawSig []byte, repo, tag string) (*Manifest, error)
}
```

- `strictVerifier`: `RequireAuthority` = `RequireTrust()`; `CheckEligibility` = the current
  dev-rejection + `VerifyVersionFloor(current, tag, floor)`; `UpdateCandidate` = current
  `checkLatest` candidate logic (`IsDevOrUnknown` → false,nil; else `CheckUpdateCandidate`);
  `RequiredAssets` = ["release-manifest.json", "release-manifest.sig"]; `VerifyBundle` =
  `VerifyManifest`. Every path delegates verbatim — no reimplementation.
- `checksumVerifier` (`verifier_checksum.go`): `RequireAuthority` passes only when consent is
  active, else returns ErrNoTrustedKey; `CheckEligibility` keeps canonical semantics for
  ReleaseBuild, enforces floor-only comparison for DevelopmentBuild (candidate must be
  canonical and strictly greater than a non-empty applied floor), and keeps rejecting
  NonCanonicalBuild; `UpdateCandidate` reports a candidate when dev + empty floor or
  tag > floor; `RequiredAssets` = ["release-manifest.json"]; `VerifyBundle` validates the
  manifest via the extracted `validateManifestShape` and never calls `ed25519.Verify`
  (rawSig may be nil/absent).
- `defaultVerifier()` (defined in `verifier.go`, hardcoded strict until task-utp-003) resolves
  through `profile.go`: bundle present ⇒ strict; absent + consent ⇒ checksum; absent + no
  consent ⇒ strict (so the unchanged ErrNoTrustedKey message remains the fail-closed
  contract); any attempted insecure selection ⇒ error, never returned.

### Seam rewiring (`download.go`, task-utp-001 only)

`downloadAndVerifyReleaseWithFloor` replaces three inline blocks with verifier calls:
(1) line 152 `RequireTrust()` → `v.RequireAuthority()`; (2) lines 155-163 nil/dev rejection +
floor → `v.CheckEligibility(...)` with identical error values; (3) asset-name constant
collection (165-176) and signature fetch + `VerifyManifest` (204-216) →
`v.RequiredAssets()` driven collection with conditional sig fetch and `v.VerifyBundle(...)`.
Artifact download (`DownloadArtifactBytes`) and OS/Arch matching are untouched — shared for
both profiles. `ValidateManifestShape` is extracted out of `VerifyManifest` in `manifest.go`
as an unexported function; `VerifyManifest` becomes shape validation + key lookup +
`ed25519.Verify` with identical errors (oracle: existing `manifest_test.go` passes
unmodified).

### Consent plumbing (no new config file)

Repo conventions: kebab-case flags in the hand-written dispatcher (`--check`,
`--scheduled`) and `CORTEX_IA_*` env vars (`CORTEX_IA_UPDATE_INLINE_CHECK`). `profile.go`
owns `SetChecksumConsent(bool)` + getter behind a mutex (mirrors `trust.go` store pattern) and
`consentFromEnvironment()` reading `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` (values parsed like
`debugEnvEnabled` in `internal/app/app.go`: 1|true vs 0|false, anything else = not given).
`internal/app/update.go` parses `--allow-checksum-updates` in `runUpdate`, calls
`updater.SetChecksumConsent(true)`, and both entry gates (lines 106, 180) call the exported
`updater.RequireProfileAuthority()` wrapper so bundles and consent are resolved before any
network call. Tests use exported test-setters (pattern: `SetTrustedKeysForTesting`).

### Receipt and dev notice (`internal/app/update.go`, task-utp-003)

- `runManualUpdate`/`runScheduledUpdateCheck`: when the active profile is checksum and the
  build class is not ReleaseBuild, replace the hard `printDevelopmentBuildNotice()` early
  return with the checksum-dev informational line and continue; strict behavior unchanged.
- On apply success the final line becomes
  `Successfully updated cortex-ia to %s! (verification: %s)\n` with `defaultVerifier().Name()`.
- Exactly one consent warning line per process when checksum is active:
  `Warning: updates are verified by SHA-256 checksum only (consent: flag|env)`.

## Data models

No persisted state changes: `UpdateState` schema stays version 1 (non-goal). Profile and
consent are process-scoped; the applied floor in existing state remains the anti-replay
authority for dev-checksum runs.

## Trade-offs and rejected alternatives

1. Global consent store vs per-client field: chosen global (mirrors trust store; avoids
   widening `Client` public surface and threading consent through TUI/scheduler callers).
2. Reimplementing manifest validation for checksum vs extraction: chosen extraction of
   `validateManifestShape` inside `manifest.go` — one canonical validator, zero drift risk.
3. Accepting `checksums.txt`: rejected — the asset does not exist in this repo's release
   pipeline; adding it would expand scope into tools/genmanifest (declared non-goal).
4. Config-file consent: rejected — no general config-key store exists for updater policy;
   env + flag is the documented surface and keeps consent per-run by design.
5. `insecure` activation: rejected for this change — the value exists in the type system for
   receipt completeness; selection fails closed (defense in depth in defaultVerifier).
