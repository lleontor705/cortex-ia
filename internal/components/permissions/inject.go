// Package permissions injects file-access deny patterns and tool-permission rules into agent configurations.
package permissions

import (
	"encoding/json"
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// InjectionResult describes the outcome of permissions injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// Inject applies security guardrails to the agent's configuration.
func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	if adapter == nil {
		return InjectionResult{}, nil
	}
	return injectJSONPermissions(homeDir, adapter, buildOpenCodeOverlay())
}

func buildOpenCodeOverlay() []byte {
	overlay := map[string]any{
		"permission": map[string]any{
			"bash": map[string]any{
				"*":             "ask",
				"rm -rf /":      "deny",
				"rm -rf /*":     "deny",
				"rm -rf ~":      "deny",
				"sudo rm -rf *": "deny",
				":(){ :|:& };:": "deny",
			},
			"read": map[string]any{
				"*":                    "allow",
				".env":                 "deny",
				".env.*":               "deny",
				".env.example":         "allow",
				"*.pem":                "deny",
				"*.key":                "deny",
				"*.p12":                "deny",
				"*.pfx":                "deny",
				"credentials.json":     "deny",
				"service-account.json": "deny",
				"**/secrets/**":        "deny",
				"**/.secrets/**":       "deny",
			},
		},
	}
	data, _ := json.Marshal(overlay)
	return data
}

func injectJSONPermissions(homeDir string, adapter agents.Adapter, overlay []byte) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	result, err := filemerge.MutateJSONFile(settingsPath, filemerge.JSONMutation{
		Overlay: overlay, RemovePaths: [][]string{{"permissions"}},
	})
	if err != nil {
		return InjectionResult{}, fmt.Errorf("patch OpenCode permissions: %w", err)
	}
	return InjectionResult{Changed: result.Changed, Files: []string{settingsPath}}, nil
}
