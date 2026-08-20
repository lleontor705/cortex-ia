# cortex-ia Agent Guide

## Toolchain

- `go.mod` is authoritative: use Go `1.26.1` (older Go versions mentioned in prose are stale).
- This is a Go CLI/TUI. `package.json` only installs Husky; `npm test` intentionally fails and is not the test runner.
- Build with `go build -o bin/cortex-ia ./cmd/cortex-ia`; run with `go run ./cmd/cortex-ia`. No arguments launch the Bubble Tea TUI; arguments use the hand-written dispatcher in `internal/app/`.

## Verification

- Full local gates, in hook order: `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, then `go test -count=1 ./...`.
- **Testing Scope & Anti-Overengineering Rule**: No crear tests innecesarios ni sobreingenierizados. Las pruebas se limitan exclusivamente a:
  1. Interfaz TUI (`internal/tui/...`).
  2. Validación simple de existencia y copia de archivos/carpetas hacia OpenCode (`internal/pipeline/install_test.go`).
- El objetivo principal del proyecto es copiar limpiamente todos los assets (skills, agentes, comandos) hacia las carpetas de configuración de OpenCode y permitir agregar MCPs manualmente. Deeper transactional oracles run as ephemeral smokes and are deleted after execution.
- Focus a package with `go test ./internal/tui/...`; focus a test with `go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1`.
- Tests use temporary home directories. Never point pipeline or TUI tests at the developer's real agent configuration.

## Architecture

- `cmd/cortex-ia/main.go` only sets the release version and calls `internal/app`; CLI dispatch and the zero-argument TUI split live in `internal/app/app.go`. The dispatcher owns no install, merge, or ownership logic; it parses intent and renders receipts.
- `internal/install` is the service facade: install, sync, doctor, rollback, uninstall, and every MCP operation. All ownership decisions belong to the service and its collaborators.
- `internal/pipeline` plans and applies the asset copy transactionally: `InstallV2` plans first, captures a verified backup, applies, and restores from the backup if the apply phase fails.
- `internal/backup` owns snapshots, manifests, verification, dedup, and retention pruning.
- `internal/mcpmanager` owns the managed MCP catalog (`presets.go`), desired-entry validation, qualification, and conflict errors for the `mcp add/list/remove` surface.
- `internal/state` and `internal/installmeta` own installation metadata, lock, agreement checks, and MCP digests under `~/.cortex-ia/`.
- `internal/agents/opencode` declares the OpenCode native layout (`layout.go`) and the pure asset mapping (`assetmap.go`): every destination is `path.Join(config root, source)`, validated against the single layout declaration, failing closed on unsafe paths, off-surface destinations, and collisions (including case-insensitive ones on Windows/macOS).
- `internal/components/filemerge` owns JSONC decode/merge and atomic writes; reuse it instead of writing ad-hoc merge code.
- Skills, prompts, and commands under `internal/assets/` are runtime source files embedded by `go:embed`; changing them requires rebuilding the binary. `internal/assets/_shared` contracts are compile-time only and are never installed.
- The product configures OpenCode only. Do not reintroduce multi-agent adapters, personas, profiles, model routing, or SDD compiler/registry surfaces; retired commands and flags fail closed in `internal/app/app.go`.

## Repository Workflow

- Before code changes, load the task-matched `SKILL.md`; SDD and utility skill sources are under `internal/assets/skills/`. Load `go-testing` when writing tests. See `docs/sdd-workflow.md` only when the phase map is needed.
- PR CI requires a branch matching `<type>/<lowercase-name>`, a body containing `Closes #N`, `Fixes #N`, or `Resolves #N`, every linked issue labeled `status:approved`, and exactly one `type:*` PR label.
- Commit first lines are enforced only to 10-72 characters by Husky, but repository convention is Conventional Commits; release changelog inclusion depends on `feat`, `fix`, `refactor`, and `perf` prefixes.

Installed skill assets and their triggers:

| Skill | Phase | When to load |
|-------|-------|--------------|
| `bootstrap` | SDD-0 | Starting a new SDD session |
| `investigate` | SDD-1 | Mapping unknown areas of the codebase |
| `draft-proposal` | SDD-2 | Drafting a change proposal |
| `write-specs` | SDD-3 | Producing Given/When/Then scenarios |
| `architect` | SDD-4 | Designing the implementation approach |
| `decompose` | SDD-5 | Breaking the design into tasks |
| `implement` | SDD-6 | Applying spec → code |
| `validate` | SDD-7 | Verifying scenarios pass |
| `finalize` | SDD-8 | Archiving the change set |
| `orchestrator`, `planner`, `reviewer` | cross-phase | Routing, planning, and review roles |
| `debate`, `code-review-adversary` | cross-phase | Multi-position and adversarial deliberation |
| `fast-tdd`, `property-based-testing`, `mutation-testing`, `ast-impact-analysis` | utility | Verification acceleration |
| `context-distiller`, `parallel-dispatch`, `spike-prototype`, `hotfix-triage` | utility | See each SKILL.md for the trigger |
