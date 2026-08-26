# Contributing to cortex-ia

Thank you for your interest in contributing to **cortex-ia** — a Go CLI/TUI configurator for **OpenCode**. It installs the embedded workflow asset set under `~/.config/opencode/`, manages Cortex and Context7 MCP entries, and provides its own SQLite task/lease control through `cortex-ia work`.

Before you dive in, please read this guide fully. We have a structured workflow to keep the project organized and maintainable.

---

## Table of Contents

- [Issue-First Workflow](#issue-first-workflow)
- [Label System](#label-system)
- [Project Architecture](#project-architecture)
- [Development Setup](#development-setup)
- [Testing](#testing)
- [Commit Convention](#commit-convention)
- [Branch Naming](#branch-naming)
- [Pull Request Rules](#pull-request-rules)
- [Code of Conduct](#code-of-conduct)

---

## Issue-First Workflow

**No PR without an issue. No exceptions.**

This project follows a strict issue-first workflow:

1. **Open an issue** using the appropriate template ([Bug Report](https://github.com/lleontor705/cortex-ia/issues/new?template=bug_report.yml) or [Feature Request](https://github.com/lleontor705/cortex-ia/issues/new?template=feature_request.yml))
2. **Wait for approval** — a maintainer will add the `status:approved` label when the issue is ready to be worked on
3. **Comment on the issue** to let others know you're working on it
4. **Open a PR** referencing the approved issue

PRs that are not linked to an approved issue will be **automatically rejected** by CI.

---

## Label System

### Type Labels (applied to PRs)

| Label | Description |
|-------|-------------|
| `type:bug` | Bug fix |
| `type:feature` | New feature or enhancement |
| `type:refactor` | Code refactoring, no functional changes |
| `type:docs` | Documentation only |
| `type:test` | Test coverage additions |
| `type:chore` | Build, CI, tooling changes |
| `type:breaking` | Breaking change |

### Status Labels (applied to Issues)

| Label | Description |
|-------|-------------|
| `status:needs-review` | Newly opened, awaiting maintainer review |
| `status:approved` | Approved for implementation — work can begin |
| `status:in-progress` | Being worked on |
| `status:blocked` | Blocked by another issue or external dependency |
| `status:wont-fix` | Out of scope or won't be addressed |

### Priority Labels

| Label | Description |
|-------|-------------|
| `priority:critical` | Blocking issues, security vulnerabilities |
| `priority:high` | Important, affects many users |
| `priority:medium` | Normal priority |
| `priority:low` | Nice to have |

---

## Project Architecture

cortex-ia installs a single embedded asset set into the OpenCode config root, transactionally: it plans first, captures a verified backup, applies, and restores from the backup if the apply phase fails. Familiarize yourself with these concepts before opening a non-trivial PR.

### Supported Agents

`opencode` — the product configures OpenCode only and never writes anywhere else. The OpenCode native layout and pure asset mapping live in `internal/agents/opencode/`.

### Key Packages

- **`internal/install`** — service facade: `install`, `sync`, `doctor`, `rollback`, `uninstall`, and every MCP operation.
- **`internal/pipeline`** — plans and applies the asset copy transactionally.
- **`internal/backup`** — snapshots, manifests, verification, dedup, retention pruning.
- **`internal/mcpmanager`** — managed MCP catalog (`cortex`, `context7`), legacy ForgeSpec removal, desired-entry validation, and conflict errors.
- **`internal/delegation`** — SQLite WAL store for work-control tasks/claims/leases and external delegation jobs.
- **`internal/components/filemerge`** — JSONC decode/merge and atomic writes.
- **`internal/state` / `internal/installmeta`** — installation metadata, lock, and MCP digests under `~/.cortex-ia/`.
- **`internal/tui`** — Bubble Tea terminal UI (launched with no arguments).

### Embedded Assets

Skills, agents, commands, and the plugin under `internal/assets/` are embedded via `go:embed` and copied byte-for-byte at install time (14 agents, 9 commands, 21 skills, 1 plugin). Changing them requires rebuilding the binary.

### Healthcheck (`cortex-ia doctor`)

A strictly read-only report: state/lock presence and agreement, per-artifact digest checks, MCP ownership per preset, and an overall verdict. Doctor never mutates anything.

---

## Development Setup

### Prerequisites

- Go 1.26.1+ (`go.mod` is authoritative)
- Git

### Clone and Build

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o cortex-ia ./cmd/cortex-ia
```

### Run Locally

```bash
./cortex-ia            # interactive TUI
./cortex-ia --help     # CLI reference
./cortex-ia doctor     # read-only health report
./cortex-ia install --dry-run
```

---

## Testing

### Full Local Gates

Run the complete gate set in hook order before opening a PR:

```bash
gofmt -s -w .
go vet ./...
golangci-lint run ./...
go test -count=1 ./...
```

### Test Scope

Persistent tests cover exactly the TUI (`internal/tui/...`) and the pipeline install behavior (`internal/pipeline/install_test.go`). Do not add test suites elsewhere without maintainer agreement; deeper transactional oracles run as ephemeral smokes and are deleted after execution.

Focus a package or a single test:

```bash
go test ./internal/tui/...
go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1
```

Tests use temporary home directories. Never point tests at your real OpenCode configuration.

---

## Commit Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/).

Commit messages **must** match this pattern:

```
^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9\._-]+\))?!?: .+
```

### Format

```
<type>(<optional-scope>)!: <description>

[optional body]

[optional footer]
```

### Allowed Types

| Type | Purpose |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code change (no behavior change) |
| `chore` | Maintenance, dependencies, tooling |
| `style` | Formatting, linting (no logic change) |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `build` | Build system or external deps |
| `ci` | CI configuration |
| `revert` | Reverts a previous commit |

### Examples

```
feat(tui): add MCP preset picker to install review
fix(pipeline): rollback on apply failure preserves prior state
docs: document managed MCP presets
chore(deps): bump bubbletea to v1.4
refactor(mcpmanager): extract desired-entry validation
style: gofmt internal/agents/opencode
perf(doctor): reuse tolerant state load in one report
test(backup): cover checksum dedup path
build: pin goreleaser build image
ci: add check-branch-name to pr-check workflow
revert: undo custom MCP header support
```

### Breaking Changes

Add `!` after the type/scope and include a `BREAKING CHANGE:` footer:

```
feat(cli)!: require --yes for uninstall

BREAKING CHANGE: `cortex-ia uninstall` now requires an explicit `--yes`
confirmation. Update your scripts and aliases accordingly.
```

Breaking changes map to the `type:breaking` label.

---

## Branch Naming

Branch names **must** match this pattern:

```
^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$
```

**Rules:**
- All lowercase
- Use hyphens, dots, or underscores as separators (no spaces, no uppercase)
- Description must be short and descriptive

**Examples:** `feat/mcp-preset-import`, `fix/backup-dedup-windows`, `docs/cortex-memory-tools`, `ci/pin-go-version`

---

## Pull Request Rules

### Before Opening a PR

- [ ] There is a linked approved issue (`Closes #<N>`)
- [ ] Full gates pass (`gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`)
- [ ] Commits follow Conventional Commits format
- [ ] Code is self-reviewed

### PR Title

Use the same Conventional Commits format as commit messages:

```
feat(tui): add MCP preset picker to install review
fix(pipeline): handle empty selection gracefully
```

### Automated PR Checks

All PRs go through automated checks:

| Check | What It Verifies |
|-------|-----------------|
| **Check Issue Reference** | PR body contains `Closes/Fixes/Resolves #N` |
| **Check Issue Has status:approved** | The linked issue has been approved by a maintainer |
| **Check PR Has type:* Label** | Exactly one `type:*` label is applied |
| **Check Branch Name Convention** | Branch matches `<type>/<lowercase-name>` |

In addition, the **CI** workflow runs Go quality and security scans on the PR branch.

**All checks must pass** before a PR can be merged.

### Linking Your Issue

In the PR body, include one of:

```
Closes #42
Fixes #42
Resolves #42
```

---

## Code of Conduct

Be respectful. We're building something together.

- Critique code, not people
- Be constructive in reviews
- Welcome newcomers

Violations may result in removal from the project.

---

## Questions?

Use [GitHub Discussions](https://github.com/lleontor705/cortex-ia/discussions) — not issues — for questions, ideas, and general conversation.
