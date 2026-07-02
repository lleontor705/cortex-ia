# Dashboard & TUI

← [Codebase Guide](../CODEBASE-GUIDE.md)

The interactive Bubbletea dashboard that powers `cortex-ia` with no arguments. This page covers the TUI architecture, screen router, and dependency-injection patterns — it does not cover the CLI subcommand dispatch (see [repository-map.md](repository-map.md)) or the pipeline internals (see [mental-model.md](mental-model.md)).

## Scope Boundary

| Concern | Covered here | Not covered |
|---------|-------------|-------------|
| Bubbletea Model-Update-View architecture | ✅ | — |
| Screen router & welcome hub groups | ✅ | — |
| Progress channel pattern | ✅ | — |
| Dependency-injected callbacks | ✅ | — |
| CLI subcommand dispatch | — | [repository-map.md](repository-map.md) |
| Pipeline execution internals | — | [mental-model.md](mental-model.md) |

## Architecture

`internal/tui/` implements the Elm Architecture (Bubbletea's Model-Update-View):

| Phase | Role | Key Method |
|-------|------|-----------|
| **Model** | Immutable state struct; holds current screen, selection, progress, errors. | `Model` struct in `tui.go` |
| **Update** | Handles `tea.Msg`, transitions screens, processes progress events. | `Update(msg) (Model, tea.Cmd)` |
| **View** | Renders the current screen via lipgloss styles. | `View() string` |

The TUI is launched from `internal/app/` when `cortex-ia` is invoked with no subcommand.

## Screen Router

| Component | Package | Purpose |
|-----------|---------|---------|
| Root model | `internal/tui/tui.go` | Holds active screen, routes messages, manages lifecycle. |
| Screens | `internal/tui/screens/` | Individual screen implementations (28 screens). |
| Styles | `internal/tui/styles/` | Colors, padding, layout via lipgloss. `theme.go` centralizes palette. |
| Welcome hub | `internal/tui/screens/` | Landing menu grouping actions into SETUP / CUSTOMIZE / MAINTAIN. |

### Welcome Hub Groups

| Group | Actions | Trigger |
|-------|---------|---------|
| **SETUP** | Detect agents, install agents, select components, apply preset | First-run onboarding |
| **CUSTOMIZE** | Toggle components, select persona, configure model assignments, manage skills, manage profiles | Ongoing configuration |
| **MAINTAIN** | Verify/doctor, repair, rollback, uninstall, update, sync | Ongoing maintenance |

## Dependency Injection

The TUI does **not** import the pipeline directly. Operations are injected as function-typed fields, enabling testability and decoupling.

| Callback Type | Purpose | Real impl |
|--------------|---------|-----------|
| `ExecuteFn` | Run a full install/sync from a `Selection`. | `pipeline.Install` |
| `SyncFn` | Reconcile drift from saved state. | `pipeline.SelectionFromState → Install` |
| `RestoreFn` | Roll back to a backup snapshot. | `backup.RestoreService.Restore` |

| Benefit | Detail |
|---------|--------|
| Testability | Tests inject mock callbacks; no real filesystem mutation needed. |
| Decoupling | `internal/tui` has no import dependency on `internal/pipeline`. |
| Configurability | The CLI layer (`internal/app`) wires real implementations; the TUI is agnostic. |

## Progress Channel

Long-running operations (install, sync, restore) report progress through a Go channel consumed by the TUI's `Update` loop.

```
ExecuteFn(selection) → progressCh ← TUI Update() polls via tea.Tick
                        ↓
              tea.Msg (progress update) → View() re-renders progress bar
```

| Aspect | Detail |
|--------|--------|
| Channel type | `chan ProgressEvent` (or equivalent struct msg) |
| Consumer | TUI `Update` loop via `tea.Tick` / `tea.Cmd` |
| Rendering | Progress bar + current step label via lipgloss |
| Completion | Close channel → final status msg → return to hub or error screen |

### Progress Invariants

- The TUI never blocks on a synchronous pipeline call — work happens in a goroutine.
- Progress events carry a human-readable label for the current step.
- Errors surface as a dedicated message type, not a panic.

## Invariants

- `internal/tui/` has zero direct imports of `internal/pipeline` — all pipeline access is via injected callbacks.
- The root `Model` is immutable; `Update` returns a new `Model`, never mutates in place.
- All 28 screens live under `internal/tui/screens/`; the root model dispatches by screen enum.
- Styles are centralized in `internal/tui/styles/theme.go`; screens do not hardcode colors.

## Contributor Checklist

- [ ] Adding a screen? Create it in `internal/tui/screens/`, add the screen enum value, wire it into the router's `Update`/`View` dispatch.
- [ ] Adding a welcome hub action? Place it in the correct group (SETUP / CUSTOMIZE / MAINTAIN) and add a navigation entry.
- [ ] Adding a new operation type? Define a new `*Fn` callback field on the Model, inject the real impl from `internal/app`, do not import pipeline in tui.
- [ ] Changing colors or layout? Edit `internal/tui/styles/theme.go` only — do not scatter styles across screens.
- [ ] Long-running work? Report progress through the channel pattern; never block `Update` synchronously.
- [ ] Test TUI logic with injected mock callbacks — do not spin up a real install in unit tests.

---

← Prev: [Sync, State & Backup](sync-and-cloud.md) · Next: [Integrations](integrations.md) →
