# Contributing to cortex-ia

Thank you for your interest in contributing to **cortex-ia** — a Go CLI/TUI ecosystem configurator that supercharges AI coding agents (Claude Code, OpenCode, Gemini CLI, Codex, Cursor, VS Code, Windsurf, Antigravity) with persistent memory (Cortex), spec-driven development (SDD) skills, MCP servers, personas, and permissions.

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

cortex-ia uses a **2-stage installation pipeline** (Prepare → Apply with rollback) and a granular **component model**. Familiarize yourself with these concepts before opening a non-trivial PR.

### Supported Agents (8)

`claude`, `opencode`, `cursor`, `gemini`, `vscode`, `codex`, `windsurf`, `antigravity`. Each agent has its own adapter under `internal/agents/<id>/`.

### Components

- **`cortex`** — Persistent memory (31 MCP tools): observations, sessions, knowledge graph, FTS5 search, importance scoring, temporal reasoning.
- **`forgespec`** — Spec-driven artifact contracts (15 tools).
- **`mailbox`** — Inter-agent messaging (21 tools).
- **`mcpinject`** — Generic MCP injector (per-agent strategy: JSON / merge / TOML / config-file).
- **`context7`** — Live docs lookup MCP (2 tools).
- **`conventions`** — Shared memory protocol & convention file.
- **`persona`** — Tone presets: `professional`, `mentor`, `minimal`.
- **`permissions`** — Security guardrails.
- **`sdd`** — Spec-Driven Development workflow injection (roles, skills, prompts).
- **`skills`** — 19 SDD skills loader (embedded + community + project layers).
- **`theme`** — Per-agent theme overlay.
- **`gga`** — AI provider switcher (Anthropic / OpenAI / Google / Ollama).

### Healthchecks (`cortex-ia doctor`)

Six checks: managed files, cortex binary, node/npx, skills layout, convention file, state/lock integrity.

---

## Development Setup

### Prerequisites

- Go 1.24+
- Docker (for E2E tests)
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
./cortex-ia detect     # detect installed agents + runtime deps
./cortex-ia install --dry-run --preset full
```

---

## Testing

### Unit Tests

Run the full unit test suite:

```bash
go test ./...
```

Run tests for a specific package:

```bash
go test ./internal/pipeline/...
go test ./internal/components/...
```

Run with verbose output:

```bash
go test -v ./...
```

### Golden File Tests

Component injection output is verified against golden fixtures in `testdata/golden/`. To regenerate after intentional changes:

```bash
go test -update ./internal/components/...
```

CI runs without `-update` and fails on any drift.

### E2E Tests

E2E tests are Docker-based shell scripts. Docker must be running.

```bash
cd e2e
chmod +x docker-test.sh
./docker-test.sh             # all distros (ubuntu, fedora)
./docker-test.sh ubuntu      # specific distro
```

> E2E tests spin up containers to simulate real installation environments. They may take a few minutes to complete.

### Windows — Known Test Limitations

Some unit tests require OS-level capabilities that are restricted on Windows by default.

#### Symlink tests (`SeCreateSymbolicLinkPrivilege`)

Tests that create symbolic links (e.g. in `internal/components/filemerge`) will be **skipped automatically** on Windows builds where the process lacks `SeCreateSymbolicLinkPrivilege` (`ERROR_PRIVILEGE_NOT_HELD`, errno 1314). This is a Windows security policy, not a bug in the code.

To run these tests without restrictions, choose one of:

- **Enable Developer Mode** — Settings → System → For developers → Developer Mode. Grants symlink creation to all processes without admin rights.
- **Run as Administrator** — open your terminal as Administrator before running `go test ./...`.
- **Grant the privilege explicitly** via Group Policy: `Local Security Policy → User Rights Assignment → Create symbolic links`.

> On Linux and macOS these tests always run without any extra setup.

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
feat(tui): add agent-builder TUI flow
fix(pipeline): rollback on apply failure preserves prior state
docs: document opencode SDD profiles
chore(deps): bump bubbletea to v1.4
refactor(components): extract MCP injector strategy interface
style: gofmt internal/agents/*
perf(verify): cache health check results within a single doctor run
test(backup): cover compression dedup path
build: add Dockerfile.arch for e2e matrix
ci: add check-branch-name to pr-check workflow
revert: undo persona auto-detection
```

### Breaking Changes

Add `!` after the type/scope and include a `BREAKING CHANGE:` footer:

```
feat(cli)!: rename --preset flag to --profile

BREAKING CHANGE: the --preset flag has been renamed to --profile.
Update your scripts and aliases accordingly.
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

**Examples:** `feat/agent-builder-tui`, `fix/persona-injection-windows`, `docs/cortex-memory-tools`, `ci/add-arch-dockerfile`

---

## Pull Request Rules

### Before Opening a PR

- [ ] There is a linked approved issue (`Closes #<N>`)
- [ ] All unit tests pass (`go test ./...`)
- [ ] Golden tests pass (`go test ./internal/components/...`)
- [ ] E2E tests pass (`cd e2e && ./docker-test.sh`)
- [ ] Commits follow Conventional Commits format
- [ ] Code is self-reviewed

### PR Title

Use the same Conventional Commits format as commit messages:

```
feat(tui): add agent-builder preview screen
fix(pipeline): handle empty selection gracefully
```

### Automated PR Checks

All PRs go through automated checks:

| Check | What It Verifies |
|-------|-----------------|
| **Check Issue Reference** | PR body contains `Closes/Fixes/Resolves #N` |
| **Check Issue Has status:approved** | The linked issue has been approved by a maintainer |
| **Check PR Has type:* Label** | Exactly one `type:*` label is applied |

In addition, **Unit Tests**, **Golden Tests**, and **E2E Tests** run via the `ci.yml` workflow and must pass on the PR branch.

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
