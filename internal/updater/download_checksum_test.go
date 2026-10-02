package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func createMockArchive(t *testing.T, binName, content string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	body := []byte(content)
	hdr := &tar.Header{
		Name: binName,
		Mode: 0755,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("failed to write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	h := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(h[:])
}

func TestDownloadChecksumPipeline(t *testing.T) {
	tag := "v0.9.0"
	archiveName := fmt.Sprintf("cortex-ia_%s_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), runtime.GOOS, runtime.GOARCH)
	archiveBytes, archiveHash := createMockArchive(t, "cortex-ia", "dummy-binary-payload")

	t.Run("successful_download_and_verify", func(t *testing.T) {
		checksumsContent := fmt.Sprintf("%s  %s\n", archiveHash, archiveName)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/checksums.txt":
				_, _ = w.Write([]byte(checksumsContent))
			case "/archive.tar.gz":
				_, _ = w.Write(archiveBytes)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		rel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: ChecksumsFileName, DownloadURL: srv.URL + "/checksums.txt"},
				{Name: archiveName, DownloadURL: srv.URL + "/archive.tar.gz", Size: int64(len(archiveBytes))},
			},
		}

		data, asset, err := downloadAndVerifyReleaseWithFloor(context.Background(), srv.Client(), DefaultRepo, "v0.8.0", rel, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if asset == nil || asset.Name != archiveName {
			t.Fatalf("expected asset %s, got %+v", archiveName, asset)
		}
		if !bytes.Equal(data, archiveBytes) {
			t.Fatal("downloaded bytes do not match expected archive")
		}
	})

	t.Run("digest_mismatch_fails_closed", func(t *testing.T) {
		corruptedBytes := append([]byte("tampered-"), archiveBytes...)
		checksumsContent := fmt.Sprintf("%s  %s\n", archiveHash, archiveName)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/checksums.txt":
				_, _ = w.Write([]byte(checksumsContent))
			case "/archive.tar.gz":
				_, _ = w.Write(corruptedBytes)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		rel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: ChecksumsFileName, DownloadURL: srv.URL + "/checksums.txt"},
				{Name: archiveName, DownloadURL: srv.URL + "/archive.tar.gz", Size: int64(len(corruptedBytes))},
			},
		}

		_, _, err := downloadAndVerifyReleaseWithFloor(context.Background(), srv.Client(), DefaultRepo, "v0.8.0", rel, "")
		if err == nil || !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("expected ErrDigestMismatch, got: %v", err)
		}
	})

	t.Run("missing_checksums_asset", func(t *testing.T) {
		rel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: archiveName, DownloadURL: "https://github.com/archive.tar.gz", Size: 100},
			},
		}
		_, _, err := downloadAndVerifyReleaseWithFloor(context.Background(), http.DefaultClient, DefaultRepo, "v0.8.0", rel, "")
		if !errors.Is(err, ErrChecksumsAssetNotFound) {
			t.Fatalf("expected ErrChecksumsAssetNotFound, got: %v", err)
		}
	})

	t.Run("asset_not_found_in_checksums", func(t *testing.T) {
		checksumsContent := "4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b  cortex-ia_other_platform.tar.gz\n"

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(checksumsContent))
		}))
		defer srv.Close()

		rel := &Release{
			TagName: tag,
			Assets: []ReleaseAsset{
				{Name: ChecksumsFileName, DownloadURL: srv.URL + "/checksums.txt"},
				{Name: archiveName, DownloadURL: srv.URL + "/archive.tar.gz", Size: 100},
			},
		}

		_, _, err := downloadAndVerifyReleaseWithFloor(context.Background(), srv.Client(), DefaultRepo, "v0.8.0", rel, "")
		if !errors.Is(err, ErrAssetNotFoundInChecksums) {
			t.Fatalf("expected ErrAssetNotFoundInChecksums, got: %v", err)
		}
	})

	t.Run("applied_floor_prevents_downgrade", func(t *testing.T) {
		rel := &Release{TagName: "v0.8.0"}
		_, _, err := downloadAndVerifyReleaseWithFloor(context.Background(), http.DefaultClient, DefaultRepo, "v0.7.0", rel, "v0.8.0")
		if !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("expected ErrDowngradeOrReplay, got: %v", err)
		}
	})

	t.Run("candidate_without_trust_bundle_succeeds", func(t *testing.T) {
		v := defaultVerifier()
		if err := v.RequireAuthority(); err != nil {
			t.Fatalf("expected nil authority error, got: %v", err)
		}
		candidate, err := v.UpdateCandidate("v0.9.0", "v1.0.0", "")
		if err != nil || !candidate {
			t.Fatalf("expected candidate for release build, got cand=%v err=%v", candidate, err)
		}
	})
}
