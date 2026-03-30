// Package tui implements the interactive terminal installer for cortex-ia.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/catalog"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// screen identifies the current TUI screen.
type screen int

const (
	screenWelcome screen = iota
	screenAgents
	screenPreset
	screenReview
	screenInstalling
	screenComplete
)

// agentItem represents a detected agent in the selection list.
type agentItem struct {
	id       model.AgentID
	name     string
	binary   string
	selected bool
}

// installDoneMsg signals that installation finished.
type installDoneMsg struct {
	result pipeline.InstallResult
	err    error
}

// Model is the root Bubbletea model.
type Model struct {
	screen    screen
	registry  *agents.Registry
	homeDir   string
	version   string
	cursor    int
	agents    []agentItem
	preset    model.PresetID
	presets   []model.PresetID
	resolved  []model.ComponentID
	result    pipeline.InstallResult
	installErr error
	quitting  bool
}

// New creates a new TUI model.
func New(registry *agents.Registry, homeDir, version string) Model {
	return Model{
		screen:   screenWelcome,
		registry: registry,
		homeDir:  homeDir,
		version:  version,
		presets:  []model.PresetID{model.PresetFull, model.PresetMinimal},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen != screenInstalling {
				m.quitting = true
				return m, tea.Quit
			}
		}
	case installDoneMsg:
		m.result = msg.result
		m.installErr = msg.err
		m.screen = screenComplete
		return m, nil
	}

	switch m.screen {
	case screenWelcome:
		return m.updateWelcome(msg)
	case screenAgents:
		return m.updateAgents(msg)
	case screenPreset:
		return m.updatePreset(msg)
	case screenReview:
		return m.updateReview(msg)
	case screenInstalling:
		return m, nil
	case screenComplete:
		return m.updateComplete(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	switch m.screen {
	case screenWelcome:
		return m.viewWelcome()
	case screenAgents:
		return m.viewAgents()
	case screenPreset:
		return m.viewPreset()
	case screenReview:
		return m.viewReview()
	case screenInstalling:
		return m.viewInstalling()
	case screenComplete:
		return m.viewComplete()
	}
	return ""
}

// --- Welcome ---

func (m Model) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "enter" {
			m.detectAgents()
			m.screen = screenAgents
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) viewWelcome() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(styles.Logo))
	sb.WriteString("\n\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("v%s — AI Agent Ecosystem Configurator", m.version)))
	sb.WriteString("\n\n")
	sb.WriteString("Configure your AI coding agents with:\n")
	sb.WriteString(styles.StatusOK.Render("  ● Cortex") + "       — Persistent memory + knowledge graph\n")
	sb.WriteString(styles.StatusOK.Render("  ● ForgeSpec") + "    — SDD contracts + task board\n")
	sb.WriteString(styles.StatusOK.Render("  ● Mailbox") + "      — Inter-agent messaging\n")
	sb.WriteString(styles.StatusOK.Render("  ● Orchestrator") + " — Multi-CLI routing\n")
	sb.WriteString(styles.StatusOK.Render("  ● Context7") + "     — Live documentation\n")
	sb.WriteString(styles.StatusOK.Render("  ● SDD") + "          — 9-phase development workflow\n")
	sb.WriteString("\n")
	sb.WriteString(styles.Help.Render("Press Enter to start • q to quit"))
	return sb.String()
}

// --- Agents ---

func (m *Model) detectAgents() {
	m.agents = nil
	for _, adapter := range m.registry.All() {
		installed, binary, _, _, _ := adapter.Detect(m.homeDir)
		if installed {
			m.agents = append(m.agents, agentItem{
				id:       adapter.Agent(),
				name:     string(adapter.Agent()),
				binary:   binary,
				selected: true, // Pre-select all detected
			})
		}
	}
}

func (m Model) updateAgents(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
		case " ":
			if m.cursor < len(m.agents) {
				m.agents[m.cursor].selected = !m.agents[m.cursor].selected
			}
		case "a":
			allSelected := true
			for _, a := range m.agents {
				if !a.selected {
					allSelected = false
					break
				}
			}
			for i := range m.agents {
				m.agents[i].selected = !allSelected
			}
		case "enter":
			hasSelected := false
			for _, a := range m.agents {
				if a.selected {
					hasSelected = true
					break
				}
			}
			if hasSelected {
				m.screen = screenPreset
				m.cursor = 0
			}
		case "esc":
			m.screen = screenWelcome
		}
	}
	return m, nil
}

