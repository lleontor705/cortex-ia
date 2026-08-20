# Architecture

cortex-ia is an OpenCode-only installer: it copies the embedded workflow
asset set into OpenCode's config root, manages MCP entries inside the
OpenCode config, and does both transactionally. It is not a workflow
scheduler and not a resident process — after a run, only files exist.

```
cmd/cortex-ia            main(): release version + internal/app
internal/app             CLI dispatch, flags, receipts, retired-surface guard
internal/tui             Bubble Tea TUI (Install/Sync, MCPs, Doctor/Recovery, Uninstall)
internal/install         Service facade: install, sync, doctor, rollback, uninstall, MCP ops
internal/pipeline        Plan/apply transaction: InstallV2, effects, journal, Rollback
internal/backup          Snapshots, manifests, verification, dedup, retention pruning
internal/mcpmanager      Managed MCP catalog, desired-entry validation, qualification, conflicts
internal/state           Installation metadata, lock, v2 agreement under ~/.cortex-ia/
internal/installmeta     MCP identity digests and install accreditation
internal/agents/opencode OpenCode native layout + pure asset mapping
internal/components/filemerge  JSONC decode/merge, atomic writes, symlink-parent rejection
internal/assets          go:embed runtime assets (config, AGENTS.md, agents, commands, skills, plugin)
```

## Layering Rules

- `internal/app` owns no install, merge, or ownership logic. It parses
  intent, delegates to `internal/install.Service`, and renders receipts.
- All ownership decisions live in the service and its collaborators
  (`pipeline`, `backup`, `mcpmanager`, `state`, `installmeta`).
- The dispatcher fails closed on retired surfaces (multi-agent, persona,
  profile, model, skill-registry, SDD compiler) via a preflight scan that
  stops at the `--` verbatim separator.

## Asset Mapping

`internal/agents/opencode/layout.go` is the single declaration of
OpenCode's native discovery surface:

| Kind | Destination under `~/.config/opencode/` |
|------|------------------------------------------|
| config | `opencode.json` / `opencode.jsonc` (three-way merge) |
| agents-doc | `AGENTS.md` |
| agent | `agents/<name>.md` |
| command | `commands/<name>.md` |
| skill | `skills/<name>/SKILL.md` |
| plugin | `plugin/...` |

`assetmap.go` maps embedded sources to destinations with
`dest = path.Join(config root, source)` and validates each one against the
same layout declaration, so selection and mapping can never disagree. The
mapping fails closed on:

- unsafe paths (absolute paths, traversal outside the root),
- kinds without a native destination (`internal/assets/_shared` contracts
  are compile-time only and never installed),
- destinations off the native surface (non-`SKILL.md` skill fragments,
  nested agents/commands),
- destination collisions, including case-insensitive collisions on Windows
  and macOS.

## Transactional Pipeline

`InstallV2` runs in three phases:

1. **Plan** — build the complete effect set (create/update/managed-update
   per artifact) plus conflict detection. Dry-run and apply share this
   plan; a dry run writes nothing at all.
2. **Backup** — snapshot every path the apply phase may touch and verify that
   manifest before any write begins. A failed verification aborts before any
   mutation.
3. **Apply** — write files atomically (temp file + rename), merge the
   config via `filemerge` (comments and user keys survive), journal the
   transaction, and commit v2 metadata + lock last. Any apply failure
   restores the pre-run state from that verified backup and reports the restore
   result in the receipt.

`internal/backup` also contains snapshot helpers for checksum-based dedup and
retention pruning. These helpers are present but **not wired into the production
Install/Sync/Rollback flow**, so no automatic dedup/prune behavior is promised
today.

`sync` uses the same path and additionally plans removals for stale owned
artifacts from older versions.

## MCP Manager

`internal/mcpmanager` owns the catalog (`cortex`, `forgespec`,
`context7`), the typed `Desired` contract (preset / local argv vector /
remote http URL, with `--env` and `--header` assignments bound to exactly
one kind each), qualification of present entries against recorded identity
digests, and typed conflict errors that never contain secrets. The CLI's
sanitized `mcp list --json` projection is allow-listed to identity
evidence: names, statuses, digests, entry types, and env/header variable
names.

## Embedded Assets

`internal/assets/` is the runtime source of truth: `opencode.jsonc`,
`AGENTS.md`, 5 sub-agent definitions, 9 commands, 12 skills, and the
5 plugin runtimes (33 assets total). Assets are embedded with `go:embed`; changing
them requires rebuilding the binary. Nothing is generated at install time —
the installed bytes are the repository bytes.

## Testing

Persistent tests are deliberately scoped (anti-overengineering rule):

1. TUI behavior in `internal/tui/`.
2. Simple existence/copy validation toward OpenCode in
   `internal/pipeline/install_test.go` (temp homes only).

Deeper transactional oracles run as ephemeral smokes and are deleted after
execution. Full gates, in hook order: `gofmt -s -w .`, `go vet ./...`,
`golangci-lint run ./...`, `go test -count=1 ./...`.
