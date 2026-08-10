# cortex-ia Agent Guide

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.

## Verification

- Full local gates, in hook order: `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, then `go test -count=1 ./...`.
- **Testing Scope & Anti-Overengineering Rule**: No crear tests innecesarios ni sobreingenierizados. Las pruebas unitarias/integración se limitan exclusivamente a:
  1. Interfaz TUI (`internal/tui/...`).
  2. Integración e inyección de MCPs (`internal/components/mcpinject/...`, `internal/components/mcpprobe/...`).
  3. Validación simple de existencia y copia de archivos/carpetas hacia OpenCode (`internal/pipeline/install_test.go`, `internal/components/skills/inject_test.go`).
- El objetivo principal del proyecto es copiar limpiamente todos los assets (skills, agentes, comandos) hacia las carpetas de configuración de OpenCode y permitir agregar MCPs manualmente.
- Focus a package with `go test ./internal/tui/...`; focus a test with `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1`.
- Injection output is snapshot-tested in `testdata/golden/`. Regenerate only intentional output changes with `go test -update ./internal/components/...`, then inspect the fixture diff and rerun without `-update`.
- Docker E2E requires a running Docker daemon: `./e2e/docker-test.sh` covers Ubuntu, Fedora, and Arch; pass one distro such as `./e2e/docker-test.sh ubuntu` for a focused run.
- Windows tests that need symlink creation skip when `SeCreateSymbolicLinkPrivilege` is unavailable; use Developer Mode or an elevated shell to exercise them.

## Architecture

## Repository Skills

These are **repo-specific** versions that complement the generic builtin skills above. They contain cortex-ia specific templates, CI checks, and labels. Use these when working ON cortex-ia itself; use the builtin versions for other projects.

| Skill | Complements | Path |
|-------|-------------|------|
| `cortex-ia-issue-creation` | `file-issue` (builtin) | [`skills/issue-creation/SKILL.md`](skills/issue-creation/SKILL.md) |
| `cortex-ia-branch-pr` | `open-pr` (builtin) | [`skills/branch-pr/SKILL.md`](skills/branch-pr/SKILL.md) |

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

Highlights:

| Skill | Phase | When to load |
|-------|-------|--------------|
| `bootstrap` | SDD-0 | Starting a new SDD session |
| `investigate` / `sdd-explore` | SDD-1 | Mapping unknown areas of the codebase |
| `draft-proposal` / `sdd-propose` | SDD-2 | Drafting a change proposal |
| `write-specs` / `sdd-spec` | SDD-3 | Producing Given/When/Then scenarios |
| `architect` / `sdd-design` | SDD-4 | Designing the implementation approach |
| `decompose` / `sdd-tasks` | SDD-5 | Breaking the design into tasks |
| `implement` / `sdd-apply` | SDD-6 | Applying spec → code |
| `validate` / `sdd-verify` | SDD-7 | Verifying scenarios pass |
| `finalize` / `sdd-archive` | SDD-8 | Archiving the change set |
| `judgment-day` | cross-phase | Adversarial dual-review of a change before merge |
| `debate` | cross-phase | Multi-position deliberation |
| `onboard` | cross-phase | Guided end-to-end SDD walkthrough |
| `chained-pr` | cross-phase | Splitting oversized PRs into chained/stacked PRs |
| `work-unit-commits` | cross-phase | Planning reviewable commits during implementation |
| `debug`, `monitor`, `ideate`, `execute-plan`, `open-pr`, `file-issue`, `parallel-dispatch`, `scan-registry`, `go-testing`, `cognitive-doc-design`, `comment-writer`, `skill-creator`, `skill-improver` | utility | See each SKILL.md for the trigger |
