## Cortex Persistent Memory — Protocol

You have access to Cortex, a persistent memory system with knowledge graph capabilities that survives across sessions and compactions.
This protocol is MANDATORY and ALWAYS ACTIVE — not something you activate on demand.

### PROACTIVE SAVE TRIGGERS (mandatory — do NOT wait for user to ask)

Call `mem_save` IMMEDIATELY and WITHOUT BEING ASKED after any of these:
- Architecture or design decision made
- Team convention documented or established
- Workflow change agreed upon
- Tool or library choice made with tradeoffs
- Bug fix completed (include root cause)
- Feature implemented with non-obvious approach
- Configuration change or environment setup done
- Non-obvious discovery about the codebase
- Gotcha, edge case, or unexpected behavior found
- Pattern established (naming, structure, convention)
- User preference or constraint learned

Self-check after EVERY task: "Did I make a decision, fix a bug, learn something non-obvious, or establish a convention? If yes, call mem_save NOW."

Format for `mem_save`:
- **title**: Verb + what — short, searchable (e.g. "Fixed N+1 query in UserList")
- **type**: bugfix | decision | architecture | discovery | pattern | config | preference
- **scope**: `project` (default) | `personal`
- **topic_key** (recommended for evolving topics): stable key like `architecture/auth-model`
- **content**:
  - **What**: One sentence — what was done
  - **Why**: What motivated it (user request, bug, performance, etc.)
  - **Where**: Files or paths affected
  - **Learned**: Gotchas, edge cases, things that surprised you (omit if none)

Topic update rules:
- Different topics MUST NOT overwrite each other
- Same topic evolving → use same `topic_key` (upsert)
- Unsure about key → call `mem_suggest_topic_key` first
- Know exact ID to fix → use `mem_update`

### KNOWLEDGE GRAPH (Cortex-exclusive)

After saving related observations, connect them for traceability (Why: knowledge graph connections enable mem_graph traversal — without them, observations are isolated islands that can't be navigated):

```
mem_relate(from: {new_obs_id}, to: {upstream_obs_id}, relation: "references")
```

Supported relations: `references`, `relates_to`, `follows`, `supersedes`, `contradicts`

To explore connections from any observation:
```
mem_graph(id: {obs_id}, depth: 2)
```

Use `mem_graph` for: recovering context after compaction, understanding artifact lineage, finding related work.

### IMPORTANCE SCORING

Check observation relevance with `mem_score(id: {obs_id})`. High-score observations are accessed frequently and recently — prioritize them when context is limited.

### WHEN TO SEARCH MEMORY

On any variation of "remember", "recall", "what did we do", or references to past work:
1. Call `mem_context` — checks recent session history (fast, cheap)
2. If not found, call `mem_search` with relevant keywords
3. If found, use `mem_get_observation` for full untruncated content (Why: mem_search returns 300-char previews only — working with truncated data leads to wrong conclusions and missed context)

Also search PROACTIVELY when:
- Starting work on something that might have been done before
- User mentions a topic you have no context on
- User's FIRST message references the project, a feature, or a problem — call `mem_search` with keywords from their message

### SESSION CLOSE PROTOCOL (mandatory)

Before ending a session, call `mem_session_summary`:

## Goal
[What we were working on this session]

## Instructions
[User preferences or constraints discovered — skip if none]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done — for the next session]

## Relevant Files
- path/to/file — [what it does or what changed]

This is NOT optional (Why: if you skip this, the next session starts blind — all decisions, discoveries, and progress are lost. The 30 seconds to save prevents hours of re-work).

### AFTER COMPACTION

If you see a compaction message or context loss:
1. IMMEDIATELY call `mem_session_summary` with what you remember — this persists pre-compaction work
2. Call `mem_context` to recover additional context from previous sessions
3. Use `mem_graph` to explore connections from recently accessed observations
4. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction is lost from memory.
