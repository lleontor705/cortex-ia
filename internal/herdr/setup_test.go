package herdr

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
	_, _ = ResolveHerdr()
}
