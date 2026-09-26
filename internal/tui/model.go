package tui

import (
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// screen enumerates the conceptual screens of the TUI. Confirmation is
// an overlay state on the current screen, not a separate screen.
type screen int

const (
	screenHome screen = iota
	screenReview
	screenRunning
	screenResult
	screenMCP
	screenWeb
	screenAgentStudio
	screenStats
	screenModels
	screenProviders
)

// productionBootScreen is the surface shipped to users: launched with zero
// arguments the TUI opens the usage stats panel from the first frame
// (REQ-US-002).
const productionBootScreen = screenStats

// bootScreen is the effective boot surface consulted by newModel. The legacy
// Home-first navigation suite pins it to screenHome from its own test setup;
// boot oracles exercise production by resetting it to productionBootScreen.
var bootScreen = productionBootScreen

// confirmKind identifies which destructive intent a confirmation modal guards.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmOverwrite
	confirmUninstall
	confirmMCPRemove
	confirmRollback
	confirmCortexInstall
)

// homeEntries are the fixed Home menu actions, in display order.
var homeEntries = []string{
	"Install / Sync",
	"Manage MCPs",
	"CortexIA Web Console",
	"Agent Studio (Create Sub-agent)",
	"Estadísticas de uso",
	"Doctor / Recovery",
	"Uninstall",
	"Quit",
	"Configuración de modelos",
	"Install custom provider",
}

// statsEntryIndex is the Home cursor position of the usage stats entry.
const statsEntryIndex = 4

// providersEntryIndex is the Home cursor position of the custom providers
// entry. Its digit key is intentionally unbound: digits 1-9 stay frozen on
// entries 0-8 and the entry opens through "p"/"P" or Enter (REQ-PROV-006).
const providersEntryIndex = 9

// managedNames lists the managed MCP presets in toggle order.
var managedNames = []string{"cortex", "context7"}

// themeRowIndex is the Review toggle row for the opt-in cortex theme; it
// follows the managed MCP rows so the cursor range is one past them.
var themeRowIndex = len(managedNames)

// confirmState is the active confirmation overlay. arg carries the MCP name
// or backup ID the confirmed action applies to.
type confirmState struct {
	kind confirmKind
	arg  string
}

// runningState drives the phase display while one operation executes.
type runningState struct {
	title         string
	phases        []string
	current       int
	spinner       int
	finished      bool
	startedAt     time.Time
	progressModel progress.Model
	spinnerModel  spinner.Model
}

// opResult is the typed projection of one completed operation. The Result
// screen renders exactly this; PASS means the service reported success.
type opResult struct {
	title       string
	pass        bool
	changed     int
	backupID    string
	rollbackCmd string
	canRollback bool
	detail      []string
}

// model is the whole TUI state.
type model struct {
	svc     ServiceAPI
	homeDir string
	version string

	screen    screen
	width     int
	height    int
	cursor    int // Home menu index or MCP row index
	mcpCursor int // Review MCP toggle index

	// Review state.
	installMode  string          // "install" or "sync", derived from the plan
	opts         install.Options // current selection for install/sync
	plan         *pipeline.Plan  // latest read-only plan
	planErr      error
	reviewStatus string // transient feedback when Review input cannot proceed
	overwrite    bool   // explicit overwrite authorization
	hadConflict  bool   // the initial plan carried conflicts
	replanning   bool
	// cortexPrompted records that the missing-cortex consent was already
	// offered for this Review entry, so replans never re-prompt.
	cortexPrompted bool
	// cortexInstalling is true while the automatic install runs.
	cortexInstalling bool
	// cortexStatus is the transient feedback line for the cortex preflight.
	cortexStatus string

	// MCP Manager state.
	mcpReport *install.MCPListReport
	mcpErr    error

	// Delegation and Wizard state.
	delegationCfg delegation.DelegationConfig
	wizardCursor  int

	// Confirmation overlay (valid on any screen).
	confirm confirmState

	running runningState
	result  opResult
	// Vertical scroll offsets for height-clamped screens. Review scrolls
	// with pgup/pgdown; Result with up/down. They reset whenever the
	// underlying content changes (new plan, new result).
	reviewScroll int
	resultScroll int
	quitting     bool

	// Agent Studio state
	studioStep      int
	studioArchIdx   int
	studioResultMsg string

	// Web Console state
	webReady    bool
	webURL      string
	webErr      error
	webStarting bool

	// Usage stats state
	stats statsState

	// Models configuration state
	models modelsState

	// Custom providers screen state
	providers providersState

	// Animation state
	logoFrame int
}

