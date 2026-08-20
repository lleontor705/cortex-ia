package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// screen enumerates the five conceptual screens of the TUI. Confirmation is
// an overlay state on the current screen, not a sixth screen.
type screen int

const (
	screenHome screen = iota
	screenReview
	screenRunning
	screenResult
	screenMCP
)

// confirmKind identifies which destructive intent a confirmation modal guards.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmOverwrite
	confirmUninstall
	confirmMCPRemove
	confirmRollback
)

// homeEntries are the fixed Home menu actions, in display order.
var homeEntries = []string{
	"Install / Sync",
	"Manage MCPs",
	"Doctor / Recovery",
	"Uninstall",
	"Quit",
}

// managedNames lists the managed MCP presets in toggle order.
var managedNames = []string{"cortex", "forgespec", "context7"}

// confirmState is the active confirmation overlay. arg carries the MCP name
// or backup ID the confirmed action applies to.
type confirmState struct {
	kind confirmKind
	arg  string
}

// runningState drives the phase display while one operation executes.
type runningState struct {
	title    string
	phases   []string
	current  int
	spinner  int
	finished bool
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
	installMode string          // "install" or "sync", derived from the plan
	opts        install.Options // current selection for install/sync
	plan        *pipeline.Plan  // latest read-only plan
	planErr     error
	overwrite   bool // explicit overwrite authorization
	hadConflict bool // the initial plan carried conflicts
	replanning  bool

	// MCP Manager state.
	mcpReport *install.MCPListReport
	mcpErr    error

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
}

// newModel builds the model bound to a service implementation.
func newModel(svc ServiceAPI, homeDir, version string) model {
	return model{
		svc:     svc,
		homeDir: homeDir,
		version: version,
		screen:  screenHome,
		opts:    install.DefaultOptions(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// Update routes one message by screen, with the confirmation overlay taking
// precedence over screen keys.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if m.screen == screenRunning && !m.running.finished {
			m.running.spinner = (m.running.spinner + 1) % len(spinnerFrames)
			return m, spinTick()
		}
		return m, nil
	case planMsg:
		return m.onPlan(msg)
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
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(homeEntries)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectHomeEntry(m.cursor)
	}
	return m, nil
}

func (m model) selectHomeEntry(index int) (tea.Model, tea.Cmd) {
	m.cursor = index
	switch index {
	case 0: // Install / Sync → Review
		m.screen = screenReview
		m.opts = install.DefaultOptions()
		m.opts.Version = m.version
		m.overwrite = false
		m.hadConflict = false
		m.installMode = ""
		m.plan = nil
		m.planErr = nil
		m.replanning = false
		m.mcpCursor = 0
		m.replanning = true
		return m, planCmd(m.svc, m.reviewOptions())
	case 1: // Manage MCPs
		m.screen = screenMCP
		m.mcpReport = nil
		m.mcpErr = nil
		return m, mcpListCmd(m.svc)
	case 2: // Doctor / Recovery
		return m.startRunning("Doctor", []string{"Inspect state", "Compare digests", "Assess MCPs", "Report"}, doctorCmd(m.svc))
	case 3: // Uninstall (destructive: explicit confirmation first)
		m.confirm = confirmState{kind: confirmUninstall}
		return m, nil
	case 4: // Quit
		m.quitting = true
		return m, tea.Quit
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
	case "up", "k":
		if m.mcpCursor > 0 {
			m.mcpCursor--
		}
	case "down", "j":
		if m.mcpCursor < len(managedNames)-1 {
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
			m.opts.ForgeSpec = !m.opts.ForgeSpec
		case 2:
			m.opts.Context7 = !m.opts.Context7
		}
		m.replanning = true
		return m, planCmd(m.svc, m.reviewOptions())
	case "o":
		if m.plan != nil && len(m.plan.Conflicts) > 0 {
			m.overwrite = !m.overwrite
			m.replanning = true
			return m, planCmd(m.svc, m.reviewOptions())
		}
	case "enter":
		if m.planErr != nil || m.plan == nil {
			return m, nil
		}
		if len(m.plan.Conflicts) > 0 {
			return m, nil // blocking conflicts remain; nothing runs
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
	m.running = runningState{title: title, phases: phases, current: 0}
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
		m.confirm = confirmState{}
		return m, nil
	case "n", "N":
		m.confirm = confirmState{}
		return m, nil
	case "y", "Y":
		kind, arg := m.confirm.kind, m.confirm.arg
		m.confirm = confirmState{}
		switch kind {
		case confirmOverwrite:
			return m.startRunning(m.installModeTitle(), installPhases, installRunCmd(m.svc, m.installMode, m.confirmedOptions()))
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
