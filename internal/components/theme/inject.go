// Package theme injects terminal color-theme configuration into agent system prompts.
package theme

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// InjectionResult describes the outcome of theme injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// ThemeID identifies a terminal theme.
type ThemeID string

const (
	ThemeCortex  ThemeID = "cortex"
	ThemeDefault ThemeID = "default"
)

// themeOverlays maps theme IDs to JSON overlays for agent settings.
var themeOverlays = map[ThemeID][]byte{
	ThemeCortex: []byte(`{
  "theme": {
    "name": "cortex",
    "colors": {
      "primary": "#7C3AED",
      "secondary": "#06B6D4",
      "success": "#22C55E",
      "warning": "#F59E0B",
      "error": "#EF4444"
    }
  }
}`),
}

// Inject applies a theme overlay to the agent's settings file.
// Agents without a settings path are skipped.
func Inject(homeDir string, adapter agents.Adapter, theme ThemeID) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	if theme == "" {
		theme = ThemeCortex
	}

	overlay, ok := themeOverlays[theme]
	if !ok {
		return InjectionResult{}, fmt.Errorf("unknown theme: %q", theme)
	}

	baseJSON, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return InjectionResult{}, fmt.Errorf("read settings: %w", err)
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("merge theme: %w", err)
	}

	wr, err := filemerge.WriteFileAtomic(settingsPath, merged, 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: wr.Changed, Files: []string{settingsPath}}, nil
}
