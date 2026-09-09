package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"v0.4.14", "v0.4.15", -1},
		{"v0.4.15", "v0.4.14", 1},
		{"v0.4.14", "v0.4.14", 0},
		{"0.4.14", "v0.4.14", 0},
		{"v1.0.0", "v0.4.14", 1},
		{"v0.4.9", "v0.4.14", -1},
		{"v0.4.14", "v0.4.9", 1},
		{"dev", "v0.4.14", -1},
		{"v0.4.14", "dev", 1},
		{"dev", "dev", 0},
		{"v0.4.14-beta.1", "v0.4.14", 0},
	}

	for _, tt := range tests {
		got := CompareSemver(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("CompareSemver(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v0.4.14", "v0.4.15") {
		t.Error("expected v0.4.15 to be newer than v0.4.14")
	}
	if IsNewer("v0.4.15", "v0.4.14") {
		t.Error("did not expect v0.4.14 to be newer than v0.4.15")
	}
	if IsNewer("v0.4.14", "v0.4.14") {
		t.Error("did not expect v0.4.14 to be newer than v0.4.14")
	}
	if !IsNewer("dev", "v0.4.14") {
		t.Error("expected tagged release to be newer than dev")
	}
}

func TestFindAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "cortex-ia_0.4.14_windows_amd64.zip", DownloadURL: "https://example.com/win_amd64.zip"},
		{Name: "cortex-ia_0.4.14_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux_amd64.tar.gz"},
		{Name: "cortex-ia_0.4.14_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin_arm64.tar.gz"},
	}

	// Match Windows amd64
	assetWin, err := FindAsset(assets, "windows", "amd64", "v0.4.14")
	if err != nil {
		t.Fatalf("unexpected error finding windows/amd64: %v", err)
	}
	if assetWin.Name != "cortex-ia_0.4.14_windows_amd64.zip" {
		t.Errorf("expected windows asset, got %s", assetWin.Name)
	}

	// Match Linux amd64
	assetLinux, err := FindAsset(assets, "linux", "amd64", "v0.4.14")
	if err != nil {
		t.Fatalf("unexpected error finding linux/amd64: %v", err)
	}
	if assetLinux.Name != "cortex-ia_0.4.14_linux_amd64.tar.gz" {
		t.Errorf("expected linux asset, got %s", assetLinux.Name)
	}

	// No match
	_, err = FindAsset(assets, "freebsd", "riscv64", "v0.4.14")
	if err == nil {
		t.Fatal("expected error for unsupported os/arch, got nil")
	}
}

func TestExtractFromZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("cortex-ia.exe")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("fake-binary-content")
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractFromZip(buf.Bytes(), "cortex-ia.exe")
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if string(extracted) != string(content) {
		t.Fatalf("got %q, want %q", string(extracted), string(content))
	}
}

func TestExtractFromTarGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("fake-linux-binary")
	hdr := &tar.Header{
		Name: "cortex-ia",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractFromTarGz(buf.Bytes(), "cortex-ia")
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if string(extracted) != string(content) {
		t.Fatalf("got %q, want %q", string(extracted), string(content))
	}
}
