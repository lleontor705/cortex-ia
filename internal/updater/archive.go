package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	MaxArchiveMembers      = 1024
	MaxExtractedBinarySize = 128 * 1024 * 1024 // 128 MiB
)

var (
	ErrTooManyArchiveMembers = errors.New("archive exceeds member limit (1024 members)")
	ErrBinaryTooLarge        = errors.New("extracted binary exceeds maximum size (128 MiB)")
	ErrUnsafeArchivePath     = errors.New("unsafe archive path detected")
	ErrCaseCollision         = errors.New("case-colliding archive member detected")
	ErrDuplicateMember       = errors.New("duplicate archive member detected")
	ErrDisallowedEntryType   = errors.New("disallowed archive entry type: only regular files and directories permitted")
	ErrMultipleBinaries      = errors.New("multiple executable binaries found in archive")
	ErrBinaryNotFound        = errors.New("target executable binary not found in archive")
	ErrUnsupportedArchive    = errors.New("unsupported archive format")
)

func validateArchivePath(rawPath string, seen map[string]bool, lowerSeen map[string]bool) (string, error) {
	if strings.IndexByte(rawPath, 0) != -1 {
		return "", fmt.Errorf("%w: path contains NUL byte", ErrUnsafeArchivePath)
	}
	if strings.Contains(rawPath, "\\") {
		return "", fmt.Errorf("%w: backslash not permitted in archive path %q", ErrUnsafeArchivePath, rawPath)
	}
	if len(rawPath) >= 2 && rawPath[1] == ':' && ((rawPath[0] >= 'a' && rawPath[0] <= 'z') || (rawPath[0] >= 'A' && rawPath[0] <= 'Z')) {
		return "", fmt.Errorf("%w: drive letter not permitted in %q", ErrUnsafeArchivePath, rawPath)
	}
	if strings.HasPrefix(rawPath, "/") {
		return "", fmt.Errorf("%w: absolute path not permitted in %q", ErrUnsafeArchivePath, rawPath)
	}

	clean := path.Clean(rawPath)
	if clean == "." || clean == "/" || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("%w: path traversal or root %q", ErrUnsafeArchivePath, rawPath)
	}
	parts := strings.Split(clean, "/")
	for _, p := range parts {
		if p == ".." {
			return "", fmt.Errorf("%w: directory traversal in %q", ErrUnsafeArchivePath, rawPath)
		}
	}

	if seen[clean] {
		return "", fmt.Errorf("%w: %q", ErrDuplicateMember, clean)
	}
	seen[clean] = true

	lower := strings.ToLower(clean)
	if lowerSeen[lower] {
		return "", fmt.Errorf("%w: %q collides with existing member", ErrCaseCollision, clean)
	}
	lowerSeen[lower] = true

	return clean, nil
}

func ValidateAndExtractZip(data []byte, targetName string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip archive: %w", err)
	}

	if len(r.File) > MaxArchiveMembers {
		return nil, fmt.Errorf("%w: count %d", ErrTooManyArchiveMembers, len(r.File))
	}

	seen := make(map[string]bool)
	lowerSeen := make(map[string]bool)
	var extracted []byte
	foundTarget := false
	var validationErr error

	for _, file := range r.File {
		mode := file.Mode()
		if mode&0o170000 != 0 && !mode.IsDir() && !mode.IsRegular() {
			if validationErr == nil {
				validationErr = fmt.Errorf("%w: %s (mode %v)", ErrDisallowedEntryType, file.Name, mode)
			}
		}

		clean, err := validateArchivePath(file.Name, seen, lowerSeen)
		if err != nil {
			if validationErr == nil {
				validationErr = err
			}
			continue
		}

		if file.FileInfo().IsDir() {
			continue
		}

		baseName := path.Base(clean)
		if strings.EqualFold(baseName, targetName) {
			if clean != targetName {
				if validationErr == nil {
					validationErr = fmt.Errorf("%w: nested binary %q", ErrMultipleBinaries, clean)
				}
				continue
			}
			if foundTarget {
				if validationErr == nil {
					validationErr = fmt.Errorf("%w: duplicate binary entry %q", ErrMultipleBinaries, clean)
				}
				continue
			}
			foundTarget = true

			rc, err := file.Open()
			if err != nil {
				if validationErr == nil {
					validationErr = err
				}
				continue
			}
			content, err := io.ReadAll(io.LimitReader(rc, MaxExtractedBinarySize+1))
			_ = rc.Close()
			if err != nil {
				if validationErr == nil {
					validationErr = err
				}
				continue
			}
			if int64(len(content)) > MaxExtractedBinarySize {
				if validationErr == nil {
					validationErr = ErrBinaryTooLarge
				}
				continue
			}
			extracted = content
		}
	}

	if validationErr != nil {
		return nil, validationErr
	}
	if !foundTarget || extracted == nil {
		return nil, fmt.Errorf("%w: %q", ErrBinaryNotFound, targetName)
	}
	return extracted, nil
}

func ValidateAndExtractTarGz(data []byte, targetName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	seen := make(map[string]bool)
	lowerSeen := make(map[string]bool)
	var extracted []byte
	foundTarget := false
	memberCount := 0
	var validationErr error

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		memberCount++
		if memberCount > MaxArchiveMembers {
			return nil, fmt.Errorf("%w: count exceeds %d", ErrTooManyArchiveMembers, MaxArchiveMembers)
		}

		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
			if validationErr == nil {
				validationErr = fmt.Errorf("%w: %s (typeflag %d)", ErrDisallowedEntryType, hdr.Name, hdr.Typeflag)
			}
		}

		clean, err := validateArchivePath(hdr.Name, seen, lowerSeen)
		if err != nil {
			if validationErr == nil {
				validationErr = err
			}
			continue
		}

		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		baseName := path.Base(clean)
		if strings.EqualFold(baseName, targetName) {
			if clean != targetName {
				if validationErr == nil {
					validationErr = fmt.Errorf("%w: nested binary %q", ErrMultipleBinaries, clean)
				}
				continue
			}
			if foundTarget {
				if validationErr == nil {
					validationErr = fmt.Errorf("%w: duplicate binary entry %q", ErrMultipleBinaries, clean)
				}
				continue
			}
			foundTarget = true

			content, err := io.ReadAll(io.LimitReader(tr, MaxExtractedBinarySize+1))
			if err != nil {
				if validationErr == nil {
					validationErr = err
				}
				continue
			}
			if int64(len(content)) > MaxExtractedBinarySize {
				if validationErr == nil {
					validationErr = ErrBinaryTooLarge
				}
				continue
			}
			extracted = content
		}
	}

	if validationErr != nil {
		return nil, validationErr
	}
	if !foundTarget || extracted == nil {
		return nil, fmt.Errorf("%w: %q", ErrBinaryNotFound, targetName)
	}
	return extracted, nil
}

func ValidateAndExtractBinary(data []byte, filename string, targetOS string) ([]byte, error) {
	targetName := "cortex-ia"
	if targetOS == "windows" {
		targetName = "cortex-ia.exe"
	}

	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".zip") {
		return ValidateAndExtractZip(data, targetName)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return ValidateAndExtractTarGz(data, targetName)
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedArchive, filename)
}
