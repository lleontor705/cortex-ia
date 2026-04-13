package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/agentbuilder"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/system"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
	"github.com/lleontor705/cortex-ia/internal/update"
)

// Screen identifies the current TUI screen.
type Screen int

const (
	ScreenUnknown Screen = iota
	ScreenWelcome
	ScreenDetection
	ScreenAgents
	ScreenPersona
	ScreenPreset
	ScreenClaudeModelPicker
	ScreenSDDMode
	ScreenStrictTDD
	ScreenDependencyTree
	ScreenSkillPicker
	ScreenReview
	ScreenInstalling
	ScreenComplete
	ScreenBackups
	ScreenRestoreConfirm
	ScreenRestoreResult
	ScreenDeleteConfirm
	ScreenDeleteResult
	ScreenRenameBackup
	ScreenUpgrade
	ScreenSync
	ScreenUpgradeSync
	ScreenModelConfig
	ScreenProfiles
	ScreenProfileCreate
	ScreenProfileDelete
	ScreenAgentBuilderEngine
	ScreenAgentBuilderPrompt
	ScreenAgentBuilderSDD
	ScreenAgentBuilderSDDPhase
	ScreenAgentBuilderGenerating
	ScreenAgentBuilderPreview
	ScreenAgentBuilderInstalling
	ScreenAgentBuilderComplete
	ScreenOpenCodeModels
	ScreenOpenCodeProviderPicker
	ScreenOpenCodeModelPicker
)

// --- Message types ---

// StepProgressMsg is sent when a pipeline step changes status.
type StepProgressMsg struct {
	StepID string
	Status string
	Err    error
}

// PipelineDoneMsg is sent when the installation pipeline finishes.
type PipelineDoneMsg struct {
	Result pipeline.InstallResult
	Err    error
}

// BackupRestoreMsg is sent when a backup restore completes.
type BackupRestoreMsg struct {
	Err error
}

// BackupDeleteMsg is sent when a backup deletion completes.
type BackupDeleteMsg struct {
	Err error
}

// UpdateCheckResultMsg is sent when the background update check completes.
type UpdateCheckResultMsg struct {
	Results []update.CheckResult
}

// UpgradeDoneMsg is sent when the upgrade operation completes.
type UpgradeDoneMsg struct {
	Err error
}

// SyncDoneMsg is sent when the sync operation completes.
type SyncDoneMsg struct {
	FilesChanged int
	Err          error
}

// AgentBuilderGeneratedMsg is sent when agent generation completes.
type AgentBuilderGeneratedMsg struct {
	Err error
}

// AgentBuilderInstallDoneMsg is sent when agent installation completes.
type AgentBuilderInstallDoneMsg struct {
	Err error
}

// --- Function type signatures for dependency injection ---

// ExecuteFunc runs the installation pipeline.
type ExecuteFunc func(
	selection model.Selection,
	onProgress pipeline.ProgressFunc,
) pipeline.InstallResult

// RestoreFunc restores a backup from a manifest.
type RestoreFunc func(manifest backup.Manifest) error

// DeleteBackupFunc deletes an entire backup directory.
type DeleteBackupFunc func(manifest backup.Manifest) error

// RenameBackupFunc renames a backup's description.
type RenameBackupFunc func(manifest backup.Manifest, newDescription string) error

// ListBackupsFn returns the current list of available backups and any warnings.
type ListBackupsFn func() ([]backup.Manifest, []string)

// UpgradeFunc performs tool upgrades.
type UpgradeFunc func(results []update.CheckResult) error

// SyncFunc syncs managed configuration files.
// profileName is the name of a saved profile whose model assignments to apply.
type SyncFunc func(profileName string) (int, error)

// --- Agent item ---

// AgentItem represents a detected agent in the selection list.
type AgentItem struct {
	ID       model.AgentID
	Name     string
	Binary   string
	Selected bool
}

// SkillItem represents a community skill in the picker.
type SkillItem struct {
	Name     string
	Selected bool
}

// --- Welcome menu ---

// WelcomeOption identifies an option on the welcome menu.
type WelcomeOption int

const (
	WelcomeInstall WelcomeOption = iota
	WelcomeUpgrade
	WelcomeSync
	WelcomeUpgradeSync
	WelcomeModelConfig
	WelcomeProfiles
	WelcomeAgentBuilder
	WelcomeBackups
	WelcomeOpenCodeModels
	WelcomeQuit
)

