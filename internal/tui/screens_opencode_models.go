package tui

// OpenCode model configuration screens: sub-agent list with assignments,
// and a flat model picker with search filter.

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

// --- Load models ---

func (m *Model) loadOpenCodeModels() {
	// Load saved assignments or defaults
	saved, err := state.LoadOpenCodeModels(m.HomeDir)
	if err != nil || len(saved) == 0 {
		m.OpenCodeAssignments = model.OpenCodeDefaultAssignments()
	} else {
		m.OpenCodeAssignments = saved
	}
	// Load available models (CLI → cache → fallback)
	m.OpenCodeProviders = opencode.DetectModels(m.HomeDir)
	m.OCFlatModels = opencode.FlatModelList(m.OpenCodeProviders)
}

// --- ScreenOpenCodeModels: Sub-agent list ---

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
				m.OCModelCursor = 0
				m.OCModelFilter.Deactivate()
				m.setScreen(ScreenOpenCodeModelPicker)
			}
		case "s":
			// Save to cortex-ia state AND apply to opencode.json
			if err := state.SaveOpenCodeModels(m.HomeDir, m.OpenCodeAssignments); err != nil {
				m.OCErr = err
				return m, nil
			}
			if err := opencode.ApplyToOpenCodeConfig(m.HomeDir, m.OpenCodeAssignments); err != nil {
				m.OCErr = err
				m.ActiveToast = Toast{Text: fmt.Sprintf("Saved but failed to update opencode.json: %v", err), IsError: true, Visible: true}
			} else {
				m.OCErr = nil
				m.ActiveToast = Toast{Text: "Models saved and applied to opencode.json", Visible: true}
			}
			return m, dismissToastAfter(3 * time.Second)
		case "r":
			// Reload models from CLI
			m.OpenCodeProviders = opencode.DetectModels(m.HomeDir)
			m.OCFlatModels = opencode.FlatModelList(m.OpenCodeProviders)
			m.ActiveToast = Toast{Text: fmt.Sprintf("Loaded %d models", len(m.OCFlatModels)), Visible: true}
			return m, dismissToastAfter(2 * time.Second)
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

	if m.OCErr != nil {
		sb.WriteString(styles.StatusFail.Render(fmt.Sprintf("Error: %v", m.OCErr)))
		sb.WriteString("\n\n")
	}

	sb.WriteString(styles.Description.Render(fmt.Sprintf("%d models available from %d providers",
		len(m.OCFlatModels), len(m.OpenCodeProviders))))
	sb.WriteString("\n\n")

	agents := model.OpenCodeSubAgents()
	for i, agent := range agents {
		cursor := "  "
		if i == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}

		assignment, ok := m.OpenCodeAssignments[agent]
		modelStr := styles.Description.Render("(not set)")
		if ok && assignment.Model != "" {
			modelStr = styles.Selected.Render(assignment.FormatOpenCodeModel())
		}

		desc := model.OpenCodeSubAgentDescription(agent)
		name := fmt.Sprintf("%-16s", agent)
		fmt.Fprintf(&sb, "%s%s %s  %s\n",
			cursor,
			styles.Subtitle.Render(name),
			modelStr,
			styles.Description.Render(desc))
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Enter select • s save & apply • r reload models • Esc back"))
	return sb.String()
}

// --- ScreenOpenCodeModelPicker: Flat model list with filter ---

func (m Model) updateOpenCodeModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When filter is active, delegate to filter input
	if m.OCModelFilter.Active {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.OCModelFilter.Deactivate()
				return m, nil
			case "enter":
				// Select the model at cursor if visible
				visible := m.visibleOCModels()
				if m.OCModelCursor < len(visible) {
					m.assignOCModel(visible[m.OCModelCursor])
					m.OCModelFilter.Deactivate()
					m.setScreen(ScreenOpenCodeModels)
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.OCModelFilter.Input, cmd = m.OCModelFilter.Input.Update(msg)
		visible := m.visibleOCModels()
		if m.OCModelCursor >= len(visible) {
			m.OCModelCursor = max(len(visible)-1, 0)
		}
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		visible := m.visibleOCModels()
		switch key.String() {
		case "up", "k":
			if m.OCModelCursor > 0 {
				m.OCModelCursor--
			}
		case "down", "j":
			if m.OCModelCursor < len(visible)-1 {
				m.OCModelCursor++
			}
		case "/":
			m.OCModelFilter.Activate()
			return m, m.OCModelFilter.Input.Focus()
		case "enter":
			if m.OCModelCursor < len(visible) {
				m.assignOCModel(visible[m.OCModelCursor])
				m.setScreen(ScreenOpenCodeModels)
			}
		case "esc":
			m.OCModelFilter.Deactivate()
			m.setScreen(ScreenOpenCodeModels)
		}
	}
	return m, nil
}

