# Skill Resolver Protocol

Defines how the orchestrator resolves which skills to load before delegating work to sub-agents. Reference this file instead of duplicating these rules in individual skills.

## Purpose

Sub-agents run with a fresh context and no memory. The orchestrator is responsible for deciding which `SKILL.md` files each sub-agent should read before starting task-specific work. This protocol makes that decision deterministic and auditable.

The resolver runs ONCE per session, caches the result, and reuses the cache for every delegation. If the cache is lost (compaction), the resolver re-reads the registry and rebuilds the cache.

## Resolution Steps

```
1. Orchestrator resolves skills from the registry ONCE (session start or first delegation)
2. Caches the skill index: skill name, trigger/description, scope, exact path
3. For each sub-agent launch, matches relevant skills by:
   a. Code context  — file extensions / paths the sub-agent will touch
   b. Task context   — actions it will perform (review, PR creation, testing, etc.)
4. Copies matching SKILL.md paths into the sub-agent prompt as "Skills to load before work"
5. Sub-agents read those exact files BEFORE task-specific work
```

Key rule: pass paths, not generated summaries. Sub-agents read the full `SKILL.md` files so author intent is preserved. Summarizing a skill in the orchestrator prompt loses the structure and detail the skill author wrote.

## Registry Source

The skill registry is the source of truth for which skills exist and when they apply.

| Source | Location | When Used |
|--------|----------|-----------|
| Cortex (preferred) | `mem_search(query: "skill-registry", project: "{project}")` → `mem_get_observation(id)` | Cortex MCP available |
| Filesystem fallback | `.sdd/skill-registry.md` (project root) | Cortex unavailable or registry not persisted |
| None | — | No registry exists; proceed without project-specific standards |

If no registry exists, warn the user and proceed without project-specific skills. This is not an error — log a note recommending `/bootstrap` to generate the registry.

`mem_search` returns 300-character previews. Call `mem_get_observation(id)` for the full registry content. Working with the preview leads to incomplete skill matching.

## Cached Skill Index

After the first read, the orchestrator caches:

| Field | Example |
|-------|---------|
| Skill name | `branch-pr` |
| Trigger / description | "When creating a pull request or opening a PR" |
| Scope | project |
| Exact path | `internal/assets/skills/branch-pr/SKILL.md` |

The cache is session-scoped. It is reused for every subsequent delegation without re-reading the registry.

## Matching Heuristics

Match skills by BOTH axes — a skill matches only if it is relevant on both code context and task context.

### Code Context (what files the sub-agent touches)

| Signal | Example match |
|--------|---------------|
| File extension | `*.go` → `go-testing` |
| Directory / path | `internal/assets/skills/**` → `skill-creator`, `skill-improver` |
| Glob pattern | `docs/**` → `cognitive-doc-design` |

### Task Context (what actions the sub-agent performs)

| Signal | Example match |
|--------|---------------|
| Review / audit | `judgment-day`, `validate` |
| PR creation | `branch-pr`, `open-pr` |
| Testing | `go-testing` |
| Refactoring | `parallel-dispatch` |

If no skill matches, the sub-agent proceeds with only the cortex-convention and any phase-specific skill passed by the orchestrator.

## Sub-Agent Prompt Injection

For each matched skill, the orchestrator adds a section to the sub-agent prompt:

```
## Skills to load before work

Read these exact files BEFORE starting task-specific work:

- {exact-path-to-SKILL.md}
- {exact-path-to-SKILL.md}
```

The sub-agent reads each file at startup (per the Skill Loading Protocol in `cortex-convention.md`). If a loaded skill declares `requires` in frontmatter, the sub-agent loads those dependency skills first.

## Feedback Loop

After every delegation returns, the orchestrator inspects the `skill_resolution` field in the sub-agent's status contract:

| Value | Meaning | Action |
|-------|---------|--------|
| `paths-injected` | All matching skill paths were passed and loaded | None — cache intact |
| `fallback-registry` | Skill cache was lost; sub-agent re-read the registry from Cortex | Re-read the registry and refresh the orchestrator cache |
| `fallback-path` | Skill cache was lost; sub-agent fell back to `.sdd/skill-registry.md` | Same as above |
| `none` | No registry available | Proceed without skills; warn user |

Fallback values indicate the orchestrator dropped context (likely due to compaction). Do NOT ignore them — re-read the registry immediately and pass skill paths in all subsequent delegations.

See `sdd-status-contract.md` in this directory for the full status contract format.
