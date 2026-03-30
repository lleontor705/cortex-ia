package state

import (
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	s := State{
		InstalledAgents: []model.AgentID{model.AgentClaudeCode, model.AgentOpenCode},
		Preset:          model.PresetFull,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
		LastInstall:     time.Now(),
		LastBackupID:    "backup-001",
		Version:         "dev",
	}

	if err := Save(tmpDir, s); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.InstalledAgents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(loaded.InstalledAgents))
	}
	if loaded.Preset != model.PresetFull {
		t.Errorf("preset = %s", loaded.Preset)
	}
	if loaded.LastBackupID != "backup-001" {
		t.Errorf("backup ID = %s", loaded.LastBackupID)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.InstalledAgents) != 0 {
		t.Error("expected empty state for non-existent file")
	}
}
