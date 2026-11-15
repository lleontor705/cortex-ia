# Skill Style Guide

**Installed contract:** `~/.cortex-ia/opencode/contracts/skill-style-guide.md`

Use this when you author or materially change a `SKILL.md` procedure document. It companions `agent-writing-contract.md`. Size targets and the hard max live in that contract's **Skill Body Budget** section and are not restated here.

## Scope

- **`SKILL.md` bodies** are sized by the Skill Body Budget and shaped by the guidance below.
- **Role and agent prompts** follow the layered anatomy in `agent-writing-contract.md`. This guide does not apply to them.

## Trigger-first descriptions

The frontmatter `description` is the routing signal. The loader decides from it alone.

- Open with what the skill does: a verb and its object.
- Then name the conditions that load it: task shapes, file types, phrases. Name each distinct trigger concretely.
- Collapse synonyms that select the same branch. Keep one leading term per concept.
- Avoid vague capability prose ("helps with X") and motivation-first openings. A mandatory rule must be reachable from the description.

## Section order

A `SKILL.md` reads well in this order. This is a guide, not a template. Omit sections a skill does not need.

| Section | Carries |
|---|---|
| Frontmatter | `name`, trigger-first `description`, `license`, `metadata` |
| Activation | When the skill applies and what it excludes. One short paragraph |
| Hard rules | Invariants and safety boundaries, each with its reason |
| Decision gates | Branches that change the procedure, each with its selecting condition. Fail closed by default |
| Steps | Ordered procedure. Each step has a completion criterion and points to primary artifacts |
| Output contract | The receipt or artifact shape, by pointer to its schema |
| References | Pointers to authoritative contracts and tools, with the condition that requires each |

A lean skill beats a padded one. Do not add a section only to fill the table.

## Review checklist

- [ ] The description opens with what the skill does, then its load triggers.
- [ ] The body is within the Skill Body Budget target; at or over the hard max it is trimmed or split.
- [ ] Each normative rule has one source; other files point to it.
- [ ] No sizing rule appears outside the Skill Body Budget section.
- [ ] No restated contract text, changelog, or dead text.
