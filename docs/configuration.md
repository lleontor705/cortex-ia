# Configuration

## CLI Reference

```
cortex-ia                              # Launch interactive TUI
cortex-ia install [flags]              # Install ecosystem
cortex-ia detect                       # Show detected agents and system info
cortex-ia version                      # Show version
cortex-ia help                         # Show usage
```

### Install Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--agent <id>` | Target specific agent (repeatable) | `--agent claude-code --agent opencode` |
| `--preset <id>` | Installation preset | `--preset minimal` |
| `--dry-run` | Preview without making changes | `--dry-run` |

If no `--agent` is specified, cortex-ia auto-detects all installed agents.

If no `--preset` is specified, defaults to `full`.

## Interactive TUI

When run without arguments, cortex-ia launches a 6-screen interactive installer:

1. **Welcome** — Logo, ecosystem overview, press Enter to start
2. **Agents** — Multi-select from detected agents (↑↓ navigate, Space toggle, `a` select all)
3. **Preset** — Choose full or minimal (↑↓ navigate, Enter select)
4. **Review** — Shows selected agents + resolved components with descriptions
5. **Installing** — Progress while pipeline runs
6. **Complete** — Summary with file count, backup ID, and any warnings

Navigation: `Esc` goes back, `q` quits, `Enter` confirms.

## State

cortex-ia persists installation state at `~/.cortex-ia/state.json`:

```json
{
  "installed_agents": ["claude-code", "opencode", "gemini-cli"],
  "preset": "full",
  "components": ["cortex", "forgespec", "agent-mailbox", "cli-orchestrator", "context7", "conventions", "sdd", "skills"],
  "last_install": "2026-03-30T04:30:00Z",
  "last_backup_id": "20260330-043000",
  "version": "0.1.0"
}
```

Used by future `sync` and `upgrade` operations to know what was installed.

## Backup & Restore

### Automatic Backups

Every `install` creates a backup snapshot before modifying any files:

```
~/.cortex-ia/backups/20260330-043000/
├── manifest.json          # Metadata: source, timestamp, file list
└── files/                 # Copies of all files that will be modified
    ├── .claude/
    │   ├── CLAUDE.md
    │   └── mcp/cortex.json
    └── .config/opencode/
        └── opencode.json
```

### Manifest

```json
{
  "id": "20260330-043000",
  "created_at": "2026-03-30T04:30:00Z",
  "source": "install",
  "file_count": 5,
  "created_by_version": "0.1.0",
  "entries": [
    {"original_path": "/home/user/.claude/CLAUDE.md", "snapshot_path": "...", "existed": true, "mode": 420}
  ]
}
```

### Restore

Restore reverts all files to their pre-installation state:
- Files that existed are restored to their original content and permissions
- Files that didn't exist are removed

## Dependency Resolution

When selecting components, dependencies are automatically resolved:

```
Selected: [sdd]
Resolved: [cortex, forgespec, agent-mailbox, sdd]
```

The resolver uses topological sort to ensure dependencies are installed before dependents.

## Idempotency

cortex-ia is fully idempotent:
- MCP configs: atomic write with content comparison — skips if identical
- System prompts: marker-based injection replaces only managed sections
- Skills: written via atomic write — no change if content matches
- Running `install` twice produces zero file changes on the second run
