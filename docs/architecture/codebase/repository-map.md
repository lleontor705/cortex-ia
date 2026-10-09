> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# Repository Map

← [Codebase Guide](../codebase-guide.md)

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
- **Role**: Bubble Tea interactive terminal user interface (10 screens).
- **Key Files**: `tui.go`, `model.go`, `views.go`, `actions.go`, `styles/theme.go`.
- **Screens**: `Home`, `Review`, `Running`, `Result`, `MCP Manager`, `CortexIA Web`, `Agent Studio`, `Stats`, `Models`.

### `internal/install`
- **Role**: High-level service facade orchestrating all install, sync, doctor, rollback, uninstall, and MCP operations.
- **Key Files**: `service.go`, `receipt.go`, `doctor.go`, `mcp.go`, `rollback.go`, `uninstall.go`, `txn.go`.

### `internal/pipeline`
- **Role**: Transactional copy engine: planning, backup verification, atomic apply, journaling, rollback recovery.
- **Key Files**: `engine.go`, `plan.go`, `apply.go`, `journal.go`, `pipeline.go`.

### `internal/mcpmanager`
- **Role**: MCP server catalog (`cortex`, `context7`), retired-preset cleanup, desired-entry validation, qualification, and conflict detection.
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
- **Key Files**: `assets.go`, `opencode.jsonc`, `AGENTS.md`, `agents/`, `commands/`, `skills/`, `plugins/`, `themes/`, `tui/`.

---

## 3. Supported Platforms

- **Primary Platform**: **OpenCode** (`~/.config/opencode/`, `~/.agents/skills/`, `~/.cortex-ia/opencode/`)
- **Secondary (legacy)**: **Claude Code** (`~/.claude.json`) — MCP-only `--target claude`; no asset set
- No additional platform targets are planned; the multi-platform adapter surface is retired and fails closed.

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

1. **OpenCode First**: Asset installations target only the declared OpenCode discovery roots (`~/.config/opencode/`, `~/.agents/skills/`) plus the Cortex-IA workflow roots under `~/.cortex-ia/opencode/`; nothing outside those roots is written.
2. **Compile-Time Embedding**: Assets in `internal/assets/` are embedded via `go:embed` and delivered byte-for-byte.
3. **Fail-Closed Verification**: Unmanaged conflicting files are never overwritten without explicit `--overwrite` and user confirmation.
4. **Verified Backup**: A full snapshot is captured and verified before any filesystem mutation begins.

---

← Prev: [Mental Model](mental-model.md) · Next: [MCP Boundaries](mcp-boundaries.md) →
