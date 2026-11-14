package delegation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeRecord represents an authoritative Git worktree entry from git worktree list --porcelain.
type WorktreeRecord struct {
	Path     string `json:"path"`
	HEAD     string `json:"head"`
	Branch   string `json:"branch,omitempty"`
	Detached bool   `json:"detached,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
	Prunable bool   `json:"prunable,omitempty"`
}

// ParseWorktreePorcelain parses the raw output of git worktree list --porcelain.
func ParseWorktreePorcelain(output string) ([]WorktreeRecord, error) {
	var records []WorktreeRecord
	var current *WorktreeRecord

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != nil {
				records = append(records, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if current != nil {
				records = append(records, *current)
			}
			rawPath := strings.TrimPrefix(line, "worktree ")
			canonical, err := CanonicalWorkspace(rawPath)
			if err != nil {
				canonical = filepath.ToSlash(filepath.Clean(rawPath))
			}
			current = &WorktreeRecord{
				Path: canonical,
			}
		} else if current != nil {
			if strings.HasPrefix(line, "HEAD ") {
				current.HEAD = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			} else if strings.HasPrefix(line, "branch ") {
				current.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			} else if trimmed == "detached" {
				current.Detached = true
			} else if trimmed == "bare" {
				current.Bare = true
			} else if trimmed == "locked" || strings.HasPrefix(trimmed, "locked ") {
				current.Locked = true
			} else if trimmed == "prunable" || strings.HasPrefix(trimmed, "prunable ") {
				current.Prunable = true
			}
		}
	}
	if current != nil {
		records = append(records, *current)
	}
	return records, nil
}

// ListWorktrees executes git worktree list --porcelain in repoPath and parses the authoritative records.
func ListWorktrees(repoPath string) ([]WorktreeRecord, error) {
	if strings.TrimSpace(repoPath) == "" {
		repoPath = "."
	}
	repoPath = filepath.Clean(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list --porcelain failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return ParseWorktreePorcelain(string(out))
}

// ResolveWorktree looks up targetPath against the authoritative Git worktree records of repoPath.
func ResolveWorktree(repoPath, targetPath string) (*WorktreeRecord, error) {
	records, err := ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}
	targetCanonical, err := CanonicalWorkspace(targetPath)
	if err != nil {
		targetCanonical = filepath.ToSlash(filepath.Clean(targetPath))
	}
	for i := range records {
		if SameWorkspace(records[i].Path, targetCanonical) {
			return &records[i], nil
		}
	}
	return nil, fmt.Errorf("worktree %q not found in repository worktrees", targetPath)
}

// ValidateWorktreeContract verifies that targetPath exists on disk, is an authoritative
// Git worktree of repoPath according to git worktree list --porcelain, and that its HEAD
// commit matches expectedHEAD (if provided). Fails closed on any mismatch.
func ValidateWorktreeContract(repoPath, targetPath, expectedHEAD string) (*WorktreeRecord, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil, errors.New("target worktree path is required")
	}
	targetClean := filepath.Clean(targetPath)
	info, err := os.Stat(targetClean)
	if err != nil {
		return nil, fmt.Errorf("target worktree path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target worktree path %q is not a directory", targetPath)
	}

	record, err := ResolveWorktree(repoPath, targetClean)
	if err != nil {
		return nil, fmt.Errorf("worktree contract validation failed: %w", err)
	}

	expectedHEAD = strings.TrimSpace(expectedHEAD)
	if expectedHEAD != "" {
		if !strings.HasPrefix(record.HEAD, expectedHEAD) && !strings.HasPrefix(expectedHEAD, record.HEAD) {
			return nil, fmt.Errorf("worktree HEAD mismatch: expected %s, git reports %s", expectedHEAD, record.HEAD)
		}
	}

	return record, nil
}
