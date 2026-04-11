package tui

// Install flow screens: Claude Model Picker, SDD Mode, Strict TDD,
// Dependency Tree, Skill Picker.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// --- Claude Model Picker ---

func (m Model) updateClaudeModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	models := []model.ClaudeModelAlias{model.ModelOpus, model.ModelSonnet, model.ModelHaiku}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.ClaudeModelCursor > 0 {
				m.ClaudeModelCursor--
			}
		case "down", "j":
			if m.ClaudeModelCursor < len(models)-1 {
				m.ClaudeModelCursor++
			}
		case "enter":
			presets := []model.ModelPreset{model.ModelPresetPerformance, model.ModelPresetBalanced, model.ModelPresetEconomy}
			if m.ClaudeModelCursor < len(presets) {
				m.ModelPreset = presets[m.ClaudeModelCursor]
			} else {
				m.ModelPreset = model.ModelPresetBalanced
			}
			m.ModelAssignments = model.ModelsForPreset(m.ModelPreset)
			m.setScreen(ScreenSDDMode)
		case "esc":
			if m.ModelConfigMode {
				m.ModelConfigMode = false
				m.setScreen(ScreenWelcome)
			} else {
				m.setScreen(ScreenPreset)
			}
		}
	}
	return m, nil
}

func (m Model) viewClaudeModelPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Select Claude Model"))
	sb.WriteString("\n\n")

	models := []struct {
		alias model.ClaudeModelAlias
		desc  string
	}{
		{model.ModelOpus, "Most capable — deep reasoning, complex tasks"},
		{model.ModelSonnet, "Balanced — fast and capable (recommended)"},
		{model.ModelHaiku, "Fastest — simple tasks, low latency"},
	}

	for i, m2 := range models {
		cursor := "  "
		if i == m.ClaudeModelCursor {
			cursor = styles.Cursor.Render("> ")
		}
		name := styles.Subtitle.Render(string(m2.alias))
		desc := styles.Description.Render(" — " + m2.desc)
		fmt.Fprintf(&sb, "%s%s%s\n", cursor, name, desc)
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Enter select • Esc back"))
	return sb.String()
}

// --- SDD Mode ---

func (m Model) updateSDDMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k", "down", "j":
			m.SDDEnabled = !m.SDDEnabled
		case "enter":
			m.setScreen(ScreenStrictTDD)
		case "esc":
			m.setScreen(ScreenClaudeModelPicker)
		}
	}
	return m, nil
}

func (m Model) viewSDDMode() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("SDD Integration"))
	sb.WriteString("\n\n")
	sb.WriteString("Enable Spec-Driven Development workflow?\n")
	sb.WriteString("SDD provides a 9-phase structured development process.\n\n")

	options := []struct {
		label    string
		selected bool
	}{
		{"Enable SDD (recommended)", m.SDDEnabled},
		{"Disable SDD", !m.SDDEnabled},
	}

	for i, opt := range options {
		cursor := "  "
		if (i == 0 && m.SDDEnabled) || (i == 1 && !m.SDDEnabled) {
			cursor = styles.Cursor.Render("> ")
		}
		marker := "( )"
		if opt.selected {
			marker = styles.Selected.Render("(*)")
		}
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, marker, opt.label)
	}

	sb.WriteString(styles.Help.Render("\n↑↓ toggle • Enter confirm • Esc back"))
	return sb.String()
}

// --- Strict TDD ---

func (m Model) updateStrictTDD(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k", "down", "j":
			m.StrictTDDEnabled = !m.StrictTDDEnabled
		case "enter":
			m.setScreen(ScreenDependencyTree)
		case "esc":
			m.setScreen(ScreenSDDMode)
		}
	}
	return m, nil
}

