package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// InstallClaude configures Claude Code with the Cortex-IA MCP server.
func InstallClaude(homeDir string, dryRun bool) (*TargetResult, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	configFile := filepath.Join(homeDir, ".claude.json")
	res := &TargetResult{
		Target:  TargetClaude,
		Success: true,
	}

	overlay, err := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"cortex": map[string]any{
				"command": "cortex-ia",
				"args":    []string{"mcp", "serve"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Claude MCP overlay: %w", err)
	}

	rel, _ := filepath.Rel(homeDir, configFile)
	res.Changes = append(res.Changes, fmt.Sprintf("mutate %s (configure mcpServers.cortex)", filepath.ToSlash(rel)))

	if !dryRun {
		_, err := filemerge.MutateJSONFile(configFile, filemerge.JSONMutation{Overlay: overlay})
		if err != nil {
			return nil, fmt.Errorf("configure Claude MCP server: %w", err)
		}
	}

	res.Message = fmt.Sprintf("Configured Cortex MCP server in %s", filepath.ToSlash(configFile))
	return res, nil
}

// UninstallClaude removes the Cortex-IA MCP server from Claude Code.
func UninstallClaude(homeDir string, dryRun bool) (*TargetResult, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	configFile := filepath.Join(homeDir, ".claude.json")
	res := &TargetResult{
		Target:  TargetClaude,
		Success: true,
	}

	if _, err := os.Stat(configFile); err != nil {
		res.Message = "No ~/.claude.json file found"
		return res, nil
	}

	rel, _ := filepath.Rel(homeDir, configFile)
	res.Changes = append(res.Changes, fmt.Sprintf("mutate %s (remove mcpServers.cortex)", filepath.ToSlash(rel)))

	if !dryRun {
		mutation := filemerge.JSONMutation{
			RemovePaths: [][]string{{"mcpServers", "cortex"}},
		}
		_, _ = filemerge.MutateJSONFile(configFile, mutation)
	}

	res.Message = "Removed Cortex MCP server from Claude configuration"
	return res, nil
}
