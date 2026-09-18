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
	LegacyTUIPluginPath = "./plugins/cortex-ia-tui.js"
)

// ConfigureTUIPlugin ensures OpenCode's tui.jsonc contains the cortex-ia TUI plugin entry.
func ConfigureTUIPlugin(homeDir string) (string, error) {
	path, _, err := ConfigureTUIPluginWithResult(homeDir)
	return path, err
}

// ConfigureTUIPluginWithResult ensures OpenCode's CLI/TUI config (cli.json, tui.jsonc, or tui.json)
// contains the cortex-ia TUI plugin entry and reports whether the file was modified.
func ConfigureTUIPluginWithResult(homeDir string) (string, bool, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", false, err
	}
	tuiPath := filepath.Join(configDir, "tui.jsonc")
	if _, err := os.Stat(filepath.Join(configDir, "cli.json")); err == nil {
		tuiPath = filepath.Join(configDir, "cli.json")
	} else if _, err := os.Stat(filepath.Join(configDir, "tui.jsonc")); err == nil {
		tuiPath = filepath.Join(configDir, "tui.jsonc")
	} else if _, err := os.Stat(filepath.Join(configDir, "tui.json")); err == nil {
		tuiPath = filepath.Join(configDir, "tui.json")
	}

	plugins := []any{}
	current := map[string]any{}
	if raw, readErr := os.ReadFile(tuiPath); readErr == nil {
		var decodeErr error
		current, decodeErr = filemerge.DecodeJSONObject(raw)
		if decodeErr != nil {
			return "", false, decodeErr
		}
		if configured, exists := current["plugin"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return "", false, errors.New("OpenCode config plugin must be an array")
			}
			plugins = append(plugins, values...)
		}
		if configured, exists := current["plugins"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return "", false, errors.New("OpenCode config plugins must be an array")
			}
			plugins = append(plugins, values...)
		}
	} else if !os.IsNotExist(readErr) {
		return "", false, readErr
	}

	filtered := make([]any, 0, len(plugins))
	for _, configured := range plugins {
		if value, ok := configured.(string); ok && value == LegacyTUIPluginPath {
			continue
		}
		filtered = append(filtered, configured)
	}
	plugins = filtered
	found := false
	for _, configured := range plugins {
		if value, ok := configured.(string); ok && value == TUIPluginPath {
			found = true
			break
		}
	}
	if !found {
		plugins = append(plugins, TUIPluginPath)
	}

	schemaURL := "https://opencode.ai/tui.json"
	pluginKey := "plugin"
	if filepath.Base(tuiPath) == "cli.json" {
		schemaURL = "https://opencode.ai/v2/cli.json"
		pluginKey = "plugins"
	} else if _, exists := current["plugins"]; exists {
		pluginKey = "plugins"
	}

	overlayMap := map[string]any{
		"$schema": schemaURL,
		pluginKey: plugins,
	}
	themeVal, hasTheme := current["theme"]
	if !hasTheme || themeVal == "opencode" || themeVal == "" {
		overlayMap["theme"] = map[string]any{
			"name": "cortex",
			"mode": "dark",
		}
	}

	overlay, err := json.Marshal(overlayMap)
	if err != nil {
		return "", false, err
	}
	mutated, err := filemerge.MutateJSONFile(tuiPath, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return "", false, err
	}
	_ = CleanupLegacyFlatFiles(homeDir)
	return tuiPath, mutated.Changed || mutated.Created, nil
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
