# Installation

## Prerequisites

- **OpenCode** — the only configured target.
- **Node.js 18+ with `npx`** — required by the `forgespec` and `context7`
  local MCP presets. Optional if you manage neither.
- **`cortex` on PATH** — required by the `cortex` local MCP preset, which
  launches `cortex mcp --tools=agent`. Optional if you do not manage that
  preset.
- Nothing else: the binary is self-contained; all assets are embedded.

## Methods

### Go

Requires Go `1.26.1` or newer (`go.mod` is authoritative):

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

### Homebrew (macOS / Linux)

```bash
brew install lleontor705/tap/cortex-ia
```

### Install script (Linux / macOS)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

### From source

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia ./cmd/cortex-ia
```

Release archives for supported platforms are attached to each
[release](https://github.com/lleontor705/cortex-ia/releases).

## Verify the Install

```bash
cortex-ia version
```

## First Run

```bash
cortex-ia            # interactive TUI
cortex-ia install    # or headless
```

Both paths install the same asset set with the same transactional
guarantees — see [Safety & Recovery](security.md). Uninstalling later is
ownership-accredited and safe: `cortex-ia uninstall --dry-run` previews
exactly what would be removed.

## Upgrading

Install the new binary and run:

```bash
cortex-ia sync
```

`sync` reconciles your home with the current embedded asset set: updated
files are rewritten, stale owned artifacts from previous versions are
removed, and conflicts fail closed exactly like `install`.
