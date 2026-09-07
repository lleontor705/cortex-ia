package delegation

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// WorktreeOptions defines parameters for creating and configuring a Git worktree.
type WorktreeOptions struct {
	RepoPath     string `json:"repo_path,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	Branch       string `json:"branch,omitempty"`
	BaseRef      string `json:"base_ref,omitempty"`
	Detach       bool   `json:"detach,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
}

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

var invalidPathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// DefaultWorktreeBaseDir returns the root directory for centrally managed worktrees:
// $CORTEX_IA_HOME/worktrees or ~/.cortex-ia/worktrees.
func DefaultWorktreeBaseDir() (string, error) {
	cortexHome := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME"))
	if cortexHome != "" {
		return filepath.Join(cortexHome, "worktrees"), nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(userHome, ".cortex-ia", "worktrees"), nil
}

// RepoSlug generates a deterministic and safe slug identifying repoPath.
func RepoSlug(repoPath string) string {
	clean := filepath.Clean(repoPath)
	canonical, err := CanonicalWorkspace(clean)
	if err != nil || canonical == "" {
		canonical = filepath.ToSlash(clean)
	}
	baseName := filepath.Base(clean)
	if baseName == "." || baseName == "/" || baseName == "\\" || baseName == "" {
		baseName = "repo"
	}
	safeBase := invalidPathChars.ReplaceAllString(baseName, "-")
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%s-%x", safeBase, h[:4])
}

// DefaultWorktreeDir returns the standard managed worktree path for the given repository and identifier.
func DefaultWorktreeDir(repoPath, identifier string) (string, error) {
	baseDir, err := DefaultWorktreeBaseDir()
	if err != nil {
		return "", err
	}
	slug := RepoSlug(repoPath)
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = fmt.Sprintf("wt-%s", time.Now().Format("20060102-150405"))
	}
	safeIdentifier := invalidPathChars.ReplaceAllString(identifier, "-")
	safeIdentifier = strings.Trim(safeIdentifier, "-")
	if safeIdentifier == "" {
		safeIdentifier = "default"
	}
	return filepath.Join(baseDir, slug, safeIdentifier), nil
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

// CreateManagedWorktree creates an isolated worktree based on WorktreeOptions.
func CreateManagedWorktree(opts WorktreeOptions) (*WorktreeRecord, error) {
	repoPath := strings.TrimSpace(opts.RepoPath)
	if repoPath == "" {
		repoPath = "."
	}
	repoClean, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}
	opts.RepoPath = repoClean

	worktreePath := strings.TrimSpace(opts.WorktreePath)
	if worktreePath == "" {
		identifier := strings.TrimSpace(opts.Branch)
		if identifier == "" {
			identifier = strings.TrimSpace(opts.TaskID)
		}
		autoDir, err := DefaultWorktreeDir(opts.RepoPath, identifier)
		if err != nil {
			return nil, fmt.Errorf("generate default worktree dir: %w", err)
		}
		worktreePath = autoDir
	}

	worktreeClean, err := filepath.Abs(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("resolve worktree path: %w", err)
	}
	worktreePath = filepath.Clean(worktreeClean)

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return nil, fmt.Errorf("create worktree parent directory: %w", err)
	}

	// If worktree already exists as an authoritative worktree, clean and return it
	if record, err := ResolveWorktree(opts.RepoPath, worktreePath); err == nil {
		if err := CleanWorktree(worktreePath); err != nil {
			return nil, fmt.Errorf("clean existing worktree: %w", err)
		}
		return record, nil
	}

	// If directory exists on disk but is not an authoritative worktree:
	if fi, err := os.Stat(worktreePath); err == nil && fi.IsDir() {
		entries, readErr := os.ReadDir(worktreePath)
		if readErr == nil && len(entries) > 0 {
			baseDir, _ := DefaultWorktreeBaseDir()
			if baseDir != "" && strings.HasPrefix(worktreePath, filepath.Clean(baseDir)) {
				_ = os.RemoveAll(worktreePath)
			} else {
				return nil, fmt.Errorf("target worktree directory %q already exists and is not empty", worktreePath)
			}
		}
	}

	baseRef := strings.TrimSpace(opts.BaseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}

	var gitArgs []string
	branch := strings.TrimSpace(opts.Branch)

	if branch != "" {
		// Check if branch already exists
		cmdCheck := exec.Command("git", "-C", opts.RepoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		cmdCheck.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		if cmdCheck.Run() == nil {
			// Branch exists: checkout into worktree
			gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", worktreePath, branch}
		} else {
			// Branch does not exist: create and checkout
			gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", "-b", branch, worktreePath, baseRef}
		}
	} else if opts.Detach {
		gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", "--detach", worktreePath, baseRef}
	} else if strings.TrimSpace(opts.TaskID) != "" {
		// Auto-derive a safe branch from TaskID
		taskBranch := "task/" + invalidPathChars.ReplaceAllString(strings.TrimSpace(opts.TaskID), "-")
		cmdCheck := exec.Command("git", "-C", opts.RepoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+taskBranch)
		cmdCheck.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		if cmdCheck.Run() == nil {
			gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", worktreePath, taskBranch}
		} else {
			gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", "-b", taskBranch, worktreePath, baseRef}
		}
	} else {
		// Default to detached HEAD
		gitArgs = []string{"-C", opts.RepoPath, "worktree", "add", "--detach", worktreePath, baseRef}
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return ResolveWorktree(opts.RepoPath, worktreePath)
}

// CreateEphemeralWorktree creates a clean Git worktree for isolated task execution.
// Preserved for backwards compatibility with earlier callers.
func CreateEphemeralWorktree(repoPath, worktreePath string) (string, error) {
	record, err := CreateManagedWorktree(WorktreeOptions{
		RepoPath:     repoPath,
		WorktreePath: worktreePath,
		Detach:       true,
	})
	if err != nil {
		return "", err
	}
	return record.Path, nil
}

// CleanWorktree resets tracked modifications and removes untracked files in the worktree.
func CleanWorktree(worktreePath string) error {
	worktreePath = filepath.Clean(worktreePath)
	if _, err := os.Stat(worktreePath); err != nil {
		return err
	}
	// git reset --hard HEAD
	cmdReset := exec.Command("git", "-C", worktreePath, "reset", "--hard", "HEAD")
	cmdReset.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if out, err := cmdReset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset in worktree failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// git clean -fd
	cmdClean := exec.Command("git", "-C", worktreePath, "clean", "-fd")
	cmdClean.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if out, err := cmdClean.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean in worktree failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DropEphemeralWorktree removes the worktree and prunes git worktree references.
// Includes backoff retry for Windows filesystem handles that may be temporarily locked.
func DropEphemeralWorktree(repoPath, worktreePath string) error {
	worktreePath = filepath.Clean(worktreePath)
	if repoPath == "" {
		repoPath = filepath.Dir(worktreePath)
	}
	// git worktree remove --force <worktreePath>
	cmd := exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	_ = cmd.Run()

	// Windows file locks may linger momentarily; retry remove up to 3 times
	var removeErr error
	for attempt := 0; attempt < 3; attempt++ {
		removeErr = os.RemoveAll(worktreePath)
		if removeErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// git worktree prune
	cmdPrune := exec.Command("git", "-C", repoPath, "worktree", "prune")
	cmdPrune.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	_ = cmdPrune.Run()
	return removeErr
}

// PruneOrphanWorktrees cleans unreferenced worktrees from git and the managed worktree folder.
func PruneOrphanWorktrees(repoPath string) ([]string, error) {
	if strings.TrimSpace(repoPath) == "" {
		repoPath = "."
	}
	repoClean, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	cmdPrune := exec.Command("git", "-C", repoClean, "worktree", "prune")
	cmdPrune.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if out, err := cmdPrune.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree prune failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	records, err := ListWorktrees(repoClean)
	if err != nil {
		return nil, err
	}

	activeMap := make(map[string]struct{}, len(records))
	for _, rec := range records {
		activeMap[rec.Path] = struct{}{}
	}

	baseDir, err := DefaultWorktreeBaseDir()
	if err != nil {
		return nil, nil
	}
	slug := RepoSlug(repoClean)
	repoWorktreeDir := filepath.Join(baseDir, slug)

	entries, err := os.ReadDir(repoWorktreeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pruned []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(repoWorktreeDir, entry.Name())
		canonical, err := CanonicalWorkspace(dirPath)
		if err != nil {
			canonical = filepath.ToSlash(filepath.Clean(dirPath))
		}
		if _, active := activeMap[canonical]; !active {
			if err := os.RemoveAll(dirPath); err == nil || os.IsNotExist(err) {
				pruned = append(pruned, dirPath)
			}
		}
	}

	return pruned, nil
}
