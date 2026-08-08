package cortex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SyncManifest holds metadata for the exported git-shared project memory.
type SyncManifest struct {
	SchemaVersion string    `json:"schema_version"`
	Project       string    `json:"project"`
	ExportedAt    time.Time `json:"exported_at"`
	ObsCount      int       `json:"observation_count"`
	RelCount      int       `json:"relationship_count"`
}

// ObservationRecord represents an observation exported to .cortex/observations.jsonl.
type ObservationRecord struct {
	ID        string   `json:"id"`
	TopicKey  string   `json:"topic_key"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// ExportProjectMemory exports local Cortex observations for project to .cortex/ in repoRoot.
func ExportProjectMemory(repoRoot string, projectName string) error {
	cortexDir := filepath.Join(repoRoot, ".cortex")
	if err := os.MkdirAll(cortexDir, 0o755); err != nil {
		return fmt.Errorf("create .cortex dir: %w", err)
	}

	manifest := SyncManifest{
		SchemaVersion: "1.0.0",
		Project:       projectName,
		ExportedAt:    time.Now().UTC(),
		ObsCount:      0,
		RelCount:      0,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(cortexDir, "manifest.json"), manifestData, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Ensure empty observations.jsonl exists if none exist
	obsPath := filepath.Join(cortexDir, "observations.jsonl")
	if _, err := os.Stat(obsPath); os.IsNotExist(err) {
		if err := os.WriteFile(obsPath, []byte(""), 0o644); err != nil {
			return fmt.Errorf("create empty observations.jsonl: %w", err)
		}
	}

	return nil
}

// ImportProjectMemory imports observations from .cortex/ in repoRoot into local Cortex.
func ImportProjectMemory(repoRoot string) (*SyncManifest, error) {
	cortexDir := filepath.Join(repoRoot, ".cortex")
	manifestPath := filepath.Join(cortexDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no .cortex/manifest.json found in repo at %s", repoRoot)
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest SyncManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &manifest, nil
}
