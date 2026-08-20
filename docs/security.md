# Safety & Recovery

cortex-ia writes into your home configuration. Every safeguard below is
enforced by the transactional pipeline, not by convention.

## Plan First, Write Second

`install`, `sync`, and every `mcp` mutation build a complete plan before
touching the filesystem. `--dry-run` executes the same planning path and
creates **nothing** — not even destination directories, journals, backups,
or state files. The real run reuses that plan; a dry run is a faithful
preview, never an approximation.

Confirmation is **digest-bound**: the interactive preview displays the plan
digest (and every `--overwrite` replacement), and the confirmed run re-plans
with identical options and must reproduce that exact digest. If the source,
selection, options, target, or config changed in between, the run aborts
with a typed stale-plan error before any backup or write — zero side
effects.

## One Writer per Home

After confirmation and before the first write, every mutating command
(`install`, `sync`, `mcp add/remove`, `rollback`, `recover`, `uninstall`)
acquires a cross-process lock keyed by the canonical home, using
`LockFileEx` on Windows and `flock` on Unix. A second concurrent writer
receives a typed busy result without mutating. Read-only commands (`plan`,
list, `doctor` without recovery) never steal the writer lock.

## Fail-Closed Conflicts

A *conflict* is an unmanaged file sitting at a destination cortex-ia wants
to write. Conflicting runs abort with nothing written and no metadata
committed. The preview lists every blocker read-only.

`--overwrite` (install and sync only) authorizes replacing those files, but
only when:

1. the flag is given explicitly on the command line, and
2. an interactive terminal confirms with an explicit `y`/`yes`.

Piped, redirected, or closed stdin always fails closed. Before any
replacement happens, a **verified backup** captures the original bytes, so
an authorized overwrite is reversible via `rollback`.

The base config `opencode.jsonc` is never byte-replaced: it is three-way
merged so your keys and comments survive while template keys are added.

## Verified Backups

Every mutating run snapshots the affected files under
`~/.cortex-ia/backups/<id>/` together with a manifest. The snapshot is
verified after capture — a backup that cannot be proven complete fails the
run before the apply phase starts. Snapshots accumulate under
`~/.cortex-ia/backups/`; automatic deduplication and retention pruning
exist as library capabilities in `internal/backup` but are not enabled in
the product yet.

## Rollback on Failure

The apply phase is transactional. If any write fails mid-run, the pipeline
restores the pre-run state from the verified backup before returning the
error, and the receipt reports the restore. A failed install leaves your
configuration as it was.

Restoration itself is fail-closed: every manifest entry is validated for
normalized home/checkpoint containment before any write (absolute paths,
traversal, aliases, duplicates, and checkpoint escapes are rejected up
front), declared targets are journaled, restoration proceeds in reverse
order, and a later write or verification failure reverts the earlier writes
from the journal. Success is impossible unless the complete restoration is
verified.

## Explicit Rollback

```bash
cortex-ia rollback [backup-id]
```

Restores the recorded backup, or the one named on the command line. The
real run is destructive (managed files written after that backup are
removed or reverted) and requires interactive confirmation; piped input
fails closed. The receipt reports the backup ID, verification result, files
restored, and files removed.

## Ownership-Accredited Uninstall

```bash
cortex-ia uninstall [--dry-run]
```

Uninstall removes only what the installation metadata can accredit:

- Every file is checked against the digest recorded at install time.
- Anything unverifiable, drifted, or co-owned with another tool is
  **retained and reported**, never guessed at or deleted.
- Managed MCP entries are removed only through the same ownership proof as
  `mcp remove`; unknown entries are left untouched.
- The run requires interactive confirmation; `--dry-run` previews the exact
  removal set without confirmation.

## Recovery

Interrupted transactions leave a journal under `~/.cortex-ia/`. Two
commands cover them:

```bash
cortex-ia recover list        # read-only: candidate journals + reasons
cortex-ia recover <journal-id>  # typing the exact ID confirms the run
```

`doctor` detects incomplete validated journals and reports them as a
degraded, recoverable finding — without exposing secrets. `recover` requires
explicit confirmation (typing the exact journal ID), reacquires the home
lock, re-validates provenance, containment, and postimages, then runs the
same reverse-order verified restoration as rollback. The receipt is PASS
only after complete verified restoration. Corrupt, completed, foreign-home,
escaped, or postimage-drifted journals return typed non-success and write
nothing outside the validated transaction. Recovery never chooses a journal
automatically.

## Destructive Command Summary

| Command | Destructive action | Guard |
|---------|--------------------|-------|
| `install`/`sync --overwrite` | Replace unmanaged conflicting files | Explicit flag + interactive `y`/`yes` + verified backup |
| `mcp remove <name>` | Delete the managed MCP entry | Interactive confirmation |
| `rollback [id]` | Restore backup, reverting newer managed writes | Interactive confirmation |
| `recover <journal-id>` | Restore an interrupted transaction | Typing the exact journal ID + validated provenance |
| `uninstall` | Remove the accredited installation | Interactive confirmation |

Piped or non-interactive invocations of any destructive action fail closed
with nothing written — safe for CI, scripts, and salt-driven runs, which
should rely on `--dry-run` for inspection.

## Doctor

`cortex-ia doctor` is read-only. It reports the OpenCode root, the active
selection, per-artifact status counts (`ok`, `missing`, `drifted`,
`irregular`), managed MCP status versus expectation, unknown MCPs, pending
recovery journals, and health findings. A degraded or blocked verdict —
including a recoverable interrupted transaction — exits non-zero so scripts
can gate on it.
