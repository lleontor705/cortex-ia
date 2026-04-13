// Package tui implements the interactive terminal installer for cortex-ia.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/screens"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle dialog overlay first — intercept all key events when dialog is active
	if m.ActiveDialog.Type != DialogNone {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				switch m.ActiveDialog.Type {
				case DialogRestoreConfirm:
					m.ActiveDialog = Dialog{}
					m.OperationRunning = true
					return m, func() tea.Msg {
						if m.RestoreFn != nil {
							err := m.RestoreFn(m.SelectedBackup)
							return BackupRestoreMsg{Err: err}
						}
						return BackupRestoreMsg{Err: fmt.Errorf("restore not available")}
					}
				case DialogDeleteConfirm:
					m.ActiveDialog = Dialog{}
					m.OperationRunning = true
					return m, func() tea.Msg {
						if m.DeleteBackupFn != nil {
							err := m.DeleteBackupFn(m.SelectedBackup)
							return BackupDeleteMsg{Err: err}
						}
						return BackupDeleteMsg{Err: fmt.Errorf("delete not available")}
					}
				case DialogProfileDelete:
					m.ActiveDialog = Dialog{}
					if m.Cursor < len(m.Profiles) {
						m.Profiles = append(m.Profiles[:m.Cursor], m.Profiles[m.Cursor+1:]...)
						m.saveProfilesToDisk()
					}
					return m, nil
				}
			case "n", "esc":
				m.ActiveDialog = Dialog{}
				return m, nil
			}
			return m, nil // swallow all other keys while dialog is open
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Help.Width = msg.Width
		m.ProgressBar.Width = min(msg.Width-10, 40)
		m.AgentBuilderViewport.Width = min(msg.Width-4, 76)
		m.AgentBuilderViewport.Height = max(msg.Height-12, 10)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		case "q":
			if m.Screen != ScreenInstalling && !m.PipelineRunning && !m.OperationRunning {
				m.Quitting = true
				return m, tea.Quit
			}
		case "?":
			// Toggle full help on screens that are not text input
			if m.Screen != ScreenAgentBuilderPrompt &&
				m.Screen != ScreenRenameBackup &&
				m.Screen != ScreenProfileCreate {
				m.Keys.ShowFullHelp = !m.Keys.ShowFullHelp
				m.Help.ShowAll = m.Keys.ShowFullHelp
				return m, nil
			}
		case "ctrl+h":
			// Home shortcut — return to welcome from any screen
			if m.Screen != ScreenWelcome && m.Screen != ScreenInstalling &&
				!m.PipelineRunning && !m.OperationRunning {
				m.AgentFilter.Deactivate()
				m.SkillFilter.Deactivate()
				m.setScreen(ScreenWelcome)
				return m, nil
			}
		case "t":
			// Toggle dark/light theme on non-input screens
			if m.Screen != ScreenAgentBuilderPrompt &&
				m.Screen != ScreenRenameBackup &&
				m.Screen != ScreenProfileCreate &&
				!m.AgentFilter.Active && !m.SkillFilter.Active {
				styles.ToggleTheme()
				return m, nil
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case StepProgressMsg:
		if msg.Err != nil {
			m.Progress.Mark(m.Progress.Current, ProgressStatusFailed)
			m.Progress.AppendLog("Error: %v", msg.Err)
		} else {
			idx := -1
			for i, item := range m.Progress.Items {
				if item.Label == msg.StepID {
					idx = i
					break
				}
			}
			if idx >= 0 {
				if msg.Status == ProgressStatusRunning {
					m.Progress.Start(idx)
				} else {
					m.Progress.Mark(idx, msg.Status)
				}
			}
		}
		// Re-schedule the channel listener while the pipeline is still running.
		if m.PipelineRunning && m.progressCh != nil {
			return m, listenProgress(m.progressCh)
		}
		return m, nil

	case PipelineDoneMsg:
		m.Result = msg.Result
		m.InstallErr = msg.Err
		m.PipelineRunning = false
		m.setScreen(ScreenComplete)
		return m, nil

	case BackupRestoreMsg:
		m.RestoreErr = msg.Err
		m.OperationRunning = false
		if m.ListBackupsFn != nil {
			m.Backups, m.BackupWarnings = m.ListBackupsFn()
		}
		if msg.Err != nil {
			m.ActiveToast = Toast{Text: fmt.Sprintf("Restore failed: %v", msg.Err), IsError: true, Visible: true}
		} else {
			m.ActiveToast = Toast{Text: "Restore complete", Visible: true}
		}
		return m, dismissToastAfter(3 * time.Second)

	case BackupDeleteMsg:
		m.DeleteErr = msg.Err
		m.OperationRunning = false
		if m.ListBackupsFn != nil {
			m.Backups, m.BackupWarnings = m.ListBackupsFn()
		}
		if msg.Err != nil {
			m.ActiveToast = Toast{Text: fmt.Sprintf("Delete failed: %v", msg.Err), IsError: true, Visible: true}
		} else {
			m.ActiveToast = Toast{Text: "Backup deleted", Visible: true}
		}
		return m, dismissToastAfter(3 * time.Second)

	case SyncDoneMsg:
		m.SyncFilesChanged = msg.FilesChanged
		m.SyncErr = msg.Err
		m.OperationRunning = false
		if m.Screen == ScreenUpgradeSync {
			m.UpgradeSyncPhase = "done"
		}
		return m, nil

	case UpgradeDoneMsg:
		m.UpgradeErr = msg.Err
		m.OperationRunning = false
		return m, nil

	case TransitionFrameMsg:
		cmd := m.ScreenTransition.advanceTransition()
		return m, cmd

	case ToastDismissMsg:
		m.ActiveToast.Visible = false
		return m, nil

	case UpdateCheckResultMsg:
		m.UpdateResults = msg.Results
		m.UpdateCheckDone = true
		m.OperationRunning = false
		// If on UpgradeSync screen, auto-start sync phase
		if m.Screen == ScreenUpgradeSync && m.UpgradeSyncPhase == "checking" {
			m.UpgradeSyncPhase = "syncing"
			m.OperationRunning = true
			profileName := m.SelectedProfile
			return m, func() tea.Msg {
				changed, err := m.SyncFn(profileName)
				return SyncDoneMsg{FilesChanged: changed, Err: err}
			}
		}
		return m, nil
	}

	// Screen-specific update handling
	switch m.Screen {
	case ScreenWelcome:
		return m.updateWelcome(msg)
	case ScreenDetection:
		return m.updateDetection(msg)
	case ScreenAgents:
		return m.updateAgents(msg)
	case ScreenPersona:
		return m.updatePersona(msg)
	case ScreenPreset:
		return m.updatePreset(msg)
	case ScreenClaudeModelPicker:
		return m.updateClaudeModelPicker(msg)
	case ScreenSDDMode:
		return m.updateSDDMode(msg)
	case ScreenStrictTDD:
		return m.updateStrictTDD(msg)
	case ScreenDependencyTree:
		return m.updateDependencyTree(msg)
	case ScreenSkillPicker:
		return m.updateSkillPicker(msg)
	case ScreenReview:
		return m.updateReview(msg)
	case ScreenInstalling:
		return m, nil
	case ScreenComplete:
		return m.updateComplete(msg)
	case ScreenBackups:
		return m.updateBackups(msg)
	case ScreenRestoreConfirm:
		return m.updateRestoreConfirm(msg)
	case ScreenRestoreResult:
		return m.updateRestoreResult(msg)
	case ScreenDeleteConfirm:
		return m.updateDeleteConfirm(msg)
	case ScreenDeleteResult:
		return m.updateDeleteResult(msg)
	case ScreenRenameBackup:
		return m.updateRenameBackup(msg)
	case ScreenUpgrade:
		return m.updateUpgrade(msg)
	case ScreenSync:
		return m.updateSync(msg)
	case ScreenUpgradeSync:
		return m.updateUpgradeSync(msg)
	case ScreenModelConfig:
		return m.updateModelConfig(msg)
	case ScreenProfiles:
		return m.updateProfiles(msg)
	case ScreenProfileCreate:
		return m.updateProfileCreate(msg)
	case ScreenProfileDelete:
		return m.updateProfileDelete(msg)
	case ScreenAgentBuilderEngine:
		return m.updateAgentBuilderEngine(msg)
	case ScreenAgentBuilderPrompt:
		return m.updateAgentBuilderPrompt(msg)
	case ScreenAgentBuilderSDD:
		return m.updateAgentBuilderSDD(msg)
	case ScreenAgentBuilderSDDPhase:
		return m.updateAgentBuilderSDDPhase(msg)
	case ScreenAgentBuilderGenerating:
		return m.updateAgentBuilderGenerating(msg)
	case ScreenAgentBuilderPreview:
		return m.updateAgentBuilderPreview(msg)
	case ScreenAgentBuilderInstalling:
		return m.updateAgentBuilderInstalling(msg)
	case ScreenAgentBuilderComplete:
		return m.updateAgentBuilderComplete(msg)
	case ScreenOpenCodeModels:
		return m.updateOpenCodeModels(msg)
	case ScreenOpenCodeProviderPicker:
		return m.updateOpenCodeProviderPicker(msg)
	case ScreenOpenCodeModelPicker:
		return m.updateOpenCodeModelPicker(msg)
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.Quitting {
		return ""
	}

	var content string
	switch m.Screen {
	case ScreenWelcome:
		content = m.viewWelcome()
	case ScreenDetection:
		content = m.viewDetection()
	case ScreenAgents:
		content = m.viewAgents()
	case ScreenPersona:
		content = m.viewPersona()
	case ScreenPreset:
		content = m.viewPreset()
	case ScreenClaudeModelPicker:
		content = m.viewClaudeModelPicker()
	case ScreenSDDMode:
		content = m.viewSDDMode()
	case ScreenStrictTDD:
		content = m.viewStrictTDD()
	case ScreenDependencyTree:
		content = m.viewDependencyTree()
	case ScreenSkillPicker:
		content = m.viewSkillPicker()
	case ScreenReview:
		content = m.viewReview()
	case ScreenInstalling:
		content = m.viewInstalling()
	case ScreenComplete:
		content = m.viewComplete()
	case ScreenBackups:
		content = m.viewBackups()
	case ScreenRestoreConfirm:
		content = m.viewRestoreConfirm()
	case ScreenRestoreResult:
		content = m.viewRestoreResult()
	case ScreenDeleteConfirm:
		content = m.viewDeleteConfirm()
	case ScreenDeleteResult:
		content = m.viewDeleteResult()
	case ScreenRenameBackup:
		content = m.viewRenameBackup()
	case ScreenUpgrade:
		content = m.viewUpgrade()
	case ScreenSync:
		content = m.viewSync()
	case ScreenUpgradeSync:
		content = m.viewUpgradeSync()
	case ScreenModelConfig:
		content = m.viewModelConfig()
	case ScreenProfiles:
		content = m.viewProfiles()
	case ScreenProfileCreate:
		content = m.viewProfileCreate()
	case ScreenProfileDelete:
		content = m.viewProfileDelete()
	case ScreenAgentBuilderEngine:
		content = m.viewAgentBuilderEngine()
	case ScreenAgentBuilderPrompt:
		content = m.viewAgentBuilderPrompt()
	case ScreenAgentBuilderSDD:
		content = m.viewAgentBuilderSDD()
	case ScreenAgentBuilderSDDPhase:
		content = m.viewAgentBuilderSDDPhase()
	case ScreenAgentBuilderGenerating:
		content = m.viewAgentBuilderGenerating()
	case ScreenAgentBuilderPreview:
		content = m.viewAgentBuilderPreview()
	case ScreenAgentBuilderInstalling:
		content = m.viewAgentBuilderInstalling()
	case ScreenAgentBuilderComplete:
		content = m.viewAgentBuilderComplete()
	case ScreenOpenCodeModels:
		content = m.viewOpenCodeModels()
	case ScreenOpenCodeProviderPicker:
		content = m.viewOpenCodeProviderPicker()
	case ScreenOpenCodeModelPicker:
		content = m.viewOpenCodeModelPicker()
	}

	// Apply transition animation
	content = applyTransition(content, m.ScreenTransition)

	// Add breadcrumb and help
	bc := renderBreadcrumb(m.Screen)
	helpView := m.Help.View(m.screenKeyMap())
	statusBar := renderStatusBar(m)

	var view string
	if bc != "" {
		view = bc + "\n\n" + content + "\n" + helpView
	} else {
		view = content + "\n" + helpView
	}

	// Center content with responsive width
	if m.Width > 0 {
		maxWidth := min(m.Width, 80)
		view = lipgloss.NewStyle().MaxWidth(maxWidth).Render(view)

		// Reserve last line for status bar
		contentHeight := m.Height - 1
		if contentHeight < 1 {
			contentHeight = m.Height
		}
		view = lipgloss.Place(m.Width, contentHeight, lipgloss.Left, lipgloss.Top, view, lipgloss.WithWhitespaceChars(" "))
		view += "\n" + statusBar
	}

	// Render dialog overlay on top of everything
	if m.ActiveDialog.Type != DialogNone {
		view = renderDialog(m.ActiveDialog, m.Width, m.Height)
	}

	// Render toast notification
	if m.ActiveToast.Visible {
		toastLine := renderToast(m.ActiveToast, m.Width)
		if toastLine != "" {
			view = toastLine + "\n" + view
		}
	}

	return view
}

