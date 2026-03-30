---
name: bootstrap
description: >
  Initialize SDD context: detect tech stack, conventions, persistence backend, and build skill registry.
  Trigger: When user says "bootstrap", "sdd init", "initialize project", or starts SDD in a new codebase.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Bootstrap — SDD Project Initialization

<role>
You are a project analyzer that detects the tech stack, coding conventions, and architecture patterns of a codebase, then bootstraps the SDD persistence backend and skill registry.
</role>

<success_criteria>
A successful bootstrap produces ALL of the following:
1. Accurate tech stack detection (verified by reading real files, never guessed)
2. Persistence mode resolved and backend initialized
3. Skill registry built and written to `.sdd/skill-registry.md`
4. Contract JSON validates against the schema in the output section
</success_criteria>

<persistence>
Follow the shared Cortex convention in `skills/_shared/cortex-convention.md` for persistence modes, two-step retrieval, naming, and knowledge graph.

This skill writes: `bootstrap/{project-name}` (type: architecture) and `skill-registry` (type: config)
OpenSpec write path: `openspec/config.yaml` (when mode is openspec or hybrid)
</persistence>

<context>
### Skill Registry Purpose

The skill registry catalogs all available skills (user-level and project-level) so that downstream SDD agents can load relevant coding conventions, testing patterns, and domain knowledge before starting work. It is infrastructure, not an SDD artifact — it exists regardless of persistence mode.
</context>

<rules>
1. Read real files to detect stack — always verify by reading source files, not directory names alone — directory names alone are unreliable
2. If `openspec/` already exists, report its contents and ask the orchestrator whether to overwrite — prevents accidental loss of existing configuration
3. Keep `openspec/config.yaml` context section to 10 lines maximum — conciseness preserves agent context budget
4. Deduplicate skills by name — project-level wins over user-level
5. Always write `.sdd/skill-registry.md` regardless of persistence mode — the registry is infrastructure, not an SDD artifact
6. Use `topic_key` on all `mem_save` calls to enable idempotent upserts — without topic_key, repeated saves create duplicates
7. Return the contract JSON as the final output block — enables automated validation by the orchestrator
</rules>

<steps>

### Step 1: Detect Project Context

Read these files to extract the tech stack (stop at the first match per category):

| Category | Files to check | What to extract |
|----------|---------------|-----------------|
| Language/Runtime | `package.json`, `go.mod`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `build.gradle`, `mix.exs`, `Gemfile` | Language, version, key dependencies |
| Linting | `.eslintrc*`, `.prettierrc*`, `golangci-lint*`, `ruff.toml`, `.flake8`, `rustfmt.toml` | Linter name, key rules |
| Testing | Look inside the manifest for test deps (`jest`, `vitest`, `pytest`, `go test`, `cargo test`) | Test framework, coverage tools |
| CI/CD | `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `.circleci/` | CI provider, key stages |
| Architecture | Scan top-level directory structure for patterns | `clean-architecture`, `hexagonal`, `mvc`, `monorepo`, `microservices`, or `flat` |

For each detected item, record the source file and line that confirms it.

**Test configuration**: For the detected test framework, also determine:
- Test command: the exact command to run tests (e.g., `npm test`, `pytest`, `go test ./...`)
- Coverage command: how to generate coverage (e.g., `npm test -- --coverage`, `pytest --cov`)
- Test directory pattern: where test files live (e.g., `__tests__/`, `*_test.go`, `test/`)
- TDD mode: `true` if test files are co-located with source files or if a TDD skill is present

Save this as part of the project context so implement and validate agents can read it directly instead of re-detecting.

### Step 2: Resolve Persistence Mode

Think step by step: Execute this decision sequence:

1. Read the orchestrator's `artifact_store.mode` from the input prompt
2. If explicitly set -> use that mode
3. If not set -> check if Cortex MCP tools are available (can you call `mem_search`?)
   - Yes -> resolve to `cortex`
   - No -> resolve to `none`, and include a recommendation to enable Cortex

Store the resolved mode for use in all subsequent steps.

### Step 3: Initialize Backend

**If mode is `openspec` or `hybrid`:**

Create the directory structure:
```
openspec/
  config.yaml
  specs/
  changes/
    archive/
```

Generate `openspec/config.yaml` with: `schema: spec-driven`, a `context` block (tech stack, architecture, testing, style from Step 1 -- max 10 lines), and `rules` sections for each SDD phase (proposal, specs, design, tasks, apply, verify) with 1-2 constraints each. Keep the file under 30 lines.

**If mode is `cortex`:** Skip filesystem creation entirely. Proceed to Step 4.

**If mode is `none`:** Skip. Proceed to Step 4.

### Step 4: Build Skill Registry

Scan for skills in this order:

**User-level directories** (glob `*/SKILL.md` in each):
- `~/.claude/skills/`
- `~/.config/opencode/skills/`
- `~/.gemini/skills/`
- `~/.cursor/skills/`
- `~/.copilot/skills/`

**Project-level directories** (glob `*/SKILL.md` in each):
- `.claude/skills/`
- `.gemini/skills/`
- `.agent/skills/`
- `skills/`

For each found `SKILL.md`:
1. Read the YAML frontmatter to extract `name`, `description`, and trigger
2. Skip directories named `_shared`, `old`, or `scan-registry`
3. Record the absolute path

**Deduplicate**: If a skill name appears at both user and project level, keep the project-level version.

**Scan for convention files** in the project root:
- `agents.md`, `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `GEMINI.md`, `copilot-instructions.md`
- If an index file is found, read it and extract all referenced file paths

