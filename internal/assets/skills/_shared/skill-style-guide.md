# Cortex-ia Skill Authoring Style Guide

Authoritative reference for writing and reviewing SKILL.md files in cortex-ia. The `skill-creator` and `skill-improver` meta-skills reference this file as the single source of truth for format, structure, and quality standards.

## TL;DR — Quick Checklist

- [ ] Frontmatter has `name`, `description`, `license`, `metadata.author`, `metadata.version`
- [ ] Body uses `<tag>` XML sections — NO markdown section headers at the top level
- [ ] The `<role>` tag is present and defines who the agent is
- [ ] The `<success_criteria>` tag defines when the skill is DONE
- [ ] No source-framework invocation-control frontmatter fields (see Forbidden Fields below)
- [ ] Paths use `.sdd/` for project config and `cortex` as the persistence backend name
- [ ] Content is substantive and actionable — no placeholder text
- [ ] Shared convention file is referenced, not duplicated inline

## Frontmatter Specification

Frontmatter is YAML between `---` delimiters at the top of the file.

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier. Lowercase, hyphen-separated. Matches the directory name. |
| `description` | string (multi-line `>` allowed) | One-to-three sentence summary including a `Trigger:` clause specifying when to invoke. |
| `license` | string | SPDX identifier (e.g., `MIT`, `Apache-2.0`). |
| `metadata.author` | string | Author name or org. |
| `metadata.version` | string | Semantic version (quoted to avoid YAML float coercion, e.g., `"1.0.0"`). |

### Forbidden Fields

Three frontmatter fields from the source framework (controlling model invocation, user invocability, and delegation restriction) are NOT recognized by the cortex-ia skill loader. Their presence causes parse warnings and MUST be stripped during porting.

To verify which fields are forbidden, consult the catalog skill loader at `internal/catalog/skills.go` — it maintains the authoritative list of recognized fields. Any frontmatter key not in that list will trigger a parse warning.

If a ported skill contains any unrecognized invocation-control fields, strip them before committing. Do not replace them with cortex-ia equivalents — cortex-ia has no equivalent concepts.

### Example Frontmatter

```yaml
---
name: example-skill
description: >
  One-line summary of what the skill does.
  Trigger: When user says "example", "demo", or the orchestrator dispatches this phase.
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
```

## Body Format — XML Tags

cortex-ia skills use XML-style tags for body sections. This is a structural format: each `<tag>` defines a section boundary, and the tag name is semantically meaningful to agents that parse skills.

### Core Rule

```
DO NOT use markdown section headers for top-level body structure.
DO use XML tags (<role>, <success_criteria>, <rules>, etc.) for section boundaries.
```

Markdown formatting (bold, lists, code blocks, tables) is allowed INSIDE tags — only section headers are replaced.

### Canonical Tag Set

Tags are listed in recommended order. Not every skill needs every tag, but `name` + `description` (frontmatter) + `<role>` + `<success_criteria>` are the minimum viable skill.

| Tag | Required | Purpose |
|-----|----------|---------|
| `<role>` | YES | Defines the agent identity and what it receives from the orchestrator. |
| `<success_criteria>` | YES | Bulleted list of conditions that signal the skill is DONE. |
| `<persistence>` | Recommended | Defines what the skill reads and writes to Cortex. References `cortex-convention.md`. |
| `<context>` | Recommended | Describes the phase position and inputs/outputs. |
| `<delegation>` | Recommended | States whether this is a leaf agent or coordinator. References `cortex-convention.md`. |
| `<rules>` | Recommended | Numbered rules. Uses `<critical>` and `<guidance>` child tags. |
| `<approach>` | Optional | High-level methodology or protocol (e.g., ReAct loop, decision tree). |
| `<steps>` | Recommended | Ordered implementation steps. Sub-steps use bold headings inside the tag. |
| `<output>` | Recommended | Output format template (markdown report + JSON contract). |
| `<examples>` | Optional | Concrete worked examples and anti-patterns. |
| `<collaboration>` | Optional | P2P messaging patterns for agent coordination. |
| `<mcp_integration>` | Optional | MCP tool integration points (Cortex, ForgeSpec, Agent Mailbox, Context7). |
| `<self_check>` | Optional | Pre-return self-critique protocol. |
| `<verification>` | Optional | Final checklist before returning the contract. |

