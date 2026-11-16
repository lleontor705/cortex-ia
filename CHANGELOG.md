# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/); tags are `vMAJOR.MINOR.PATCH`.

## Unreleased

### Removed

- Spec-Driven Development and OpenSpec: the `cortex-ia openspec` command (`new`, `validate`,
  `status`, `archive`), `cortex-ia work archive`, the `--contract-file` and `--spec-plane` flags,
  the `cortex_ia_openspec_validate`, `cortex_ia_openspec_write` and `cortex_ia_change_archive` tools,
  the spec planes (`openspec`, `cortex`, `hybrid`) and the `sdd-lite` and `sdd-full` workflows.
  Retired commands and flags fail with a "retired" error.
- Tools `cortex_ia_work_lease_renew` (merged into `cortex_ia_work_renew`) and the unusable
  `cortex_ia_work_watch`; the `path`, `ttl`, `status` and `role` aliases on work tools.

### Changed

- Lightweight planning: a plan is written only for feature work of three or more files, unclear
  requirements, multi-session work, or more than one task. The plan is one file,
  `docs/plans/<board-id>.md`, with executable acceptance checks. There is no archive step; the
  plan stays as history and the tests are the living spec.
- Work workflows are `direct-change`, `fast-tdd`, `hotfix` and `ops-task`.
- Migration: a schema migration converts existing SDD tasks to `direct-change` tasks, clears their
  contracts and drops the `change_archives` table.
- Agent instructions rewritten: the installed `AGENTS.md` drops from about 6,900 to 750 tokens,
  role prompts shrink by 40-80%, every rule lives in one contract, and capitalised MUST/NEVER
  imperatives are replaced by plain rules with their reasons.
- Organic orchestration: the orchestrator picks the route, scales effort to the request, does
  targeted reads itself, and runs parallel implementers only for independent tasks (at most four).
- One dispatch brief (with `context` and `evidence_refs`) and one completion receipt; per-check
  verdicts use `check_id`.
- The reviewer works from goals and risk lenses instead of a five-phase script.
- Claude Code and agy now give every role the global guide, not only the orchestrator.
- `cortex_ia_work_status` returns a compact view (`detail: true` for the full record); tool
  descriptions state preconditions and error codes; `cortex_ia_workload_audit` is available to
  implement and reviewer.
- `scripts/prompt-budget.mjs` ratchets prompt size and imperatives in CI;
  `docs/evals/agent-evals.md` defines an A/B evaluation protocol.

### Fixed

- `work approve` requires evidence for a `PASS` verdict.
- The retry-after-FAIL gotcha rule is documented.
- The reviewer role is granted the `cortex_search` permission.
- The planner can write its plan file under all hosts' write guards.
- `cortex_ia_snapshot_read` and `cortex_ia_workload_audit` return valid OpenCode tool results.

### Flow coherence

- The work state machine is one table in the protocol (actor, required state, tool), and the
  orchestrator and implement prompts follow it: retry after a FAIL calls `work_retry` before
  re-dispatch, a failed or recovered claim is reported instead of re-claimed, and two FAILs
  route the task to decompose.
- `cortex_ia_work_create` takes `workload_policy`, so the strict LOC budget can apply.
- Plans: the planner is the only board creator and gains an `update` phase for check changes
  and close; implementers never lease the plan file; the Progress section is gone; acceptance
  checks travel as `AC-<n>: <command>` and reviewers verify only the checks a task names.
- The orchestrator can run `workload_audit` and save gotchas, so auto-approval and failure
  memory rest on its own evidence.
- Claude Code: the planner can write its plan and the orchestrator can ask the user; agy:
  the planner can write; both host preambles state what the guard actually enforces.
- Advisory step budgets match the frontmatter caps more closely, and budget warnings use
  fixed text so they no longer break prompt caching.
- The mutation-testing skill reverts with an inverse edit instead of `git checkout`, which
  discarded the implementer's uncommitted fix.

### Security

- Agent shells can no longer approve their own work through `cortex-ia work`: the host guard
  normalizes case, quotes and global flags, and Claude Code shells now carry
  `CORTEX_IA_AGENT_SHELL=1` so the CLI refuses authority verbs however they are invoked.
- agy agents, which the host guard cannot identify, may create and implement tasks but not
  approve, recover, retry, reconcile or decompose them.
- The symlink-parent guard for managed writes now checks real ancestors; it previously
  checked paths relative to the current directory and passed every write.
- `cortex_ia_workload_audit` rejects option-like `baseRef` values (a `--output=` value could
  overwrite files) and fails closed when `git diff` fails.
- The orchestrator's env-file guard is case-insensitive and covers `.envrc` and `*.env`.

## v1.0.0 (2026-11-15) — initial release

First release on the new repository history. The git history was reset to a single snapshot and the
earlier `v0.x` tags and releases were removed; this entry describes the product as it ships today.

### Highlights

- **Three hosts** — `cortex-ia install --target opencode,claude,agy` (or `all`). OpenCode is the
  primary target with the full native asset set; Claude Code and Antigravity CLI (agy) receive host
  bundles that are not yet exercised in a real host session
- **Durable work authority** — boards, the task dependency graph, claims, per-file leases with TTL,
  revision-checked transitions, approvals and an append-only event log in one local SQLite database
  (`~/.cortex-ia/delegation.db`)
- **Six native roles** — `orchestrator`, `discovery`, `investigate`, `planner`, `implement` and
  `reviewer`, with the same embedded role prompts, skills and contracts on every host
- **Adaptive review** — only `work approve --verdict PASS` with evidence produces `done`;
  independent review is mandatory for high-risk code, and the review tier can be raised but never
  lowered
- **Specification-driven development** — `cortex-ia openspec new|validate|status|archive` with
  three routing tiers, from direct answers to coordinated SDD on an initiative board
- **Transactional install** — plan, cross-process lock, verified backup, atomic apply and rollback
  on failure for `install`, `sync`, `uninstall` and every `mcp` mutation
- **Host integration** — `cortex-ia mcp serve --host <claude|agy>` exposes the `cortex_ia_*` tools
  over stdio, and `cortex-ia hook pre-tool --host <claude|agy>` denies writes without a live lease
- **Operations console** — `cortex-ia web`, a loopback-only console for boards, tasks, claims,
  leases and the activity stream
- **Self-update** — `cortex-ia update` verifies release archives against `checksums.txt`

### Documentation

- README rewritten around the problems the tool solves, with a contents list and a command overview
- New explanatory diagrams: architecture overview, task lifecycle, install pipeline, install targets
  and routing tiers; the hero banner no longer shows retired features
- The full command tables moved from the README to [`docs/reference/cli.md`](docs/reference/cli.md)
- [`docs/index.md`](docs/index.md) is now a grouped map of every documentation page
- Images are organized under `docs/assets/brand/` and `docs/assets/diagrams/`, with authoring rules
  in [`docs/assets/README.md`](docs/assets/README.md)

### Upgrading from v0.7.0

- The code is the v0.7.0 tree plus dependency updates; no command or flag changed
- The delegation database stays at schema v18. Binaries older than v0.7.0 refuse a migrated database
- `v0.x` releases and tags no longer exist on GitHub. Pin `v1.0.0` or later in scripts and in
  `go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@v1.0.0`
