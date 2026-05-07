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

func TestRenderWelcome_GroupsRenderTitlesAndHints(t *testing.T) {
	data := WelcomeData{
		Version: "0.3.0",
		Groups: []MenuGroup{
			{Title: "SETUP", Items: []MenuItem{
				{Hotkey: "1", Label: "Install ecosystem", Hint: "Detect agents"},
			}},
			{Title: "MAINTAIN", Items: []MenuItem{
				{Hotkey: "2", Label: "Manage backups", Hint: "Browse snapshots"},
			}},
		},
		Cursor: 0,
	}
	output := RenderWelcome(data)
	for _, want := range []string{"SETUP", "MAINTAIN", "[1]", "[2]", "Install ecosystem", "Detect agents"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestRenderWelcome_GroupsCursorOnSecondGroup(t *testing.T) {
	data := WelcomeData{
		Version: "0.3.0",
		Groups: []MenuGroup{
			{Items: []MenuItem{{Hotkey: "1", Label: "First"}}},
			{Items: []MenuItem{{Hotkey: "2", Label: "Second"}}},
		},
		Cursor: 1, // global index → second group, first item
	}
	output := RenderWelcome(data)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Second") && strings.Contains(line, "> ") {
			return
		}
	}
	t.Errorf("cursor did not land on Second across groups\n%s", output)
}

func TestFindItemByHotkey(t *testing.T) {
	groups := []MenuGroup{
		{Items: []MenuItem{{Hotkey: "1", Label: "A"}, {Hotkey: "2", Label: "B"}}},
		{Items: []MenuItem{{Hotkey: "q", Label: "Quit"}}},
	}
	if idx, ok := FindItemByHotkey(groups, "2"); !ok || idx != 1 {
		t.Errorf("FindItemByHotkey(2) = (%d, %v), want (1, true)", idx, ok)
	}
	if idx, ok := FindItemByHotkey(groups, "q"); !ok || idx != 2 {
		t.Errorf("FindItemByHotkey(q) = (%d, %v), want (2, true)", idx, ok)
	}
	if _, ok := FindItemByHotkey(groups, "z"); ok {
		t.Error("FindItemByHotkey(z) should not match")
	}
}

func TestFlattenMenu(t *testing.T) {
	groups := []MenuGroup{
		{Items: []MenuItem{{Label: "A"}}},
		{Items: []MenuItem{{Label: "B"}, {Label: "C"}}},
	}
	flat := FlattenMenu(groups)
	if len(flat) != 3 || flat[0].Label != "A" || flat[2].Label != "C" {
		t.Errorf("FlattenMenu = %v", flat)
	}
}

func TestRenderHelpBar(t *testing.T) {
	got := RenderHelpBar("enter select", "esc back")
	if !strings.Contains(got, "enter select") || !strings.Contains(got, "esc back") {
		t.Errorf("help bar missing bindings: %s", got)
	}
	if RenderHelpBar() != "" {
		t.Error("empty input should return empty string")
	}
}
