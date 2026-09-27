# Reference Map

← [Codebase Guide](../CODEBASE-GUIDE.md)

Quick-reference index of CLI commands, Go packages, key types, MCP tools, config files, and documentation pages. This page is a lookup table — for explanations see the sibling pages linked below. Use `Ctrl+F` to find any identifier.

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| CLI commands | ✅ | — |
| Go packages | ✅ | — |
| Key types | ✅ | — |
| MCP servers & tools | ✅ | — |
| Config files | ✅ | — |
| Documentation pages | ✅ | — |
| Architecture explanation | — | [mental-model.md](mental-model.md) |

## CLI Commands

| Command | Purpose |
|---------|---------|
| `cortex-ia` (no args) | Launch interactive TUI dashboard |
| `cortex-ia install` | Install assets + plugins for `opencode`, `claude`, or `all` |
| `cortex-ia sync` | Reconcile an installed home with the current asset set |
| `cortex-ia snapshot` | Read and verify one bounded local Cortex snapshot (`read`) |
| `cortex-ia work` | Manage the local task DAG, claims, leases, transitions, approvals |
| `cortex-ia worktree` | List or validate authoritative Git worktrees (`list`, `validate`) |
| `cortex-ia board` | Create and manage durable task boards (`create`, `list`, `status`, `archive`, `unarchive`, `delete`, `serve`) |
| `cortex-ia ledger` | Inspect or update the Dual Ledger (`fact`, `progress`, `status`) |
| `cortex-ia ui` | Print a bounded read-only TUI snapshot (`snapshot`) |
| `cortex-ia openspec` | Manage the OpenSpec SDD workspace (`validate`, `list`, `status`, `archive`, `new`) |
| `cortex-ia web` | Launch the local operations dashboard |
| `cortex-ia doc` | Convert office/PDF documents to Markdown or inspect metadata (`convert`, `inspect`) |
| `cortex-ia diagram` | Validate, render, compare, or trace system diagrams (`validate`, `render`, `compare`, `reach`) |
| `cortex-ia mcp` | Manage MCP entries (`add`, `remove`, `list`, `adopt`) |
| `cortex-ia model` | Manage global-config agent model assignments (`list`, `get`, `set`, `unset`, `doctor`, `catalog`) |
| `cortex-ia stats` | Print bounded read-only usage statistics (`[--json]`) |
| `cortex-ia report` | Report errors or manage reporting config (`error`/`send`, `config`, `flush`, `status`) |
| `cortex-ia doctor` | Assess installation health (read-only) |
| `cortex-ia rollback` | Restore a backup or list available backups (`[backup-id]`, `list`) |
| `cortex-ia recover` | List or restore pending recovery journals (`[list]`, `<journal-id>`) |
| `cortex-ia uninstall` | Remove the accredited installation |
| `cortex-ia update` | Check for / install the latest release (alias: `upgrade`) |
| `cortex-ia version` | Show version |
| `cortex-ia help` | Show usage |

Retired surfaces: `delegate`, `herdr`, and `hook` belong to the removed external AGY plane. They are not dispatched and fail closed with a retired-surface error. `internal/herdr` remains in the tree only as an optional diagnostics helper for the web console status display.

## Go Packages