// welcomeOptions returns the ordered list of welcome menu items.
func welcomeOptions() []WelcomeOption {
	return []WelcomeOption{
		WelcomeInstall,
		WelcomeUpgrade,
		WelcomeSync,
		WelcomeUpgradeSync,
		WelcomeModelConfig,
		WelcomeProfiles,
		WelcomeAgentBuilder,
		WelcomeBackups,
		WelcomeOpenCodeModels,
		WelcomeQuit,
	}
}

// welcomeLabel returns the display label for a welcome option.
func welcomeLabel(opt WelcomeOption) string {
	switch opt {
	case WelcomeInstall:
		return "Install ecosystem"
	case WelcomeUpgrade:
		return "Upgrade tools"
	case WelcomeSync:
		return "Sync configs"
	case WelcomeUpgradeSync:
		return "Upgrade + Sync"
	case WelcomeModelConfig:
		return "Configure models"
	case WelcomeAgentBuilder:
		return "Create your own Agent"
	case WelcomeProfiles:
		return "Manage profiles"
	case WelcomeBackups:
		return "Manage backups"
	case WelcomeOpenCodeModels:
		return "OpenCode models"
	case WelcomeQuit:
		return "Quit"
	}
	return ""
}

// --- System info cache ---

// SysInfoCache holds cached system detection results.
type SysInfoCache struct {
	OS, Arch, PkgMgr, Shell    string
	NodeVer, GitVer, GoVer     string
	Npx, Cortex                bool
	DetectedAgents             int
}

// --- Model ---

// Model is the root Bubbletea model for the TUI installer.
type Model struct {
	Screen         Screen
	PreviousScreen Screen
	Width          int
	Height         int
	Cursor         int
	Version        string

	// Bubbles components
	Spinner      spinner.Model
	ProgressBar  progress.Model
	Help         help.Model
	Keys         KeyMap

	// Dialog overlay
	ActiveDialog Dialog

	// Toast notification
	ActiveToast Toast

	// Screen transition animation
	ScreenTransition Transition

	// Filters
	AgentFilter FilterInput
	SkillFilter FilterInput

	// Core state
	Registry *agents.Registry
	HomeDir  string
	Agents   []AgentItem
	Preset   model.PresetID
	Presets  []model.PresetID
	Personas []model.PersonaID
	Persona  model.PersonaID
	Resolved []model.ComponentID
	SysInfo  *SysInfoCache

	// Model selection
	ModelPreset        model.ModelPreset
	ModelAssignments   model.ModelAssignments
	ClaudeModelCursor  int
	SDDEnabled         bool
	StrictTDDEnabled   bool
	SkillSelection     []model.SkillID
	AvailableSkills    []SkillItem
	SkillCursor        int

	// Installation
	Progress    ProgressState
	Result      pipeline.InstallResult
	InstallErr  error

	// Backups
	Backups          []backup.Manifest
	BackupWarnings   []string
	SelectedBackup   backup.Manifest
	BackupScroll     int
	BackupRenameInput textinput.Model
	RestoreErr       error
	DeleteErr        error
	RenameErr        error

	// Post-install operations
	UpdateResults    []update.CheckResult
	UpdateCheckDone  bool
	SyncFilesChanged int
	SyncErr          error
	UpgradeErr       error
	UpgradeSyncPhase string // "", "checking", "syncing", "done"

	// Model config mode
	ModelConfigMode bool

	// Profiles
	Profiles         []model.Profile
	SelectedProfile  string
	ProfileInput     textinput.Model
	ProfileErr       error

	// Agent builder
	AgentBuilderEngine    model.AgentID
	AgentBuilderTextArea  textarea.Model
	AgentBuilderSDDMode   string
	AgentBuilderSDDPhase  string
	AgentBuilderErr       error
	AgentBuilderViewport  viewport.Model
	AgentBuilderGenerated *agentbuilder.GeneratedAgent

	// OpenCode model configuration
	OpenCodeAssignments model.OpenCodeModelAssignments
	OpenCodeProviders   []model.OpenCodeProvider
	OCModelCursor       int
	OCProviderCursor    int
	OCSelectedAgent     string
	OCErr               error

	// Pipeline tracking
	PipelineRunning  bool
	OperationRunning bool
	progressCh       chan StepProgressMsg

	// Dependency injection
	ExecuteFn      ExecuteFunc
	RestoreFn      RestoreFunc
	DeleteBackupFn DeleteBackupFunc
	RenameBackupFn RenameBackupFunc
	ListBackupsFn  ListBackupsFn
	UpgradeFn      UpgradeFunc
	SyncFn         SyncFunc

	Quitting bool
}

