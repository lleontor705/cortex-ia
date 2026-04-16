package tui

import "testing"

func TestNextScreen_MainFlow(t *testing.T) {
	chain := []Screen{
		ScreenWelcome,
		ScreenDetection,
		ScreenAgents,
		ScreenPersona,
		ScreenClaudeModelPicker,
		ScreenSkillPicker,
		ScreenReview,
		ScreenInstalling,
		ScreenComplete,
	}

	for i := 0; i < len(chain)-1; i++ {
		got, ok := NextScreen(chain[i])
		if !ok {
			t.Fatalf("NextScreen(%d) returned ok=false", chain[i])
		}
		if got != chain[i+1] {
			t.Errorf("NextScreen(%d) = %d, want %d", chain[i], got, chain[i+1])
		}
	}
}

func TestPreviousScreen_MainFlow(t *testing.T) {
	tests := []struct {
		from, want Screen
	}{
		{ScreenDetection, ScreenWelcome},
		{ScreenAgents, ScreenDetection},
		{ScreenPersona, ScreenAgents},
		{ScreenClaudeModelPicker, ScreenPersona},
		{ScreenSkillPicker, ScreenClaudeModelPicker},
		{ScreenReview, ScreenSkillPicker},
	}

	for _, tt := range tests {
		got, ok := PreviousScreen(tt.from)
		if !ok {
			t.Fatalf("PreviousScreen(%d) returned ok=false", tt.from)
		}
		if got != tt.want {
			t.Errorf("PreviousScreen(%d) = %d, want %d", tt.from, got, tt.want)
		}
	}
}

func TestNextScreen_Unknown(t *testing.T) {
	_, ok := NextScreen(ScreenUnknown)
	if ok {
		t.Error("NextScreen(ScreenUnknown) should return false")
	}
}

func TestPreviousScreen_Welcome(t *testing.T) {
	_, ok := PreviousScreen(ScreenWelcome)
	if ok {
		t.Error("PreviousScreen(ScreenWelcome) should return false")
	}
}

func TestNextScreen_Complete(t *testing.T) {
	_, ok := NextScreen(ScreenComplete)
	if ok {
		t.Error("NextScreen(ScreenComplete) should return false")
	}
}

func TestBackupRoutes(t *testing.T) {
	backupScreens := []struct {
		screen Screen
		want   Screen
	}{
		{ScreenBackups, ScreenWelcome},
		{ScreenRenameBackup, ScreenBackups},
	}

	for _, tt := range backupScreens {
		got, ok := PreviousScreen(tt.screen)
		if !ok {
			t.Fatalf("PreviousScreen(%d) returned ok=false", tt.screen)
		}
		if got != tt.want {
			t.Errorf("PreviousScreen(%d) = %d, want %d", tt.screen, got, tt.want)
		}
	}
}

func TestAgentBuilderRoutes(t *testing.T) {
	// Forward chain
	got, ok := NextScreen(ScreenAgentBuilderPrompt)
	if !ok || got != ScreenAgentBuilderSDD {
		t.Errorf("NextScreen(AgentBuilderPrompt) = %d, %v; want %d, true",
			got, ok, ScreenAgentBuilderSDD)
	}

	// Backward chain
	backTests := []struct {
		from, want Screen
	}{
		{ScreenAgentBuilderEngine, ScreenWelcome},
		{ScreenAgentBuilderPrompt, ScreenAgentBuilderEngine},
		{ScreenAgentBuilderSDD, ScreenAgentBuilderPrompt},
		{ScreenAgentBuilderGenerating, ScreenAgentBuilderPrompt},
		{ScreenAgentBuilderPreview, ScreenAgentBuilderPrompt},
		{ScreenAgentBuilderComplete, ScreenWelcome},
	}

	for _, tt := range backTests {
		got, ok := PreviousScreen(tt.from)
		if !ok {
			t.Fatalf("PreviousScreen(%d) returned ok=false", tt.from)
		}
		if got != tt.want {
			t.Errorf("PreviousScreen(%d) = %d, want %d", tt.from, got, tt.want)
		}
	}
}

func TestMaintenanceRoutes(t *testing.T) {
	got, ok := PreviousScreen(ScreenMaintenance)
	if !ok || got != ScreenWelcome {
		t.Errorf("PreviousScreen(Maintenance) = %d, %v; want ScreenWelcome", got, ok)
	}

	got, ok = PreviousScreen(ScreenProfileCreate)
	if !ok || got != ScreenMaintenance {
		t.Errorf("PreviousScreen(ProfileCreate) = %d, %v; want ScreenMaintenance", got, ok)
	}
}

func TestModelConfigRoutes(t *testing.T) {
	got, ok := PreviousScreen(ScreenModelConfig)
	if !ok || got != ScreenWelcome {
		t.Errorf("PreviousScreen(ModelConfig) = %d, %v; want ScreenWelcome", got, ok)
	}

	got, ok = PreviousScreen(ScreenOpenCodeModelPicker)
	if !ok || got != ScreenModelConfig {
		t.Errorf("PreviousScreen(OpenCodeModelPicker) = %d, %v; want ScreenModelConfig", got, ok)
	}
}
