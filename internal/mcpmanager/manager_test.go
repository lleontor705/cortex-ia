package mcpmanager

import (
	"testing"
)

func TestPresets(t *testing.T) {
	presets := Presets()
	if len(presets) == 0 {
		t.Fatal("expected at least 1 preset")
	}

	// Lookup cortex
	cortexPreset, found := Lookup("cortex")
	if !found {
		t.Fatal("expected 'cortex' preset to be found")
	}
	if cortexPreset.Name != "cortex" {
		t.Errorf("expected name 'cortex', got %s", cortexPreset.Name)
	}
	cmd, ok := cortexPreset.Command()
	if !ok || len(cmd) == 0 {
		t.Errorf("expected valid command vector for cortex preset: %+v", cmd)
	}

	// Lookup retired
	retired := RetiredPresets()
	if len(retired) == 0 {
		t.Fatal("expected at least 1 retired preset")
	}
	if !IsRetired("forgespec") {
		t.Error("expected forgespec to be recognized as retired")
	}

	// Default selection
	defaults := DefaultSelection()
	if len(defaults) == 0 {
		t.Fatal("expected non-empty default selection")
	}
}

func TestDesiredValidation(t *testing.T) {
	// 1. Valid preset
	desiredPreset := Desired{
		Name:   "cortex",
		Kind:   DesiredPreset,
		Preset: "cortex",
	}
	if err := desiredPreset.Validate(); err != nil {
		t.Fatalf("expected valid desired preset, got error: %v", err)
	}
	entry, err := desiredPreset.Entry()
	if err != nil {
		t.Fatalf("Entry() failed: %v", err)
	}
	if entry["type"] != "local" {
		t.Errorf("expected entry type local, got: %v", entry["type"])
	}

	// 2. Valid custom local
	desiredLocal := Desired{
		Name:    "my-local-mcp",
		Kind:    DesiredLocal,
		Command: []string{"node", "server.js"},
		Env:     []string{"PORT=8080"},
	}
	if err := desiredLocal.Validate(); err != nil {
		t.Fatalf("expected valid desired local, got error: %v", err)
	}

	// 3. Valid custom remote
	desiredRemote := Desired{
		Name:    "my-remote-mcp",
		Kind:    DesiredRemote,
		URL:     "https://mcp.example.com/sse",
		Headers: []string{"Authorization=Bearer test"},
	}
	if err := desiredRemote.Validate(); err != nil {
		t.Fatalf("expected valid desired remote, got error: %v", err)
	}

	// 4. Invalid name
	invalidName := desiredLocal
	invalidName.Name = "invalid name with spaces!"
	if err := invalidName.Validate(); err == nil {
		t.Error("expected error for invalid MCP name")
	}
}
