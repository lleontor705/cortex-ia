# Architecture

## Project Structure

```
cortex-ia/
├── cmd/cortex-ia/
│   └── main.go                    # Entry point, ldflags version
├── internal/
│   ├── app/
│   │   ├── app.go                 # CLI dispatch + TUI launch
│   │   └── version.go             # Version resolution
│   ├── model/
│   │   ├── types.go               # AgentID, ComponentID, SkillID, strategies
│   │   └── selection.go           # User selection struct
│   ├── agents/                    # 8 agent adapters
│   │   ├── interface.go           # Adapter interface (23 methods)
│   │   ├── registry.go            # Registry with insertion order
│   │   ├── factory.go             # Default registry builder
│   │   ├── errors.go              # Sentinel errors
│   │   ├── claude/adapter.go
│   │   ├── opencode/adapter.go
│   │   ├── gemini/adapter.go
│   │   ├── cursor/adapter.go
│   │   ├── vscode/adapter.go
│   │   ├── codex/adapter.go
│   │   ├── windsurf/adapter.go
│   │   └── antigravity/adapter.go
│   ├── catalog/
│   │   ├── components.go          # Component definitions, presets, ResolveDeps()
│   │   └── skills.go              # Skill ID lists
│   ├── components/
│   │   ├── mcpinject/             # Shared MCP injection engine
│   │   │   └── mcpinject.go       # ServerTemplates + 4-strategy dispatch
│   │   ├── cortex/                # Cortex MCP (Go binary)
│   │   ├── orchestrator/          # CLI Orchestrator MCP (npm)
│   │   ├── mailbox/               # Agent Mailbox MCP (npm)
│   │   ├── forgespec/             # ForgeSpec MCP (npm)
│   │   ├── context7/              # Context7 MCP (npm/remote)
│   │   ├── sdd/                   # SDD workflow injection
│   │   │   └── inject.go          # Orchestrator + skills + commands + sub-agents
│   │   ├── skills/                # Non-SDD skill injection
│   │   │   └── inject.go          # With SDD skip logic
│   │   ├── conventions/           # Cortex convention + protocol
│   │   │   └── inject.go
│   │   └── filemerge/             # File operation primitives
│   │       ├── section.go         # Marker-based markdown injection
│   │       ├── json_merge.go      # Deep JSON merge + comment stripping
│   │       ├── toml.go            # TOML block upsert
│   │       └── writer.go          # Atomic file write (temp + rename)
│   ├── assets/                    # Embedded content (go:embed)
│   │   ├── assets.go              # embed.FS + Read/ListDir
│   │   ├── skills/                # 19 SKILL.md files + _shared/
│   │   ├── generic/               # Orchestrator prompts + cortex protocol
│   │   └── opencode/commands/     # 10 slash commands
│   ├── pipeline/
│   │   ├── pipeline.go            # Full install pipeline (resolve → backup → inject → state)
│   │   ├── runner.go              # Sequential step execution with rollback
│   │   └── types.go               # Step, RollbackStep interfaces
│   ├── backup/
│   │   ├── snapshot.go            # File snapshotter
│   │   ├── manifest.go            # JSON manifest read/write
│   │   └── restore.go             # Restore from snapshot
│   ├── state/
│   │   └── state.go               # ~/.cortex-ia/state.json persistence
│   ├── system/
│   │   └── detect.go              # OS/platform/package manager detection
│   └── tui/
│       ├── tui.go                 # Bubbletea model + 6 screens
│       └── styles/theme.go        # Colors and layout styles
├── docs/                          # Documentation
├── scripts/install.sh             # Curl-pipe installer
├── .github/workflows/             # CI, PR checks, release, stale
├── .goreleaser.yaml               # Cross-platform release config
├── .golangci.yml                  # Lint config
├── Makefile                       # Build, test, lint, coverage targets
├── Dockerfile                     # Multi-stage Alpine build
└── go.mod
```

## Key Patterns

### Adapter Pattern

Every agent implements the `Adapter` interface (23 methods). Components query the adapter for paths and strategies — no agent-specific switch statements in component code.

```go
type Adapter interface {
    Agent() model.AgentID
    Detect(homeDir string) (installed, binaryPath, configPath, configFound, err)
    GlobalConfigDir(homeDir string) string
    SystemPromptFile(homeDir string) string
    SkillsDir(homeDir string) string
    MCPStrategy() model.MCPStrategy
    MCPConfigPath(homeDir string, serverName string) string
    SupportsTaskDelegation() bool
    SupportsSubAgents() bool
    // ... 14 more methods
}
```

Adding a new agent: create `internal/agents/<name>/adapter.go`, implement the interface, register in `factory.go`.

### Strategy Dispatch

MCP injection uses `ServerTemplates` to define per-strategy JSON/TOML templates:

```go
type ServerTemplates struct {
    Name                   string
    SeparateFileJSON       []byte  // Claude Code
    DefaultOverlayJSON     []byte  // Cursor, Windsurf, Gemini
    OpenCodeOverlayJSON    []byte  // OpenCode (different key structure)
    VSCodeOverlayJSON      []byte  // VS Code (uses "servers" key)
    TOMLCommand, TOMLArgs  string, []string  // Codex
}
```

Adding a new MCP server: create `config.go` with templates + `inject.go` that delegates to `mcpinject.Inject()`.

### Marker-Based Injection

System prompt injection uses HTML comment markers:
```
<!-- cortex-ia:sdd-orchestrator -->
{injected content}
<!-- /cortex-ia:sdd-orchestrator -->
```

Content outside markers is never touched. Idempotent: re-running replaces content between markers.

### Embedded Assets

All skills, prompts, and commands are embedded at compile time:
```go
//go:embed all:skills all:generic all:opencode
var FS embed.FS
```

No external files needed at runtime — everything ships in the binary.

## Build & Test

```bash
make build           # Build binary → bin/cortex-ia
make test            # Run all tests (60+)
make test-coverage   # HTML coverage report
make lint            # golangci-lint (errcheck, govet, staticcheck, unused, ineffassign)
make fmt             # gofmt -s -w
make tidy            # go mod tidy
make security        # govulncheck
make docker-build    # Build Docker image
make install         # Install to $GOPATH/bin
```

## Testing

- **16 test packages** with 60+ tests
- Tests use `t.TempDir()` for isolation — no real agent configs modified
- MCP injection tests verify all 4 strategies (separate, merge, config, TOML)
- Catalog tests verify dependency resolution and preset expansion
- Pipeline tests verify step execution and rollback
- Filemerge tests cover marker injection, JSON merge, TOML upsert, atomic write

## Dependencies

| Package | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | Terminal UI framework |
| `charmbracelet/lipgloss` | Terminal styling |

Zero external dependencies beyond the Go standard library + Bubbletea/Lipgloss.
