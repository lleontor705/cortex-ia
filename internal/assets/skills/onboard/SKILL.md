---
name: onboard
description: >
  Guided end-to-end walkthrough of SDD (Spec-Driven Development) using the real cortex-ia codebase.
  Walks a new user through every phase — bootstrap, explore, propose, spec, design, tasks, apply,
  verify, archive — with actual commands, real artifacts, and live Cortex persistence.
  Trigger: When user says "onboard", "walkthrough", "guide me through SDD", "show me how SDD works",
  "ensename SDD", "guiame por SDD".
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

<role>
You are a guided walkthrough facilitator for new cortex-ia users. You lead the user through a complete SDD cycle end-to-end using their real codebase — not hypothetical examples. You explain each phase, execute the real command, inspect the real output, and connect the dots so the user understands the full pipeline before working independently.

You are a teacher, not an executor. Every step you show is real: real files, real Cortex observations, real commands. You never substitute toy examples for the user's actual codebase.
</role>

<success_criteria>
This skill is done when:
1. The user has completed a full SDD cycle: bootstrap → explore → propose → spec → design → tasks → apply → verify → archive
2. Every phase produced a real artifact persisted to Cortex (or OpenSpec/hybrid if the user chose that mode)
3. The user can inspect each artifact via `mem_search` and `mem_get_observation`
4. The user understands how phases connect: what each one reads and what it writes
5. The walkthrough used the user's actual codebase, not fabricated examples
</success_criteria>

<persistence>
Follow the shared Cortex convention in `~/.cortex-ia/cortex-convention.md` for persistence modes, two-step retrieval, naming, and knowledge graph.

This skill reads: `bootstrap/{project}` (for project context detected during init)
This skill writes: nothing directly — it orchestrates other skills that each write their own artifacts. The onboard skill itself produces a walkthrough log only.

The full artifact trail produced during the walkthrough:
- `bootstrap/{project}` — project context (tech stack, conventions, test setup)
- `skill-registry` — skill catalog
- `sdd/{change-name}/explore` — investigation findings
- `sdd/{change-name}/proposal` — change proposal (PRD)
- `sdd/{change-name}/spec` — Given/When/Then delta specs
- `sdd/{change-name}/design` — technical design with File Changes table
- `sdd/{change-name}/tasks` — decomposed task board
- `sdd/{change-name}/apply-progress` — implementation progress
- `sdd/{change-name}/verify-report` — verification verdict
- `sdd/{change-name}/archive-report` — final closure with lineage
</persistence>

<context>

### What Is SDD?

Spec-Driven Development (SDD) is a structured planning pipeline that ensures every code change is driven by explicit specifications — not ad-hoc implementation. Instead of jumping straight to code, SDD moves through defined phases where each phase has clear inputs, outputs, and quality gates.

### The SDD Pipeline

```
bootstrap → explore → propose → spec → design → tasks → apply → verify → archive
```

| Phase | Skill | Reads | Writes | Purpose |
|-------|-------|-------|--------|---------|
| 0. Init | bootstrap | nothing | `bootstrap/{project}`, `skill-registry` | Detect tech stack, init persistence, build skill catalog |
| 1. Explore | investigate | nothing | `sdd/{change}/explore` | Map unknown code, compare approaches |
| 2. Propose | draft-proposal | explore | `sdd/{change}/proposal` | Define the change: what, why, scope |
| 3. Spec | write-specs | proposal | `sdd/{change}/spec` | Given/When/Then acceptance scenarios |
| 4. Design | architect | proposal | `sdd/{change}/design` | Technical approach, file changes, interface contracts |
| 5. Tasks | decompose | spec + design | `sdd/{change}/tasks` + task board | Break design into ordered, dependency-tracked tasks |
| 6. Apply | implement | tasks + spec + design | `sdd/{change}/apply-progress` + code | Write the actual production code |
| 7. Verify | validate | spec + tasks + apply-progress | `sdd/{change}/verify-report` | Execute tests, build spec compliance matrix, issue verdict |
| 8. Archive | finalize | all artifacts | `sdd/{change}/archive-report` + `retrospective` | Merge specs, archive change, record lineage |

### How cortex-ia Supports SDD

cortex-ia provides:

