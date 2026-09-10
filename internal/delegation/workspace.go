package delegation

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveProjectRoot returns the repository root that owns start. Directories
// outside Git repositories are treated as standalone project roots.
func ResolveProjectRoot(start string) (string, error) {
	start = strings.TrimSpace(start)
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	absolute, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("project root must be a directory")
	}
	cmd := exec.Command("git", "-C", absolute, "rev-parse", "--show-toplevel")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if output, gitErr := cmd.Output(); gitErr == nil {
		absolute = strings.TrimSpace(string(output))
	}
	return CanonicalWorkspace(absolute)
}

// CanonicalWorkspace normalizes a durable workspace key for comparisons.
func CanonicalWorkspace(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	absolute = platformCanonicalPath(absolute)
	absolute = filepath.Clean(absolute)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		absolute = strings.ToLower(absolute)
	}
	return filepath.ToSlash(absolute), nil
}

// SameWorkspace reports whether two workspace paths resolve to the same canonical path.
func SameWorkspace(left, right string) bool {
	leftKey, leftErr := CanonicalWorkspace(left)
	rightKey, rightErr := CanonicalWorkspace(right)
	return leftErr == nil && rightErr == nil && leftKey != "" && leftKey == rightKey
}

// WorkspacesCompatible reports whether two workspace paths are identical or have
// an enclosing parent/child relationship (e.g. an umbrella project directory containing nested Git repositories).
func WorkspacesCompatible(left, right string) bool {
	leftKey, leftErr := CanonicalWorkspace(left)
	rightKey, rightErr := CanonicalWorkspace(right)
	if leftErr != nil || rightErr != nil || leftKey == "" || rightKey == "" {
		return false
	}
	if leftKey == rightKey {
		return true
	}
	if strings.HasPrefix(rightKey, leftKey+"/") {
		return true
	}
	if strings.HasPrefix(leftKey, rightKey+"/") {
		return true
	}
	return false
}

// WorkspaceRelativePrefix returns the relative slash-separated path from parent to child
// if child is identical to or strictly inside parent.
func WorkspaceRelativePrefix(parent, child string) (string, bool) {
	pKey, pErr := CanonicalWorkspace(parent)
	cKey, cErr := CanonicalWorkspace(child)
	if pErr != nil || cErr != nil || pKey == "" || cKey == "" {
		return "", false
	}
	if pKey == cKey {
		return "", true
	}
	prefix := pKey + "/"
	if strings.HasPrefix(cKey, prefix) {
		return strings.TrimPrefix(cKey, prefix), true
	}
	return "", false
}

func sameWorkspace(left, right string) bool {
	return SameWorkspace(left, right)
}
