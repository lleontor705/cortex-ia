# Project & Extension Guide

← [Codebase Guide](../CODEBASE-GUIDE.md)

How to extend `cortex-ia`: adding new skills, agents, commands, plugins, or MCP presets for **OpenCode**, and the expansion architecture for future platforms (**Google Antigravity** and **Claude CLI**).

---

## 1. Extension Types

| Extension Target | Files to Touch | Pattern to Follow |
| :--- | :--- | :--- |
| **New Native Skill** | `internal/assets/skills/<name>/SKILL.md` | Standard `SKILL.md` with YAML frontmatter + prompt guidelines. |
| **New Sub-Agent** | `internal/assets/agents/<name>.md` | Markdown role specification with persona, tools, and model recommendations. |
| **New Slash Command** | `internal/assets/commands/<name>.md` | Command instruction markdown file. |
| **New Plugin** | `internal/assets/plugin/<name>.ts` | OpenCode TypeScript plugin runtime. |
| **New MCP Catalog Preset** | `internal/mcpmanager/presets.go` | Declare `Preset` with typed command vector and description. |
| **New CLI Subcommand** | `internal/app/app.go`, `internal/app/cli.go` | Route command in dispatcher, bind flags, render typed receipt. |

---

## 2. Adding an OpenCode Skill

All skills are embedded into the binary at compile-time using `go:embed`:

1. Create `internal/assets/skills/<name>/SKILL.md` with frontmatter:
   ```yaml
   ---
   name: my-skill
   description: Brief description of what this skill does and its trigger.
   ---
   # Instructions
   ...
   ```
2. The skill will automatically be discovered by `opencode.MapAssets()` and copied under `~/.config/opencode/skills/<name>/SKILL.md`.
3. Rebuild the binary (`go build -o bin/cortex-ia ./cmd/cortex-ia`).

---

## 3. Adding an MCP Catalog Preset

To register a new managed preset available in `cortex-ia mcp add <name> --preset`:

1. Open `internal/mcpmanager/presets.go`.
2. Add a `Preset` struct to the catalog:
   ```go
   Preset{
       Name:        "my-tool",
       Kind:        EntryLocal,
       Command:     []string{"npx", "-y", "my-tool-mcp"},
       Description: "Description of capabilities",
       DefaultOn:   false,
   }
   ```
3. Run `go test ./...` to verify qualification.

---

## 4. Platform Expansion Roadmap

`cortex-ia` will introduce additional platforms natively:

1. **Google Antigravity**:
   - Location: `~/.gemini/antigravity/`
   - Maps native rules, custom skills, and sidecars without legacy adapter baggage.
2. **Claude CLI**:
   - Location: `~/.claude/`
   - Maps native prompt files and stdio MCP servers.

---

← Prev: [Integrations](integrations.md) · Next: [Maintainer Playbook](maintainer-playbook.md) →