// --- Welcome screen ---

func (m Model) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		opts := welcomeOptions()
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(opts)-1 {
				m.Cursor++
			}
		case "enter":
			if m.Cursor < len(opts) {
				switch opts[m.Cursor] {
				case WelcomeInstall:
					m.RunDetection()
					m.setScreen(ScreenDetection)
				case WelcomeUpgrade:
					m.setScreen(ScreenUpgrade)
				case WelcomeSync:
					m.setScreen(ScreenSync)
				case WelcomeUpgradeSync:
					m.setScreen(ScreenUpgradeSync)
				case WelcomeModelConfig:
					m.ModelConfigMode = true
					m.ClaudeModelCursor = 0
					m.setScreen(ScreenClaudeModelPicker)
				case WelcomeProfiles:
					m.loadProfilesFromDisk()
					m.ProfileErr = nil
					m.setScreen(ScreenProfiles)
				case WelcomeAgentBuilder:
					m.resetAgentBuilder()
					m.setScreen(ScreenAgentBuilderEngine)
				case WelcomeOpenCodeModels:
					m.loadOpenCodeModels()
					m.OCErr = nil
					m.setScreen(ScreenOpenCodeModels)
				case WelcomeBackups:
					if m.ListBackupsFn != nil {
						m.Backups, m.BackupWarnings = m.ListBackupsFn()
					}
					m.RenameErr = nil
					m.RestoreErr = nil
					m.DeleteErr = nil
					m.setScreen(ScreenBackups)
				case WelcomeQuit:
					m.Quitting = true
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m Model) viewWelcome() string {
	opts := welcomeOptions()
	labels := make([]string, len(opts))
	for i, opt := range opts {
		labels[i] = welcomeLabel(opt)
	}

	// Add "(updates available)" badge to Upgrade options when updates are found.
	if m.UpdateCheckDone {
		hasUpdate := false
		for _, r := range m.UpdateResults {
			if r.Error == nil && !r.UpToDate {
				hasUpdate = true
				break
			}
		}
		if hasUpdate {
			for i, opt := range opts {
				if opt == WelcomeUpgrade || opt == WelcomeUpgradeSync {
					labels[i] += " (updates available)"
				}
			}
		}
	}

	return screens.RenderWelcome(screens.WelcomeData{
		Version: m.Version,
		Options: labels,
		Cursor:  m.Cursor,
	})
}

// --- Detection screen ---

func (m Model) updateDetection(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.setScreen(ScreenAgents)
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewDetection() string {
	si := m.SysInfo
	if si == nil {
		return "Detecting..."
	}
	return screens.RenderDetection(screens.DetectionData{
		OS: si.OS, Arch: si.Arch, PkgMgr: si.PkgMgr, Shell: si.Shell,
		NodeVer: si.NodeVer, GitVer: si.GitVer, GoVer: si.GoVer,
		Npx: si.Npx, Cortex: si.Cortex, DetectedAgents: si.DetectedAgents,
	})
}

// --- Agents screen ---

func (m Model) updateAgents(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When filter is active, delegate to filter input first
	if m.AgentFilter.Active {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.AgentFilter.Deactivate()
				return m, nil
			case "enter":
				m.AgentFilter.Deactivate()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.AgentFilter.Input, cmd = m.AgentFilter.Input.Update(msg)
		// Clamp cursor to visible items
		visible := m.visibleAgents()
		if m.Cursor >= len(visible) {
			m.Cursor = max(len(visible)-1, 0)
		}
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		visible := m.visibleAgents()
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(visible)-1 {
				m.Cursor++
			}
		case " ":
			if m.Cursor < len(visible) {
				idx := visible[m.Cursor]
				m.Agents[idx].Selected = !m.Agents[idx].Selected
			}
		case "a":
			allSelected := true
			for _, a := range m.Agents {
				if !a.Selected {
					allSelected = false
					break
				}
			}
			for i := range m.Agents {
				m.Agents[i].Selected = !allSelected
			}
		case "/":
			m.AgentFilter.Activate()
			return m, m.AgentFilter.Input.Focus()
		case "enter":
			if m.HasSelectedAgents() {
				m.AgentFilter.Deactivate()
				m.setScreen(ScreenPersona)
			}
		case "esc":
			m.AgentFilter.Deactivate()
			m.setScreen(ScreenDetection)
		}
	}
	return m, nil
}

// visibleAgents returns indices of agents matching the current filter.
func (m Model) visibleAgents() []int {
	var indices []int
	for i, a := range m.Agents {
		if m.AgentFilter.Matches(a.Name) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m Model) viewAgents() string {
	visible := m.visibleAgents()
	data := make([]screens.AgentData, len(visible))
	for i, idx := range visible {
		a := m.Agents[idx]
		data[i] = screens.AgentData{Name: a.Name, Binary: a.Binary, Selected: a.Selected}
	}
	listHeight := max(m.Height-10, 5)
	content := screens.RenderAgents(screens.AgentsData{Agents: data, Cursor: m.Cursor, MaxHeight: listHeight})
	if m.AgentFilter.Active || m.AgentFilter.Query() != "" {
		return m.AgentFilter.View() + content
	}
	return content
}

// --- Persona screen ---

func (m Model) updatePersona(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Personas)-1 {
				m.Cursor++
			}
		case "enter":
			m.Persona = m.Personas[m.Cursor]
			m.setScreen(ScreenPreset)
		case "esc":
			m.setScreen(ScreenAgents)
		}
	}
	return m, nil
}

