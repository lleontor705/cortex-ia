package tui

// Route defines the forward and backward transitions for a screen.
type Route struct {
	Forward  Screen
	Backward Screen
}

var linearRoutes = map[Screen]Route{
	// Main install flow
	ScreenWelcome:           {Forward: ScreenDetection},
	ScreenDetection:         {Forward: ScreenAgents, Backward: ScreenWelcome},
	ScreenAgents:            {Forward: ScreenPersona, Backward: ScreenDetection},
	ScreenPersona:           {Forward: ScreenPreset, Backward: ScreenAgents},
	ScreenPreset:            {Forward: ScreenClaudeModelPicker, Backward: ScreenPersona},
	ScreenClaudeModelPicker: {Forward: ScreenSDDMode, Backward: ScreenPreset},
	ScreenSDDMode:           {Forward: ScreenStrictTDD, Backward: ScreenClaudeModelPicker},
	ScreenStrictTDD:         {Forward: ScreenDependencyTree, Backward: ScreenSDDMode},
	ScreenDependencyTree:    {Forward: ScreenSkillPicker, Backward: ScreenStrictTDD},
	ScreenSkillPicker:       {Forward: ScreenReview, Backward: ScreenDependencyTree},
	ScreenReview:            {Forward: ScreenInstalling, Backward: ScreenSkillPicker},
	ScreenInstalling:        {Forward: ScreenComplete},
	ScreenComplete:          {},

	// Backup management
	ScreenBackups:      {Backward: ScreenWelcome},
	ScreenRenameBackup: {Backward: ScreenBackups},

	// Post-install operations
	ScreenUpgrade:     {Backward: ScreenWelcome},
	ScreenSync:        {Backward: ScreenWelcome},
	ScreenUpgradeSync: {Backward: ScreenWelcome},

	// Profile management
	ScreenProfiles:      {Backward: ScreenWelcome},
	ScreenProfileCreate: {Backward: ScreenProfiles},

	// Agent builder
	ScreenAgentBuilderEngine:     {Backward: ScreenWelcome},
	ScreenAgentBuilderPrompt:     {Forward: ScreenAgentBuilderSDD, Backward: ScreenAgentBuilderEngine},
	ScreenAgentBuilderSDD:        {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderSDDPhase:   {Backward: ScreenAgentBuilderSDD},
	ScreenAgentBuilderGenerating: {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderPreview:    {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderInstalling: {Forward: ScreenAgentBuilderComplete},
	ScreenAgentBuilderComplete:   {Backward: ScreenWelcome},

	// OpenCode model configuration
	ScreenOpenCodeModels:      {Backward: ScreenWelcome},
	ScreenOpenCodeModelPicker: {Backward: ScreenOpenCodeModels},
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
