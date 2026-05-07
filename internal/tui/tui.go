// Package tui implements the interactive terminal installer for cortex-ia.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
					// Delete the target profile (set when dialog was opened)
					target := m.ProfileDeleteTarget
					m.ProfileDeleteTarget = ""
					for i, p := range m.Profiles {
						if p.Name == target {
							old := m.Profiles
							m.Profiles = append(m.Profiles[:i], m.Profiles[i+1:]...)
							m.saveProfilesToDisk()
							if m.ProfileErr != nil {
								m.Profiles = old // rollback on save failure
							} else {
								// Clear SelectedProfile if we just deleted it
								if m.SelectedProfile == target {
									m.SelectedProfile = ""
								}
								// Clamp cursor to new bounds
								if m.Cursor >= len(m.Profiles) {
									m.Cursor = max(len(m.Profiles)-1, 0)
								}
							}
							break
						}
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
				m.Screen != ScreenProfileCreate &&
				!m.AgentFilter.Active && !m.SkillFilter.Active && !m.OCModelFilter.Active {
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
		case "ctrl+b":
			// Quick jump to backups
			if m.Screen != ScreenWelcome && m.Screen != ScreenInstalling &&
				!m.PipelineRunning && !m.OperationRunning {
				if m.ListBackupsFn != nil {
					m.Backups, m.BackupWarnings = m.ListBackupsFn()
				}
				m.BackupFilter.Deactivate()
				m.setScreen(ScreenBackups)
				return m, nil
			}
		case "alt+m":
			// Quick jump to maintenance (alt+m — ctrl+m collides with Enter in terminals)
			if m.Screen != ScreenWelcome && m.Screen != ScreenInstalling &&
				!m.PipelineRunning && !m.OperationRunning {
				m.loadProfilesFromDisk()
				m.setScreen(ScreenMaintenance)
				return m, nil
			}
		case "t":
			// Toggle theme on non-input screens
			if m.Screen != ScreenAgentBuilderPrompt &&
				m.Screen != ScreenRenameBackup &&
				m.Screen != ScreenProfileCreate &&
				!m.AgentFilter.Active && !m.SkillFilter.Active && !m.OCModelFilter.Active {
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
			m.ActiveToast = Toast{Text: fmt.Sprintf("Restore failed: %v — press r to retry", msg.Err), IsError: true, Visible: true}
		} else {
			m.ActiveToast = Toast{Text: "Restore complete", Visible: true}
		}
		return m, dismissToastAfter(5 * time.Second)

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
		if msg.Err != nil {
			m.ActiveToast = Toast{Text: "Sync failed — press Enter to retry", IsError: true, Visible: true}
			return m, dismissToastAfter(5 * time.Second)
		}
		return m, nil

	case UpgradeDoneMsg:
		m.UpgradeErr = msg.Err
		m.OperationRunning = false
		return m, nil

	case ToastDismissMsg:
		m.ActiveToast.Visible = false
		m.Toasts.Dismiss()
		return m, nil

	case RepairDoneMsg:
		m.OperationRunning = false
		if msg.Err != nil {
			m.ActiveToast = Toast{Text: fmt.Sprintf("Repair failed: %v", msg.Err), IsError: true, Visible: true}
		} else {
			m.ActiveToast = Toast{Text: fmt.Sprintf("Repair complete: %d files updated", len(msg.Result.FilesChanged)), Visible: true}
		}
		return m, dismissToastAfter(4 * time.Second)

	case DryRunDoneMsg:
		m.OperationRunning = false
		if msg.Err != nil {
			m.ActiveToast = Toast{Text: fmt.Sprintf("Dry-run failed: %v", msg.Err), IsError: true, Visible: true}
		} else {
			m.DryRunResult = &msg.Result
			m.ActiveToast = Toast{Text: fmt.Sprintf("Preview: %d components, %d files would change", len(msg.Result.ComponentsDone), len(msg.Result.FilesChanged)), Visible: true}
		}
		return m, dismissToastAfter(5 * time.Second)

	case UpdateCheckResultMsg:
		m.UpdateResults = msg.Results
		m.UpdateCheckDone = true
		m.OperationRunning = false
		// If chain mode (Upgrade+Sync), auto-run sync after check completes
		if m.UpgradeSyncChain && m.SyncFn != nil {
			m.UpgradeSyncChain = false
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
	case ScreenClaudeModelPicker:
		return m.updateClaudeModelPicker(msg)
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
	case ScreenRenameBackup:
		return m.updateRenameBackup(msg)
	case ScreenMaintenance:
		return m.updateMaintenance(msg)
	case ScreenProfileCreate:
		return m.updateProfileCreate(msg)
	case ScreenAgentBuilderEngine:
		return m.updateAgentBuilderEngine(msg)
	case ScreenAgentBuilderPrompt:
		return m.updateAgentBuilderPrompt(msg)
	case ScreenAgentBuilderSDD:
		return m.updateAgentBuilderSDD(msg)
	case ScreenAgentBuilderGenerating:
		return m.updateAgentBuilderGenerating(msg)
	case ScreenAgentBuilderPreview:
		return m.updateAgentBuilderPreview(msg)
	case ScreenAgentBuilderComplete:
		return m.updateAgentBuilderComplete(msg)
	case ScreenModelConfig:
		return m.updateModelConfig(msg)
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
	case ScreenClaudeModelPicker:
		content = m.viewClaudeModelPicker()
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
	case ScreenRenameBackup:
		content = m.viewRenameBackup()
	case ScreenMaintenance:
		content = m.viewMaintenance()
	case ScreenProfileCreate:
		content = m.viewProfileCreate()
	case ScreenAgentBuilderEngine:
		content = m.viewAgentBuilderEngine()
	case ScreenAgentBuilderPrompt:
		content = m.viewAgentBuilderPrompt()
	case ScreenAgentBuilderSDD:
		content = m.viewAgentBuilderSDD()
	case ScreenAgentBuilderGenerating:
		content = m.viewAgentBuilderGenerating()
	case ScreenAgentBuilderPreview:
		content = m.viewAgentBuilderPreview()
	case ScreenAgentBuilderComplete:
		content = m.viewAgentBuilderComplete()
	case ScreenModelConfig:
		content = m.viewModelConfig()
	case ScreenOpenCodeModelPicker:
		content = m.viewOpenCodeModelPicker()
	}

	// Add breadcrumb, validation error, and help
	bc := renderBreadcrumb(m.Screen)
	helpView := m.Help.View(m.screenKeyMap())
	statusBar := renderStatusBar(m)

	// Inline validation error
	validationLine := ""
	if m.ValidationErr != "" {
		validationLine = "\n" + styles.StatusFail.Render("⚠ "+m.ValidationErr) + "\n"
	}

	var view string
	if bc != "" {
		view = bc + "\n\n" + content + validationLine + "\n" + helpView
	} else {
		view = content + validationLine + "\n" + helpView
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

	// Render toast notifications (queue + legacy single toast)
	if m.Toasts.HasVisible() {
		toastLines := renderToastQueue(m.Toasts, m.Width)
		if toastLines != "" {
			view = toastLines + "\n" + view
		}
	} else if m.ActiveToast.Visible {
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
		case "esc":
			m.Quitting = true
			return m, tea.Quit
		case "enter":
			if m.Cursor < len(opts) {
				switch opts[m.Cursor] {
				case WelcomeInstall:
					m.RunDetection()
					m.setScreen(ScreenDetection)
				case WelcomeMaintenance:
					m.loadProfilesFromDisk()
					m.SyncErr = nil
					m.SyncFilesChanged = 0
					m.UpgradeErr = nil
					m.ProfileErr = nil
					m.setScreen(ScreenMaintenance)
				case WelcomeModelConfig:
					m.ClaudeModelCursor = 0
					m.ModelConfigTab = ModelConfigTabClaude
					m.setScreen(ScreenModelConfig)
				case WelcomeAgentBuilder:
					m.resetAgentBuilder()
					m.setScreen(ScreenAgentBuilderEngine)
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

	// Add contextual badges to menu items
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
				if opt == WelcomeMaintenance {
					labels[i] += " (updates available)"
				}
			}
		}
	}
	// Show current model preset on Configure models
	if m.ModelPreset != "" {
		for i, opt := range opts {
			if opt == WelcomeModelConfig {
				labels[i] += " (" + string(m.ModelPreset) + ")"
			}
		}
	}

	return screens.RenderWelcome(screens.WelcomeData{
		Version:  m.Version,
		Options:  labels,
		Cursor:   m.Cursor,
		FirstRun: m.FirstRun,
	})
}

// --- Detection screen ---

func (m Model) updateDetection(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.QuickInstall = false
			m.setScreen(ScreenAgents)
		case "f":
			// Quick install: skip to agents with fast flag
			m.QuickInstall = true
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
	content := screens.RenderDetection(screens.DetectionData{
		OS: si.OS, Arch: si.Arch, PkgMgr: si.PkgMgr, Shell: si.Shell,
		NodeVer: si.NodeVer, GitVer: si.GitVer, GoVer: si.GoVer,
		Npx: si.Npx, Cortex: si.Cortex, DetectedAgents: si.DetectedAgents,
	})
	content += "\n" + styles.Description.Render("Enter customize • f quick install (recommended defaults)")
	return content
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
		m.ValidationErr = "" // clear inline error on any input
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
				if m.QuickInstall {
					m.applyQuickDefaults()
					m.setScreen(ScreenReview)
				} else {
					m.setScreen(ScreenPersona)
				}
			} else {
				m.ValidationErr = "Select at least one agent to continue"
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
	hint := m.AgentFilter.Hint()
	if hint != "" {
		return content + "\n" + hint
	}
	return content
}

// --- Persona screen ---

func (m Model) updatePersona(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Auto-skip if only one persona available
	if len(m.Personas) == 1 {
		m.Persona = m.Personas[0]
		components := catalog.ComponentsForPreset(m.Preset)
		m.Resolved = catalog.ResolveDeps(components)
		m.setScreen(ScreenClaudeModelPicker)
		return m, nil
	}

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
			components := catalog.ComponentsForPreset(m.Preset)
			m.Resolved = catalog.ResolveDeps(components)
			m.setScreen(ScreenClaudeModelPicker)
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

// --- Review screen ---

// ReviewCursor tracks which item has focus on the review screen.
const (
	reviewCursorPreset = iota
	reviewCursorSDD
	reviewCursorTDD
	reviewCursorConfirm
)

func (m Model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < reviewCursorConfirm {
				m.Cursor++
			}
		case " ":
			// Toggle based on cursor
			switch m.Cursor {
			case reviewCursorPreset:
				// Toggle between Full and Minimal preset
				if m.Preset == model.PresetFull {
					m.Preset = model.PresetMinimal
				} else {
					m.Preset = model.PresetFull
				}
				// Re-resolve components for the new preset
				components := catalog.ComponentsForPreset(m.Preset)
				m.Resolved = catalog.ResolveDeps(components)
			case reviewCursorSDD:
				m.SDDEnabled = !m.SDDEnabled
			case reviewCursorTDD:
				m.StrictTDDEnabled = !m.StrictTDDEnabled
			}
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
		case "e":
			// Export config
			if err := m.exportConfig(); err != nil {
				m.ActiveToast = Toast{Text: fmt.Sprintf("Export failed: %v", err), IsError: true, Visible: true}
			} else {
				m.ConfigExported = true
				m.ActiveToast = Toast{Text: "Config exported to ~/.cortex-ia/export-config.json", Visible: true}
			}
			return m, dismissToastAfter(4 * time.Second)
		case "d":
			// Dry-run preview
			if !m.OperationRunning {
				m.OperationRunning = true
				m.DryRunResult = nil
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
				sel := model.Selection{
					Agents:           m.SelectedAgentIDs(),
					Preset:           m.Preset,
					Persona:          m.Persona,
					Components:       components,
					ModelAssignments: m.ModelAssignments,
					StrictTDD:        m.StrictTDDEnabled,
					CommunitySkills:  m.SkillSelection,
				}
				return m, func() tea.Msg {
					result, err := pipeline.Install(m.HomeDir, m.Registry, sel, m.Version, true)
					return DryRunDoneMsg{Result: result, Err: err}
				}
			}
		case "c":
			// Customize: go to full flow from Persona
			if m.QuickInstall {
				m.QuickInstall = false
				m.setScreen(ScreenPersona)
				return m, nil
			}
		case "esc", "n":
			if prev, ok := PreviousScreen(ScreenReview); ok {
				m.setScreen(prev)
			}
		}
	}
	return m, nil
}

func (m Model) viewReview() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Review Installation"))
	sb.WriteString("\n\n")

	// Agents
	sb.WriteString(styles.Subtitle.Render("Agents:"))
	sb.WriteString("\n")
	for _, a := range m.Agents {
		if a.Selected {
			fmt.Fprintf(&sb, "  %s %s\n", styles.StatusOK.Render("●"), a.Name)
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Persona: %s", m.Persona)))
	sb.WriteString("  ")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Model: %s", m.ModelPreset)))
	sb.WriteString("\n\n")

	// Preset toggle (Full / Minimal)
	presetCursor := "  "
	if m.Cursor == reviewCursorPreset {
		presetCursor = styles.Cursor.Render("> ")
	}
	presetLabel := "Full"
	presetDesc := "All 8 components (Cortex, ForgeSpec, SDD, Mailbox, Context7, Conventions, GGA, Skills)"
	if m.Preset == model.PresetMinimal {
		presetLabel = "Minimal"
		presetDesc = "Cortex + ForgeSpec + Context7 + SDD only"
	}
	fmt.Fprintf(&sb, "%s%s Preset: %s %s\n", presetCursor,
		styles.Selected.Render("●"),
		styles.Subtitle.Render(presetLabel),
		styles.Description.Render("— "+presetDesc))

	// SDD toggle
	sddCursor := "  "
	if m.Cursor == reviewCursorSDD {
		sddCursor = styles.Cursor.Render("> ")
	}
	sddCheck := "○"
	if m.SDDEnabled {
		sddCheck = styles.Selected.Render("●")
	}
	fmt.Fprintf(&sb, "%s%s SDD Integration %s\n", sddCursor, sddCheck,
		styles.Description.Render("— 9-phase structured dev workflow"))

	// TDD toggle
	tddCursor := "  "
	if m.Cursor == reviewCursorTDD {
		tddCursor = styles.Cursor.Render("> ")
	}
	tddCheck := "○"
	if m.StrictTDDEnabled {
		tddCheck = styles.Selected.Render("●")
	}
	fmt.Fprintf(&sb, "%s%s Strict TDD %s\n", tddCursor, tddCheck,
		styles.Description.Render("— Tests required before implementation"))

	sb.WriteString("\n")

	// Components (dependency tree)
	sb.WriteString(styles.Subtitle.Render("Components:"))
	sb.WriteString("\n")
	cmap := catalog.ComponentMap() // static catalog, cheap to build
	for i, id := range m.Resolved {
		prefix := "├── "
		if i == len(m.Resolved)-1 {
			prefix = "└── "
		}
		desc := ""
		if info, ok := cmap[id]; ok {
			desc = " " + styles.Description.Render(info.Description)
		}
		fmt.Fprintf(&sb, "  %s%s%s\n", styles.Description.Render(prefix), styles.Selected.Render(string(id)), desc)
	}

	sb.WriteString("\n")
	confirmCursor := "  "
	if m.Cursor == reviewCursorConfirm {
		confirmCursor = styles.Cursor.Render("> ")
	}
	sb.WriteString(confirmCursor + styles.Subtitle.Render("Confirm & Install"))
	sb.WriteString("\n\n")
	// Show dry-run results if available
	if m.DryRunResult != nil {
		sb.WriteString("\n")
		sb.WriteString(styles.Subtitle.Render("Dry-run preview:"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "  Components: %d  Files: %d\n",
			len(m.DryRunResult.ComponentsDone), len(m.DryRunResult.FilesChanged))
		for _, f := range m.DryRunResult.FilesChanged {
			fmt.Fprintf(&sb, "  %s %s\n", styles.StatusOK.Render("~"), styles.Description.Render(f))
		}
	}

	hints := "Space toggle • d dry-run • Enter confirm • Esc back"
	if m.QuickInstall {
		hints = "Space toggle • d dry-run • Enter confirm • c customize • Esc back"
	}
	sb.WriteString(styles.Description.Render(hints))

	return sb.String()
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
		case "u":
			// Undo: restore from the backup created during this install
			if m.Result.BackupID != "" && m.RestoreFn != nil && m.ListBackupsFn != nil {
				backups, _ := m.ListBackupsFn()
				for _, bk := range backups {
					if bk.ID == m.Result.BackupID {
						m.OperationRunning = true
						manifest := bk
						return m, func() tea.Msg {
							err := m.RestoreFn(manifest)
							return BackupRestoreMsg{Err: err}
						}
					}
				}
			}
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

	// Inject RepairFn — wraps pipeline.Repair
	m.RepairFn = func() (pipeline.InstallResult, error) {
		return pipeline.Repair(homeDir, registry, version, false)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
