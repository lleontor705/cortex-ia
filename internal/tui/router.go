package tui

// Route defines the forward and backward transitions for a screen.
type Route struct {
	Forward  Screen
	Backward Screen
}

var linearRoutes = map[Screen]Route{
	// Main install flow (simplified: 8 steps)
	ScreenWelcome:           {Forward: ScreenDetection},
	ScreenDetection:         {Forward: ScreenAgents, Backward: ScreenWelcome},
	ScreenAgents:            {Forward: ScreenPersona, Backward: ScreenDetection},
	ScreenPersona:           {Forward: ScreenClaudeModelPicker, Backward: ScreenAgents},
	ScreenClaudeModelPicker: {Forward: ScreenSkillPicker, Backward: ScreenPersona},
	ScreenSkillPicker:       {Forward: ScreenReview, Backward: ScreenClaudeModelPicker},
	ScreenReview:            {Forward: ScreenInstalling, Backward: ScreenSkillPicker},
	ScreenInstalling:        {Forward: ScreenComplete},
	ScreenComplete:          {},

	// Backup management
	ScreenBackups:      {Backward: ScreenWelcome},
	ScreenRenameBackup: {Backward: ScreenBackups},

	// Maintenance (upgrade, sync, profiles)
	ScreenMaintenance:   {Backward: ScreenWelcome},
	ScreenProfileCreate: {Backward: ScreenMaintenance},

	// Agent builder (simplified: 5 screens)
	ScreenAgentBuilderEngine:     {Backward: ScreenWelcome},
	ScreenAgentBuilderPrompt:     {Forward: ScreenAgentBuilderSDD, Backward: ScreenAgentBuilderEngine},
	ScreenAgentBuilderSDD:        {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderGenerating: {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderPreview:    {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderComplete:   {Backward: ScreenWelcome},

	// Model configuration (Claude + OpenCode unified)
	ScreenModelConfig:         {Backward: ScreenWelcome},
	ScreenOpenCodeModelPicker: {Backward: ScreenModelConfig},
}

// NextScreen returns the forward screen for the given screen, if defined.
func NextScreen(screen Screen) (Screen, bool) {
	route, ok := linearRoutes[screen]
	if !ok || route.Forward == ScreenUnknown {
		return ScreenUnknown, false
	}
	return route.Forward, true
}

// PreviousScreen returns the backward screen for the given screen, if defined.
func PreviousScreen(screen Screen) (Screen, bool) {
	route, ok := linearRoutes[screen]
	if !ok || route.Backward == ScreenUnknown {
		return ScreenUnknown, false
	}
	return route.Backward, true
}
