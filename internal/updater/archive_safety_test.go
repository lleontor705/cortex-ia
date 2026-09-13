package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"testing"
)

func createZipWithEntries(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%s) failed: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("zip.Write failed: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip.Close failed: %v", err)
	}
	return buf.Bytes()
}

func createTarWithEntries(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar.WriteHeader failed: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("tar.Write failed: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close failed: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip.Close failed: %v", err)
	}
	return buf.Bytes()
}

func TestUpdateArchive(t *testing.T) {
	binaryContent := []byte("VALID_BIN")

	t.Run("MemberCount1024And1025", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("cortex-ia.exe")
		_, _ = w.Write(binaryContent)
		for i := 1; i <= 1023; i++ {
			wi, _ := zw.Create(fmt.Sprintf("doc_%04d.txt", i))
			_, _ = wi.Write([]byte("ok"))
		}
		_ = zw.Close()
		if _, err := ValidateAndExtractZip(buf.Bytes(), "cortex-ia.exe"); err != nil {
			t.Fatalf("1024 members must pass, got err: %v", err)
		}

		buf.Reset()
		zw = zip.NewWriter(&buf)
		w, _ = zw.Create("cortex-ia.exe")
		_, _ = w.Write(binaryContent)
		for i := 1; i <= 1024; i++ {
			wi, _ := zw.Create(fmt.Sprintf("doc_%04d.txt", i))
			_, _ = wi.Write([]byte("ok"))
		}
		_ = zw.Close()
		if _, err := ValidateAndExtractZip(buf.Bytes(), "cortex-ia.exe"); !errors.Is(err, ErrTooManyArchiveMembers) {
			t.Fatalf("1025 members must fail with ErrTooManyArchiveMembers, got: %v", err)
		}
	})

	t.Run("UnsafePaths", func(t *testing.T) {
		cases := []struct {
			name string
			path string
		}{
			{"Absolute", "/cortex-ia.exe"},
			{"DriveLetter", "C:cortex-ia.exe"},
			{"Backslash", "sub\\cortex-ia.exe"},
			{"NULByte", "cortex-ia" + string([]byte{0}) + ".exe"},
			{"Traversal", "../cortex-ia.exe"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				z := createZipWithEntries(t, map[string][]byte{
					"cortex-ia.exe": binaryContent,
					tc.path:         []byte("bad"),
				})
				if _, err := ValidateAndExtractZip(z, "cortex-ia.exe"); !errors.Is(err, ErrUnsafeArchivePath) {
					t.Fatalf("expected ErrUnsafeArchivePath for %s (%s), got: %v", tc.name, tc.path, err)
				}
			})
		}
	})

	t.Run("CaseCollidingAndDuplicateMembers", func(t *testing.T) {
		z := createZipWithEntries(t, map[string][]byte{
			"cortex-ia.exe": binaryContent,
			"README.md":     []byte("one"),
			"readme.md":     []byte("two"),
		})
		if _, err := ValidateAndExtractZip(z, "cortex-ia.exe"); !errors.Is(err, ErrCaseCollision) {
			t.Fatalf("expected ErrCaseCollision, got: %v", err)
		}
	})

	t.Run("DisallowedEntryTypes", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		_ = tw.WriteHeader(&tar.Header{Name: "cortex-ia", Mode: 0o755, Size: int64(len(binaryContent)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(binaryContent)
		_ = tw.WriteHeader(&tar.Header{Name: "evil-symlink", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
		_ = tw.Close()
		_ = gw.Close()

		if _, err := ValidateAndExtractTarGz(buf.Bytes(), "cortex-ia"); !errors.Is(err, ErrDisallowedEntryType) {
			t.Fatalf("expected ErrDisallowedEntryType for symlink, got: %v", err)
		}
	})

	t.Run("MultipleBinariesAndMissingBinary", func(t *testing.T) {
		multi := createZipWithEntries(t, map[string][]byte{
			"cortex-ia.exe":     binaryContent,
			"sub/cortex-ia.exe": binaryContent,
		})
		if _, err := ValidateAndExtractZip(multi, "cortex-ia.exe"); !errors.Is(err, ErrMultipleBinaries) {
			t.Fatalf("expected ErrMultipleBinaries for nested binary, got: %v", err)
		}

		missing := createZipWithEntries(t, map[string][]byte{
			"README.md": []byte("no binary"),
		})
		if _, err := ValidateAndExtractZip(missing, "cortex-ia.exe"); !errors.Is(err, ErrBinaryNotFound) {
			t.Fatalf("expected ErrBinaryNotFound, got: %v", err)
		}
	})

	t.Run("LateMaliciousMemberFails", func(t *testing.T) {
		// Valid binary is entry 1, but entry 2 has directory traversal
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w1, _ := zw.Create("cortex-ia.exe")
		_, _ = w1.Write(binaryContent)
		w2, _ := zw.Create("docs/../../escape.txt")
		_, _ = w2.Write([]byte("escaped"))
		_ = zw.Close()

		if _, err := ValidateAndExtractZip(buf.Bytes(), "cortex-ia.exe"); !errors.Is(err, ErrUnsafeArchivePath) {
			t.Fatalf("late malicious member must fail extraction, got: %v", err)
		}
	})

	t.Run("SuccessfulTarGzAndZipExtraction", func(t *testing.T) {
		zipBytes := createZipWithEntries(t, map[string][]byte{
			"cortex-ia.exe": binaryContent,
			"README.md":     []byte("inert docs"),
		})
		extracted, err := ValidateAndExtractBinary(zipBytes, "cortex-ia_1.5.0_windows_amd64.zip", "windows")
		if err != nil || string(extracted) != string(binaryContent) {
			t.Fatalf("zip extract failed: err=%v", err)
		}

		tarBytes := createTarWithEntries(t, map[string][]byte{
			"cortex-ia": binaryContent,
			"LICENSE":   []byte("inert license"),
		})
		extracted, err = ValidateAndExtractBinary(tarBytes, "cortex-ia_1.5.0_linux_amd64.tar.gz", "linux")
		if err != nil || string(extracted) != string(binaryContent) {
			t.Fatalf("tar.gz extract failed: err=%v", err)
		}
	})
}