type webReadyMsg struct {
	url string
}

type webErrMsg struct {
	err error
}

// newModel builds the model bound to a service implementation.
func newModel(svc ServiceAPI, homeDir, version string) model {
	cfg, err := delegation.Load(filepath.Join(homeDir, ".config", "opencode"))
	if err != nil {
		cfg = delegation.NormalConfig()
	}
	m := model{
		svc:           svc,
		homeDir:       homeDir,
		version:       version,
		screen:        bootScreen,
		stats:         newStatsState(),
		models:        newModelsState(homeDir),
		providers:     newProvidersState(homeDir),
		opts:          install.DefaultOptions(),
		delegationCfg: cfg,
	}
	m.opts.DelegationConfig = &m.delegationCfg
	return m
}

func (m model) Init() tea.Cmd {
	if m.screen == screenStats {
		return statsLoadCmd()
	}
	if m.screen == screenModels {
		return modelsLoadCmd(m.homeDir)
	}
	return homeTick()
}

// Update routes one message by screen, with the confirmation overlay taking
// precedence over screen keys.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case homeTickMsg:
		if m.screen == screenHome {
			m.logoFrame = (m.logoFrame + 1) % 120
			return m, homeTick()
		}
		return m, nil
	case tickMsg:
		if m.screen == screenRunning && !m.running.finished {
			m.running.spinner = (m.running.spinner + 1) % len(spinnerFrames)
			return m, spinTick()
		}
		return m, nil
	case planMsg:
		return m.onPlan(msg)
	case cortexInstallMsg:
		return m.onCortexInstall(msg)
	case installMsg:
		return m.onInstallDone(msg)
	case doctorMsg:
		return m.onDoctorDone(msg)
	case uninstallMsg:
		return m.onUninstallDone(msg)
	case rollbackMsg:
		return m.onRollbackDone(msg)
	case mcpListMsg:
		return m.onMCPList(msg)
	case mcpMutateMsg:
		return m.onMCPMutateDone(msg)
	case statsLoadedMsg:
		if m.screen == screenStats {
			m.stats = m.stats.onLoaded(msg)
		}
		return m, nil
	case modelsLoadedMsg:
		if m.screen == screenModels {
			m.models = m.models.onLoaded(msg)
		}
		return m, nil
	case modelsMutatedMsg:
		if m.screen == screenModels {
			m.models = m.models.onMutated(msg)
		}
		return m, nil
	case providersCatalogMsg:
		if m.screen == screenProviders {
			m.providers = m.providers.onLoaded(msg)
		}
		return m, nil
	case providersResultMsg:
		if m.screen == screenProviders {
			m.providers = m.providers.onResult(msg)
		}
		return m, nil
	case webReadyMsg:
		m.webReady = true
		m.webURL = msg.url
		m.webErr = nil
		m.webStarting = false
		return m, nil
	case webErrMsg:
		m.webReady = false
		m.webErr = msg.err
		m.webStarting = false
		return m, nil
	case tea.KeyMsg:
		if m.confirm.kind != confirmNone {
			return m.updateConfirm(msg)
		}
		switch m.screen {
		case screenHome:
			return m.updateHome(msg)
		case screenReview:
			return m.updateReview(msg)
		case screenRunning:
			return m.updateRunning(msg)
		case screenResult:
			return m.updateResult(msg)
		case screenMCP:
			return m.updateMCP(msg)
		case screenWeb:
			return m.updateWeb(msg)
		case screenAgentStudio:
			return m.updateAgentStudio(msg)
		case screenStats:
			return m.updateStats(msg)
		case screenModels:
			return m.updateModels(msg)
		case screenProviders:
			return m.updateProviders(msg)
		}
	}
	return m, nil
}

// --- Home ---

