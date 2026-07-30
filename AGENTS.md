# cortex-ia — Agent Skills Index

When working on this project, load the relevant skill(s) BEFORE writing any code.

## How to Use

1. Check the trigger column to find skills that match your current task
2. Load the skill by reading the SKILL.md file at the listed path
3. Follow ALL patterns and rules from the loaded skill
4. Multiple skills can apply simultaneously

## Community Skills (`skills/`)

These are **repo-specific** versions that complement the generic builtin skills above. They contain cortex-ia specific templates, CI checks, and labels. Use these when working ON cortex-ia itself; use the builtin versions for other projects.

| Skill | Complements | Path |
|-------|-------------|------|
| `cortex-ia-issue-creation` | `file-issue` (builtin) | [`skills/issue-creation/SKILL.md`](skills/issue-creation/SKILL.md) |
| `cortex-ia-branch-pr` | `open-pr` (builtin) | [`skills/branch-pr/SKILL.md`](skills/branch-pr/SKILL.md) |

## Built-in SDD Skills (`internal/assets/skills/`)

These skills ship embedded in the binary and are deployed by the SDD component into each agent's skills directory. The full set is documented in [`docs/sdd-workflow.md`](docs/sdd-workflow.md).

Highlights:

| Skill | Phase | When to load |
|-------|-------|--------------|
| `bootstrap` | SDD-0 | Starting a new SDD session |
| `investigate` / `sdd-explore` | SDD-1 | Mapping unknown areas of the codebase |
| `draft-proposal` / `sdd-propose` | SDD-2 | Drafting a change proposal |
| `write-specs` / `sdd-spec` | SDD-3 | Producing Given/When/Then scenarios |
| `architect` / `sdd-design` | SDD-4 | Designing the implementation approach |
| `decompose` / `sdd-tasks` | SDD-5 | Breaking the design into tasks |
| `implement` / `sdd-apply` | SDD-6 | Applying spec → code |
| `validate` / `sdd-verify` | SDD-7 | Verifying scenarios pass |
| `finalize` / `sdd-archive` | SDD-8 | Archiving the change set |
| `judgment-day` | cross-phase | Adversarial dual-review of a change before merge |
| `debate` | cross-phase | Multi-position deliberation |
| `onboard` | cross-phase | Guided end-to-end SDD walkthrough |
| `chained-pr` | cross-phase | Splitting oversized PRs into chained/stacked PRs |
| `work-unit-commits` | cross-phase | Planning reviewable commits during implementation |
| `debug`, `monitor`, `ideate`, `execute-plan`, `open-pr`, `file-issue`, `parallel-dispatch`, `scan-registry`, `go-testing`, `cognitive-doc-design`, `comment-writer`, `skill-creator`, `skill-improver` | utility | See each SKILL.md for the trigger |
