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

## Format & Transport Separation (No Raw JSON in Chat)

Never instruct or compel an agent to output raw JSON code blocks as their chat response to the user.
- Chat text belongs to the human operator: format with clear Markdown, tables, headings, and code snippets.
- Structured receipts, state handoffs, and verification verdicts must be transmitted via typed tool arguments (`cortex_ia_work_transition`, `cortex_ia_work_approve`) or semantic envelopes.
- Do not mix Markdown narrative and raw JSON in the same chat turn; doing so invites syntax errors, token waste, and attention degradation.

## Lossless Blocking Prompts

When presenting an interactive decision, choice menu, or approval gate to the user (via `question`, `grill-me`, session alignment, or diagnostic doctor prompts):
- Preserve the complete user-facing choice envelope: why input is required, every option with its original label and description, and the exact allowed-answer domain.
- Never silently default, infer, reorder, truncate, or decide on the user's behalf.
- If the interaction is ambiguous, re-present the complete choice envelope and wait.

## Hierarchical 4-Layer System Prompt Anatomy

System prompts must be organized into four distinct, semantically tagged layers to prevent attention dilution, eliminate ambiguity, and maintain high cognitive fidelity across frontier models:

1. `<identity>`: Concise, authoritative definition of the agent's identity, primary function, and operational scope. Avoid redundant conversational filler.
2. `<capabilities_and_tools>`: Explicit permissions, permitted tools, strictly prohibited tools, and execution boundaries.
3. `<workflow_protocol>`: Step-by-step operational procedure with deterministic ordering, state transitions, and checkable completion criteria.
4. `<hard_invariants>`: Non-negotiable negative boundaries, safety rules, and anti-patterns that must never be bypassed.

Shared operational policies that must remain identical across all roles (`Language Domain Contract`, `Delivery Guarantee`, `Format & Transport Separation`) are grouped under a canonical `<global_contracts>` section to maximize KV-cache reuse.

## Intent-Preserving Delegation Protocol (IPDP) & Non-Goals

Delegation is a sociotechnical transfer of authority and accountability, not merely mechanical task decomposition. To prevent **Cascade Amplification** (where orchestrator ambiguities compound down worker chains):
- Every minion dispatch envelope MUST define positive requirements along with explicit **negative boundaries (`non_goals`)**: paths not to touch, architectural patterns not to alter, and third-party dependencies not to introduce.
- Delegated subagents receive narrowed authority (principle of least privilege) and MUST NOT exceed the scope defined in `allowed_files` and `non_goals`.

## Double-Blind Adversarial Verification (Consensus Paradox Defense)

Multi-agent swarms inherently suffer from the **Consensus Paradox**, prioritizing internal sycophancy and architectural agreement over external logical truth. To preserve empirical rigor:
- Reviewers and evaluators must perform **Double-Blind Verification**: audit strictly against primary empirical artifacts (actual `git diff`, deterministic test suite exit codes, AST cycle checks, and contract pins).
- A reviewer MUST NEVER receive or rely on the implementer's self-assessed narrative, internal chain-of-thought, or claims of confidence. Self-reported success is advisory evidence only.

## Static-First KV-Cache Preservation

To maximize Transformer KV-cache reuse (achieving up to 90% latency and cost reduction):
- All invariant role prompts, schemas, and structural instructions must be placed at the very beginning of the prompt as a stable static prefix (`[STATIC_PREFIX_V3]`).
- Volatile, session-specific variables (`task_id`, diff contents, user instructions, timestamps) must strictly be placed at the tail of the context window or inside the dynamic dispatch envelope.
- Keys in structured JSON envelopes must be serialized deterministically in sorted order to avoid cache fragmentation.

## Structured ACI Failure Tracing

To prevent context window blowout from verbose compiler errors, stack traces, and linter outputs (Agent-Computer Interface principle):
- Subagents must extract bounded, structured failure traces rather than dumping raw logs.
- Failure traces must isolate the offending file, line number, error code, minimal reproduction command, and a focused diagnostic snippet bounded to $\le 25$ lines:
```xml
<failure_trace>
  <target_file>path/to/file.go</target_file>
  <location>line 42, col 8</location>
  <error_code>COMPILATION_ERROR</error_code>
  <minimal_repro>go test -v ./pkg/... -run TestName</minimal_repro>
  <diagnostic_snippet>
    cannot use x (variable of type string) as int in argument
  </diagnostic_snippet>
</failure_trace>
```

## Review gate

Before accepting an instruction change, verify that every new pointer has a real trigger, every normative rule has one source of truth, conditional detail is disclosed only when needed, and completion can be distinguished from premature stopping.

