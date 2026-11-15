# Skill Style Guide

**Installed contract:** `~/.cortex-ia/opencode/contracts/skill-style-guide.md`

Use this guide when authoring or materially changing a `SKILL.md` procedure document. It is a companion to `agent-writing-contract.md`, not a replacement for it. The target band, hard max, and estimation heuristic for skill bodies live in that contract's **Skill Body Budget** section; this guide references them and never restates their values.

## Scope boundary

- **`SKILL.md` bodies**: sized by **Skill Body Budget** and shaped by the routing/structure guidance in this guide.
- **Role and agent system prompts**: governed exclusively by the **Hierarchical 4-Layer System Prompt Anatomy** in `agent-writing-contract.md` (`<identity>` / `<capabilities_and_tools>` / `<workflow_protocol>` / `<hard_invariants>` + `<global_contracts>`). Nothing in this guide sizes or restructures them.

The section mapping below describes a *procedure document*. It is a reading-order recommendation for `SKILL.md` files, not a competing anatomy for agent prompts.

## Trigger-first descriptions

The frontmatter `description` is the routing signal: the loader decides whether to load a skill from the description alone.

- **Open with what the skill does**: an action verb plus the object it operates on.
- **Then state trigger-first conditions**: the task shapes, file types, and phrases that make this skill the right choice. Name distinct triggers concretely.
- Collapse synonyms that select the same branch; keep one canonical leading term per concept.
- Avoid vague capability prose ("helps with X") and motivation-first openings. If a rule matters, its trigger must be reachable from the description.
- Treat an ambiguous pointer to a mandatory rule as a routing defect: state the trigger, then point to the detail.

## Recommended section mapping

A `SKILL.md` reads well when its body maps to these sections in this order. This is a recommendation, not a required template.

| Section | Carries | Notes |
|---|---|---|
| Frontmatter | `name`, `description` (trigger-first), `license`, `metadata` | Description drives routing; it is excluded from the body budget |
| Activation | When the skill applies, and what it does not cover | One short paragraph |
| Hard Rules | Non-negotiable invariants and safety boundaries | State the positive target first; keep prohibitions for real boundaries |
| Decision Gates | Branches that change the procedure, each with its selecting condition | Fail-closed defaults |
| Steps | Ordered procedure with a checkable completion criterion per step | Point to primary artifacts; do not cache easy lookups |
| Output Contract | The receipt or artifact shape the skill returns | Reference the schema by pointer; do not duplicate it |
| References | Pointers to authoritative contracts and tools | Name what each controls and the condition that requires reading it |

Collapse or omit sections a skill does not need. A lean, in-band skill beats a padded one that fills every row.

## Fitting the body budget (advisory)

1. Estimate the body with the advisory token-estimation heuristic defined in the **Skill Body Budget** section of `agent-writing-contract.md`.
2. Compare the estimate against the target band and hard max defined in the same section. Inside the band is the steer; a body slightly outside it is acceptable when the procedure genuinely needs the space.
3. A body at or beyond the hard max must be trimmed or split before the change is accepted.
4. Re-estimate after trimming. Estimation is an authoring and review aid only; no runtime validator counts tokens and the hard max is checked at material-change review.

## Anti-patterns

- Restating another contract's normative rule instead of pointing to it (creates a second source of truth).
- Narrative echo of obvious mechanics, changelogs, and commented-out dead text.
- Inlining steps every branch needs while burying conditional detail behind an ambiguous pointer.
- Padding a body to "complete" the section mapping when the procedure does not need those sections.

## Review checklist

- [ ] `description` opens with what the skill does, then its load triggers.
- [ ] Body size is within the Skill Body Budget steer; at or beyond the hard max it is trimmed or split.
- [ ] Every normative rule has exactly one source of truth; other files reference it.
- [ ] The section mapping is used as a guide, not padded to fit.
- [ ] No sizing rule is introduced outside the contract's **Skill Body Budget** section.