| Package | Location | One-liner |
|---------|----------|-----------|
| Entry point | `cmd/cortex-ia/` | `main.go` — ldflags version injection |
| CLI dispatch | `internal/app/` | Subcommand routing + TUI launch; retired-surface fail-closed guard |
| Install facade | `internal/install/` | Service facade: install, sync, doctor, rollback, uninstall, MCP operations |
| Pipeline | `internal/pipeline/` | Transactional copy engine: plan, verified backup, atomic apply, journal, rollback |
| Backup | `internal/backup/` | Snapshots, manifests, restore verification, dedup, retention pruning |
| State | `internal/state/` | Install metadata v2, lock agreement, semantic and postimage digests under `~/.cortex-ia/` |
| Install metadata | `internal/installmeta/` | Versioned, secret-free semantic digests for MCP and model entries |
| Delegation | `internal/delegation/` | SQLite work authority: boards, DAG tasks, claims, TTL file leases, approvals, recovery |
| Assets | `internal/assets/` | `go:embed` access to agents, commands, skills, plugins, themes, and TUI assets |
| OpenCode layout | `internal/agents/opencode/` | Declarative home-relative layout + pure asset mapping and collision checks |
| MCP manager | `internal/mcpmanager/` | Managed MCP presets, ownership accreditation, qualification, conflict errors |
| Model manager | `internal/modelmgr/` | Global-config agent model assignment, doctor, and catalog |
| Provider manager | `internal/providermgr/` | Per-provider JSON catalogs under `~/.cortex-ia/` |
| OpenSpec | `internal/openspec/` | Planning-structure validation; never approves semantics or work |
| Diagram | `internal/diagram/` | System diagram validate, render, compare, and reach |
| Doc conversion | `internal/docconv/` | Office/PDF conversion to Markdown plus metadata inspection |
| Telemetry | `internal/telemetry/` | Bounded error-report outbox |
| Updater | `internal/updater/` | Release check, authenticated download, checksum/trust verification, atomic replacement |
| Targets | `internal/targets/` | Install target parsing and Claude target configuration |
| CLI detection | `internal/clidetect/` | Host CLI availability detection for target recommendations |
| Home lock | `internal/homelock/` | Cross-process, per-home exclusive lock |
| Logging | `internal/logging/` | Debug tracing sink (stderr + file); never writes to stdout |
| OpenCode stats | `internal/ocstats/` | Read-only, windowed aggregates of OpenCode usage data |
| Web console | `internal/cortexiaweb/` | Loopback-only Preact operations console and HTTP/API server |
| Components | `internal/components/filemerge/` | Atomic writes, JSON/TOML deep merge, section-based text merge |
| TUI | `internal/tui/` | Bubble Tea dashboard, 10 screens |
| TUI assets | `internal/tuiassets/` | TypeScript source for the compiled TUI plugin |
| Herdr diagnostics | `internal/herdr/` | Optional diagnostics helper consumed only by the web console status display |

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `TargetID` | `internal/targets/targets.go` | Install target identifier (`opencode`, `claude`, `all`) |
| `TargetResult` | `internal/targets/targets.go` | Outcome of installing or updating one target |
| `Layout` | `internal/agents/opencode/layout.go` | Home-relative OpenCode discovery and Cortex-IA workflow roots |
| `Mapping` | `internal/agents/opencode/assetmap.go` | Embedded asset → managed destination pair with content hash |
| `Preset` | `internal/mcpmanager/presets.go` | Managed MCP server entry template |
| `EntryStatus` | `internal/mcpmanager/manager.go` | Ownership classification of an observed MCP entry |
| `WorkStatus` | `internal/delegation/work.go` | Task lifecycle state (`backlog`/`ready`/`in_progress`/`in_review`/`done`/`blocked`) |
| `WorkloadPolicy` | `internal/delegation/work.go` | Line-budget policy (`strict`/`flexible`/`unbounded`) |
| `WorkItem` | `internal/delegation/work.go` | Durable task DAG node |
| `WorkClaim` | `internal/delegation/work.go` | Live task claim authority |
| `WorkLease` | `internal/delegation/work.go` | TTL file reservation owned by a claim |
| `RetiredSurfaceError` | `internal/app/app.go` | Fail-closed error for removed commands and flags |

## MCP Servers

| Server | Execution vector | Purpose | Managed in |
|--------|------------------|---------|------------|
| Cortex | `cortex mcp --tools=agent` | Persistent memory + knowledge graph | `internal/mcpmanager/presets.go` |
| Context7 | `npx -y @upstash/context7-mcp@4.1.0` | Library documentation lookup | `internal/mcpmanager/presets.go` |

`cortex-ia work` provides task coordination as a built-in Go CLI and SQLite store; it is not an MCP server. The retired `forgespec` preset is recognized only so `sync` can remove it.

## Config & Build Files

