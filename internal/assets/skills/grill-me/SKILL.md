---
name: grill-me
description: Relentless structured interview to sharpen a plan, architecture decision, or feature design before implementation.
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

# Relentless Design & Decision Grilling

Interview the user relentlessly until you reach a concrete, shared understanding of requirements, architecture, constraints, and trade-offs. Map this exploration as a **design tree**: every foundational decision branches into the secondary decisions that depend on it.

## 1. The Decision Frontier

Work the tree in **rounds**. The **frontier** consists of every decision whose prerequisites are already settled — questions you can ask *now* without guessing at answers you haven't received yet.

Ask the entire frontier in one round:
- Number each question (`Q1`, `Q2`, ...).
- Provide a clear, concrete rationale and the viable options.
- Always include your **recommended answer** with the technical justification.
- Wait for the user's answers before formulating the next round.

## 2. Round Formatting Contract

Format every grilling round strictly as follows:

```
❓ **Q1** - **<Decision / Question Title>**: <Detailed body, context, trade-offs, and options (A/B/C)>

➡️ **Recomendación**: <Your recommended option and technical justification>

---

❓ **Q2** - **<Decision / Question Title>**: <Detailed body, context, trade-offs, and options (A/B/C)>

➡️ **Recomendación**: <Your recommended option and technical justification>
```

## 3. Autonomous Fact-Finding via Subagents (Zero Busywork for User)

Finding **facts** is strictly the system's job, never the user's:
- The `orchestrator` has no code inspection or shell tools. When a frontier question depends on a codebase fact (existing APIs, schemas, configurations, dependency versions, test runners, project structure), the orchestrator **MUST dispatch the `investigate` subagent** to inspect the repository and return factual evidence.
- **Never ask the user for information that `investigate` can discover in the codebase.**
- **Decisions** belong to the user; **facts** belong to `investigate` diagnostics.

## 4. Tree Progression & Stop Condition

- Each round of user answers settles active branches and pushes the frontier outward, unblocking downstream architectural choices.
- Recompute the frontier and ask the next round if unsettled choices remain.
- A question whose answer depends on an open question in the current round belongs to a *future round*, not the current one.
- **Completion Gate**: The grilling session concludes when the **frontier is empty** — every branch of the design tree has been visited, edge cases evaluated, and nothing is left silently assumed.
- Summarize the final agreed design or sync it into the active OpenSpec/Cortex proposal before delegating implementation.