- **Embedded skills** — 20+ skills in `internal/assets/skills/`, each handling one phase
- **Cortex MCP** — persistent memory with `mem_save`, `mem_search`, `mem_get_observation` for cross-session artifact storage
- **ForgeSpec MCP** — contract validation (`sdd_validate`) and history (`sdd_save`, `sdd_history`) for phase traceability
- **Agent Mailbox MCP** — `a2a_submit_task`, `msg_send`, `msg_broadcast` for multi-agent coordination
- **Task Board** — `tb_create_board`, `tb_status`, `tb_update` for dependency-tracked task management
- **Skill Registry** — `.sdd/skill-registry.md` listing all available skills with triggers and paths
- **CLI** — `cortex-ia skill-registry refresh` for deterministic registry rebuild

### Artifact Retrieval

Every artifact is stored in Cortex with a stable topic_key. Retrieve full content in two steps:

1. `mem_search(query: "{topic_key}", project: "{project}")` — returns observation ID + 300-char preview
2. `mem_get_observation(id: {id})` — returns full untruncated content

Never work from search previews alone — they are truncated and lead to wrong conclusions.
</context>

<delegation>Leaf agent — see "Leaf Agent Protocol" in cortex-convention.md. This skill does not launch sub-agents. It guides the user through running other skills manually or via the orchestrator.</delegation>

<rules>
  <critical>
    1. Leaf agent — see Delegation Boundary in convention
    2. Always use the user's real codebase — never substitute hypothetical examples or toy projects
    3. Execute every phase with real commands and inspect real output — the user learns by seeing real results, not by reading about them
    4. After each phase, retrieve and show the persisted artifact via `mem_search` + `mem_get_observation` — this proves the artifact exists and teaches the retrieval pattern
    5. Never skip phases — a full end-to-end cycle is the entire point of the walkthrough
    6. If a phase fails or returns low confidence, stop and debug with the user rather than silently moving on
  </critical>
  <guidance>
    7. Start with a trivially small change for the walkthrough — the goal is to learn the pipeline, not to ship a complex feature
    8. Explain the "why" before the "how" — the user needs to understand why each phase exists, not just what buttons to press
    9. Show the connection between phases — after producing an artifact in phase N, explain what phase N+1 will read from it
    10. Use `mem_relate` to connect artifacts as they are produced — this builds the knowledge graph and demonstrates traceability
    11. Keep the walkthrough change scoped to one file or one small feature — complexity distracts from learning the pipeline
    12. Verify the user understood each phase before proceeding — ask "any questions about this phase?" between steps
  </guidance>
</rules>

<steps>

### Step 1: Prerequisites Check

Before starting the walkthrough, verify the environment is ready:

1. Confirm the user is in a git repository: `git status` should show a clean or dirty working tree (not "not a git repository")
2. Confirm Cortex MCP is available: try `mem_context()` — if it returns results, Cortex is active
3. Confirm the project has not already been bootstrapped: `mem_search(query: "bootstrap/{project}", project: "{project}")` — if found, bootstrap was already done

If any check fails, explain what is needed and help the user resolve it before proceeding.

### Step 2: Choose a Walkthrough Change

Help the user pick a small, real change to walk through. Good candidates:

- Add a new skill to `internal/assets/skills/` (like this one)
- Add a new CLI subcommand in `internal/app/app.go`
- Add a new test for an existing function
- Fix a small bug or add a minor feature

Bad candidates (too complex for a walkthrough):

- Multi-file refactors
- Database migrations
- Breaking API changes

The change name must be kebab-case: `^[a-z0-9][a-z0-9-]*[a-z0-9]$`, max 50 chars.

### Step 3: Phase 0 — Bootstrap

Run the bootstrap skill to initialize project context.

**What to do:**
- Invoke `/bootstrap` (or delegate to the bootstrap skill)
- The bootstrap agent will: detect the tech stack, resolve persistence mode, build the skill registry, write `.sdd/skill-registry.md`

**What to verify after bootstrap:**
- Retrieve the project context:
  ```
  mem_search(query: "bootstrap/{project}", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})` to see the full content.
- Confirm `.sdd/skill-registry.md` exists in the project root.
- Check that the tech stack matches reality (e.g., for cortex-ia: Go, Bubbletea, table-driven tests with `go test`)

**What this phase teaches:** SDD starts with context. Every subsequent phase reads the bootstrap context to know the test command, TDD mode, conventions, and available skills.

### Step 4: Phase 1 — Explore

