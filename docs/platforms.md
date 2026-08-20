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

All writes are confined to two roots beneath the user's home directory:

| Root | Contents |
|------|----------|
| `~/.config/opencode/` | Config, `AGENTS.md`, agents, commands, skills, plugin |
| `~/.cortex-ia/` | Installation metadata, lock, MCP digests, backups |

The transactional pipeline validates every destination against the layout
declaration before writing; absolute paths and traversal outside these
roots fail closed.

## Target AI Platforms

| Platform | Support Tier | Configuration Directory | Notes |
| :--- | :---: | :--- | :--- |
| **OpenCode** | **Active (Native)** | `~/.config/opencode/` | Full SDD stack: 5 sub-agents, 9 commands, 12 skills, 5 plugins, managed MCPs. |
| **Google Antigravity** | *Roadmap* | `~/.gemini/antigravity/` | Native rules, custom skills, sidecar orchestration. |
| **Claude CLI** | *Roadmap* | `~/.claude/` | Native prompts, tool definitions, stdio MCPs. |

## Shells and Terminals

The TUI runs in any terminal Bubble Tea supports. Destructive CLI
operations (`--overwrite`, `mcp remove`, `rollback`, `uninstall`) require
an **interactive terminal** for their confirmation prompt; piped input
fails closed by design — scripts should use `--dry-run`.
