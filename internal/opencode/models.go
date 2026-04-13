// Package opencode provides utilities for reading OpenCode's model configuration.
package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// cacheModel represents a model entry in OpenCode's models.json cache.
type cacheModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToolCall bool   `json:"tool_call"`
}

// cacheProvider represents a provider entry in OpenCode's models.json cache.
type cacheProvider struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Env    []string              `json:"env"`
	Models map[string]cacheModel `json:"models"`
}

// LoadModelsCache reads OpenCode's cached models from ~/.cache/opencode/models.json.
// Returns nil and error if the file doesn't exist or is invalid.
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

// DetectProviders tries to load from cache first, falls back to static list.
func DetectProviders(homeDir string) []model.OpenCodeProvider {
	providers, err := LoadModelsCache(homeDir)
	if err == nil && len(providers) > 0 {
		return providers
	}
	return FallbackProviders()
}

// FallbackProviders returns a static list of common providers with popular models.
func FallbackProviders() []model.OpenCodeProvider {
	return []model.OpenCodeProvider{
		{
			ID:   "anthropic",
			Name: "Anthropic",
			Models: []model.OpenCodeModel{
				{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", ToolCall: true},
				{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ToolCall: true},
				{ID: "claude-haiku-4-20250506", Name: "Claude Haiku 4", ToolCall: true},
			},
		},
		{
			ID:   "openai",
			Name: "OpenAI",
			Models: []model.OpenCodeModel{
				{ID: "gpt-4o", Name: "GPT-4o", ToolCall: true},
				{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ToolCall: true},
				{ID: "o3", Name: "o3", ToolCall: true},
				{ID: "o4-mini", Name: "o4-mini", ToolCall: true},
			},
		},
		{
			ID:   "google",
			Name: "Google",
			Models: []model.OpenCodeModel{
				{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", ToolCall: true},
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", ToolCall: true},
			},
		},
		{
			ID:   "opencode",
			Name: "OpenCode (Built-in)",
			Models: []model.OpenCodeModel{
				{ID: "opencode", Name: "OpenCode Default", ToolCall: true},
			},
		},
	}
}
