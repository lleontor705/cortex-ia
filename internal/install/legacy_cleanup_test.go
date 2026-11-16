package install

import (
	"os"
	"path/filepath"
	"testing"
)

// Legacy flat pane and authority files are removed by exact prefix and suffix;
// unrelated files that share only a leading fragment survive.
func TestCleanupLegacyFlatFilesMatchesPrefixAndSuffix(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	removed := []string{
		"cortex-authority-state-abc.json",
		"cortex-delegation-panes-1.json",
		"cortex-delegation-events.jsonl",
	}
	kept := []string{
		"cortex-delegation-panes-1.txt",
		"cortex-delegation-pane-1.json",
		"cortex-authority-state-abc.jsonl",
		"opencode.jsonc",
	}
	for _, name := range append(append([]string{}, removed...), kept...) {
		if err := os.WriteFile(filepath.Join(configDir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := CleanupLegacyFlatFiles(home); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	for _, name := range removed {
		if _, err := os.Stat(filepath.Join(configDir, name)); !os.IsNotExist(err) {
			t.Errorf("legacy file %s was not removed (err=%v)", name, err)
		}
	}
	for _, name := range kept {
		if _, err := os.Stat(filepath.Join(configDir, name)); err != nil {
			t.Errorf("unrelated file %s was removed: %v", name, err)
		}
	}
}
