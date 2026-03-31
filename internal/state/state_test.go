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

func TestSaveAndLoadLock(t *testing.T) {
	tmpDir := t.TempDir()

	lock := Lockfile{
		InstalledAgents: []model.AgentID{model.AgentCodex},
		Preset:          model.PresetMinimal,
		Components:      []model.ComponentID{model.ComponentCortex, model.ComponentSDD},
		Files:           []string{"C:/Users/test/.codex/agents.md", "C:/Users/test/.codex/config.toml"},
		GeneratedAt:     time.Now(),
		LastBackupID:    "backup-123",
		Version:         "v0.1.0",
	}

	if err := SaveLock(tmpDir, lock); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLock(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(loaded.Files))
	}
	if loaded.LastBackupID != "backup-123" {
		t.Errorf("backup ID = %s", loaded.LastBackupID)
	}
}
