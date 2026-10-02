# Proposal — TUI Menu Reordering and Interactive Software Upgrade Screen

Change: `tui-reorder-and-upgrade` | Domain: `tui` | Plane: `hybrid`

## Problem

The Cortex-IA interactive terminal interface (`internal/tui/`) has evolved through several feature additions (models configuration, custom providers catalog), resulting in an organically grown main menu where configuration items are split across the top and bottom of the list. Currently, the menu contains 10 entries with disjoint logical grouping (e.g., "Install custom provider" sits at index 9, while "Install / Sync" is at index 0 and "Manage MCPs" is at index 1).

Furthermore, updating the Cortex-IA binary currently requires either accepting a transient one-line prompt at boot time (`MaybePromptUpdate` in `update_prompt.go`) or dropping out of the TUI into the CLI (`cortex-ia update`). Users inside the TUI have no dedicated, interactive screen to check whether an update is available, inspect release notes, verify that work authority (active claims, leases, or running tasks) is clear, and apply the update without leaving the interactive workflow.

## User value

1. **Intuitive Menu Hierarchy**: The Home menu is reorganized into four cohesive functional categories:
   - **Gestión / Configuración**: Install / Sync, Manage MCPs, Configuración de modelos, Install custom provider.
   - **Herramientas / Dashboards**: CortexIA Web Console, Estadísticas de uso, Agent Studio.
   - **Mantenimiento**: Doctor / Recovery, Actualizar software (Upgrade).
   - **Ciclo de vida**: Uninstall, Quit.
2. **First-Class In-TUI Upgrade Experience**: A dedicated interactive sub-screen (`screenUpgrade`) allows users to trigger asynchronous GitHub Release checks on demand, inspect current and latest release versions and notes, and safely apply binary upgrades with preflight authority checks.
3. **Operational Safety & Authority Protection**: Upgrades cannot proceed if active claims or unreleased file leases exist in `delegation.db`, preventing binary replacement while agentic tasks are executing.
4. **Immediate Visibility**: An unobtrusive badge on the Home menu notifies users whenever a newer release is ready to install.

## Approach

1. **Reorder Home Menu Arrays and Constants**: Reorder `homeEntries` and `homeDescriptions` to 11 items. Adjust cursor index constants (`statsEntryIndex`, `providersEntryIndex`, etc.), bind numeric keys 1-9 to indices 0-8, and update `navigation_test.go` and `providers_home_test.go` to maintain invariant parity.
2. **Self-Contained `upgrade_screen.go`**: Implement `upgradeState` encapsulating an explicit FSM (`idle`, `checking`, `up_to_date`, `update_available`, `confirm`, `applying`, `success`, `blocked`, `error`), along with Bubble Tea commands and typed messages (`upgradeCheckMsg`, `upgradeApplyMsg`).
3. **Pluggable Test Seams**: Introduce package-level seams (`upgradeCheckSeam`, `upgradeApplySeam`) delegating by default to `updater.New("")` and `probeActiveAuthority`, enabling hermetic unit tests with zero network or filesystem side-effects.
4. **Wire-up & Home Badge**: Connect `screenUpgrade` to `model.go` and `views.go`, add hotkey `u`/`U`, and conditionally display a `[NEW: vX.Y.Z]` indicator on the Home menu.
5. **Modular Test Coverage**: Implement `upgrade_screen_test.go` (bounded to <= 250 LOC) covering all states, transitions, error handling, and authority gating.

## Non-goals

- No modifications to the cryptographic verification pipelines in `internal/updater/` (checksums.txt / SHA-256 verification remains owned by updater).
- No automatic silent binary downloads in the background without explicit user action or consent.
- No changes to CLI flags or CLI update syntax (`cortex-ia update`).
- No modifications to `UpdateState` JSON schema format.

## Risks & Mitigations

- **Risk: Breaking Existing Menu Navigation Tests**: The navigation and custom providers tests rely on specific cursor positions and numeric keys.
  - *Mitigation*: Update `navigation_test.go` and `providers_home_test.go` in Layer 1 to match the new constants and indices before introducing new screens.
- **Risk: Binary Corruption or Process Replacement During Active Work**: Replacing the running executable while tasks are in progress could cause undefined behavior or state inconsistency.
  - *Mitigation*: Enforce `probeAuthority` check against `delegation.db` before triggering binary replacement; fail closed if tasks, leases, or claims are active.
- **Risk: File Bloat in TUI Package**: Adding screens can cause `model.go` and `views.go` to balloon in size.
  - *Mitigation*: Follow the modular design of `providers_screen.go` where `upgradeState` owns its own `update(msg)` and `view(width)` methods, keeping edits in `model.go` and `views.go` minimal.
