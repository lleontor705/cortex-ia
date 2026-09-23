# Configuration

## CLI Reference

```
cortex-ia                              # Launch interactive TUI
cortex-ia install [--target <list>] [--dry-run] [--overwrite]
cortex-ia sync [--target <list>] [--dry-run] [--overwrite]
cortex-ia uninstall [--target <list>] [--dry-run]
cortex-ia mcp add <name> (--preset | --local ... -- <cmd> | --remote <url>) [--dry-run]
cortex-ia mcp list [--json]
cortex-ia mcp remove <name> [--dry-run]
cortex-ia doctor                       # Read-only installation health report
cortex-ia rollback [backup-id]         # Restore a backup
cortex-ia rollback list                # List available backups
cortex-ia recover [list]               # List pending recovery journals
cortex-ia update [--check]             # Check for / install the latest release
cortex-ia version                      # Show version
cortex-ia help                         # Show usage
```

The remaining commands — `snapshot`, `work`, `worktree`, `board`, `ledger`, `ui`, `openspec`, `web`, `doc`, `diagram`, and `report` — are part of the operations surface. See [`codebase/reference-map.md`](codebase/reference-map.md) and `cortex-ia help` for their subcommands. The former `delegate`, `herdr`, and `hook` subcommands are retired and fail closed.

### Install and sync flags

| Flag | Description |
|------|-------------|
| `--target <list>` | Comma-separated targets: `opencode`, `claude`, or `all`. Defaults to `opencode` |
| `--dry-run` | Preview the plan without writing; no backup is created |
| `--overwrite` | Replace unmanaged conflicting files (explicit; a verified backup is captured first) |

Install and sync preview the final plan — including every `--overwrite` replacement — and bind the real run to that exact plan digest. If anything drifts between preview and apply, the run aborts with a stale-plan error and nothing is written.

### Uninstall flags

| Flag | Description |
|------|-------------|
| `--target <list>` | Comma-separated targets: `opencode`, `claude`, or `all`. Defaults to `opencode` |
| `--dry-run` | Print the planned operations without writing |

Uninstall is destructive and requires an interactive terminal and an explicit confirmation. A snapshot tagged `BackupSourceUninstall` is captured before any change, so `cortex-ia rollback` restores the pre-uninstall state. See [`rollback.md`](rollback.md).

### Managed MCP entries

```bash
# Register a managed catalog preset
cortex-ia mcp add <name> --preset [--dry-run]

# Register a managed custom local server from an exact command vector
cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]

# Register a managed custom remote server endpoint (http/https)
cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]

cortex-ia mcp list [--json]
cortex-ia mcp remove <name> [--dry-run]
```

`--preset`, `--local`, and `--remote` are mutually exclusive: exactly one is required per `add`. `--env` and `--header` values reach the config file only and are never printed.

### Updates

```bash
cortex-ia update          # Check GitHub Releases and install the latest release
cortex-ia update --check  # Check only, without downloading or applying
```

## Environment Variables

cortex-ia requires no environment variables. Optional:

| Variable | Description |
|----------|-------------|
| `CORTEX_IA_HOME` | Override the state root (default `~/.cortex-ia/`). Must resolve to an absolute path; used by automation and tests. |
| `CORTEX_IA_DEBUG` | Enable debug tracing when set to `1`, `true`, or `yes`. Debug output goes to stderr and a file; stdout stays reserved for command receipts. The `--debug` flag enables the same tracing per invocation. |

The former `CORTEX_IA_AGY_AUTH` and `GEMINI_API_KEY` variables authenticated the retired external AGY leaf. They are dead: execution is native-only and neither variable is read.

## Interactive TUI

When run without arguments, cortex-ia launches the interactive installation dashboard. Navigation: `Esc` goes back, `q` quits, `Enter` confirms.

## State

cortex-ia persists installation state at `~/.cortex-ia/`:

| File | Purpose |
|------|---------|
| `state.json` | Installation metadata — the record `sync` reconciles against |
| `cortex-ia.lock` | Concrete written-file list with checksums |
| `install-status.json` | Crash-detection marker (ephemeral) |

## Health Checks

`cortex-ia doctor` runs a read-only assessment of the installed home: artifact presence and drift, detected coding CLIs, the resolved OpenCode root and selection, managed MCP entries, and any unknown MCPs. It prints a verdict and exits non-zero when the verdict is degraded or blocked.

## Backup & Restore

### Automatic backups

Every `install`, `sync`, and `uninstall` creates a snapshot under `~/.cortex-ia/backups/`:

```
~/.cortex-ia/backups/
├── manifest.json
└── files/
    └── (copies of all files that will be modified)
```

### Commands

```bash
cortex-ia rollback list        # list all backups newest-first
cortex-ia rollback             # restore the most recent backup
cortex-ia rollback <backup-id> # restore a specific backup
```

### Retention

Backups carry two optional manifest fields (both `omitempty`, so legacy backups still load):

| Field | Purpose |
|-------|---------|
| `pinned` | Excludes the backup from pruning. Reserved for internally retained snapshots. |
| `checksum` | SHA-256 over the snapshot inputs. Used by `IsDuplicate` to skip duplicate backups. |

Default retention is the **5 most recent unpinned backups** (`backup.DefaultRetentionCount`). Pruning runs at the end of `install` / `sync` so the backup directory does not grow without bound.

## Dependency Resolution

The installer resolves components with Kahn's topological sort and parallel group detection:

```
Level 0 (parallel): cortex, context7, skills, built-in work control
Level 1 (after cortex): conventions
Level 2 (after cortex + built-in work control): sdd
```

## Idempotency

cortex-ia is idempotent:

- MCP configs: atomic write with content comparison — skipped when identical
- System prompts: marker-based injection replaces only managed sections
- Skills: atomic write — no change when content matches
- Running `install` twice produces zero file changes on the second run
