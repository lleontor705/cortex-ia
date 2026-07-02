# Mental Model

← [Codebase Guide](../CODEBASE-GUIDE.md)

How cortex-ia works end-to-end: a user selects AI coding agents and ecosystem components, the pipeline injects configs into agent-specific paths, and managed files are written to disk with backup and rollback. This page covers the big-picture data flow only — for per-directory detail see [repository-map.md](repository-map.md), for module contracts see [interfaces.md](interfaces.md).

## End-to-End Flow

```
 ┌─────────────┐    ┌──────────────────┐    ┌─────────────────────────┐
 │   User /    │    │   catalog        │    │   pipeline              │
 │   TUI       │───▶│   ResolveDeps()  │───▶│   2-stage:              │
 │  selection  │    │   (dep order)    │    │   Prepare → Apply       │
 └─────────────┘    └──────────────────┘    └────────────┬────────────┘
                                                       │
                          ┌────────────────────────────┴────────────────────┐
                          │  one sequential chain PER agent (run parallel)   │
                          │                                                  │
                          │  adapter.X()  ──▶  component.Inject()  ──▶  files│
                          │     ▲                                  ▲          │
                          │     │ paths &                          │ injects  │
                          │     │ strategies                        │ via      │
                          │     │                                  │ filemerge│
                          └──────────────────────────────────────────────────┘
                                                       │
                          ┌────────────────────────────┴────────────────────┐
                          │  state.Save + lockfile   backup manifest         │
                          │  (rollback available on failure)                 │
                          └──────────────────────────────────────────────────┘
```

## Core Concepts

| Concept | Package | What it is |
|---------|---------|------------|
| Adapter | `internal/agents` | Per-agent implementation of a 23-method interface. Knows paths + strategies; no agent-specific switches in component code. |
| Component | `internal/components/*` | A self-contained injector. Each writes its slice of config (MCP servers, skills, prompts, hooks) using the adapter for paths. |
| ComponentMap | `internal/catalog` | Registry of installable components indexed by `model.ComponentID`, with dependency declarations. |
| Injector | `pipeline.buildInjectors` | Ordered list of component injectors built per agent; filtered by the resolved component set. |
| Strategy | `internal/model` | `MCPStrategy` and `SystemPromptStrategy` enums. The adapter picks HOW content is injected; the component never branches on agent. |
| Golden files | `testdata/golden/` | Expected output fixtures for component tests. Regenerate with `go test -update ./internal/components/...`. |

## Component Map (14 packages, 11 installable IDs)

| Component package | ComponentID | MCP server? | Purpose |
|-------------------|-------------|-------------|---------|
| `components/cortex` | `cortex` | Yes (Go binary) | Persistent memory + knowledge graph (19 tools) |
| `components/mailbox` | `agent-mailbox` | Yes (npm) | Messaging, A2A tasks, resource leases, DLQ (26 tools) |
| `components/forgespec` | `forgespec` | Yes (npm) | SDD contracts, task boards, file reservations (15 tools) |
| `components/context7` | `context7` | Yes (npm/remote) | Live framework/library docs via MCP |
| `components/sdd` | `sdd` | — | SDD orchestrator prompt + 19 skills + commands + sub-agents |
| `components/skills` | `skills` | — | Non-SDD utility skills injection |
| `components/conventions` | `conventions` | — | Cortex convention + memory protocol |
| `components/gga` | `gga` | — | Guardian Angel pre-commit code review hook |
| `components/persona` | `persona` | — | Communication-style persona injection |
| `components/permissions` | `permissions` | — | Permissions component |
| `components/theme` | `theme` | — | Terminal theme |
| `components/mcpinject` | — | — | Shared MCP injection engine (library, not installed) |
| `components/filemerge` | — | — | File-operation primitives (library, not installed) |
| `components/uninstall` | — | — | Uninstall command support (not installed) |

## Dependency Resolution

`catalog.ResolveDeps()` performs a DFS topological sort so dependencies install before dependents.

| Component | Depends on |
|-----------|------------|
| `conventions` | `cortex` |
| `sdd` | `cortex`, `forgespec`, `agent-mailbox` |
| all others | none |

Presets:
- **full** — all components in `AllComponents()`.
- **minimal** — `cortex`, `forgespec`, `context7`, `sdd` (mailbox + conventions auto-pulled via deps).

## Pipeline Stages

| Stage | Runner | Failure behavior |
|-------|--------|------------------|
| Prepare | `RunStage` (sequential) | Stop on error; rollback completed steps in reverse order. |
| Apply | `RunParallelChains` (per-agent parallel, components sequential within agent) | `ContinueOnError` — collects errors, continues remaining agents/components. |

Agents run in parallel (different config dirs). Within an agent, components run sequentially (same config files). A per-agent chain is built by filtering `buildInjectors()` against the resolved component set.

## Invariants

- Components NEVER branch on `AgentID`. Path and strategy logic lives in the adapter.
- Content outside `<!-- cortex-ia:ID --> ... <!-- /cortex-ia:ID -->` markers is never modified.
- Marker-based injection is idempotent — re-running replaces only the marked block.
- Prepare failure always rolls back; Apply failure never rolls back managed files (leaves them in place, marks install "in-progress" for `doctor` to detect).
- Dependencies are installed before dependents within every chain.
- Shared files (orchestrator prompt, shared skills) are written once via `sync.Once` to avoid file-lock races across parallel agents (Windows safety).

## Contributor Checklist

- [ ] Adding an agent? Create `internal/agents/<name>/adapter.go`, implement the full `Adapter` interface, register in `factory.go`. No component changes needed.
- [ ] Adding a component? Create `internal/components/<name>/`, add a `ComponentID` constant in `model/types.go`, add a `ComponentInfo` entry in `catalog/components.go`, and wire an `injectorEntry` in `pipeline.buildInjectors()`.
- [ ] Adding an MCP server? Add per-strategy templates in a `config.go` + an `inject.go` that delegates to `mcpinject.Inject()`.
- [ ] Changing injection logic? Use `filemerge` primitives (`WriteFileAtomic`, `InjectSection`) — never raw `os.WriteFile` on managed paths.
- [ ] After changing expected outputs, regenerate golden files: `go test -update ./internal/components/...`.
- [ ] Parallel-write safety: shared, cross-agent files must go through the `sync.Once` guard in the SDD injector or an equivalent single-writer pattern.

---

← Prev: (start) · Next: [Repository Map](repository-map.md) →
