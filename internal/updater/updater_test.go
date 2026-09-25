package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type captureTransport struct {
	status        int
	body          string
	header        http.Header
	authorization string
	conditional   string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.authorization = req.Header.Get("Authorization")
	t.conditional = req.Header.Get("If-None-Match")

	if t.header == nil {
		t.header = make(http.Header)
	}
	return &http.Response{
		StatusCode: t.status,
		Status:     fmt.Sprintf("%d %s", t.status, http.StatusText(t.status)),
		Header:     t.header,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}

func withCheckTrust(t *testing.T) {
	t.Helper()
	t.Cleanup(SetTrustedKeysForTesting([]TrustedKey{{ID: "check-test-key", PublicKey: make([]byte, ed25519.PublicKeySize)}}))
}

const latestReleaseBody = `{"tag_name":"v0.5.0","published_at":"2026-01-01T00:00:00Z","assets":[]}`

func TestCheckLatestAuthorizationHeader(t *testing.T) {
	withCheckTrust(t)

	cases := []struct{ name, github, gh, want string }{
		{"github token wins", "token-a", "token-b", "Bearer token-a"},
		{"gh token fallback", "", "token-b", "Bearer token-b"},
		{"no token omits header", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tc.github)
			t.Setenv("GH_TOKEN", tc.gh)

			tr := &captureTransport{status: http.StatusOK, body: latestReleaseBody}
			client := New("owner/repo")
			client.HTTPClient = &http.Client{Transport: tr}
			if _, _, err := client.CheckLatestFresh(context.Background(), "v0.4.9"); err != nil {
				t.Fatalf("CheckLatestFresh: %v", err)
			}
			if tr.authorization != tc.want {
				t.Errorf("authorization = %q, want %q", tr.authorization, tc.want)
			}
		})
	}
}

func TestCheckLatestTypedStatusErrors(t *testing.T) {
	withCheckTrust(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	reset := strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)

	cases := []struct {
		name      string
		status    int
		header    http.Header
		want      error
		wantMsg   string
		wantReset bool
	}{
		{"exhausted rate limit", http.StatusForbidden, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {reset}}, ErrRateLimited, "rate limit", true},
		{"access denied", http.StatusForbidden, http.Header{"X-Ratelimit-Remaining": {"42"}}, ErrGitHubAccessDenied, "access denied", false},
		{"release not found", http.StatusNotFound, nil, ErrReleaseNotFound, "no latest release", false},
		{"server error", http.StatusInternalServerError, nil, ErrGitHubServerError, "500", false},
		{"unexpected status", http.StatusTeapot, nil, ErrUnexpectedStatus, "418", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &captureTransport{status: tc.status, header: tc.header, body: "boom"}
			client := New("owner/repo")
			client.HTTPClient = &http.Client{Transport: tr}

			rel, hasUpdate, err := client.CheckLatest(context.Background(), "v0.4.9")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if hasUpdate || rel != nil {
				t.Fatalf("failed check must yield no release, got rel=%v hasUpdate=%v", rel, hasUpdate)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("message %q missing %q", err.Error(), tc.wantMsg)
			}
			if tc.wantReset && !strings.Contains(err.Error(), "resets") {
				t.Errorf("rate-limit message must name the reset time: %q", err.Error())
			}
		})
	}
}

func TestCheckLatestConditionalETagCacheHit(t *testing.T) {
	withCheckTrust(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	home := t.TempDir()
	if err := SaveUpdateStateAtomic(home, UpdateState{
		SchemaVersion: UpdateStateSchemaVersion,
		ReleaseETag:   `"etag-cached"`,
		Available:     "v0.5.0",
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	t.Run("not modified reuses the cached release", func(t *testing.T) {
		tr := &captureTransport{status: http.StatusNotModified}
		client := New("owner/repo")
		client.StateHome = home
		client.HTTPClient = &http.Client{Transport: tr}

		rel, hasUpdate, err := client.CheckLatest(context.Background(), "v0.4.9")
		if err != nil {
			t.Fatalf("CheckLatest: %v", err)
		}
		if tr.conditional != `"etag-cached"` {
			t.Errorf("If-None-Match = %q, want the cached etag", tr.conditional)
		}
		if !hasUpdate || rel == nil || rel.TagName != "v0.5.0" || rel.ETag != `"etag-cached"` {
			t.Fatalf("304 must reuse the cached release: rel=%+v hasUpdate=%v", rel, hasUpdate)
		}
	})

	t.Run("fresh fetch omits the conditional header and captures the etag", func(t *testing.T) {
		tr := &captureTransport{status: http.StatusOK, body: latestReleaseBody, header: http.Header{"Etag": {`"etag-200"`}}}
		client := New("owner/repo")
		client.StateHome = home
		client.HTTPClient = &http.Client{Transport: tr}

		rel, _, err := client.CheckLatestFresh(context.Background(), "v0.4.9")
		if err != nil {
			t.Fatalf("CheckLatestFresh: %v", err)
		}
		if tr.conditional != "" {
			t.Errorf("fresh fetch must not send If-None-Match, got %q", tr.conditional)
		}
		if rel == nil || rel.ETag != `"etag-200"` {
			t.Fatalf("response etag not captured: %+v", rel)
		}
	})
}

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
