package tui

// Agent Builder screens (simplified): Engine, Prompt, SDD, Generating, Preview+Install, Complete.

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/agentbuilder"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// SDD integration mode constants.
const (
	sddModeFull  = "full"
	sddModePhase = "phase"
	sddModeNone  = "none"
)

// sddPhases is the ordered list of SDD phases for the phase picker.
var sddPhases = []string{"init", "explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}

// --- Agent Builder: Engine ---

func (m Model) updateAgentBuilderEngine(msg tea.Msg) (tea.Model, tea.Cmd) {
	agents := m.availableEngines()
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
				m.AgentBuilderEngine = agents[m.Cursor]
				m.AgentBuilderTextArea.Focus()
				m.setScreen(ScreenAgentBuilderPrompt)
			}
		case "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

func (m Model) availableEngines() []model.AgentID {
	return []model.AgentID{
		model.AgentClaudeCode,
		model.AgentOpenCode,
		model.AgentGeminiCLI,
		model.AgentCursor,
		model.AgentVSCodeCopilot,
		model.AgentCodex,
		model.AgentWindsurf,
		model.AgentAntigravity,
	}
}

// engineDescription returns a short description for each engine.
func engineDescription(engine model.AgentID) string {
	switch engine {
	case model.AgentClaudeCode:
		return "Anthropic's CLI coding agent"
	case model.AgentOpenCode:
		return "Open-source terminal coding agent"
	case model.AgentGeminiCLI:
		return "Google's Gemini CLI agent"
	case model.AgentCursor:
		return "AI-powered code editor"
	case model.AgentVSCodeCopilot:
		return "GitHub Copilot in VS Code"
	case model.AgentCodex:
		return "OpenAI's Codex CLI agent"
	case model.AgentWindsurf:
		return "Codeium's AI code editor"
	case model.AgentAntigravity:
		return "Antigravity AI agent"
	}
	return ""
}

func (m Model) viewAgentBuilderEngine() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Agent — Select Engine"))
	sb.WriteString("\n\n")

	for i, engine := range m.availableEngines() {
		cursor := "  "
		if i == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}
		name := styles.Subtitle.Render(string(engine))
		desc := styles.Description.Render(" — " + engineDescription(engine))
		fmt.Fprintf(&sb, "%s%s%s\n", cursor, name, desc)
	}

	return sb.String()
}

// agentTemplate defines a pre-built agent purpose template.
type agentTemplate struct {
	Name    string
	Purpose string
}

var agentTemplates = []agentTemplate{
	{"Code Reviewer", "Review code changes for bugs, security issues, and best practices. Provide actionable feedback with severity levels."},
	{"Test Generator", "Generate comprehensive test suites for existing code. Cover edge cases, error paths, and integration scenarios."},
	{"Documentation Writer", "Generate and maintain documentation from code. Create READMEs, API docs, and inline comments."},
	{"Refactoring Assistant", "Identify code smells and refactoring opportunities. Suggest improvements while preserving behavior."},
}

// --- Agent Builder: Prompt ---

func (m Model) updateAgentBuilderPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.setScreen(ScreenAgentBuilderEngine)
			return m, nil
		case "enter":
			if m.AgentBuilderTextArea.Value() != "" {
				m.setScreen(ScreenAgentBuilderSDD)
			}
			return m, nil
		case "1", "2", "3", "4":
			// Quick template selection
			idx := int(key.Runes[0]-'1')
			if idx >= 0 && idx < len(agentTemplates) {
				m.AgentBuilderTextArea.SetValue(agentTemplates[idx].Purpose)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.AgentBuilderTextArea, cmd = m.AgentBuilderTextArea.Update(msg)
	return m, cmd
}

func (m Model) viewAgentBuilderPrompt() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Agent — Describe Purpose"))
	sb.WriteString("\n\n")

	// Show templates
	sb.WriteString(styles.Subtitle.Render("Templates:"))
	sb.WriteString("\n")
	for i, t := range agentTemplates {
		fmt.Fprintf(&sb, "  %s %s %s\n",
			styles.Subtitle.Render(fmt.Sprintf("[%d]", i+1)),
			t.Name,
			styles.Description.Render("— "+t.Purpose[:min(50, len(t.Purpose))]))
	}
	sb.WriteString("\n")

	sb.WriteString("Or describe a custom purpose:\n\n")
	sb.WriteString(m.AgentBuilderTextArea.View())
	sb.WriteString("\n\n")
	sb.WriteString(styles.Description.Render("1-4 template • Enter continue • Esc back"))
	return sb.String()
}

// --- Agent Builder: SDD (merged SDD + Phase selection) ---

