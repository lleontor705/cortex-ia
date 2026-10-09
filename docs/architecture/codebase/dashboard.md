> **English only** — this page has no Spanish translation yet. The Spanish site
> falls back to this English version.
>
> **Solo en inglés** — esta página aún no tiene traducción al español. El sitio en
> español muestra esta versión en inglés.

# Dashboard & TUI

← [Codebase Guide](../codebase-guide.md)

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

## 2. Ten-Screen Workflow

The TUI exposes **ten** conceptual screens (`internal/tui/model.go`) with a global destructive-action confirmation overlay. The production build boots directly into the usage Stats panel (`productionBootScreen`); the Home dashboard remains the navigation entry point for every Home-menu action.

```text
[ Home Dashboard ]
  ├── 1. Install / Sync            ──▶ [ Review Plan & MCPs ] ──▶ (Confirm Overwrite?) ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  ├── 2. Manage MCPs               ──▶ [ MCP Manager Screen ] ──▶ (Confirm Remove?)     ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  ├── 3. CortexIA Web Console      ──▶ [ Web Console Screen ]   (auto-starts the loopback console)
  ├── 4. Agent Studio              ──▶ [ Archetype Select ] ──▶ [ Preview & Confirm ]   ──▶ [ Agent Studio Result ]
  ├── 5. Usage Stats               ──▶ [ Usage Stats Screen ]   (also the production boot screen)
  ├── 6. Doctor / Recovery         ──▶ [ Running Pipeline ]     ──▶ [ Result Receipt ]
  ├── 7. Uninstall                 ──▶ (Confirm Modal)          ──▶ [ Running Pipeline ] ──▶ [ Result Receipt ]
  ├── 8. Quit
  └── 9. Model Configuration       ──▶ [ Models Screen ]
```

### Screen Details

1. **`screenHome`**: Landing dashboard with stylized ASCII logo banner, OpenCode status indicator, numbered Home-menu options, and direct hotkey navigation.
2. **`screenReview`**: Reactive plan inspector. Displays MCP toggles (`[x] cortex`, `[ ] context7`) with live re-planning, delegation choices, categorized operation badges, and overwrite warning toggle.
3. **`screenRunning`**: Asynchronous execution timeline with high-framerate dot spinner (`⠋ ⠙ ⠹ ...`) and numbered stage progression (`Plan` → `Backup` → `Apply` → `Verify` → `Commit`).
4. **`screenResult`**: Comprehensive receipt card with `PASS`/`FAIL` Hero badge, changed artifact count, verified backup ID, detailed scrollable log, and one-key rollback trigger (`[ r ]`).
5. **`screenMCP`**: Interactive MCP catalog table with accreditation badges (`managed`, `absent`, `conflict`) and single-key add/remove toggling (`space`/`enter`).
6. **`screenWeb`**: CortexIA Web Console launcher. Starts and reports the loopback operations console (`cortex-ia web`) without leaving the TUI.
7. **`screenAgentStudio`**: Two-step sub-agent authoring flow — archetype selection, then a preview/confirm step (with an overwrite confirmation modal) that installs the generated agent asset and renders its result.
8. **`screenStats`**: Usage stats panel and the production boot surface. Shows per-model month-to-date and 24h usage against the nan catalog quota, with the `:model` picker for model and effort selection.
9. **`screenModels`**: Model-configuration screen for inspecting and assigning agent models, including variants and effective sources.

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

- Centralized in `internal/tui/styles/theme.go` with the unified brand palette (Primary Violet `#7C3AED`, Secondary Cyan `#06B6D4`, Success Emerald `#10B981`, Warning Amber `#F59E0B`, Error Rose `#F43F5E`, Muted Slate `#64748B`, White Slate `#F8FAFC`, navy dark background `#0A0E17`).
- **Responsive Clamping**: `clampScreen` dynamically adapts header, content, and footer to terminal heights down to 16 rows.

