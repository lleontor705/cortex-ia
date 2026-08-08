// Package permissions injects file-access deny patterns and tool-permission rules into agent configurations.
package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// InjectionResult describes the outcome of permissions injection.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// denyPatterns are file patterns that agents should never read or modify.
var denyPatterns = []string{
	".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx",
	"credentials.json", "service-account.json",
	"**/secrets/**", "**/.secrets/**",
}

// Inject applies security guardrails to the agent's configuration.
// For agents with settings files, it merges deny-list patterns.
// For agents with system prompts only, it injects safety instructions.
func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	switch adapter.Agent() {
	case model.AgentClaudeCode:
		return injectClaudePermissions(homeDir, adapter)
	case model.AgentOpenCode:
		return injectJSONPermissions(homeDir, adapter, buildOpenCodeOverlay())
	case model.AgentCodex, model.AgentVSCodeCopilot:
		return injectPromptPermissions(homeDir, adapter)
	default:
		return InjectionResult{}, nil
	}
}

func injectClaudePermissions(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	overlay := map[string]any{
		"permissions": map[string]any{
			"deny": denyPatterns,
		},
	}
	overlayJSON, _ := json.Marshal(overlay)

	baseJSON, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return InjectionResult{}, fmt.Errorf("read settings: %w", err)
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlayJSON)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("merge permissions: %w", err)
	}

	wr, err := filemerge.WriteFileAtomic(settingsPath, merged, 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{settingsPath}}, nil
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

func injectPromptPermissions(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	if !adapter.SupportsSystemPrompt() {
		return InjectionResult{}, nil
	}
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return InjectionResult{}, nil
	}

	section := buildPermissionsPrompt()

	existing, err := os.ReadFile(promptFile)
	if err != nil && !os.IsNotExist(err) {
		return InjectionResult{}, fmt.Errorf("read system prompt: %w", err)
	}

	updated := filemerge.InjectMarkdownSection(string(existing), "cortex-permissions", section)
	wr, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: wr.Changed, Files: []string{promptFile}}, nil
}

func buildPermissionsPrompt() string {
	var b strings.Builder
	b.WriteString("## Security Guardrails\n\n")
	b.WriteString("NEVER read, modify, or display the contents of these files:\n")
	for _, p := range denyPatterns {
		fmt.Fprintf(&b, "- `%s`\n", p)
	}
	b.WriteString("\nNEVER execute destructive commands like `rm -rf /`, `rm -rf ~`, or fork bombs.\n")
	b.WriteString("Always confirm before executing commands that modify system-level files or configurations.\n")
	return b.String()
}
