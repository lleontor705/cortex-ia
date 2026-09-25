package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

func setV2Detector(t *testing.T) {
	t.Helper()
	restore := SetOpenCodeVersionDetectorForTesting(func(string) (int, bool) { return opencodeMajorV2, true })
	t.Cleanup(restore)
}

func testConfigDir(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return dir
}

func decodeTestConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return config
}

func stringValues(config map[string]any, key string) []string {
	values, _ := config[key].([]any)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}

func agentEntry(config map[string]any, name string) map[string]any {
	agents, _ := config["agents"].(map[string]any)
	entry, _ := agents[name].(map[string]any)
	return entry
}

func TestConfigureTUIPluginV2(t *testing.T) {
	setV2Detector(t)
	home := t.TempDir()
	configDir := testConfigDir(t, home)

	cliPath, changed, outcome, err := ConfigureTUIPluginWithResult(home, false)
	if err != nil {
		t.Fatalf("ConfigureTUIPluginWithResult failed: %v", err)
	}
	if filepath.Base(cliPath) != cliConfigName {
		t.Fatalf("expected cli.json target, got %s", cliPath)
	}
	if !changed || outcome != ThemeOutcomeApplied {
		t.Fatalf("expected changed theme outcome %q, got changed=%v outcome=%q", ThemeOutcomeApplied, changed, outcome)
	}

	cli := decodeTestConfig(t, cliPath)
	if cli["$schema"] != cliSchemaV2 {
		t.Errorf("expected v2 cli schema, got %v", cli["$schema"])
	}
	if got := stringValues(cli, "plugins"); countString(got, TUIPluginDirV2) != 1 {
		t.Errorf("expected exactly one %s entry in cli.json, got %v", TUIPluginDirV2, got)
	}
	theme, _ := cli["theme"].(map[string]any)
	if theme["name"] != cortexThemeName || theme["mode"] != "dark" {
		t.Errorf("expected cortex theme by default, got %v", cli["theme"])
	}

	if _, err := os.Stat(filepath.Join(configDir, tuiConfigName)); !os.IsNotExist(err) {
		t.Errorf("v2 install must not create tui.jsonc: %v", err)
	}

	managed := decodeTestConfig(t, filepath.Join(configDir, opencodeConfigName))
	if got := stringValues(managed, "plugins"); countString(got, TUIPluginDirV2) != 1 {
		t.Errorf("expected the plugin registered in opencode.jsonc, got %v", got)
	}
	for _, name := range builtinAgentsDisabled {
		if entry := agentEntry(managed, name); entry["disabled"] != true {
			t.Errorf("expected agents.%s.disabled=true, got %v", name, entry)
		}
	}
	if _, touched := managed["agents"].(map[string]any)["general"]; touched {
		t.Errorf("agents.general must never be touched, got %v", managed["agents"])
	}
	for _, bridgeFile := range []string{"tui.js", "package.json"} {
		if _, err := os.Stat(filepath.Join(configDir, "tui-plugins", "cortex-ia", bridgeFile)); err != nil {
			t.Errorf("expected bridge file %s: %v", bridgeFile, err)
		}
	}

	// Re-running must be a no-op across both files.
	_, changedAgain, _, err := ConfigureTUIPluginWithResult(home, false)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if changedAgain {
		t.Errorf("expected the second v2 run to be unchanged")
	}
	if got := stringValues(decodeTestConfig(t, cliPath), "plugins"); countString(got, TUIPluginDirV2) != 1 {
		t.Errorf("expected one plugin entry after re-run, got %v", got)
	}
}

func TestConfigureTUIPluginV2PreservesThirdParty(t *testing.T) {
	setV2Detector(t)
	home := t.TempDir()
	configDir := testConfigDir(t, home)
	writeConfig(t, filepath.Join(configDir, cliConfigName),
		`{"$schema":"https://opencode.ai/v2/cli.json","plugins":["existing.js"],"theme":{"name":"gruvbox","mode":"light"}}`)
	writeConfig(t, filepath.Join(configDir, opencodeConfigName),
		`{"plugins":["other-plugin.js","./tui-plugins/cortex-ia","./tui-plugins/cortex-ia-tui.js"],"agents":{"general":{"model":"x"},"build":{"disabled":false,"model":"y"}}}`)

	if _, _, _, err := ConfigureTUIPluginWithResult(home, false); err != nil {
		t.Fatalf("configure failed: %v", err)
	}

	cli := decodeTestConfig(t, filepath.Join(configDir, cliConfigName))
	plugins := stringValues(cli, "plugins")
	if countString(plugins, TUIPluginDirV2) != 1 || countString(plugins, "existing.js") != 1 {
		t.Errorf("expected deduped v2 entry plus third-party, got %v", plugins)
	}
	if countString(plugins, TUIPluginPath) != 0 {
		t.Errorf("legacy flat plugin must be removed from cli.json, got %v", plugins)
	}
	theme, _ := cli["theme"].(map[string]any)
	if theme["name"] != cortexThemeName || theme["mode"] != "light" {
		t.Errorf("expected cortex theme with preserved light mode, got %v", cli["theme"])
	}

	managed := decodeTestConfig(t, filepath.Join(configDir, opencodeConfigName))
	managedPlugins := stringValues(managed, "plugins")
	if countString(managedPlugins, "other-plugin.js") != 1 || countString(managedPlugins, TUIPluginDirV2) != 1 {
		t.Errorf("expected third-party preserved and one cortex entry, got %v", managedPlugins)
	}
	if countString(managedPlugins, TUIPluginPath) != 0 {
		t.Errorf("legacy flat plugin must be removed from opencode.jsonc, got %v", managedPlugins)
	}
	if general := agentEntry(managed, "general"); general["model"] != "x" {
		t.Errorf("expected agents.general preserved, got %v", general)
	}
	if build := agentEntry(managed, "build"); build["disabled"] != true || build["model"] != "y" {
		t.Errorf("expected agents.build disabled with model preserved, got %v", build)
	}
}

