package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const (
	AGYPluginName = "cortex-ia"
)

// InstallAGY packages and installs the Cortex-IA plugin bundle for Antigravity.
func InstallAGY(homeDir string, dryRun bool) (*TargetResult, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	configDir := filepath.Join(homeDir, ".gemini", "config")
	pluginDir := filepath.Join(configDir, "plugins", AGYPluginName)

	res := &TargetResult{
		Target:  TargetAGY,
		Success: true,
	}

	// 1. Files to write in plugin
	manifest := map[string]string{
		"name":        AGYPluginName,
		"version":     "1.0.0",
		"description": "Cortex-IA Bridge, Work Control & Task Authority for Antigravity",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")

	hooks := map[string]any{
		"cortex-lease-guard": map[string]any{
			"PreToolUse": []map[string]any{
				{
					"matcher": "write_to_file|edit|replace_file_content|apply_patch",
					"hooks": []map[string]any{
						{
							"type":    "command",
							"command": "cortex-ia hook pre-tool",
							"timeout": 10,
						},
					},
				},
			},
			"Stop": []map[string]any{
				{
					"hooks": []map[string]any{
						{
							"type":    "command",
							"command": "cortex-ia hook stop",
							"timeout": 10,
						},
					},
				},
			},
		},
	}
	hooksBytes, _ := json.MarshalIndent(hooks, "", "  ")

	mcp := map[string]any{
		"mcpServers": map[string]any{
			"cortex": map[string]any{
				"command": "cortex-ia",
				"args":    []string{"mcp", "serve"},
				"env":     map[string]string{},
			},
		},
	}
	mcpBytes, _ := json.MarshalIndent(mcp, "", "  ")

	agentsDoc, err := assets.Read("AGENTS.md")
	if err != nil {
		agentsDoc = "# Cortex-IA Protocol for Antigravity\n"
	}

	filesToWrite := map[string][]byte{
		filepath.Join(pluginDir, "plugin.json"):        manifestBytes,
		filepath.Join(pluginDir, "hooks.json"):         hooksBytes,
		filepath.Join(pluginDir, "mcp_config.json"):    mcpBytes,
		filepath.Join(pluginDir, "rules", "AGENTS.md"): []byte(agentsDoc),
	}

	// Add embedded skills, agents, and commands
	inventory, err := assets.Inventory()
	if err == nil {
		for _, file := range inventory {
			if file.Kind == assets.KindSkill || file.Kind == assets.KindAgent || file.Kind == assets.KindCommand {
				data, err := assets.ReadBytes(file.Path)
				if err == nil {
					dest := filepath.Join(pluginDir, filepath.FromSlash(file.Path))
					if file.Kind == assets.KindAgent {
						data = adaptAgentForAGY(file.Path, data)
					}
					filesToWrite[dest] = data
				}
			}
		}
	}

	for dest, content := range filesToWrite {
		rel, _ := filepath.Rel(homeDir, dest)
		res.Changes = append(res.Changes, fmt.Sprintf("write %s", filepath.ToSlash(rel)))
		if !dryRun {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return nil, fmt.Errorf("create directory for %s: %w", dest, err)
			}
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", dest, err)
			}
		}
	}

	// 2. Enable plugin in ~/.gemini/config/config.json
	configFile := filepath.Join(configDir, "config.json")
	overlay, err := json.Marshal(map[string]any{
		"plugins": map[string]any{
			AGYPluginName: map[string]any{
				"enabled": true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode AGY config overlay: %w", err)
	}

	relConfig, _ := filepath.Rel(homeDir, configFile)
	res.Changes = append(res.Changes, fmt.Sprintf("mutate %s (enable %s)", filepath.ToSlash(relConfig), AGYPluginName))

	if !dryRun {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, fmt.Errorf("create config directory: %w", err)
		}
		_, err := filemerge.MutateJSONFile(configFile, filemerge.JSONMutation{Overlay: overlay})
		if err != nil {
			return nil, fmt.Errorf("enable AGY plugin in config.json: %w", err)
		}
		if agyBin, err := exec.LookPath("agy"); err == nil {
			cmd := exec.Command(agyBin, "plugin", "validate", pluginDir)
			if out, err := cmd.CombinedOutput(); err == nil {
				res.Changes = append(res.Changes, fmt.Sprintf("validated plugin %q with agy CLI (%s)", AGYPluginName, strings.TrimSpace(string(out))))
			}
		}
	}

	res.Message = fmt.Sprintf("Installed Antigravity plugin %q at %s", AGYPluginName, filepath.ToSlash(pluginDir))
	return res, nil
}

func adaptAgentForAGY(agentPath string, data []byte) []byte {
	role := strings.TrimSuffix(filepath.Base(agentPath), ".md")
	content := string(data)

	if !strings.HasPrefix(content, "---") {
		return data
	}

	rest := strings.TrimPrefix(content, "---")
	parts := strings.SplitN(rest, "---", 2)
	if len(parts) != 2 {
		return data
	}

	yamlPart := parts[0]
	body := parts[1]

	description := fmt.Sprintf("Cortex-IA %s role", role)
	for _, line := range strings.Split(yamlPart, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "description:") {
			desc := strings.TrimPrefix(trimmed, "description:")
			description = strings.Trim(strings.TrimSpace(desc), `"'`)
			break
		}
	}

	var agyTools []string
	if role == "reviewer" || role == "investigate" || role == "discovery" {
		agyTools = []string{"run_command", "view_file", "grep_search", "find_by_name", "list_dir"}
	} else {
		agyTools = []string{"run_command", "write_to_file", "view_file", "grep_search", "find_by_name", "list_dir", "replace_file_content"}
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %s\n", role)
	fmt.Fprintf(&sb, "description: %q\n", description)
	sb.WriteString("mode: subagent\n")
	sb.WriteString("tools:\n")
	for _, tool := range agyTools {
		fmt.Fprintf(&sb, "  - %s\n", tool)
	}
	sb.WriteString("permission:\n  bash:\n    \"*\": allow\n")
	sb.WriteString("---\n")
	sb.WriteString(body)

	return []byte(sb.String())
}

// UninstallAGY removes the Cortex-IA plugin bundle for Antigravity.
func UninstallAGY(homeDir string, dryRun bool) (*TargetResult, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	configDir := filepath.Join(homeDir, ".gemini", "config")
	pluginDir := filepath.Join(configDir, "plugins", AGYPluginName)
	configFile := filepath.Join(configDir, "config.json")

	res := &TargetResult{
		Target:  TargetAGY,
		Success: true,
	}

	if _, err := os.Stat(pluginDir); err == nil {
		rel, _ := filepath.Rel(homeDir, pluginDir)
		res.Changes = append(res.Changes, fmt.Sprintf("remove %s", filepath.ToSlash(rel)))
		if !dryRun {
			_ = os.RemoveAll(pluginDir)
		}
	}

	if _, err := os.Stat(configFile); err == nil {
		rel, _ := filepath.Rel(homeDir, configFile)
		res.Changes = append(res.Changes, fmt.Sprintf("mutate %s (disable %s)", filepath.ToSlash(rel), AGYPluginName))
		if !dryRun {
			overlay, _ := json.Marshal(map[string]any{
				"plugins": map[string]any{
					AGYPluginName: map[string]any{
						"enabled": false,
					},
				},
			})
			_, _ = filemerge.MutateJSONFile(configFile, filemerge.JSONMutation{Overlay: overlay})
		}
	}

	res.Message = fmt.Sprintf("Uninstalled Antigravity plugin %q", AGYPluginName)
	return res, nil
}
