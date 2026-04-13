package tui

import "testing"

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()
	if len(km.Up.Keys()) == 0 {
		t.Error("Up should have key bindings")
	}
	if len(km.Down.Keys()) == 0 {
		t.Error("Down should have key bindings")
	}
	if len(km.Enter.Keys()) == 0 {
		t.Error("Enter should have key bindings")
	}
	if len(km.Filter.Keys()) == 0 {
		t.Error("Filter should have key bindings")
	}
}

func TestScreenKeyMap_Welcome(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenWelcome
	km := m.screenKeyMap()
	if _, ok := km.(WelcomeKeyMap); !ok {
		t.Errorf("Welcome should return WelcomeKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_Agents(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenAgents
	km := m.screenKeyMap()
	if _, ok := km.(CheckboxKeyMap); !ok {
		t.Errorf("Agents should return CheckboxKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_SkillPicker(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenSkillPicker
	km := m.screenKeyMap()
	if _, ok := km.(CheckboxKeyMap); !ok {
		t.Errorf("SkillPicker should return CheckboxKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_Backups(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenBackups
	km := m.screenKeyMap()
	if _, ok := km.(BackupKeyMap); !ok {
		t.Errorf("Backups should return BackupKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_Profiles(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenProfiles
	km := m.screenKeyMap()
	if _, ok := km.(ProfileKeyMap); !ok {
		t.Errorf("Profiles should return ProfileKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_AgentBuilderPrompt(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenAgentBuilderPrompt
	km := m.screenKeyMap()
	if _, ok := km.(InputKeyMap); !ok {
		t.Errorf("AgentBuilderPrompt should return InputKeyMap, got %T", km)
	}
}

func TestScreenKeyMap_DefaultScreen(t *testing.T) {
	m := New(nil, "/tmp", "1.0.0")
	m.Screen = ScreenDetection
	km := m.screenKeyMap()
	if _, ok := km.(NavigateKeyMap); !ok {
		t.Errorf("Detection should return NavigateKeyMap, got %T", km)
	}
}

func TestCheckboxKeyMap_ShortHelp(t *testing.T) {
	km := CheckboxKeyMap{}
	help := km.ShortHelp()
	if len(help) == 0 {
		t.Error("ShortHelp should return bindings")
	}
}

func TestNavigateKeyMap_FullHelp(t *testing.T) {
	km := NavigateKeyMap{}
	help := km.FullHelp()
	if len(help) == 0 {
		t.Error("FullHelp should return binding groups")
	}
}
