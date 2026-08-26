# Installation

## Prerequisites

- **OpenCode** — supported target for AI agent orchestration.
- **Herdr** (optional) — for live pane splitting and multiplexed delegation.
- **Node.js 18+** — for OpenCode plugins runtime.
- **`cortex` on PATH** (optional) — for Cortex Knowledge Graph MCP server.

---

## Quick Install

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.ps1 | iex
```

### Linux & macOS (Bash)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

Both installers automatically:
1. Download and install the `cortex-ia` binary into your user bin directory.
2. Add the binary directory to your persistent `PATH`.
3. Set `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"` across shell profiles.
4. Execute `cortex-ia sync` to deploy all MCPs, Herdr/OpenCode plugins, and agent prompts.

---

## Alternative Methods

### Go Install

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
cortex-ia sync
```

### Homebrew (macOS / Linux)

```bash
brew install lleontor705/tap/cortex-ia
cortex-ia sync
```

### From Source

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia.exe ./cmd/cortex-ia
.\bin\cortex-ia.exe sync
```

---

## Verify Installation

```bash
cortex-ia version
cortex-ia herdr status
```

## First Run

```bash
cortex-ia            # Interactive Bubble Tea TUI
cortex-ia web        # Web Console (http://127.0.0.1:7331)
cortex-ia sync       # Reconcile all plugins, MCPs, and agents
```