func (m model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "1":
		return m.selectHomeEntry(0)
	case "2":
		return m.selectHomeEntry(1)
	case "3":
		return m.selectHomeEntry(2)
	case "4":
		return m.selectHomeEntry(3)
	case "5":
		return m.selectHomeEntry(4)
	case "6":
		return m.selectHomeEntry(5)
	case "7":
		return m.selectHomeEntry(6)
	case "8":
		return m.selectHomeEntry(7)
	case "9":
		return m.selectHomeEntry(8)
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(homeEntries)-1 {
			m.cursor++
		}
	case "m", "M":
		return m.openModels()
	case "p", "P":
		return m.openProviders()
	case "enter":
		return m.selectHomeEntry(m.cursor)
	}
	return m, nil
}

func (m model) selectHomeEntry(index int) (tea.Model, tea.Cmd) {
	m.cursor = index
	switch index {
	case 0: // Install / Sync → Direct to Review / Plan
		m.screen = screenReview
		m.wizardCursor = 0
		m.opts = install.DefaultOptions()
		m.opts.Version = m.version
		m.opts.DelegationConfig = &m.delegationCfg
		meta := state.LoadMetadataV2(m.homeDir)
		if meta.Presence == state.PresenceV2 {
			m.opts.Cortex = meta.Metadata.Selection.Cortex
			m.opts.Context7 = meta.Metadata.Selection.Context7
		}
		m.overwrite = false
		m.hadConflict = false
		m.installMode = ""
		m.plan = nil
		m.planErr = nil
		m.reviewStatus = ""
		m.replanning = false
		m.cortexPrompted = false
		m.cortexInstalling = false
		m.cortexStatus = ""
		m.mcpCursor = 0
		return m, planCmd(m.svc, m.reviewOptions())
	case 1: // Manage MCPs
		m.screen = screenMCP
		m.mcpReport = nil
		m.mcpErr = nil
		return m, mcpListCmd(m.svc)
	case 2: // CortexIA Web Console
		m.screen = screenWeb
		if m.webReady {
			return m, nil
		}
		m.webStarting = true
		m.webErr = nil
		return m, startWebCmd(m.homeDir)
	case 3: // Agent Studio (Create Sub-agent)
		m.screen = screenAgentStudio
		m.studioStep = 0
		m.studioArchIdx = 0
		m.studioResultMsg = ""
		return m, nil
	case 4: // Estadísticas de uso
		m.screen = screenStats
		m.stats = newStatsState()
		return m, statsLoadCmd()
	case 5: // Doctor / Recovery
		return m.startRunning("Doctor", []string{"Inspect state", "Compare digests", "Assess MCPs", "Report"}, doctorCmd(m.svc))
	case 6: // Uninstall (destructive: explicit confirmation first)
		m.confirm = confirmState{kind: confirmUninstall}
		return m, nil
	case 7: // Quit
		m.quitting = true
		return m, tea.Quit
	case 8: // Configuración de modelos
		return m.openModels()
	case 9: // Install custom provider
		return m.openProviders()
	}
	return m, nil
}

func (m model) updateWeb(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		m.screen = screenHome
		m.cursor = 2
		return m, homeTick()
	case "o", "O", "enter", " ":
		if m.webReady && m.webURL != "" {
			openBrowser(m.webURL)
		}
		return m, nil
	}
	return m, nil
}

// --- Review ---

