package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// backupRoot is the default backup root resolver. It returns ~/.cortex-ia/backups.
func backupRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cortex-ia", "backups"), nil
}

// BackupRootFn is the function used to resolve the backup root directory.
// Package-level var for testability — swapped in tests to use a temp directory.
// Exported so tests in other packages can override it as well.
var BackupRootFn = backupRoot

// BackupSource identifies what operation created a backup.
type BackupSource string

const (
	BackupSourceInstall   BackupSource = "install"
	BackupSourceSync      BackupSource = "sync"
	BackupSourceUpgrade   BackupSource = "upgrade"
	BackupSourceUninstall BackupSource = "uninstall"
)

func (s BackupSource) Label() string {
	switch s {
	case BackupSourceInstall:
		return "install"
	case BackupSourceSync:
		return "sync"
	case BackupSourceUpgrade:
		return "upgrade"
	case BackupSourceUninstall:
		return "uninstall"
	default:
		return "unknown source"
	}
}

const ManifestFilename = "manifest.json"

// Manifest describes a backup snapshot.
type Manifest struct {
	ID               string          `json:"id"`
	CreatedAt        time.Time       `json:"created_at"`
	RootDir          string          `json:"root_dir"`
	Entries          []ManifestEntry `json:"entries"`
	Source           BackupSource    `json:"source,omitempty"`
	Description      string          `json:"description,omitempty"`
	FileCount        int             `json:"file_count,omitempty"`
	CreatedByVersion string          `json:"created_by_version,omitempty"`

	// Pinned marks a backup as protected from automatic pruning.
	// Manifests written before pinning was supported lack this field; absent ⇒ false.
	Pinned bool `json:"pinned,omitempty"`

	// ArchiveFile optionally points to the compressed archive inside RootDir.
	ArchiveFile string `json:"archive_file,omitempty"`

	// ArchiveSize is the compressed archive size in bytes.
	ArchiveSize int64 `json:"archive_size,omitempty"`

	// Checksum is the SHA-256 digest of the snapshot inputs (per ComputeChecksum).
	// Absent on legacy manifests; an empty string disables dedup for that backup.
	Checksum string `json:"checksum,omitempty"`
}

func (m Manifest) DisplayLabel() string {
	base := fmt.Sprintf("%s — %s", m.Source.Label(), m.CreatedAt.Local().Format("2006-01-02 15:04"))
	if m.FileCount > 0 {
		if m.ArchiveSize > 0 {
			return fmt.Sprintf("%s (%d files, %s)", base, m.FileCount, FormatBytes(m.ArchiveSize))
		}
		return fmt.Sprintf("%s (%d files)", base, m.FileCount)
	}
	return base
}

