# Project & Extension Guide

← [Codebase Guide](../CODEBASE-GUIDE.md)

How to extend cortex-ia: adding a new agent adapter, a new skill, a new component, or a new CLI subcommand. This page is a recipe reference with exact files to touch and patterns to follow — it does not explain the adapter contract internals (see [interfaces.md](interfaces.md)) or the pipeline mechanics (see [mental-model.md](mental-model.md)).

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| Adding an agent adapter | ✅ | — |
| Adding a skill | ✅ | — |
| Adding a component | ✅ | — |
| Adding a CLI subcommand | ✅ | — |
| Adapter interface contract details | — | [interfaces.md](interfaces.md) |
| Pipeline execution internals | — | [mental-model.md](mental-model.md) |

## Extension Types

| Extension Type | Files to Touch | Pattern to Follow |
|---------------|---------------|-------------------|
| Agent adapter | `internal/agents/<name>/adapter.go`, `internal/agents/factory.go` | Copy `internal/agents/claude/adapter.go`; implement `Adapter` (23 methods); register in `factory.go` (insertion order preserved) |
| Skill | `internal/assets/skills/<name>/SKILL.md`, `internal/model/types.go` (`SkillID` constant), `internal/catalog/skills.go` | Copy an existing `SKILL.md`; add `SkillID` const; add catalog entry |
| Component | `internal/catalog/components.go` (`catalog.ComponentMap`), `pipeline buildInjectors` | Add `ComponentMap` entry + dependency declaration; wire injector into pipeline |
| CLI subcommand | `internal/app/app.go` | Add case to `runCLI` switch; add help text in `printHelp()` |

## Adding an Agent Adapter

Cortex-ia supports 12 agents. To add a 13th:

### Files

| File | Action |
|------|--------|
| `internal/agents/<name>/adapter.go` | **Create.** Implement all 23 `Adapter` interface methods. |
| `internal/agents/factory.go` | **Modify.** Register the new adapter in the default registry. Insertion order is preserved. |
| `internal/model/types.go` | **Modify.** Add the `AgentID` constant. |

### The Adapter Interface (23 methods)

The `Adapter` interface (`internal/agents/interface.go`) defines 23 methods covering:

| Category | Methods cover |
|----------|--------------|
| Identity | Agent name, ID, binary, config dir |
| System prompt | Path, read, write, strategy |
| Skills | Skills dir, install |
| MCP | Config path, strategy, read, write |
| Capabilities | Task delegation support, sub-agent support, slash commands |
| Detection | Is the agent installed on this system |

### Recipe

```
1. mkdir internal/agents/<name>/
2. Copy internal/agents/claude/adapter.go → internal/agents/<name>/adapter.go
3. Replace all claude-specific values with <name> specifics
4. Implement all 23 Adapter methods
5. Add AgentID constant in internal/model/types.go
6. Register in internal/agents/factory.go (appends to registry in order)
7. Add docs entry in docs/AGENTS.md
8. Run make check — all tests must pass
```

### Current Agents (12)

`claude`, `opencode`, `gemini`, `cursor`, `vscode`, `codex`, `windsurf`, `antigravity`, `kilocode`, `kimi`, `kiro`, `qwen`.

## Adding a Skill

Skills are markdown `SKILL.md` files embedded into the binary.

### Files

| File | Action |
|------|--------|
| `internal/assets/skills/<name>/SKILL.md` | **Create.** Skill instructions with YAML frontmatter. |
| `internal/model/types.go` | **Modify.** Add `SkillID` constant. |
| `internal/catalog/skills.go` | **Modify.** Add catalog entry mapping SkillID to metadata. |

### Recipe

```
1. mkdir internal/assets/skills/<name>/
2. Create SKILL.md with frontmatter (name, description, trigger, license, metadata)
3. Add SkillID constant in internal/model/types.go
4. Add entry in internal/catalog/skills.go
5. Verify the //go:embed directive covers skills/ (it uses all:skills)
6. Run make check
```

### Skill Count

~27 skills. Check `internal/catalog/skills.go` for the authoritative list.

## Adding a Component

Components are injectable ecosystem pieces (MCP servers, hooks, personas, themes, etc.). 14 components exist in `catalog.ComponentMap`.

### Files

| File | Action |
|------|--------|
| `internal/catalog/components.go` | **Modify.** Add `catalog.ComponentMap` entry + dependency declaration. |
| `pipeline buildInjectors` (in `internal/pipeline/`) | **Modify.** Wire the component's injector into the pipeline. |
| `internal/components/<name>/` | **Create.** Component logic package (if it has custom injection logic). |

### Recipe

```
1. Add ComponentID constant in internal/model/types.go
2. Add entry in internal/catalog/components.go ComponentMap
3. Declare dependencies (if any) via ResolveDeps topology
4. Create internal/components/<name>/ injector package if needed
5. Wire injector into pipeline buildInjectors
6. Run make check
```

### Current Components (14)

Registered in `catalog.ComponentMap`. Includes MCP servers (cortex, mailbox, forgespec, context7), SDD workflow, skills, conventions, GGA, persona, permissions, theme, uninstall, and others.

## Adding a CLI Subcommand

`internal/app/app.go` dispatches CLI subcommands. 18 subcommands exist.

### Files

| File | Action |
|------|--------|
| `internal/app/app.go` | **Modify.** Add case to `runCLI` switch; add help text to `printHelp()`. |

### Existing Subcommands

`install`, `sync`, `detect`, `doctor`/`verify`, `repair`, `rollback`, `uninstall`, `gga`, `profiles`, `agent-builder`, `update`, `config`, `list`, `init`, `skill`, `auto-install` (+ help/version).

### Recipe

```
1. Add case to runCLI switch in internal/app/app.go
2. Implement the handler (inline or delegate to a package)
3. Add usage text to printHelp()
4. Add golden test if output is deterministic
5. Run make check
```

## Invariants

- Every agent adapter MUST implement all 23 `Adapter` interface methods — partial implementation does not compile.
- `factory.go` registry preserves insertion order — deterministic agent listing.
- Every skill MUST have a `SkillID` constant and a catalog entry — orphans are invisible to the system.
- Every component MUST have a `ComponentMap` entry and a pipeline injector — missing either breaks install.
- Every CLI subcommand MUST have help text in `printHelp()` — undocumented commands violate the CLI contract.
- All embedded assets live under `internal/assets/` and are covered by the `//go:embed` directive.

## Contributor Checklist

- [ ] New agent? Implement 23 methods, add `AgentID` const, register in `factory.go`, add `docs/AGENTS.md` entry.
- [ ] New skill? Create `SKILL.md`, add `SkillID` const, add `catalog/skills.go` entry.
- [ ] New component? Add `ComponentMap` entry, declare deps, wire pipeline injector.
- [ ] New subcommand? Add `runCLI` case + `printHelp()` text.
- [ ] After any extension: run `make check` (lint + test + vet) — it must pass.
- [ ] After adding an agent: update the agent count references in docs and the welcome hub detection list.

---

← Prev: [Integrations](integrations.md) · Next: [Maintainer Playbook](maintainer-playbook.md) →
