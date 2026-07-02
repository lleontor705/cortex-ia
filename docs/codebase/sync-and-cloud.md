# Sync, State & Backup

← [Codebase Guide](../CODEBASE-GUIDE.md)

How cortex-ia tracks what it installed, how it restores managed files, and how a subsequent `sync` reconciles drift. This page covers local state persistence and on-disk backup only — cloud synchronization is **not implemented** and is listed as roadmap. For the install pipeline mechanics see [mental-model.md](mental-model.md); for file paths see [repository-map.md](repository-map.md).

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| State files (`state.json`, lockfile, status) | ✅ | — |
| Backup snapshots, manifest, restore | ✅ | — |
| Local sync reconciliation (`SelectionFromState → Install`) | ✅ | — |
| Snapshot pruning & retention | ✅ | — |
| Remote / cloud sync | ❌ roadmap | — |
| Install pipeline step ordering | — | [mental-model.md](mental-model.md) |

## State Files

All state lives under `~/.cortex-ia/`. Owned by `internal/state/`.

| File | Purpose | Owner | Written by |
|------|---------|-------|-----------|
| `state.json` | Installed agents, active preset, selected components, persona, model assignments. Source of truth for `sync`. | `internal/state/` | `pipeline.Apply` on install/sync |
| `cortex-ia.lock` | Concrete list of files written to disk (checksums + paths). Enables drift detection and clean uninstall. | `internal/state/` | `pipeline.Apply`, `filemerge` |
| `install-status.json` | Crash-detection marker. Written before apply, cleared after success. Presence at startup triggers repair prompt. | `internal/state/` | `pipeline` runner |
| `profiles/` | Named preset profiles (agent + component bundles). | `internal/state/` | `cortex-ia profiles` |
| `skills/` | Installed skill manifests and metadata. | `internal/state/` | `cortex-ia skill` |

### State Invariants

- `state.json` is the single source of truth for "what is installed" — it drives `sync`.
- `cortex-ia.lock` must contain every managed file path; absence of a path means cortex-ia did not write it.
- `install-status.json` is **ephemeral** — a leftover file means the previous run crashed mid-apply.
- State files are JSON; the lockfile is regenerated from actual disk state during `sync`.

## Backup System

Owned by `internal/backup/`. Every `install`/`sync` snapshots managed files before mutation.

| Component | Key Type / File | Purpose |
|-----------|----------------|---------|
| Snapshotter | `snapshot.go` | Captures pre-install file state into a tar.gz snapshot. |
| Manifest | `manifest.go` | Manifest format: per-snapshot metadata with pinned flag, checksum dedup, file list. |
| RestoreService | `restore.go` | `cortex-ia rollback` reads a snapshot manifest and restores original files. |
| Prune | `snapshot.go` | Retention policy: keeps most-recent N unpinned snapshots (default 5). Pinned snapshots never pruned. |
| Checksum | `ComputeChecksum` | Content-addresses snapshot entries to dedup identical files across snapshots. |

### Backup Lifecycle

```
install/sync
  → Snapshotter.Snapshot()       // capture current managed files
    → ComputeChecksum per file    // content dedup
      → write tar.gz + manifest
        → Prune (keep ≤ 5 unpinned)

rollback
  → RestoreService.Restore(snapshot_id)
    → read manifest → extract tar.gz → overwrite managed files
```

| Parameter | Default | Configurable |
|-----------|---------|-------------|
| Max unpinned snapshots | 5 | Retention count in Prune |
| Compression | tar.gz | No |
| Pinned snapshots | Never auto-pruned | User pins via CLI |
| Checksum algorithm | SHA-256 (per-file) | No |

## Local Sync Flow

Sync is **local only**. There is no remote server, no cloud account, no cross-machine sync.

```
cortex-ia sync
  → pipeline.SelectionFromState()   // rebuild Selection from state.json
    → pipeline.Install(selection)    // 2-stage: Prepare (validate + backup) → Apply (parallel chains)
      → update state.json + cortex-ia.lock
```

`SelectionFromState()` reconstructs the install `Selection` from persisted state, then runs the normal install pipeline. This reconciles drift if managed files were manually edited or deleted.

### Sync Invariants

- Sync never fetches from the network (state is local).
- A successful sync leaves `state.json` and `cortex-ia.lock` consistent.
- If `install-status.json` exists at sync start, sync aborts and prompts `repair`.

## Cloud Sync (Roadmap — Not Implemented)

| Capability | Status |
|-----------|--------|
| Remote state server | ❌ Not implemented |
| Cross-machine profile sync | ❌ Not implemented |
| Account / auth | ❌ Not implemented |
| Conflict resolution | ❌ Not implemented |

Cloud sync is a planned future capability. Do not assume any cloud-related code paths exist. All persistence is currently local filesystem under `~/.cortex-ia/`.

## Invariants

- `state.json` is the source of truth for installed state; `cortex-ia.lock` is the source of truth for written files.
- A snapshot is taken **before** any managed file is mutated — never after.
- Pinned snapshots survive Prune indefinitely.
- `install-status.json` present at startup ⇒ previous run crashed ⇒ trigger repair flow.
- No network calls occur during `sync`; all reconciliation is local.

## Contributor Checklist

- [ ] Changing what gets persisted? Update `state.json` schema in `internal/state/state.go` and bump/validate the version field.
- [ ] Adding a new managed file type? Ensure `cortex-ia.lock` records it so `uninstall` cleans it up.
- [ ] Modifying backup format? Regenerate golden test fixtures and verify `RestoreService` round-trips.
- [ ] Changing Prune retention? Update the default constant and document the new number.
- [ ] Adding crash-detection logic? Write through `install-status.json`, not a new file.
- [ ] Do **not** add network calls to the sync path — cloud sync is roadmap, keep local-only contracts intact.

---

← Prev: [Interfaces](interfaces.md) · Next: [Dashboard & TUI](dashboard.md) →