func (m model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.replanning {
		return m, nil
	}
	switch key := msg.String(); key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = screenHome
		m.cursor = 0
		return m, homeTick()
	case "up", "k":
		if m.mcpCursor > 0 {
			m.mcpCursor--
		}
	case "down", "j":
		if m.mcpCursor < themeRowIndex {
			m.mcpCursor++
		}
	case "pgup":
		if m.reviewScroll > 0 {
			m.reviewScroll--
		}
	case "pgdown":
		m.reviewScroll++ // clamped against the content length at render time
	case " ", "tab":
		switch m.mcpCursor {
		case 0:
			m.opts.Cortex = !m.opts.Cortex
		case 1:
			m.opts.Context7 = !m.opts.Context7
		case themeRowIndex:
			m.opts.ApplyTheme = !m.opts.ApplyTheme
		}
		m.plan = nil
		m.reviewStatus = ""
		m.replanning = true
		return m, planCmd(m.svc, m.reviewOptions())
	case "o", "O":
		if m.plan != nil && (len(m.plan.Conflicts) > 0 || m.overwrite || m.hadConflict) {
			m.overwrite = !m.overwrite
			if !m.overwrite {
				// Deauthorizing drops the sticky hint so a conflict-free replan
				// never keeps advertising an overwrite the user withdrew.
				m.hadConflict = false
			}
			m.plan = nil
			m.reviewStatus = ""
			m.replanning = true
			return m, planCmd(m.svc, m.reviewOptions())
		}
	case "b", "B":
		m.screen = screenHome
		m.cursor = 0
		return m, homeTick()
	case "enter":
		if m.replanning {
			m.reviewStatus = "cannot run: the plan is still being computed"
			return m, nil
		}
		if m.cortexInstalling {
			m.reviewStatus = "cannot run: cortex installation is in progress"
			return m, nil
		}
		if m.planErr != nil {
			m.reviewStatus = "cannot run: fix the plan error above"
			return m, nil
		}
		if m.plan == nil {
			m.reviewStatus = "cannot run: no plan loaded"
			return m, nil
		}
		if len(m.plan.Conflicts) > 0 {
			// Blocking conflicts remain; surface why instead of swallowing the key.
			m.reviewStatus = conflictNotice(m.plan.Conflicts)
			return m, nil
		}
		if m.hadConflict && m.overwrite {
			m.confirm = confirmState{kind: confirmOverwrite}
			return m, nil
		}
		return m.startRunning(m.installModeTitle(), installPhases, installRunCmd(m.svc, m.installMode, m.confirmedOptions()))
	}
	return m, nil
}

func (m model) installModeTitle() string {
	if m.installMode == "sync" {
		return "Sync"
	}
	return "Install"
}

// onPlan records the read-only plan and derives the operation mode from the
// recorded installation metadata: an agreed v2 home reconciles via sync.
func (m model) onPlan(msg planMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenReview {
		return m, nil
	}
	m.replanning = false
	m.planErr = msg.err
	m.plan = msg.plan
	m.reviewScroll = 0
	m.reviewStatus = ""
	if msg.plan != nil {
		if m.plan.MetadataPresence == state.PresenceV2 {
			m.installMode = "sync"
		} else {
			m.installMode = "install"
		}
		if len(msg.plan.Conflicts) > 0 && !m.overwrite {
			m.hadConflict = true
		}
	}
	if msg.cortexMissing && m.opts.Cortex && !m.cortexPrompted {
		m.cortexPrompted = true
		m.confirm = confirmState{kind: confirmCortexInstall}
	}
	return m, nil
}

// --- Running ---

func (m model) updateRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// startRunning switches to the Running screen and dispatches the operation.
func (m model) startRunning(title string, phases []string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.screen = screenRunning
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(styles.Secondary)
	p := progress.New(
		progress.WithScaledGradient(string(styles.Primary), string(styles.Secondary)),
		progress.WithWidth(44),
	)
	m.running = runningState{
		title:         title,
		phases:        phases,
		current:       0,
		startedAt:     time.Now(),
		progressModel: p,
		spinnerModel:  sp,
	}
	return m, tea.Batch(spinTick(), cmd)
}

// advanceRunning phases the display forward while the operation executes.
func (m *model) advanceRunning() {
	if m.running.current < len(m.running.phases)-1 {
		m.running.current++
	}
}

// --- Result ---

func (m model) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter", "esc", "m":
		m.screen = screenHome
		m.cursor = 0
		return m, homeTick()
	case "r":
		if m.result.canRollback {
			m.confirm = confirmState{kind: confirmRollback}
			return m, nil
		}
	case "up", "k":
		if m.resultScroll > 0 {
			m.resultScroll--
		}
	case "down", "j":
		m.resultScroll++ // clamped against the detail length at render time
	}
	return m, nil
}

// --- MCP Manager ---

