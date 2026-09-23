package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const (
	TUIPluginPath       = "./tui-plugins/cortex-ia-tui.js"
	TUIPluginDirV2      = "./tui-plugins/cortex-ia"
	LegacyTUIPluginPath = "./plugins/cortex-ia-tui.js"
)

// ThemeApplyOutcome reports how the optional cortex theme was resolved by one
// configuration pass.
type ThemeApplyOutcome string

const (
	// ThemeOutcomeApplied reports the cortex theme entry was written or refreshed.
	ThemeOutcomeApplied ThemeApplyOutcome = "applied"
	// ThemeOutcomeSkipped reports the theme key was left untouched because the
	// user never opted in.
	ThemeOutcomeSkipped ThemeApplyOutcome = "skipped-not-requested"
	// ThemeOutcomeError reports the requested theme could not be written.
	ThemeOutcomeError ThemeApplyOutcome = "error"
)

// ConfigureTUIPlugin ensures OpenCode's configuration contains the cortex-ia TUI
// plugin entry. It never applies the theme: callers that want it must ask for it
// explicitly through ConfigureTUIPluginWithResult.
func ConfigureTUIPlugin(homeDir string) (string, error) {
	path, _, _, err := ConfigureTUIPluginWithResult(homeDir, false)
	return path, err
}

// ConfigureTUIPluginWithResult ensures OpenCode's CLI/TUI config (cli.json, tui.jsonc, or tui.json)
// contains the cortex-ia TUI plugin entry, ensures the OpenCode v2 bridge directory exists,
// and reports whether the file was modified. applyTheme is the explicit opt-in
// for the bundled cortex theme; without it the theme key is neither read nor
// rewritten, and the outcome reports that the theme was skipped.
func ConfigureTUIPluginWithResult(homeDir string, applyTheme bool) (string, bool, ThemeApplyOutcome, error) {
	// A pass that fails before writing reports the theme as skipped when nobody
	// asked for it, and as an error when a requested theme could not be written.
	failureOutcome := ThemeOutcomeSkipped
	if applyTheme {
		failureOutcome = ThemeOutcomeError
	}
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", false, failureOutcome, err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", false, failureOutcome, err
	}

	cliPath := filepath.Join(configDir, "cli.json")
	tuiCPath := filepath.Join(configDir, "tui.jsonc")
	tuiPath := filepath.Join(configDir, "tui.json")

	var primaryPath string
	var secondaryPaths []string

	if _, err := os.Stat(cliPath); err == nil {
		primaryPath = cliPath
		if _, err := os.Stat(tuiCPath); err == nil {
			secondaryPaths = append(secondaryPaths, tuiCPath)
		}
		if _, err := os.Stat(tuiPath); err == nil {
			secondaryPaths = append(secondaryPaths, tuiPath)
		}
	} else if _, err := os.Stat(tuiCPath); err == nil {
		primaryPath = tuiCPath
		if _, err := os.Stat(tuiPath); err == nil {
			secondaryPaths = append(secondaryPaths, tuiPath)
		}
	} else if _, err := os.Stat(tuiPath); err == nil {
		primaryPath = tuiPath
	} else {
		primaryPath = tuiCPath
	}

	primaryChanged, err := configureSingleTUIFile(primaryPath, applyTheme)
	if err != nil {
		return "", false, failureOutcome, err
	}
	anyChanged := primaryChanged

	for _, sec := range secondaryPaths {
		secChanged, secErr := configureSingleTUIFile(sec, applyTheme)
		if secErr == nil && secChanged {
			anyChanged = true
		}
	}

	_ = ensureTUIBridge(configDir)
	_ = CleanupLegacyFlatFiles(homeDir)
	if applyTheme {
		return primaryPath, anyChanged, ThemeOutcomeApplied, nil
	}
	return primaryPath, anyChanged, ThemeOutcomeSkipped, nil
}

