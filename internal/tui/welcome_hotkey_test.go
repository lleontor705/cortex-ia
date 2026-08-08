package tui

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
)

// newWelcomeTestModel returns a Model wired with a real default registry so
// dispatchWelcome's DetectAgents path doesn't nil-deref.
func newWelcomeTestModel() Model {
	return New(agents.NewDefaultRegistry(), "/tmp", "1.0.0")
}

// TestWelcome_HotkeyJumpsToInstall verifies that pressing "1" on the welcome
// screen advances directly to the Detection screen (the Install action target),
// without needing arrow keys + Enter.
func TestWelcome_HotkeyJumpsToInstall(t *testing.T) {
	m := newWelcomeTestModel()
	if m.Screen != ScreenWelcome {
		t.Fatalf("setup: expected starting screen Welcome, got %v", m.Screen)
	}

	result, _ := m.Update(keyMsg("1"))
	rm := result.(Model)
	if rm.Screen != ScreenDetection {
		t.Errorf("after pressing '1', screen = %v, want ScreenDetection", rm.Screen)
	}
}

// TestWelcome_HotkeyQuit verifies that "q" on the welcome screen quits.
func TestWelcome_HotkeyQuit(t *testing.T) {
	m := newWelcomeTestModel()
	result, cmd := m.Update(keyMsg("q"))
	rm := result.(Model)
	if !rm.Quitting {
		t.Error("after pressing 'q', Quitting should be true")
	}
	if cmd == nil {
		t.Error("after pressing 'q', cmd should be tea.Quit")
	}
}

func TestWelcome_RetiredHotkeyCannotRouteAction(t *testing.T) {
	m := newWelcomeTestModel()
	result, cmd := m.Update(keyMsg("7"))
	rm := result.(Model)
	if cmd != nil {
		t.Error("retired hotkey returned a command")
	}
	if rm.Screen != ScreenWelcome {
		t.Errorf("after pressing '7', screen = %v, want ScreenWelcome", rm.Screen)
	}
	if rm.Cursor != 0 {
		t.Errorf("after pressing '7', cursor = %d, want 0", rm.Cursor)
	}
}

func TestWelcome_SupportedLifecycleHotkeys(t *testing.T) {
	tests := []struct {
		key  string
		want Screen
	}{
		{key: "2", want: ScreenSync},
		{key: "3", want: ScreenBackups},
		{key: "4", want: ScreenUpgrade},
		{key: "5", want: ScreenUpgradeSync},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := newWelcomeTestModel()
			result, _ := m.Update(keyMsg(tt.key))
			rm := result.(Model)
			if rm.Screen != tt.want {
				t.Errorf("after pressing %q, screen = %v, want %v", tt.key, rm.Screen, tt.want)
			}
		})
	}
}

// TestWelcome_HomeEndShortcuts verifies g/G jumps the cursor to first/last.
func TestWelcome_HomeEndShortcuts(t *testing.T) {
	m := newWelcomeTestModel()
	m.Cursor = 3

	result, _ := m.Update(keyMsg("g"))
	rm := result.(Model)
	if rm.Cursor != 0 {
		t.Errorf("after 'g', cursor = %d, want 0", rm.Cursor)
	}

	result, _ = rm.Update(keyMsg("G"))
	rm = result.(Model)
	if rm.Cursor != len(welcomeOptions())-1 {
		t.Errorf("after 'G', cursor = %d, want %d", rm.Cursor, len(welcomeOptions())-1)
	}
}
