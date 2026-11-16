package filemerge

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WriteResult describes the outcome of a file write operation.
type WriteResult struct {
	Changed bool
	Created bool
}

// WriteFileAtomic writes content to path using a temp file + rename pattern.
// If the file already exists with identical content, no write occurs.
// It rejects parent paths that traverse symlinks or reparse points and never
// chmods an already-existing parent directory.
func WriteFileAtomic(path string, content []byte, perm fs.FileMode) (WriteResult, error) {
	if perm == 0 {
		perm = 0o644
	}

	created := false
	existing, err := os.ReadFile(path)
	if err == nil {
		if bytes.Equal(existing, content) {
			return WriteResult{}, nil
		}
	} else if !os.IsNotExist(err) {
		return WriteResult{}, fmt.Errorf("read existing file %q: %w", path, err)
	} else {
		created = true
	}

	dir := filepath.Dir(path)
	if err := rejectSymlinkParent(dir); err != nil {
		return WriteResult{}, fmt.Errorf("unsafe parent for %q: %w", path, err)
	}
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return WriteResult{}, fmt.Errorf("create parent directories for %q: %w", path, err)
		}
	}

	tmp, err := os.CreateTemp(dir, ".cortex-ia-*.tmp")
	if err != nil {
		return WriteResult{}, fmt.Errorf("create temp file for %q: %w", path, err)
	}

	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return WriteResult{}, fmt.Errorf("write temp file for %q: %w", path, err)
	}

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return WriteResult{}, fmt.Errorf("set permissions on temp file for %q: %w", path, err)
	}

	if err := tmp.Close(); err != nil {
		return WriteResult{}, fmt.Errorf("close temp file for %q: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return WriteResult{}, fmt.Errorf("replace %q atomically: %w", path, err)
	}

	cleanup = false
	return WriteResult{Changed: true, Created: created}, nil
}

// rejectSymlinkParent walks every existing ancestor of dir from the volume
// root and rejects a symlink or reparse point unless symlinkTrusted accepts
// it. The walk must start at the root: relative components would be resolved
// against the process cwd and check the wrong entries (or none at all).
func rejectSymlinkParent(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", dir, err)
	}
	cleaned := filepath.Clean(abs)
	current := filepath.VolumeName(cleaned) + string(filepath.Separator)
	for _, part := range splitPath(cleaned) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 && !symlinkTrusted(current) {
			return fmt.Errorf("%q traverses a symlink/reparse point", current)
		}
	}
	return nil
}

// symlinkTrusted accepts only symlinks that belong to the OS or to the user's
// own layout, so a link planted inside a managed directory (for example
// ~/.config/opencode/agents -> /elsewhere) fails closed. Accepted:
//   - a direct child of the volume root (macOS /var, /tmp, /etc aliases,
//     which only root can create);
//   - a link at or above the home directory or the temp directory, because
//     macOS keeps $TMPDIR under /var/folders and homes may sit behind aliases;
//   - a dot-entry directly under home (~/.config, ~/.claude) that a dotfile
//     manager points elsewhere.
//
// Every other symlink, including one below home that is not a dot-entry, is
// refused.
func symlinkTrusted(link string) bool {
	parent := filepath.Dir(link)
	if parent == filepath.VolumeName(link)+string(filepath.Separator) {
		return true
	}
	if containsPath(link, os.TempDir()) {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	home = filepath.Clean(home)
	if containsPath(link, home) {
		return true
	}
	return parent == home && strings.HasPrefix(filepath.Base(link), ".")
}

// containsPath reports whether ancestor is target itself or one of its parents.
func containsPath(ancestor, target string) bool {
	ancestor = filepath.Clean(ancestor)
	target = filepath.Clean(target)
	return target == ancestor || strings.HasPrefix(target, ancestor+string(filepath.Separator))
}

// splitPath returns the components of an absolute, cleaned path below its
// volume root: "/a/b/c" yields ["a", "b", "c"].
func splitPath(cleaned string) []string {
	rest := strings.TrimPrefix(cleaned[len(filepath.VolumeName(cleaned)):], string(filepath.Separator))
	if rest == "" {
		return nil
	}
	return strings.Split(rest, string(filepath.Separator))
}
