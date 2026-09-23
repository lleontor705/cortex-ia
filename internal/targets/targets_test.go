package targets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTargets(t *testing.T) {
	cases := []struct {
		input    string
		expected []TargetID
		wantErr  bool
	}{
		{"", []TargetID{TargetOpenCode, TargetClaude}, false},
		{"all", []TargetID{TargetOpenCode, TargetClaude}, false},
		{"opencode", []TargetID{TargetOpenCode}, false},
		{"claude", []TargetID{TargetClaude}, false},
		{"opencode,claude", []TargetID{TargetOpenCode, TargetClaude}, false},
		{"agy", nil, true},
		{"opencode,agy", nil, true},
		{"invalid", nil, true},
	}

	for _, tc := range cases {
		got, err := ParseTargets(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseTargets(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && len(got) != len(tc.expected) {
			t.Errorf("ParseTargets(%q) len = %d, want %d", tc.input, len(got), len(tc.expected))
		}
	}
}

func TestInstallAndUninstallClaude(t *testing.T) {
	tempHome := t.TempDir()

	res, err := InstallClaude(tempHome, false)
	if err != nil {
		t.Fatalf("InstallClaude failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success on InstallClaude")
	}

	claudeFile := filepath.Join(tempHome, ".claude.json")
	raw, err := os.ReadFile(claudeFile)
	if err != nil {
		t.Fatalf("read .claude.json failed: %v", err)
	}

	var conf map[string]any
	if err := json.Unmarshal(raw, &conf); err != nil {
		t.Fatalf("unmarshal .claude.json failed: %v", err)
	}
	servers, ok := conf["mcpServers"].(map[string]any)
	if !ok || servers["cortex"] == nil {
		t.Errorf("mcpServers.cortex missing from .claude.json")
	}

	// Uninstall
	unres, err := UninstallClaude(tempHome, false)
	if err != nil {
		t.Fatalf("UninstallClaude failed: %v", err)
	}
	if !unres.Success {
		t.Errorf("expected success on UninstallClaude")
	}
}
