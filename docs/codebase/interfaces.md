# Key Interfaces & Types

← [Codebase Guide](../CODEBASE-GUIDE.md)

The Go interfaces and types that define module boundaries in cortex-ia. This page is a reference contract index — what each type is, where it lives, and who consumes it. It does not explain the injection flow (see [mental-model.md](mental-model.md)) or coordination protocols (see [sdd-coordination.md](sdd-coordination.md)).

## Core Model Types

| Type | Package | Purpose | Consumers |
|------|---------|---------|-----------|
| `model.AgentID` | `internal/model` | Identifies a supported AI coding agent (12 constants) | `agents`, `catalog`, `pipeline`, `tui` |
| `model.ComponentID` | `internal/model` | Identifies an installable ecosystem component (11 constants) | `catalog`, `pipeline`, `state`, all `components/*` |
| `model.SkillID` | `internal/model` | Identifies an SDD or utility skill (27 constants) | `components/sdd`, `components/skills`, `skillregistry` (planned) |
| `model.MCPStrategy` | `internal/model` | Enum: how MCP server configs are written (SeparateFiles, MergeIntoSettings, ConfigFile, TOMLFile) | `agents`, `components/mcpinject`, all MCP components |
| `model.SystemPromptStrategy` | `internal/model` | Enum: how system prompt is managed (MarkdownSections, FileReplace, AppendToFile) | `agents`, `components/sdd`, `components/conventions` |
| `model.PresetID` | `internal/model` | Install preset (full, minimal, custom) | `catalog`, `pipeline`, `tui` |
| `model.Selection` | `internal/model` | User selection struct (agents, preset, components, persona, model assignments) | `pipeline`, `tui`, `app` |
| `model.ModelAssignments` | `internal/model` | Map of SDD skill name → Claude model alias | `components/sdd`, `app` |

## Agent Layer

| Interface/Type | Package | Purpose | Consumers |
|----------------|---------|---------|-----------|
| `agents.Adapter` | `internal/agents` | Core abstraction for AI agent integration (23 methods). Identity, detection, config paths, strategies, capabilities. | Every component; `pipeline` |
| `agents.Registry` | `internal/agents` | Adapter registry with insertion-order preservation. `Get(AgentID)` lookups. | `pipeline`, `app`, `tui`, `verify` |
| `agents.Registry.Get` | `internal/agents` | Returns the `Adapter` for an `AgentID` | Component injectors |

### `agents.Adapter` method groups

| Group | Methods |
|-------|---------|
| Identity | `Agent()`, `Tier()` |
| Detection | `Detect(homeDir)` |
| Config paths | `GlobalConfigDir`, `SystemPromptDir`, `SystemPromptFile`, `SkillsDir`, `SettingsPath`, `CommandsDir`, `SubAgentsDir`, `MCPConfigPath` |
| Strategies | `SystemPromptStrategy()`, `MCPStrategy()` |
| Capabilities | `SupportsSkills`, `SupportsSystemPrompt`, `SupportsMCP`, `SupportsSlashCommands`, `SupportsTaskDelegation`, `SupportsSubAgents`, `SupportsAutoInstall` |
| Auto-install | `InstallCommands(profile)` |

## Catalog Layer

| Function | Package | Purpose | Consumers |
|----------|---------|---------|-----------|
| `catalog.AllComponents()` | `internal/catalog` | All components in dependency order | `tui` (display) |
| `catalog.ComponentMap()` | `internal/catalog` | Components indexed by `ComponentID` | `catalog.ResolveDeps` |
| `catalog.ComponentsForPreset(preset)` | `internal/catalog` | Component IDs for a preset | `pipeline.Install` |
| `catalog.ResolveDeps(selected)` | `internal/catalog` | DFS topological sort; deps before dependents | `pipeline.Install` |
| `catalog.ComponentInfo` | `internal/catalog` | Struct: ID, Name, Description, Deps | catalog internals |

## Pipeline Layer

| Type/Function | Package | Purpose | Consumers |
|---------------|---------|---------|-----------|
| `pipeline.Step` | `internal/pipeline` | Interface: `Name() string`, `Run() error` | `runner.go` |
| `pipeline.RollbackStep` | `internal/pipeline` | Extends `Step` with `Rollback() error` | `RunStage` (reverse-order rollback) |
| `pipeline.StageResult` | `internal/pipeline` | Tracks completed/failed/error per stage | orchestrator |
| `pipeline.Orchestrator` | `internal/pipeline` | Two-stage struct: `Prepare`, `Apply`, `Policy` | `RunOrchestrator` |
| `pipeline.FailurePolicy` | `internal/pipeline` | `StopOnError` (default) / `ContinueOnError` | `Orchestrator` |
| `pipeline.InstallResult` | `internal/pipeline` | Outcome: BackupID, FilesChanged, ComponentsDone, Errors | `app` |
| `pipeline.Install(...)` | `internal/pipeline` | Full 2-stage install entry point | `app` |
| `pipeline.RunStage(steps)` | `internal/pipeline` | Sequential execution + reverse rollback | Prepare stage |
| `pipeline.RunParallelChains(chains)` | `internal/pipeline` | Per-agent parallel, components sequential within agent | Apply stage |

