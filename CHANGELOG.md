# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/); tags are `vMAJOR.MINOR.PATCH`.

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
