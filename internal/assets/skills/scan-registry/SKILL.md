---
name: scan-registry
description: "Scans all skill directories, builds a unified skill registry catalog, and persists it for sub-agent discovery."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Scan Registry

<role>
You are a skill catalog builder who scans all known skill locations, assembles a unified registry, and persists it so that every sub-agent can discover available skills without redundant searching.
</role>

<success_criteria>
- All skill directories (user-level and project-level) are scanned.
- A complete registry file is written to `.sdd/skill-registry.md` in the project root.
- The registry is saved to Cortex (if available) for cross-session persistence.
- A summary is returned listing all skills and conventions found.
</success_criteria>

<delegation>none — you are a LEAF agent. Do NOT use the task() tool. Do NOT launch sub-agents. Do all work directly.</delegation>

<rules>

Announce at start: "I'm using the scan-registry skill to build the skill catalog."

The skill registry is a catalog that sub-agents read before starting any task. Run this skill after installing/removing skills, setting up a new project, or when the user asks to update the registry.

## Scanning Rules

1. Do NOT use the task() tool or launch sub-agents under any circumstance — you are a leaf agent
2. Scan ALL known skill directories listed in Step 1 -- always check every path, even after finding matches.
3. Read only the frontmatter (first 10 lines) of each SKILL.md -- always limit reads to frontmatter only.
4. Skip directories named `old`, `_shared`, and `scan-registry` (this skill).
5. Deduplicate by skill name: if the same name appears at both user-level and project-level, keep the project-level version (more specific wins).
6. If the same name appears in two user-level locations, keep the first one found.

## Persistence Rules

7. Write the registry file to `.sdd/skill-registry.md` regardless of other persistence options.
8. Save to Cortex if the `mem_save` tool is available. Use `topic_key: "skill-registry"` for upsert behavior.
9. Add `.sdd/` to the project's `.gitignore` if `.gitignore` exists and `.sdd` is not already listed.

## Output Rules

10. Include every skill found, even if its frontmatter is incomplete (mark description as "No description" if missing).
11. Include every convention file found, with paths expanded from index files.
12. Always write an empty registry when no skills or conventions are found, so sub-agents know to stop searching.

</rules>

<steps>

## Step 1: Scan Skill Directories

Glob for `*/SKILL.md` files across ALL of the following paths. Check every path that exists on the system:

**User-level (global skills):**
- `~/.claude/skills/`
- `~/.config/opencode/skills/`
- `~/.gemini/skills/`
- `~/.cursor/skills/`
- `~/.copilot/skills/`

**Project-level (workspace skills):**
- `{project-root}/.claude/skills/`
- `{project-root}/.opencode/skills/`
- `{project-root}/.gemini/skills/`
- `{project-root}/.agent/skills/`
- `{project-root}/skills/`

For each `SKILL.md` found:
1. Read only the first 10 lines (frontmatter).
2. Extract the `name` field.
3. Extract the `description` field. If the description contains "Trigger:", extract the trigger text separately.
4. Record the full absolute path to the SKILL.md file.
5. Apply skip rules: ignore `old`, `_shared`, and `scan-registry` directories.
6. Apply deduplication rules: project-level wins over user-level for same name.

## Step 2: Scan Project Conventions

Check the project root for convention files:
- `agents.md` or `AGENTS.md`
- `CLAUDE.md` (project-level only, not `~/.claude/CLAUDE.md`)
- `.cursorrules`
- `GEMINI.md`
- `copilot-instructions.md`

For index files (`agents.md`, `AGENTS.md`):
1. Read the file contents.
2. Extract all referenced file paths.
3. Include both the index file and every path it references in the registry.

For standalone files (`.cursorrules`, `CLAUDE.md`, etc.):
1. Record the file path directly.

## Step 3: Build Registry Markdown

Assemble the registry in this format:

```markdown
# Skill Registry

As your FIRST step before starting any work, identify and load skills relevant to your task from this registry.

## Skills

| Trigger | Skill | Path |
|---------|-------|------|
| {trigger or "N/A"} | {skill name} | {absolute path to SKILL.md} |

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| {filename} | {absolute path} | {Index / Referenced by X / standalone} |

Read the convention files listed above for project-specific patterns and rules.
```

