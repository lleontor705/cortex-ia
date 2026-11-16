# Quickstart Guide

Get up and running with **Cortex-IA** in three simple steps.

---

## 1. Install the Binary

```bash
# Via Go (recommended)
go install github.com/lleontor705/cortex-ia/v2/cmd/cortex-ia@latest

# Or via install script (Linux / macOS)
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

---

## 2. Initialize Your Environment

### Option A: Interactive Wizard (TUI)
Launch the rich terminal interface:
```bash
cortex-ia
```
Install and Uninstall open a host selection screen (OpenCode, Claude Code, agy), with the hosts whose CLI is detected pre-selected.

### Option B: Automated Install
```bash
cortex-ia install                          # OpenCode only
cortex-ia install --target opencode,claude # or: agy, all
```

This transactionally installs the embedded asset set across its declared OpenCode roots (`~/.config/opencode/` and `~/.agents/skills/`):
- `AGENTS.md` system prompt and 6 native roles (`orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `reviewer`).
- Slash commands, skills, and OpenCode plugins (the retired Herdr delegation bridge is no longer installed).
- Registers the **Cortex** knowledge graph MCP server.

---

## 3. Verify Health & Launch Web Dashboard

```bash
# 1. Check system diagnostics & path wiring
cortex-ia doctor

# 2. Launch the real-time operations web console
cortex-ia web --open
```

---

## 4. Explore the Core Workflows

### A. Manage Task Boards & Slices
```bash
# Create an initiative board
cortex-ia board create my-feature "Authentication & JWT"

# Add tasks with dependency chaining
cortex-ia work create auth-1.1 "Scaffold JWT token handler" --board my-feature
cortex-ia work create auth-1.2 "Add refresh token rotation" --board my-feature --depends auth-1.1
```

### B. Plan a Larger Change
Small changes run directly as one task. Feature work that touches three or more files, has unclear requirements, or spans sessions gets one plan file, `docs/plans/<board-id>.md`, whose acceptance checks are executable commands. See [Planning](../operations/planning.md).

### C. Watch Work State Natively
Every role controller executes natively inside OpenCode under Cortex-IA work authority; external delegation is retired. Watch boards, claims, leases, and review transitions in the local operations console:

```bash
cortex-ia web --open
```