| File | Location | Purpose |
|------|----------|---------|
| `go.mod` | root | Module `github.com/lleontor705/cortex-ia`, Go 1.26.1 |
| `Makefile` | root | build, test, lint, fmt, tidy, docker, install, security, check |
| `.goreleaser.yaml` | root | Cross-platform release config |
| `.golangci.yml` | root | Lint config (errcheck, govet, staticcheck, unused, ineffassign) |
| `Dockerfile` | root | Multi-stage Alpine build |
| `.gitattributes` | root | Pins Go sources to `eol=lf` |
| `.github/workflows/ci.yml` | `.github/workflows/` | Quality + security CI |
| `.github/workflows/release.yml` | `.github/workflows/` | Tag-triggered release pipeline |
| `.github/workflows/pr-check.yml` | `.github/workflows/` | PR compliance checks |
| `.github/workflows/stale.yml` | `.github/workflows/` | Issue/PR lifecycle (30d stale, 14d close) |
| `scripts/install.sh` | `scripts/` | Curl-pipe installer with SHA-256 verify |

## State Files

| File | Location | Purpose |
|------|----------|---------|
| `state.json` | `~/.cortex-ia/` | Installed agents, preset, components — sync source of truth |
| `cortex-ia.lock` | `~/.cortex-ia/` | Concrete written-file list with checksums |
| `install-status.json` | `~/.cortex-ia/` | Crash-detection marker (ephemeral) |
| `delegation.db` | `~/.cortex-ia/` | SQLite work authority: boards, tasks, claims, leases, approvals, events |

## Key Counts

| Metric | Value |
|--------|-------|
| AI agents | 6 |
| Slash commands | 13 |
| Skills | 17 (+ `_shared`) |
| Plugins | 8 |
| CLI subcommands | 23 |
| TUI screens | 10 |
| Build targets | 6 (3 OS × 2 arch) |
| Go version | 1.26.1 |

## Documentation Pages

| Page | Location | Covers |
|------|----------|--------|
| Mental Model | `docs/codebase/mental-model.md` | End-to-end data flow |
| Repository Map | `docs/codebase/repository-map.md` | Directory-by-directory index |
| MCP Boundaries | `docs/codebase/mcp-boundaries.md` | MCP server contracts & limits |
| SDD Coordination | `docs/codebase/sdd-coordination.md` | Spec-Driven Development workflow |
| Interfaces | `docs/codebase/interfaces.md` | Go interface contracts |
| Sync, State & Backup | `docs/codebase/sync-and-cloud.md` | State files, snapshots, local sync |
| Dashboard & TUI | `docs/codebase/dashboard.md` | Bubbletea architecture, screens |
| Integrations | `docs/codebase/integrations.md` | CI/CD, GoReleaser, installer |
| Project & Extension | `docs/codebase/project-and-extension.md` | Adding agents/skills/components/subcommands |
| Maintainer Playbook | `docs/codebase/maintainer-playbook.md` | Release & dependency process |
| Reference Map | `docs/codebase/reference-map.md` | This page — quick lookup index |
| Architecture | `docs/architecture.md` | High-level architecture overview |
| Agents | `docs/AGENTS.md` | Agent topology & coordination contracts (6 native roles) |
| SDD Workflow | `docs/sdd-workflow.md` | SDD skills & workflow docs |

## Invariants

- This page is a lookup index — if you need explanation, follow the sibling page link.
- Counts are maintained manually; update them when adding agents/skills/components.
- Package locations reflect `internal/` structure; `cmd/` is the only non-internal importable package.
- MCP servers are managed as presets in `internal/mcpmanager/presets.go`; `internal/components/` holds only the shared `filemerge` primitives.

## Contributor Checklist

- [ ] Added a new agent/skill/component/subcommand? Update the corresponding table and count above.
- [ ] Added a new Go package? Add it to the Go Packages table.
- [ ] Added a new config/build file? Add it to the Config & Build Files table.
- [ ] Added a new doc page? Add it to the Documentation Pages table.
- [ ] Keep this page as a pure index — move explanations to sibling pages.

---

← Prev: [Maintainer Playbook](maintainer-playbook.md) · Next: (end) →
