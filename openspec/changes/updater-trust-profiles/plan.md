# Plan — Updater Verification Profiles (sdd-lite integrated)

Change: updater-trust-profiles | Workflow: sdd-lite | Spec plane: hybrid (OpenSpec markdown + Cortex evidence)

## 1. Intent

Add explicit, consent-gated verification profiles to the self-update pipeline so that
bundle-less local builds can self-update via a SHA-256 + pinned-HTTPS integrity path while
production (bundle-bearing) builds keep the byte-identical Ed25519 strict path. Detailed
Given/When/Then requirements live in `specs/updater/spec.md`; component design in
`design.md`; the executable DAG in `tasks.md`.

Non-goals (binding, prevent cascade amplification): no changes to `tools/genmanifest` or
release CI; no `checksums.txt` asset support; no `UpdateState` schema change; no TUI edits;
no activation path for `insecure`; no config-file persistence of consent; no new exported
API beyond the seam types.

## 2. Requirements

Machine-readable requirements with at least three Given/When/Then scenarios each are defined
in `specs/updater/spec.md` under ADDED Requirements:

- REQ-UPD-001 — ReleaseVerifier seam extraction, strict semantics byte-identical.
- REQ-UPD-002 — Checksum profile verification semantics (HTTPS pinning, SHA-256/size, repo/tag binding, no ed25519).
- REQ-UPD-003 — Explicit consent gate for checksum; strict never downgraded; insecure unselectable.
- REQ-UPD-004 — Dev-build routing under checksum consent with applied-floor protection.
- REQ-UPD-005 — Update receipt names the active profile; user-facing documentation.

## 3. Design

Single seam at `internal/updater/download.go` (currently lines 152-216) resolves to
`defaultVerifier()`: a package-level selector returning a `ReleaseVerifier` implementing
`Name/RequireAuthority/CheckEligibility/UpdateCandidate/RequiredAssets/VerifyBundle`.
`strictVerifier` delegates verbatim to `RequireTrust`, `VerifyVersionFloor`,
`CheckUpdateCandidate`, and `VerifyManifest` (whose shape validation is extracted into a
shared unexported `validateManifestShape`). `checksumVerifier` gates on consent, treats
`release-manifest.sig` as optional, verifies the manifest with `validateManifestShape`
only, and enforces the persisted applied floor for dev builds. Selection:
bundle present ⇒ strict (consent ignored); bundle absent + consent ⇒ checksum; bundle
absent without consent ⇒ existing `ErrNoTrustedKey` fail-closed. Consent arrives via
`--allow-checksum-updates` (set by `internal/app/update.go` before any gate) or
`CORTEX_IA_ALLOW_CHECKSUM_UPDATES=1|true`. Full interface, flows, and trade-offs in
`design.md`.

## 4. Tasks

Sequential chain in board `updater-trust-profiles` (disjoint-by-construction, CAS-safe):

- [ ] task-utp-001 — Extract ReleaseVerifier seam with byte-identical strict adapter.
  Requirements: REQ-UPD-001
- [ ] task-utp-002 — Add checksum verifier profile plus profile/consent resolution core and unit tests.
  Requirements: REQ-UPD-002, REQ-UPD-003
- [ ] task-utp-003 — Wire profile selection, dev-build routing, consent flag, profile-named receipts.
  Requirements: REQ-UPD-003, REQ-UPD-004, REQ-UPD-005
- [ ] task-utp-004 — Document verification profiles and cross-link release-key provisioning docs.
  Requirements: REQ-UPD-005

Verification gates per task are the raw commands recorded in `tasks.md` and the work-item
`verification` fields (`go build ./...`, `go test -count=1 ./internal/updater/...`,
`golangci-lint run ./internal/updater/...`). Persistent-test scope is the repository's
authorized `internal/updater/...` allowance; new suites go in dedicated test files of at
most 250 lines and are never appended to `updater_test.go` (>300 lines).
