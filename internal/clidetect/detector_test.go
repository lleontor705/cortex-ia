package clidetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAll_TempHome(t *testing.T) {
	tempHome := t.TempDir()

	// Simulate fake config directories
	opencodeConfig := filepath.Join(tempHome, ".config", "opencode")
	if err := os.MkdirAll(opencodeConfig, 0o755); err != nil {
		t.Fatal(err)
	}

	geminiConfig := filepath.Join(tempHome, ".gemini")
	if err := os.MkdirAll(geminiConfig, 0o755); err != nil {
		t.Fatal(err)
	}

	results := DetectAll(tempHome)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	foundOpenCode := false
	foundAGY := false
	foundClaude := false

	for _, info := range results {
		switch info.ID {
		case CLIOpenCode:
			foundOpenCode = true
			if !info.ConfigFound {
				t.Errorf("expected opencode config to be found")
			}
		case CLIAGY:
			foundAGY = true
			if !info.ConfigFound {
				t.Errorf("expected agy config to be found")
			}
		case CLIClaude:
			foundClaude = true
			if info.ConfigFound {
				t.Errorf("expected claude config not to be found")
			}
		}
	}

	if !foundOpenCode || !foundAGY || !foundClaude {
		t.Errorf("missing expected CLIs in DetectAll results")
	}
}

func TestResolveBinary_Missing(t *testing.T) {
	_, err := resolveBinary("nonexistent_cli_12345", []string{"/tmp/fake1", "/tmp/fake2"})
	if err == nil {
		t.Errorf("expected error for nonexistent binary, got nil")
	}
}
