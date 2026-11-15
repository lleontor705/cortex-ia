# Tasks — Updater Migration to GoReleaser checksums.txt

Board: `updater-checksums-migration` | Workflow: `sdd-lite` | Workload: `stacked-layers`
Sequential DAG: `task-chk-001` ──▶ `task-chk-002` ──▶ `task-chk-003` ──▶ `task-chk-004`

---

### Task: task-chk-001 Layer 1: Checksums Parser and Contracts
- **Requirements**: REQ-CHK-001
- **Status**: done (PASS)
- **Allowed Files**:
  - `internal/updater/checksums.go`
  - `internal/updater/checksums_test.go`
- **Objective**:
  Implement a dedicated, robust parser for GoReleaser standard `checksums.txt` files (`<sha256: 64 hex>  <filename>` or `<sha256> *<filename>`). Enforce strict limits and validation: `MaxChecksumsSize` (1 MiB), lowercase 64-hexadecimal character checking, path traversal sanitization (rejection of `/`, `\`, or `..`), duplicate filename rejection, and comment/blank line handling. Define typed errors (`ErrChecksumsTooLarge`, `ErrMalformedChecksumLine`, `ErrMalformedChecksumHash`, `ErrInvalidChecksumFilename`, `ErrDuplicateChecksumArtifact`, `ErrAssetNotFoundInChecksums`, `ErrChecksumsAssetNotFound`). Accompany with comprehensive table-driven unit tests covering valid parsing, corrupt hex, path injection, duplicates, and size boundaries.
- **Verification**:
  `go test -v -count=1 ./internal/updater -run TestParseChecksums`

---

### Task: task-chk-002 Layer 2: Core Updater and Download Pipeline Refactoring
- **Requirements**: REQ-CHK-002, REQ-CHK-003
- **Status**: done (PASS)
- **Allowed Files**:
  - `internal/updater/download.go`
  - `internal/updater/verifier.go`
  - `internal/updater/verifier_checksum.go`
  - `internal/updater/updater.go`
  - `internal/updater/download_checksum_test.go`
- **Objective**:
  Refactor `downloadAndVerifyReleaseWithFloor`, `ApplyUpdateToTarget`, and `checkLatest` to verify downloads using the new `checksums.txt` pipeline instead of `release-manifest.json` and Ed25519 signatures. Replace Ed25519 `RequireTrust()` calls in the updater core with direct candidate eligibility and `AppliedFloor` verification. Locate the `checksums.txt` release asset, download it via `SafeHTTPClient` from allowlisted hosts, parse it using `ParseChecksums`, resolve the platform asset via `FindAsset`, verify the asset's computed SHA-256 against `checksums.txt`, and safely extract the binary. Author modular integration unit tests in `download_checksum_test.go` against a loopback HTTP test server.
- **Verification**:
  `go test -v -count=1 ./internal/updater -run "TestDownload.*|TestVerifier.*|TestUpdateCandidate.*"`

---

### Task: task-chk-003 Layer 3: CLI and Application Surfaces
- **Requirements**: REQ-CHK-004
- **Status**: done (PASS)
- **Allowed Files**:
  - `internal/app/update.go`
  - `internal/app/update_checksum_test.go`
  - `internal/tui/update_prompt.go`
- **Objective**:
  Update `internal/app/update.go` to remove obsolete Ed25519 trust gate checks (`RequireProfileAuthority`, `ErrNoTrustedKey` handling, and trust bundle failure notices). Retain `--allow-checksum-updates` and `CORTEX_IA_ALLOW_CHECKSUM_UPDATES` as accepted deprecated no-op options so existing automation and scripts run without errors. Update help text and documentation in `printUpdateHelp()`. Format the success receipt as `Successfully updated cortex-ia to vX.Y.Z! (verification: checksums.txt)`. Ensure TUI prompt logic in `internal/tui/update_prompt.go` operates seamlessly with the updated engine. Provide focused unit tests in `internal/app/update_checksum_test.go`.
- **Verification**:
  `go test -v -count=1 ./internal/app -run "TestUpdate.*"`

---

### Task: task-chk-004 Layer 4: CI/Workflows, Legacy Tooling Cleanup, and Documentation
- **Requirements**: REQ-CHK-001, REQ-CHK-002, REQ-CHK-004
- **Status**: done (PASS)
- **Allowed Files**:
  - `.github/workflows/release.yml`
  - `.goreleaser.yaml`
  - `docs/reference/updater-verification-profiles.md`
  - `docs/operations/release-keys.md`
- **Objective**:
  Clean up release automation and remove dead Ed25519 code and tooling:
  1. In `.github/workflows/release.yml`: remove `Require release secrets` step, remove `CORTEX_IA_TRUST_BUNDLE` env var from GoReleaser, remove `Generate and sign release manifest` step, and remove `Publish manifest and signature` step.
  2. In `.goreleaser.yaml`: remove ldflags injection for `productionTrustBundle` while keeping `checksum: name_template: "checksums.txt"`.
  3. Delete `tools/genmanifest/` directory (`main.go`, `main_test.go`).
  4. Delete obsolete Ed25519 updater files: `internal/updater/trust.go`, `internal/updater/trust_test.go`, `internal/updater/trust_inject_test.go`, `internal/updater/trust_rotation_test.go`, `internal/updater/manifest.go`, `internal/updater/manifest_test.go`, `internal/updater/authenticated_update_test.go`, `internal/updater/download_auth_test.go`, `internal/app/update_auth_test.go`.
  5. Update documentation in `docs/reference/updater-verification-profiles.md` and `docs/operations/release-keys.md` explaining the transition to standard GoReleaser `checksums.txt` over HTTPS.
- **Verification**:
  `go build ./... && go test -count=1 ./...`
