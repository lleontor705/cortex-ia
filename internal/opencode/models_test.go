package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestParseModelsOutput_Basic(t *testing.T) {
	output := `provider-a/model-a
	provider-a/model-b
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
		if p.ID == "provider-a" {
			if len(p.Models) != 2 {
				t.Errorf("anthropic should have 2 models, got %d", len(p.Models))
			}
		}
	}
}

func TestParseModelsOutput_OpenRouter(t *testing.T) {
	output := `openrouter/provider-a/model-b
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
	found := false
	for _, candidate := range providers[0].Models {
		if candidate.ID == "provider-a/model-b" {
			found = true
		}
	}
	if !found {
		t.Errorf("openrouter models = %#v, want provider-a/model-b", providers[0].Models)
	}
}

func TestParseModelsOutput_EmptyLines(t *testing.T) {
	output := `
	provider-a/model-a

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
		{ID: "provider-a", Models: []model.OpenCodeModel{{ID: "model-a"}}},
		{ID: "openai", Models: []model.OpenCodeModel{{ID: "gpt-4o"}, {ID: "gpt-5"}}},
	}
	list := FlatModelList(providers)
	if len(list) != 3 {
		t.Fatalf("expected 3 models, got %d", len(list))
	}
	if list[0] != "provider-a/model-a" {
		t.Errorf("list[0] = %q, want %q", list[0], "provider-a/model-a")
	}
}

func TestDiscoverModels_NoFallbackReturnsTypedUnresolvedEvidence(t *testing.T) {
	now := time.Now().UTC()
	snapshot, err := DiscoverModels(context.Background(), t.TempDir(), DiscoveryOptions{
		Now: func() time.Time { return now },
		Run: func(context.Context) ([]byte, error) { return nil, errors.New("opencode unavailable") },
	})
	if err == nil {
		t.Fatal("expected unresolved discovery error")
	}
	var unresolved *UnresolvedDiscoveryError
	if !errors.As(err, &unresolved) {
		t.Fatalf("error = %T, want *UnresolvedDiscoveryError", err)
	}
	if len(snapshot.Providers) != 0 {
		t.Fatalf("unresolved discovery returned providers: %#v", snapshot.Providers)
	}
	if snapshot.Evidence.ReasonID != ReasonDiscoveryUnavailable || snapshot.Evidence.Source != SourceDiscovery {
		t.Fatalf("unexpected unresolved evidence: %#v", snapshot.Evidence)
	}
}

func TestDiscoverModels_UsesFreshCLIProvenance(t *testing.T) {
	now := time.Now().UTC()
	snapshot, err := DiscoverModels(context.Background(), t.TempDir(), DiscoveryOptions{
		Now: func() time.Time { return now },
		Run: func(context.Context) ([]byte, error) { return []byte("provider-x/model-y\n"), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := FlatModelList(snapshot.Providers); len(got) != 1 || got[0] != "provider-x/model-y" {
		t.Fatalf("providers = %#v", got)
	}
	if snapshot.Evidence.Source != SourceDiscovery || !snapshot.Evidence.Qualified || snapshot.Evidence.FreshUntil.Before(now) {
		t.Fatalf("missing fresh provenance: %#v", snapshot.Evidence)
	}
}

func TestApplyToOpenCodeConfigResolved_RejectsUnresolvedBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "opencode.json")
	before := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyToOpenCodeConfigResolved(dir, map[string]ResolvedAssignment{
		"implement": {Assignment: model.OpenCodeModelAssignment{Provider: "provider-x", Model: "model-y"}},
	})
	if err == nil {
		t.Fatal("expected unresolved assignment error")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("config mutated on unresolved assignment: %s", after)
	}
}

func TestApplyToOpenCodeConfigResolved_PreservesConfigAndReturnsReceipt(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","agent":{"implement":{"mode":"subagent"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	receipt, err := ApplyToOpenCodeConfigResolved(dir, map[string]ResolvedAssignment{
		"implement": {
			Assignment: model.OpenCodeModelAssignment{Provider: "provider-x", Model: "model-y"},
			Evidence:   DiscoveryEvidence{Source: SourceConfig, ObservedAt: now, FreshUntil: now.Add(time.Hour), Qualified: true, Digest: "cfg-digest"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.BeforeDigest == "" || receipt.AfterDigest == "" || receipt.BeforeDigest == receipt.AfterDigest || receipt.EvidenceDigest != "cfg-digest" {
		t.Fatalf("invalid receipt: %#v", receipt)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !strings.Contains(string(data), "provider-x/model-y") || !strings.Contains(string(data), "dark") {
		t.Fatalf("config was not preserved and updated: %s", data)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != `{"theme":"dark","agent":{"implement":{"mode":"subagent"}}}` {
		t.Fatalf("rollback did not restore original bytes: %s", restored)
	}
}

func TestApplyToOpenCodeConfigResolvedPrefersJSONCAndRollsBackExactBytes(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(configDir, "opencode.json")
	jsonBefore := []byte(`{"agent":{"implement":{"model":"json/model"}}}`)
	if err := os.WriteFile(jsonPath, jsonBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	jsoncPath := filepath.Join(configDir, "opencode.jsonc")
	jsoncBefore := []byte("{\n  // Effective global override.\n  \"agent\": {\n    \"implement\": { \"mode\": \"subagent\" },\n    \"review\": { \"permission\": [\n      // Keep unrelated agent trivia.\n      \"read\",\n    ] },\n  },\n}\n")
	if err := os.WriteFile(jsoncPath, jsoncBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	receipt, err := ApplyToOpenCodeConfigResolved(home, map[string]ResolvedAssignment{
		"implement": {
			Assignment: model.OpenCodeModelAssignment{Provider: "provider-x", Model: "model-y"},
			Evidence:   DiscoveryEvidence{Source: SourceConfig, ObservedAt: now, FreshUntil: now.Add(time.Hour), Qualified: true, Digest: "cfg-digest"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ConfigPath != jsoncPath {
		t.Fatalf("receipt path = %q, want %q", receipt.ConfigPath, jsoncPath)
	}
	jsonAfter, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(jsonAfter) != string(jsonBefore) {
		t.Fatalf("lower-precedence JSON changed: %s", jsonAfter)
	}
	jsoncAfter, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsoncAfter), "// Effective global override.") || !strings.Contains(string(jsoncAfter), "// Keep unrelated agent trivia.") || !strings.Contains(string(jsoncAfter), "provider-x/model-y") {
		t.Fatalf("JSONC was not patched safely:\n%s", jsoncAfter)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(jsoncBefore) {
		t.Fatalf("rollback did not restore exact JSONC bytes:\n%s", restored)
	}
}

func TestConfigReceiptRollbackRejectsConcurrentChange(t *testing.T) {
	home := t.TempDir()
	now := time.Now().UTC()
	receipt, err := ApplyToOpenCodeConfigResolved(home, map[string]ResolvedAssignment{
		"implement": {
			Assignment: model.OpenCodeModelAssignment{Provider: "provider-x", Model: "model-y"},
			Evidence:   DiscoveryEvidence{Source: SourceConfig, ObservedAt: now, FreshUntil: now.Add(time.Hour), Qualified: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	concurrent := []byte(`{"user":"change"}`)
	if err := os.WriteFile(receipt.ConfigPath, concurrent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err == nil {
		t.Fatal("rollback overwrote a concurrent config change")
	}
	after, err := os.ReadFile(receipt.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(concurrent) {
		t.Fatalf("concurrent bytes changed: %s", after)
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
			"orchestrator": map[string]interface{}{
				"mode": "primary",
			},
		},
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(filepath.Join(configDir, "opencode.json"), data, 0644)

	// Apply assignments
	assignments := model.OpenCodeModelAssignments{
		"orchestrator": {Provider: "provider-a", Model: "model-a"},
		"implement":    {Provider: "openai", Model: "gpt-4o"},
	}
	_, err := ApplyToOpenCodeConfig(dir, assignments)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify
	result, _ := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	var config map[string]interface{}
	json.Unmarshal(result, &config)

	agents := config["agent"].(map[string]interface{})

	// Check orchestrator
	orch := agents["orchestrator"].(map[string]interface{})
	if orch["model"] != "provider-a/model-a" {
		t.Errorf("orchestrator model = %q, want %q", orch["model"], "provider-a/model-a")
	}
	if orch["mode"] != "primary" {
		t.Error("existing fields should be preserved")
	}

	// Check implement
	impl := agents["implement"].(map[string]interface{})
	if impl["model"] != "openai/gpt-4o" {
		t.Errorf("implement model = %q, want %q", impl["model"], "openai/gpt-4o")
	}
	if _, hasLegacy := agents["sdd-implement"]; hasLegacy {
		t.Error("should not create legacy sdd-implement model-only entry")
	}

	// Check theme preserved
	if config["theme"] != "dark" {
		t.Error("theme should be preserved")
	}
}

func TestApplyToOpenCodeConfig_DropsLegacyPortableTeamLead(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "opencode.json")
	legacy := `{"agent":{"team-lead":{"model":"legacy"},"implement":{"mode":"subagent"}}}`
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	assignments := model.OpenCodeModelAssignments{
		"team-lead": {Provider: "provider-a", Model: "model-a"},
		"implement": {Provider: "provider-a", Model: "model-a"},
	}

	if _, err := ApplyToOpenCodeConfig(homeDir, assignments); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	agents := config["agent"].(map[string]any)
	if _, exists := agents["team-lead"]; exists {
		t.Fatal("legacy portable team-lead config must not be restored by model assignment")
	}
	if agents["implement"].(map[string]any)["model"] != "provider-a/model-a" {
		t.Fatal("replacement implement model assignment was not applied")
	}
}

func TestApplyToOpenCodeConfig_PreservesLegacyAgentWhenOnlyLegacyExists(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".config", "opencode")
	os.MkdirAll(configDir, 0755)

	initial := map[string]interface{}{
		"agent": map[string]interface{}{
			"sdd-orchestrator": map[string]interface{}{
				"mode": "primary",
			},
		},
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(filepath.Join(configDir, "opencode.json"), data, 0644)

	assignments := model.OpenCodeModelAssignments{
		"orchestrator": {Provider: "provider-a", Model: "model-a"},
	}
	if _, err := ApplyToOpenCodeConfig(dir, assignments); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	var config map[string]interface{}
	json.Unmarshal(result, &config)
	agents := config["agent"].(map[string]interface{})
	legacy := agents["sdd-orchestrator"].(map[string]interface{})
	if legacy["model"] != "provider-a/model-a" {
		t.Errorf("legacy orchestrator model = %q", legacy["model"])
	}
	if _, hasNew := agents["orchestrator"]; hasNew {
		t.Error("should not create a duplicate orchestrator when only legacy key exists")
	}
}

func TestApplyToOpenCodeConfig_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	assignments := model.OpenCodeModelAssignments{
		"orchestrator": {Provider: "provider-a", Model: "model-a"},
	}
	_, err := ApplyToOpenCodeConfig(dir, assignments)
	if err != nil {
		t.Fatalf("should create config if missing: %v", err)
	}

	// Verify file created
	path := filepath.Join(dir, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file should have been created")
	}
}
