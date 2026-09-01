package delegation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorktreeLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	wtDir := filepath.Join(tempDir, "worktrees", "task-1")

	// 1. Initialize a real git repo
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = repoDir
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Skipf("git not available or init failed: %v (%s)", err, string(out))
	}
	cmdEmail := exec.Command("git", "config", "user.email", "test@test.com")
	cmdEmail.Dir = repoDir
	if out, err := cmdEmail.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v (%s)", err, string(out))
	}
	cmdName := exec.Command("git", "config", "user.name", "Test")
	cmdName.Dir = repoDir
	if out, err := cmdName.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v (%s)", err, string(out))
	}

	testFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Initial Repo\n"), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = repoDir
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v (%s)", err, string(out))
	}
	cmdCommit := exec.Command("git", "commit", "-m", "initial commit")
	cmdCommit.Dir = repoDir
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (%s)", err, string(out))
	}

	// 2. Create Ephemeral Worktree
	wt, err := CreateEphemeralWorktree(repoDir, wtDir)
	if err != nil {
		t.Fatalf("CreateEphemeralWorktree failed: %v", err)
	}
	if wt != wtDir {
		t.Errorf("expected wt path %s, got %s", wtDir, wt)
	}

	// 3. Clean Worktree (modify and clean)
	wtFile := filepath.Join(wtDir, "untracked.txt")
	_ = os.WriteFile(wtFile, []byte("dirty file"), 0644)
	if err := CleanWorktree(wtDir); err != nil {
		t.Fatalf("CleanWorktree failed: %v", err)
	}
	if _, err := os.Stat(wtFile); !os.IsNotExist(err) {
		t.Errorf("expected untracked file to be cleaned")
	}

	// 4. Drop Worktree
	if err := DropEphemeralWorktree(repoDir, wtDir); err != nil {
		t.Fatalf("DropEphemeralWorktree failed: %v", err)
	}
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree to be dropped")
	}
}