## Step 4: Persist the Registry

### A. Write the file (mandatory):

1. Create the `.sdd/` directory in the project root if it does not exist.
2. Write the registry markdown to `.sdd/skill-registry.md`.
3. If `.gitignore` exists in the project root and does not contain `.sdd`, append `.sdd/` to it.

### B. Save to Cortex (if available):

If the `mem_save` tool is available:

```
mem_save(
  title: "skill-registry",
  topic_key: "skill-registry",
  type: "config",
  project: "{project-name}",
  content: "{registry markdown from Step 3}"
)
```

The `topic_key` ensures running this skill again updates the same observation rather than creating duplicates.

## Step 5: Return Summary

Present a summary to the user:

```markdown
## Skill Registry Updated

**Project**: {project name}
**Location**: .sdd/skill-registry.md
**Cortex**: {saved / not available}

### Skills Found ({count})
| Skill | Trigger |
|-------|---------|
| {name} | {trigger or "N/A"} |

### Project Conventions Found ({count})
| File | Path |
|------|------|
| {filename} | {path} |

### Next Steps
Sub-agents will automatically load relevant skills from this registry.
To update after installing or removing skills, run this skill again.
```

</steps>

<output>
Two persistent artifacts:
1. **File**: `.sdd/skill-registry.md` in the project root -- the authoritative skill catalog.
2. **Cortex observation** (if available): same content saved under `topic_key: "skill-registry"` for cross-session discovery.

Plus a summary message printed to the user confirming what was found and where it was saved.
</output>

<examples>

### Example 1: Project With Multiple Skills

**INPUT**: User runs scan-registry on a webapp project with 5 skills and 3 convention files.

**OUTPUT**:
```markdown
## Skill Registry Updated

**Project**: my-webapp
**Location**: .sdd/skill-registry.md
**Cortex**: saved

### Skills Found (5)
| Skill | Trigger |
|-------|---------|
| ideate | "brainstorm", "design", "ideate" |
| execute-plan | "execute plan", "implement plan" |
| tdd | "test driven", "tdd" |
| debugging | "debug", "investigate" |
| frontend-design | "UI design", "mockup" |

### Project Conventions Found (3)
| File | Path |
|------|------|
| AGENTS.md | {project-root}/AGENTS.md |
| coding-standards.md | {project-root}/docs/coding-standards.md |
| CLAUDE.md | {project-root}/CLAUDE.md |

### Next Steps
Sub-agents will automatically load relevant skills from this registry.
To update after installing or removing skills, run this skill again.
```

### Example 2: Empty Project

**INPUT**: User runs scan-registry on a new project with no skills or convention files.

**OUTPUT**:
```markdown
## Skill Registry Updated

**Project**: new-project
**Location**: .sdd/skill-registry.md
**Cortex**: not available

### Skills Found (0)
No skills found in any scanned directory.

### Project Conventions Found (0)
No convention files found.

### Next Steps
Install skills to any of the scanned directories and run this skill again.
```

</examples>

<self_check>
Before producing your final output, verify:
1. All skill directories scanned?
2. Deduplication applied (project > user)?
3. Registry saved to both filesystem and Cortex?
</self_check>

<verification>
Before completing this skill, confirm:

- [ ] All user-level skill directories were checked (5 paths).
- [ ] All project-level skill directories were checked (5 paths).
- [ ] Only frontmatter was read from each SKILL.md (first 10 lines).
- [ ] `old`, `_shared`, and `scan-registry` directories were skipped.
- [ ] Deduplication was applied (project-level wins over user-level).
- [ ] Convention files were scanned, with index file references expanded.
- [ ] Registry file was written to `.sdd/skill-registry.md`.
- [ ] `.sdd/` was added to `.gitignore` if applicable.
- [ ] Cortex save was attempted if `mem_save` is available.
- [ ] A summary was returned to the user.
</verification>
