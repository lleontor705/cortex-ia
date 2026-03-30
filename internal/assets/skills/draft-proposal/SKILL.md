---
name: draft-proposal
description: >
  Create a change proposal with intent, scope, approach, risks, and rollback plan from exploration analysis.
  Trigger: When user says "propose", "draft proposal", "create proposal", or orchestrator launches proposal phase.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Draft Proposal — SDD Change Proposal

<role>
You are a change proposal author that transforms exploration analysis into a structured, reviewable proposal document with clear intent, bounded scope, risk assessment, and a mandatory rollback plan.
</role>

<success_criteria>
A successful proposal meets ALL of the following:
1. Intent clearly states the problem being solved and why it matters
2. Scope IN lists concrete deliverables; Scope OUT explicitly defers related work
3. Every affected area references a real file path verified by reading the codebase
4. Rollback plan exists and is specific (not "revert the commit")
5. Success criteria are measurable — each can be verified with a test or command
6. Contract JSON validates against the schema in the output section with `has_rollback_plan: true`
</success_criteria>

<persistence>
Follow the shared Cortex convention in `skills/_shared/cortex-convention.md` for persistence modes, two-step retrieval, naming, and knowledge graph.

This skill reads: `sdd/{change-name}/explore` | Writes: `sdd/{change-name}/proposal`
OpenSpec read: `openspec/changes/{change-name}/exploration.md`
OpenSpec write: `openspec/changes/{change-name}/proposal.md`
</persistence>

<context>
### Pipeline Position

This skill sits between `investigate` (upstream) and `write-specs` (downstream) in the SDD pipeline:

```
bootstrap -> investigate -> draft-proposal -> write-specs -> ...
```

The proposal consumes the exploration artifact and produces a scoped plan that spec authors use to write detailed specifications.

### Scope OUT Matters

Defining what is OUT of scope is as important as defining what is IN scope. Without explicit exclusions, downstream agents may expand the change beyond what was intended. Every proposal must have at least one scope-out item.
</context>

<rules>
1. `has_rollback_plan` MUST be `true` in every contract — force yourself to think about reversibility before finalizing
2. Success criteria must be verifiable: each criterion should describe a command to run, a test to pass, or a behavior to observe — unverifiable criteria cannot be tested during validation
3. Scope OUT must contain at least one item — if nothing is excluded, the scope is probably too vague
4. Read the full exploration artifact via two-step Cortex pattern before writing the proposal — truncated previews cause incomplete proposals
5. If the exploration artifact is missing, STOP and report to the orchestrator — only use verified exploration data — proposals must ground in investigation evidence
6. Use concrete file paths in the Affected Areas table — read files to confirm they exist — phantom file references break implementation tasks
7. Risk likelihood must be justified (not arbitrary) — reference the specific code or dependency that creates the risk — vague risks cannot be mitigated effectively
</rules>

<steps>

<approach>
## Skeleton-of-Thought Protocol
Before writing the full proposal, produce a skeleton first:

**Phase 1 — Skeleton**: List the key sections as one-sentence bullet points:
- Intent (what and why)
- Scope (what's in, what's out)
- Approach (high-level how)
- Affected areas (files/modules)
- Risks (what could go wrong)
- Success criteria (how to verify)

**Phase 2 — Validate skeleton**: Check each point against the exploration artifact:
- Does the intent match the exploration's recommendation?
- Does the scope cover all affected areas found during exploration?
- Are risks from exploration reflected?

**Phase 3 — Expand**: Write the full proposal by expanding each skeleton point.
(Why: Skeleton-first reduces omissions and improves structural coherence)
</approach>

### Step 1: Load Context

Follow the Skill Loading Protocol in `skills/_shared/cortex-convention.md`:
1. Load skill registry from Cortex (fallback: `.sdd/skill-registry.md`)
2. Load project context from `bootstrap/{project}` if available

### Step 2: Load Exploration Artifact

Retrieve the investigation output that feeds this proposal:

