# CI & Distribution Inputs Qualification

Requirements: REQ-DISTRIBUTION-001, REQ-VERIFY-001, REQ-VERIFY-002, REQ-PLATFORM-001, REQ-UPDATE-001, REQ-INSTALL-001.

## 1. Authentic Upstream Action & Workflow Provenance

In accordance with REQ-DISTRIBUTION-001, all mutable branch and major tag references in GitHub Actions workflows are pinned to authentic upstream 40-character hexadecimal commit SHAs resolved via `git ls-remote`:

| Action / Reusable Workflow | Upstream Ref | Peeled Tag | Authentic Immutable Commit SHA |
|---|---|---|---|
| `actions/checkout` | `refs/tags/v4` | `v4.4.0` | `11d5960a326750d5838078e36cf38b85af677262` |
| `actions/setup-go` | `refs/tags/v5` | `v5.6.0` | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| `golangci/golangci-lint-action` | `refs/tags/v8` | `v8.0.0` | `4afd733a84b1f43292c63897423277bb7f4313a9` |
| `actions/upload-artifact` | `refs/tags/v4` | `v4.6.2` | `ea165f8d65b6e75b540449e92b4886f43607fa02` |
| `goreleaser/goreleaser-action` | `refs/tags/v6` | `v6.4.0` | `e435ccd777264be153ace6237001ef4d979d3a7a` |
| `actions/github-script` | `refs/tags/v7` | `v7.1.0` | `f28e40c7f34bde8b3046d885e986cb6290c5673b` |
| `actions/stale` | `refs/tags/v9` | `v9.1.0` | `5bef64f19d7facfb25b37b414482c7164d639639` |
| `lleontor705/ats-deploy-public` (quality-go) | `refs/heads/main` | — | `fe9f00320a3a9618d6f8bb841eae891861ec5774` |
| `lleontor705/ats-deploy-public` (security-scan) | `refs/heads/main` | — | `fe9f00320a3a9618d6f8bb841eae891861ec5774` |

## 2. Toolchain Baseline Declarations

- **Go**: Version `1.26.1` with `GOTOOLCHAIN=local`.
- **GolangCI-Lint**: Version `2.11.4`.
- **GoReleaser**: Version `2` with configuration schema `2`.

## 3. Six Canonical Target Runners

All six targets build with `CGO_ENABLED=0`:
1. `linux/amd64` (CI matrix runner)
2. `linux/arm64` (CI matrix runner)
3. `darwin/amd64` (CI matrix runner)
4. `darwin/arm64` (CI matrix runner)
5. `windows/amd64` (Verified locally on Windows host)
6. `windows/arm64` (CI matrix runner)

## 4. Verification

Run the authentic provenance and target runner qualification checks:
```bash
node --test scripts/check-qualification.test.mjs && node scripts/check-qualification.mjs --mode inputs --record scripts/qualification-inputs.json --require-authentic-provenance
node --test scripts/run-target-qualification.test.mjs && node scripts/run-target-qualification.mjs --isolated --require-go 1.26.1 --require-all-targets --record scripts/qualification-inputs.json
```
All checks exit 0.
