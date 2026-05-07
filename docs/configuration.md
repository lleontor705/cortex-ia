# Configuration

## CLI Reference

```
cortex-ia                              # Launch interactive TUI
cortex-ia install [flags]              # Install ecosystem
cortex-ia sync [flags]                 # Refresh managed files
cortex-ia detect                       # Show agents + runtime deps
cortex-ia config                       # Show current configuration
cortex-ia list agents|components|backups
cortex-ia init                         # Create .cortex-ia.yaml
cortex-ia skill add|list|remove        # Manage community skills
cortex-ia auto-install [--dry-run]     # Install missing agents
cortex-ia doctor                       # Run health checks
cortex-ia repair [--dry-run]           # Re-apply from state
cortex-ia rollback [--backup ID]       # Restore from backup
cortex-ia update                       # Check for updates
cortex-ia uninstall [flags]            # Reverse cortex-ia injections (with snapshot)
cortex-ia gga --provider <id>          # Switch GGA provider
cortex-ia profiles list|create|set|apply|delete   # OpenCode SDD profiles
cortex-ia agent-builder list|create|remove        # AI-generated custom skills
cortex-ia version                      # Show version
cortex-ia help                         # Show usage
```

### Install Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--agent <id>` | Target specific agent (repeatable) | `--agent claude-code --agent opencode` |
| `--preset <id>` | Installation preset | `--preset minimal` |
| `--model-preset <id>` | Per-phase model routing | `--model-preset economy` |
| `--persona <id>` | Communication style | `--persona mentor` |
| `--local` | Load project .cortex-ia.yaml config | `--local` |
| `--dry-run` | Preview without making changes | `--dry-run` |

If no `--agent` is specified, cortex-ia auto-detects all installed agents.

If no `--preset` is specified, defaults to `full`.

Valid `--agent` values: `claude-code`, `opencode`, `gemini-cli`, `cursor`, `vscode-copilot`, `codex`, `windsurf`, `antigravity`, `kilocode`, `kimi`, `kiro-ide`, `qwen-code`.

### Uninstall Flags

| Flag | Description |
|------|-------------|
| `--agent <id>` | Uninstall only from a specific agent (repeatable) |
| `--component <id>` | Uninstall only a specific component (repeatable) |
| `--all` | Reverse every managed change and clear `state.json` |
| `--dry-run` | Print the planned operations without writing |
| `--yes`, `-y` | Skip the destructive-action confirmation prompt |
| `--no-backup` | Skip the pre-uninstall snapshot (not recommended) |

A snapshot tagged `BackupSourceUninstall` is taken before any change so rollback works exactly like an install rollback.

### GGA Switcher

```bash
cortex-ia gga --provider anthropic    # Anthropic API directly
cortex-ia gga --provider ollama       # Local Ollama (sets API_BASE)
cortex-ia gga --provider claude       # Route via Claude Code (default)
cortex-ia gga --list                  # List supported providers
cortex-ia gga --show                  # Print current ~/.config/gga/config
```

Direct-LLM providers (`anthropic`, `openai`, `google`, `ollama`) emit a `MODEL=` line; agent-routed providers (`claude`, `opencode`, `gemini`, `codex`) do not.

### OpenCode SDD Profiles

```bash
# Create a profile that maps every SDD phase to one model
cortex-ia profiles create cheap:openai/gpt-4o-mini

# Override one phase
cortex-ia profiles set cheap:sdd-design:anthropic/claude-opus-4

# Write the profile's per-phase models into ~/.config/opencode/opencode.json
cortex-ia profiles apply cheap

cortex-ia profiles list
cortex-ia profiles delete cheap
```

Profile values may be either a Claude alias (`opus` / `sonnet` / `haiku`, expanded to `anthropic/claude-<alias>-N`) or a fully-qualified `provider/model` string. `apply` writes them to the real SDD agent entries in `opencode.json` (`architect`, `decompose`, `team-lead`, `implement`, etc.). The `sdd-apply` phase maps to both `team-lead` and `implement`.

### Agent Builder

```bash
cortex-ia agent-builder create \
  --engine claude \
  --purpose "review go diffs against project conventions" \
  --target claude-code --target opencode \
  --persona professional

cortex-ia agent-builder list
cortex-ia agent-builder remove <name>
```

Supported `--engine` values: `claude-code`, `opencode`, `gemini-cli`, `codex`. The engine binary must be on `PATH`. The default `--timeout` is 120 s; `--dry-run` prints the prompt that would be sent to the engine. The persisted registry lives at `~/.cortex-ia/agentbuilder/registry.json`.

## Interactive TUI

When run without arguments, cortex-ia launches an 8-screen interactive installer:

1. **Welcome** — Logo, ecosystem overview, press Enter to start
2. **Detection** — Platform info, runtime dependencies (Node, npx, Git, Go, Cortex, shell), detected agents
3. **Agents** — Multi-select from detected agents (↑↓ navigate, Space toggle, `a` select all)
4. **Preset** — Choose full or minimal (↑↓ navigate, Enter select)
5. **Persona** — Choose communication style: professional, mentor, or minimal
6. **Review** — Shows selected agents + resolved components + persona
7. **Installing** — Progress while pipeline runs
8. **Complete** — Summary with file count, backup ID, and any warnings

