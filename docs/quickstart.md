# Quickstart

Three commands from a fresh machine to a fully wired cortex-ia ecosystem.

## 1. Install the binary

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

Or grab a release binary from [GitHub Releases](https://github.com/lleontor705/cortex-ia/releases) and drop it on `PATH`.

## 2. Detect what you already have

```bash
cortex-ia detect
```

Lists every supported agent (12 today: `claude-code`, `opencode`, `cursor`, `gemini-cli`, `vscode-copilot`, `codex`, `windsurf`, `antigravity`, `kilocode`, `kimi`, `kiro-ide`, `qwen-code`) and reports which ones have their config dirs and binaries on disk. Also reports runtime deps (`node`/`npx`, `git`, `cortex` MCP).

## 3. Install the ecosystem

Interactive (recommended for first-time users):

```bash
cortex-ia
```

This launches the Bubbletea TUI: pick agents → preset → persona → review → install.

Non-interactive equivalent:

```bash
cortex-ia install --preset full --persona professional --model-preset balanced
```

Add `--dry-run` to preview changes without touching disk. Add `--local` to read [`.cortex-ia.yaml`](configuration.md) from the project.

## 4. Verify

```bash
cortex-ia doctor   # 6 health checks
cortex-ia config   # show resolved configuration
cortex-ia list backups
```

If any check fails, `cortex-ia repair` re-applies from the lockfile. To roll back, `cortex-ia rollback`.

## What just happened

- `~/.cortex-ia/state.json` records the current selection
- `~/.cortex-ia/cortex-ia.lock` is the lockfile used by `repair`
- `~/.cortex-ia/skills/` holds the 19 SDD skills
- Each agent's config dir (`~/.claude/`, `~/.config/opencode/`, …) was patched non-destructively via `<!-- cortex-ia:* -->` markers, with a backup snapshot under `~/.cortex-ia/backups/`

## Where to go next

- [`docs/agents.md`](agents.md) — supported agents and per-agent config paths
- [`docs/components.md`](components.md) — every component and what it injects
- [`docs/sdd-workflow.md`](sdd-workflow.md) — the spec-driven loop
- [`docs/configuration.md`](configuration.md) — `.cortex-ia.yaml` and CLI flags
- [`docs/installation.md`](installation.md) — full installer reference
