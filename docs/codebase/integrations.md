# Integrations & CI/CD

← [Codebase Guide](../CODEBASE-GUIDE.md)

External tooling integrations: release pipeline, CI workflows, installer script, and package distribution. This page covers how cortex-ia is built, tested in CI, and distributed — it does not cover the Go internals (see [repository-map.md](repository-map.md)) or the release runbook steps (see [maintainer-playbook.md](maintainer-playbook.md)).

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| GoReleaser config | ✅ | — |
| GitHub Actions workflows | ✅ | — |
| install.sh curl-pipe installer | ✅ | — |
| Homebrew tap distribution | ✅ | — |
| Makefile targets | ✅ | — |
| Release runbook (step-by-step) | — | [maintainer-playbook.md](maintainer-playbook.md) |
| Go package internals | — | [repository-map.md](repository-map.md) |

## Integrations Overview

| Integration | Config File | Trigger | Output |
|------------|-------------|---------|--------|
| GoReleaser | `.goreleaser.yaml` | Git tag push | Cross-platform binaries + archives + Homebrew formula update |
| CI (quality + security) | `.github/workflows/ci.yml` | Push, PR | Lint, test, security scan results |
| Release | `.github/workflows/release.yml` | Tag push | Published release artifacts |
| PR Check | `.github/workflows/pr-check.yml` | PR opened/edited | Pass/fail on issue ref, labels, branch name |
| Stale | `.github/workflows/stale.yml` | Schedule (daily) | Stale/closed issue & PR labels |
| Installer | `scripts/install.sh` | User curl-pipe | Binary download + SHA-256 verify + install |
| Homebrew | `lleontor705/homebrew-tap` | GoReleaser publish | `brew install` formula |

## GoReleaser

`.goreleaser.yaml` drives cross-platform binary builds and distribution.

| Aspect | Detail |
|--------|--------|
| Build targets | `linux`/`darwin`/`windows` × `amd64`/`arm64` |
| Archives | Per-OS tar.gz / zip |
| Homebrew tap | `lleontor705/homebrew-tap` — formula auto-updated on release |
| Changelog | Filters configured (e.g., exclude merge commits, dependency bots) |
| Checksums | Generated per release |
| Signing | Per `.goreleaser.yaml` config |

## GitHub Actions Workflows

### ci.yml — Quality & Security

| Step | Tool | Purpose |
|------|------|---------|
| Lint | golangci-lint | Static analysis (errcheck, govet, staticcheck, unused, ineffassign) |
| Test | `go test` | Unit + integration test suite |
| Security | security scanner | Dependency / vulnerability scan |

**Trigger**: push and pull request.

### release.yml — Release Pipeline

```
tag push → test → lint → manual approve → goreleaser publish
```

| Step | Gate |
|------|------|
| 1. Test | Must pass |
| 2. Lint | Must pass |
| 3. Approve | Manual approval gate |
| 4. GoReleaser | Builds + publishes artifacts + updates Homebrew tap |

**Trigger**: tag push (e.g., `v1.2.3`).

### pr-check.yml — PR Compliance

Enforces contribution rules on every PR.

| Check | Rule |
|-------|------|
| Issue reference | PR description or branch must reference a GitHub issue |
| Approved label | Requires `approved` label before merge |
| Type label | Requires a type label (e.g., `feature`, `bug`, `docs`) |
| Branch name | Must follow naming convention |

**Trigger**: PR opened or edited.

### stale.yml — Lifecycle Management

| Action | Threshold |
|--------|-----------|
| Mark stale | 30 days of inactivity |
| Close stale | 14 days after stale label (total 44 days) |

**Trigger**: scheduled (daily).

## Installer Script

`scripts/install.sh` is a curl-pipe installer for Unix systems.

| Step | Detail |
|------|--------|
| Detect OS/arch | uname-based platform detection |
| Download binary | Fetches release asset for detected platform |
| Verify checksum | SHA-256 checksum validation against published checksums |
| Install | Moves binary to install path (e.g., `/usr/local/bin`) |

```bash
curl -fsSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

| Platform | Installer | Status |
|----------|-----------|--------|
| Linux / macOS | `install.sh` | ✅ Available |
| Windows | `install.ps1` | ❌ Not yet created (planned) |

## Distribution Channels

| Channel | Repo / Path | Status |
|---------|-------------|--------|
| Homebrew tap | `lleontor705/homebrew-tap` | ✅ Active |
| Scoop bucket (Windows) | — | ❌ Not configured (being added) |
| GitHub Releases | `.goreleaser.yaml` | ✅ Active |
| Docker | `Dockerfile` (multi-stage Alpine) | ✅ Active |

## Makefile Targets

| Target | Purpose |
|--------|---------|
| `make build` | Compile the binary |
| `make test` | Run the test suite |
| `make test-coverage` | Run tests with coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format Go source (gofmt / goimports) |
| `make tidy` | `go mod tidy` — clean dependencies |
| `make docker-build` | Build the Docker image |
| `make install` | Install the binary locally |
| `make security` | Run security scans |
| `make check` | Full pre-flight: lint + test + vet (run before commit/push) |

## Invariants

- A release requires all CI gates (lint, test, security) to pass before GoReleaser runs.
- `install.sh` always verifies SHA-256 checksums before installing — never installs an unverified binary.
- GoReleaser builds all 6 platform targets (3 OS × 2 arch) on every release.
- `pr-check.yml` blocks merge until issue ref, approved label, and type label are present.
- The Homebrew tap is updated automatically by GoReleaser — do not manually edit formula outside a release.

## Contributor Checklist

- [ ] Adding a new CI check? Add it to `ci.yml` and ensure `make check` covers it locally.
- [ ] Changing build targets? Update `.goreleaser.yaml` and verify all 6 platform combos build.
- [ ] Adding a new install method (e.g., Scoop, install.ps1)? Document it in this table and update the distribution channels section.
- [ ] Adding a Makefile target? Follow the existing naming and add it to the table above.
- [ ] Modifying release gating? Ensure `release.yml` still runs test → lint → approve → goreleaser in order.
- [ ] Changing changelog filters? Test with `goreleaser release --snapshot` locally.

---

← Prev: [Dashboard & TUI](dashboard.md) · Next: [Project & Extension](project-and-extension.md) →
