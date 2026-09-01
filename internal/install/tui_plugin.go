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
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}
	tuiPath := filepath.Join(configDir, "tui.jsonc")
	plugins := []any{}
	if raw, readErr := os.ReadFile(tuiPath); readErr == nil {
		current, decodeErr := filemerge.DecodeJSONObject(raw)
		if decodeErr != nil {
			return "", decodeErr
		}
		if configured, exists := current["plugin"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return "", errors.New("OpenCode tui.jsonc plugin must be an array")
			}
			plugins = append(plugins, values...)
		}
	} else if !os.IsNotExist(readErr) {
		return "", readErr
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
	overlay, err := json.Marshal(map[string]any{
		"$schema": "https://opencode.ai/tui.json",
		"plugin":  plugins,
	})
	if err != nil {
		return "", err
	}
	if _, err := filemerge.MutateJSONFile(tuiPath, filemerge.JSONMutation{Overlay: overlay}); err != nil {
		return "", err
	}
	_ = CleanupLegacyFlatFiles(homeDir)
	return tuiPath, nil
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
