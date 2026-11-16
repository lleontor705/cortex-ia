package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveEntry describes a single file to include in a tar.gz archive (for
// creation) or a file that was extracted from one (for extraction).
type ArchiveEntry struct {
	// RelPath is the relative path inside the archive (e.g. "files/.config/foo.json").
	RelPath string
	// SourcePath is the absolute path on disk (source when creating; destination when extracting).
	SourcePath string
	// Mode is the file permission bits.
	Mode os.FileMode
}

// CreateArchive writes a tar.gz archive at archivePath containing all entries.
// Each entry's content is read from SourcePath and stored under RelPath inside
// the archive. Intermediate directories in archivePath are created automatically.
//
// The archive uses filepath.ToSlash for cross-platform tar path compatibility.
func CreateArchive(archivePath string, entries []ArchiveEntry) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		return fmt.Errorf("create archive directory %q: %w", filepath.Dir(archivePath), err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive file %q: %w", archivePath, err)
	}
	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for _, entry := range entries {
		if err := addFileToTar(tw, entry); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}

	return nil
}

func addFileToTar(tw *tar.Writer, entry ArchiveEntry) error {
	data, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return fmt.Errorf("read source file %q: %w", entry.SourcePath, err)
	}

	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     filepath.ToSlash(entry.RelPath),
		Mode:     int64(entry.Mode),
		Size:     int64(len(data)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %q: %w", entry.RelPath, err)
	}

	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write tar content for %q: %w", entry.RelPath, err)
	}

	return nil
}
