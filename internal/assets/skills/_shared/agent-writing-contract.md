# Agent Writing Contract

**Installed contract:** `~/.cortex-ia/opencode/contracts/agent-writing-contract.md`

Use this when you create or materially change an agent prompt, skill, command, `AGENTS.md` or shared contract. It governs how instructions are written, not product documentation.

## Style rules

- Write plainly. State a rule once with its reason in the same sentence. Do not use all-caps emphasis. Keep absolute wording only for real authority and safety boundaries (token secrecy, no self-approval, no writes without a lease), and give the reason there.
- One source per rule. Other files point to it by file and section name and do not restate it, not even as a digest.
- Do not restate what frontmatter permissions or tools already enforce. Say what to do when a tool refuses, and let the refusal message carry the rest.
- Describe good output and a few hard constraints. Let the model order its own steps. Use numbered steps only where order matters, such as claim before edit or transition after verify.
- Give each step a checkable completion criterion.
- Keep tool names exact. Do not rename or drop tools in a prompt edit.
- Keep the section names that other files reference (see `cortex-work-protocol.md`).
- No invented citations, no emoji, no marketing words. Technical artifacts are written in English.
- Delete stale and internal material: retired surfaces, legacy envelopes, repository-internal issue IDs, cache-tuning notes, step budgets written as prose, and ceremonial lines.

## Context pointers

A pointer says what the referenced material controls and the conditions that require reading it. Put the triggering concept first and collapse synonyms that describe one branch. A mandatory rule behind a vague pointer is a routing defect.

Keep always-loaded text small. Inline the steps and invariants every branch needs. Place conditional detail behind a pointer. Keep a concept's definition, rules and exceptions in one authoritative file.

## Role prompt anatomy

Role prompts use four tagged layers:

1. `<identity>`: who the agent is and what it is responsible for.
2. `<capabilities_and_tools>`: what it may use, and what it does not do.
3. `<workflow_protocol>`: the procedure, with checkable completion criteria.
4. `<hard_invariants>`: the few boundaries that must hold, each with its reason.

Shared policies that must read the same in every role go in `<global_contracts>`: language, delivery and output format.

## Handoffs and phase boundaries

Handoffs carry pointers to primary artifacts, task IDs, decisions and evidence, not copied transcripts. Do not restate what is already in the plan file, Cortex evidence, a receipt, a diff or a project profile.

Compact or hand off at a phase boundary, not in the middle of an investigation or an implementation loop. Dispatch a bounded leaf when it can run independently. Otherwise keep the smallest context that the next step needs.

## Dispatch briefs

Briefs are context-complete and carry `non_goals`. Subagents get narrowed authority: they stay within `allowed_files` and `non_goals`. The envelope fields and communication patterns are defined in `cortex-work-protocol.md` §5 and §6.

## Communication

- No narration between tool calls. Do not announce each step.
- Final replies are concise: a heading with the verdict, a short synthesis, and the evidence that matters. Omit apologies and restated diffs.
- Lead with the verdict, route and status. Keep the detailed record in the control plane (SQLite, Cortex), not in chat.
- Never put raw JSON in a chat reply. Structured receipts go through typed tools or the final report.
- Saving to Cortex or changing work state does not replace the reply. Finish every turn with a complete answer and no tool call after it. If a tool call fails, say so briefly and still answer.

## Interactive decisions

When a choice needs the user, present the full envelope: why input is needed, every option with its label and description, and the allowed answers. Never default, reorder, truncate or decide for the user. If the answer is ambiguous, present the envelope again and wait.

## Review

Reviewers audit against primary evidence: the actual diff, test exit codes, and the contract. They do not rely on the implementer's narrative or stated confidence. Self-reported success is advisory only.

## Code comments

Code says how; comments say why. Do not write comments that restate the code, record task or session attribution, or keep commented-out code. Keep comments for a non-obvious reason, a workaround, or an invariant the type system cannot express.

## Failure traces

Report a failure as a bounded trace: target file, location, error code, minimal reproduction command, and a snippet of 25 lines or fewer. Never paste whole logs.

## Skill Body Budget

This section is the single source of truth for `SKILL.md` body sizing. It governs `SKILL.md` bodies only. Role prompts follow the layered anatomy above; the budget does not size them.

- **Target**: 180–450 tokens per `SKILL.md` body, frontmatter excluded. Inside this band is the steer. Slightly outside is acceptable when the procedure needs the space.
- **Hard max**: 1000 tokens. A body at or over the hard max is split or trimmed before the change is accepted.
- **Trigger-first descriptions**: the frontmatter `description` opens with what the skill does, then the conditions that load it, so routing decides from the description alone.
- **Estimation**: characters divided by 4 (roughly words × 1.3). An authoring and review aid only. No runtime validator counts tokens.
- **Enforcement**: checked at the review gate that accepts a body change, never by tooling.

## Review gate

Before accepting an instruction change, check that each new pointer has a real trigger, each normative rule has one source, conditional detail appears only where it is needed, and completion can be told apart from premature stopping. Any rule that sizes skill bodies lives in the Skill Body Budget section only.
