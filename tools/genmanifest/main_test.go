package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

const (
	testRepo  = "lleontor705/cortex-ia"
	testTag   = "v0.5.0"
	testKeyID = "release-2026-01"
)

type fixture struct {
	name    string
	os      string
	arch    string
	payload []byte
}

func testFixtures() []fixture {
	return []fixture{
		{name: "cortex-ia_0.5.0_windows_amd64.zip", os: "windows", arch: "amd64", payload: []byte("windows-amd64-binary")},
		{name: "cortex-ia_0.5.0_linux_amd64.tar.gz", os: "linux", arch: "amd64", payload: []byte("linux-amd64-binary")},
		{name: "cortex-ia_0.5.0_darwin_arm64.tar.gz", os: "darwin", arch: "arm64", payload: []byte("darwin-arm64-binary")},
	}
}

func generateKey(t *testing.T) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(priv), pub
}

func signingEnv(encodedKey string) func(string) string {
	return func(key string) string {
		switch key {
		case signingKeyEnv:
			return encodedKey
		case keyIDEnv:
			return testKeyID
		default:
			return ""
		}
	}
}

func trustStore(t *testing.T, pub ed25519.PublicKey) {
	t.Helper()
	t.Cleanup(updater.SetTrustedKeysForTesting([]updater.TrustedKey{{
		ID:         testKeyID,
		PublicKey:  pub,
		Repository: testRepo,
		MinVersion: "v0.1.0",
	}}))
}

func writeFixtures(t *testing.T, dir string, fixtures []fixture) {
	t.Helper()
	for _, f := range fixtures {
		var archive []byte
		if f.os == "windows" {
			archive = zipArchive(t, f.payload)
		} else {
			archive = tarGzArchive(t, f.payload)
		}
		if err := os.WriteFile(filepath.Join(dir, f.name), archive, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", f.name, err)
		}
	}
}

func zipArchive(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("cortex-ia.exe")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func tarGzArchive(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "cortex-ia", Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("write tar payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func assertNoOutput(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no output files, found %d", len(entries))
	}
}

func TestRunProducesManifestVerifiableByUpdater(t *testing.T) {
	dist, out := t.TempDir(), t.TempDir()
	fixtures := testFixtures()
	writeFixtures(t, dist, fixtures)

	encodedKey, pub := generateKey(t)
	trustStore(t, pub)
	if err := run([]string{"-tag", testTag, "-dist", dist, "-out", out}, signingEnv(encodedKey)); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	rawManifest, err := os.ReadFile(filepath.Join(out, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	rawSig, err := os.ReadFile(filepath.Join(out, signatureFileName))
	if err != nil {
		t.Fatalf("read signature: %v", err)
	}

	manifest, err := updater.VerifyManifest(rawManifest, rawSig, testRepo, testTag)
	if err != nil {
		t.Fatalf("updater rejected the generated manifest: %v", err)
	}
	if manifest.Repository != testRepo || manifest.Tag != testTag || manifest.SchemaVersion != updater.ManifestSchemaVersion {
		t.Fatalf("manifest binding wrong: %+v", *manifest)
	}
	if len(manifest.Artifacts) != len(fixtures) {
		t.Fatalf("expected %d artifacts, got %d", len(fixtures), len(manifest.Artifacts))
	}

	for _, f := range fixtures {
		artifact, err := updater.FindManifestArtifact(manifest, f.os, f.arch)
		if err != nil {
			t.Fatalf("resolve %s/%s: %v", f.os, f.arch, err)
		}
		if artifact.Name != f.name {
			t.Fatalf("artifact name = %q, want %q", artifact.Name, f.name)
		}
		archive, err := os.ReadFile(filepath.Join(dist, f.name))
		if err != nil {
			t.Fatalf("read archive %s: %v", f.name, err)
		}
		digest := sha256.Sum256(archive)
		if artifact.SHA256 != hex.EncodeToString(digest[:]) || artifact.Size != int64(len(archive)) {
			t.Fatalf("%s digest/size mismatch: %+v", f.name, *artifact)
		}
		extracted, err := updater.ValidateAndExtractBinary(archive, f.name, f.os)
		if err != nil {
			t.Fatalf("updater could not extract %s: %v", f.name, err)
		}
		if !bytes.Equal(extracted, f.payload) {
			t.Fatalf("extracted payload mismatch for %s", f.name)
		}
	}
}

func TestRunMissingSigningKeyFailsClosed(t *testing.T) {
	dist, out := t.TempDir(), t.TempDir()
	writeFixtures(t, dist, testFixtures()[:1])

	env := func(key string) string {
		if key == keyIDEnv {
			return testKeyID
		}
		return ""
	}
	err := run([]string{"-tag", testTag, "-dist", dist, "-out", out}, env)
	if !errors.Is(err, ErrMissingSigningKey) {
		t.Fatalf("expected ErrMissingSigningKey, got %v", err)
	}
	assertNoOutput(t, out)
}

func TestRunNonCanonicalTagFailsClosed(t *testing.T) {
	dist, out := t.TempDir(), t.TempDir()
	writeFixtures(t, dist, testFixtures()[:1])

	encodedKey, _ := generateKey(t)
	err := run([]string{"-tag", "v0.5.0-rc1", "-dist", dist, "-out", out}, signingEnv(encodedKey))
	if !errors.Is(err, ErrNonCanonicalTag) {
		t.Fatalf("expected ErrNonCanonicalTag, got %v", err)
	}
	assertNoOutput(t, out)
}

func TestTamperedManifestRejected(t *testing.T) {
	dist, out := t.TempDir(), t.TempDir()
	writeFixtures(t, dist, testFixtures()[:1])

	encodedKey, pub := generateKey(t)
	trustStore(t, pub)
	if err := run([]string{"-tag", testTag, "-dist", dist, "-out", out}, signingEnv(encodedKey)); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	rawManifest, err := os.ReadFile(filepath.Join(out, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	rawSig, err := os.ReadFile(filepath.Join(out, signatureFileName))
	if err != nil {
		t.Fatalf("read signature: %v", err)
	}

	tampered := bytes.Replace(rawManifest, []byte("windows"), []byte("wind0ws"), 1)
	if bytes.Equal(tampered, rawManifest) {
		t.Fatal("tamper fixture did not modify the manifest")
	}
	if _, err := updater.VerifyManifest(tampered, rawSig, testRepo, testTag); !errors.Is(err, updater.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for a tampered manifest, got %v", err)
	}
}
