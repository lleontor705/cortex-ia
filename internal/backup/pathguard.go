package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// ErrUnsupportedLink is returned when a declared backup path is, or
	// traverses, a symlink/reparse point. Backup capture and restore never
	// follow links.
	ErrUnsupportedLink = errors.New("unsupported symlink/reparse point in backup path")

	// ErrManifestInvalid is the typed fail-closed verdict for manifests whose
	// structure, accreditation, or containment cannot be proven.
	ErrManifestInvalid = errors.New("invalid backup manifest")
)

// caseInsensitivePaths mirrors the platform path-identity policy: Windows
// and macOS fold case for path equality; Unix does not.
var caseInsensitivePaths = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// foldKey derives the duplicate-detection key for a path.
func foldKey(path string) string {
	if caseInsensitivePaths {
		return strings.ToLower(path)
	}
	return path
}

// validateAbsolutePath fails closed unless path is absolute, clean,
// drive-letter rooted on Windows (no UNC/device namespaces), and free of
// NTFS alternate-data-stream colons, trailing dots/spaces, and reserved
// device names in any component. The rules hold on every platform so a
// manifest stays safe when carried across operating systems.
func validateAbsolutePath(path, field string) error {
	if path == "" {
		return fmt.Errorf("%s is empty", field)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s %q must be absolute", field, path)
	}
	if volume := filepath.VolumeName(path); volume != "" && !isDriveVolume(volume) {
		return fmt.Errorf("%s %q uses a non-drive (UNC/device) root", field, path)
	}
	if clean := filepath.Clean(path); clean != path {
		return fmt.Errorf("%s %q is not a clean path", field, path)
	}
	trimmed := strings.TrimPrefix(path, filepath.VolumeName(path))
	for _, component := range strings.Split(trimmed, string(filepath.Separator)) {
		if err := validatePathComponent(component, field, path); err != nil {
			return err
		}
	}
	return nil
}

func isDriveVolume(volume string) bool {
	return len(volume) == 2 && volume[1] == ':' &&
		((volume[0] >= 'A' && volume[0] <= 'Z') || (volume[0] >= 'a' && volume[0] <= 'z'))
}

// validatePathComponent rejects Windows alias vectors: NTFS alternate data
// streams (":"), trailing dots/spaces (stripped by Win32 naming), and
// reserved device names with or without an extension.
func validatePathComponent(component, field, path string) error {
	if component == "" {
		return nil // separators at volume boundaries
	}
	if strings.ContainsRune(component, ':') {
		return fmt.Errorf("%s %q contains an alternate-data-stream colon", field, path)
	}
	if component != strings.TrimRight(component, ". ") {
		return fmt.Errorf("%s %q contains a trailing dot or space", field, path)
	}
	if isReservedDeviceName(component) {
		return fmt.Errorf("%s %q contains a reserved device name", field, path)
	}
	return nil
}

func isReservedDeviceName(component string) bool {
	name := strings.ToUpper(component)
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	switch name {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 {
		if (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9' {
			return true
		}
	}
	return false
}

// containedPath proves candidate is strictly inside root. Both must already
// be validated absolute clean paths.
func containedPath(root, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return fmt.Errorf("%q cannot be related to %q", candidate, root)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q escapes %q", candidate, root)
	}
	return nil
}

// rejectLinkChain fails closed when any path component at or below the first
// real (non-link) filesystem entry is a symlink or other reparse-like
// indirection. Symlink components that prefix the whole chain before any
// real entry exists (for example a macOS /var volume alias) are tolerated:
// they cannot be distinguished from benign OS layout without a trusted
// anchor, and every component below the first real anchor is still
// accredited. The final component is always inspected with Lstat by the
// caller before this helper runs.
func rejectLinkChain(path, display string) error {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("inspect %q: path must be absolute", display)
	}
	volume := filepath.VolumeName(cleaned)
	current := volume + string(filepath.Separator)
	remainder := strings.TrimPrefix(strings.TrimPrefix(cleaned, volume), string(filepath.Separator))
	anchored := false
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil // nothing deeper can exist, let alone as a link
		}
		if err != nil {
			return fmt.Errorf("inspect %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			if anchored {
				return fmt.Errorf("%w: %q traverses link %q", ErrUnsupportedLink, display, current)
			}
			continue
		}
		anchored = true
	}
	return nil
}

// isHexSHA256 reports whether digest is a well-formed lowercase SHA-256 hex
// string.
func isHexSHA256(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
