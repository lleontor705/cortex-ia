package delegation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateWorktreeContract(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	wtDir := filepath.Join(tempDir, "worktrees", "task-1")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v unavailable or failed: %v (%s)", args, err, string(out))
		}
	}
	git("init")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Initial Repo\n"), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	git("add", ".")
	git("commit", "-m", "initial commit")
	git("worktree", "add", "--detach", wtDir, "HEAD")

	rec, err := ValidateWorktreeContract(repoDir, wtDir, "")
	if err != nil {
		t.Fatalf("ValidateWorktreeContract failed: %v", err)
	}
	if rec == nil || !SameWorkspace(rec.Path, wtDir) {
		t.Fatalf("expected valid record for %s", wtDir)
	}

	if _, err := ValidateWorktreeContract(repoDir, filepath.Join(tempDir, "missing"), ""); err == nil {
		t.Fatal("expected validation failure for missing worktree path")
	}
}
