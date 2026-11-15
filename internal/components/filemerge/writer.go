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

// rejectSymlinkParent walks each existing component of dir and rejects any
// symlink or reparse point so a managed write can never follow a substituted
// path.
func rejectSymlinkParent(dir string) error {
	cleaned := filepath.Clean(dir)
	if cleaned == "." || cleaned == string(filepath.Separator) || filepath.VolumeName(cleaned) != "" && cleaned == filepath.VolumeName(cleaned)+string(filepath.Separator) {
		return nil
	}
	current := ""
	for _, part := range splitPath(cleaned) {
		current = filepath.Join(current, part)
		if current == "" {
			continue
		}
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return fmt.Errorf("inspect %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q traverses a symlink/reparse point", current)
		}
	}
	return nil
}

func splitPath(cleaned string) []string {
	vol := filepath.VolumeName(cleaned)
	rest := cleaned[len(vol):]
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	if rest == "" {
		return nil
	}
	parts := strings.Split(rest, string(filepath.Separator))
	if vol != "" {
		return append([]string{vol + string(filepath.Separator)}, parts...)
	}
	return parts
}
