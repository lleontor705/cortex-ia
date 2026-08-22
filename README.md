<p align="center">
  <br>
  <img src="docs/assets/logo.svg" alt="cortex-ia" width="400" />
  <br><br>
  <em>One command. OpenCode only. Transactional by default.</em>
  <br><br>
  <a href="https://github.com/lleontor705/cortex-ia/actions/workflows/ci.yml"><img src="https://github.com/lleontor705/cortex-ia/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/lleontor705/cortex-ia/releases/latest"><img src="https://img.shields.io/github/v/release/lleontor705/cortex-ia" alt="Release"></a>
  <a href="https://github.com/lleontor705/cortex-ia/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lleontor705/cortex-ia" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lleontor705/cortex-ia"><img src="https://goreportcard.com/badge/github.com/lleontor705/cortex-ia" alt="Go Report Card"></a>
</p>

---

cortex-ia is a single Go binary that installs the embedded OpenCode workflow
asset set into your home configuration and manages its MCP server entries.
It copies skills, agents, commands, the plugin, and the base config under
`~/.config/opencode/`, registers the managed MCP selection, and does all of
it transactionally: plan first, verified backup before any write, rollback on
failure, and fail-closed on anything it does not own. It configures OpenCode
only and never writes anywhere else.

## Quick Start

```bash
cortex-ia                 # Interactive TUI
cortex-ia install         # Install assets + default managed MCPs
cortex-ia install --dry-run
cortex-ia doctor          # Read-only health report
```

See [Quickstart](docs/quickstart.md) for the guided path.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install lleontor705/tap/cortex-ia
```

### Go

Requires Go `1.26.1` or newer (`go.mod` is authoritative):

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

### Install Script (Linux / macOS)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

### From Source

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia ./cmd/cortex-ia
```

## What Gets Installed

Every installed asset is embedded in the binary and copied
structure-preserving under the OpenCode config root — **33 native assets**
in total: the base config template, `AGENTS.md`, 5 sub-agents, 9 commands,
12 skills, and 5 plugins. Nothing is generated at install time, and
copied assets land byte-for-byte. Two deliberate exceptions: the base
`opencode.jsonc` is three-way merged (your keys and comments survive)
instead of replaced, and the `internal/assets/_shared/` contracts are
compile-time only — they are never installed.

| Asset kind | Source | Destination under `~/.config/opencode/` |
|------------|--------|------------------------------------------|
| Base config | `opencode.jsonc` template | `opencode.jsonc` (three-way merge, comments preserved) |
| System prompt | `AGENTS.md` | `AGENTS.md` |
| Sub-Agents | `agents/*.md` (5 roles) | `agents/<name>.md` |
| Commands | `commands/*.md` (9 commands) | `commands/<name>.md` |
| Skills | `skills/<name>/SKILL.md` (12 skills) | `skills/<name>/SKILL.md` |
| Plugins | `plugin/*.ts` (5 files) | `plugin/*.ts` |
| State & backups | — | `~/.cortex-ia/` (metadata, lock, backups) |

The full mapping rules (single-source layout, collision and path-safety
checks) are documented in [Architecture](docs/architecture.md).

## Managed MCP Services

`install` registers the default managed selection: **Cortex** and
**ForgeSpec**. **Context7** is available as a catalog preset but is not
selected by default.

| Preset | Kind | Launches | What it does |
|--------|------|----------|--------------|
| `cortex` | local | `cortex mcp --tools=agent` | Persistent memory with knowledge graph, FTS5, revision history |
| `forgespec` | local | `npx -y forgespec-mcp` | SDD contracts, task board, claims, file reservation |
| `context7` | local | `npx -y @upstash/context7-mcp` | Live framework and library documentation via MCP |

Every preset is registered in OpenCode's native local-server shape
(`{"type":"local","command":[...],"enabled":true}`) with the exact argv
above — no remote URLs, no hidden fields.

Custom servers are first-class:

```bash
# Catalog preset
cortex-ia mcp add forgespec --preset

# Custom local server from an exact command vector
cortex-ia mcp add my-server --local --env API_TOKEN=secret -- npx -y my-mcp

# Custom remote endpoint (headers use KEY=VALUE syntax)
cortex-ia mcp add my-remote --remote https://mcp.example.com/sse --header "Authorization=Bearer secret"

# Ownership listing (plain or sanitized JSON)
cortex-ia mcp list
cortex-ia mcp list --json

# Deregister (interactive confirmation required for the real run)
cortex-ia mcp remove my-server --dry-run
cortex-ia mcp remove my-server
```

Full reference — flags, statuses (`managed`, `unmanaged-equivalent`,
`conflict`, `absent`), secret handling, and ownership rules — lives in
[MCP Manager](docs/mcp.md).

## Transactional Safety

- **Plan before write.** `--dry-run` and the real run share the same plan;
  a dry run creates nothing, not even directories.
