# Plan — TUI Menu Reordering and Interactive Software Upgrade Screen (sdd-lite & sdd-full)

Change: `tui-reorder-and-upgrade` | Workflow: sdd-full | Spec plane: hybrid

## Intent

Reorganize the Cortex-IA TUI main menu in `internal/tui/` into four coherent functional groups (Gestión/Configuración, Herramientas/Dashboards, Mantenimiento, Ciclo de vida), and introduce a dedicated, interactive sub-screen (`screenUpgrade`) in `upgrade_screen.go` allowing users to check for GitHub releases on demand, inspect release notes and work authority, and trigger binary upgrades directly from within the TUI.

Non-goals: No modification of low-level updater verification mechanics (`internal/updater/`); no silent automated background updates without operator consent; no changes to CLI argument structures; no changes to `UpdateState` schema format.

## Requirements

Delta requirements and Given/When/Then scenarios are fully defined in `specs/tui/spec.md`:
- `REQ-TUI-001`: Home menu reordered into functional groups with exact invariant preservation (`len(homeEntries) == len(homeDescriptions) == 11`).
- `REQ-TUI-002`: Dedicated hotkeys for sub-screens (`m`/`M` for models, `p`/`P` for providers, `u`/`U` for upgrade).
- `REQ-TUI-003`: Interactive upgrade screen FSM and presentation (`upgradeState` covering idle, checking, up_to_date, update_available, confirm, applying, success, blocked, error).
- `REQ-TUI-004`: Authority gating and safe update application (`delegation.db` active claims/leases check before apply).
- `REQ-TUI-005`: Home menu update badge notification (`[NEW vX.Y.Z]` indicator on entry 8).

## Design

Full architecture, FSM state diagrams, and layout wireframes are defined in `design.md`:
- `upgrade_screen.go`: Self-contained Bubble Tea component with `upgradeState`, FSM phases, actions, typed messages (`upgradeCheckMsg`, `upgradeApplyMsg`), and testable package seams (`upgradeCheckSeam`, `upgradeApplySeam`).
- `model.go` & `views.go`: Addition of `screenUpgrade`, updated index constants (`modelsEntryIndex = 2`, `providersEntryIndex = 3`, `webEntryIndex = 4`, `statsEntryIndex = 5`, `studioEntryIndex = 6`, `doctorEntryIndex = 7`, `upgradeEntryIndex = 8`, `uninstallEntryIndex = 9`, `quitEntryIndex = 10`), numeric hotkeys 1-9 mapping to indices 0-8, and conditional badge rendering.
- Test Isolation: Hermetic unit testing using temporary test directories and stubbed seams without network or database side-effects.

## Tasks

Sequential implementation DAG defined in `tasks.md`:
- [ ] task-tui-001 — [tui] Reorganize Home menu entries, descriptions, hotkeys, and navigation suites. (Requirements: REQ-TUI-001, REQ-TUI-002)
- [ ] task-tui-002 — [tui] Implement upgrade_screen.go FSM, Bubble Tea messages, and updater seams. (Requirements: REQ-TUI-003, REQ-TUI-004)
- [ ] task-tui-003 — [tui] Wire upgrade screen into model.go, views.go, and add Home update badge. (Requirements: REQ-TUI-002, REQ-TUI-003, REQ-TUI-005)
- [ ] task-tui-004 — [tui] Create upgrade_screen_test.go covering all FSM states and transitions. (Requirements: REQ-TUI-001, REQ-TUI-003, REQ-TUI-004, REQ-TUI-005)