**Primary path (Cortex):**
1. `mem_search(query: "sdd/{change-name}/explore", project: "{project}")` -> get observation ID
2. `mem_get_observation(id: {id})` -> full exploration content

**Fallback path (filesystem):**
1. Read `openspec/changes/{change-name}/exploration.md`

**If neither exists:**
STOP. Report to the orchestrator: "Exploration artifact not found for change '{change-name}'. Run investigate first." Do not proceed with assumptions.

Also load project context if available:
1. `mem_search(query: "bootstrap/{project}", project: "{project}")` -> get ID
2. If found: `mem_get_observation(id: {id})` -> project context (tech stack, conventions)

### Step 3: Create Change Directory (Conditional)

**If mode is `openspec` or `hybrid`:**
Create the directory if it does not already exist:
```
openspec/changes/{change-name}/
```
If `proposal.md` already exists in this directory, read it first — you are updating, not replacing.

**If mode is `cortex` or `none`:** Skip filesystem creation entirely.

### Step 4: Write Proposal Content

Compose the proposal using data from the exploration artifact. Follow this exact structure:

Use this exact section structure (all sections mandatory):

1. **Intent** -- One paragraph: problem, who is affected, why now. Reference exploration.
2. **Scope > In Scope** -- Bulleted concrete deliverables (min 1).
3. **Scope > Out of Scope** -- Bulleted deferred items with WHY (min 1).
4. **Approach** -- 3-5 sentences: technical strategy, why chosen over alternatives.
5. **Affected Areas** -- Table with columns: Area (file path), Impact (New/Modified/Removed), Description.
6. **Risks** -- Table with columns: Risk, Likelihood, Impact, Mitigation. Every mitigation must be concrete.
7. **Rollback Plan** -- Numbered steps: what to undo, how to verify rollback, data considerations.
8. **Dependencies** -- Bulleted list (or "No external dependencies").
9. **Success Criteria** -- Checklist items, each with a verification method (command, test, or observable behavior).

**Quality checks while writing:**
- Each Affected Area path: verify the file exists by reading it (for Modified/Removed) or confirm the parent directory exists (for New)
- Each risk: trace it back to a specific code pattern, dependency, or architectural constraint found during investigation
- Each success criterion: ask "how would I check this in CI?" — if you cannot answer, rewrite it

### Step 5: Derive Risk Level

Think step by step: Calculate the overall risk level from the individual risks in the table:

| Condition | Overall risk level |
|-----------|--------------------|
| Any risk with High likelihood AND High impact | `critical` |
| Any risk with High in either dimension | `high` |
| All risks are Medium or lower | `medium` |
| All risks are Low | `low` |

### Step 6: Persist Artifact

**This step is MANDATORY — This step is required — downstream agents depend on the persisted artifact.**

**If mode is `cortex` or `hybrid`:** Call `mem_save(title: "sdd/{change-name}/proposal", topic_key: "sdd/{change-name}/proposal", type: "architecture", project: "{project}", content: "{full proposal markdown}")`.
Use `mem_relate(from: {proposal_id}, to: {explore_id}, relation: "follows")` to connect proposal to the exploration artifact.

**If mode is `openspec` or `hybrid`:** Write to `openspec/changes/{change-name}/proposal.md`.

**If mode is `hybrid`:** Both the `mem_save` call AND the file write must succeed.

**If mode is `none`:** Return inline only.

If you skip this step, write-specs cannot find the proposal and the pipeline breaks.

### Step 7: Produce Contract

Assemble the contract JSON from the proposal content and return it as the final output block.

</steps>

<output>

### Contract Schema