func (m Model) updateAgentBuilderSDD(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			totalItems := 3 // full, single-phase (collapsed), none
			if m.AgentBuilderSDDMode == sddModePhase {
				totalItems = 2 + len(sddPhases) + 1 // full + header + phases + none
			}
			if m.Cursor < totalItems-1 {
				m.Cursor++
			}
		case "enter":
			switch {
			case m.Cursor == 0:
				// Full SDD
				m.AgentBuilderSDDMode = sddModeFull
				m.AgentBuilderSDDPhase = ""
				m.setScreen(ScreenAgentBuilderGenerating)
				m.OperationRunning = true
				return m, m.agentBuilderGenerateCmd()
			case m.Cursor == 1 && m.AgentBuilderSDDMode != sddModePhase:
				// Expand phase list
				m.AgentBuilderSDDMode = sddModePhase
				m.Cursor = 2 // move to first phase
				return m, nil
			case m.Cursor == 1 && m.AgentBuilderSDDMode == sddModePhase:
				// Collapse phase list
				m.AgentBuilderSDDMode = ""
				return m, nil
			case m.AgentBuilderSDDMode == sddModePhase && m.Cursor >= 2 && m.Cursor <= len(sddPhases)+1:
				// Select specific phase
				m.AgentBuilderSDDPhase = sddPhases[m.Cursor-2]
				m.setScreen(ScreenAgentBuilderGenerating)
				m.OperationRunning = true
				return m, m.agentBuilderGenerateCmd()
			default:
				// No SDD (last item)
				m.AgentBuilderSDDMode = sddModeNone
				m.AgentBuilderSDDPhase = ""
				m.setScreen(ScreenAgentBuilderGenerating)
				m.OperationRunning = true
				return m, m.agentBuilderGenerateCmd()
			}
		case "esc":
			m.AgentBuilderSDDMode = ""
			m.setScreen(ScreenAgentBuilderPrompt)
		}
	}
	return m, nil
}

func (m Model) viewAgentBuilderSDD() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Agent — SDD Integration"))
	sb.WriteString("\n\n")

	idx := 0

	// Full SDD option
	cursor := "  "
	if idx == m.Cursor {
		cursor = styles.Cursor.Render("> ")
	}
	fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render("Full SDD"),
		styles.Description.Render("— Agent participates in all SDD phases"))
	idx++

	if m.AgentBuilderSDDMode == sddModePhase {
		// Expanded: show ▼ indicator + phase list
		cursor = "  "
		if idx == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render("▼ Single Phase"),
			styles.Description.Render("— Select a phase below"))
		idx++
		for _, phase := range sddPhases {
			cursor = "  "
			if idx == m.Cursor {
				cursor = styles.Cursor.Render("> ")
			}
			fmt.Fprintf(&sb, "    %s%s\n", cursor, phase)
			idx++
		}
	} else {
		// Collapsed: show ▶ indicator
		cursor = "  "
		if idx == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render("▶ Single Phase"),
			styles.Description.Render("— Agent specializes in one SDD phase"))
		idx++
	}

	// No SDD option
	cursor = "  "
	if idx == m.Cursor {
		cursor = styles.Cursor.Render("> ")
	}
	fmt.Fprintf(&sb, "%s%s %s\n", cursor, styles.Subtitle.Render("No SDD"),
		styles.Description.Render("— Standalone agent without SDD integration"))

	return sb.String()
}

// --- Agent Builder: Generating ---

func (m Model) updateAgentBuilderGenerating(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case AgentBuilderGeneratedMsg:
		m.OperationRunning = false
		m.AgentBuilderErr = v.Err
		m.setScreen(ScreenAgentBuilderComplete)
	case agentBuilderGeneratedResultMsg:
		m.OperationRunning = false
		m.AgentBuilderGenerated = v.Agent
		if v.Agent != nil {
			m.AgentBuilderViewport.SetContent(v.Agent.SkillContent)
			m.AgentBuilderViewport.GotoTop()
		}
		m.setScreen(ScreenAgentBuilderPreview)
	}
	return m, nil
}

func (m Model) viewAgentBuilderGenerating() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("%s Generating Agent...", m.Spinner.View())))
	sb.WriteString("\n\n")
	sb.WriteString("Creating agent configuration based on your description.\n")
	return sb.String()
}

// --- Agent Builder: Preview (with Install action) ---

func (m Model) updateAgentBuilderPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case AgentBuilderInstallDoneMsg:
		m.OperationRunning = false
		m.AgentBuilderErr = v.Err
		m.setScreen(ScreenAgentBuilderComplete)
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if !m.OperationRunning {
				m.OperationRunning = true
				return m, m.agentBuilderInstallCmd()
			}
		case "esc":
			m.setScreen(ScreenAgentBuilderPrompt)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.AgentBuilderViewport, cmd = m.AgentBuilderViewport.Update(msg)
	return m, cmd
}

