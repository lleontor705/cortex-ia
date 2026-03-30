---
agent: orchestrator
subtask: false
---

Launch an adversarial debate on the given topic.

## Protocol

Follow the DEBATE MODE section in the orchestrator prompt (`prompts/orchestrator.md`) exactly. The steps are:

1. **Identify positions.** If the user supplied explicit positions, use them. Otherwise, delegate to @investigate first to surface 2-3 natural competing approaches from the codebase or problem domain.

2. **Create task board.** Call `tb_create_board` with one entry per position (2-4 agents, matching the number of competing approaches).

3. **Spawn debate agents.** Launch 2-4 @investigate agents in parallel, one per position. Each agent's prompt MUST include:
   - `ADVERSARIAL ROLE: You defend position '{X}'. Challenge opposing positions via msg_send. Read rebuttals via msg_read_inbox. You have 3 rounds of 1 message each.`
   - `ENABLED CLIs: {list from CLI Selection Protocol}`
   - The topic context and any relevant prior artifacts

4. **Monitor rounds.** Allow 3 rounds. If an agent does not respond within its turn, skip it. After 2 skips from the same agent, remove it from the debate.

5. **Collect results.** After round 3 (or all agents have posted), read all threads via `msg_activity_feed(limit: 50)`.

6. **Tie-breaker.** If no clear winner emerges, the orchestrator decides based on:
   - (a) Which position had fewer unaddressed counterarguments
   - (b) Which aligns better with existing codebase patterns
   - (c) Which has lower risk

7. **Persist results to Cortex.** Call `mem_save(title: "sdd/{change-name}/debate", topic_key: "sdd/{change-name}/debate", type: "architecture", project: "{project}", content: "{debate summary}")` so the decision is recoverable in future sessions.

8. **Present to user.** Show the debate summary including:
   - Each position and its strongest arguments
   - Key counterarguments raised and whether they were addressed
   - The winning position and rationale
   - Confidence level in the recommendation
   - Ask for user approval before proceeding with the winning approach