## Component Injection Contracts

| Function | Package | Purpose | Consumers |
|----------|---------|---------|-----------|
| `cortexcomp.Inject(homeDir, adapter)` | `internal/components/cortex` | Inject Cortex MCP config | `pipeline.buildInjectors` |
| `mailbox.Inject(homeDir, adapter)` | `internal/components/mailbox` | Inject Mailbox MCP config | `pipeline.buildInjectors` |
| `forgespeccomp.Inject(homeDir, adapter)` | `internal/components/forgespec` | Inject ForgeSpec MCP config | `pipeline.buildInjectors` |
| `context7.Inject(homeDir, adapter)` | `internal/components/context7` | Inject Context7 MCP config | `pipeline.buildInjectors` |
| `sdd.Inject(homeDir, adapter, assignments, strictTDD)` | `internal/components/sdd` | Inject orchestrator prompt + skills + commands + sub-agents | `pipeline.buildInjectors` |
| `skillscomp.Inject(homeDir, adapter, communitySkills)` | `internal/components/skills` | Inject non-SDD utility skills | `pipeline.buildInjectors` |
| `conventions.Inject(homeDir, adapter)` | `internal/components/conventions` | Inject cortex convention + protocol | `pipeline.buildInjectors` |
| `ggacomp.Inject(homeDir, agents)` | `internal/components/gga` | Inject GGA pre-commit hook config | `pipeline.buildInjectors` |
| `persona.Inject(homeDir, adapter, personaID)` | `internal/components/persona` | Inject communication-style persona | `pipeline.Install` (separate from component chain) |

Each injector returns a `{ Files []string; Changed bool; ... }`-shaped result. `pipeline` reads `Files` for change tracking.

## MCP Injection Engine

| Type/Function | Package | Purpose | Consumers |
|---------------|---------|---------|-----------|
| `mcpinject.ServerTemplates` | `internal/components/mcpinject` | Per-strategy JSON/TOML templates | All MCP components |
| `mcpinject.Inject(...)` | `internal/components/mcpinject` | 4-strategy dispatch (separate, merge, config, TOML) | MCP component `inject.go` files |

## File Merge Primitives

| Function | Package | Purpose | Consumers |
|----------|---------|---------|-----------|
| `filemerge.WriteFileAtomic(path, data, mode)` | `internal/components/filemerge` | Temp-write + rename (atomic) | All injectors |
| `filemerge.InjectSection(...)` | `internal/components/filemerge` | Marker-based markdown section injection | `sdd`, `conventions` |
| JSON deep-merge | `internal/components/filemerge` | `json_merge.go` with comment stripping | MCP components |
| TOML block upsert | `internal/components/filemerge` | `toml.go` | Codex MCP injection |

## Assets Access

| Function | Package | Purpose | Consumers |
|----------|---------|---------|-----------|
| `assets.FS` | `internal/assets` | `embed.FS` for `skills`, `generic`, `opencode`, `gga` | `sdd`, `conventions`, `gga`, `skills` |
| `assets.Read(path)` | `internal/assets` | Read embedded asset as string | injectors |
| `assets.ReadBytes(path)` | `internal/assets` | Read embedded asset as bytes | injectors |
| `assets.ListDir(path)` | `internal/assets` | List embedded directory entries | `sdd.FilesToBackup` |

## Invariants

- `agents.Adapter` is the ONLY agent-aware abstraction; components depend on it, never on concrete adapters.
- `pipeline.Step` / `RollbackStep` are the only execution units; everything in the pipeline implements one or both.
- All managed-file writes go through `filemerge` — never raw `os.WriteFile` on agent paths.
- Component injectors share a `{ Files []string; Changed bool }` result shape so `pipeline` can aggregate uniformly.
- `model` imports nothing from `internal/` — it is the shared vocabulary leaf package.
- `assets.FS` is read-only at runtime; all content is embedded at compile time via `//go:embed`.

## Contributor Checklist

- [ ] New agent capability? Add a method to `agents.Adapter`, implement it in all 12 adapters, and expose it via a `Supports*()` capability flag.
- [ ] New installable component? Define a `ComponentID`, add a `ComponentInfo` with deps, write an `Inject()` matching the result shape, and wire it in `pipeline.buildInjectors()`.
- [ ] New pipeline behavior? Implement `Step` (and `RollbackStep` if undoable); choose the right runner (`RunStage` for stop-on-error, `RunStageContinue` for collect-errors, `RunParallelChains` for per-agent concurrency).
- [ ] New embedded asset? Add under `internal/assets/<dir>/` and verify the `//go:embed` directive covers it.
- [ ] Adding a type shared across packages? Put it in `internal/model` — keep that package dependency-free.
- [ ] Changing an injector signature? Update every call site in `pipeline.buildInjectors()` and the matching `FilesToBackup` if one exists.

---

← Prev: [SDD Coordination](sdd-coordination.md) · Next: [Sync, State & Backup](sync-and-cloud.md) →
