package delegation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateEphemeralWorktree creates a clean Git worktree for isolated task execution.
func CreateEphemeralWorktree(repoPath, worktreePath string) (string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("base repository path is required")
	}
	repoPath = filepath.Clean(repoPath)
	if strings.TrimSpace(worktreePath) == "" {
		return "", errors.New("worktree destination path is required")
	}
	worktreePath = filepath.Clean(worktreePath)
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", fmt.Errorf("create worktree parent directory: %w", err)
	}
	// If worktree directory already exists, ensure it is clean
	if _, err := os.Stat(worktreePath); err == nil {
		if err := CleanWorktree(worktreePath); err != nil {
			return "", err
		}
		return worktreePath, nil
	}
	// git worktree add --detach <worktreePath> HEAD
	cmd := exec.Command("git", "worktree", "add", "--detach", worktreePath, "HEAD")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return worktreePath, nil
}

// CleanWorktree resets tracked modifications and removes untracked files in the worktree.
func CleanWorktree(worktreePath string) error {
	worktreePath = filepath.Clean(worktreePath)
	if _, err := os.Stat(worktreePath); err != nil {
		return err
	}
	// git reset --hard HEAD
	cmdReset := exec.Command("git", "reset", "--hard", "HEAD")
	cmdReset.Dir = worktreePath
	if out, err := cmdReset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset in worktree failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// git clean -fd
	cmdClean := exec.Command("git", "clean", "-fd")
	cmdClean.Dir = worktreePath
	if out, err := cmdClean.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean in worktree failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DropEphemeralWorktree removes the worktree and prunes git worktree references.
func DropEphemeralWorktree(repoPath, worktreePath string) error {
	worktreePath = filepath.Clean(worktreePath)
	if repoPath == "" {
		repoPath = filepath.Dir(worktreePath)
	}
	// git worktree remove --force <worktreePath>
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	_ = cmd.Run()
	_ = os.RemoveAll(worktreePath)
	// git worktree prune
	cmdPrune := exec.Command("git", "worktree", "prune")
	cmdPrune.Dir = repoPath
	_ = cmdPrune.Run()
	return nil
}