func (m model) updateMCP(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "m":
		m.screen = screenHome
		m.cursor = 1 // back on the Manage MCPs entry that opened this screen
		return m, homeTick()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if rows := m.mcpRows(); m.cursor < rows-1 {
			m.cursor++
		}
	case "r":
		return m, mcpListCmd(m.svc)
	case " ", "enter":
		entry, ok := m.selectedEntry()
		if !ok {
			return m, nil
		}
		switch entry.Status {
		case mcpmanager.StatusAbsent:
			// Adding a managed preset is the non-destructive direction and
			// fails closed by itself; no confirmation is required.
			return m.startRunning("MCP add "+entry.Name, mcpPhases, mcpMutateCmd(m.svc, entry.Name, true))
		case mcpmanager.StatusManaged:
			// Removing an accredited entry is destructive: confirm first.
			m.confirm = confirmState{kind: confirmMCPRemove, arg: entry.Name}
			return m, nil
		default:
			// Unmanaged-equivalent and conflicting entries are user-owned;
			// removal attempts require an explicit confirmation.
			m.confirm = confirmState{kind: confirmMCPRemove, arg: entry.Name}
			return m, nil
		}
	}
	return m, nil
}

func (m model) onMCPList(msg mcpListMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenMCP {
		return m, nil
	}
	m.mcpErr = msg.err
	m.mcpReport = msg.report
	m.cursor = 0
	return m, nil
}

// mcpRows is the cursor bound: managed presets only; unknown entries are
// informational and never selectable.
func (m model) mcpRows() int {
	if m.mcpReport == nil {
		return 0
	}
	return len(m.mcpReport.Entries)
}

func (m model) selectedEntry() (mcpmanager.EntryReport, bool) {
	if m.mcpReport == nil || m.cursor >= len(m.mcpReport.Entries) {
		return mcpmanager.EntryReport{}, false
	}
	return m.mcpReport.Entries[m.cursor], true
}

// --- Confirmation overlay ---

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.declineConfirm(), nil
	case "n", "N":
		return m.declineConfirm(), nil
	case "y", "Y":
		kind, arg := m.confirm.kind, m.confirm.arg
		m.confirm = confirmState{}
		switch kind {
		case confirmOverwrite:
			return m.startRunning(m.installModeTitle(), installPhases, installRunCmd(m.svc, m.installMode, m.confirmedOptions()))
		case confirmCortexInstall:
			m.cortexInstalling = true
			m.cortexStatus = "installing cortex with 'go install " + install.CortexModulePath + "'…"
			return m, cortexInstallCmd()
		case confirmUninstall:
			return m.startRunning("Uninstall", uninstallPhases, uninstallCmd(m.svc))
		case confirmMCPRemove:
			return m.startRunning("MCP remove "+arg, mcpPhases, mcpMutateCmd(m.svc, arg, false))
		case confirmRollback:
			return m.startRunning("Rollback", rollbackPhases, rollbackCmd(m.svc))
		}
	}
	return m, nil
}

// declineConfirm closes the overlay and records the manual remedy when the
// user declines the optional cortex install; every other confirmation is a
// silent cancel.
func (m model) declineConfirm() model {
	if m.confirm.kind == confirmCortexInstall {
		m.cortexStatus = "cortex installation skipped — run manually: " + install.CortexManualCommand
	}
	m.confirm = confirmState{}
	return m
}

// --- Usage stats ---

// updateStats delegates key handling to the self-contained stats state and
// applies the screen-level action it requests.
func (m model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var action statsAction
	m.stats, action = m.stats.update(msg)
	switch action {
	case statsActionHome:
		m.screen = screenHome
		m.cursor = statsEntryIndex
		return m, homeTick()
	case statsActionQuit:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// --- Models configuration ---

// openModels enters the models configuration screen and loads the registry
// through the service seam.
func (m model) openModels() (tea.Model, tea.Cmd) {
	m.screen = screenModels
	m.models = newModelsState(m.homeDir)
	return m, modelsLoadCmd(m.homeDir)
}

// updateModels delegates key handling to the self-contained models state and
// applies the screen-level action it requests.
func (m model) updateModels(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var action modelsAction
	var cmd tea.Cmd
	m.models, action, cmd = m.models.update(msg)
	switch action {
	case modelsActionHome:
		m.screen = screenHome
		return m, homeTick()
	case modelsActionQuit:
		m.quitting = true
		return m, tea.Quit
	}
	return m, cmd
}