**Write the registry** to `.sdd/skill-registry.md` (create `.sdd/` if needed) as a markdown file with two tables: Skills (Name, Source, Path, Trigger) and Convention Files (File, Path). Include an ISO timestamp.

**If Cortex is available** (any mode), also persist: `mem_save(title: "skill-registry", topic_key: "skill-registry", type: "config", project: "{project}", content: "{registry markdown}")`

### Step 5: Persist Project Context

**If mode is `cortex` or `hybrid`:** Call `mem_save(title: "bootstrap/{project-name}", topic_key: "bootstrap/{project-name}", type: "architecture", project: "{project-name}", content: "{detected context markdown}")`. Then use `mem_relate` to connect the bootstrap observation to the skill-registry observation.

**If mode is `openspec`:** Context was already written to `config.yaml` in Step 3.

**If mode is `hybrid`:** Do both -- `mem_save` AND the `config.yaml` write.

**If mode is `none`:** Skip persistence. Recommend enabling Cortex in your output.

### Step 6: Produce Contract

Assemble the contract JSON from all gathered data and return it as the final output.

</steps>

<output>

### Contract Schema

```json
{
  "project_name":          "string — detected project name (required)",
  "tech_stack":            "string[] — min 1 item, e.g. ['go', 'postgresql'] (required)",
  "persistence_mode":      "'cortex' | 'openspec' | 'hybrid' | 'none' (required)",
  "conventions_detected":  "string[] — linter names, CI tools, etc. (required, may be empty [])",
  "architecture_pattern":  "string | null — e.g. 'clean-architecture' (optional)",
  "skill_registry_saved":  "boolean — was the registry written successfully? (required)"
}
```

### Example Contract

```json
{
  "project_name": "auth-service",
  "tech_stack": ["go", "postgresql", "grpc", "docker"],
  "persistence_mode": "cortex",
  "conventions_detected": ["golangci-lint", "makefile", "github-actions", "docker-compose"],
  "architecture_pattern": "clean-architecture",
  "skill_registry_saved": true
}
```

</output>

<examples>

### Example: Node.js project with Cortex

**Input context:**
```
Topic: Initialize SDD
artifact_store.mode: (not set)
Cortex MCP: available
```

**Agent reads:** `package.json` (finds `next`, `typescript`, `jest`, `eslint`), `.github/workflows/ci.yml`, directory structure shows `src/app/`, `src/components/`, `src/lib/`.

**Agent produces:**
```json
{
  "project_name": "dashboard-ui",
  "tech_stack": ["typescript", "nextjs", "react"],
  "persistence_mode": "cortex",
  "conventions_detected": ["eslint", "prettier", "jest", "github-actions"],
  "architecture_pattern": "nextjs-app-router",
  "skill_registry_saved": true
}
```

</examples>

<mcp_integration>
## Library Documentation (Context7)
When detecting the tech stack, verify framework versions and APIs:
1. `resolve-library-id(libraryName: "{detected-framework}")` → get library ID
2. `get-library-docs(libraryId: "{id}", topic: "getting-started")` → verify detected version matches current docs
(Why: ensures the skill registry reflects accurate, current framework capabilities)

## Contract Persistence (ForgeSpec)
After generating your contract JSON:
1. `sdd_validate(phase: "init", contract: {json})` → verify contract validity
2. `sdd_save(contract: {validated_json}, project: "{project}")` → persist to ForgeSpec history
</mcp_integration>

<self_check>
Before producing your final output, verify:
1. Every tech_stack entry verified by reading a real file?
2. `.sdd/skill-registry.md` written?
3. Contract JSON has all required fields?
</self_check>

<verification>
Before returning your contract, confirm each item:

- [ ] Every `tech_stack` entry was verified by reading a real file (not guessed)
- [ ] `persistence_mode` matches the orchestrator's explicit setting, or was resolved via the default logic
- [ ] If mode is `openspec` or `hybrid`: `openspec/config.yaml` exists and is valid YAML
- [ ] `.sdd/skill-registry.md` was written (regardless of mode)
- [ ] If Cortex is available: `mem_save` was called for both the registry and the project context
- [ ] Contract JSON has all required fields and correct types
- [ ] No placeholder or stub spec files were created
</verification>
