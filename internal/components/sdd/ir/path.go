package ir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Resolve resolves a portable workflow destination beneath homeDir. External
// configuration is intentionally rejected; callers handling protected config
// must use ResolveExternal and an explicit ownership policy.
func (p AssetPath) Resolve(homeDir string) (string, error) {
	if p.Scope == ScopeExternalConfig {
		return "", errors.New("external-config asset paths are not portable workflow destinations")
	}
	if p.Scope != ScopeWorkflowRoot {
		return "", fmt.Errorf("unsupported asset path scope %q", p.Scope)
	}
	return resolveBeneath(homeDir, p.Relative)
}

// ResolveExternal resolves an explicitly declared external root. It never
// rebases the root under homeDir and rejects roots that are not absolute.
func (p AssetPath) ResolveExternal() (string, error) {
	if p.Scope != ScopeExternalConfig {
		return "", fmt.Errorf("asset path scope %q is not external-config", p.Scope)
	}
	if !filepath.IsAbs(p.Absolute) {
		return "", fmt.Errorf("external asset root must be absolute: %q", p.Absolute)
	}
	return resolveBeneath(p.Absolute, p.Relative)
}

func resolveBeneath(root, relative string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("asset path root is required")
	}
	if relative == "" || strings.ContainsAny(relative, `\:`) || strings.HasPrefix(relative, "/") || strings.HasPrefix(relative, "//") {
		return "", fmt.Errorf("asset path %q is not a portable relative path", relative)
	}
	relative = filepath.ToSlash(relative)
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(clean)) || strings.HasPrefix(clean, "//") {
		return "", fmt.Errorf("asset path %q escapes its root", relative)
	}
	parts := strings.Split(clean, "/")
	if len(parts) > 1 && parts[0] == parts[1] {
		return "", fmt.Errorf("asset path %q contains a double root", relative)
	}
	if strings.HasPrefix(clean, "internal/") || strings.HasPrefix(clean, "src/") || strings.HasPrefix(clean, "testdata/") {
		return "", fmt.Errorf("asset path %q is source-shaped", relative)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve asset root: %w", err)
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(clean))
	if !contained(rootAbs, target) {
		return "", fmt.Errorf("asset path %q escapes its root", relative)
	}
	if err := rejectSymlinkComponents(rootAbs, target); err != nil {
		return "", err
	}
	return target, nil
}

func contained(root, target string) bool {
	r := filepath.Clean(root)
	t := filepath.Clean(target)
	if filepath.VolumeName(r) != "" {
		r, t = strings.ToLower(r), strings.ToLower(t)
	}
	rel, err := filepath.Rel(r, t)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func rejectSymlinkComponents(root, target string) error {
	root = filepath.Clean(root)
	current := root
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			break
		}
		if statErr != nil {
			return fmt.Errorf("inspect asset path %q: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset path %q traverses a symlink/reparse point", current)
		}
	}
	return nil
}
