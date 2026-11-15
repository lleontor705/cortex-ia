# Agent evals

Purpose: judge a change to the agent harness (role prompts, skills, shared contracts, orchestration
rules) by what it does on real repository work, not by how the prompt reads. Prompt size is tracked
separately by `node scripts/prompt-budget.mjs`; this protocol measures whether the agents still
finish the work, at what cost, and with how much human rescue.

## Scope and rules

- Compare two harness versions (A and B) on the same model, the same tasks and the same acceptance
  commands. Only the harness changes between arms.
- Every run starts from a clean checkout of the task's base commit, in a fresh temporary
  `HOME` and `CORTEX_IA_HOME`. Never run evals against a developer's real OpenCode, Claude Code or
  Cortex state (see the Security and Persistence invariants in `AGENTS.md`).
- Acceptance is decided by the command, not by the agent's report. A run that claims success and
  fails its command is a fail.
- Tests stay synthetic and temporary; no eval adds a persistent test file to the repository.

## Task set

Tiers: **T1** read-only questions with an answer key; **T2** single bounded changes verified by one
command; **T3** initiatives that need a plan and a same-board task DAG.

Commands are run from the repository root of the task's base commit. Paths are verified against the
tree at the time of writing; if a path moves, update the task, not the acceptance rule.

### Tier 1: questions (6)

| ID | Prompt | Answer key |
|---|---|---|
| T1-01 | Which package owns lease verification? | `internal/delegation` (`lease_verification.go`) |
| T1-02 | Where is the managed MCP catalog defined, and which package validates desired entries? | `internal/mcpmanager/presets.go`; `internal/mcpmanager/desired.go` |
| T1-03 | Which function composes the role prompt that host bundles ship, and which file holds it? | `composeRolePrompt` in `internal/targets/render.go` |
| T1-04 | Which command runs the host pre-tool guard, and where is its decision logic? | `cortex-ia hook pre-tool --host <claude\|agy>`; logic in `internal/hostbridge/hook.go` |
| T1-05 | Where does the delegation database live, and which variable overrides its state root? | `~/.cortex-ia/delegation.db`; `CORTEX_IA_HOME` (isolated override only) |
| T1-06 | Which file is the canonical phase matrix, and which installed path mirrors it? | `internal/assets/skills/_shared/workflow-map.md`; `~/.cortex-ia/opencode/contracts/workflow-map.md` |

Grading: exact match on the key's file or command, or a reviewer's judgement that the answer is
equivalent. Missing the owning file is a fail even if the package is right.

### Tier 2: single tasks (9)

| ID | Prompt (summary) | Acceptance command |
|---|---|---|
| T2-01 | Bug fix: a stale revision is accepted when revising a work definition. Add a reproducing test first, then fix `internal/delegation/work_revise.go`. | `go test ./internal/delegation -run '^TestReviseWorkDefinitionRejectsStaleRevisionWithoutEffects$' -count=1` |
| T2-02 | Bug fix: engine rollback does not restore the pre-apply backup. Fix it in `internal/pipeline`. | `go test ./internal/pipeline -run '^TestRollback_RestoresFromEngineBackup$' -count=1` |
| T2-03 | Refactor under green tests: extract the duplicated key-merge loop in `internal/components/filemerge/json_merge.go` into one helper. No behaviour change. | `go test ./internal/components/filemerge/... -count=1` |
| T2-04 | Bug fix: the checksum verifier accepts a candidate when the authority gate is missing. Fix in `internal/updater/verifier_checksum.go`. | `go test ./internal/updater -run '^TestChecksumVerifierAuthorityGate$' -count=1` |
| T2-05 | Doc fix: remove stale references to the retired ForgeSpec from the reference and getting-started pages. | `! grep -rqi 'forgespec' docs/reference docs/getting-started` |
| T2-06 | Config change: raise the `steps` budget of the orchestrator from 500 to 600 in `internal/assets/agents/orchestrator.md` frontmatter. | `grep -q '^steps: 600$' internal/assets/agents/orchestrator.md && go test ./internal/assets/... -count=1` |
| T2-07 | TUI tweak: change the Home menu entries so the first entry is the stats screen. Update the menu test to match. | `go test ./internal/tui -run '^TestHomeMenuEntries$' -count=1` |
| T2-08 | TUI tweak: make the stats screen default to the all-time window instead of the today summary. | `go test ./internal/tui -run '^TestStatsDefaultWindowIsTodoSummary$' -count=1` |
| T2-09 | Bug fix: the shell decision in the host guard allows a command it should deny. Fix in `internal/hostbridge/hook.go` (or `policy.go`). | `go test ./internal/hostbridge -run '^TestDecideShell$' -count=1` |

Seeding: for T2-01, T2-02, T2-04 and T2-09 the harness applies a one-line fault to the base commit
before the run, so the acceptance command is red at start. The agent must make it green without
editing the acceptance test. For T2-07 and T2-08 the test is updated by the seed to the new
expectation, so the run is red until the TUI change lands.

Pass rule: the acceptance command exits 0 and `git diff --name-only` touches only the files the
task names (plus a test file, where the task asks for one). Touching other files is a fail even if
the command is green.

### Tier 3: initiatives (5)

Each initiative must produce a plan file, a board with a same-board DAG (`cortex-ia board create`,
`cortex-ia work create --board <id>`), and reviewed completion through `work approve`. The
acceptance command is run after the orchestrator reports done.

