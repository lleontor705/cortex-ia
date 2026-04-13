// Package opencode provides utilities for reading OpenCode's model configuration.
package opencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// RunModelsCommand executes `opencode models` and parses the output into
// grouped providers. Each line of output is "provider/model-id".
func RunModelsCommand() ([]model.OpenCodeProvider, error) {
	cmd := exec.Command("opencode", "models")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run 'opencode models': %w", err)
	}
	return ParseModelsOutput(string(output))
}

// ParseModelsOutput parses the output of `opencode models` command.
// Each line is "provider/model-id". Groups by provider.
func ParseModelsOutput(output string) ([]model.OpenCodeProvider, error) {
	providerMap := make(map[string][]model.OpenCodeModel)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "provider/model-id" or "provider/sub/model-id"
		slashIdx := strings.Index(line, "/")
		if slashIdx < 0 {
			continue
		}
		provider := line[:slashIdx]
		modelID := line[slashIdx+1:]
		if provider == "" || modelID == "" {
			continue
		}
		providerMap[provider] = append(providerMap[provider], model.OpenCodeModel{
			ID:       modelID,
			Name:     modelID,
			ToolCall: true,
		})
	}

	var providers []model.OpenCodeProvider
	for id, models := range providerMap {
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		providers = append(providers, model.OpenCodeProvider{
			ID:     id,
			Name:   id,
			Models: models,
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	return providers, nil
}

// FlatModelList converts grouped providers into a flat list of "provider/model" strings.
func FlatModelList(providers []model.OpenCodeProvider) []string {
	var list []string
	for _, p := range providers {
		for _, m := range p.Models {
			list = append(list, p.ID+"/"+m.ID)
		}
	}
	return list
}

// DetectModels tries `opencode models` CLI first, then cache, then static fallback.
func DetectModels(homeDir string) []model.OpenCodeProvider {
	// 1. Try CLI command
	providers, err := RunModelsCommand()
	if err == nil && len(providers) > 0 {
		return providers
	}

	// 2. Try cache file
	providers, err = LoadModelsCache(homeDir)
	if err == nil && len(providers) > 0 {
		return providers
	}

	// 3. Static fallback
	return FallbackProviders()
}

// --- Cache reading (kept as secondary source) ---

type cacheModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToolCall bool   `json:"tool_call"`
}

type cacheProvider struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Models map[string]cacheModel `json:"models"`
}

// LoadModelsCache reads OpenCode's cached models from ~/.cache/opencode/models.json.
func LoadModelsCache(homeDir string) ([]model.OpenCodeProvider, error) {
	path := filepath.Join(homeDir, ".cache", "opencode", "models.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]cacheProvider
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var providers []model.OpenCodeProvider
	for _, cp := range raw {
		var models []model.OpenCodeModel
		for _, cm := range cp.Models {
			if cm.ToolCall {
				models = append(models, model.OpenCodeModel{
					ID:       cm.ID,
					Name:     cm.Name,
					ToolCall: cm.ToolCall,
				})
			}
		}
		if len(models) == 0 {
			continue
		}
		sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
		providers = append(providers, model.OpenCodeProvider{
			ID:     cp.ID,
			Name:   cp.Name,
			Models: models,
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name < providers[j].Name })
	return providers, nil
}

// FallbackProviders returns a static list of common providers.
func FallbackProviders() []model.OpenCodeProvider {
	return []model.OpenCodeProvider{
		{
			ID:   "anthropic",
			Name: "anthropic",
			Models: []model.OpenCodeModel{
				{ID: "claude-opus-4-20250514", Name: "claude-opus-4-20250514", ToolCall: true},
				{ID: "claude-sonnet-4-20250514", Name: "claude-sonnet-4-20250514", ToolCall: true},
				{ID: "claude-haiku-4-5-20251001", Name: "claude-haiku-4-5-20251001", ToolCall: true},
			},
		},
		{
			ID:   "openai",
			Name: "openai",
			Models: []model.OpenCodeModel{
				{ID: "gpt-4o", Name: "gpt-4o", ToolCall: true},
				{ID: "gpt-5.4", Name: "gpt-5.4", ToolCall: true},
			},
		},
		{
			ID:   "google",
			Name: "google",
			Models: []model.OpenCodeModel{
				{ID: "gemini-2.5-pro", Name: "gemini-2.5-pro", ToolCall: true},
				{ID: "gemini-2.5-flash", Name: "gemini-2.5-flash", ToolCall: true},
			},
		},
	}
}

// ApplyToOpenCodeConfig reads opencode.json, sets "model" field on each agent, and writes back.
func ApplyToOpenCodeConfig(homeDir string, assignments model.OpenCodeModelAssignments) error {
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")

	// Read existing config
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = make(map[string]interface{})
		} else {
			return fmt.Errorf("read opencode.json: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse opencode.json: %w", err)
		}
	}

	// Get or create agent section
	agentSection, ok := config["agent"].(map[string]interface{})
	if !ok {
		agentSection = make(map[string]interface{})
	}

	// Apply model assignments to each agent
	for agentName, assignment := range assignments {
		modelStr := assignment.FormatOpenCodeModel()
		if modelStr == "" {
			continue
		}

		// All SDD agents use "sdd-" prefix in OpenCode config
		configName := "sdd-" + agentName

		agentConf, ok := agentSection[configName].(map[string]interface{})
		if !ok {
			agentConf = make(map[string]interface{})
		}
		agentConf["model"] = modelStr
		agentSection[configName] = agentConf
	}

	config["agent"] = agentSection

	// Write back
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode.json: %w", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(configPath, out, 0644)
}
