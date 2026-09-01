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

func sameWorkspace(left, right string) bool {
	leftKey, leftErr := CanonicalWorkspace(left)
	rightKey, rightErr := CanonicalWorkspace(right)
	return leftErr == nil && rightErr == nil && leftKey != "" && leftKey == rightKey
}
