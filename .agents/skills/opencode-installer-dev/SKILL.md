---
name: opencode-installer-dev
description: Develop, debug, test, and maintain configuration installers and environment orchestrators targeting OpenCode v2 (opencode2).
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# OpenCode v2 Configuration Installer Development Skill

This skill guides engineers building or maintaining installers, synchronizers, and setup automation tools (such as `cortex-ia`) targeting **OpenCode v2 (`opencode2`)**.

---

## 1. Core Installer Principles & Invariants

An installer for OpenCode v2 must respect the runtime's architectural expectations:

1. **Dual Configuration Split**:
   - `cli.json`: Strictly for TUI/CLI appearance, keybindings, diffs, sound, and theme (`"theme": { "name": "...", "mode": "..." }`).
   - `opencode.jsonc`: Strictly for background service, providers, models, agents, compaction, permissions, and MCP servers.
   - *Invariant*: Never place server properties in `cli.json` or TUI properties in `opencode.jsonc`.

2. **Permissions Array Format**:
   - OpenCode v2 mandates an ordered array format:
     ```json
     "permissions": [
       { "action": "shell", "resource": "*", "effect": "allow" },
       { "action": "shell", "resource": "git push *", "effect": "ask" },
       { "action": "read", "resource": "**/.env", "effect": "deny" }
     ]
     ```
   - Evaluation uses first-match or specific precedence: specific `deny` or `ask` rules must precede broad `allow` rules.
   - Action names are updated in v2: `shell` (not `bash`), `subagent` (not `task`), `edit` (not `write`/`patch`).

3. **Comments and Formatting Preservation**:
   - OpenCode configuration files frequently contain explanatory comments (`//`) and trailing commas.
   - *Invariant*: Always use comment-preserving AST parsers (e.g. `filemerge.MutateJSONFile`) when updating configuration files. Never round-trip through standard `json.Unmarshal` / `json.Marshal`, which destroys user comments.

4. **Safe Discovery & File Destinations**:
   - Themes belong in: `~/.config/opencode/themes/<name>.json` or `.opencode/themes/<name>.json`.
   - Plugins belong in: `~/.config/opencode/plugins/<name>.ts` or `.opencode/plugins/<name>.ts`.
   - Project skills belong in: `.agents/skills/<name>/SKILL.md` or `.opencode/skills/<name>/SKILL.md`.
   - Project instructions belong in: `AGENTS.md` (root).

---

## 2. Precedence Hierarchy

OpenCode v2 traverses directories from the active workspace root upward:
1. Ancestor `opencode.json(c)` files.
2. Workspace root `opencode.json(c)`.
3. Workspace root `.opencode/opencode.json(c)` (Always overrides direct files).
4. Global user settings in `~/.config/opencode/opencode.json(c)` and `cli.json`.

---

## 3. Local Diagnostic Tools

When troubleshooting an OpenCode v2 installation on the developer machine:
```bash
# 1. Check version
opencode2 --version

# 2. Inspect active configuration sources and merged tree
opencode2 debug config

# 3. Verify global system paths (data, cache, config, logs)
opencode2 debug paths

# 4. Inspect active models and provider connections
opencode2 models

# 5. Check real-time server logs
opencode2 --print-logs
```
