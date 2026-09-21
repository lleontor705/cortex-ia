package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureTUIPlugin(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Fresh configuration
	tuiPath, err := ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed on fresh home: %v", err)
	}
	data, err := os.ReadFile(tuiPath)
	if err != nil {
		t.Fatalf("failed to read tui.jsonc: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, TUIPluginPath) {
		t.Errorf("expected %s in tui.jsonc, got:\n%s", TUIPluginPath, content)
	}
	if !strings.Contains(content, `"name":"cortex"`) && !strings.Contains(content, `"name": "cortex"`) {
		t.Errorf("expected cortex theme in tui.jsonc, got:\n%s", content)
	}

	// 2. Re-run idempotency
	_, err = ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin re-run failed: %v", err)
	}
	data2, _ := os.ReadFile(tuiPath)
	if strings.Count(string(data2), TUIPluginPath) != 1 {
		t.Errorf("expected exactly one plugin entry, got:\n%s", string(data2))
	}

	// 3. Migration from legacy plugin path
	legacyJSON := `{"plugin": ["./plugins/cortex-ia-tui.js", "other-plugin.js"]}`
	_ = os.WriteFile(filepath.Join(tempHome, ".config", "opencode", "tui.jsonc"), []byte(legacyJSON), 0644)
	_, err = ConfigureTUIPlugin(tempHome)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed migrating legacy: %v", err)
	}
	data3, _ := os.ReadFile(tuiPath)
	content3 := string(data3)
	if strings.Contains(content3, LegacyTUIPluginPath) {
		t.Errorf("expected legacy plugin to be removed, got:\n%s", content3)
	}
	if !strings.Contains(content3, TUIPluginPath) || !strings.Contains(content3, "other-plugin.js") {
		t.Errorf("expected TUIPluginPath and other-plugin.js, got:\n%s", content3)
	}

	// 4. Configuration with existing cli.json (OpenCode v2)
	tempHomeV2 := t.TempDir()
	cliJSON := `{"$schema": "https://opencode.ai/v2/cli.json", "plugins": ["existing-plugin.js"]}`
	_ = os.MkdirAll(filepath.Join(tempHomeV2, ".config", "opencode"), 0755)
	_ = os.WriteFile(filepath.Join(tempHomeV2, ".config", "opencode", "cli.json"), []byte(cliJSON), 0644)
	cliPath, err := ConfigureTUIPlugin(tempHomeV2)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed on cli.json home: %v", err)
	}
	if filepath.Base(cliPath) != "cli.json" {
		t.Errorf("expected cli.json target, got: %s", cliPath)
	}
	dataV2, err := os.ReadFile(cliPath)
	if err != nil {
		t.Fatalf("failed to read cli.json: %v", err)
	}
	contentV2 := string(dataV2)
	if !strings.Contains(contentV2, "https://opencode.ai/v2/cli.json") {
		t.Errorf("expected v2 schema in cli.json, got:\n%s", contentV2)
	}
	if !strings.Contains(contentV2, TUIPluginDirV2) || !strings.Contains(contentV2, "existing-plugin.js") {
		t.Errorf("expected TUIPluginDirV2 and existing-plugin.js in cli.json, got:\n%s", contentV2)
	}
	bridgeFile := filepath.Join(tempHomeV2, ".config", "opencode", "tui-plugins", "cortex-ia", "tui.js")
	if _, err := os.Stat(bridgeFile); err != nil {
		t.Errorf("expected bridge file at %s, got error: %v", bridgeFile, err)
	}
	manifestFile := filepath.Join(tempHomeV2, ".config", "opencode", "tui-plugins", "cortex-ia", "package.json")
	manifest, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Errorf("expected bridge manifest at %s, got error: %v", manifestFile, err)
	} else if !strings.Contains(string(manifest), `"./tui": "./tui.js"`) {
		t.Errorf("expected ./tui export in bridge manifest, got:\n%s", string(manifest))
	}

	// 5. Dual configuration when both cli.json and tui.jsonc exist
	tempHomeDual := t.TempDir()
	dualConfigDir := filepath.Join(tempHomeDual, ".config", "opencode")
	_ = os.MkdirAll(dualConfigDir, 0755)
	_ = os.WriteFile(filepath.Join(dualConfigDir, "cli.json"), []byte(`{"$schema": "https://opencode.ai/v2/cli.json", "plugins": []}`), 0644)
	_ = os.WriteFile(filepath.Join(dualConfigDir, "tui.jsonc"), []byte(`{"$schema": "https://opencode.ai/tui.json", "plugin": []}`), 0644)

	dualPath, err := ConfigureTUIPlugin(tempHomeDual)
	if err != nil {
		t.Fatalf("ConfigureTUIPlugin failed on dual home: %v", err)
	}
	if filepath.Base(dualPath) != "cli.json" {
		t.Errorf("expected primary to be cli.json, got: %s", dualPath)
	}
	cliData, err := os.ReadFile(filepath.Join(dualConfigDir, "cli.json"))
	if err != nil || !strings.Contains(string(cliData), TUIPluginDirV2) {
		t.Errorf("expected TUIPluginDirV2 in cli.json, got error: %v, content: %s", err, string(cliData))
	}
	tuiData, err := os.ReadFile(filepath.Join(dualConfigDir, "tui.jsonc"))
	if err != nil || !strings.Contains(string(tuiData), TUIPluginPath) {
		t.Errorf("expected TUIPluginPath in tui.jsonc, got error: %v, content: %s", err, string(tuiData))
	}
}