// FormatBytes returns human-readable byte sizes.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ManifestEntry describes a single backed-up file.
type ManifestEntry struct {
	OriginalPath string `json:"original_path"`
	SnapshotPath string `json:"snapshot_path"`
	Existed      bool   `json:"existed"`
	Mode         uint32 `json:"mode,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
}

// Validate proves a manifest is accredited before anyone may act on it:
// RootDir must be an existing real directory reachable through a no-follow
// Lstat chain (no symlink/reparse component below the first real anchor),
// every OriginalPath must be a clean absolute path without alias vectors,
// every recorded snapshot must be contained in RootDir, accredited with the
// same no-follow Lstat chain, and carry a valid SHA-256; no exact or
// case-folded duplicate originals are allowed. It performs no filesystem
// mutation.
func (m Manifest) Validate() error {
	if err := validateAbsolutePath(m.RootDir, "manifest root_dir"); err != nil {
		return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if err := rejectLinkChain(m.RootDir, "manifest root_dir"); err != nil {
		return err
	}
	info, err := os.Lstat(m.RootDir)
	if err != nil {
		return fmt.Errorf("%w: inspect root_dir %q: %v", ErrManifestInvalid, m.RootDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: root_dir %q is not a real directory", ErrManifestInvalid, m.RootDir)
	}
	seen := make(map[string]bool, len(m.Entries))
	seenFold := make(map[string]bool, len(m.Entries))
	for i := range m.Entries {
		entry := m.Entries[i]
		if err := validateAbsolutePath(entry.OriginalPath, fmt.Sprintf("entry %d original_path", i)); err != nil {
			return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
		}
		if seen[entry.OriginalPath] {
			return fmt.Errorf("%w: duplicate original %q", ErrManifestInvalid, entry.OriginalPath)
		}
		if seenFold[foldKey(entry.OriginalPath)] {
			return fmt.Errorf("%w: case-alias original %q", ErrManifestInvalid, entry.OriginalPath)
		}
		seen[entry.OriginalPath] = true
		seenFold[foldKey(entry.OriginalPath)] = true
		if entry.Existed {
			if err := validateAbsolutePath(entry.SnapshotPath, fmt.Sprintf("entry %d snapshot_path", i)); err != nil {
				return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
			}
			if err := containedPath(m.RootDir, entry.SnapshotPath); err != nil {
				return fmt.Errorf("%w: snapshot for %q: %v", ErrManifestInvalid, entry.OriginalPath, err)
			}
			if err := rejectLinkChain(entry.SnapshotPath, fmt.Sprintf("snapshot for %q", entry.OriginalPath)); err != nil {
				return err
			}
			if !isHexSHA256(entry.SHA256) {
				return fmt.Errorf("%w: entry %q lacks a valid SHA-256", ErrManifestInvalid, entry.OriginalPath)
			}
			continue
		}
		if entry.SnapshotPath != "" || entry.SHA256 != "" {
			return fmt.Errorf("%w: missing source %q records snapshot evidence", ErrManifestInvalid, entry.OriginalPath)
		}
	}
	return nil
}

// ValidateForRestore validates a manifest against an expected backup root and
// optional expected ID before any write may occur. It keeps the base
// accreditation checks and adds explicit linkage to the expected checkpoint.
func (m Manifest) ValidateForRestore(expectedRoot, expectedBackupID string) error {
	if expectedRoot == "" {
		return fmt.Errorf("%w: expected backup root is empty", ErrManifestInvalid)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if err := validateAbsolutePath(expectedRoot, "expected backup root"); err != nil {
		return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if err := rejectLinkChain(expectedRoot, "expected backup root"); err != nil {
		return err
	}
	if !equalManifestPath(expectedRoot, m.RootDir) {
		return fmt.Errorf("%w: manifest root_dir %q does not match expected %q", ErrManifestInvalid, m.RootDir, expectedRoot)
	}
	if expectedBackupID == "" {
		return nil
	}
	if m.ID != expectedBackupID {
		return fmt.Errorf("%w: manifest id %q does not match expected backup %q", ErrManifestInvalid, m.ID, expectedBackupID)
	}
	return nil
}

func equalManifestPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if caseInsensitivePaths {
		return foldKey(left) == foldKey(right)
	}
	return left == right
}

func WriteManifest(path string, manifest Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory %q: %w", path, err)
	}

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write manifest %q: %w", path, err)
	}
	return nil
}

func ReadManifest(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal manifest %q: %w", path, err)
	}
	return manifest, nil
}

func DeleteBackup(manifest Manifest) error {
	if manifest.RootDir == "" {
		return fmt.Errorf("backup has no root directory")
	}
	root := filepath.Clean(manifest.RootDir)
	if !filepath.IsAbs(root) {
		return fmt.Errorf("backup root %q must be absolute", root)
	}
	if parent := filepath.Dir(root); parent == root {
		return fmt.Errorf("backup root %q is a filesystem root", root)
	}
	return os.RemoveAll(root)
}

func RenameBackup(manifest Manifest, newDescription string) error {
	if manifest.RootDir == "" {
		return fmt.Errorf("backup has no root directory")
	}
	manifest.Description = newDescription
	manifestPath := filepath.Join(manifest.RootDir, ManifestFilename)
	return WriteManifest(manifestPath, manifest)
}

// ListResult holds manifests and any warnings from scanning.
type ListResult struct {
	Manifests []Manifest
	Warnings  []string
}

// ListManifests reads all backup manifests from the given backups directory.
// Each immediate subdirectory is expected to contain a manifest.json file.
// Directories without a valid manifest produce a warning in the result.
func ListManifests(backupsDir string) ListResult {
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return ListResult{}
	}

	var result ListResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		manifestPath := filepath.Join(backupsDir, name, ManifestFilename)
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("backup %s: missing manifest.json", name))
			continue
		}
		m, err := ReadManifest(manifestPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("backup %s: %v", name, err))
			continue
		}
		result.Manifests = append(result.Manifests, m)
	}
	return result
}
