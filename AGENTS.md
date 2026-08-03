# cortex-ia Agent Guide

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.

## Verification

- Full local gates, in hook order: `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, then `go test -count=1 ./...`.
- Focus a package with `go test ./internal/pipeline/...`; focus a test with `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1`.
- Injection output is snapshot-tested in `testdata/golden/`. Regenerate only intentional output changes with `go test -update ./internal/components/...`, then inspect the fixture diff and rerun without `-update`.
- Docker E2E requires a running Docker daemon: `./e2e/docker-test.sh` covers Ubuntu, Fedora, and Arch; pass one distro such as `./e2e/docker-test.sh ubuntu` for a focused run.
- Windows tests that need symlink creation skip when `SeCreateSymbolicLinkPrivilege` is unavailable; use Developer Mode or an elevated shell to exercise them.

## Architecture

- `cmd/cortex-ia/main.go` only sets the release version and calls `internal/app`; CLI dispatch and the zero-argument TUI split live in `internal/app/app.go`.
- Agent-specific paths, capabilities, and injection strategies belong in `internal/agents/<agent>/` behind `agents.Adapter`. Register new adapters in `internal/agents/factory.go`; components should not switch on agent IDs.
- Component injectors live in `internal/components/`. Reuse `mcpinject` for MCP strategy dispatch and `filemerge` for marker, JSON, TOML, and atomic-write behavior instead of writing agent-specific merge code.
- `pipeline.Install` deliberately runs Prepare sequentially with rollback, then runs agent chains in parallel while keeping each agent's components sequential because they share config files. Preserve that concurrency boundary.
- Skills, prompts, and commands under `internal/assets/` are runtime source files embedded by `go:embed`; changing them requires rebuilding the binary and often updating injection/golden tests.
- Tests use temporary home directories. Never point injector or pipeline tests at the developer's real agent configuration.

## Repository Workflow

- Before code changes, load the task-matched `SKILL.md`: community issue/PR workflows are under `skills/`; SDD and utility skill sources are under `internal/assets/skills/`. Load `go-testing` when writing tests. See `docs/sdd-workflow.md` only when the phase map is needed.
- PR CI requires a branch matching `<type>/<lowercase-name>`, a body containing `Closes #N`, `Fixes #N`, or `Resolves #N`, every linked issue labeled `status:approved`, and exactly one `type:*` PR label.
- Commit first lines are enforced only to 10-72 characters by Husky, but repository convention is Conventional Commits; release changelog inclusion depends on `feat`, `fix`, `refactor`, and `perf` prefixes.