func (m Model) viewPersona() string {
	return screens.RenderPersona(screens.PersonaData{
		Personas: m.Personas,
		Cursor:   m.Cursor,
		Selected: m.Persona,
	})
}

// --- Preset screen ---

func (m Model) updatePreset(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Presets)-1 {
				m.Cursor++
			}
		case "enter":
			m.Preset = m.Presets[m.Cursor]
			components := catalog.ComponentsForPreset(m.Preset)
			m.Resolved = catalog.ResolveDeps(components)
			m.setScreen(ScreenClaudeModelPicker)
		case "esc":
			m.setScreen(ScreenPersona)
		}
	}
	return m, nil
}

func (m Model) viewPreset() string {
	return screens.RenderPreset(screens.PresetData{
		Presets:  m.Presets,
		Cursor:   m.Cursor,
		Selected: m.Preset,
	})
}

// --- Review screen ---

func (m Model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "y":
			// Build expected step labels from selected agents x resolved components.
			var labels []string
			for _, a := range m.Agents {
				if a.Selected {
					for _, c := range m.Resolved {
						labels = append(labels, fmt.Sprintf("%s/%s", a.ID, c))
					}
				}
			}
			m.Progress = NewProgressState(labels)

			ch := make(chan StepProgressMsg, 100)
			m.progressCh = ch
			m.setScreen(ScreenInstalling)
			m.PipelineRunning = true
			return m, tea.Batch(m.runInstallWithProgress(ch), listenProgress(ch))
		case "esc", "n":
			if prev, ok := PreviousScreen(ScreenReview); ok {
				m.setScreen(prev)
			}
		}
	}
	return m, nil
}

