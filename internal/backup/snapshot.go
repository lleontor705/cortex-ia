package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Snapshotter creates file snapshots for backup purposes.
type Snapshotter struct {
	now func() time.Time
}

func NewSnapshotter() Snapshotter {
	return Snapshotter{now: time.Now}
}

// Create snapshots the given file paths into snapshotDir and writes a manifest.
func (s Snapshotter) Create(snapshotDir string, paths []string) (Manifest, error) {
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create snapshot directory %q: %w", snapshotDir, err)
	}

	manifest := Manifest{
		ID:        filepath.Base(snapshotDir),
		CreatedAt: s.now().UTC(),
		RootDir:   snapshotDir,
		Entries:   make([]ManifestEntry, 0, len(paths)),
	}

	for _, path := range paths {
		entry, err := s.snapshotPath(snapshotDir, path)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Entries = append(manifest.Entries, entry)
		if entry.Existed {
			manifest.FileCount++
		}
	}

	if err := WriteManifest(filepath.Join(snapshotDir, ManifestFilename), manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func (s Snapshotter) snapshotPath(snapshotDir string, sourcePath string) (ManifestEntry, error) {
	cleanSource := filepath.Clean(sourcePath)
	entry := ManifestEntry{OriginalPath: cleanSource}

	info, err := os.Lstat(cleanSource)
	if err != nil {
		if os.IsNotExist(err) {
			return entry, nil
		}
		return ManifestEntry{}, fmt.Errorf("stat source path %q: %w", cleanSource, err)
	}

	// No-follow capture: links and reparse-like indirections are rejected
	// before any byte is read, both for the target and its parents.
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
		return ManifestEntry{}, fmt.Errorf("%w: %q", ErrUnsupportedLink, cleanSource)
	}
	if info.IsDir() {
		return entry, nil
	}
	if !info.Mode().IsRegular() {
		return ManifestEntry{}, fmt.Errorf("snapshot source %q is not a regular file", cleanSource)
	}
	if err := rejectLinkChain(cleanSource, cleanSource); err != nil {
		return ManifestEntry{}, err
	}

	// Strip volume name (e.g. "C:") on Windows for safe relative paths.
	relative := strings.TrimPrefix(cleanSource, filepath.VolumeName(cleanSource))
	relative = strings.TrimPrefix(relative, string(filepath.Separator))
	if relative == "" {
		relative = "root"
	}

	destination := filepath.Join(snapshotDir, "files", relative)
	if err := copyFile(cleanSource, destination, info.Mode()); err != nil {
		return ManifestEntry{}, err
	}

	entry.SnapshotPath = destination
	entry.Existed = true
	entry.Mode = uint32(info.Mode())
	digest, err := fileSHA256(destination)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry.SHA256 = digest
	return entry, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for verification %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash file %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFile(source string, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", source, err)
	}
	defer func() { _ = input.Close() }()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create backup directory for %q: %w", destination, err)
	}

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("create snapshot file %q: %w", destination, err)
	}

	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy %q to %q: %w", source, destination, err)
	}

	if err := output.Close(); err != nil {
		return fmt.Errorf("close snapshot file %q: %w", destination, err)
	}
	return nil
}