func ensureTUIBridge(configDir string) error {
	bridgeDir := filepath.Join(configDir, "tui-plugins", "cortex-ia")
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		return err
	}
	bridgeFile := filepath.Join(bridgeDir, "tui.js")
	bridgeContent := []byte("// OpenCode v2 directory bridge for cortex-ia TUI plugin\nexport * from \"../cortex-ia-tui.js\";\nexport { default } from \"../cortex-ia-tui.js\";\n")
	if err := writeFileIfChanged(bridgeFile, bridgeContent); err != nil {
		return err
	}
	// OpenCode v2 imports the "./tui" export of the configured package, so the bridge
	// directory must expose a module manifest or the cli.json entry resolves to nothing.
	manifestFile := filepath.Join(bridgeDir, "package.json")
	manifestContent := []byte("{\n  \"name\": \"cortex-ia-tui\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"exports\": {\n    \".\": \"./tui.js\",\n    \"./tui\": \"./tui.js\"\n  }\n}\n")
	return writeFileIfChanged(manifestFile, manifestContent)
}

func writeFileIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func configureSingleTUIFile(tuiPath string, applyTheme bool) (bool, error) {
	plugins := []any{}
	current := map[string]any{}
	isCLI := filepath.Base(tuiPath) == "cli.json"
	targetPlugin := TUIPluginPath
	if isCLI {
		targetPlugin = TUIPluginDirV2
	}

	if raw, readErr := os.ReadFile(tuiPath); readErr == nil {
		var decodeErr error
		current, decodeErr = filemerge.DecodeJSONObject(raw)
		if decodeErr != nil {
			return false, decodeErr
		}
		if configured, exists := current["plugin"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return false, errors.New("OpenCode config plugin must be an array")
			}
			plugins = append(plugins, values...)
		}
		if configured, exists := current["plugins"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return false, errors.New("OpenCode config plugins must be an array")
			}
			plugins = append(plugins, values...)
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}

	filtered := make([]any, 0, len(plugins))
	for _, configured := range plugins {
		if value, ok := configured.(string); ok {
			if value == LegacyTUIPluginPath {
				continue
			}
			if isCLI && value == TUIPluginPath {
				continue
			}
		}
		filtered = append(filtered, configured)
	}
	plugins = filtered
	found := false
	for _, configured := range plugins {
		if value, ok := configured.(string); ok && value == targetPlugin {
			found = true
			break
		}
	}
	if !found {
		plugins = append(plugins, targetPlugin)
	}

	schemaURL := "https://opencode.ai/tui.json"
	pluginKey := "plugin"
	if isCLI {
		schemaURL = "https://opencode.ai/v2/cli.json"
		pluginKey = "plugins"
	} else if _, exists := current["plugins"]; exists {
		pluginKey = "plugins"
	}

	overlayMap := map[string]any{
		"$schema": schemaURL,
		pluginKey: plugins,
	}
	if applyTheme {
		overlayMap["theme"] = cortexThemeOverlay(current)
	}

	overlay, err := json.Marshal(overlayMap)
	if err != nil {
		return false, err
	}
	mutated, err := filemerge.MutateJSONFile(tuiPath, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return false, err
	}
	return mutated.Changed || mutated.Created, nil
}

// cortexThemeOverlay builds the opted-in cortex theme entry. An explicit
// light/dark preference is preserved because it encodes the user's terminal
// contrast choice, which is unrelated to the theme name. Anything else —
// including a theme name left behind by an earlier install — is refreshed to
// the cortex theme, since a stale name is what silently prevented the cortex
// theme from ever loading.
func cortexThemeOverlay(current map[string]any) map[string]any {
	mode := "dark"
	if existing, ok := current["theme"].(map[string]any); ok {
		if value, ok := existing["mode"].(string); ok && (value == "light" || value == "dark") {
			mode = value
		}
	}
	return map[string]any{
		"name": "cortex",
		"mode": mode,
	}
}

// CleanupLegacyFlatFiles cleans up orphaned flat files like cortex-authority-state-*.json,
// cortex-delegation-panes-*.json, and cortex-delegation-events.jsonl in ~/.config/opencode/.
func CleanupLegacyFlatFiles(homeDir string) error {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if (len(name) > 23 && name[:23] == "cortex-authority-state-" && len(name) > 5 && name[len(name)-5:] == ".json") ||
			(len(name) > 22 && name[:22] == "cortex-delegation-panes-" && len(name) > 5 && name[len(name)-5:] == ".json") ||
			name == "cortex-delegation-events.jsonl" {
			_ = os.Remove(filepath.Join(configDir, name))
		}
	}
	return nil
}
