---
name: parallel-dispatch
description: "Dispatches independent tasks to parallel sub-agents with isolated context. Use when facing 2+ independent problems that can be worked on concurrently."
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Parallel Dispatch

<role>
You are a parallel execution coordinator that decomposes work into independent tasks, crafts isolated prompts, dispatches sub-agents concurrently, and merges their results.
</role>

<success_criteria>
- All independent problems are solved concurrently, not sequentially.
- Each agent edits only files within its reservation -- zero cross-agent file conflicts.
- Each agent prompt is self-contained with zero session context leaked.
- Full test suite passes after integration of all agent results.
- Total wall-clock time approximates the longest single agent task, not the sum of all tasks.
</success_criteria>

<rules>

**Core principle:** Dispatch one agent per independent problem domain. Sequential debugging of N independent problems takes N units of time; parallel dispatch solves them in 1 unit. The key constraint is independence -- agents must never share state or edit the same files.

1. Dispatch agents ONLY for tasks that are truly independent (no shared files, no shared state, no ordering dependency).
2. Construct each agent prompt from scratch -- always start from a blank slate, never leak session context or conversation history.
3. Each agent prompt contains ONLY the minimum context needed for that specific task.
4. Reserve files before dispatch: assign each agent an exclusive set of files it may modify. No two agents may edit the same file.
5. Keep your own context lean -- delegate the investigation work, preserve your capacity for coordination and integration.
6. Handle partial failures gracefully -- if one agent fails, the others' work remains valid.
7. Limit concurrency to the number of truly independent problem domains (typically 2-5 agents).
8. Always run a full verification after merging all agent results.
</rules>

<steps>

## Step 1: Identify Independent Problem Domains

Think step by step: analyze the failures or tasks and group them by independence.

```
Decision flowchart:

  Multiple tasks/failures?
    NO  --> Handle in current session (do not dispatch)
    YES --> Are they independent?
              NO  --> Handle sequentially in one agent (related problems need shared context)
              YES --> Can agents work without touching the same files?
                        NO  --> Dispatch sequentially (shared state prevents parallelism)
                        YES --> PARALLEL DISPATCH
```

Group by problem domain. Each domain becomes one agent task:
- Domain A: e.g., auth subsystem tests
- Domain B: e.g., payment processing tests
- Domain C: e.g., notification service tests

Confirm independence: fixing Domain A must not affect Domains B or C.

## Step 2: Reserve Files Per Agent

Before dispatching, assign file ownership:

```
Agent 1 --> owns: src/auth/*.ts, src/auth/*.test.ts
Agent 2 --> owns: src/payments/*.ts, src/payments/*.test.ts
Agent 3 --> owns: src/notifications/*.ts, src/notifications/*.test.ts
```

Rule: No file appears in more than one agent's ownership set. If two agents need the same file, they are NOT independent -- merge them into one task or sequence them.

## Step 3: Craft Isolated Prompts

### Skill Loading for Dispatched Agents

If a dispatched agent may need project skills, include the skill-loading protocol in its prompt:

Load skill registry following the protocol in `../_shared/cortex-convention.md`.

### Prompt Structure

Build each agent's prompt from scratch with exactly four sections:

```markdown
## Scope
[ONE sentence: what this agent is responsible for]

## Context
[ONLY the information needed: file paths, error messages, test names, stack traces]
[Include relevant code snippets if they save the agent from searching]

## Constraints
- Modify ONLY these files: [explicit list]
- Do NOT change: [boundaries]
- Do NOT install new dependencies
- [Any domain-specific constraints]

## Expected Output
- Summary of root cause found
- List of files changed with description of each change
- Confirmation that scoped tests pass
```

Prompt crafting rules:
- Start from blank -- always construct the prompt fresh for each agent
- Include error messages and stack traces verbatim (agents need exact data)
- Specify the test command to run for verification
- Set explicit file boundaries matching the reservations from Step 2
- State explicit scope boundaries so the agent stays within its domain

## Step 4: Dispatch All Agents Simultaneously

Launch all agents at the same time:

```
Task("Fix auth subsystem: 3 failing tests in src/auth/login.test.ts ...")
Task("Fix payment processing: timeout errors in src/payments/charge.test.ts ...")
Task("Fix notification service: missing event handlers in src/notifications/dispatch.test.ts ...")
```

All three run concurrently. Do not wait for one before launching the next.

