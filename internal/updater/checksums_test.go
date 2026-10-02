package updater

import (
	"errors"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	validHash1 := "4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b"
	validHash2 := "1f2e3d4c5b6a708192a3b4c5d6e7f8091a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d"
	validHash3 := "9c8b7a6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b"

	t.Run("valid_standard_and_binary_formats", func(t *testing.T) {
		input := strings.Join([]string{
			"# GoReleaser checksums.txt",
			"",
			validHash1 + "  cortex-ia_0.5.0_linux_amd64.tar.gz",
			validHash2 + " *cortex-ia_0.5.0_darwin_arm64.tar.gz",
			"   # inline spaced comment",
			validHash3 + "  cortex-ia_0.5.0_windows_amd64.zip\r",
			"",
		}, "\n")

		table, err := ParseChecksums([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(table) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(table))
		}

		h1, err := table.FindChecksum("cortex-ia_0.5.0_linux_amd64.tar.gz")
		if err != nil || h1 != validHash1 {
			t.Errorf("find h1 failed: %v, hash: %s", err, h1)
		}
		h2, err := FindChecksum(table, "cortex-ia_0.5.0_darwin_arm64.tar.gz")
		if err != nil || h2 != validHash2 {
			t.Errorf("find h2 failed: %v, hash: %s", err, h2)
		}
		h3, err := table.FindChecksum("cortex-ia_0.5.0_windows_amd64.zip")
		if err != nil || h3 != validHash3 {
			t.Errorf("find h3 failed: %v, hash: %s", err, h3)
		}
	})

	t.Run("size_exceeded", func(t *testing.T) {
		huge := make([]byte, MaxChecksumsSize+1)
		_, err := ParseChecksums(huge)
		if !errors.Is(err, ErrChecksumsTooLarge) {
			t.Fatalf("expected ErrChecksumsTooLarge, got %v", err)
		}
	})

	t.Run("find_checksum_missing_and_nil", func(t *testing.T) {
		var nilTable ChecksumTable
		if _, err := nilTable.FindChecksum("cortex-ia.tar.gz"); !errors.Is(err, ErrAssetNotFoundInChecksums) {
			t.Errorf("expected ErrAssetNotFoundInChecksums for nil table, got %v", err)
		}
		emptyTable := make(ChecksumTable)
		if _, err := FindChecksum(emptyTable, "cortex-ia.tar.gz"); !errors.Is(err, ErrAssetNotFoundInChecksums) {
			t.Errorf("expected ErrAssetNotFoundInChecksums for empty table, got %v", err)
		}
	})

	t.Run("rejections", func(t *testing.T) {
		tests := []struct {
			name        string
			input       string
			expectedErr error
		}{
			{
				name:        "non_hex_characters",
				input:       "4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3g  cortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumHash,
			},
			{
				name:        "uppercase_hex_characters",
				input:       "4A3B2C1D0E9F8A7B6C5D4E3F2A1B0C9D8E7F6A5B4C3D2E1F0A9B8C7D6E5F4A3B  cortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumHash,
			},
			{
				name:        "duplicate_entries",
				input:       validHash1 + "  cortex-ia.tar.gz\n" + validHash2 + "  cortex-ia.tar.gz",
				expectedErr: ErrDuplicateChecksumArtifact,
			},
			{
				name:        "single_space_separator",
				input:       validHash1 + " cortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumLine,
			},
			{
				name:        "three_spaces_separator",
				input:       validHash1 + "   cortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumLine,
			},
			{
				name:        "tab_separator",
				input:       validHash1 + "\tcortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumLine,
			},
			{
				name:        "leading_space",
				input:       " " + validHash1 + "  cortex-ia.tar.gz",
				expectedErr: ErrMalformedChecksumLine,
			},
			{
				name:        "line_too_short",
				input:       validHash1,
				expectedErr: ErrMalformedChecksumLine,
			},
			{
				name:        "traversal_parent_dir",
				input:       validHash1 + "  ../cortex-ia.tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "traversal_sub_dir_slash",
				input:       validHash1 + "  builds/cortex-ia.tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "traversal_sub_dir_backslash",
				input:       validHash1 + "  builds\\cortex-ia.zip",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "traversal_current_dir",
				input:       validHash1 + "  ./cortex-ia.tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "invalid_extension_exe",
				input:       validHash1 + "  cortex-ia.exe",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "extension_only_tar_gz",
				input:       validHash1 + "  .tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "extension_only_zip",
				input:       validHash1 + "  .zip",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "double_asterisk",
				input:       validHash1 + " **cortex-ia.tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
			{
				name:        "filename_with_spaces",
				input:       validHash1 + "  cortex ia.tar.gz",
				expectedErr: ErrInvalidChecksumFilename,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ParseChecksums([]byte(tc.input))
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.expectedErr)
				}
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
				}
			})
		}
	})

	t.Run("empty_and_comments_only", func(t *testing.T) {
		table, err := ParseChecksums([]byte("# only comments\n\n# another\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(table) != 0 {
			t.Fatalf("expected empty table, got %d", len(table))
		}
		tableEmpty, err := ParseChecksums([]byte(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tableEmpty) != 0 {
			t.Fatalf("expected empty table, got %d", len(tableEmpty))
		}
	})
}
