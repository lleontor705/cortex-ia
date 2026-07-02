# Maintainer Playbook

← [Codebase Guide](../CODEBASE-GUIDE.md)

Step-by-step runbook for cortex-ia maintainers: pre-release checks, the release flow, post-release verification, and dependency maintenance. This page covers the **process** — for the tooling config (GoReleaser, workflows) see [integrations.md](integrations.md), for the repo layout see [repository-map.md](repository-map.md).

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| Pre-release checklist | ✅ | — |
| Release flow (tag → CI → publish) | ✅ | — |
| Post-release verification | ✅ | — |
| Dependency updates | ✅ | — |
| GoReleaser / workflow config | — | [integrations.md](integrations.md) |
| How to extend the codebase | — | [project-and-extension.md](project-and-extension.md) |

## Pre-Release Checklist

Run these before tagging a release.

| # | Step | Command | Expected |
|---|------|---------|----------|
| 1 | Full pre-flight check | `make check` | Lint + test + vet all pass |
| 2 | Run test coverage | `make test-coverage` | No regressions vs last release |
| 3 | Format check | `make fmt` | No formatting diffs |
| 4 | Tidy dependencies | `make tidy` | `go.sum` clean, no diff after |
| 5 | Security scan | `make security` | No new vulnerabilities |
| 6 | Golden files line endings | Verify `.gitattributes` pins `testdata/golden/**` to `eol=lf` | No CRLF contamination |
| 7 | Verify clean tree | `git status` | Working tree clean, nothing uncommitted |

### Golden Files

| Aspect | Detail |
|--------|--------|
| Location | `testdata/golden/**` |
| Line ending | `eol=lf` enforced via `.gitattributes` |
| Risk | Windows checkouts can introduce CRLF; `.gitattributes` normalizes at commit |
| Check | `git diff` should show no line-ending-only changes after checkout on any platform |

## Release Flow

Releases are triggered by pushing a semver tag to `main`. The `release.yml` workflow automates the rest.

```
1. Tag the release          → git tag vX.Y.Z && git push origin vX.Y.Z
2. release.yml triggers     → test → lint → manual approve → goreleaser
3. GoReleaser builds        → 6 targets (linux/darwin/windows × amd64/arm64)
4. GoReleaser publishes     → GitHub Release + archives + checksums
5. Homebrew tap updates     → lleontor705/homebrew-tap formula auto-updated
```

### Release Gates

| Gate | Enforced by | Blocks on failure |
|------|------------|-------------------|
| Tests pass | `release.yml` step 1 | ✅ |
| Lint passes | `release.yml` step 2 | ✅ |
| Manual approval | `release.yml` step 3 | ✅ |
| GoReleaser succeeds | `release.yml` step 4 | ✅ |

### Tagging Convention

| Format | Example | Usage |
|--------|---------|-------|
| `vX.Y.Z` | `v1.2.3` | Stable release |
| `vX.Y.Z-rc.N` | `v1.2.3-rc.1` | Release candidate (optional) |

## Post-Release Verification

After GoReleaser completes, verify the release is healthy.

| # | Step | How to verify |
|---|------|---------------|
| 1 | Check GitHub Release page | All 6 platform archives + checksums present |
| 2 | Verify Homebrew install | `brew install lleontor705/homebrew-tap/cortex-ia` succeeds on a clean machine |
| 3 | Verify curl installer | `curl -fsSL .../install.sh \| bash` succeeds and checksum validates |
| 4 | Verify binary runs | `cortex-ia version` reports the new version |
| 5 | Verify `cortex-ia doctor` | Clean run on a fresh install with no errors |
| 6 | Spot-check cross-platform | At minimum: linux/amd64 and darwin/arm64 binaries run |

## Dependency Updates

| # | Step | Command |
|---|------|---------|
| 1 | Update go.mod | `go get -u ./...` |
| 2 | Tidy | `make tidy` (runs `go mod tidy`) |
| 3 | Test | `make test` |
| 4 | Lint | `make lint` |
| 5 | Security scan | `make security` |
| 6 | Commit | `chore(deps): update dependencies` |

### Dependency Rules

| Rule | Detail |
|------|--------|
| Run `go mod tidy` after any dependency change | Ensures `go.sum` is consistent |
| Check for breaking changes | Review changelogs of major version bumps |
| Security advisories | Address immediately; do not defer |
| Go version | Toolchain is Go 1.26.1; update `go.mod` `go` directive deliberately |

## Emergency: Bad Release

If a release is broken and already published:

| # | Step |
|---|------|
| 1 | Do **not** delete the tag (consumers may have cached it) |
| 2 | Assess severity — is it a crash, data loss, or cosmetic? |
| 3 | If fixable quickly: tag `vX.Y.(Z+1)` patch release |
| 4 | If critical: mark the GitHub Release as a pre-release / draft, communicate via issue |
| 5 | Homebrew tap can be manually rolled back if needed |

## Invariants

- Never tag a release from a dirty working tree — `git status` must be clean first.
- Never skip `make check` — it is the single gate that catches lint, test, and vet failures.
- Golden files must be `eol=lf` on all platforms — `.gitattributes` enforces this; do not override.
- The release workflow is tag-triggered — do not run `goreleaser` manually for official releases.
- All 6 platform targets must build — a release with missing platforms is incomplete.

## Contributor Checklist

- [ ] Before tagging: `make check` passes, `git status` clean, golden files `eol=lf`.
- [ ] Tag follows `vX.Y.Z` semver format.
- [ ] After release: verify GitHub Release, Homebrew install, curl installer, and `cortex-ia version`.
- [ ] Dependency updates always followed by `make tidy` + `make check`.
- [ ] Security advisories addressed before any feature work.
- [ ] Go toolchain version updated deliberately in `go.mod`, not accidentally.

---

← Prev: [Project & Extension](project-and-extension.md) · Next: [Reference Map](reference-map.md) →
