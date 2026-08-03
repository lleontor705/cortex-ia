# Architecture

cortex-ia is a capability-aware workflow **compiler and installer**, not a resident execution engine. It lowers canonical Go workflow and quality-policy IR into deterministic target assets and manifests, then plans, diagnoses, backs up, installs, and restores those assets. External agent runtimes execute them.

## Compiler and Installer Boundary

```text
Canonical WorkflowIR + QualityPolicy + capability catalog + operator opt-ins
  -> validate -> resolve capabilities -> select profile -> render bundle/manifests
  -> static conformance -> immutable dry-run plan -> doctor -> backup -> apply
  -> post-install doctor; explicit receipt-backed rollback
```

The domain packages do not launch workers or own workflow state. Runtime syntax, probes, filesystems, and service configuration sit behind volatile adapter boundaries. Installation must consume the exact plan disclosed by dry-run; it must not recompute effects after approval.

| Boundary | cortex-ia responsibility | Explicit limit |
|----------|--------------------------|----------------|
| Compile | Validate IR, resolve capability facts, select a profile, render deterministic assets | Does not execute roles or tasks |
| Diagnose | Check versions, evidence freshness, bindings, hashes, ownership, permissions, manifests, and profile qualification | A finding is evidence, not runtime control |
| Install | Preserve customizations, create verified backups, apply owned assets atomically | Does not mutate external service authority |
| Rollback | Restore selected managed assets/configuration and report conflicts | No runtime-session or in-flight task-state migration |

## Capability Profiles and Enforcement

Profile selection is deterministic and conservative:

1. `portable-sequential` requires no delegation.
2. `portable-flat` requires fresh, proven direct-child delegation and assumes neither nesting nor runtime DAG scheduling.
3. `native-advanced` requires every requested native capability to be qualified. Experimental native capabilities always require explicit operator opt-in.

Capabilities resolve to `native`, `emulated`, `advisory`, or `unsupported`. Enforcement is recorded independently as `runtime`, `hook`, `mcp`, `prompt`, or `none`. Documentation or prompt text cannot prove runtime enforcement. Stale evidence cannot upgrade a profile, and unsupported required semantics block generation or installation.

Each bundle records normalized semantics plus machine/human security and degradation disclosures: versions, fingerprint, target/profile, evidence and freshness, resolutions, enforcement, substitutions, permissions, trust boundaries, opaque secret references, external service requirements, hashes, findings, and degradations. Target-specific asset paths may vary; the conformance index and goldens are the source of truth for advertised fixtures.

## Service Ownership

| Service | Authoritative state | cortex-ia relationship |
|---------|---------------------|------------------------|
| ForgeSpec | SDD contracts; task dependencies, readiness, claims, and status; file reservations | Configure and validate a versioned external dependency. Transactional task claim/expansion remains an explicit upstream ForgeSpec capability. |
| Cortex | Durable memory, evidence, provenance, and knowledge relationships | Configure bindings and preserve references; never create a second memory authority. |
| Runtime-native dispatch | Child execution transport only | Never durable task authority |

Cross-service IDs are references, not copied mutable records. Runtime-native state may accelerate execution but remains non-authoritative unless an explicit ForgeSpec binding maps it.

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
│   ├── agents/                    # 4 agent adapters (claude, opencode, vscode, codex)
│   │   ├── interface.go           # Adapter interface (23 methods)
│   │   ├── registry.go            # Registry with insertion order
│   │   ├── factory.go             # Default registry builder
│   │   ├── errors.go              # Sentinel errors
│   │   ├── claude/adapter.go
│   │   ├── opencode/adapter.go
│   │   ├── vscode/adapter.go
│   │   ├── codex/adapter.go
│   ├── catalog/
│   │   ├── components.go          # Component definitions, presets, ResolveDeps()
│   │   └── skills.go              # Skill ID lists
│   ├── components/
│   │   ├── mcpinject/             # Shared MCP injection engine
│   │   │   └── mcpinject.go       # ServerTemplates + 4-strategy dispatch
│   │   ├── cortex/                # Cortex MCP (Go binary)
│   │   ├── forgespec/             # ForgeSpec MCP (npm)
│   │   ├── context7/              # Context7 MCP (npm/remote)
│   │   ├── sdd/                   # Workflow IR, compiler, profiles, renderers,
│   │   │                          # manifests, conformance, install, qualification
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
│   │   ├── skills/                # 28 SKILL.md files + _shared/
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
    DefaultOverlayJSON     []byte  // Default overlay for supported agents
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

### Conformance and Qualification

- Static CI covers schema validation, profile selection, deterministic renderer goldens, semantic equivalence, manifest disclosure, ownership/merge behavior, doctor findings, backup, and rollback.
- Every supported target has a sequential-compatible golden. Stronger entries in `internal/components/sdd/renderers/testdata/conformance/index.golden.json` advertise only the exact tested fixture.
- Credentialed runtime qualification is separate, explicit opt-in, budgeted, redacted, and attributable to runtime/version/adapter/model/workflow/schema/profile/trial. Fewer than three valid trials, flakes, missing credentials, or budget exhaustion are inconclusive—not passes.
- Static compiler conformance and a credentialed runtime sample do not establish universal behavior for every model, version, host, or future runtime.

## Quality Policy

Planning depth is selected from observable behavior, risk, reversibility, trust boundaries, dependency breadth, migration impact, evidence needs, and test capability; model confidence cannot waive required phases. Vertical RED/GREEN/REFACTOR evidence is required only when a deterministic focused runner, writable tests, and baseline evidence exist. Otherwise the work records a recognized exception and compensating evidence.

Gherkin is reserved for stakeholder-visible generator, installation, diagnostic, degradation, and rollback behavior where executable examples add value. Architecture checks enforce declared dependency principles rather than fixed folder shapes. Mutation starts report-only and requires a green baseline, coverage, pinned tooling, attributable rules, and budgets. Property/fuzz tests apply to meaningful invariants or untrusted inputs. Missing capability, timeout, flakes, cancellation, or exhausted budgets remain `inconclusive` or `degraded`.

## Migration and Rollback

Major-version cutover regenerates owned workflow assets and a deterministic asset manifest. Pre-doctor and static conformance run before the immutable plan is applied. Ownership sidecars and merge bases preserve non-overlapping user customization; overlapping edits block rather than overwrite. A verified backup precedes the first mutation, and explicit rollback restores the selected managed bundle while preserving conflict-free unmanaged changes. Active sessions receive a reload warning only—runtime state is never imported, rewritten, or migrated.

## Dependencies

| Package | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | Terminal UI framework |
| `charmbracelet/lipgloss` | Terminal styling |

Zero external dependencies beyond the Go standard library + Bubbletea/Lipgloss.
