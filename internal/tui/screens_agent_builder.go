package tui

// Agent Builder screens: Engine, Prompt, SDD, SDDPhase, Generating,
// Preview, Installing, Complete.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/agentbuilder"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

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

// --- Agent Builder: Prompt ---

func (m Model) updateAgentBuilderPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.setScreen(ScreenAgentBuilderEngine)
			return m, nil
		case "ctrl+enter":
			if m.AgentBuilderTextArea.Value() != "" {
				m.setScreen(ScreenAgentBuilderSDD)
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
	sb.WriteString("What should this agent do?\n\n")
	sb.WriteString(m.AgentBuilderTextArea.View())
	sb.WriteString("\n\n")
	sb.WriteString(styles.Description.Render("Press "))
	sb.WriteString(styles.Subtitle.Render("Ctrl+Enter"))
	sb.WriteString(styles.Description.Render(" to continue, "))
	sb.WriteString(styles.Subtitle.Render("Esc"))
	sb.WriteString(styles.Description.Render(" to go back"))
	return sb.String()
}

// --- Agent Builder: SDD ---

func (m Model) updateAgentBuilderSDD(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k", "down", "j":
			switch m.AgentBuilderSDDMode {
			case "phase":
				m.AgentBuilderSDDMode = "full"
			case "full":
				m.AgentBuilderSDDMode = "none"
			default:
				m.AgentBuilderSDDMode = "phase"
			}
		case "enter":
			if m.AgentBuilderSDDMode == "phase" {
				m.setScreen(ScreenAgentBuilderSDDPhase)
			} else {
				m.setScreen(ScreenAgentBuilderGenerating)
				m.OperationRunning = true
				return m, m.agentBuilderGenerateCmd()
			}
		case "esc":
			m.setScreen(ScreenAgentBuilderPrompt)
		}
	}
	return m, nil
}

func (m Model) viewAgentBuilderSDD() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Agent — SDD Integration"))
	sb.WriteString("\n\n")

	modes := []struct {
		id   string
		name string
		desc string
	}{
		{"full", "Full SDD", "Agent participates in all SDD phases"},
		{"phase", "Single Phase", "Agent specializes in one SDD phase"},
		{"none", "No SDD", "Standalone agent without SDD integration"},
	}

	for _, mode := range modes {
		cursor := "  "
		selected := m.AgentBuilderSDDMode == mode.id
		if selected {
			cursor = styles.Cursor.Render("> ")
		}
		marker := "( )"
		if selected {
			marker = styles.Selected.Render("(*)")
		}
		fmt.Fprintf(&sb, "%s%s %s %s\n", cursor, marker,
			styles.Subtitle.Render(mode.name),
			styles.Description.Render("— "+mode.desc))
	}

	return sb.String()
}

// --- Agent Builder: SDD Phase ---

func (m Model) updateAgentBuilderSDDPhase(msg tea.Msg) (tea.Model, tea.Cmd) {
	phases := []string{"init", "explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(phases)-1 {
				m.Cursor++
			}
		case "enter":
			m.AgentBuilderSDDPhase = phases[m.Cursor]
			m.setScreen(ScreenAgentBuilderGenerating)
			m.OperationRunning = true
			return m, m.agentBuilderGenerateCmd()
		case "esc":
			m.setScreen(ScreenAgentBuilderSDD)
		}
	}
	return m, nil
}

func (m Model) viewAgentBuilderSDDPhase() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Create Agent — Select SDD Phase"))
	sb.WriteString("\n\n")

	phases := []string{"init", "explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}
	for i, phase := range phases {
		cursor := "  "
		if i == m.Cursor {
			cursor = styles.Cursor.Render("> ")
		}
		fmt.Fprintf(&sb, "%s%s\n", cursor, phase)
	}

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

// --- Agent Builder: Preview ---

func (m Model) updateAgentBuilderPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.setScreen(ScreenAgentBuilderInstalling)
			m.OperationRunning = true
			return m, m.agentBuilderInstallCmd()
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
	sb.WriteString(styles.Title.Render("Agent Preview"))
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
	return sb.String()
}

// --- Agent Builder: Installing ---

func (m Model) updateAgentBuilderInstalling(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case AgentBuilderInstallDoneMsg:
		m.OperationRunning = false
		m.AgentBuilderErr = v.Err
		m.setScreen(ScreenAgentBuilderComplete)
	}
	return m, nil
}

func (m Model) viewAgentBuilderInstalling() string {
	var sb strings.Builder
	sb.WriteString(styles.Title.Render(fmt.Sprintf("%s Installing Agent...", m.Spinner.View())))
	sb.WriteString("\n\n")
	sb.WriteString(styles.Description.Render("Please wait..."))
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

// agentBuilderGenerateCmd returns a tea.Cmd that generates the agent config.
func (m Model) agentBuilderGenerateCmd() tea.Cmd {
	spec := agentbuilder.AgentSpec{
		Engine:   m.AgentBuilderEngine,
		Purpose:  m.AgentBuilderTextArea.Value(),
		SDDMode:  agentbuilder.SDDIntegrationMode(m.AgentBuilderSDDMode),
		SDDPhase: m.AgentBuilderSDDPhase,
	}
	return func() tea.Msg {
		agent, err := agentbuilder.Generate(spec)
		if err != nil {
			return AgentBuilderGeneratedMsg{Err: err}
		}
		return agentBuilderGeneratedResultMsg{Agent: agent}
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