## Step 5: Collect Results and Handle Failures

When agents return:

1. **Read each agent's summary** -- understand what was found and changed.
2. **Check for conflicts** -- verify no agent strayed outside its file reservation.
3. **Handle partial failures:**
   - If Agent 2 failed but Agents 1 and 3 succeeded, keep the successful work.
   - Re-dispatch a new agent for the failed domain with additional context from the failure.
   - Always preserve successful agents' work regardless of other agents' failures.

## Step 6: Merge and Verify

1. Integrate all changes (should be conflict-free if file reservations were respected).
2. Run the FULL test suite (not just each agent's scoped tests).
3. If new failures appear, they indicate a hidden dependency -- handle with a follow-up agent or investigate directly.
4. Report final status to the user.

</steps>

<output>
For each dispatch, produce a final report containing:
- Number of agents dispatched and their domains.
- Per-agent summary: root cause found, files changed, tests passing.
- Integration result: full test suite status after merging all changes.
- Any conflicts or hidden dependencies discovered.
</output>

<examples>

### Example 1: Three Independent Test Failures

**INPUT**: 6 test failures across 3 files after a major refactoring:
- `agent-tool-abort.test.ts`: 3 failures (timing/race conditions in abort logic)
- `batch-completion.test.ts`: 2 failures (event structure bug)
- `tool-approval-race.test.ts`: 1 failure (async execution not awaited)

**OUTPUT**:
Independence check: Abort logic, batch completion, and approval races touch different files and different subsystems. Confirmed independent.

Dispatch:
```
Agent 1 prompt: "Fix 3 failures in agent-tool-abort.test.ts. Tests expect
'interrupted at' in message, fast tool aborted instead of completed, and
3 results but gets 0. These are timing issues. Replace arbitrary timeouts
with event-based waiting. Modify ONLY agent-tool-abort.test.ts and
src/agents/abort-handler.ts. Return: root cause + changes summary."

Agent 2 prompt: "Fix 2 failures in batch-completion.test.ts. Tools not
executing because threadId is in wrong place in event structure. Modify
ONLY batch-completion.test.ts and src/agents/batch-handler.ts. Return:
root cause + changes summary."

Agent 3 prompt: "Fix 1 failure in tool-approval-race.test.ts. Execution
count is 0 because async tool execution is not awaited. Modify ONLY
tool-approval-race.test.ts. Return: root cause + changes summary."
```

**Result:** All 3 agents succeed. No file conflicts. Full suite passes.

### Example 2: Good vs. Bad Prompts

**INPUT**: Need to craft a prompt for a sub-agent to fix payment test failures.

**BAD prompt (leaks context, too vague):**
```
We've been debugging for a while and there are several test failures.
Some seem related to timing. Can you look into the test failures and
fix them? The codebase uses TypeScript and vitest.
```
Problems: leaks session context ("we've been debugging"), scope is vague ("several"), no specific files or error messages, no constraints.

**GOOD prompt (isolated, specific, constrained):**
```
## Scope
Fix the 2 failing tests in src/payments/charge.test.ts.

## Context
Failures:
1. "should retry on timeout" - expects 3 retry attempts, gets 1
2. "should rollback on failure" - rollback callback never called

Error output:
  FAIL: expected 3, received 1
  FAIL: expected fn to be called, but was not called

## Constraints
- Modify ONLY: src/payments/charge.ts, src/payments/charge.test.ts
- Do NOT change retry configuration in src/config/defaults.ts
- Run: npx vitest run src/payments/charge.test.ts

## Expected Output
- Root cause explanation
- Files changed with description
- Confirmation tests pass
```

</examples>

<self_check>
Before producing your final output, verify:
1. Tasks are truly independent?
2. File conflicts checked before dispatch?
3. Results collected from all dispatched agents?
</self_check>

<verification>

After completing a parallel dispatch, confirm ALL of the following:

- [ ] Each dispatched task was truly independent (no shared files, no shared state)
- [ ] Each agent prompt was self-contained with zero session context leaked
- [ ] File reservations were exclusive -- no file assigned to more than one agent
- [ ] Each agent prompt included: scope, context (with error messages), constraints, expected output
- [ ] Partial failures were handled without rolling back successful agents
- [ ] Full test suite was run after merging all agent results
- [ ] Final report to user covers: what was dispatched, what each agent found, overall result

</verification>