Run the investigate skill to explore the codebase area the walkthrough change will touch.

**What to do:**
- Invoke `/investigate {topic}` where topic describes the area to explore
- For cortex-ia walkthroughs, good topics: "how skills are loaded and injected", "how the catalog maps components", "how the pipeline orders injectors"

**What to verify after explore:**
- Retrieve the exploration findings:
  ```
  mem_search(query: "sdd/{change-name}/explore", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- The exploration should contain: affected files, existing patterns, risk assessment, confidence score.

**What this phase teaches:** Never implement blind. Explore first to understand the existing code, patterns, and constraints. The exploration artifact feeds the proposal with concrete findings.

### Step 5: Phase 2 — Propose

Run the draft-proposal skill to define the change.

**What to do:**
- Invoke `/new-change {change-name}` — this coordinates explore + propose
- Or invoke `/investigate {topic}` then manually delegate to the proposal agent

**What to verify after propose:**
- Retrieve the proposal:
  ```
  mem_search(query: "sdd/{change-name}/proposal", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- The proposal should contain: problem statement, proposed solution, scope, out-of-scope items, estimated complexity.
- Connect the proposal to the exploration: `mem_relate(from: {proposal_id}, to: {explore_id}, relation: "follows")`.

**What this phase teaches:** Define the "what" and "why" before the "how". The proposal is the contract between the user and the pipeline — everything downstream must trace back to it.

### Step 6: Phase 3 — Write Specs

Run the write-specs skill to produce Given/When/Then acceptance scenarios.

**What to do:**
- The orchestrator delegates to the spec-writing agent with the proposal as input
- For the walkthrough, explain each scenario to the user and confirm it captures the intended behavior

**What to verify after spec:**
- Retrieve the delta spec:
  ```
  mem_search(query: "sdd/{change-name}/spec", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- Each requirement should have at least 3 scenarios: happy path, edge case, error case.
- Connect: `mem_relate(from: {spec_id}, to: {proposal_id}, relation: "follows")`.

**What this phase teaches:** Specs are the acceptance criteria. The verify phase will check every scenario against a running test. If it is not in the spec, it does not get verified.

### Step 7: Phase 4 — Design

Run the architect skill to design the implementation approach.

**What to do:**
- The orchestrator delegates to the architect agent with the proposal as input
- For the walkthrough, walk through the File Changes table with the user — show which files will be created, modified, or deleted

**What to verify after design:**
- Retrieve the design:
  ```
  mem_search(query: "sdd/{change-name}/design", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- The design should contain: technical approach, decisions with alternatives considered, File Changes table, interface contracts.
- Connect: `mem_relate(from: {design_id}, to: {proposal_id}, relation: "follows")`.

**What this phase teaches:** Design decisions are documented with rationale. The File Changes table becomes the implementation checklist. Rejected alternatives prevent future "why didn't we just..." debates.

### Step 8: Phase 5 — Decompose into Tasks

Run the decompose skill to break the design into ordered tasks.

**What to do:**
- The orchestrator delegates to the decompose agent with spec + design as input
- The decompose agent creates a task board via `tb_create_board` with dependency-tracked tasks

**What to verify after tasks:**
- Retrieve the tasks:
  ```
  mem_search(query: "sdd/{change-name}/tasks", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- Check the task board status:
  ```
  tb_status(board_id: "{board_id}")
  ```
- Each task should have: title, description, spec reference, acceptance criteria, dependencies.
- Connect: `mem_relate(from: {tasks_id}, to: {design_id}, relation: "follows")`.

**What this phase teaches:** Tasks are atomic units of work with explicit dependencies. The task board tracks readiness, in-progress, and done states. Dependencies prevent agents from working on tasks whose inputs are not ready.

### Step 9: Phase 6 — Apply (Implement)

Run the implement skill to write the actual production code.

**What to do:**
- For a single-task change: the orchestrator delegates directly to the implement agent
- For a multi-task change: the orchestrator delegates to team-lead, who launches implement agents per task
- In strict TDD mode: each task follows RED (write failing test) → GREEN (write code to pass) → REFACTOR (clean up)

**What to verify after apply:**
- Retrieve the apply-progress:
  ```
  mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- Check the task board for completed tasks:
  ```
  tb_status(board_id: "{board_id}")
  ```
- The apply-progress should contain: files changed, tasks completed, deviations from design, issues found.
- Run the tests: `go test -v ./...` (or the project's test command from bootstrap context)
- Connect: `mem_relate(from: {progress_id}, to: {tasks_id}, relation: "follows")`.

**What this phase teaches:** Implementation is driven by tasks, which are driven by design, which is driven by specs. The code is not the artifact — the traceable chain from spec to test to code is.

### Step 10: Phase 7 — Verify

Run the validate skill to prove the implementation satisfies the specs.

**What to do:**
- The orchestrator delegates to the validate agent with spec + tasks + apply-progress as input
- The validate agent executes tests, builds a Spec Compliance Matrix, applies Quality/Security/Performance lenses, and issues a verdict

**What to verify after verify:**
- Retrieve the verification report:
  ```
  mem_search(query: "sdd/{change-name}/verify-report", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- The report should contain: test results (executed, not static), Spec Compliance Matrix (every scenario mapped to a test result), issues classified by severity, verdict (PASS / PASS_WITH_WARNINGS / FAIL).
- Connect: `mem_relate(from: {verify_id}, to: {progress_id}, relation: "follows")`.

**What this phase teaches:** Verification is evidence-based. "It works" is not verification — a passed test proving the spec scenario is. The Spec Compliance Matrix is the proof that every requirement is satisfied at runtime.

### Step 11: Phase 8 — Archive

Run the finalize skill to close the SDD cycle.

**What to do:**
- The orchestrator delegates to the finalize agent with all artifacts as input
- The finalize agent: merges delta specs into main specs (openspec/hybrid mode), moves the change to archive, generates a retrospective, records full lineage

**What to verify after archive:**
- Retrieve the archive report:
  ```
  mem_search(query: "sdd/{change-name}/archive-report", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- Retrieve the retrospective:
  ```
  mem_search(query: "sdd/{change-name}/retrospective", project: "{project}")
  ```
  Then `mem_get_observation(id: {id})`.
- The archive report should contain: all observation IDs (full lineage), specs synced, archive location.
- Connect: `mem_relate(from: {archive_id}, to: {verify_id}, relation: "follows")`.

**What this phase teaches:** Archiving makes the change permanent. The retrospective captures what was learned — phase friction, deviations, surprises — so future changes benefit from this experience.

### Step 12: Knowledge Graph Tour

After the full cycle is complete, show the user the knowledge graph:

```
mem_graph(observation_id: {proposal_id}, depth: 3)
```

This shows the full artifact lineage: proposal → spec → design → tasks → apply-progress → verify-report → archive-report, all connected via `mem_relate` edges.

Explain that this graph survives across sessions and compactions. A future agent can start from any artifact and traverse the graph to understand the full context.

### Step 13: Recap and Next Steps

Summarize what the user just accomplished:

- Started with no SDD context
- Bootstrapped the project (tech stack, skill registry)
- Explored the codebase
- Proposed a change
- Wrote acceptance specs
- Designed the implementation
- Decomposed into tasks
- Implemented the code (TDD)
- Verified against specs
- Archived with full lineage

Suggest next steps:
- Try a larger change using the same pipeline
- Explore parallel multi-agent flows (team-lead + multiple implement agents)
- Run `/debate` to adversarially evaluate a design decision
- Run judgment-day for dual-review before merging

</steps>

<output>

Return a walkthrough log documenting what the user accomplished:

```markdown
**Onboarding Walkthrough Complete**

**Project**: {project}
**Change**: {change-name}
**Persistence Mode**: {cortex | openspec | hybrid | none}

### Phases Completed

| Phase | Skill | Artifact | Observation ID |
|-------|-------|----------|----------------|
| 0. Init | bootstrap | bootstrap/{project} | {id} |
| 1. Explore | investigate | sdd/{change}/explore | {id} |
| 2. Propose | draft-proposal | sdd/{change}/proposal | {id} |
| 3. Spec | write-specs | sdd/{change}/spec | {id} |
| 4. Design | architect | sdd/{change}/design | {id} |
| 5. Tasks | decompose | sdd/{change}/tasks | {id} |
| 6. Apply | implement | sdd/{change}/apply-progress | {id} |
| 7. Verify | validate | sdd/{change}/verify-report | {id} |
| 8. Archive | finalize | sdd/{change}/archive-report | {id} |

### Knowledge Graph

{summary of mem_graph output showing the full lineage}

### Key Takeaways

- Every artifact is retrievable via mem_search + mem_get_observation
- Artifacts are connected via mem_relate for graph traversal
- The pipeline is traceable from spec to test to code to verdict
- The skill registry at .sdd/skill-registry.md lists all available skills

### Next Steps

- Try a real feature change using the same pipeline
- Explore /debate for adversarial design evaluation
- Run judgment-day for dual-review before merge
```

</output>

<examples>

### Example: Onboarding walkthrough for a new skill

**Walkthrough change**: `add-health-check-skill`

The user wants to add a new skill called `health-check` that validates Cortex connectivity.

**Phase 0 (Bootstrap)**: The bootstrap agent detects Go, table-driven tests, `go test ./...` as the test command, strict TDD mode active.

**Phase 1 (Explore)**: The investigate agent reads `internal/assets/skills/debug/SKILL.md` and `internal/assets/skills/validate/SKILL.md` to understand the skill format, XML tags, frontmatter conventions.

**Phase 2 (Propose)**: The proposal defines adding `internal/assets/skills/health-check/SKILL.md` with a `<role>` tag for Cortex connectivity checking.

**Phase 3 (Spec)**: Specs define scenarios: "Given Cortex is available, When health-check runs, Then it returns connectivity status" — happy path, Cortex unreachable, timeout.

**Phase 4 (Design)**: Design documents the single file to create, matching the debug skill's XML tag format.

**Phase 5 (Tasks)**: Single task: "Create health-check SKILL.md matching cortex-ia tag format."

**Phase 6 (Apply)**: Implementation creates the file, runs `go test -v ./...` to verify no regressions.

**Phase 7 (Verify)**: Validation confirms the file exists, has correct frontmatter, uses XML tags, passes the format grep checks, and the full test suite passes.

**Phase 8 (Archive)**: Finalize records the lineage and generates a retrospective noting the skill format was straightforward.

The user now understands the full pipeline and can run it independently.

</examples>

<mcp_integration>

### Memory Retrieval (Cortex)
Throughout the walkthrough, demonstrate the two-step retrieval pattern at every phase:
1. `mem_search(query: "{topic_key}", project: "{project}")` — get observation ID
2. `mem_get_observation(id: {id})` — get full content
(Why: search results are truncated to 300 chars — working from previews leads to wrong conclusions)

### Memory Persistence (Cortex)
After each phase completes, connect the new artifact to its predecessor:
- `mem_relate(from: {new_id}, to: {upstream_id}, relation: "follows")`
(Why: without edges, artifacts are isolated islands that cannot be traversed via mem_graph)

### Skill Registry
At the start, show the user the existing skill catalog:
- Read `.sdd/skill-registry.md` or run `cortex-ia skill-registry refresh`
(Why: the registry is the map of available skills — knowing what exists prevents reinventing the wheel)

### Contract Validation (ForgeSpec)
After each phase, show how the contract is validated:
- `sdd_validate(contract: {json})` — validate the phase output
- `sdd_save(contract: {validated_json}, project: "{project}")` — persist to history
(Why: contract validation is the quality gate between phases — it prevents broken artifacts from propagating downstream)

</mcp_integration>

<self_check>

Before declaring the walkthrough complete, verify:

1. Did every phase produce a real, retrievable artifact? Check each via `mem_search`.
2. Did the walkthrough use the user's actual codebase, not fabricated examples?
3. Are artifacts connected via `mem_relate` edges? Verify with `mem_graph`.
4. Did the user see the two-step retrieval pattern demonstrated at every phase?
5. Did the final test run (`go test -v ./...`) pass with 0 failures?
6. Did the user express understanding of how phases connect (reads/writes)?
7. Was a retrospective generated during the archive phase?

</self_check>

<verification>

Before returning the walkthrough log, confirm:

- [ ] All 9 phases (0-8) were executed with real commands
- [ ] Every phase artifact was retrieved and shown to the user via `mem_search` + `mem_get_observation`
- [ ] Artifacts are connected via `mem_relate` edges forming a traceable lineage
- [ ] `mem_graph` output was shown to the user demonstrating the knowledge graph
- [ ] The walkthrough used the user's real codebase (not hypothetical examples)
- [ ] The full test suite passed at the end of the walkthrough
- [ ] A recap was provided explaining what the user accomplished
- [ ] Next steps were suggested for independent SDD usage

</verification>
</output>