- **Digest-bound confirmation.** The confirmed run re-plans with identical
  options and must reproduce the previewed plan digest exactly. If anything
  drifts between preview and apply, the run aborts with a typed stale-plan
  error and nothing is written.
- **One writer per home.** Every mutating command holds a cross-process lock
  keyed by the canonical home (LockFileEx on Windows, flock on Unix) after
  confirmation and before any write; a concurrent writer gets a typed busy
  result, never a race.
- **Fail-closed conflicts.** Unmanaged files at a managed destination abort
  the run with nothing written. `--overwrite` must be given explicitly and
  confirmed on an interactive TTY; a verified backup is captured first.
- **Verified backups.** Every mutating run snapshots affected files under
  `~/.cortex-ia/backups/` and verifies the snapshot before the apply phase;
  a backup that cannot be proven complete fails the run. Snapshots
  accumulate — automatic deduplication and retention pruning are library
  capabilities not yet enabled in the product.
- **Rollback on failure.** If an apply phase fails mid-way, the transaction
  restores the pre-run state from the backup before reporting the error.
  Rollback itself fails closed: every manifest entry is validated for
  containment and duplicates before any write, restoration runs in reverse
  order under a journal, and success is reported only after the complete
  result is verified.
- **Recoverable journals.** `doctor` detects interrupted transactions and
  reports them as a degraded (recoverable) finding without exposing secrets.
  `cortex-ia recover list` is read-only; `cortex-ia recover <journal-id>`
  restores one journal only after you type its exact ID, and the receipt is
  PASS only after complete verified restoration. Corrupt, foreign-home, or
  drifted journals are rejected with typed errors and nothing written.
- **Ownership-accredited uninstall.** `uninstall` removes only files whose
  digest matches the installation metadata; anything unverifiable or
  co-owned is retained and reported, never guessed.
- **Destructive commands confirm.** `rollback`, `recover`, `uninstall`,
  `mcp remove`, and `--overwrite` require an interactive terminal and an
  explicit confirmation; piped or closed input always fails closed.

Details in [Safety & Recovery](docs/security.md).

## CLI Commands

```
cortex-ia                          Interactive TUI
cortex-ia install [--dry-run] [--overwrite]
                                   Install assets + default managed MCPs
cortex-ia sync [--dry-run] [--overwrite]
                                   Reconcile an installed home with the
                                   current embedded asset set
cortex-ia mcp add <name> (--preset | --local ... -- <cmd> | --remote <url>) [--dry-run]
cortex-ia mcp list [--json]        List managed MCP entries and ownership
cortex-ia mcp remove <name> [--dry-run]
cortex-ia doctor                   Read-only health report
cortex-ia rollback [backup-id]     Restore the recorded (or given) backup
cortex-ia recover [list]           List pending recovery journals (read-only)
cortex-ia recover <journal-id>     Restore one pending journal; typing its
                                    exact ID confirms the recovery
cortex-ia uninstall [--dry-run]    Remove the accredited installation
cortex-ia version | help
```

`sync` also removes stale artifacts from previous versions that the current
asset set no longer ships. Former multi-agent, persona, profile, model,
skill-registry, and SDD-compiler surfaces were removed and fail with an
explicit retired-surface error.

## Documentation

| Doc | Description |
|-----|-------------|
| [Quickstart](docs/quickstart.md) | Three-command setup |
| [Installation](docs/installation.md) | Install methods, prerequisites |
| [MCP Manager](docs/mcp.md) | Presets, custom local/remote servers, ownership |
| [Safety & Recovery](docs/security.md) | Backups, rollback, uninstall ownership, fail-closed rules |
| [SDD Workflow](docs/sdd-workflow.md) | The 9-phase workflow the installed skills implement |
| [Architecture](docs/architecture.md) | Codebase structure, asset mapping, testing |
| [Platforms](docs/platforms.md) | OS support matrix |
| [Changelog](CHANGELOG.md) | Version history |

## Prerequisites

- **Go 1.26.1+** — only for building from source (`go.mod` is authoritative)
- **Node.js 18+ with `npx`** — for the `forgespec` and `context7` local MCP presets
- **`cortex` on PATH** — for the `cortex` local MCP preset (`cortex mcp --tools=agent`):
  `go install github.com/lleontor705/cortex/cmd/cortex@latest` or `brew install lleontor705/tap/cortex`
- **OpenCode** — the only configured target

## Related Projects

| Project | Description |
|---------|-------------|
| [cortex](https://github.com/lleontor705/cortex) | Persistent memory MCP server (Go binary) |
| [forgespec-mcp](https://github.com/lleontor705/forgespec-mcp) | SDD contracts + task board + file reservation |
| [OpenCode](https://opencode.ai) | The AI coding agent cortex-ia configures |

## License

[MIT](LICENSE)
