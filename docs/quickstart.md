# Quickstart Guide

Get up and running with **Cortex-IA** in three simple steps.

---

## 1. Install the Binary

```bash
# Via Go (recommended)
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest

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

### Option B: Automated Install
```bash
cortex-ia install
```

This transactionally installs the embedded asset set under `~/.config/opencode/`:
- `AGENTS.md` system prompt and 5 subagents (`orchestrator`, `investigate`, `planner`, `implement`, `reviewer`).
- SDD slash commands, skills, and Herdr delegation bridge plugin.
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

### B. Validate OpenSpec Proposals
```bash
openspec validate
```

### C. Check Multi-Agent Delegation
When OpenCode delegates tasks, watch them stream in real time in your Herdr terminal panes!