| ID | Prompt (summary) | Acceptance command |
|---|---|---|
| T3-01 | Add a `--json` output to `cortex-ia board status` that prints the same fields as the table view. | `go test ./internal/app -count=1 && CORTEX_IA_HOME="$(mktemp -d)" go run ./cmd/cortex-ia board status default --json \| python3 -m json.tool >/dev/null` |
| T3-02 | Add one managed MCP preset with ownership and conflict tests, in the catalog and its validator. | `go test ./internal/mcpmanager/... -count=1` |
| T3-03 | Make host bundle uninstall refuse to remove a file whose content changed since cortex-ia wrote it, unless `--overwrite` is given. | `go test ./internal/targets/... -count=1` |
| T3-04 | Add a skill `changelog-check` that validates `CHANGELOG.md` entries, with its SKILL.md under the body budget. | `go test ./internal/assets/... ./internal/pipeline -count=1 && node scripts/prompt-budget.mjs` |
| T3-05 | Show the number of blocked tasks on the board page of the web console (read-only; no new mutation endpoint). Rebuild the static assets. | `npm --prefix web run build && go test ./internal/cortexiaweb/... -count=1` |

Grading for T3 runs also includes the reviewer's verdict and whether the orchestrator dispatched
roles inside the work authority (no claim tokens in prompts, no approval by the implementer).

## What to record per run

One row per run. Fill every field; write `n/a` rather than estimating.

| Field | Source |
|---|---|
| Task ID, arm (A or B), run number (1-3) | Eval sheet |
| Acceptance command: pass or fail, exit code | Re-run by the grader, not the agent |
| Total tokens (input + output, all agents) | Host session usage report |
| Tool calls | Host transcript count |
| Subagents dispatched | Count of role dispatches in the transcript |
| Wall time | From first prompt to acceptance command exit |
| Reviewer verdict (T2 high-risk and all T3) | `PASS`, `FAIL` or `none` (auto-approved) |
| Human interventions | Count of messages the grader sent to unblock the run |
| Notes | One line: where the run stalled or was rescued |

## How to run an A/B

1. Pick two harness versions: A (baseline commit) and B (candidate commit). Record both SHAs.
   Prompt-size regressions should already pass `node scripts/prompt-budget.mjs`.
2. Pin one model and one host for both arms. Record the model ID and the host version.
3. For each task, create the seeded base commit once and reuse it for every run of that task.
4. For each run, create a fresh temporary `HOME` and `CORTEX_IA_HOME`, install the arm's harness
   into that home, and check out the base commit into a new worktree:
   ```sh
   export HOME="$(mktemp -d)" CORTEX_IA_HOME="$HOME/.cortex-ia"
   git worktree add "$TMPDIR/eval-<id>-<arm>-<n>" <base-sha>
   ```
   Delete the worktree and the temporary home after the run.
5. Run tasks in a fixed order and interleave the arms (A, B, A, B) so drift in the host does not
   favour one arm.
6. Run each task 3 times per arm, 20 tasks, 2 arms: 120 runs in total.
7. Compare per task: pass count out of 3, and the median of tokens, tool calls, subagents and wall
   time across the 3 runs. Compare overall: pass rate per tier and the median of per-task medians.
8. Decide:
   - B is not adopted if its pass count drops in any tier by more than one run in total.
   - B is adopted when pass rates hold and median tokens or wall time drop, or when human
     interventions drop with no new failures.
   - Any T2 or T3 run that touches a file outside the task's scope is a fail for that arm.

Three runs per task is a small sample. Treat differences that rest on one run as noise, and rerun
the task before acting on them.

## Results template

Copy this table into the run record for each comparison. Fill the medians from the per-run rows.

| Task | Tier | A pass /3 | B pass /3 | A tokens (med) | B tokens (med) | A wall (med) | B wall (med) | A subagents (med) | B subagents (med) | A interventions | B interventions | Verdict notes |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1-01 | 1 | | | | | | | | | | | |
| T1-02 | 1 | | | | | | | | | | | |
| T1-03 | 1 | | | | | | | | | | | |
| T1-04 | 1 | | | | | | | | | | | |
| T1-05 | 1 | | | | | | | | | | | |
| T1-06 | 1 | | | | | | | | | | | |
| T2-01 | 2 | | | | | | | | | | | |
| T2-02 | 2 | | | | | | | | | | | |
| T2-03 | 2 | | | | | | | | | | | |
| T2-04 | 2 | | | | | | | | | | | |
| T2-05 | 2 | | | | | | | | | | | |
| T2-06 | 2 | | | | | | | | | | | |
| T2-07 | 2 | | | | | | | | | | | |
| T2-08 | 2 | | | | | | | | | | | |
| T2-09 | 2 | | | | | | | | | | | |
| T3-01 | 3 | | | | | | | | | | | |
| T3-02 | 3 | | | | | | | | | | | |
| T3-03 | 3 | | | | | | | | | | | |
| T3-04 | 3 | | | | | | | | | | | |
| T3-05 | 3 | | | | | | | | | | | |

Summary:

| Metric | Tier 1 | Tier 2 | Tier 3 | All |
|---|---|---|---|---|
| Pass rate A (passes / runs) | | | | |
| Pass rate B (passes / runs) | | | | |
| Median tokens A / B | | | | |
| Median wall time A / B | | | | |
| Human interventions A / B | | | | |
| Decision | | | | |

Record the arm SHAs, model ID, host version and date at the top of the run record, next to this
table.