func (m Model) viewStrictTDD() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Strict TDD Mode"))
	sb.WriteString("\n\n")
	sb.WriteString("Enable strict Test-Driven Development?\n")
	sb.WriteString("When enabled, tests must be written before implementation.\n\n")

	options := []struct {
		label    string
		selected bool
	}{
		{"Enable Strict TDD", m.StrictTDDEnabled},
		{"Disable Strict TDD (default)", !m.StrictTDDEnabled},
	}

	for i, opt := range options {
		cursor := "  "
		if (i == 0 && m.StrictTDDEnabled) || (i == 1 && !m.StrictTDDEnabled) {
			cursor = styles.Cursor.Render("> ")
		}
		marker := "( )"
		if opt.selected {
			marker = styles.Selected.Render("(*)")
		}
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, marker, opt.label)
	}

	sb.WriteString(styles.Help.Render("\n↑↓ toggle • Enter confirm • Esc back"))
	return sb.String()
}

// --- Dependency Tree ---

func (m Model) updateDependencyTree(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			// Load community skills before showing picker.
			names := state.ListCommunitySkills(m.HomeDir)
			m.AvailableSkills = make([]SkillItem, len(names))
			for i, name := range names {
				m.AvailableSkills[i] = SkillItem{Name: name, Selected: true}
			}
			m.SkillCursor = 0
			m.setScreen(ScreenSkillPicker)
		case "esc":
			m.setScreen(ScreenStrictTDD)
		}
	}
	return m, nil
}

func (m Model) viewDependencyTree() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Dependency Tree"))
	sb.WriteString("\n\n")
	sb.WriteString(styles.Subtitle.Render("Resolved components (in dependency order):"))
	sb.WriteString("\n\n")

	for i, id := range m.Resolved {
		prefix := "├── "
		if i == len(m.Resolved)-1 {
			prefix = "└── "
		}
		fmt.Fprintf(&sb, "  %s%s\n", styles.Description.Render(prefix), styles.Selected.Render(string(id)))
	}

	sb.WriteString(styles.Help.Render("\nEnter to continue • Esc back"))
	return sb.String()
}

// --- Skill Picker ---

func (m Model) updateSkillPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.SkillCursor > 0 {
				m.SkillCursor--
			}
		case "down", "j":
			if m.SkillCursor < len(m.AvailableSkills)-1 {
				m.SkillCursor++
			}
		case " ":
			if m.SkillCursor < len(m.AvailableSkills) {
				m.AvailableSkills[m.SkillCursor].Selected = !m.AvailableSkills[m.SkillCursor].Selected
			}
		case "a":
			allSelected := true
			for _, s := range m.AvailableSkills {
				if !s.Selected {
					allSelected = false
					break
				}
			}
			for i := range m.AvailableSkills {
				m.AvailableSkills[i].Selected = !allSelected
			}
		case "enter":
			// Collect selected skills into SkillSelection.
			m.SkillSelection = nil
			for _, s := range m.AvailableSkills {
				if s.Selected {
					m.SkillSelection = append(m.SkillSelection, model.SkillID(s.Name))
				}
			}
			m.setScreen(ScreenReview)
		case "esc":
			m.setScreen(ScreenDependencyTree)
		}
	}
	return m, nil
}

func (m Model) viewSkillPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Community Skills"))
	sb.WriteString("\n\n")

	if len(m.AvailableSkills) == 0 {
		sb.WriteString(styles.Description.Render("No community skills installed."))
		sb.WriteString("\n")
		sb.WriteString("Use " + styles.Subtitle.Render("cortex-ia skill add <path>") + " to install skills.\n")
		sb.WriteString("Or use " + styles.Subtitle.Render("Create your own Agent") + " from the main menu.\n")
	} else {
		sb.WriteString(fmt.Sprintf("Found %d community skill(s). Toggle which to include:\n\n", len(m.AvailableSkills)))
		for i, s := range m.AvailableSkills {
			cursor := "  "
			if i == m.SkillCursor {
				cursor = styles.Cursor.Render("> ")
			}
			check := "○"
			if s.Selected {
				check = styles.Selected.Render("●")
			}
			fmt.Fprintf(&sb, "%s%s %s\n", cursor, check, s.Name)
		}
	}

	sb.WriteString(styles.Help.Render("\n↑↓ navigate • Space toggle • a all • Enter confirm • Esc back"))
	return sb.String()
}
