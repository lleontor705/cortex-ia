# Product Boundary Delta Specification

## ADDED Requirements

### REQ-SCOPE-001: Public surfaces only install configuration

CLI, TUI, and current documentation MUST NOT offer commands that execute AI engines, install global software, or perform model routing. `agent-builder`, `auto-install`, `profiles`, `--profile`, and `--model-preset` MUST return a non-zero out-of-scope error without mutation during their removal window.

Primary oracle: CLI/TUI surface tests plus temporary-home no-mutation tests.

#### Scenario: Installer command remains available

- **Given** a supported adapter and valid selection
- **When** install, repair, sync, doctor, rollback, or uninstall is invoked
- **Then** the configuration lifecycle command remains available

#### Scenario: Retired surface is invoked

- **Given** any retired command or flag
- **When** it is invoked
- **Then** the process exits non-zero, explains that the capability is outside product scope, and performs no mutation

#### Scenario: Help and TUI are rendered

- **Given** the CLI help or TUI welcome menu
- **When** visible actions are enumerated
- **Then** no retired surface is advertised

Coverage: missing.

### REQ-SCOPE-002: Supported payload and adapters remain stable

The default registry MUST contain exactly Claude Code, OpenCode, VS Code Copilot, and Codex. GGA MUST remain absent. SDD prompts, skills, commands, and renderer assets MUST remain installable configuration payload without requiring public profiles or model-routing commands.

Primary oracle: registry tests, SDD golden tests, and four-adapter install integration tests.

#### Scenario: Registry is enumerated

- **Given** the default registry
- **When** its IDs are read
- **Then** the result is exactly `claude-code`, `opencode`, `vscode-copilot`, and `codex`

#### Scenario: SDD is selected

- **Given** any supported adapter
- **When** the SDD component is installed
- **Then** its expected payload is written without executing an AI engine or installing external software

#### Scenario: Removed adapter or GGA is requested

- **Given** a removed agent ID or the `gga` command
- **When** it is resolved
- **Then** resolution fails without mutation

Coverage: partial; registry and GGA negative tests exist, routing-independent payload installation must be proven.

### REQ-UPDATE-001: Update checks are semantic and immutable

Update checks MUST compare valid SemVer values rather than textual inequality. Development or invalid versions MUST report that comparison is unavailable. Recommendations MUST link to the immutable release/tag and MUST NOT recommend piping a mutable `main` script into a shell.

Primary oracle: `internal/update` table-driven tests.

#### Scenario: Newer release exists

- **Given** current `v1.9.0` and latest `v1.10.0`
- **When** versions are compared
- **Then** an update is reported with an HTTPS release/tag URL

#### Scenario: Local version is newer

- **Given** current `v1.10.0` and latest `v1.9.0`
- **When** versions are compared
- **Then** no update is reported

#### Scenario: Development version

- **Given** current `dev` or another invalid SemVer value
- **When** update is checked
- **Then** the result states that comparison is unavailable and does not recommend mutable shell execution

Coverage: missing for semantic ordering.

## Traceability and Ambiguity Log

- Agent Builder rollback remains specified because its package may exist during staged removal.
- Agent Mailbox configuration is owned and removable; runtime data is explicitly not owned.
- SDD remains payload. Whether compiler packages are physically extracted is an architecture decision after payload parity is proven.
- Legacy unsafe backup manifests fail closed; automatic migration is intentionally excluded.

## Global Acceptance Gates

- Every mutation test uses `t.TempDir()` or an injected temporary home.
- `go test -race` passes for backup, pipeline, uninstall, state, and filemerge where supported.
- `gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, and `go test -count=1 ./...` pass.
- Docker E2E is run when a daemon is available.
