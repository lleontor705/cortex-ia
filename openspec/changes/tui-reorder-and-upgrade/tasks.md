# Tasks — TUI Menu Reordering and Interactive Software Upgrade Screen

Board: `tui-reorder-and-upgrade` | Workflow: sdd-full | Workload: flexible | Sequential DAG

### Task: task-tui-001 [tui] Reorganize Home menu entries, descriptions, hotkeys, and navigation suites
- Requirements: REQ-TUI-001, REQ-TUI-002
- Depends: none
- Allowed files: internal/tui/model.go, internal/tui/views.go, internal/tui/navigation_test.go, internal/tui/providers_home_test.go
- Objective: Reorder homeEntries and homeDescriptions into the 4 agreed functional groups: Configuration (0: Install/Sync, 1: Manage MCPs, 2: Models, 3: Custom providers), Tools/Dashboards (4: Web Console, 5: Usage stats, 6: Agent Studio), Maintenance (7: Doctor/Recovery, 8: Software Upgrade), and Lifecycle (9: Uninstall, 10: Quit). Update index constants (modelsEntryIndex = 2, providersEntryIndex = 3, webEntryIndex = 4, statsEntryIndex = 5, studioEntryIndex = 6, doctorEntryIndex = 7, upgradeEntryIndex = 8, uninstallEntryIndex = 9, quitEntryIndex = 10) in model.go. Map numeric digits 1-9 to indices 0-8 in updateHome. Update existing navigation_test.go and providers_home_test.go assertions so that test suites compile and pass against the newly indexed menu layout while maintaining the invariant len(homeEntries) == len(homeDescriptions) == 11.
- Acceptance: Given a fresh TUI model when View is called on Home then exactly 11 entries are rendered in order with corresponding descriptions; Given keys 1 through 9 when pressed from Home then cursor jumps to indices 0 through 8; Given navigation_test.go and providers_home_test.go when go test runs then all existing navigation and providers tests pass.
- Verification: go test -count=1 ./internal/tui -run "TestHomeMenuEntries|TestScreenSet|TestNavigationWalksAllScreens|TestHomeNumericHotkeys|TestREQ_PROV_006"

### Task: task-tui-002 [tui] Implement upgrade_screen.go FSM, Bubble Tea messages, and updater seams
- Requirements: REQ-TUI-003, REQ-TUI-004
- Depends: task-tui-001
- Allowed files: internal/tui/upgrade_screen.go
- Objective: Implement internal/tui/upgrade_screen.go encapsulating the complete upgradeState FSM with phases (idle, checking, up_to_date, update_available, confirm, applying, success, blocked, error), actions (none, home, quit), typed messages (upgradeCheckMsg, upgradeApplyMsg), and asynchronous commands (upgradeCheckCmd, upgradeApplyCmd). Define package-level testable seams (upgradeCheckSeam, upgradeApplySeam) integrating with updater.Client (CheckLatest/CheckLatestFresh) and delegation.HasAnyActiveAuthority (probeActiveAuthority). Provide self-contained update(msg tea.KeyMsg) (upgradeState, upgradeAction, tea.Cmd) and view(width int) string methods adhering strictly to the file budget cap of <= 350 LOC.
- Acceptance: Given upgradeState in idle phase when c is pressed then checking phase begins and upgradeCheckCmd dispatches; Given upgradeCheckMsg with hasAuthority=true when processed then state transitions to upgradePhaseBlocked; Given upgradePhaseAvailable when u is pressed then confirm phase prompts for confirmation; Given upgradePhaseSuccess when rendered then restart guidance with [q] is displayed.
- Verification: go build ./internal/tui/...

### Task: task-tui-003 [tui] Wire upgrade screen into model.go, views.go, and add Home update badge
- Requirements: REQ-TUI-002, REQ-TUI-003, REQ-TUI-005
- Depends: task-tui-002
- Allowed files: internal/tui/model.go, internal/tui/views.go, internal/tui/actions.go
- Objective: Extend the screen enum in model.go with screenUpgrade. Wire openUpgrade() in actions.go to switch to screenUpgrade and trigger an initial upgrade check. Route updateUpgrade in model.go to delegate key handling to upgradeState.update and handle resulting navigation actions (home returns to Home menu at upgradeEntryIndex, quit terminates with tea.Quit). Update model.Update to route upgradeCheckMsg and upgradeApplyMsg to upgradeState. Connect screenUpgrade in views.go to render upgradeState.view. Add a conditional badge in viewHome beside entry 8 ("Actualizar software (Upgrade)") when a cached update is detected.
- Acceptance: Given Home menu when u or U is pressed then screen switches to screenUpgrade; Given cursor on entry 8 when Enter is pressed then screen switches to screenUpgrade; Given cached update state with an available update when Home is viewed then entry 8 renders the [NEW ...] badge; Given Esc pressed on screenUpgrade then screen transitions back to Home at cursor 8.
- Verification: go test -count=1 ./internal/tui/...

### Task: task-tui-004 [tui] Create upgrade_screen_test.go covering all FSM states and transitions
- Requirements: REQ-TUI-001, REQ-TUI-003, REQ-TUI-004, REQ-TUI-005
- Depends: task-tui-003
- Allowed files: internal/tui/upgrade_screen_test.go
- Objective: Create internal/tui/upgrade_screen_test.go providing hermetic, comprehensive unit test coverage of all upgradeState FSM phases and transitions without making network calls or touching developer delegation state. Stub upgradeCheckSeam and upgradeApplySeam using t.Cleanup. Assert behavior across: (1) idle to checking, (2) checking to up-to-date, (3) checking to update-available with release notes rendering, (4) active authority blocking with clear guidance, (5) confirmation prompt yes/no handling, (6) applying to success with floor recording and restart instructions, and (7) network/verification failure handling. Guarantee that the test file remains <= 250 LOC.
- Acceptance: Given stubbed upgrade seams when unit tests run then all FSM states (idle, checking, up_to_date, update_available, confirm, applying, success, blocked, error) are exercised and pass; Given test execution when file size is checked then upgrade_screen_test.go contains <= 250 LOC; Given active authority in delegation.db stub when upgrade is attempted then update is blocked and no apply is executed.
- Verification: go test -v -count=1 ./internal/tui -run "TestUpgradeScreen"