Navigation: `Esc` goes back, `q` quits, `Enter` confirms.

## Per-Phase Model Routing

Assign Claude model tiers (opus/sonnet/haiku) to SDD phases for cost/quality optimization:

| Preset | Orchestrator | Architect | Implement | Validate | Finalize |
|--------|:-:|:-:|:-:|:-:|:-:|
| **balanced** (default) | opus | opus | sonnet | opus | haiku |
| **performance** | opus | opus | sonnet | opus | haiku |
| **economy** | sonnet | sonnet | sonnet | sonnet | haiku |

```bash
cortex-ia install --model-preset economy
```

For OpenCode, these assignments are written to each agent's `model` field in `opencode.json`; the orchestrator no longer passes model names as text in delegation prompts.

## Persona System

| Persona | Style |
|---------|-------|
| `professional` (default) | Direct, concise, technical terminology |
| `mentor` | Teaching-oriented, explains trade-offs and patterns |
| `minimal` | Code only, no explanations unless asked |

```bash
cortex-ia install --persona mentor
cortex-ia sync --persona minimal    # Change without full reinstall
```

Persona is injected via `<!-- cortex-ia:cortex-persona -->` markers in the agent's system prompt.

## Project Configuration (.cortex-ia.yaml)

Create per-repo config to standardize settings across your team:

```bash
cortex-ia init    # Creates .cortex-ia.yaml with defaults
```

```yaml
# .cortex-ia.yaml
preset: full
persona: professional
model-preset: balanced
agents:
  - claude-code
  - opencode
disabled-components:
  - mailbox
custom-skills:
  - path: ./skills/domain-validator
```

```bash
cortex-ia install --local    # Merges project config with global settings
```

Config lookup: walks up from CWD to root looking for `.cortex-ia.yaml`. Project config overrides global defaults (but CLI flags override both).

## Community Skills

Three-layer skill system:

| Layer | Location | Priority |
|-------|----------|:--------:|
| Embedded | Binary (go:embed, 19 skills) | Low (fallback) |
| Community | `~/.cortex-ia/skills-community/` | Medium |
| Project | `.cortex-ia.yaml` → `custom-skills` | High (override) |

```bash
cortex-ia skill add ./my-skill-dir    # Copies SKILL.md to community dir
cortex-ia skill list                   # Shows installed community skills
cortex-ia skill remove my-skill        # Removes from community dir
cortex-ia sync                         # Deploys community skills to shared dir
```

## State

cortex-ia persists installation state at `~/.cortex-ia/state.json`:

```json
{
  "installed_agents": ["claude-code", "opencode"],
  "preset": "full",
  "components": ["cortex", "forgespec", "agent-mailbox", "context7", "conventions", "sdd"],
  "last_install": "2026-03-31T00:00:00Z",
  "last_backup_id": "20260331-000000",
  "version": "0.2.0"
}
```

## Health Checks

`cortex-ia doctor` runs 6 checks:

| Check | Severity | What it verifies |
|-------|:--------:|-----------------|
| files-exist | Error | All tracked files from lockfile present |
| cortex-binary | Warning | Cortex MCP binary in PATH |
| node-npx | Warning | Node.js and npx available for MCP servers |
| skills-present | Warning | Core skill files in shared dir |
| convention-present | Warning | Cortex convention file exists |
| state-lock-consistent | Warning | State and lock files agree on agents/components |

## Backup & Restore

### Automatic Backups

Every `install` creates a snapshot via the 2-stage pipeline (Prepare stage):

```
~/.cortex-ia/backups/20260331-000000/
├── manifest.json
└── files/
    └── (copies of all files that will be modified)
```

### Commands

```bash
cortex-ia list backups     # Show available backups
cortex-ia rollback         # Restore from most recent backup
cortex-ia rollback --backup 20260331-000000  # Restore specific backup
cortex-ia repair           # Re-apply current state (no restore, just re-inject)
```

### Retention & Pinning

Backups now carry two optional fields (omitempty for backwards compatibility):

| Field | Purpose |
|-------|---------|
| `pinned` | Excludes the backup from `Prune`. Pin manually for known-good snapshots. |
| `checksum` | SHA-256 over the snapshot inputs. Used by `IsDuplicate` to skip duplicate backups. |

Default retention is **5 unpinned backups** (`backup.DefaultRetentionCount`). `Prune` runs at the end of `install` / `sync` so the backup directory does not grow without bound.

## Dependency Resolution

Uses Kahn's algorithm (topological sort) with parallel group detection:

```
Level 0 (parallel): cortex, forgespec, mailbox, context7, skills
Level 1 (after cortex): conventions
Level 2 (after cortex+forgespec+mailbox): sdd
```

## Idempotency

cortex-ia is fully idempotent:
- MCP configs: atomic write with content comparison — skips if identical
- System prompts: marker-based injection replaces only managed sections
- Skills: written via atomic write — no change if content matches
- Running `install` twice produces zero file changes on the second run
