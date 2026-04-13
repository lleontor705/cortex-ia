package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestParseModelsOutput_Basic(t *testing.T) {
	output := `anthropic/claude-sonnet-4-20250514
anthropic/claude-opus-4-20250514
openai/gpt-4o
openai/gpt-5.4
google/gemini-2.5-pro
`
	providers, err := ParseModelsOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}

	// Check anthropic has 2 models
	for _, p := range providers {
		if p.ID == "anthropic" {
			if len(p.Models) != 2 {
				t.Errorf("anthropic should have 2 models, got %d", len(p.Models))
			}
		}
	}
}

func TestParseModelsOutput_OpenRouter(t *testing.T) {
	output := `openrouter/anthropic/claude-opus-4
openrouter/google/gemini-2.5-pro
`
	providers, err := ParseModelsOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider (openrouter), got %d", len(providers))
	}
	if providers[0].ID != "openrouter" {
		t.Errorf("provider ID = %q, want %q", providers[0].ID, "openrouter")
	}
	// openrouter models include the sub-path
	if providers[0].Models[0].ID != "anthropic/claude-opus-4" {
		t.Errorf("model ID = %q, want %q", providers[0].Models[0].ID, "anthropic/claude-opus-4")
	}
}

func TestParseModelsOutput_EmptyLines(t *testing.T) {
	output := `
anthropic/claude-sonnet-4

openai/gpt-4o

`
	providers, err := ParseModelsOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestParseModelsOutput_Empty(t *testing.T) {
	providers, err := ParseModelsOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected 0 providers for empty output, got %d", len(providers))
	}
}

func TestFlatModelList(t *testing.T) {
	providers := []model.OpenCodeProvider{
		{ID: "anthropic", Models: []model.OpenCodeModel{{ID: "claude-sonnet-4"}}},
		{ID: "openai", Models: []model.OpenCodeModel{{ID: "gpt-4o"}, {ID: "gpt-5"}}},
	}
	list := FlatModelList(providers)
	if len(list) != 3 {
		t.Fatalf("expected 3 models, got %d", len(list))
	}
	if list[0] != "anthropic/claude-sonnet-4" {
		t.Errorf("list[0] = %q, want %q", list[0], "anthropic/claude-sonnet-4")
	}
}

func TestFallbackProviders(t *testing.T) {
	providers := FallbackProviders()
	if len(providers) == 0 {
		t.Fatal("FallbackProviders should return providers")
	}
}

func TestLoadModelsCache_FileNotFound(t *testing.T) {
	_, err := LoadModelsCache(t.TempDir())
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestApplyToOpenCodeConfig(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".config", "opencode")
	os.MkdirAll(configDir, 0755)

	// Write initial config
	initial := map[string]interface{}{
		"theme": "dark",
		"agent": map[string]interface{}{
			"sdd-orchestrator": map[string]interface{}{
				"mode": "primary",
			},
		},
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(filepath.Join(configDir, "opencode.json"), data, 0644)

	// Apply assignments
	assignments := model.OpenCodeModelAssignments{
		"orchestrator": {Provider: "anthropic", Model: "claude-opus-4"},
		"implement":    {Provider: "openai", Model: "gpt-4o"},
	}
	err := ApplyToOpenCodeConfig(dir, assignments)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify
	result, _ := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	var config map[string]interface{}
	json.Unmarshal(result, &config)

	agents := config["agent"].(map[string]interface{})

	// Check orchestrator
	orch := agents["sdd-orchestrator"].(map[string]interface{})
	if orch["model"] != "anthropic/claude-opus-4" {
		t.Errorf("orchestrator model = %q, want %q", orch["model"], "anthropic/claude-opus-4")
	}
	if orch["mode"] != "primary" {
		t.Error("existing fields should be preserved")
	}

	// Check implement
	impl := agents["sdd-implement"].(map[string]interface{})
	if impl["model"] != "openai/gpt-4o" {
		t.Errorf("implement model = %q, want %q", impl["model"], "openai/gpt-4o")
	}

	// Check theme preserved
	if config["theme"] != "dark" {
		t.Error("theme should be preserved")
	}
}

func TestApplyToOpenCodeConfig_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	assignments := model.OpenCodeModelAssignments{
		"orchestrator": {Provider: "anthropic", Model: "claude-opus-4"},
	}
	err := ApplyToOpenCodeConfig(dir, assignments)
	if err != nil {
		t.Fatalf("should create config if missing: %v", err)
	}

	// Verify file created
	path := filepath.Join(dir, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file should have been created")
	}
}
