package tui

// Install flow screens: Claude Model Picker, Skill Picker.
// Removed: Preset, SDD Mode, Strict TDD, Dependency Tree (integrated into Review).

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/state"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// readSkillDescription reads the first non-empty, non-frontmatter line from a SKILL.md.
func readSkillDescription(homeDir, skillName string) string {
	path := filepath.Join(state.CommunitySkillsDir(homeDir), skillName, "SKILL.md")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 60 {
			return line[:57] + "..."
		}
		return line
	}
	return ""
}

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
			// Load community skills before showing picker
			if len(m.AvailableSkills) == 0 {
				names := state.ListCommunitySkills(m.HomeDir)
				m.AvailableSkills = make([]SkillItem, len(names))
				for i, name := range names {
					m.AvailableSkills[i] = SkillItem{
						Name:        name,
						Description: readSkillDescription(m.HomeDir, name),
						Selected:    true,
					}
				}
			}
			m.SkillCursor = 0
			m.setScreen(ScreenSkillPicker)
		case "esc":
			m.setScreen(ScreenPersona)
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

	return sb.String()
}

// --- Skill Picker ---

func (m Model) updateSkillPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When filter is active, delegate to filter input
	if m.SkillFilter.Active {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.SkillFilter.Deactivate()
				return m, nil
			case "enter":
				m.SkillFilter.Deactivate()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.SkillFilter.Input, cmd = m.SkillFilter.Input.Update(msg)
		visible := m.visibleSkills()
		if m.SkillCursor >= len(visible) {
			m.SkillCursor = max(len(visible)-1, 0)
		}
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		visible := m.visibleSkills()
		switch key.String() {
		case "up", "k":
			if m.SkillCursor > 0 {
				m.SkillCursor--
			}
		case "down", "j":
			if m.SkillCursor < len(visible)-1 {
				m.SkillCursor++
			}
		case " ":
			if m.SkillCursor < len(visible) {
				idx := visible[m.SkillCursor]
				m.AvailableSkills[idx].Selected = !m.AvailableSkills[idx].Selected
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
		case "/":
			m.SkillFilter.Activate()
			return m, m.SkillFilter.Input.Focus()
		case "enter":
			m.SkillSelection = nil
			for _, s := range m.AvailableSkills {
				if s.Selected {
					m.SkillSelection = append(m.SkillSelection, model.SkillID(s.Name))
				}
			}
			m.SkillFilter.Deactivate()
			m.setScreen(ScreenReview)
		case "esc":
			m.SkillFilter.Deactivate()
			m.setScreen(ScreenClaudeModelPicker)
		}
	}
	return m, nil
}

// visibleSkills returns indices of skills matching the current filter.
func (m Model) visibleSkills() []int {
	var indices []int
	for i, s := range m.AvailableSkills {
		if m.SkillFilter.Matches(s.Name) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m Model) viewSkillPicker() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Community Skills"))
	sb.WriteString("\n\n")

	if len(m.AvailableSkills) == 0 {
		sb.WriteString(styles.Description.Render("No community skills installed."))
		sb.WriteString("\n")
		sb.WriteString("Use " + styles.Subtitle.Render("cortex-ia skill add <path>") + " to install skills.\n")
		sb.WriteString("Or use " + styles.Subtitle.Render("Agent Builder") + " from the main menu.\n")
	} else {
		fmt.Fprintf(&sb, "Found %d community skill(s). Toggle which to include:", len(m.AvailableSkills))
		sb.WriteString(m.SkillFilter.Hint())
		sb.WriteString("\n\n")

		// Show filter input
		if m.SkillFilter.Active || m.SkillFilter.Query() != "" {
			sb.WriteString(m.SkillFilter.View())
		}

		visible := m.visibleSkills()
		for i, idx := range visible {
			s := m.AvailableSkills[idx]
			cursor := "  "
			if i == m.SkillCursor {
				cursor = styles.Cursor.Render("> ")
			}
			check := "○"
			if s.Selected {
				check = styles.Selected.Render("●")
			}
			if s.Description != "" {
				fmt.Fprintf(&sb, "%s%s %s %s\n", cursor, check, s.Name,
					styles.Description.Render("— "+s.Description))
			} else {
				fmt.Fprintf(&sb, "%s%s %s\n", cursor, check, s.Name)
			}
		}

		if len(visible) == 0 && m.SkillFilter.Query() != "" {
			sb.WriteString(styles.Description.Render("  No matching skills.\n"))
		}
	}

	return sb.String()
}

// --- Model Config screen (unified Claude + OpenCode) ---

// ModelConfigTab identifies the active tab on the Model Config screen.
type ModelConfigTab int

const (
	ModelConfigTabClaude ModelConfigTab = iota
	ModelConfigTabOpenCode
)

func (m Model) updateModelConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			if m.ModelConfigTab == ModelConfigTabClaude {
				m.ModelConfigTab = ModelConfigTabOpenCode
				if len(m.OpenCodeProviders) == 0 {
					m.loadOpenCodeModels()
				}
			} else {
				m.ModelConfigTab = ModelConfigTabClaude
			}
			m.Cursor = 0
			return m, nil
		case "esc":
			m.setScreen(ScreenWelcome)
			return m, nil
		}

		if m.ModelConfigTab == ModelConfigTabClaude {
			return m.updateModelConfigClaude(msg)
		}
		return m.updateModelConfigOpenCode(msg)
	}
	return m, nil
}

func (m Model) updateModelConfigClaude(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.ActiveToast = Toast{Text: "Model preset updated: " + string(m.ModelPreset), Visible: true}
			return m, dismissToastAfter(3 * time.Second)
		}
	}
	return m, nil
}

func (m Model) viewModelConfig() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Configure Models"))
	sb.WriteString("\n\n")

	sb.WriteString(renderTabBar([]string{"Claude", "OpenCode"}, int(m.ModelConfigTab)))
	sb.WriteString("\n\n")

	if m.ModelConfigTab == ModelConfigTabClaude {
		sb.WriteString(m.viewModelConfigClaude())
	} else {
		sb.WriteString(m.viewModelConfigOpenCode())
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Description.Render("Tab switch • Esc back"))
	return sb.String()
}

func (m Model) viewModelConfigClaude() string {
	var sb strings.Builder
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

	if m.ModelPreset != "" {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Current: %s\n", styles.Selected.Render(string(m.ModelPreset)))
	}

	return sb.String()
}
