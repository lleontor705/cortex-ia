package config_test

// Integration test that proves the full chain:
//
//	.cortex-ia.yaml (profile: cheap)
//	   → config.LoadFile
//	   → config.ApplyToSelection
//	   → pipeline.Install (with opencode adapter + profiles.json on disk)
//	   → opencode.json gains model entries on the real SDD agent names
//
// If any link in this chain regresses, the test fails immediately and surfaces
// the broken hop in its assertion.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/config"
	"github.com/lleontor705/cortex-ia/internal/model"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
	"github.com/lleontor705/cortex-ia/internal/state"
)

const yamlSample = `
preset: full
persona: professional
profile: cheap
agents:
  - opencode
`

func TestYAMLProfile_FlowsThroughInstallToOpencodeJSON(t *testing.T) {
	homeDir := t.TempDir()

	// 1. Persist a profile that the yaml will reference by name.
	profiles := []model.Profile{{
		Name: "cheap",
		ModelAssignments: model.ModelAssignments{
			"sdd-design": "openai/gpt-4o-mini",
			"sdd-apply":  "anthropic/claude-haiku-4-5",
		},
	}}
	if err := state.SaveProfiles(homeDir, profiles); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	// 2. Write .cortex-ia.yaml in a separate "project" dir and load it.
	projectDir := t.TempDir()
	yamlPath := filepath.Join(projectDir, config.FileName)
	if err := os.WriteFile(yamlPath, []byte(yamlSample), 0o644); err != nil {
		t.Fatalf("WriteFile yaml: %v", err)
	}
	cfg, err := config.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.Profile != "cheap" {
		t.Fatalf("yaml load lost Profile field: got %q", cfg.Profile)
	}

	// 3. Apply yaml to a Selection.
	sel := model.Selection{}
	config.ApplyToSelection(cfg, &sel)
	if sel.ProfileName != "cheap" {
		t.Fatalf("ApplyToSelection did not set ProfileName: got %q", sel.ProfileName)
	}
	if len(sel.Agents) != 1 || sel.Agents[0] != model.AgentOpenCode {
		t.Fatalf("agents not propagated: %v", sel.Agents)
	}

	// 4. Run install with a tiny registry containing only opencode.
	reg := agents.NewRegistry()
	reg.Register(opencode.NewAdapter())
	if _, err := pipeline.Install(homeDir, reg, sel, "test-v1", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// 5. opencode.json must contain the per-phase models from the profile.
	cfgPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile opencode.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, string(data))
	}
	agentSection, ok := parsed["agent"].(map[string]any)
	if !ok {
		t.Fatalf("agent section missing: %s", string(data))
	}
	design, _ := agentSection["architect"].(map[string]any)
	if design == nil || design["model"] != "openai/gpt-4o-mini" {
		t.Errorf("architect.model = %v, want openai/gpt-4o-mini\n%s", design, string(data))
	}
	apply, _ := agentSection["team-lead"].(map[string]any)
	if apply == nil || apply["model"] != "anthropic/claude-haiku-4-5" {
		t.Errorf("team-lead.model = %v\n%s", apply, string(data))
	}
	worker, _ := agentSection["implement"].(map[string]any)
	if worker == nil || worker["model"] != "anthropic/claude-haiku-4-5" {
		t.Errorf("implement.model = %v\n%s", worker, string(data))
	}
	if _, hasLegacy := agentSection["sdd-apply"]; hasLegacy {
		t.Errorf("legacy sdd-apply entry should not be created\n%s", string(data))
	}

	// 6. state.json should record the active profile so subsequent `sync` reuses it.
	st, err := state.Load(homeDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if st.LastProfile != "cheap" {
		t.Errorf("state.LastProfile = %q, want cheap", st.LastProfile)
	}
}
