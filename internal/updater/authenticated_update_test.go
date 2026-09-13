package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func createTestArchive(name string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(data); err != nil {
		return nil, err
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes(), nil
}

func setupReleaseServer(t *testing.T, tag string, priv ed25519.PrivateKey, keyID string, data []byte, corrupt bool) (*Release, *httptest.Server) {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	verNum := strings.TrimPrefix(tag, "v")
	name := fmt.Sprintf("cortex-ia_%s_%s_%s%s", verNum, runtime.GOOS, runtime.GOARCH, ext)
	h := sha256.Sum256(data)
	man := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Repository:    DefaultRepo,
		Tag:           tag,
		Artifacts:     []ManifestArtifact{{Name: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Size: int64(len(data)), SHA256: hex.EncodeToString(h[:])}},
	}
	rawMan, _ := json.Marshal(man)
	rawSig, _ := SignManifest(rawMan, keyID, priv)
	if corrupt {
		rawSig = bytes.Replace(rawSig, []byte("a"), []byte("b"), 1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/m.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(rawMan) })
	mux.HandleFunc("/m.sig", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(rawSig) })
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	})
	srv := httptest.NewServer(mux)

	return &Release{
		TagName: tag,
		Assets: []ReleaseAsset{
			{Name: "release-manifest.json", DownloadURL: srv.URL + "/m.json", Size: int64(len(rawMan))},
			{Name: "release-manifest.sig", DownloadURL: srv.URL + "/m.sig", Size: int64(len(rawSig))},
			{Name: name, DownloadURL: srv.URL + "/bin", Size: int64(len(data))},
		},
	}, srv
}

func TestAuthenticatedUpdate(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen failed: %v", err)
	}
	keyID := "test-key-u4"
	defer SetTrustedKeysForTesting([]TrustedKey{
		{ID: keyID, PublicKey: pub, Repository: DefaultRepo, MinVersion: "v1.0.0"},
	})()

	binName := "cortex-ia"
	if runtime.GOOS == "windows" {
		binName = "cortex-ia.exe"
	}

	validBytes := []byte("new-authenticated-executable-v1.1.0")
	archiveBytes, err := createTestArchive(binName, validBytes)
	if err != nil {
		t.Fatalf("create archive failed: %v", err)
	}

	t.Run("producer consumer success", func(t *testing.T) {
		rel, srv := setupReleaseServer(t, "v1.1.0", priv, keyID, archiveBytes, false)
		defer srv.Close()

		target := filepath.Join(t.TempDir(), binName)
		_ = os.WriteFile(target, []byte("old-v1.0.0"), 0o755)

		c := New("")
		if err := c.ApplyUpdateToTarget(context.Background(), "v1.0.0", rel, target); err != nil {
			t.Fatalf("ApplyUpdateToTarget failed: %v", err)
		}
		got, _ := os.ReadFile(target)
		if !bytes.Equal(got, validBytes) {
			t.Fatalf("target mismatch: got %q", got)
		}
		if _, err := os.Stat(target + ".old"); err == nil {
			t.Fatalf(".old backup not cleaned up")
		}
	})

	t.Run("authenticated equal check", func(t *testing.T) {
		rel, srv := setupReleaseServer(t, "v1.1.0", priv, keyID, archiveBytes, false)
		defer srv.Close()

		target := filepath.Join(t.TempDir(), binName)
		cur := []byte("bin-v1.1.0")
		_ = os.WriteFile(target, cur, 0o755)

		hasUpdate, err := CheckUpdateCandidate("v1.1.0", rel.TagName)
		if err != nil || hasUpdate {
			t.Fatalf("expected hasUpdate=false, got %v, err=%v", hasUpdate, err)
		}

		c := New("")
		err = c.ApplyUpdateToTarget(context.Background(), "v1.1.0", rel, target)
		if !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("expected ErrDowngradeOrReplay on equal check, got %v", err)
		}
		got, _ := os.ReadFile(target)
		if !bytes.Equal(got, cur) {
			t.Fatalf("target was modified during equal check")
		}
	})

	t.Run("successful applied floor replay protection", func(t *testing.T) {
		relOlder, srv := setupReleaseServer(t, "v1.0.5", priv, keyID, archiveBytes, false)
		defer srv.Close()

		target := filepath.Join(t.TempDir(), binName)
		cur := []byte("bin-v1.1.0")
		_ = os.WriteFile(target, cur, 0o755)

		c := New("")
		err := c.ApplyUpdateToTarget(context.Background(), "v1.1.0", relOlder, target)
		if !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("expected ErrDowngradeOrReplay on replay, got %v", err)
		}
		if err := VerifyVersionFloor("v1.1.0", "v1.0.9", "v1.1.0"); !errors.Is(err, ErrDowngradeOrReplay) {
			t.Fatalf("expected floor rejection: %v", err)
		}
		got, _ := os.ReadFile(target)
		if !bytes.Equal(got, cur) {
			t.Fatalf("target modified on replay")
		}
	})

	t.Run("broken chain unchanged target", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), binName)
		orig := []byte("orig-bytes")
		c := New("")

		// Bad signature
		_ = os.WriteFile(target, orig, 0o755)
		relBadSig, srv1 := setupReleaseServer(t, "v1.2.0", priv, keyID, archiveBytes, true)
		defer srv1.Close()
		if err := c.ApplyUpdateToTarget(context.Background(), "v1.0.0", relBadSig, target); err == nil {
			t.Fatal("expected error on bad signature")
		}
		got, _ := os.ReadFile(target)
		if !bytes.Equal(got, orig) {
			t.Fatal("target modified on bad signature")
		}

		// Corrupted archive digest
		_ = os.WriteFile(target, orig, 0o755)
		badArchive := append([]byte(nil), archiveBytes...)
		badArchive[len(badArchive)/2] ^= 0xFF
		relBadArch, srv2 := setupReleaseServer(t, "v1.2.0", priv, keyID, badArchive, false)
		defer srv2.Close()
		if err := c.ApplyUpdateToTarget(context.Background(), "v1.0.0", relBadArch, target); err == nil {
			t.Fatal("expected error on bad digest")
		}
		got, _ = os.ReadFile(target)
		if !bytes.Equal(got, orig) {
			t.Fatal("target modified on bad digest")
		}

		// Unsafe archive path
		_ = os.WriteFile(target, orig, 0o755)
		badPathArch, _ := createTestArchive("../escape.exe", validBytes)
		relBadPath, srv3 := setupReleaseServer(t, "v1.2.0", priv, keyID, badPathArch, false)
		defer srv3.Close()
		if err := c.ApplyUpdateToTarget(context.Background(), "v1.0.0", relBadPath, target); err == nil {
			t.Fatal("expected error on unsafe path")
		}
		got, _ = os.ReadFile(target)
		if !bytes.Equal(got, orig) {
			t.Fatal("target modified on unsafe path")
		}

		// Missing trusted key
		_ = os.WriteFile(target, orig, 0o755)
		relValid, srv4 := setupReleaseServer(t, "v1.2.0", priv, keyID, archiveBytes, false)
		defer srv4.Close()
		reset := SetTrustedKeysForTesting(nil)
		err := c.ApplyUpdateToTarget(context.Background(), "v1.0.0", relValid, target)
		reset()
		if !errors.Is(err, ErrNoTrustedKey) {
			t.Fatalf("expected ErrNoTrustedKey, got %v", err)
		}
		got, _ = os.ReadFile(target)
		if !bytes.Equal(got, orig) {
			t.Fatal("target modified on missing trust")
		}
	})
}