func (m Model) viewReview() string {
	var reviewAgents []screens.ReviewAgent
	for _, a := range m.Agents {
		if a.Selected {
			reviewAgents = append(reviewAgents, screens.ReviewAgent{Name: a.Name})
		}
	}
	return screens.RenderReview(screens.ReviewData{
		Agents:   reviewAgents,
		Preset:   m.Preset,
		Persona:  m.Persona,
		Resolved: m.Resolved,
	})
}

// --- Installing screen ---

// listenProgress returns a tea.Cmd that blocks on the progress channel and
// yields the next StepProgressMsg. The Update handler re-schedules this
// command after each message so all progress events are delivered.
func listenProgress(ch <-chan StepProgressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil // channel closed — pipeline finished
		}
		return msg
	}
}

func (m Model) runInstallWithProgress(ch chan StepProgressMsg) tea.Cmd {
	return func() tea.Msg {
		components := m.Resolved
		if !m.SDDEnabled {
			filtered := make([]model.ComponentID, 0, len(components))
			for _, c := range components {
				if c != model.ComponentSDD {
					filtered = append(filtered, c)
				}
			}
			components = filtered
		}

		selection := model.Selection{
			Agents:           m.SelectedAgentIDs(),
			Preset:           m.Preset,
			Persona:          m.Persona,
			Components:       components,
			ModelAssignments: m.ModelAssignments,
			StrictTDD:        m.StrictTDDEnabled,
			CommunitySkills:  m.SkillSelection,
		}

		onProgress := func(stepID, status string, err error) {
			ch <- StepProgressMsg{StepID: stepID, Status: status, Err: err}
		}

		var result pipeline.InstallResult
		var installErr error

		if m.ExecuteFn != nil {
			result = m.ExecuteFn(selection, onProgress)
		} else {
			result, installErr = pipeline.Install(m.HomeDir, m.Registry, selection, m.Version, false, onProgress)
		}
		close(ch)

		if installErr == nil && len(result.Errors) > 0 {
			installErr = fmt.Errorf("installation completed with %d warning(s)", len(result.Errors))
		}
		return PipelineDoneMsg{Result: result, Err: installErr}
	}
}