// assignOCModel parses "provider/model" and sets the assignment.
func (m *Model) assignOCModel(fullModel string) {
	slashIdx := strings.Index(fullModel, "/")
	if slashIdx < 0 {
		return
	}
	provider := fullModel[:slashIdx]
	modelID := fullModel[slashIdx+1:]
	if m.OpenCodeAssignments == nil {
		m.OpenCodeAssignments = make(model.OpenCodeModelAssignments)
	}
	m.OpenCodeAssignments[m.OCSelectedAgent] = model.OpenCodeModelAssignment{
		Provider: provider,
		Model:    modelID,
	}
}

// visibleOCModels returns models matching the current filter.
func (m Model) visibleOCModels() []string {
	if m.OCModelFilter.Query() == "" {
		return m.OCFlatModels
	}
	var visible []string
	for _, mdl := range m.OCFlatModels {
		if m.OCModelFilter.Matches(mdl) {
			visible = append(visible, mdl)
		}
	}
	return visible
}

func (m Model) viewOpenCodeModelPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("Select Model for %s", styles.Subtitle.Render(m.OCSelectedAgent))))
	sb.WriteString("\n\n")

	// Filter input
	if m.OCModelFilter.Active || m.OCModelFilter.Query() != "" {
		sb.WriteString(m.OCModelFilter.View())
	}

	visible := m.visibleOCModels()

	// Get current assignment for highlighting
	currentModel := ""
	if a, ok := m.OpenCodeAssignments[m.OCSelectedAgent]; ok {
		currentModel = a.FormatOpenCodeModel()
	}

	// Show count
	if m.OCModelFilter.Query() != "" {
		sb.WriteString(styles.Description.Render(fmt.Sprintf("%d/%d matching", len(visible), len(m.OCFlatModels))))
		sb.WriteString("\n\n")
	}

	// Windowed rendering for long lists
	maxVisible := max(m.Height-10, 10)
	total := len(visible)
	start := 0
	if m.OCModelCursor >= maxVisible {
		start = m.OCModelCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = max(end-maxVisible, 0)
	}

	// Scroll indicator above
	if start > 0 {
		sb.WriteString(styles.Description.Render(fmt.Sprintf("  ▲ %d more above\n", start)))
	}

	lastProvider := ""
	for i := start; i < end; i++ {
		mdl := visible[i]
		// Show provider header when it changes
		slashIdx := strings.Index(mdl, "/")
		if slashIdx > 0 {
			provider := mdl[:slashIdx]
			if provider != lastProvider {
				if lastProvider != "" {
					sb.WriteString("\n")
				}
				sb.WriteString(styles.Subtitle.Render("  "+provider))
				sb.WriteString("\n")
				lastProvider = provider
			}
		}

		cursor := "  "
		if i == m.OCModelCursor {
			cursor = styles.Cursor.Render("> ")
		}

		marker := "  "
		if mdl == currentModel {
			marker = styles.StatusOK.Render("✓ ")
		}

		// Show just the model part after provider/
		displayName := mdl
		if slashIdx > 0 {
			displayName = mdl[slashIdx+1:]
		}

		fmt.Fprintf(&sb, "  %s%s%s\n", cursor, marker, displayName)
	}

	// Scroll indicator below
	if end < total {
		sb.WriteString(styles.Description.Render(fmt.Sprintf("  ▼ %d more below\n", total-end)))
	}

	if len(visible) == 0 {
		sb.WriteString(styles.Description.Render("  No matching models.\n"))
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("/ filter • Enter select • Esc back"))
	return sb.String()
}
