package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name    string
		ldflags string
		want    string
	}{
		{"ldflags set", "v1.2.3", "v1.2.3"},
		{"ldflags with whitespace", "  v1.0.0  ", "v1.0.0"},
		{"ldflags dev falls through", "dev", "dev"},
		{"empty ldflags falls through", "", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveVersion(tt.ldflags)
			if got != tt.want {
				t.Errorf("ResolveVersion(%q) = %q, want %q", tt.ldflags, got, tt.want)
			}
		})
	}
}

func TestGitDescribeVersionRestrictedToProjectRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	t.Run("outside any repository is empty", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if got := GitDescribeVersion(); got != "" {
			t.Errorf("GitDescribeVersion() outside a repository = %q, want empty", got)
		}
	})

	t.Run("unrelated repository module is ignored", func(t *testing.T) {
		dir := initVersionSyntheticRepo(t, "example.com/other", "v9.9.9")
		t.Chdir(dir)
		if got := GitDescribeVersion(); got != "" {
			t.Errorf("GitDescribeVersion() in an unrelated repository = %q, want empty", got)
		}
	})

	t.Run("project repository is described", func(t *testing.T) {
		dir := initVersionSyntheticRepo(t, "github.com/"+updater.DefaultRepo, "v1.2.3")
		t.Chdir(dir)
		if got := GitDescribeVersion(); got != "v1.2.3" {
			t.Errorf("GitDescribeVersion() in the project repository = %q, want v1.2.3", got)
		}
	})
}

// initVersionSyntheticRepo builds a one-commit git repository whose go.mod
// declares modulePath and whose HEAD is tagged tag.
func initVersionSyntheticRepo(t *testing.T, modulePath, tag string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.26.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "go.mod"},
		{"-c", "user.email=test@example.com", "-c", "user.name=cortex-ia-test", "commit", "-m", "init"},
		{"tag", tag},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}
