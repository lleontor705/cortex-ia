package agentbuilder

import (
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestLoadRegistry_MissingFile(t *testing.T) {
	reg, err := LoadRegistry(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if reg.Version != 1 {
		t.Errorf("Version = %d, want 1", reg.Version)
	}
	if len(reg.Agents) != 0 {
		t.Errorf("expected empty Agents, got %v", reg.Agents)
	}
}

func TestSaveAndLoad_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	reg := &Registry{Version: 1, Agents: []RegistryEntry{
		{Name: "my-skill", Engine: model.AgentClaudeCode, SDDMode: SDDStandalone},
	}}
	if err := SaveRegistry(path, reg); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "my-skill" {
		t.Errorf("roundtrip lost data: %+v", got)
	}
}

func TestRegistry_AddFindRemove(t *testing.T) {
	r := &Registry{Version: 1}
	r.Add(RegistryEntry{Name: "a"})
	r.Add(RegistryEntry{Name: "b"})

	if got := r.FindByName("a"); got == nil || got.Name != "a" {
		t.Errorf("FindByName(a) failed: %v", got)
	}
	if got := r.FindByName("missing"); got != nil {
		t.Errorf("FindByName(missing) should be nil, got %v", got)
	}

	if !r.RemoveByName("a") {
		t.Error("RemoveByName(a) returned false")
	}
	if r.FindByName("a") != nil {
		t.Error("a still present after removal")
	}
	if r.RemoveByName("missing") {
		t.Error("RemoveByName(missing) returned true")
	}
}

func TestHasConflictWithBuiltin(t *testing.T) {
	if !HasConflictWithBuiltin("sdd-init") {
		t.Error("sdd-init should be flagged as builtin conflict")
	}
	if !HasConflictWithBuiltin(string(model.SkillJudgmentDay)) {
		t.Error("judgment-day should be flagged as builtin conflict")
	}
	if HasConflictWithBuiltin("totally-novel-skill") {
		t.Error("user-named skill should not collide")
	}
}
