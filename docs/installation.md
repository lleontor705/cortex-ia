# Installation

## Methods

### Go Install (recommended)

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

Requires Go 1.22+. Binary is placed in `$GOPATH/bin`.

### Homebrew (macOS/Linux)

```bash
brew install lleontor705/tap/cortex-ia
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/lleontor705/cortex-ia/releases):

| Platform | Architecture | File |
|----------|-------------|------|
| Linux | x86_64 | `cortex-ia_X.Y.Z_linux_amd64.tar.gz` |
| Linux | ARM64 | `cortex-ia_X.Y.Z_linux_arm64.tar.gz` |
| macOS | Intel | `cortex-ia_X.Y.Z_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `cortex-ia_X.Y.Z_darwin_arm64.tar.gz` |
| Windows | x86_64 | `cortex-ia_X.Y.Z_windows_amd64.zip` |
| Windows | ARM64 | `cortex-ia_X.Y.Z_windows_arm64.zip` |

### Install Script (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash

# Specific version
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash -s -- v0.1.0
```

### Docker

```bash
docker build -t cortex-ia .
docker run cortex-ia detect
```

## Prerequisites

### Required

- **Node.js 18+** with `npx` on PATH — required for npm-based MCP servers (forgespec-mcp, agent-mailbox-mcp, cli-orchestrator-mcp, @upstash/context7-mcp)
- **Cortex binary** — the persistent memory MCP server:
  ```bash
  go install github.com/lleontor705/cortex/cmd/cortex@latest
  # or
  brew install lleontor705/tap/cortex
  ```
- At least one supported AI coding agent installed

### Optional

- **Go 1.22+** — only needed if installing cortex-ia via `go install`
- **Docker** — only for containerized usage

## Verify Installation

```bash
# Check version
cortex-ia version

# Detect installed agents and system info
cortex-ia detect

# Preview what install would do
cortex-ia install --dry-run
```

## Platform Notes

### Windows

- Uses `winget` as package manager
- Agent config paths use `%APPDATA%` for VS Code and Windsurf
- Cortex data stored in `~/.cortex/`

### macOS

- Uses `brew` as package manager
- VS Code config at `~/Library/Application Support/Code/User/`
- Windsurf config at `~/Library/Application Support/Windsurf/User/`

### Linux

- Supports `apt` (Ubuntu/Debian), `pacman` (Arch), `dnf` (Fedora), and `brew`
- VS Code config at `~/.config/Code/User/` (respects `$XDG_CONFIG_HOME`)
