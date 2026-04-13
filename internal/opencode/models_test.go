package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFallbackProviders(t *testing.T) {
	providers := FallbackProviders()
	if len(providers) == 0 {
		t.Fatal("FallbackProviders should return at least one provider")
	}
	// Check Anthropic is present
	found := false
	for _, p := range providers {
		if p.ID == "anthropic" {
			found = true
			if len(p.Models) == 0 {
				t.Error("Anthropic should have models")
			}
		}
	}
	if !found {
		t.Error("Anthropic provider not found in fallback list")
	}
}

func TestLoadModelsCache_FileNotFound(t *testing.T) {
	_, err := LoadModelsCache(t.TempDir())
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestLoadModelsCache_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".cache", "opencode")
	os.MkdirAll(cacheDir, 0755)

	cacheJSON := `{
		"anthropic": {
			"id": "anthropic",
			"name": "Anthropic",
			"models": {
				"claude-sonnet-4": {
					"id": "claude-sonnet-4",
					"name": "Claude Sonnet 4",
					"tool_call": true
				},
				"claude-haiku-no-tools": {
					"id": "claude-haiku-no-tools",
					"name": "Claude Haiku (no tools)",
					"tool_call": false
				}
			}
		}
	}`
	os.WriteFile(filepath.Join(cacheDir, "models.json"), []byte(cacheJSON), 0644)

	providers, err := LoadModelsCache(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].ID != "anthropic" {
		t.Errorf("provider ID = %q, want %q", providers[0].ID, "anthropic")
	}
	// Should only include tool_call=true models
	if len(providers[0].Models) != 1 {
		t.Errorf("expected 1 model (tool_call=true only), got %d", len(providers[0].Models))
	}
}

func TestDetectProviders_FallsBack(t *testing.T) {
	providers := DetectProviders(t.TempDir())
	if len(providers) == 0 {
		t.Error("DetectProviders should return fallback when no cache")
	}
}
