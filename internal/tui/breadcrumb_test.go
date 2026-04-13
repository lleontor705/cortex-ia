package tui

import (
	"strings"
	"testing"
)

func TestScreenName_AllScreensHaveNames(t *testing.T) {
	screens := []Screen{
		ScreenWelcome, ScreenDetection, ScreenAgents, ScreenPersona,
		ScreenPreset, ScreenClaudeModelPicker, ScreenSDDMode, ScreenStrictTDD,
		ScreenDependencyTree, ScreenSkillPicker, ScreenReview, ScreenInstalling,
		ScreenComplete, ScreenBackups,
		ScreenRenameBackup, ScreenUpgrade, ScreenSync, ScreenProfiles,
		ScreenAgentBuilderEngine, ScreenAgentBuilderPrompt,
	}
	for _, s := range screens {
		name := screenName(s)
		if name == "" {
			t.Errorf("screenName(%d) returned empty string", s)
		}
	}
}

func TestRenderBreadcrumb_WelcomeReturnsEmpty(t *testing.T) {
	bc := renderBreadcrumb(ScreenWelcome)
	if bc != "" {
		t.Errorf("welcome breadcrumb should be empty, got %q", bc)
	}
}

func TestRenderBreadcrumb_InstallFlow(t *testing.T) {
	bc := renderBreadcrumb(ScreenAgents)
	if !strings.Contains(bc, "Step") {
		t.Errorf("install flow breadcrumb should contain 'Step', got %q", bc)
	}
	if !strings.Contains(bc, "Agents") {
		t.Errorf("install flow breadcrumb should contain screen name, got %q", bc)
	}
}

func TestRenderBreadcrumb_SubFlow(t *testing.T) {
	bc := renderBreadcrumb(ScreenBackups)
	if bc == "" {
		t.Error("sub-flow breadcrumb should not be empty")
	}
	if !strings.Contains(bc, "Backups") {
		t.Errorf("sub-flow breadcrumb should contain 'Backups', got %q", bc)
	}
}

func TestRenderBreadcrumb_DeepSubFlow(t *testing.T) {
	bc := renderBreadcrumb(ScreenRenameBackup)
	if !strings.Contains(bc, "Rename") {
		t.Errorf("deep sub-flow breadcrumb should contain 'Rename', got %q", bc)
	}
}
