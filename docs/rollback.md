# Backups & Rollback

Every install, sync, and uninstall in cortex-ia creates a snapshot. You can list them and roll back to any of them; older unpinned snapshots are pruned automatically.

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
cortex-ia rollback list        # list all backups newest-first
cortex-ia rollback             # restore the most recent backup
cortex-ia rollback <backup-id> # restore a specific backup
```

`rollback list` (alias `--list`) is read-only. A real rollback is destructive and requires an interactive terminal and explicit confirmation; piped or closed input fails closed without writing anything.

## Retention policy

`Prune` (also called automatically at the end of `install` / `sync`) keeps the **5 most recent unpinned** backups. Pinned backups never count toward the limit and are never deleted by `Prune`.

## Deduplication

When a sync produces the same content as the previous backup (same checksum), `Backup` reuses the previous snapshot rather than creating a duplicate. The lockfile still updates `LastBackupID` to the existing one.

## Rollback semantics

`rollback` restores **all files** captured in the manifest to their original paths. Files not present in the manifest are left untouched. The state.json and lockfile are restored too, so a rollback returns the entire `~/.cortex-ia/` to the snapshotted state.

## Uninstall snapshots

`cortex-ia uninstall` works the same way: before any cleaner runs, the planned file set is captured into `~/.cortex-ia/backups/<timestamp>-uninstall/` with `source: "uninstall"` (constant `backup.BackupSourceUninstall`). The manifest carries the same shape as install snapshots — pin and prune behave identically.

Recipe to undo an uninstall:

```bash
# 1. Uninstall OpenCode (snapshot taken automatically)
cortex-ia uninstall --target opencode

# 2. Inspect the snapshots
cortex-ia rollback list

# 3. Roll back if you change your mind
cortex-ia rollback <backup-id>
```

The pre-uninstall snapshot is captured automatically; there is no flag to skip it, so a rollback can always restore the previous state.

## Implementation pointers

- `internal/backup/manifest.go` — `Manifest`, `BackupSource`, `WriteManifest`, `ReadManifest`, `BackupRootFn`
- `internal/backup/compression.go` — `CreateArchive`, `ExtractArchive`, `ArchiveEntry`
- `internal/backup/retention.go` — `Prune`, `IsDuplicate`, `ComputeChecksum`, `DefaultRetentionCount = 5`
- `internal/backup/snapshot.go` — high-level snapshot API used by the pipeline
- `internal/pipeline/pipeline.go` — calls `Prune` at the end of Apply
