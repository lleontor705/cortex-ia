package state

import (
	"os"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestSaveAndLoadProfiles(t *testing.T) {
	tmpDir := t.TempDir()

	profiles := []model.Profile{
		{
			Name: "balanced",
			ModelAssignments: map[string]model.ClaudeModelAlias{
				"sdd-explore": model.ModelSonnet,
				"sdd-propose": model.ModelOpus,
			},
		},
		{
			Name: "economy",
			ModelAssignments: map[string]model.ClaudeModelAlias{
				"sdd-explore": model.ModelHaiku,
			},
		},
	}

	if err := SaveProfiles(tmpDir, profiles); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	loaded, err := LoadProfiles(tmpDir)
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(loaded))
	}
	if loaded[0].Name != "balanced" {
		t.Errorf("expected first profile name 'balanced', got %q", loaded[0].Name)
	}
	if loaded[1].Name != "economy" {
		t.Errorf("expected second profile name 'economy', got %q", loaded[1].Name)
	}
	if loaded[0].ModelAssignments["sdd-explore"] != model.ModelSonnet {
		t.Errorf("expected sdd-explore=sonnet, got %q", loaded[0].ModelAssignments["sdd-explore"])
	}
}

func TestLoadProfiles_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	profiles, err := LoadProfiles(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profiles != nil {
		t.Errorf("expected nil profiles for non-existent file, got %v", profiles)
	}
}

func TestSaveProfiles_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()

	profiles := []model.Profile{
		{Name: "test-profile"},
	}

	if err := SaveProfiles(tmpDir, profiles); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	path := ProfilesPath(tmpDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected profiles file to exist at %s", path)
	}
}
