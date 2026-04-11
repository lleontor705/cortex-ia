package screens

import (
	"strings"
	"testing"
)

func TestRenderWelcome_ContainsVersion(t *testing.T) {
	data := WelcomeData{Version: "1.2.3", Options: []string{"Install"}, Cursor: 0}
	output := RenderWelcome(data)
	if !strings.Contains(output, "1.2.3") {
		t.Error("expected version string in output")
	}
}

func TestRenderWelcome_ContainsOptions(t *testing.T) {
	data := WelcomeData{
		Version: "0.1.0",
		Options: []string{"Fresh Install", "Upgrade", "Backup"},
		Cursor:  0,
	}
	output := RenderWelcome(data)
	for _, opt := range data.Options {
		if !strings.Contains(output, opt) {
			t.Errorf("expected option %q in output", opt)
		}
	}
}

func TestRenderWelcome_CursorPosition(t *testing.T) {
	data := WelcomeData{
		Version: "0.1.0",
		Options: []string{"Alpha", "Beta", "Gamma"},
		Cursor:  1,
	}
	output := RenderWelcome(data)
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Beta") && strings.Contains(line, "> ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cursor indicator on 'Beta' option")
	}
}
