package mcpserve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

func TestDiscoveryWrite_WorkspaceReachedThroughUserSymlink(t *testing.T) {
	realProject := t.TempDir()
	link := filepath.Join(t.TempDir(), "projects", "app")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realProject, link); err != nil {
		t.Skipf("symlink creation not permitted: %v", err)
	}

	s := newSessionWith(context.Background(), hostbridge.HostClaude, t.TempDir(), link, nil)
	args := map[string]any{"content": discoveryHeading + "\n\nproject facts"}
	if _, err := discoveryWrite(context.Background(), s, args); err != nil {
		t.Fatalf("discovery write through user symlink rejected: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(realProject, ".cortex-ia", "discovery.md"))
	if err != nil {
		t.Fatalf("discovery.md not written to the real project: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("discovery.md is empty")
	}
}
