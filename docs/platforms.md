# Supported Platforms

Release binaries are published for the platforms declared in
`.goreleaser.yaml`: **linux**, **darwin**, and **windows** (amd64/arm64 as
configured per release).

## Notes by Platform

- **Windows**: all destination writes use atomic file replacement and
  reject paths that traverse symlinks or reparse points. Case-insensitive
  destination collisions are detected and fail closed at mapping time, so
  two assets differing only by case can never overwrite each other. No
  symlink creation privilege is required for install, sync, or tests.
- **macOS**: case-insensitive destination collision detection applies for
  the same reason as Windows.
- **Linux**: standard behavior; case-sensitive filesystems get the strict
  collision check.

## Paths

All managed writes are confined to the declared roots beneath the user's home directory:

| Root | Contents |
|------|----------|
| `~/.config/opencode/` | Config (`opencode.jsonc`), `AGENTS.md`, agents, commands, plugins, themes, tui-plugins |
| `~/.agents/skills/` | Native skills (`<name>/SKILL.md`) |
| `~/.cortex-ia/opencode/` | Cortex-IA workflow roots: contracts, roles, overlays, quality, manifests, models, permissions |
| `~/.cortex-ia/` | Installation metadata, lock, MCP digests, backups |

The optional `claude` target writes Cortex-IA's MCP entry to `~/.claude.json`.

The transactional pipeline validates every destination against the layout
declaration before writing; absolute paths and traversal outside these
roots fail closed.

## Target AI Platforms

| Platform | Support Tier | Configuration Directory | Notes |
| :--- | :---: | :--- | :--- |
| **OpenCode** | **Primary (Native)** | `~/.config/opencode/` (`~/.agents/skills/`, `~/.cortex-ia/opencode/`) | Full SDD stack: 6 agents, 13 slash commands, 17 skills, 8 plugins, managed MCPs. |
| **Claude Code** | *Secondary (legacy)* | `~/.claude.json` | MCP-only: `--target claude` writes or removes Cortex's `mcpServers.cortex` entry. Installs no asset set. |

No additional platform targets are planned: the historical multi-platform adapter surface is retired and fails closed.

## Shells and Terminals

The TUI runs in any terminal Bubble Tea supports. Destructive CLI
operations (`--overwrite`, `mcp remove`, `rollback`, `uninstall`) require
an **interactive terminal** for their confirmation prompt; piped input
fails closed by design — scripts should use `--dry-run`.