func (m Model) viewInstalling() string {
	if len(m.Progress.Items) == 0 {
		// No step-level progress — show simple spinner
		return screens.RenderInstalling(screens.InstallProgress{
			CurrentStep: "Configuring agents...",
			Items: []screens.ProgressItem{
				{Label: "Configuring agents with cortex-ia ecosystem", Status: ProgressStatusRunning},
			},
		}, m.Spinner.View(), m.ProgressBar)
	}
	return screens.RenderInstalling(m.Progress.ViewModel(), m.Spinner.View(), m.ProgressBar)
}

// --- Complete screen ---

func (m Model) updateComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q":
			m.Quitting = true
			return m, tea.Quit
		case "enter", "esc", "m":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewComplete() string {
	return screens.RenderComplete(screens.CompleteData{
		Err:            m.InstallErr,
		ComponentsDone: len(m.Result.ComponentsDone),
		FilesChanged:   len(m.Result.FilesChanged),
		BackupID:       m.Result.BackupID,
		Errors:         m.Result.Errors,
	})
}

// Run starts the TUI.
func Run(version string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	registry := agents.NewDefaultRegistry()
	m := New(registry, homeDir, version)

	// Inject ExecuteFn — wraps pipeline.Install
	m.ExecuteFn = func(selection model.Selection, onProgress pipeline.ProgressFunc) pipeline.InstallResult {
		var result pipeline.InstallResult
		var installErr error
		if onProgress != nil {
			result, installErr = pipeline.Install(homeDir, registry, selection, version, false, onProgress)
		} else {
			result, installErr = pipeline.Install(homeDir, registry, selection, version, false)
		}
		if installErr != nil {
			result.Errors = append(result.Errors, installErr.Error())
		}
		return result
	}

	// Inject RestoreFn — restores from a backup manifest
	m.RestoreFn = func(manifest backup.Manifest) error {
		svc := backup.RestoreService{}
		return svc.Restore(manifest)
	}

	// Inject DeleteBackupFn — deletes a backup directory
	m.DeleteBackupFn = func(manifest backup.Manifest) error {
		return backup.DeleteBackup(manifest)
	}

	// Inject RenameBackupFn — renames a backup description
	m.RenameBackupFn = func(manifest backup.Manifest, newDescription string) error {
		return backup.RenameBackup(manifest, newDescription)
	}

	// Inject ListBackupsFn — lists available backups
	m.ListBackupsFn = func() ([]backup.Manifest, []string) {
		backupsDir := filepath.Join(homeDir, ".cortex-ia", "backups")
		result := backup.ListManifests(backupsDir)
		return result.Manifests, result.Warnings
	}

	// Inject SyncFn — re-runs install from saved state
	m.SyncFn = func(profileName string) (int, error) {
		s, err := state.Load(homeDir)
		if err != nil {
			return 0, err
		}
		lock, err := state.LoadLock(homeDir)
		if err != nil {
			return 0, err
		}
		sel, err := pipeline.SelectionFromState(s, lock)
		if err != nil {
			return 0, err
		}
		if profileName != "" {
			sel.ProfileName = profileName
		}
		result, err := pipeline.Install(homeDir, registry, sel, version, false)
		return len(result.FilesChanged), err
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