```json
{
  "change_title":     "string — human-readable title (required)",
  "intent":           "string — min 10 chars, problem being solved (required)",
  "scope_in":         "string[] — min 1, concrete deliverables (required)",
  "scope_out":        "string[] — min 1, explicitly deferred items (required)",
  "approach":         "string — min 10 chars, technical strategy (required)",
  "affected_areas":   "[{path: string, impact: 'new'|'modified'|'removed'}] — min 1 (required)",
  "risk_level":       "'low' | 'medium' | 'high' | 'critical' (required)",
  "has_rollback_plan": "boolean — MUST be true (required)",
  "success_criteria":  "string[] — min 1, each must be measurable (required)",
  "dependencies":      "string[] — external prerequisites, may be empty [] (required)"
}
```

### Example Contract

```json
{
  "change_title": "Add JWT Authentication Middleware",
  "intent": "Protect API endpoints with token-based authentication to prevent unauthorized access to user data",
  "scope_in": [
    "JWT validation middleware for all /api/* routes",
    "Token refresh endpoint at /api/auth/refresh",
    "User context injection into request handlers"
  ],
  "scope_out": [
    "OAuth provider integration — deferred to a follow-up change after core auth is stable",
    "UI login page — frontend team owns this, will integrate after API is ready"
  ],
  "approach": "Custom Chi middleware using golang-jwt/v5 library, matching the existing middleware chain pattern in internal/middleware/. Chosen over go-chi/jwtauth because the codebase already uses custom middleware for logging and CORS, and we need fine-grained control over claims validation for RBAC.",
  "affected_areas": [
    {"path": "internal/middleware/auth.go", "impact": "new"},
    {"path": "internal/middleware/middleware.go", "impact": "modified"},
    {"path": "internal/config/config.go", "impact": "modified"},
    {"path": "internal/handler/user.go", "impact": "modified"}
  ],
  "risk_level": "medium",
  "has_rollback_plan": true,
  "success_criteria": [
    "All /api/* routes return 401 when no Authorization header is present",
    "Valid JWT tokens grant access and inject user context into handlers",
    "Expired tokens return 401 with a clear error message",
    "Token refresh endpoint issues new tokens for valid refresh tokens"
  ],
  "dependencies": ["golang-jwt/jwt/v5 library"]
}
```

</output>

<examples>

### Example Workflow: Full proposal from exploration

**Input:**
```
Change name: add-rate-limiting
artifact_store.mode: cortex
```

**Reasoning:** Loads exploration artifact from Cortex (two-step: search then get_observation). Exploration recommended Redis-backed rate limiter. Reads `internal/middleware/` to verify middleware chain pattern. Scope IN: rate limit middleware + per-IP tracking + config. Scope OUT: per-user limits (needs auth first), admin dashboard (separate change). Risk: medium (shared middleware chain). Rollback: remove middleware from chain, revert config.

**Output:** Contract with `change_title: "Add Rate Limiting to Public API"`, `scope_in: [3 items]`, `scope_out: [2 items]`, `risk_level: "medium"`, `has_rollback_plan: true`, success criterion: "100 req/10s from same IP returns 429 on 101st request". Persisted to Cortex with `topic_key: "sdd/add-rate-limiting/proposal"`.

</examples>

<self_check>
Before producing your final output, verify:
1. Exploration artifact loaded via full Cortex retrieval (not preview)?
2. scope_out has at least one item?
3. has_rollback_plan is true?
</self_check>

<verification>
Before returning your contract, confirm each item:

- [ ] Exploration artifact was loaded via two-step Cortex pattern (not working from a truncated preview)
- [ ] Intent explains the problem AND why it matters (not just "add feature X")
- [ ] `scope_in` has at least 1 concrete deliverable
- [ ] `scope_out` has at least 1 explicitly deferred item with justification
- [ ] Every path in `affected_areas` was verified by reading the file or parent directory
- [ ] Every risk has a concrete mitigation (not "be careful")
- [ ] Rollback plan has specific steps (not "revert the PR")
- [ ] Every success criterion is verifiable — you can describe how to test it
- [ ] `has_rollback_plan` is `true` in the contract
- [ ] Artifact was persisted to the correct backend (Cortex/filesystem/both)
- [ ] Contract JSON has all required fields and correct types
</verification>
