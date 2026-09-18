package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLegacyState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := StatePath(tmpDir)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	s := State{
		InstalledAgents: []string{"claude", "opencode"},
		Preset:          "full",
		Components:      []string{"cortex", "sdd"},
		LastBackupID:    "backup-001",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.InstalledAgents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(loaded.InstalledAgents))
	}
	if loaded.Preset != "full" {
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

func TestLoadLegacyLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := LockPath(tmpDir)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}

	lock := Lockfile{
		InstalledAgents: []string{"codex"},
		Preset:          "minimal",
		Components:      []string{"cortex", "sdd"},
		Files:           []string{"C:/Users/test/.codex/agents.md", "C:/Users/test/.codex/config.toml"},
		LastBackupID:    "backup-123",
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
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

func TestDeterministicFingerprintSalt(t *testing.T) {
	home1 := "/path/to/home1"
	home2 := "/path/to/home2"

	salt1A := DeterministicFingerprintSalt(home1)
	salt1B := DeterministicFingerprintSalt(home1)
	if salt1A == "" || salt1A != salt1B {
		t.Errorf("expected deterministic salt across calls: %s != %s", salt1A, salt1B)
	}

	salt2 := DeterministicFingerprintSalt(home2)
	if salt1A == salt2 {
		t.Errorf("expected different salts for different homes: %s == %s", salt1A, salt2)
	}

	doc := FingerprintDocument{Salt: salt1A}
	bytes, err := doc.SaltBytes()
	if err != nil {
		t.Fatalf("SaltBytes failed on deterministic salt: %v", err)
	}
	if len(bytes) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(bytes))
	}
}
