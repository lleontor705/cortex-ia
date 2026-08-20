# Quickstart

Three commands from a fresh machine to a fully configured OpenCode.

## 1. Get the binary

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

(Or use Homebrew / the install script — see [Installation](installation.md).)

## 2. Install

```bash
cortex-ia install
```

This copies the embedded asset set under `~/.config/opencode/` — the base
config, `AGENTS.md`, 14 agents, 9 commands, 21 skills, and the plugin —
and registers the default managed MCP selection (`cortex`, `forgespec`).
A verified backup is captured first; nothing unmanaged is ever replaced.

Not sure yet? Preview the exact plan with zero writes:

```bash
cortex-ia install --dry-run
```

## 3. Verify

```bash
cortex-ia doctor
```

The read-only report shows artifact status counts, managed MCP status, and
health findings. It exits non-zero on a degraded or blocked verdict.

## Next Steps

- Add MCPs: `cortex-ia mcp add context7 --preset`, or custom
  `--local` / `--remote` servers — see [MCP Manager](mcp.md).
- Refresh after a binary upgrade: `cortex-ia sync`.
- Understand the safety model: [Safety & Recovery](security.md).
- Learn the workflow the installed skills implement:
  [SDD Workflow](sdd-workflow.md).
