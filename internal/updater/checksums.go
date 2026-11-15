package updater

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// MaxChecksumsSize caps memory allocation for the checksums file.
	MaxChecksumsSize = 1024 * 1024 // 1 MiB
	// ChecksumsFileName is the standard GoReleaser asset name.
	ChecksumsFileName = "checksums.txt"
)

var (
	ErrChecksumsTooLarge         = errors.New("checksums file exceeds maximum allowed size (1 MiB)")
	ErrMalformedChecksumLine     = errors.New("malformed checksum line")
	ErrMalformedChecksumHash     = errors.New("malformed SHA-256 hash in checksums file: must be 64 lowercase hex characters")
	ErrInvalidChecksumFilename   = errors.New("invalid filename in checksums file: must be clean relative name without directory paths")
	ErrDuplicateChecksumArtifact = errors.New("duplicate artifact entry in checksums file")
	ErrAssetNotFoundInChecksums  = errors.New("target release asset not found in checksums file")
	ErrChecksumsAssetNotFound    = errors.New("release missing required checksums.txt asset")
)

// ChecksumTable maps release artifact filenames to their expected SHA-256 hashes.
type ChecksumTable map[string]string

// FindChecksum looks up the expected SHA-256 hash for the given filename.
// Returns ErrAssetNotFoundInChecksums if the artifact is not found.
func (t ChecksumTable) FindChecksum(filename string) (string, error) {
	if t == nil {
		return "", ErrAssetNotFoundInChecksums
	}
	hash, ok := t[filename]
	if !ok {
		return "", ErrAssetNotFoundInChecksums
	}
	return hash, nil
}

// FindChecksum looks up the expected SHA-256 hash for filename in the table.
// Returns ErrAssetNotFoundInChecksums if the artifact is not found.
func FindChecksum(table ChecksumTable, filename string) (string, error) {
	return table.FindChecksum(filename)
}

// ParseChecksums parses the raw bytes of a checksums.txt file into a ChecksumTable.
func ParseChecksums(data []byte) (ChecksumTable, error) {
	if len(data) > MaxChecksumsSize {
		return nil, ErrChecksumsTooLarge
	}

	table := make(ChecksumTable)
	lines := strings.Split(string(data), "\n")

	for _, rawLine := range lines {
		trimmedLeading := strings.TrimLeft(rawLine, " \t")
		if strings.HasPrefix(trimmedLeading, "#") {
			continue
		}
		line := strings.TrimRight(rawLine, "\r \t")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return nil, ErrMalformedChecksumLine
		}
		if len(line) < 67 {
			return nil, ErrMalformedChecksumLine
		}

		delim := line[64:66]
		if delim != "  " && delim != " *" {
			return nil, ErrMalformedChecksumLine
		}

		filename := line[66:]
		if strings.HasPrefix(filename, " ") || strings.HasPrefix(filename, "\t") {
			return nil, ErrMalformedChecksumLine
		}
		if strings.ContainsAny(filename, " \t") || strings.HasPrefix(filename, "*") {
			return nil, ErrInvalidChecksumFilename
		}

		hash := line[:64]
		if !isValidHexHash(hash) {
			return nil, ErrMalformedChecksumHash
		}

		if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
			return nil, ErrInvalidChecksumFilename
		}
		if filename == "." || filename == ".." || strings.Contains(filename, "..") {
			return nil, ErrInvalidChecksumFilename
		}
		if !strings.HasSuffix(filename, ".tar.gz") && !strings.HasSuffix(filename, ".zip") {
			return nil, ErrInvalidChecksumFilename
		}
		if filename == ".tar.gz" || filename == ".zip" {
			return nil, ErrInvalidChecksumFilename
		}

		if _, exists := table[filename]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateChecksumArtifact, filename)
		}
		table[filename] = hash
	}

	return table, nil
}

func isValidHexHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
