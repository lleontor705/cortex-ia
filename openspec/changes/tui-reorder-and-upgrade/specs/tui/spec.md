# Spec delta — TUI Menu Reordering and Interactive Software Upgrade Screen

Domain: tui | Plane: hybrid | Change: tui-reorder-and-upgrade

## ADDED Requirements

### Requirement: REQ-TUI-001 Home menu reordered into functional groups with exact invariant preservation
The Home menu entries and descriptions SHALL be ordered into four functional groups: Configuration (0: Install / Sync, 1: Manage MCPs, 2: Configuración de modelos, 3: Install custom provider), Dashboards (4: CortexIA Web Console, 5: Estadísticas de uso, 6: Agent Studio), Maintenance (7: Doctor / Recovery, 8: Actualizar software (Upgrade)), and Lifecycle (9: Uninstall, 10: Quit). Invariants `len(homeEntries) == len(homeDescriptions)`, zero empty strings, and zero duplicates MUST be strictly maintained.

#### Scenario: Grouped menu display order and description parity
- **GIVEN** a freshly initialized TUI model with screenHome active
- **WHEN** the Home menu is rendered
- **THEN** exactly 11 entries are displayed in the specified order and each entry matches its counterpart in homeDescriptions

#### Scenario: Digit keys 1-9 map to indices 0-8 in order
- **GIVEN** the Home menu is active
- **WHEN** the user presses any numeric key from 1 through 9
- **THEN** the selection jumps directly to the corresponding entry index from 0 to 8

#### Scenario: Cursor navigation bounds and selection of lifecycle entries
- **GIVEN** the Home cursor is at entry 8 ("Actualizar software (Upgrade)")
- **WHEN** the user presses down arrow twice
- **THEN** the cursor lands on entry 9 ("Uninstall") and then entry 10 ("Quit"), and further down arrow presses do not advance past entry 10

### Requirement: REQ-TUI-002 Dedicated hotkeys for sub-screens
The TUI Home menu SHALL provide direct single-key hotkey navigation to dedicated screens: `m`/`M` for models configuration, `p`/`P` for custom providers, and `u`/`U` for software upgrade, preserving user muscle memory regardless of menu index shifts.

#### Scenario: Hotkey u opens the upgrade screen from Home
- **GIVEN** the Home menu is active
- **WHEN** the user presses u or U
- **THEN** the active screen transitions to screenUpgrade and the upgrade check command is dispatched

#### Scenario: Hotkeys p still open the providers screen at its new index
- **GIVEN** the Home menu is active
- **WHEN** the user presses p or P
- **THEN** the active screen transitions to screenProviders with its catalog loaded

#### Scenario: Hotkeys m still open the models configuration screen at its new index
- **GIVEN** the Home menu is active
- **WHEN** the user presses m or M
- **THEN** the active screen transitions to screenModels with its registry loaded

### Requirement: REQ-TUI-003 Interactive upgrade screen FSM and presentation
The TUI SHALL provide a dedicated interactive screen screenUpgrade driven by upgradeState supporting phases: idle, checking, up_to_date, update_available, confirm, applying, success, blocked, and error. It SHALL render current version, available version, release date, truncated release notes, and actionable hotkey prompts.

#### Scenario: Checking state queries GitHub Releases via updater seam
- **GIVEN** screenUpgrade is opened or the user presses c or Enter on "Comprobar ahora"
- **WHEN** the checking phase is triggered
- **THEN** upgradeCheckCmd invokes the updater seam asynchronously and the view displays a progress indicator

#### Scenario: Up-to-date state reports current version matching release
- **GIVEN** the updater seam reports that the current version is equal to or newer than the latest published release
- **WHEN** upgradeCheckMsg is processed
- **THEN** the phase transitions to upgradePhaseUpToDate and indicates that the software is up to date

#### Scenario: Update available state displays newer version and release notes
- **GIVEN** the updater seam discovers a newer release tag than the current binary version
- **WHEN** upgradeCheckMsg arrives
- **THEN** the phase transitions to upgradePhaseAvailable, displaying the candidate tag, published date, formatted release notes preview, and an update prompt

### Requirement: REQ-TUI-004 Authority gating and safe update application
The upgrade workflow SHALL verify authority state via delegation.db before applying updates. If active authority (claims, leases, or in-progress tasks) or schema drift exists, the upgrade SHALL transition to upgradePhaseBlocked and refuse to download or replace the binary. Applying updates SHALL require explicit user confirmation and SHALL record the applied floor upon success.

#### Scenario: Active authority blocks update with clear explanation
- **GIVEN** screenUpgrade detects an available update but active claims or leases exist in delegation.db
- **WHEN** authority is evaluated during check or apply
- **THEN** the screen transitions to upgradePhaseBlocked with instructions to complete or reconcile active work before upgrading

#### Scenario: User confirmation required before applying update
- **GIVEN** screenUpgrade is in upgradePhaseAvailable with authority clear
- **WHEN** the user presses u or Enter to update
- **THEN** the screen enters upgradePhaseConfirm prompting for explicit confirmation [y/N], and pressing n or Esc cancels the operation

#### Scenario: Successful update records applied floor and instructs restart
- **GIVEN** the user confirms update application under upgradePhaseConfirm
- **WHEN** upgradeApplyCmd finishes downloading, verifying, and applying the binary
- **THEN** the applied floor is persisted to update state, upgradePhaseSuccess is shown, and the user is prompted to press q to restart

### Requirement: REQ-TUI-005 Home menu update badge notification
The Home menu SHALL render a visible update badge indicator beside entry 8 ("Actualizar software (Upgrade)") when an update is detected as available in cached update state or from a completed in-session check.

#### Scenario: Home renders notification badge when update is available
- **GIVEN** cached update state or an in-session check confirms a newer version is available
- **WHEN** the Home menu is rendered
- **THEN** entry 8 renders a highlighted badge indicating a new release is available

#### Scenario: Home omits badge when software is up to date
- **GIVEN** cached update state reports no newer version is available
- **WHEN** the Home menu is rendered
- **THEN** entry 8 renders without an update badge

#### Scenario: Home recovers clean badge state after update applied
- **GIVEN** an update was successfully applied in screenUpgrade
- **WHEN** the user returns to the Home menu via Esc or b
- **THEN** the update badge is updated or cleared to reflect the newly applied release floor
