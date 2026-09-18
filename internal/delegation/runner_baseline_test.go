package delegation

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestChangedWorktreePaths_GitOperations(t *testing.T) {
	tempDir := t.TempDir()

	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = tempDir
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Skipf("git not available or init failed: %v (%s)", err, string(out))
	}
	cmdEmail := exec.Command("git", "config", "user.email", "test@test.com")
	cmdEmail.Dir = tempDir
	_ = cmdEmail.Run()
	cmdName := exec.Command("git", "config", "user.name", "Test")
	cmdName.Dir = tempDir
	_ = cmdName.Run()

	file1 := filepath.Join(tempDir, "tracked.txt")
	fileToDelete := filepath.Join(tempDir, "to_delete.txt")
	if err := os.WriteFile(file1, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(fileToDelete, []byte("delete me\n"), 0o644); err != nil {
		t.Fatalf("write fileToDelete: %v", err)
	}

	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = tempDir
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}
	cmdCommit := exec.Command("git", "commit", "-m", "initial commit")
	cmdCommit.Dir = tempDir
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}

	cleanPaths, err := changedWorktreePaths(tempDir)
	if err != nil {
		t.Fatalf("changedWorktreePaths clean repo: %v", err)
	}
	if len(cleanPaths) != 0 {
		t.Fatalf("expected 0 changed paths on clean repo, got: %v", cleanPaths)
	}

	if err := os.WriteFile(file1, []byte("modified hello\n"), 0o644); err != nil {
		t.Fatalf("modify file1: %v", err)
	}
	untracked := filepath.Join(tempDir, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}
	if err := os.Remove(fileToDelete); err != nil {
		t.Fatalf("remove fileToDelete: %v", err)
	}

	changed, err := changedWorktreePaths(tempDir)
	if err != nil {
		t.Fatalf("changedWorktreePaths with changes: %v", err)
	}

	expected := []string{"to_delete.txt", "tracked.txt", "untracked.txt"}
	for _, exp := range expected {
		if !slices.Contains(changed, exp) {
			t.Errorf("expected changed paths to contain %q, got: %v", exp, changed)
		}
	}
}
