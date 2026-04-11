// Package tui implements the interactive terminal installer for cortex-ia.
package tui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/screens"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
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
		}

	case TickMsg:
		m.SpinnerFrame++
		if m.PipelineRunning || m.OperationRunning {
			return m, tickCmd()
		}
		return m, nil

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
		m.setScreen(ScreenRestoreResult)
		if m.ListBackupsFn != nil {
			m.Backups, m.BackupWarnings = m.ListBackupsFn()
		}
		return m, nil

	case BackupDeleteMsg:
		m.DeleteErr = msg.Err
		m.OperationRunning = false
		m.setScreen(ScreenDeleteResult)
		if m.ListBackupsFn != nil {
			m.Backups, m.BackupWarnings = m.ListBackupsFn()
		}
		return m, nil

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

	case UpdateCheckResultMsg:
		m.UpdateResults = msg.Results
		m.UpdateCheckDone = true
		m.OperationRunning = false
		// If on UpgradeSync screen, auto-start sync phase
		if m.Screen == ScreenUpgradeSync && m.UpgradeSyncPhase == "checking" {
			m.UpgradeSyncPhase = "syncing"
			m.OperationRunning = true
			profileName := m.SelectedProfile
			return m, tea.Batch(func() tea.Msg {
				changed, err := m.SyncFn(profileName)
				return SyncDoneMsg{FilesChanged: changed, Err: err}
			}, tickCmd())
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
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.Quitting {
		return ""
	}
	switch m.Screen {
	case ScreenWelcome:
		return m.viewWelcome()
	case ScreenDetection:
		return m.viewDetection()
	case ScreenAgents:
		return m.viewAgents()
	case ScreenPersona:
		return m.viewPersona()
	case ScreenPreset:
		return m.viewPreset()
	case ScreenClaudeModelPicker:
		return m.viewClaudeModelPicker()
	case ScreenSDDMode:
		return m.viewSDDMode()
	case ScreenStrictTDD:
		return m.viewStrictTDD()
	case ScreenDependencyTree:
		return m.viewDependencyTree()
	case ScreenSkillPicker:
		return m.viewSkillPicker()
	case ScreenReview:
		return m.viewReview()
	case ScreenInstalling:
		return m.viewInstalling()
	case ScreenComplete:
		return m.viewComplete()
	case ScreenBackups:
		return m.viewBackups()
	case ScreenRestoreConfirm:
		return m.viewRestoreConfirm()
	case ScreenRestoreResult:
		return m.viewRestoreResult()
	case ScreenDeleteConfirm:
		return m.viewDeleteConfirm()
	case ScreenDeleteResult:
		return m.viewDeleteResult()
	case ScreenRenameBackup:
		return m.viewRenameBackup()
	case ScreenUpgrade:
		return m.viewUpgrade()
	case ScreenSync:
		return m.viewSync()
	case ScreenUpgradeSync:
		return m.viewUpgradeSync()
	case ScreenModelConfig:
		return m.viewModelConfig()
	case ScreenProfiles:
		return m.viewProfiles()
	case ScreenProfileCreate:
		return m.viewProfileCreate()
	case ScreenProfileDelete:
		return m.viewProfileDelete()
	case ScreenAgentBuilderEngine:
		return m.viewAgentBuilderEngine()
	case ScreenAgentBuilderPrompt:
		return m.viewAgentBuilderPrompt()
	case ScreenAgentBuilderSDD:
		return m.viewAgentBuilderSDD()
	case ScreenAgentBuilderSDDPhase:
		return m.viewAgentBuilderSDDPhase()
	case ScreenAgentBuilderGenerating:
		return m.viewAgentBuilderGenerating()
	case ScreenAgentBuilderPreview:
		return m.viewAgentBuilderPreview()
	case ScreenAgentBuilderInstalling:
		return m.viewAgentBuilderInstalling()
	case ScreenAgentBuilderComplete:
		return m.viewAgentBuilderComplete()
	}
	return ""
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
					m.setScreen(ScreenClaudeModelPicker)
				case WelcomeProfiles:
					m.loadProfilesFromDisk()
					m.setScreen(ScreenProfiles)
				case WelcomeAgentBuilder:
					m.setScreen(ScreenAgentBuilderEngine)
				case WelcomeBackups:
					if m.ListBackupsFn != nil {
						m.Backups, m.BackupWarnings = m.ListBackupsFn()
					}
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
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Agents)-1 {
				m.Cursor++
			}
		case " ":
			if m.Cursor < len(m.Agents) {
				m.Agents[m.Cursor].Selected = !m.Agents[m.Cursor].Selected
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
		case "enter":
			if m.HasSelectedAgents() {
				m.setScreen(ScreenPersona)
			}
		case "esc":
			m.setScreen(ScreenDetection)
		}
	}
	return m, nil
}

func (m Model) viewAgents() string {
	data := make([]screens.AgentData, len(m.Agents))
	for i, a := range m.Agents {
		data[i] = screens.AgentData{Name: a.Name, Binary: a.Binary, Selected: a.Selected}
	}
	return screens.RenderAgents(screens.AgentsData{Agents: data, Cursor: m.Cursor})
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
		Presets: m.Presets,
		Cursor:  m.Cursor,
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
			return m, tea.Batch(m.runInstallWithProgress(ch), listenProgress(ch), tickCmd())
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
		}, m.SpinnerFrame)
	}
	return screens.RenderInstalling(m.Progress.ViewModel(), m.SpinnerFrame)
}

// --- Complete screen ---

func (m Model) updateComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "q", "esc":
			m.Quitting = true
			return m, tea.Quit
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

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
