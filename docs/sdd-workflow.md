# SDD Workflow

Spec-Driven Development (SDD) is the 9-phase workflow that the installed
skill, agent, and command assets implement. cortex-ia installs the assets;
it does not schedule or execute the pipeline — your OpenCode runtime, the
ForgeSpec MCP, and the Cortex MCP do the work.

## Authority Boundaries

| Owner | Responsibility |
|-------|----------------|
| **ForgeSpec** (MCP preset) | Contracts, task board, dependency readiness, claims, status, file reservation |
| **Cortex** (MCP preset) | Durable memory: evidence, decisions, summaries, provenance |
| OpenCode runtime | Skill/agent/command discovery and dispatch (transport only) |
| cortex-ia | Installs and updates the assets; configures the MCP bindings |

## Phase Map

| Phase | Skill | Installed agents | What happens |
|-------|-------|------------------|--------------|
| 0 · init | `bootstrap` | `bootstrap` | Detect the stack, bootstrap persistence, open the SDD session |
| 1 · explore | `investigate` | `investigate` | Map the codebase, compare approaches, rate effort/risk |
| 2 · propose | `draft-proposal` | `draft-proposal` | Change proposal with scope, risks, rollback plan |
| 3 · spec | `write-specs` | `write-specs` | Delta specs as Given/When/Then scenarios |
| 4 · design | `architect` | `architect` | Technical design with architecture decisions |
| 5 · tasks | `decompose` | `decompose` | Dependency-ordered task DAG on the ForgeSpec board |
| 6 · apply | `implement` | `implement` | One bounded work unit per minion: claim → execute → verify → receipt |
| 7 · verify | `validate` | `validate` | Run the scenario oracles, produce the compliance matrix |
| 8 · archive | `finalize` | `finalize` | Merge specs, close the change, archive the change set |

Cross-phase roles installed as agents and skills: `orchestrator` (the only
delegating role), `planner`, `reviewer`, `debate`, `code-review-adversary`,
`parallel-dispatch`.

## Commands

Installed under `~/.config/opencode/commands/`: `sdd`, `work`, `status`,
`resume`, `review`, `tdd`, `spike`, `investigate`, `hotfix`. They are
thin entry points that route into the phase skills above.

## Utility Skills

| Skill | Trigger |
|-------|---------|
| `fast-tdd`, `property-based-testing`, `mutation-testing`, `ast-impact-analysis` | Verification acceleration and adversarial test validation |
| `context-distiller` | Condensing verbose output into compact evidence |
| `spike-prototype` | Bounded uncertainty-reduction experiments |
| `hotfix-triage` | Incident containment with a strict diff |

## Contracts

Every phase transition is recorded as a ForgeSpec contract (init → explore
→ propose → spec → design → tasks → apply → verify → archive) and validated
by the ForgeSpec MCP server. The shared phase-contract documents under
`internal/assets/_shared/` in the repository are compile-time data for the
asset set; they are not installed as runtime files.

## Where the Assets Live

After `cortex-ia install`:

```text
~/.config/opencode/
  opencode.jsonc                # Merged config & managed MCP catalog
  AGENTS.md                     # System prompt (authority, routing & shell policy)
  agents/*.md                   # 5 native sub-agents (orchestrator, planner, implement, investigate, reviewer)
  commands/*.md                 # 9 slash commands (/sdd, /hotfix, /work, /tdd, /review, ...)
  skills/<name>/SKILL.md        # 12 native SDD & utility skills
  plugin/*.ts                   # 5 runtime plugins (background-supervisor, cortex, model-variants, ...)
```

Update them with `cortex-ia sync` after upgrading the binary.
