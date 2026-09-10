package targets

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseTargets(t *testing.T) {
	cases := []struct {
		input    string
		expected []TargetID
		wantErr  bool
	}{
		{"", []TargetID{TargetOpenCode, TargetAGY, TargetClaude}, false},
		{"all", []TargetID{TargetOpenCode, TargetAGY, TargetClaude}, false},
		{"opencode", []TargetID{TargetOpenCode}, false},
		{"agy", []TargetID{TargetAGY}, false},
		{"claude", []TargetID{TargetClaude}, false},
		{"opencode,agy", []TargetID{TargetOpenCode, TargetAGY}, false},
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

func TestInstallAndUninstallAGY(t *testing.T) {
	tempHome := t.TempDir()

	// Dry run
	res, err := InstallAGY(tempHome, true)
	if err != nil {
		t.Fatalf("InstallAGY dry run failed: %v", err)
	}
	if len(res.Changes) == 0 {
		t.Errorf("expected changes in dry run, got none")
	}

	pluginDir := filepath.Join(tempHome, ".gemini", "config", "plugins", AGYPluginName)
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Errorf("expected plugin dir not to exist after dry run")
	}

	// Real install
	res, err = InstallAGY(tempHome, false)
	if err != nil {
		t.Fatalf("InstallAGY real install failed: %v", err)
	}

	pluginJSON := filepath.Join(pluginDir, "plugin.json")
	if _, err := os.Stat(pluginJSON); err != nil {
		t.Errorf("plugin.json was not created: %v", err)
	}

	hooksJSON := filepath.Join(pluginDir, "hooks.json")
	if _, err := os.Stat(hooksJSON); err != nil {
		t.Errorf("hooks.json was not created: %v", err)
	}

	mcpJSON := filepath.Join(pluginDir, "mcp_config.json")
	if _, err := os.Stat(mcpJSON); err != nil {
		t.Errorf("mcp_config.json was not created: %v", err)
	}

	agentsDir := filepath.Join(pluginDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err != nil || len(entries) == 0 {
		t.Errorf("expected agents directory with agent files, got err=%v len=%d", err, len(entries))
	}

	commandsDir := filepath.Join(pluginDir, "commands")
	if entries, err := os.ReadDir(commandsDir); err != nil || len(entries) == 0 {
		t.Errorf("expected commands directory with command files, got err=%v len=%d", err, len(entries))
	}

	skillsDir := filepath.Join(pluginDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err != nil || len(entries) == 0 {
		t.Errorf("expected skills directory with skill directories, got err=%v len=%d", err, len(entries))
	}

	// Validate with agy CLI if present on the system
	if agyBin, err := exec.LookPath("agy"); err == nil {
		cmd := exec.Command(agyBin, "plugin", "validate", pluginDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("agy plugin validate failed on generated plugin: %v (output: %s)", err, string(out))
		}
	}

	// Verify config.json has plugin enabled
	configFile := filepath.Join(tempHome, ".gemini", "config", "config.json")
	raw, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config.json failed: %v", err)
	}
	var conf map[string]any
	if err := json.Unmarshal(raw, &conf); err != nil {
		t.Fatalf("unmarshal config.json failed: %v", err)
	}
	plugins, ok := conf["plugins"].(map[string]any)
	if !ok || plugins[AGYPluginName] == nil {
		t.Errorf("plugin was not enabled in config.json")
	}

	// Real uninstall
	unres, err := UninstallAGY(tempHome, false)
	if err != nil {
		t.Fatalf("UninstallAGY failed: %v", err)
	}
	if !unres.Success {
		t.Errorf("expected success on uninstall")
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Errorf("plugin dir was not removed after uninstall")
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
