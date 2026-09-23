# Design: auto-update-pipeline

## 1. Context & Evidence Anchors

- `internal/updater` (obs #243): complete check/verify/replace engine — `CheckLatest`, `DownloadAndVerifyRelease`, `VerifyManifest` (strict JSON, Ed25519, SHA-256+size, HTTPS allowlist, 128 MiB cap), `ReplaceExecutable` (stage → digest verify → rename-away → final digest → rollback), `FindAsset` naming `cortex-ia_{ver}_{os}_{arch}.zip|.tar.gz`, `ValidateAndExtractBinary` (binary `cortex-ia`/`cortex-ia.exe`).
- Fail-closed trust: `ProductionTrustedKeys = []TrustedKey{}` (`trust.go:38`); `SetTrustedKeysForTesting` is the existing test-injection seam; `MaxTrustedKeys = 2` and `TrustedKey{MinVersion,MaxVersion}` already implement rotation semantics (`trust_rotation_test.go`).
- Asymmetry: `updater.go:89-93` `IsNewer` fallback vs strict `ParseCanonicalVersion` at apply (`version.go:37-39`, `download.go:149-154`).
- Floor seam: `download.go:152` `VerifyVersionFloor(currentVersion, rel.TagName, "")` — hardcoded empty floor.
- Schema drift: `delegation/store.go:176-182` fails closed inside `initialize` (too late, no recovery hint); obs #219 documents the real incident and manual recovery.
- Authority gate building blocks: `OpenStoreReadOnly` (`store.go:132`, `query_only`), `HasActiveWorkspaceWork` (`lease_verification.go:177`, workspace-scoped only — a global probe is missing), claim expiry columns on `work_claims`.
- Install reality: `scripts/install.ps1:11` installs to `%LOCALAPPDATA%\Programs\cortex-ia\bin`; obs #243 #4 documents the live binary in user GOPATH `bin`. Release infra gap: no `.goreleaser.yaml`, no `.github/` workflows; `cmd/cortex-ia/main.go:10` documents the `-X main.version` GoReleaser convention.
- TUI boot: `tui.Run(version)` (`tui.go:51`) creates the service and program — the chosen, low-blast-radius hook point (see §7).

## 2. Intent & Non-Goals

Intent: close the loop between a signed release pipeline and the already-hardened updater, with an opt-in, never-silent auto-update experience — check-only headless, apply-only interactive, floor-persistent, drift-aware, and authority-gated.

Non-goals (enforced downstream): no daemon; no plugin hooks; no web-console surface; no silent/unattended apply; no real key ceremony execution; no schema migrations; no new telemetry; no runtime `go.mod` additions; no changes to `internal/tui/model.go`, `views.go`, or `navigation_test.go` (active sibling delta — §7).

## 3. Component Architecture

```
GitHub Actions (tag v*) ──► GoReleaser build (+ldflags: main.version, trust bundle)
        │                          │
        │                          ▼
        │                   tools/genmanifest ──► release-manifest.json + .sig (SignManifest)
        ▼
GitHub Release (archives + manifest + sig)
        │
        ▼
cortex-ia update --check --scheduled   (OS scheduler, headless, CHECK-ONLY)
        │ writes                        ▲ reads
        ▼                               │
~/.cortex-ia/update-state.json ◄── TUI boot prompt [y/N] ──► ApplyUpdate (existing)
        ▲                               │                     │ gate: HasAnyActiveAuthority
        └── applied_floor ◄─────────────┴─────────────────────┘
delegation probes: SchemaGuard (pre-open) · AuthorityProbe (global, read-only)
```

New/changed units and their single responsibilities:

| Unit | Package | Responsibility |
|---|---|---|
| Trust bundle decode | `updater` (trust.go) | ldflags string var → validated `[]TrustedKey` at init; fail-closed default |
| State store | `updater` (state.go, new) | `UpdateState` JSON v1, atomic read/write, floor accessors, install candidates |
| Symmetry check | `updater` (updater.go) | drop `IsNewer` fallback; typed `ErrNonCanonicalVersion` |
| Floor threading | `updater` (download.go, updater.go) | `Client.AppliedFloor` → private `downloadAndVerifyReleaseWithFloor`; exported signature unchanged |
| Scheduler | `updater` (scheduler.go, new) | `Enable/Disable/Status` over injectable runner; schtasks (win32) + crontab (POSIX) |
| Schema guard | `delegation` (schema_guard.go, new) | pre-open read-only `MAX(version)` probe → `SchemaDriftError` |
| Authority probe | `delegation` (authority_probe.go, new) | global unexpired claim/lease or `in_progress` detection |
| Release pipeline | `.goreleaser.yaml`, `.github/workflows/release.yml`, `tools/genmanifest` | build matrix, manifest+sign, tag workflow |
| CLI surface | `app` (update.go, update_schedule.go) | `update schedule enable|disable|status`, `--scheduled` mode, state wiring, dual-install warning |
| TUI prompt | `tui` (update_prompt.go, new; tui.go hook) | cached-state prompt [y/N], gate, apply, restart notice |

## 4. Data Models

```jsonc
// ~/.cortex-ia/update-state.json — written only via filemerge.WriteFileAtomic
{
  "schema_version": 1,
  "last_checked_at": "2026-09-22T12:00:00Z",
  "available": "v0.5.0",
  "applied_floor": "v0.4.50",
  "managed_path": "C:/Users/x/go/bin/cortex-ia.exe",
  "install_candidates": ["C:/Users/x/go/bin/cortex-ia.exe",
                          "C:/Users/x/AppData/Local/Programs/cortex-ia/bin/cortex-ia.exe"]
}
```

Rules: unknown fields ignored on load; corrupt/missing file means empty state + typed soft error flag (never a crash); `available` cleared on successful apply; `applied_floor` only moves forward (strict canonical compare).

`SchemaDriftError{OnDisk int, Supported int}` implements `error` with message `cortex database schema %d is newer than supported schema %d; run 'cortex-ia update' to upgrade cortex-ia`.

## 5. Interface Definitions (Go)

```go
// updater/trust.go (modified)
var productionTrustBundle string // ldflags -X target; base64(JSON []TrustedKey)
func decodeTrustBundle(encoded string) ([]TrustedKey, error) // validates via ValidateTrustStore

// updater/state.go (new)
type UpdateState struct { SchemaVersion int; LastCheckedAt time.Time; Available string;
    AppliedFloor string; ManagedPath string; InstallCandidates []string }
func StatePath(home string) string
func LoadUpdateState(home string) (UpdateState, error)
func SaveUpdateStateAtomic(home string, s UpdateState) error
func DetectInstallCandidates(execPath string) []string

// updater/updater.go (modified)
type Client struct { Repo string; HTTPClient *http.Client; AppliedFloor string }
// CheckLatest: strict candidate comparison; ErrNonCanonicalVersion on non-canonical input

// updater/download.go (modified, private wrapper keeps exported signature)
func downloadAndVerifyReleaseWithFloor(ctx, client, repo, currentVersion, rel, appliedFloor) ([]byte, *ManifestArtifact, error)

// updater/scheduler.go (new)
type CommandRunner interface{ Run(name string, args ...string) ([]byte, error) }
func EnableScheduledCheck(execPath string, r CommandRunner) error
func DisableScheduledCheck(r CommandRunner) error
func ScheduledCheckStatus(r CommandRunner) (ScheduledStatus, error)

// delegation/schema_guard.go (new)
const MaxSupportedSchemaVersion = 15
type SchemaDriftError struct{ OnDisk, Supported int }
func CheckSchemaDriftBeforeOpen(dbPath string) error // ro DSN probe; missing file/table = nil

// delegation/authority_probe.go (new)
func HasAnyActiveAuthority(ctx context.Context, s *Store) (bool, error) // unexpired claims/leases OR in_progress

// app/update_schedule.go (new) + update.go (modified)
func runUpdateSchedule(args []string) error // enable|disable|status
// runUpdate: routes "schedule" prefix; --scheduled = quiet check-only + SaveUpdateStateAtomic

// tui/update_prompt.go (new)
func MaybePromptUpdate(home, version string, in *bufio.Reader, out io.Writer) error
// loads state -> optional opt-in inline check -> prompt [y/N] -> gate -> ApplyUpdate -> notice
```

## 6. Sequence Flows

**Scheduled headless check (check-only):**
1. OS scheduler launches `cortex-ia update --check --scheduled` (user-level, no elevation).
2. `RequireTrust` gate; `CheckLatest` strict-compare (floor loaded from state into `Client.AppliedFloor`).
3. `SaveUpdateStateAtomic`: `last_checked_at`, `available`, install candidates; exit 0. **No download, no prompt, no apply — ever.**

**TUI boot apply path (interactive):**
1. `tui.Run` calls `MaybePromptUpdate` *before* creating the Bubble Tea program.
2. Cached `available` present? (else: opt-in inline check, short timeout, no cached write required).
3. Prompt `¿Actualizar a vX.Y.Z? [y/N]` on stdout/stdin. `n` means boot continues, state preserved.
4. `y` means gate: `delegation.OpenStoreReadOnly` + `HasAnyActiveAuthority`; active means blocked message, state preserved, boot continues.
5. Apply: `updater.New("").ApplyUpdate(ctx, version, release)` — full existing verification + rollback path.
6. Success means persist new floor, clear `available`, print restart notice. Failure means typed error, binary untouched, boot continues.

## 7. Tension Resolution (MANDATORY) & Hook-Point Trade-offs

**Headless scheduler vs interactive apply.** The scheduled task cannot prompt, and silent replacement is forbidden by the locked decision. Resolution: the scheduler's contract is strictly **CHECK-ONLY** (REQ-AU-005/006): verify trust + manifest presence, persist `available` atomically, exit. Confirmation+apply happen exclusively in the next TUI boot reading that cached state (REQ-UT-001), or via the explicit manual CLI. The inline TUI check (no cached state) is opt-in (`CORTEX_IA_UPDATE_INLINE_CHECK=1`) with a short timeout so default boot latency and offline behavior are unchanged. Therefore no code path applies without an interactive `[y/N]` in the same process that applies.

**TUI hook point.** Wiring into `internal/tui/model.go`/`Init` was rejected: that file is part of the **uncommitted usage-stats-panel sibling delta** (archive gate in `delegation/spec_contract.go`/`work.go`, dashboard gofmt are other pre-existing sibling diffs). `tui.Run` (`tui.go:51`) is the minimal, structurally-correct seam (prompt precedes the program), touching neither model.go, views.go, nor navigation_test.go. If a sibling diff later lands in `tui.go`, the implementer must re-base the single call line only.

**store.go touch justification.** `schema_guard` wiring is one guarded call inside `OpenStore` before `sql.Open`; `store.go` is not part of any blocked sibling diff. `spec_contract.go`, `work.go`, and dashboard files are **forbidden** to all tasks of this change.

**Floor threading without breaking existing tests.** `DownloadAndVerifyRelease` keeps its exported signature (used by `download_auth_test.go`, `authenticated_update_test.go`); a private `...WithFloor` variant carries the floor, and `ApplyUpdateToTarget` switches to it.

## 8. Security Invariants (R8 — all preserved)

| Invariant | Status in this change |
|---|---|
| HTTPS host allowlist (`download.go` `allowedHosts`) | untouched; scheduler adds no endpoints |
| Path-traversal / zip-slip guards (`archive.go`) | untouched; genmanifest only reads dist output it created |
| 128 MiB / 1 MiB / 16 KiB size caps | untouched |
| Fail-closed without trust keys (unsigned local builds stay valid) | preserved; bundle empty means `ErrNoTrustedKey` |
| Secrets | private key only as CI secret + `genmanifest` env var; never in repo, logs, ldflags, or state |
| Trust rotation policy | documented max 2 keys, `MinVersion`/`MaxVersion` intervals (`MaxTrustedKeys` reused) |
| No new telemetry with release data | check state contains versions/paths only, stays under `~/.cortex-ia` |
| No admin elevation | `schtasks` user-level; no POSIX sudo |

## 9. Verification Strategy

- Unit oracles per slice (raw commands in tasks.md); updater tests use `httptest` loopback servers (existing allowlist pattern in `download_auth_test.go`) and `SetTrustedKeysForTesting`; no external network.
- Scheduler tests inject a stub `CommandRunner` that records argv — the real Task Scheduler/crontab is never touched.
- `genmanifest` round-trips through the real `VerifyManifest` + `FindManifestArtifact` with a test key.
- Delegation probes tested on temp-home SQLite fixtures (authorized authority domain per AGENTS.md).
- YAML/GitHub-Actions files are declarative configs: validated structurally by review and CI itself (no ad-hoc lexers per repo policy); the machine oracle for the pipeline slice is the `genmanifest` test suite.
- Final integral gate: full `go test ./internal/... -count=1` plus gofmt/vet (task aup-09).

## 10. Release Ceremony (documentation only — never executed here)

`docs/release-keys.md` (task aup-01) specifies: generate an Ed25519 pair (offline helper or equivalent tool); private key (64-byte seed, base64) goes **only** into the GitHub Actions secret named in the workflow; public key enters the release build as the base64 ldflags trust bundle with `Repository: lleontor705/cortex-ia` and version intervals; rotation = add the new key as a second record with `MinVersion` = first release signed by it, then constrain the old key with `MaxVersion` = last release it signs (`MaxTrustedKeys = 2` is the hard ceiling); cadence guidance and compromise procedure (ship a build with the compromised key's `MaxVersion` capped, then drop it). The doc is a plan document; executing it is out of scope for this change.
