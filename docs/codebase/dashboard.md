# Dashboard & TUI

← [Codebase Guide](../CODEBASE-GUIDE.md)

The interactive Bubble Tea terminal user interface (TUI) powers `cortex-ia` when executed with no arguments. This page covers the Elm-based Model-Update-View architecture, screen states, `ServiceAPI` contract, and confirmation overlays.

---

## 1. Architecture Overview

`internal/tui/` implements the Elm architecture via [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss):

| Phase | Role | Key Method |
| :--- | :--- | :--- |
| **Model** | Holds immutable state: active screen, cursor, MCP selection, plan effects, result receipts, confirm modal. | `model` in `model.go` |
| **Update** | Handles keyboard messages (`tea.KeyMsg`), tick spinner frames, async operation messages (`tea.Cmd`). | `Update(msg) (tea.Model, tea.Cmd)` |
| **View** | Renders styled ASCII banner, cards, timelines, badges, and modals. | `View() string` |

---

## 2. Five-Screen Workflow

The TUI is strictly organized around 5 conceptual screens with a global destructive-action confirmation overlay:

```text
[ Home Dashboard ]
  ├── 1. Install / Sync  ──▶ [ Review Plan & MCPs ] ──▶ (Confirm Overwrite?) ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  ├── 2. Manage MCPs     ──▶ [ MCP Manager Screen ] ──▶ (Confirm Remove?)    ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  ├── 3. Doctor / Health ──▶ [ Running Pipeline ]   ──▶ [ Result Receipt ]
  ├── 4. Uninstall       ──▶ (Confirm Modal)        ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  └── 5. Quit
```

### Screen Details

1. **`screenHome`**: Landing dashboard with stylized ASCII logo banner, OpenCode status indicator, numbered menu options (`1-5`), and direct hotkey navigation.
2. **`screenReview`**: Reactive plan inspector. Displays MCP toggles (`[x] cortex`, `[ ] context7`) with live re-planning, delegation choices, categorized operation badges, and overwrite warning toggle.
3. **`screenRunning`**: Asynchronous execution timeline with high-framerate dot spinner (`⠋ ⠙ ⠹ ...`) and numbered stage progression (`Plan` → `Backup` → `Apply` → `Verify` → `Commit`).
4. **`screenResult`**: Comprehensive receipt card with `PASS`/`FAIL` Hero badge, changed artifact count, verified backup ID, detailed scrollable log, and one-key rollback trigger (`[ r ]`).
5. **`screenMCP`**: Interactive MCP catalog table with accreditation badges (`managed`, `absent`, `conflict`) and single-key add/remove toggling (`space`/`enter`).

---

## 3. Service Decoupling (`ServiceAPI`)

The TUI consumes `internal/install.Service` exclusively through the typed `ServiceAPI` interface:

```go
type ServiceAPI interface {
    Plan(opts install.Options) (*pipeline.Plan, error)
    Install(opts install.Options) (*install.InstallReceipt, error)
    Sync(opts install.Options) (*install.InstallReceipt, error)
    Doctor() (*install.DoctorReport, error)
    Rollback(backupID string) (*install.RollbackReceipt, error)
    Uninstall(opts install.UninstallOptions) (*install.UninstallReceipt, error)
    MCPList() (*install.MCPListReport, error)
    MCPAdd(name string, opts install.MCPOptions) (*install.MCPReceipt, error)
    MCPRemove(name string, opts install.MCPOptions) (*install.MCPReceipt, error)
}
```

- **Test Isolation**: Unit tests run against `fakeService` with zero filesystem side effects.
- **Strict Separation**: The TUI owns no copy, merge, or hash logic; it acts purely as an interactive controller.

---

## 4. Confirmation Modals (`confirmOverlay`)

Destructive operations (`--overwrite`, `uninstall`, `mcp remove`, `rollback`) are guarded by an explicit modal overlay:
- Rendered as an amber-bordered floating dialog.
- Requires explicit `y` keystroke to proceed; `n` or `esc` cancels immediately.
- Reassures users that a verified backup snapshot is always created prior to any mutation.

---

## 5. Styling and Responsiveness

- Centralized in `internal/tui/styles/theme.go` with unified palette (Primary Violet `#7C3AED`, Secondary Cyan `#06B6D4`, Success Green `#22C55E`, Warning Amber `#F59E0B`, Error Red `#EF4444`).
- **Responsive Clamping**: `clampScreen` dynamically adapts header, content, and footer to terminal heights down to 16 rows.