func (m Model) viewAgents() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Select Agents"))
	sb.WriteString("\n\n")

	for i, a := range m.agents {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.Cursor.Render("> ")
		}

		check := "○"
		nameStyle := lipgloss.NewStyle()
		if a.selected {
			check = styles.Selected.Render("●")
			nameStyle = styles.Selected
		}

		fmt.Fprintf(&sb, "%s%s %s", cursor, check, nameStyle.Render(a.name))
		if a.binary != "" {
			sb.WriteString(styles.Description.Render(fmt.Sprintf(" (%s)", a.binary)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Space toggle • a all • Enter confirm • Esc back"))
	return sb.String()
}

// --- Preset ---

func (m Model) updatePreset(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.presets)-1 {
				m.cursor++
			}
		case "enter":
			m.preset = m.presets[m.cursor]
			components := catalog.ComponentsForPreset(m.preset)
			m.resolved = catalog.ResolveDeps(components)
			m.screen = screenReview
		case "esc":
			m.screen = screenAgents
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) viewPreset() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Select Preset"))
	sb.WriteString("\n\n")

	descs := map[model.PresetID]string{
		model.PresetFull:    "All 8 components — full ecosystem",
		model.PresetMinimal: "Cortex + ForgeSpec + Context7 + SDD",
	}

	for i, p := range m.presets {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.Cursor.Render("> ")
		}

		name := styles.Subtitle.Render(string(p))
		desc := styles.Description.Render(" — " + descs[p])
		fmt.Fprintf(&sb, "%s%s%s\n", cursor, name, desc)
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Enter select • Esc back"))
	return sb.String()
}

// --- Review ---

func (m Model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "y":
			m.screen = screenInstalling
			return m, m.runInstall()
		case "esc", "n":
			m.screen = screenPreset
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) viewReview() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Review Installation"))
	sb.WriteString("\n\n")

	sb.WriteString(styles.Subtitle.Render("Agents:"))
	sb.WriteString("\n")
	for _, a := range m.agents {
		if a.selected {
			fmt.Fprintf(&sb, "  %s %s\n", styles.StatusOK.Render("●"), a.name)
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Preset: %s", m.preset)))
	sb.WriteString("\n\n")

	sb.WriteString(styles.Subtitle.Render("Components (with dependencies):"))
	sb.WriteString("\n")
	cmap := catalog.ComponentMap()
	for _, id := range m.resolved {
		if info, ok := cmap[id]; ok {
			fmt.Fprintf(&sb, "  %s %-18s %s\n",
				styles.StatusOK.Render("●"),
				styles.Selected.Render(string(id)),
				styles.Description.Render(info.Description))
		}
	}

	sb.WriteString(styles.Help.Render("\nEnter to install • Esc to go back"))
	return sb.String()
}

// --- Installing ---

func (m Model) runInstall() tea.Cmd {
	return func() tea.Msg {
		var agentIDs []model.AgentID
		for _, a := range m.agents {
			if a.selected {
				agentIDs = append(agentIDs, a.id)
			}
		}

		selection := model.Selection{
			Agents:     agentIDs,
			Preset:     m.preset,
			Components: m.resolved,
		}

		result, err := pipeline.Install(m.homeDir, m.registry, selection, m.version, false)
		return installDoneMsg{result: result, err: err}
	}
}

func (m Model) viewInstalling() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Installing..."))
	sb.WriteString("\n\n")
	sb.WriteString("Configuring agents with cortex-ia ecosystem.\n")
	sb.WriteString("This may take a moment.\n")
	return sb.String()
}

// --- Complete ---

func (m Model) updateComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewComplete() string {
	var sb strings.Builder

	if m.installErr != nil {
		sb.WriteString(styles.StatusFail.Render("Installation Failed"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.installErr)
	} else {
		sb.WriteString(styles.StatusOK.Render("✓ Installation Complete"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Components: %d\n", len(m.result.ComponentsDone))
		fmt.Fprintf(&sb, "Files changed: %d\n", len(m.result.FilesChanged))
		if m.result.BackupID != "" {
			fmt.Fprintf(&sb, "Backup: %s\n", m.result.BackupID)
		}
	}

	if len(m.result.Errors) > 0 {
		sb.WriteString(styles.StatusWarn.Render("\nWarnings:"))
		sb.WriteString("\n")
		for _, e := range m.result.Errors {
			fmt.Fprintf(&sb, "  %s\n", e)
		}
	}

	sb.WriteString(styles.Help.Render("\nPress any key to exit"))
	return sb.String()
}

// Run starts the TUI.
func Run(version string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	registry := agents.NewDefaultRegistry()
	m := New(registry, homeDir, version)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}
