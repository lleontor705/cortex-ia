# Tasks: auto-update-pipeline

Dependency-ordered DAG materialized on board `auto-update-pipeline` (workflow sdd-full, spec_plane hybrid, workload_policy flexible). Every task respects the <=4-file lease bound and the flexible Go budget (source <=700 LOC, new test files <=250 LOC each). Verification fields are raw executable commands. Forbidden files for ALL tasks (uncommitted sibling deltas): `internal/delegation/spec_contract.go`, `internal/delegation/work.go`, dashboard files, `internal/tui/model.go`, `internal/tui/views.go`, `internal/tui/navigation_test.go`.

### aup-01 — [updater] Trust bundle ldflags injection + key ceremony documentation

- **Requirements:** REQ-AU-001
- **Files:** `internal/updater/trust.go`, `internal/updater/trust_inject_test.go` (new), `docs/release-keys.md` (new)
- **Scope:** `productionTrustBundle` string var (base64 JSON `[]TrustedKey`) decoded at init via `decodeTrustBundle` with `ValidateTrustStore` invariants (<=2 keys, 32-byte Ed25519); empty or malformed bundle keeps the store empty (fail closed); `docs/release-keys.md` documents the ceremony: pair generation, GitHub secret placement, ldflags packaging, rotation with max-2 keys and `MinVersion`/`MaxVersion` intervals, compromise procedure. The ceremony is documented, never executed.
- **Verification:** `go test ./internal/updater -run TestTrustBundle -count=1`
- **Depends on:** none (Wave 1)

### aup-02 — [updater] Update state store + applied-floor enforcement across sessions

- **Requirements:** REQ-AU-002, REQ-AU-003, REQ-AU-007
- **Files:** `internal/updater/state.go` (new), `internal/updater/state_test.go` (new), `internal/updater/download.go`, `internal/updater/updater.go`
- **Scope:** `UpdateState` JSON v1 (schema_version, last_checked_at, available, applied_floor, managed_path, install_candidates), `StatePath`/`LoadUpdateState`/`SaveUpdateStateAtomic` via `filemerge.WriteFileAtomic`, corrupt/missing means empty state; `DetectInstallCandidates` (LOCALAPPDATA Programs path per `scripts/install.ps1:11` + GOPATH bin); `Client.AppliedFloor` field; private `downloadAndVerifyReleaseWithFloor` replacing the hardcoded empty floor (`download.go:152`) with the exported `DownloadAndVerifyRelease` signature unchanged; floor persisted only after a successful apply.
- **Verification:** `go test ./internal/updater -run 'TestUpdateState|TestAppliedFloor' -count=1`
- **Depends on:** none (Wave 1)

### aup-03 — [updater] Strict version symmetry in the check path

- **Requirements:** REQ-AU-004
- **Files:** `internal/updater/updater.go`, `internal/updater/version_symmetry_test.go` (new)
- **Scope:** `CheckLatest` removes the `IsNewer` fallback (`updater.go:89-93`): `CheckUpdateCandidate` errors become typed `ErrNonCanonicalVersion` with `hasUpdate` false; dev/unknown current keeps returning no-update without error; canonical pairs unchanged.
- **Verification:** `go test ./internal/updater -run TestVersionSymmetry -count=1`
- **Depends on:** aup-02 (sequential edit of updater.go)

### aup-04 — [updater] OS scheduler with injectable command runner (check-only task)

- **Requirements:** REQ-AU-005
- **Files:** `internal/updater/scheduler.go` (new), `internal/updater/scheduler_test.go` (new)
- **Scope:** `CommandRunner` seam; `EnableScheduledCheck`/`DisableScheduledCheck`/`ScheduledCheckStatus`; Windows user-level `schtasks /Create /TN "CortexIA Update Check"` with action `"<exec>" update --check --scheduled` (no elevation), POSIX crontab line with the same payload; stable task identifier; idempotent disable; status enabled/missing/error. Tests use a recording stub runner and never touch the real scheduler.
- **Verification:** `go test ./internal/updater -run TestScheduler -count=1`
- **Depends on:** none (Wave 1)

### aup-05 — [delegation] Read-only probes: pre-open schema-drift guard + global authority probe

- **Requirements:** REQ-DP-001, REQ-DP-002
- **Files:** `internal/delegation/schema_guard.go` (new), `internal/delegation/authority_probe.go` (new), `internal/delegation/store.go`, `internal/delegation/delegation_probes_test.go` (new)
- **Scope:** `MaxSupportedSchemaVersion = 15`; `CheckSchemaDriftBeforeOpen(dbPath)` — read-only DSN probe of `MAX(version)` from `schema_migrations` (missing file/table means nil), typed `SchemaDriftError{OnDisk, Supported}` naming `cortex-ia update`; one guarded call inserted in `OpenStore` before `sql.Open` (the in-initialize defense stays); `HasAnyActiveAuthority(ctx, s)` — global unexpired claims/leases or `in_progress` detection usable from a read-only store. Forbidden sibling files are not touched.
- **Verification:** `go test ./internal/delegation -run 'TestSchemaDriftGuard|TestActiveAuthorityProbe' -count=1`
- **Depends on:** none (Wave 1)

