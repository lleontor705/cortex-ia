package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestStateAndLockWriteFailuresPreservePreimages(t *testing.T) {
	writeErr := errors.New("injected atomic write failure")

	tests := []struct {
		name     string
		path     func(string) string
		save     func(string) error
		preimage []byte
	}{
		{
			name: "state absent",
			path: StatePath,
			save: func(home string) error {
				return Save(home, State{Version: "v1"})
			},
		},
		{
			name:     "state empty",
			path:     StatePath,
			preimage: []byte{},
			save: func(home string) error {
				return Save(home, State{Version: "v1"})
			},
		},
		{
			name:     "lock existing",
			path:     LockPath,
			preimage: []byte(`{"version":"old"}\n`),
			save: func(home string) error {
				return SaveLock(home, Lockfile{Version: "v1"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			path := tt.path(home)
			if tt.preimage != nil {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, tt.preimage, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			previousWriter := writeFileAtomic
			writeFileAtomic = func(string, []byte, os.FileMode) error {
				return writeErr
			}
			t.Cleanup(func() { writeFileAtomic = previousWriter })

			err := tt.save(home)
			if !errors.Is(err, writeErr) {
				t.Fatalf("Save error = %v, want wrapped %v", err, writeErr)
			}

			got, readErr := os.ReadFile(path)
			if tt.preimage == nil {
				if !os.IsNotExist(readErr) {
					t.Fatalf("metadata exists after failed write: data=%q, error=%v", got, readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != string(tt.preimage) {
				t.Fatalf("metadata changed after failed write: got %q, want %q", got, tt.preimage)
			}
		})
	}
}

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