### Nesting Rules

Child tags are used for hierarchical sub-sections. The most common nesting pattern is inside `<rules>`:

```xml
<rules>
  <critical>
    1. Hard rule with rationale (blocking)
    2. Another critical rule
  </critical>
  <guidance>
    3. Soft rule (best practice, non-blocking)
    4. Think step by step: review the spec, design, and code pattern before implementing.
  </guidance>
</rules>
```

Inside `<steps>`, sub-headings use **bold text** or numbered steps — NOT markdown section headers:

```xml
<steps>

**Step 1: Load Context**
Follow the Skill Loading Protocol from the shared convention.

**Step 2: Retrieve Artifacts**
Follow the Two-Step Retrieval Protocol.

</steps>
```

### Self-Closing and Empty Tags

Never leave a tag empty. If a section does not apply, omit the tag entirely rather than including `<tag></tag>`.

## Path Conventions

cortex-ia uses specific path and naming conventions that differ from the source framework. All references in ported skills MUST be adapted to cortex-ia conventions:

| Context | cortex-ia Value | Adaptation Rule |
|---------|-----------------|-----------------|
| Project config directory | `.sdd/` | Replace any legacy dot-prefix config directory with `.sdd/` |
| Skill registry file | `.sdd/skill-registry.md` | Registry always lives under `.sdd/` in cortex-ia |
| Persistence backend name | `cortex` | Replace the legacy backend name with `cortex` in all prose |
| Convention directory | `~/.cortex-ia/_shared/` | Shared between Cortex and cortex-ia |

### URL Preservation Rule

External URLs (`http://`, `https://`) MUST NEVER be modified during path adaptation. Only internal paths and the persistence backend name are adapted.

If a URL legitimately contains a path fragment that looks like a legacy config directory, it MUST be preserved as-is. The adaptation applies only to internal project references, never to external links.

## Content Quality Standards

### Substantive, Not Placeholder

Every section must contain real, actionable content:

- `<role>`: State the agent identity in one sentence, then list what it receives from the orchestrator.
- `<success_criteria>`: 3-6 bullet points, each a verifiable condition ("X is true when Y").
- `<rules>`: Each rule includes a parenthetical rationale explaining WHY, not just WHAT.
- `<steps>`: Each step has a clear verb (Read, Write, Execute, Persist, Return).

### Avoid Anti-Patterns

- Generic placeholders: `<role>You are an agent that does things</role>` — too vague.
- Missing rationale: `5. Follow the convention` — why? What breaks if skipped?
- Copy-paste without adaptation: legacy paths, legacy backend names, markdown section headers instead of XML tags.
- Duplicating convention content: reference `cortex-convention.md`, do not inline it.

### Rationale Style

Rules follow the pattern: `N. <Action> — <rationale in parentheses>`.

```
2. Read specs before writing any code — specs define acceptance criteria; code without them fails validation
```

## Skill Registration

New skills MUST be registered in the skill registry for discovery by other agents:

1. Add the skill to `.sdd/skill-registry.md` in the embedded skills table.
2. Update `internal/catalog/skills.go` to include the skill ID in `AllSDDSkillIDs()` and `AllSDDSkills()`.
3. Update `AGENTS.md` with the skill name, trigger, and path.
4. Run `cortex-ia skill-registry refresh` (or `go test ./internal/catalog/...`) to validate.

## Validation Checklist for a New Skill

Before a new SKILL.md is considered complete:

- [ ] Frontmatter has all required fields (`name`, `description`, `license`, `metadata`)
- [ ] No forbidden (source-framework invocation-control) fields present — verify against the catalog loader
- [ ] `name` matches the directory name exactly
- [ ] `description` includes a `Trigger:` clause
- [ ] Body uses `<tag>` XML sections (no markdown section headers at the top level)
- [ ] `<role>` and `<success_criteria>` are present
- [ ] No legacy config directory path references (must use `.sdd/`)
- [ ] No legacy persistence backend references (must use `cortex`)
- [ ] No external URLs modified during porting
- [ ] Content is substantive (no placeholder text)
- [ ] Convention is referenced, not duplicated
- [ ] Skill is registered in `.sdd/skill-registry.md`
- [ ] Skill is registered in `internal/catalog/skills.go`
- [ ] `go test ./internal/catalog/...` passes
