# Skill Token-Budget Audit — 2026-10-09

- **Scope**: every `SKILL.md` body under `internal/assets/skills/` (19 files), frontmatter excluded.
- **Requirement**: REQ-WF-008 · task `wf-009`.
- **Budget source**: **Skill Body Budget** section of `internal/assets/skills/_shared/agent-writing-contract.md`. This report references that section's target band, hard max, and estimation heuristic; it does not restate their values.
- **Companion artifact**: `internal/assets/skills/_shared/skill-style-guide.md`.
- **Cortex counterpart**: observation `audits/skill-token-budget` (#646).

## Method

1. Strip the YAML frontmatter block.
2. Count whitespace-separated words in the remaining body with `wc -w`.
3. Estimate tokens with the advisory estimation heuristic from the **Skill Body Budget** section (word count ÷ 4). No runtime validator counts tokens; this is an authoring and review aid.
4. Classify each estimate against the target band and hard max defined in the contract.

Reproduction:

```bash
for f in $(find internal/assets/skills -name SKILL.md | sort); do
  body=$(awk 'BEGIN{n=0} /^---$/{n++; next} n>=2{print}' "$f")
  echo "$(basename $(dirname "$f"))|body_words=$(printf '%s' "$body" | wc -w | tr -d ' ')"
done
```

### Verdict legend

| Verdict | Meaning | Action |
|---|---|---|
| `BELOW_TARGET` | Estimate sits below the target band | Informational only; the hard constraint is an upper bound, so no trim is required |
| `IN_BAND` | Estimate sits inside the target band | None |
| `ABOVE_TARGET` | Estimate sits above the target band but below the hard max | Advisory; trim when the procedure does not genuinely need the space |
| `OVER_HARD_MAX` | Estimate meets or exceeds the hard max | Blocking at material-change review; trim or split before acceptance |

## Findings

| # | Skill | Body words | Est. tokens | Verdict | Trim follow-up |
|---|---|---|---|---|---|
| 1 | `ast-impact-analysis` | 402 | 100 | `BELOW_TARGET` | None |
| 2 | `code-review-adversary` | 1239 | 309 | `IN_BAND` | None |
| 3 | `context-distiller` | 296 | 74 | `BELOW_TARGET` | None |
| 4 | `cortex-protocol` | 915 | 228 | `IN_BAND` | None |
| 5 | `discovery` | 856 | 214 | `IN_BAND` | None |
| 6 | `document-reader` | 158 | 39 | `BELOW_TARGET` | None |
| 7 | `fast-tdd` | 528 | 132 | `BELOW_TARGET` | None |
| 8 | `grill-me` | 378 | 94 | `BELOW_TARGET` | None |
| 9 | `hotfix-triage` | 470 | 117 | `BELOW_TARGET` | None |
| 10 | `implement` | 1469 | 367 | `IN_BAND` | None |
| 11 | `investigate` | 871 | 217 | `IN_BAND` | None |
| 12 | `mutation-testing` | 547 | 136 | `BELOW_TARGET` | None |
| 13 | `opencode2-knowledge` | 1207 | 301 | `IN_BAND` | None |
| 14 | `parallel-dispatch` | 564 | 141 | `BELOW_TARGET` | None |
| 15 | `planner` | 1996 | 499 | `ABOVE_TARGET` | Trim (see Follow-ups) |
| 16 | `property-based-testing` | 220 | 55 | `BELOW_TARGET` | None |
| 17 | `spike-prototype` | 306 | 76 | `BELOW_TARGET` | None |
| 18 | `system-diagrams` | 221 | 55 | `BELOW_TARGET` | None |
| 19 | `workflow-retrospective` | 261 | 65 | `BELOW_TARGET` | None |

### Distribution

| Verdict | Count |
|---|---|
| `BELOW_TARGET` | 12 |
| `IN_BAND` | 6 |
| `ABOVE_TARGET` | 1 |
| `OVER_HARD_MAX` | 0 |

## Verdict

**No `SKILL.md` body breaches the hard max.** The inventory is not bloated: 6 bodies sit inside the target band and 12 sit below it. A single body, `planner`, sits above the target band while remaining well under the hard max; it is the only trim candidate. Below-band bodies are not defects — the budget's hard constraint is an upper bound — but the most lean bodies (`document-reader`, `system-diagrams`, `property-based-testing`) are worth confirming as complete rather than merely short.

## Follow-ups

1. **`planner` — trim to band (advisory, not this change).** Candidate reductions, in order:
   - Deduplicate the SDD depth-selection and execution prose that restates routing mechanics owned by `~/.cortex-ia/opencode/contracts/workflow-map.md`; replace with a pointer plus the skill-specific planning rules.
   - Replace the inline Output Schema JSON block with a pointer to the canonical planner receipt schema instead of reproducing it.
   - Compress the delta-spec markdown template to its structural skeleton.
2. **All `BELOW_TARGET` and `IN_BAND` skills — no trim.** No action; re-audit only if a body is materially changed.
3. **No mass rewrite in this change.** This report is a baseline; any trim lands as its own bounded task under the **Skill Body Budget** review gate.

## Cross-references

- Cortex observation `audits/skill-token-budget` (#646) records the same measurement and points back to this file. Neither store is the sole source; reconcile explicitly and name the authoritative copy on disagreement.
- Budget thresholds and the estimation heuristic: `agent-writing-contract.md` → **Skill Body Budget** (single source of truth).
- Structural guidance: `internal/assets/skills/_shared/skill-style-guide.md`.

## Secrets guard

This report contains no tokens, keys, credentials, or private repository paths. It is committed to Git history and treated as public.
