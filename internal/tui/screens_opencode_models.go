package tui

// OpenCode model configuration screens: model list per sub-agent,
// provider picker, and model picker.

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/opencode"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// --- OpenCode Models List ---

func (m *Model) loadOpenCodeModels() {
	// Load saved assignments or defaults
	saved, err := state.LoadOpenCodeModels(m.HomeDir)
	if err != nil || len(saved) == 0 {
		m.OpenCodeAssignments = model.OpenCodeDefaultAssignments()
	} else {
		m.OpenCodeAssignments = saved
	}
	// Load available providers (hybrid: cache → fallback)
	m.OpenCodeProviders = opencode.DetectProviders(m.HomeDir)
}

func (m Model) updateOpenCodeModels(msg tea.Msg) (tea.Model, tea.Cmd) {
	agents := model.OpenCodeSubAgents()
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(agents)-1 {
				m.Cursor++
			}
		case "enter":
			if m.Cursor < len(agents) {
				m.OCSelectedAgent = agents[m.Cursor]
				m.OCProviderCursor = 0
				m.setScreen(ScreenOpenCodeProviderPicker)
			}
		case "s":
			// Save current assignments
			if err := state.SaveOpenCodeModels(m.HomeDir, m.OpenCodeAssignments); err != nil {
				m.OCErr = err
			} else {
				m.OCErr = nil
				m.ActiveToast = Toast{Text: "Models saved", Visible: true}
				return m, dismissToastAfter(3 * time.Second)
			}
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) viewOpenCodeModels() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("OpenCode Model Configuration"))
	sb.WriteString("\n\n")
	sb.WriteString(styles.Description.Render("Assign a model to each SDD sub-agent:"))
	sb.WriteString("\n\n")

	if m.OCErr != nil {
		sb.WriteString(styles.StatusFail.Render(fmt.Sprintf("Error: %v", m.OCErr)))
		sb.WriteString("\n\n")
	}

	agents := model.OpenCodeSubAgents()
	for i, agent := range agents {
		cursor := "  "
		if i == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}

		// Get current assignment
		assignment, ok := m.OpenCodeAssignments[agent]
		modelStr := styles.Description.Render("(not set)")
		if ok && assignment.Model != "" {
			modelStr = styles.Subtitle.Render(assignment.Provider+"/") +
				styles.Selected.Render(assignment.Model)
		}

		desc := model.OpenCodeSubAgentDescription(agent)
		name := fmt.Sprintf("%-16s", agent)
		fmt.Fprintf(&sb, "%s%s %s %s\n",
			cursor,
			styles.Subtitle.Render(name),
			modelStr,
			styles.Description.Render("— "+desc))
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Enter select model • s save • Esc back"))
	return sb.String()
}

// --- OpenCode Provider Picker ---

func (m Model) updateOpenCodeProviderPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.OCProviderCursor > 0 {
				m.OCProviderCursor--
			}
		case "down", "j":
			if m.OCProviderCursor < len(m.OpenCodeProviders)-1 {
				m.OCProviderCursor++
			}
		case "enter":
			if m.OCProviderCursor < len(m.OpenCodeProviders) {
				m.OCModelCursor = 0
				m.setScreen(ScreenOpenCodeModelPicker)
			}
		case "esc":
			m.setScreen(ScreenOpenCodeModels)
		}
	}
	return m, nil
}

func (m Model) viewOpenCodeProviderPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("Select Provider for %s", m.OCSelectedAgent)))
	sb.WriteString("\n\n")

	for i, p := range m.OpenCodeProviders {
		cursor := "  "
		if i == m.OCProviderCursor {
			cursor = styles.Cursor.Render("> ")
		}
		modelCount := styles.Description.Render(fmt.Sprintf("(%d models)", len(p.Models)))
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render(p.Name), modelCount)
	}

	return sb.String()
}

// --- OpenCode Model Picker ---

func (m Model) updateOpenCodeModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.OCProviderCursor >= len(m.OpenCodeProviders) {
		return m, nil
	}
	provider := m.OpenCodeProviders[m.OCProviderCursor]

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.OCModelCursor > 0 {
				m.OCModelCursor--
			}
		case "down", "j":
			if m.OCModelCursor < len(provider.Models)-1 {
				m.OCModelCursor++
			}
		case "enter":
			if m.OCModelCursor < len(provider.Models) {
				selectedModel := provider.Models[m.OCModelCursor]
				if m.OpenCodeAssignments == nil {
					m.OpenCodeAssignments = make(model.OpenCodeModelAssignments)
				}
				m.OpenCodeAssignments[m.OCSelectedAgent] = model.OpenCodeModelAssignment{
					Provider: provider.ID,
					Model:    selectedModel.ID,
				}
				m.setScreen(ScreenOpenCodeModels)
			}
		case "esc":
			m.setScreen(ScreenOpenCodeProviderPicker)
		}
	}
	return m, nil
}

func (m Model) viewOpenCodeModelPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("Select Model for %s", m.OCSelectedAgent)))
	sb.WriteString("\n\n")

	if m.OCProviderCursor >= len(m.OpenCodeProviders) {
		sb.WriteString(styles.Description.Render("No providers available."))
		return sb.String()
	}
	provider := m.OpenCodeProviders[m.OCProviderCursor]
	sb.WriteString(styles.Subtitle.Render(provider.Name))
	sb.WriteString("\n\n")

	for i, mdl := range provider.Models {
		cursor := "  "
		if i == m.OCModelCursor {
			cursor = styles.Cursor.Render("> ")
		}

		// Show checkmark if this model is currently assigned
		current, ok := m.OpenCodeAssignments[m.OCSelectedAgent]
		marker := "  "
		if ok && current.Provider == provider.ID && current.Model == mdl.ID {
			marker = styles.StatusOK.Render("✓ ")
		}

		fmt.Fprintf(&sb, "%s%s%s %s\n",
			cursor, marker,
			styles.Selected.Render(mdl.Name),
			styles.Description.Render(mdl.ID))
	}

	return sb.String()
}
