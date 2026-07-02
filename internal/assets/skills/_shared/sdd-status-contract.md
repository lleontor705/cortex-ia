# SDD Status Contract

Defines the structured status contract that every SDD phase returns to the orchestrator. Reference this file instead of duplicating these definitions in individual phase skills.

## Contract Shape

Every phase MUST return this contract (as a JSON block in its output, followed by a markdown report):

```json
{
  "status": "completed | blocked | working",
  "executive_summary": "...",
  "artifacts": [
    { "topic_key": "...", "type": "cortex | openspec | inline" }
  ],
  "next_recommended": "...",
  "risks": [
    { "description": "...", "level": "low | medium | high | critical" }
  ],
  "skill_resolution": "paths-injected | fallback-registry | fallback-path | none"
}
```

## Field Definitions

### status

The terminal state of this phase invocation.

| Value | Meaning | When to Use |
|-------|---------|-------------|
| `completed` | Phase finished successfully; artifact(s) persisted | The phase produced its deliverable and validation passed |
| `blocked` | Phase cannot proceed; requires orchestrator or user intervention | Missing dependency, failed validation after retries, ambiguous spec |
| `working` | Phase is mid-execution and will continue | Long-running phases that report intermediate progress (rare) |

Most phases return `completed` or `blocked`. `working` is reserved for phases that report checkpoint progress across multiple invocations.

### executive_summary

A concise (10+ chars) human-readable description of the phase outcome. Lead with the result, not the reasoning. Example: `"Spec written for auth-service with 7 requirements across 2 domains."`.

### artifacts

The list of artifacts this phase persisted. Each entry records where the artifact lives so downstream phases and the orchestrator can locate it.

| Field | Values |
|-------|--------|
| `topic_key` | The Cortex topic key (e.g., `sdd/add-auth/spec`) or filesystem path for openspec |
| `type` | `cortex` (persisted via `mem_save`), `openspec` (written to filesystem), `inline` (returned in message only) |

If the phase produced no persistable artifact (e.g., a pure investigation), this array may be empty.

### next_recommended

The single next phase the orchestrator should run. The orchestrator uses this as a strong hint but may override it based on dependency state, escalation triggers, or user direction.

Common values: `propose`, `spec`, `design`, `tasks`, `apply`, `verify`, `archive`, `resolve-blockers`, `halt`.

### risks

Issues that could affect downstream phases or require attention. Each risk has a description and a level.

| Level | Meaning |
|-------|---------|
| `low` | Minor concern; proceed normally |
| `medium` | May cause rework; flag for review |
| `high` | Likely to block a downstream phase |
| `critical` | Will block; halts the pipeline until resolved |

An empty `risks` array is valid when the phase encountered no issues.

### skill_resolution

Reports whether the orchestrator's skill injection succeeded for this delegation. This is the feedback loop for the Skill Resolver Protocol (see `skill-resolver.md` in this directory).

| Value | Meaning |
|-------|---------|
| `paths-injected` | Matching skill paths were passed in the prompt and loaded successfully |
| `fallback-registry` | Skill cache was lost; sub-agent re-read the registry from Cortex |
| `fallback-path` | Skill cache was lost; sub-agent fell back to `.sdd/skill-registry.md` |
| `none` | No registry available; sub-agent proceeded without project-specific skills |

`fallback-*` and `none` values indicate the orchestrator dropped context (likely compaction). The orchestrator re-reads the registry and refreshes its skill cache before the next delegation.

## How the Orchestrator Uses This Contract

After each phase returns, the orchestrator:

```
1. Read the status contract from the phase output
2. If status == "blocked":
   - Inspect risks for the blocking reason
   - Report blockedReasons to the user
   - Do NOT proceed to the next phase
3. If status == "completed":
   - Record the artifacts for dependency checks
   - Read next_recommended
   - Validate dependencies for the recommended next phase exist
   - If dependencies met: launch the next phase
   - If dependencies missing: halt and report
4. Check skill_resolution:
   - If fallback-* or none: re-read skill registry, refresh cache
5. Apply escalation triggers based on confidence and risk levels
```

## Persistence

The status contract is also persisted as part of the SDD contract lifecycle via ForgeSpec:

1. `sdd_validate(contract: {json})` — validates the full SDD contract structure (which embeds this status contract)
2. `sdd_save(contract: {json})` — persists to the ForgeSpec store

The SDD contract wraps this status contract with additional fields: `schema_version`, `phase`, `change_name`, `project`, `confidence`. See `cortex-convention.md` (Contract Persistence Protocol) for the full contract schema.

## Confidence Thresholds

Each phase has a minimum confidence for the orchestrator to accept the result without escalation:

| Phase | Minimum Confidence |
|-------|--------------------|
| bootstrap / explore | 0.5 |
| propose / design | 0.7 |
| spec / tasks | 0.8 |
| apply | 0.6 |
| verify / archive | 0.9 |

Below the threshold, the orchestrator checks escalation triggers and may warn the user or retry the phase.