func TestConfigureTUIPluginV2ReappliesClobberedTheme(t *testing.T) {
	setV2Detector(t)
	home := t.TempDir()
	configDir := testConfigDir(t, home)
	writeConfig(t, filepath.Join(configDir, cliConfigName), `{"plugins":["./tui-plugins/cortex-ia"]}`)

	if _, changed, outcome, err := ConfigureTUIPluginWithResult(home, false); err != nil {
		t.Fatalf("configure failed: %v", err)
	} else if !changed || outcome != ThemeOutcomeApplied {
		t.Fatalf("expected a theme re-apply, got changed=%v outcome=%q", changed, outcome)
	}
	theme, _ := decodeTestConfig(t, filepath.Join(configDir, cliConfigName))["theme"].(map[string]any)
	if theme["name"] != cortexThemeName {
		t.Errorf("expected a clobbered theme key to be re-applied, got %v", theme)
	}
}

func TestConfigureTUIPluginV1(t *testing.T) {
	setV2Detector(t)
	home := t.TempDir()
	configDir := testConfigDir(t, home)
	writeConfig(t, filepath.Join(configDir, tuiConfigName),
		`{"plugin":["./plugins/cortex-ia-tui.js","other-plugin.js"],"theme":{"name":"gruvbox","mode":"light"}}`)

	tuiPath, _, outcome, err := ConfigureTUIPluginWithResult(home, false)
	if err != nil {
		t.Fatalf("v1 configure failed: %v", err)
	}
	if filepath.Base(tuiPath) != tuiConfigName {
		t.Fatalf("expected tui.jsonc target, got %s", tuiPath)
	}
	if outcome != ThemeOutcomeApplied {
		t.Errorf("expected applied theme outcome, got %q", outcome)
	}
	tui := decodeTestConfig(t, tuiPath)
	plugins := stringValues(tui, "plugin")
	if countString(plugins, TUIPluginPath) != 1 || countString(plugins, "other-plugin.js") != 1 {
		t.Errorf("expected migrated v1 plugin entry, got %v", plugins)
	}
	if countString(plugins, LegacyTUIPluginPath) != 0 {
		t.Errorf("expected legacy plugin removed, got %v", plugins)
	}
	theme, _ := tui["theme"].(map[string]any)
	if theme["name"] != cortexThemeName || theme["mode"] != "light" {
		t.Errorf("expected cortex theme with preserved mode, got %v", tui["theme"])
	}
	if _, err := os.Stat(filepath.Join(configDir, cliConfigName)); !os.IsNotExist(err) {
		t.Errorf("v1 install must not create cli.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, opencodeConfigName)); !os.IsNotExist(err) {
		t.Errorf("v1 install must not create opencode.jsonc: %v", err)
	}
}

func TestConfigureTUIPluginDualConfigTargetsV2(t *testing.T) {
	restore := SetOpenCodeVersionDetectorForTesting(func(string) (int, bool) { return opencodeMajorV1, true })
	t.Cleanup(restore)
	home := t.TempDir()
	configDir := testConfigDir(t, home)
	writeConfig(t, filepath.Join(configDir, cliConfigName), `{"$schema":"https://opencode.ai/v2/cli.json","plugins":[]}`)
	legacyTUI := `{"$schema":"https://opencode.ai/tui.json","plugin":[]}`
	writeConfig(t, filepath.Join(configDir, tuiConfigName), legacyTUI)

	primary, _, _, err := ConfigureTUIPluginWithResult(home, false)
	if err != nil {
		t.Fatalf("dual configure failed: %v", err)
	}
	if filepath.Base(primary) != cliConfigName {
		t.Fatalf("existing cli.json must win detection, got %s", primary)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, tuiConfigName))
	if err != nil {
		t.Fatalf("read tui.jsonc: %v", err)
	}
	if string(raw) != legacyTUI {
		t.Errorf("v2 must leave tui.jsonc untouched, got %s", string(raw))
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