### aup-06 — [release] GoReleaser config + GitHub Actions workflow + genmanifest tool

- **Requirements:** REQ-RP-001, REQ-RP-002, REQ-RP-003
- **Files:** `.goreleaser.yaml` (new), `.github/workflows/release.yml` (new), `tools/genmanifest/main.go` (new), `tools/genmanifest/main_test.go` (new)
- **Scope:** `.goreleaser.yaml`: builds `./cmd/cortex-ia` for windows/linux/darwin x amd64/arm64, ldflags `-X main.version={{ .Version }}` + trust bundle var from env, archives `cortex-ia_{{ .Version }}_{{ .Os }}_{{ .Arch }}` (.zip windows / .tar.gz else) matching `FindAsset`; `tools/genmanifest`: walks dist, computes SHA-256+size, emits schema-1 `release-manifest.json` + Ed25519 `release-manifest.sig` via `updater.SignManifest`, key from env, fail-closed on missing key, unreadable artifacts, or non-canonical tag; workflow: tag `v*` trigger, secret-based signing, uploads archives+manifest+sig, fails closed without secrets, never echoes key material.
- **Verification:** `go test ./tools/genmanifest -count=1`
- **Depends on:** aup-01 (trust bundle ldflags contract)

### aup-07 — [app] Update CLI: schedule subcommand, --scheduled mode, state wiring, dual-install warning

- **Requirements:** REQ-AU-005, REQ-AU-006, REQ-AU-007
- **Files:** `internal/app/update.go`, `internal/app/update_schedule.go` (new), `internal/app/update_schedule_test.go` (new)
- **Scope:** `runUpdate` routes the `schedule` prefix to `runUpdateSchedule` (enable/disable/status with concise receipts); `--scheduled` flag means quiet check-only: load state into `Client.AppliedFloor`, `CheckLatest`, `SaveUpdateStateAtomic`, exit 0 with no update; dual-install warning when `DetectInstallCandidates` finds more than one binary; help text updated; no dispatcher changes in app.go.
- **Verification:** `go test ./internal/app -run TestUpdateSchedule -count=1`
- **Depends on:** aup-02, aup-03, aup-04

### aup-08 — [tui] Boot update prompt [y/N] with authority gate, apply, restart notice

- **Requirements:** REQ-UT-001, REQ-UT-002, REQ-UT-003, REQ-UT-004
- **Files:** `internal/tui/update_prompt.go` (new), `internal/tui/update_prompt_test.go` (new), `internal/tui/tui.go`
- **Scope:** `MaybePromptUpdate(home, version, in, out)`: cached `available` triggers the prompt `¿Actualizar a vX.Y.Z? [y/N]` (opt-in inline check via `CORTEX_IA_UPDATE_INLINE_CHECK=1` with a short timeout when no cache); n or EOF continues silently; y evaluates the gate via `delegation.OpenStoreReadOnly` + `HasAnyActiveAuthority` (missing DB means gate open); apply via `updater.Client.ApplyUpdate` with the floor from state, persist new floor + clear available, print the restart notice; typed failure keeps the binary and boots; `SchemaDriftError` from store access renders the `cortex-ia update` guidance. Single call hook in `tui.Run` before `tea.NewProgram`; model.go and views.go untouched.
- **Verification:** `go test ./internal/tui -run TestUpdatePrompt -count=1`
- **Depends on:** aup-02, aup-05

### aup-09 — [docs+gates] Installation docs sync + integral verification gates

- **Requirements:** REQ-AU-006, REQ-UT-001
- **Files:** `docs/installation.md`
- **Scope:** document the new surfaces in the install story: release pipeline assets (manifest+sig), `update schedule`, cached-state TUI prompt behavior, dual-install warning, and a pointer to `docs/release-keys.md`; then run the full integral gate over every touched domain.
- **Verification:** `go test ./internal/... ./tools/... -count=1`
- **Depends on:** aup-01, aup-02, aup-03, aup-04, aup-05, aup-06, aup-07, aup-08

## Integral verification checklist (reviewer / operator)

- `gofmt -s -l ./internal/updater ./internal/app ./internal/tui ./internal/delegation ./tools` must print nothing
- `go vet ./...` must be clean
- `go build ./...` must succeed with zero go.mod runtime additions
- `go test ./internal/... -count=1` must be green
- Scheduler tests executed only against the stub runner; the real Task Scheduler is never touched
- No commits touching the forbidden sibling-delta files listed above
- Manual smoke, operator, official build only: `cortex-ia update schedule status`; declined prompt boots normally