func (m Model) viewAgentBuilderPreview() string {
	var sb strings.Builder
	if m.OperationRunning {
		sb.WriteString(styles.Title.Render(fmt.Sprintf("%s Installing Agent...", m.Spinner.View())))
	} else {
		sb.WriteString(styles.Title.Render("Agent Preview"))
	}
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Engine: %s\n", styles.Subtitle.Render(string(m.AgentBuilderEngine)))
	fmt.Fprintf(&sb, "Purpose: %s\n", m.AgentBuilderTextArea.Value())
	fmt.Fprintf(&sb, "SDD Mode: %s\n", m.AgentBuilderSDDMode)
	if m.AgentBuilderSDDPhase != "" {
		fmt.Fprintf(&sb, "SDD Phase: %s\n", m.AgentBuilderSDDPhase)
	}
	if m.AgentBuilderGenerated != nil {
		fmt.Fprintf(&sb, "Skill Name: %s\n", styles.Subtitle.Render(m.AgentBuilderGenerated.SkillName))
		sb.WriteString("\n--- SKILL.md Preview ---\n\n")
		sb.WriteString(m.AgentBuilderViewport.View())
		sb.WriteString("\n")
		sb.WriteString(styles.Description.Render(fmt.Sprintf("  %d%%", int(m.AgentBuilderViewport.ScrollPercent()*100))))
	}
	if !m.OperationRunning {
		sb.WriteString("\n\n")
		sb.WriteString(styles.Description.Render("Enter install • Esc back"))
	}
	return sb.String()
}

// --- Agent Builder: Complete ---

func (m Model) updateAgentBuilderComplete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "esc":
			m.setScreen(ScreenWelcome)
		}
	}
	return m, nil
}

// agentBuilderGenerateCmd returns a tea.Cmd that generates the agent config with a 30s timeout.
func (m Model) agentBuilderGenerateCmd() tea.Cmd {
	spec := agentbuilder.AgentSpec{
		Engine:   m.AgentBuilderEngine,
		Purpose:  m.AgentBuilderTextArea.Value(),
		SDDMode:  agentbuilder.SDDIntegrationMode(m.AgentBuilderSDDMode),
		SDDPhase: m.AgentBuilderSDDPhase,
	}
	return func() tea.Msg {
		type result struct {
			agent *agentbuilder.GeneratedAgent
			err   error
		}
		ch := make(chan result, 1)
		go func() {
			agent, err := agentbuilder.Generate(spec)
			ch <- result{agent, err}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				return AgentBuilderGeneratedMsg{Err: r.err}
			}
			return agentBuilderGeneratedResultMsg{Agent: r.agent}
		case <-time.After(30 * time.Second):
			return AgentBuilderGeneratedMsg{Err: fmt.Errorf("generation timed out after 30s")}
		}
	}
}

// agentBuilderGeneratedResultMsg carries the generated agent back to the model.
type agentBuilderGeneratedResultMsg struct {
	Agent *agentbuilder.GeneratedAgent
}

// agentBuilderInstallCmd returns a tea.Cmd that installs the generated agent.
func (m Model) agentBuilderInstallCmd() tea.Cmd {
	agent := m.AgentBuilderGenerated
	homeDir := m.HomeDir
	return func() tea.Msg {
		if agent == nil {
			return AgentBuilderInstallDoneMsg{Err: fmt.Errorf("no generated agent to install")}
		}
		_, err := agentbuilder.Install(homeDir, agent)
		return AgentBuilderInstallDoneMsg{Err: err}
	}
}

func (m Model) viewAgentBuilderComplete() string {
	var sb strings.Builder
	if m.AgentBuilderErr != nil {
		sb.WriteString(styles.StatusFail.Render("Agent Installation Failed"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Error: %v\n", m.AgentBuilderErr)
	} else {
		sb.WriteString(styles.StatusOK.Render("✓ Agent Created"))
		sb.WriteString("\n\n")
		fmt.Fprintf(&sb, "Agent for %s has been configured.\n", styles.Subtitle.Render(string(m.AgentBuilderEngine)))
		if m.AgentBuilderGenerated != nil && m.AgentBuilderGenerated.SkillName != "" {
			fmt.Fprintf(&sb, "\nSkill: %s\n", styles.Subtitle.Render(m.AgentBuilderGenerated.SkillName))
			fmt.Fprintf(&sb, "Path:  %s\n", styles.Description.Render(
				fmt.Sprintf("~/.claude/skills/%s/SKILL.md", m.AgentBuilderGenerated.SkillName)))
		}
	}
	return sb.String()
}
