package herdr

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetHerdrConfigDir(t *testing.T) {
	dir, err := GetHerdrConfigDir()
	if err != nil {
		t.Fatalf("GetHerdrConfigDir failed: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty config dir")
	}
	if runtime.GOOS == "windows" {
		if !contains(dir, "herdr") {
			t.Errorf("expected 'herdr' in path, got %s", dir)
		}
	} else {
		if !contains(dir, ".config") && !contains(dir, "herdr") {
			t.Errorf("expected config path, got %s", dir)
		}
	}
}

func TestFirstRegular(t *testing.T) {
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "fake-binary")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	if err := os.WriteFile(fakeBin, []byte("echo binary"), 0755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	// Found case
	found, err := firstRegular([]string{"/non/existent/path", fakeBin}, "fake")
	if err != nil {
		t.Fatalf("expected to find binary, got error: %v", err)
	}
	if found != fakeBin {
		t.Errorf("expected %s, got %s", fakeBin, found)
	}

	// Not found case
	_, err = firstRegular([]string{"/non/existent/1", "/non/existent/2"}, "fake")
	if err == nil {
		t.Error("expected error for non-existent binary")
	}
}

func TestResolveBinaries(t *testing.T) {
	// Should not panic or crash
	_, _ = ResolveHerdr()
	_, _ = ResolveAGY()
}

func TestStatus(t *testing.T) {
	// Should execute without panic
	err := Status()
	if err != nil {
		t.Errorf("Status() returned error: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
