# Repository Map

← [Codebase Guide](../CODEBASE-GUIDE.md)

Directory-by-directory map of the active `cortex-ia` codebase.

---

## 1. Top-Level Entry Points

| Path | Purpose | Key Files |
| :--- | :--- | :--- |
| `cmd/cortex-ia/` | Application entry point with version injection via `-ldflags`. | `main.go` |
| `go.mod` | Module `github.com/lleontor705/cortex-ia`, Go 1.26.1+. | — |

---

## 2. Core Architecture Packages

### `internal/app`
- **Role**: Command-line dispatcher, intent parsing, receipt rendering, and retired surface fail-closed guard.
- **Key Files**: `app.go`, `cli.go`, `version.go`.
- **Command Surface**: `install`, `sync`, `mcp`, `doctor`, `rollback`, `recover`, `uninstall`, `version`, `help`.

### `internal/tui` & `internal/tui/styles`
- **Role**: Bubble Tea interactive terminal user interface (5 screens).
- **Key Files**: `tui.go`, `model.go`, `views.go`, `actions.go`, `styles/theme.go`.
- **Screens**: `Home`, `Review`, `Running`, `Result`, `MCP Manager`.

### `internal/install`
- **Role**: High-level service facade orchestrating all install, sync, doctor, rollback, uninstall, and MCP operations.
- **Key Files**: `service.go`, `receipt.go`, `doctor.go`, `mcp.go`, `rollback.go`, `uninstall.go`, `txn.go`.

### `internal/pipeline`
- **Role**: Transactional copy engine: planning, backup verification, atomic apply, journaling, rollback recovery.
- **Key Files**: `engine.go`, `plan.go`, `apply.go`, `journal.go`, `pipeline.go`.

### `internal/mcpmanager`
- **Role**: MCP server catalog (`cortex`, `forgespec`, `context7`), desired-entry validation, qualification, and conflict detection.
- **Key Files**: `mcpmanager.go`, `presets.go`, `evidence.go`.

### `internal/state` & `internal/installmeta`
- **Role**: State persistence under `~/.cortex-ia/`: metadata v2, lock agreement, semantic and postimage digests.
- **Key Files**: `metadata_v2.go`, `lock_v2.go`, `agreement_v2.go`, `fingerprints.go`.

### `internal/backup`
- **Role**: Pre-mutation snapshots, manifests, restore verification, deduplication and retention pruning.
- **Key Files**: `backup.go`, `manifest.go`, `restore.go`.

### `internal/agents/opencode`
- **Role**: Declarative OpenCode layout rules and pure asset mapping.
- **Key Files**: `layout.go`, `assetmap.go`.

### `internal/components/filemerge`
- **Role**: JSONC 3-way decode/merge, atomic file writing, and comments preservation.
- **Key Files**: `json_merge.go`, `json_file.go`.

### `internal/assets`
- **Role**: Embedded runtime source assets (`go:embed`).
- **Key Files**: `assets.go`, `opencode.jsonc`, `AGENTS.md`, `agents/`, `commands/`, `skills/`, `plugin/`.

---

## 3. Supported Platforms & Future Roadmap

- **Active Platform**: **OpenCode** (`~/.config/opencode/`)
- **Future Targets**: **Google Antigravity** (`~/.gemini/antigravity/`), **Claude CLI** (`~/.claude/`)

## 4. Project-Level Files

| Path | Purpose |
| :--- | :--- |
| `docs/` | Comprehensive documentation (`architecture.md`, `agents.md`, `components.md`, `mcp.md`, `codebase/`). |
| `scripts/install.sh` | Curl-pipe installer for Unix systems. |
| `.goreleaser.yaml` | Cross-platform release build automation. |
| `Makefile` | Build, test, lint, coverage, and install targets. |
| `.github/workflows/` | CI test gates and automated release pipelines. |

---

## 5. Architectural Invariants

1. **OpenCode First**: All asset installations target `~/.config/opencode/` without touching other locations.
2. **Compile-Time Embedding**: Assets in `internal/assets/` are embedded via `go:embed` and delivered byte-for-byte.
3. **Fail-Closed Verification**: Unmanaged conflicting files are never overwritten without explicit `--overwrite` and user confirmation.
4. **Verified Backup**: A full snapshot is captured and verified before any filesystem mutation begins.

---

← Prev: [Mental Model](mental-model.md) · Next: [MCP Boundaries](mcp-boundaries.md) →