// newTextInput creates a styled textinput.Model with sensible defaults.
func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Primary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(styles.White)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(styles.Secondary)
	return ti
}

// newTextArea creates a styled textarea.Model with sensible defaults.
func newTextArea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.SetWidth(60)
	ta.SetHeight(5)
	return ta
}

// New creates a new TUI model with default values.
func New(registry *agents.Registry, homeDir, version string) Model {
	// Spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Secondary)

	// Progress bar
	pb := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	// Help
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(styles.Muted)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(styles.Muted)

	return Model{
		Screen:            ScreenWelcome,
		Registry:          registry,
		HomeDir:           homeDir,
		Version:           version,
		Presets:           []model.PresetID{model.PresetFull, model.PresetMinimal},
		Personas:          []model.PersonaID{model.PersonaProfessional, model.PersonaMentor, model.PersonaMinimal},
		Persona:           model.PersonaProfessional,
		SDDEnabled:        true,
		Spinner:           sp,
		ProgressBar:       pb,
		Help:              h,
		Keys:              DefaultKeyMap(),
		AgentFilter:          NewFilterInput(),
		SkillFilter:          NewFilterInput(),
		BackupRenameInput:    newTextInput("Enter description..."),
		ProfileInput:         newTextInput("Enter profile name..."),
		AgentBuilderTextArea: newTextArea("Describe what this agent should do..."),
		AgentBuilderViewport: viewport.New(70, 20),
	}
}

// resetAgentBuilder clears all agent builder state for a fresh flow.
func (m *Model) resetAgentBuilder() {
	m.AgentBuilderEngine = ""
	m.AgentBuilderTextArea.Reset()
	m.AgentBuilderSDDMode = ""
	m.AgentBuilderSDDPhase = ""
	m.AgentBuilderErr = nil
	m.AgentBuilderGenerated = nil
	m.AgentBuilderViewport.SetContent("")
	m.AgentBuilderViewport.GotoTop()
}

// setScreen transitions to a new screen, saving the previous screen for back navigation.
func (m *Model) setScreen(s Screen) {
	m.PreviousScreen = m.Screen
	m.Screen = s
	m.Cursor = 0
	m.ScreenTransition, _ = startTransition()
}

// RunDetection performs system detection and agent discovery.
func (m *Model) RunDetection() {
	info := system.Detect()
	m.DetectAgents()
	m.SysInfo = &SysInfoCache{
		OS: info.OS, Arch: info.Arch,
		PkgMgr: info.Profile.PackageManager, Shell: info.Tools.Shell,
		NodeVer: info.Tools.NodeVersion, GitVer: info.Tools.GitVersion,
		GoVer: info.Tools.GoVersion, Npx: info.Tools.NpxAvailable,
		Cortex: info.Tools.CortexFound, DetectedAgents: len(m.Agents),
	}
}

// DetectAgents populates the agent list from the registry.
func (m *Model) DetectAgents() {
	m.Agents = nil
	for _, adapter := range m.Registry.All() {
		installed, binary, _, _, _ := adapter.Detect(m.HomeDir)
		if installed {
			m.Agents = append(m.Agents, AgentItem{
				ID:       adapter.Agent(),
				Name:     string(adapter.Agent()),
				Binary:   binary,
				Selected: true,
			})
		}
	}
}

// SelectedAgentIDs returns the IDs of all selected agents.
func (m Model) SelectedAgentIDs() []model.AgentID {
	var ids []model.AgentID
	for _, a := range m.Agents {
		if a.Selected {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// HasSelectedAgents returns true if at least one agent is selected.
func (m Model) HasSelectedAgents() bool {
	for _, a := range m.Agents {
		if a.Selected {
			return true
		}
	}
	return false
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			result := update.Check(m.Version)
			return UpdateCheckResultMsg{Results: []update.CheckResult{result}}
		},
	)
}
