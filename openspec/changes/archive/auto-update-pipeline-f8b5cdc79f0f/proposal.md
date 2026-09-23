# Proposal: auto-update-pipeline

## Why

Cortex observations #243 (topic `architecture/updater-auto-update`, follows #219) mapped `internal/updater` and found the binary is a fail-closed but **orphaned** update engine:

1. **Fails closed with no packaging path**: `ProductionTrustedKeys` is empty (`trust.go:38`), so `RequireTrust` returns `ErrNoTrustedKey` for every check/apply; the repo has **no** `.goreleaser.yaml` and **no** `.github/` workflows to ever package keys or publish signed manifests.
2. **Non-canonical version asymmetry (obs #243 Q6b)**: `CheckLatest` (`updater.go:89-93`) falls back to the lax `IsNewer` when `CheckUpdateCandidate` errors, so a build like `v0.4.50+dirty` is *announced* an update at check while apply strictly rejects it (`version.go:37-39` suffix ban) — asymmetric UX between check and apply.
3. **Anti-replay floor never persists**: `DownloadAndVerifyRelease` calls `VerifyVersionFloor(currentVersion, rel.TagName, "")` with a hardcoded empty floor (`download.go:152`), so the applied-version floor dies with the process and downgrade/replay between sessions is undetected.
4. **Schema-drift incident recurrence risk (obs #219)**: a stale PATH binary fails closed too late, inside `Store.initialize` (`delegation/store.go:176-182`), after the DB handle is open, with a message that names no recovery path; the observed recovery was a manual rebuild+swap.
5. **No trigger surface**: the only consumer is the manual CLI `cortex-ia update` (`internal/app/update.go`); no OS scheduler, no cached "update available" state, no TUI prompt, and no guard preventing an update from landing while work claims/leases are active.
6. **Dual-install reality (obs #243 #4, scripts/install.ps1:11)**: the live binary sits in user-writable `GOPATH/bin` while the supported installer targets `%LOCALAPPDATA%\Programs\cortex-ia\bin`; updates apply over `os.Executable()` with no visibility into the second copy.

The user intent is the complete signed-release pipeline plus a safe, opt-in, never-silent auto-update experience on Windows-first surfaces.

## What Changes

- **Trust store injection (deferred ceremony)**: `internal/updater/trust.go` gains an ldflags-injectable production trust bundle (encoded string var decoded and validated at init; `MaxTrustedKeys`/32-byte Ed25519 invariants and `ValidateTrustStore` reused). Default stays **fail-closed** (`ErrNoTrustedKey`) when unset; tests keep using `SetTrustedKeysForTesting`. The key ceremony itself is only *documented*, never executed.
- **Release pipeline**: `.goreleaser.yaml` (windows/linux/darwin archives named `cortex-ia_{version}_{os}_{arch}.zip|.tar.gz` matching `FindAsset`), `.github/workflows/release.yml` (tag-triggered, `-X main.version={{ .Version }}` per `cmd/cortex-ia/main.go:10`, trust bundle injected from a CI secret), and a new `tools/genmanifest` CLI that builds `release-manifest.json` (schema_version=1, SHA-256+size per artifact — the exact format `manifest.go` already validates) and signs it Ed25519 via the existing `SignManifest`.
- **Update state store**: `internal/updater/state.go` persists `~/.cortex-ia/update-state.json` atomically (`filemerge.WriteFileAtomic`): schema version, last checked, available release, applied floor, managed install path, dual-install candidates.
- **Applied-floor enforcement (R2)**: `Client.AppliedFloor` threads the persisted floor through `ApplyUpdateToTarget` → `VerifyVersionFloor` (replacing the hardcoded `""` at `download.go:152`) via a private wrapper that keeps the exported `DownloadAndVerifyRelease` signature stable for existing tests.
- **Strict version symmetry (R1)**: `CheckLatest` drops the `IsNewer` fallback; non-canonical current or candidate versions return a typed error so check and apply agree. Auto-update remains opt-in for official builds only.
- **OS scheduler (R5)**: `cortex-ia update schedule enable|disable|status` registers/removes/inspects a **user-level** scheduled task (Windows Task Scheduler via `schtasks`, POSIX crontab secondary) that runs a new headless `cortex-ia update --check --scheduled` mode: quiet, check-only, atomic cached-state write, never applies.
- **Delegation probes (R3, R6 gate)**: read-only pre-open schema-drift guard (typed error naming `cortex-ia update`) wired into `OpenStore`, plus a global active-authority probe (any live claim/lease or in-progress task) used as the hard apply gate.
- **TUI surface (R6)**: at TUI boot, if cached state says an update is available (or an opt-in inline check with a short timeout finds one), prompt `¿Actualizar a vX.Y.Z? [y/N]`; on confirmation run the existing `ApplyUpdate` flow (download+verify+rename-replace+rollback in `replacement.go`) and print the restart notice; the claims gate blocks apply when authority is active. Never a silent replacement.

## Capabilities

- `specs/updater/spec.md` — ADDED: REQ-AU-001 trust bundle injection fail-closed, REQ-AU-002 update state persistence, REQ-AU-003 applied-floor anti-replay, REQ-AU-004 strict version symmetry, REQ-AU-005 scheduler registration, REQ-AU-006 scheduled check-only mode, REQ-AU-007 dual-install detection.
- `specs/delegation/spec.md` — ADDED: REQ-DP-001 pre-open schema-drift guard, REQ-DP-002 global active-authority probe.
- `specs/release-pipeline/spec.md` — ADDED: REQ-RP-001 GoReleaser multi-OS build, REQ-RP-002 signed manifest generation, REQ-RP-003 GitHub Actions release workflow.
- `specs/update-tui/spec.md` — ADDED: REQ-UT-001 interactive prompt, REQ-UT-002 claims hard gate, REQ-UT-003 apply + restart notice, REQ-UT-004 schema-drift recovery guidance.

## Impact

- **Code (new):** `internal/updater/state.go`, `internal/updater/scheduler.go`, `internal/delegation/schema_guard.go`, `internal/delegation/authority_probe.go`, `internal/app/update_schedule.go`, `internal/tui/update_prompt.go`, `tools/genmanifest/main.go`, plus bounded test files (`<=250` LOC each) in authorized domains (updater, app update tests, TUI, delegation authority domain per AGENTS.md).
- **Code (modified, minimal):** `internal/updater/trust.go` (bundle var + decode), `internal/updater/download.go` + `internal/updater/updater.go` (floor threading + strict check), `internal/delegation/store.go` (one guard call pre-open), `internal/app/update.go` (schedule routing + scheduled mode + state wiring), `internal/tui/tui.go` (single prompt hook in `Run`).
- **CI/Release (new):** `.goreleaser.yaml`, `.github/workflows/release.yml`, `tools/genmanifest/`.
- **Docs (new):** `docs/release-keys.md` — key ceremony documentation only (generation, secret location, rotation with max-2 keys and version intervals, pipeline consumption).
- **Dependencies:** zero new runtime `go.mod` entries; CI may add release-only actions/tools. Ed25519, base64, encoding/json are stdlib; `filemerge.WriteFileAtomic` already exists.
- **Data:** one new JSON state file `~/.cortex-ia/update-state.json`; read-only SELECT against `~/.cortex-ia/delegation.db` for probes; no delegation schema changes.

## Non-Goals

- No product code is written by this change proposal itself: it specifies and materializes the DAG only.
- No key ceremony execution: no real key pair generation, no real GitHub secrets, no signed artifacts.
- No real scheduled-task registration during planning or tests: scheduler exercises only an injectable command runner with stub/dry-run binaries.
- No resident daemon, no OpenCode plugin hooks, no web-console (cortexiaweb) update surface.
- No silent or unattended apply: every apply is interactive-confirm inside the TUI or explicit CLI.
- No release-signing implementation inside this change: only the pipeline that consumes the CI secret.
- No delegation schema migration: the schema-drift guard reads, never writes or migrates.
- No new telemetry and no network calls from unit tests (loopback httptest only, already allowlisted).
