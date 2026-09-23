package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureTUIPlugin(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Fresh configuration: the plugin is registered and the theme key stays
	// absent because the caller did not opt in.
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
	if strings.Contains(content, "theme") {
		t.Errorf("default install must not write the theme key, got:\n%s", content)
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

	// 6. Opt-in applies the cortex theme with the default dark mode
	themeHome := t.TempDir()
	themePath, _, outcome, err := ConfigureTUIPluginWithResult(themeHome, true)
	if err != nil {
		t.Fatalf("opt-in configure failed: %v", err)
	}
	if outcome != ThemeOutcomeApplied {
		t.Errorf("expected theme outcome %q, got %q", ThemeOutcomeApplied, outcome)
	}
	themeData, err := os.ReadFile(themePath)
	if err != nil {
		t.Fatalf("failed to read opted-in config: %v", err)
	}
	compactApplied := compactJSON(string(themeData))
	if !strings.Contains(compactApplied, `"name":"cortex"`) || !strings.Contains(compactApplied, `"mode":"dark"`) {
		t.Errorf("expected cortex theme with dark mode, got:\n%s", string(themeData))
	}

	// 7. Opt-in refreshes a stale theme name while keeping an explicit mode
	staleHome := t.TempDir()
	staleDir := filepath.Join(staleHome, ".config", "opencode")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	staleConfig := `{"theme": {"name": "gruvbox", "mode": "light"}}`
	if err := os.WriteFile(filepath.Join(staleDir, "tui.jsonc"), []byte(staleConfig), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	stalePath, _, staleOutcome, err := ConfigureTUIPluginWithResult(staleHome, true)
	if err != nil {
		t.Fatalf("stale theme refresh failed: %v", err)
	}
	if staleOutcome != ThemeOutcomeApplied {
		t.Errorf("expected theme outcome %q, got %q", ThemeOutcomeApplied, staleOutcome)
	}
	staleData, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("failed to read refreshed config: %v", err)
	}
	compactStale := compactJSON(string(staleData))
	if !strings.Contains(compactStale, `"name":"cortex"`) {
		t.Errorf("expected the stale theme name refreshed to cortex, got:\n%s", string(staleData))
	}
	if !strings.Contains(compactStale, `"mode":"light"`) {
		t.Errorf("expected the explicit light mode preserved, got:\n%s", string(staleData))
	}

	// 8. Opt-out leaves an existing non-default theme exactly as it is
	keepHome := t.TempDir()
	keepDir := filepath.Join(keepHome, ".config", "opencode")
	if err := os.MkdirAll(keepDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keepDir, "tui.jsonc"), []byte(staleConfig), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	keepPath, _, keepOutcome, err := ConfigureTUIPluginWithResult(keepHome, false)
	if err != nil {
		t.Fatalf("opt-out configure failed: %v", err)
	}
	if keepOutcome != ThemeOutcomeSkipped {
		t.Errorf("expected theme outcome %q, got %q", ThemeOutcomeSkipped, keepOutcome)
	}
	keepData, err := os.ReadFile(keepPath)
	if err != nil {
		t.Fatalf("failed to read untouched config: %v", err)
	}
	if compact := compactJSON(string(keepData)); !strings.Contains(compact, `"name":"gruvbox"`) || strings.Contains(compact, `"name":"cortex"`) {
		t.Errorf("opt-out must not rewrite the user theme, got:\n%s", string(keepData))
	}
}

// compactJSON strips insignificant whitespace so assertions hold regardless of
// how the merge writes the object.
func compactJSON(s string) string {
	return strings.ReplaceAll(s, " ", "")
}
