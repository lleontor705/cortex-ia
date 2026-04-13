package tui

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

// screenName returns a human-readable name for the given screen.
func screenName(s Screen) string {
	switch s {
	case ScreenWelcome:
		return "Home"
	case ScreenDetection:
		return "Detection"
	case ScreenAgents:
		return "Agents"
	case ScreenPersona:
		return "Persona"
	case ScreenPreset:
		return "Preset"
	case ScreenClaudeModelPicker:
		return "Model"
	case ScreenSDDMode:
		return "SDD"
	case ScreenStrictTDD:
		return "TDD"
	case ScreenDependencyTree:
		return "Dependencies"
	case ScreenSkillPicker:
		return "Skills"
	case ScreenReview:
		return "Review"
	case ScreenInstalling:
		return "Installing"
	case ScreenComplete:
		return "Complete"
	case ScreenBackups:
		return "Backups"
	case ScreenRenameBackup:
		return "Rename"
	case ScreenUpgrade:
		return "Upgrade"
	case ScreenSync:
		return "Sync"
	case ScreenUpgradeSync:
		return "Upgrade+Sync"
	case ScreenProfiles:
		return "Profiles"
	case ScreenProfileCreate:
		return "Create"
	case ScreenAgentBuilderEngine:
		return "Engine"
	case ScreenAgentBuilderPrompt:
		return "Purpose"
	case ScreenAgentBuilderSDD:
		return "SDD"
	case ScreenAgentBuilderSDDPhase:
		return "Phase"
	case ScreenAgentBuilderGenerating:
		return "Generating"
	case ScreenAgentBuilderPreview:
		return "Preview"
	case ScreenAgentBuilderInstalling:
		return "Installing"
	case ScreenAgentBuilderComplete:
		return "Done"
	case ScreenOpenCodeModels:
		return "OpenCode Models"
	case ScreenOpenCodeModelPicker:
		return "Model"
	}
	return ""
}

// installFlowScreens defines the ordered screens in the main install flow.
var installFlowScreens = []Screen{
	ScreenDetection, ScreenAgents, ScreenPersona, ScreenPreset,
	ScreenClaudeModelPicker, ScreenSDDMode, ScreenStrictTDD,
	ScreenDependencyTree, ScreenSkillPicker, ScreenReview,
	ScreenInstalling, ScreenComplete,
}

// renderBreadcrumb returns a breadcrumb string for the current screen.
// For the install flow, it shows "Step N/12: ScreenName".
// For sub-flows, it shows "Home > Section > Current".
func renderBreadcrumb(current Screen) string {
	// Check if it's part of the install flow
	for i, s := range installFlowScreens {
		if s == current {
			step := i + 1
			total := len(installFlowScreens)
			name := screenName(current)
			return styles.Description.Render(
				fmt.Sprintf("Step %d/%d: %s", step, total, name),
			)
		}
	}

	// Build path from router for non-install-flow screens
	if current == ScreenWelcome {
		return ""
	}

	path := []string{screenName(current)}
	s := current
	for {
		prev, ok := PreviousScreen(s)
		if !ok || prev == ScreenUnknown {
			break
		}
		path = append([]string{screenName(prev)}, path...)
		s = prev
	}

	if len(path) <= 1 {
		return ""
	}

	// Render: dim all but the last segment
	result := ""
	for i, p := range path {
		if i == len(path)-1 {
			result += styles.Subtitle.Render(p)
		} else {
			result += styles.Description.Render(p + " > ")
		}
	}
	return result
}
