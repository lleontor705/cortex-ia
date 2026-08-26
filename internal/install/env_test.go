package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigureEnvironment(t *testing.T) {
	tempHome := t.TempDir()

	if runtime.GOOS != "windows" {
		bashrc := filepath.Join(tempHome, ".bashrc")
		if err := os.WriteFile(bashrc, []byte("# existing bashrc\n"), 0644); err != nil {
			t.Fatalf("failed to create fake bashrc: %v", err)
		}

		if err := ConfigureEnvironment(tempHome); err != nil {
			t.Fatalf("ConfigureEnvironment failed: %v", err)
		}

		data, err := os.ReadFile(bashrc)
		if err != nil {
			t.Fatalf("failed to read bashrc: %v", err)
		}
		content := string(data)
		if !contains(content, EnvBackgroundSubagentsKey) {
			t.Errorf("expected %s in .bashrc, got: %s", EnvBackgroundSubagentsKey, content)
		}
	} else {
		// On Windows, test process env is set
		if err := ConfigureEnvironment(tempHome); err != nil {
			t.Fatalf("ConfigureEnvironment failed on Windows: %v", err)
		}
		if val := os.Getenv(EnvBackgroundSubagentsKey); val != EnvBackgroundSubagentsVal {
			t.Errorf("expected env %s=%s, got %s", EnvBackgroundSubagentsKey, EnvBackgroundSubagentsVal, val)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
