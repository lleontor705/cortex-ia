# Agent Writing Contract

**Installed contract:** `~/.cortex-ia/opencode/contracts/agent-writing-contract.md`

Use this contract when creating or materially changing agent prompts, skills, commands, `AGENTS.md`, or shared contracts. It governs instruction design, not product documentation.

## Context pointers

A pointer must say what the referenced material controls and the distinct conditions that require reading it. Front-load the triggering concept and collapse synonyms that describe the same branch. A mandatory rule behind an ambiguous pointer is a routing defect.

Keep always-loaded text small. Inline steps and invariants every branch needs; place conditional reference behind a pointer that names when to load it. Co-locate a concept's definition, rules, and exceptions in one authoritative file.

## Executable instructions

- Give each ordered step a checkable completion criterion. Prefer exhaustive bounds such as “every modified path accounted for” over vague completion language.
- State the positive target behavior first. Retain prohibitions only for real safety or authority boundaries.
- Use one stable leading term for one concept across agents, skills, receipts, and UI.
- Treat manifests, schemas, command help, and directory layout as primary sources. Do not cache easy lookups in prompts unless the cache carries a non-obvious reason or gotcha.
- Remove duplicate rules, stale branches, generic advice, and instructions that do not change agent behavior.

## Handoffs and phase boundaries

Handoffs carry pointers to primary artifacts, task IDs, decisions, and evidence rather than copied transcripts. Do not summarize information already present in OpenSpec, Cortex evidence, a work receipt, a diff, or a project profile.

Compact or hand off at a phase boundary, not in the middle of a causal investigation or implementation loop. Continue when the next phase needs the current reasoning as a primary source; dispatch a bounded leaf when it can run independently; otherwise preserve only the smallest sufficient secondary context.

## Language Domain Contract (Persona Scope)

The user's conversational language, tone, and active persona govern ONLY direct replies, explanations, and user-facing status in chat. They DO NOT govern technical artifacts:
- Source code, identifiers, function/variable names, and comments
- UI strings, labels, accessibility text, and error messages
- Documentation, README files, commit messages, PR titles and descriptions
- Specifications, task descriptions, and test fixtures

For all technical artifacts:
- Default strictly to concise, professional English unless the user explicitly requests another language for that specific artifact or the existing codebase convention strictly mandates it.
- Never inject colloquialisms, regional slang, or persona stylistic quirks into code, tests, or persistent contracts.

## Delivery Guarantee (Saving is not Replying)

Persisting observations to Cortex MCP (`cortex_save`) or updating SQLite work authority (`cortex_ia_work_transition`, `cortex_ia_work_approve`) is internal bookkeeping. It NEVER substitutes for answering the user.
- The user never directly sees tool calls or internal state transitions. If an explanation exists only inside a tool payload or memory observation, the user never received it.
- Execute all memory, state, and authority mutations BEFORE composing the final user-facing response.
- End every turn with a complete, transparent synthesized answer for the human operator, with NO tool calls after it. Never collapse a turn into a one-line "Saved/Updated" acknowledgment.
- If a tool call fails or times out, report the failure concisely and deliver the substantive answer anyway.

## Lossless Blocking Prompts

When presenting an interactive decision, choice menu, or approval gate to the user (via `question`, `grill-me`, session alignment, or diagnostic doctor prompts):
- Preserve the complete user-facing choice envelope: why input is required, every option with its original label and description, and the exact allowed-answer domain.
- Never silently default, infer, reorder, truncate, or decide on the user's behalf.
- If the interaction is ambiguous, re-present the complete choice envelope and wait.

## Review gate

Before accepting an instruction change, verify that every new pointer has a real trigger, every normative rule has one source of truth, conditional detail is disclosed only when needed, and completion can be distinguished from premature stopping.
