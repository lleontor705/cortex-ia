# Backups & Rollback

Every install, sync, upgrade, and uninstall in cortex-ia creates a snapshot. You can list them, pin the important ones, prune the rest, and roll back to any of them.

## Where backups live

```
~/.cortex-ia/backups/
├── 20260425-093015/
│   ├── manifest.json     # which files were captured + hashes
│   └── snapshot.tar.gz   # the actual content (gzipped tar)
├── 20260425-101244/
│   └── …
```

## What's in a manifest

```json
{
  "id": "20260425-093015",
  "created_at": "2026-04-25T09:30:15Z",
  "root_dir": "/home/me/.cortex-ia/backups/20260425-093015",
  "source": "install",
  "file_count": 42,
  "created_by_version": "v0.2.1",
  "checksum": "f8c2…",
  "pinned": false,
  "entries": [
    { "original_path": "/home/me/.claude/CLAUDE.md", "snapshot_path": "files/.claude/CLAUDE.md", "existed": true, "mode": 420 }
  ]
}
```

Two new fields vs older manifests (both `omitempty`, so legacy backups still load):

- `checksum` — SHA-256 over the snapshot inputs. Used by `IsDuplicate` to skip creating a new backup when nothing changed since the previous one.
- `pinned` — `true` means the backup is exempt from `Prune`.

## Commands

```
cortex-ia list backups             # list all backups newest-first
cortex-ia backup pin <id>          # pin a backup (never auto-prune)
cortex-ia backup unpin <id>        # unpin
cortex-ia backup prune [--keep N]  # delete oldest unpinned beyond N (default 5)
cortex-ia rollback                 # restore the most recent backup
cortex-ia rollback <id>            # restore a specific backup
```

## Retention policy

`Prune` (also called automatically at the end of `install` / `sync`) keeps the **5 most recent unpinned** backups. Pinned backups never count toward the limit and are never deleted by `Prune`. Configurable via `--keep`.

## Deduplication

When a sync produces the same content as the previous backup (same checksum), `Backup` reuses the previous snapshot rather than creating a duplicate. The lockfile still updates `LastBackupID` to the existing one.

## Rollback semantics

`rollback` restores **all files** captured in the manifest to their original paths. Files not present in the manifest are left untouched. The state.json and lockfile are restored too, so a rollback returns the entire `~/.cortex-ia/` to the snapshotted state.

## Uninstall snapshots

`cortex-ia uninstall` works the same way: before any cleaner runs, the planned file set is captured into `~/.cortex-ia/backups/<timestamp>-uninstall/` with `source: "uninstall"` (constant `backup.BackupSourceUninstall`). The manifest carries the same shape as install snapshots — pin and prune behave identically.

Recipe to undo an uninstall:

```bash
# 1. Uninstall persona + cortex from claude-code (snapshot taken automatically)
cortex-ia uninstall --component persona --component cortex --agent claude-code

# 2. Inspect the snapshot
cortex-ia list backups | grep uninstall

# 3. Roll back if you change your mind
cortex-ia rollback --backup 20260426-093015-uninstall
```

Skip the snapshot only when you're certain you want a one-way uninstall:

```bash
cortex-ia uninstall --all --no-backup --yes
```

`--no-backup` is intentionally non-default because the bulk of the time-cost of an uninstall is the cleaner work, not the snapshot — and the snapshot is the only thing that lets you change your mind.

## When to pin

- Before a known-risky upgrade (`cortex-ia upgrade`)
- Before manually editing one of the managed agent config files
- When you've reached a known-good state you want to preserve indefinitely

Pinned backups are explicit; you have to `cortex-ia backup unpin` to remove that protection.

## Implementation pointers

- `internal/backup/manifest.go` — `Manifest`, `BackupSource`, `WriteManifest`, `ReadManifest`, `BackupRootFn`
- `internal/backup/compression.go` — `CreateArchive`, `ExtractArchive`, `ArchiveEntry`
- `internal/backup/retention.go` — `Prune`, `IsDuplicate`, `ComputeChecksum`, `DefaultRetentionCount = 5`
- `internal/backup/snapshot.go` — high-level snapshot API used by the pipeline
- `internal/pipeline/pipeline.go` — calls `Prune` at the end of Apply
