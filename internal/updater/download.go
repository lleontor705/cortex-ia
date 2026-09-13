package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

var (
	ErrDisallowedOrigin  = errors.New("disallowed download origin or redirect host")
	ErrArchiveTooLarge   = errors.New("archive exceeds maximum allowed size (128 MiB)")
	ErrTruncatedDownload = errors.New("downloaded artifact size mismatch or truncation")
	ErrMisleadingLength  = errors.New("server returned misleading Content-Length header")
	ErrDigestMismatch    = errors.New("downloaded artifact SHA-256 digest does not match signed manifest")
	ErrMutatedRelease    = errors.New("release object mutated or inconsistent with manifest")
	ErrInsecureScheme    = errors.New("insecure download scheme: HTTPS required")
)

var allowedHosts = map[string]bool{
	"github.com":                            true,
	"api.github.com":                        true,
	"objects.githubusercontent.com":         true,
	"github-releases.githubusercontent.com": true,
	"raw.githubusercontent.com":             true,
}

func isLoopbackHost(host string) bool {
	h := host
	if colon := strings.LastIndex(h, ":"); colon != -1 {
		h = h[:colon]
	}
	h = strings.Trim(h, "[]")
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func IsAllowedURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := u.Host
	if isLoopbackHost(host) {
		return u.Scheme == "http" || u.Scheme == "https"
	}
	if u.Scheme != "https" {
		return false
	}
	hostname := u.Hostname()
	return allowedHosts[strings.ToLower(hostname)]
}

func SafeHTTPClient(base *http.Client) *http.Client {
	var transport http.RoundTripper
	var timeout = base.Timeout
	if base.Transport != nil {
		transport = base.Transport
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if !IsAllowedURL(req.URL) {
				return fmt.Errorf("%w: %s", ErrDisallowedOrigin, req.URL.String())
			}
			return nil
		},
	}
}

func DownloadArtifactBytes(ctx context.Context, client *http.Client, urlStr string, expectedSize int64, expectedSHA256 string) ([]byte, error) {
	if expectedSize <= 0 || expectedSize > MaxArtifactSize {
		return nil, fmt.Errorf("%w: invalid expected size %d", ErrArchiveTooLarge, expectedSize)
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid download URL: %w", err)
	}
	if !IsAllowedURL(parsed) {
		if parsed.Scheme != "https" && !isLoopbackHost(parsed.Host) {
			return nil, ErrInsecureScheme
		}
		return nil, fmt.Errorf("%w: %s", ErrDisallowedOrigin, parsed.Host)
	}

	safeClient := SafeHTTPClient(client)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact download request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := safeClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("artifact download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artifact download returned status %s", resp.Status)
	}

	if resp.ContentLength > MaxArtifactSize {
		return nil, fmt.Errorf("%w: Content-Length %d", ErrArchiveTooLarge, resp.ContentLength)
	}
	if resp.ContentLength != -1 && resp.ContentLength != expectedSize {
		return nil, fmt.Errorf("%w: expected %d, got header %d", ErrMisleadingLength, expectedSize, resp.ContentLength)
	}

	limitR := io.LimitReader(resp.Body, MaxArtifactSize+1)
	data, err := io.ReadAll(limitR)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact data: %w", err)
	}

	if int64(len(data)) > MaxArtifactSize {
		return nil, ErrArchiveTooLarge
	}
	if int64(len(data)) != expectedSize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrTruncatedDownload, expectedSize, len(data))
	}

	h := sha256.Sum256(data)
	actualSHA256 := hex.EncodeToString(h[:])
	if !strings.EqualFold(actualSHA256, expectedSHA256) {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrDigestMismatch, expectedSHA256, actualSHA256)
	}

	return data, nil
}

func DownloadAndVerifyRelease(ctx context.Context, client *http.Client, repo, currentVersion string, rel *Release) ([]byte, *ManifestArtifact, error) {
	if err := RequireTrust(); err != nil {
		return nil, nil, err
	}
	if rel == nil || strings.TrimSpace(rel.TagName) == "" {
		return nil, nil, errors.New("release cannot be nil and tag name cannot be empty")
	}
	if IsDevOrUnknown(currentVersion) {
		return nil, nil, fmt.Errorf("%w: current version is %q", ErrDevUnknownVersion, currentVersion)
	}
	if err := VerifyVersionFloor(currentVersion, rel.TagName, ""); err != nil {
		return nil, nil, err
	}

	var manAsset, sigAsset *ReleaseAsset
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case "release-manifest.json":
			manAsset = &rel.Assets[i]
		case "release-manifest.sig":
			sigAsset = &rel.Assets[i]
		}
	}
	if manAsset == nil || sigAsset == nil {
		return nil, nil, fmt.Errorf("release %s missing signed release-manifest.json or release-manifest.sig asset", rel.TagName)
	}

	safeClient := SafeHTTPClient(client)

	fetchBytes := func(u string, limit int64) ([]byte, error) {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		if !IsAllowedURL(parsed) {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedOrigin, parsed.Host)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		resp, err := safeClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %s", resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, limit+1))
	}

	rawSig, err := fetchBytes(sigAsset.DownloadURL, MaxSignatureEnvelopeSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch signature envelope: %w", err)
	}
	rawMan, err := fetchBytes(manAsset.DownloadURL, MaxManifestSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch release manifest: %w", err)
	}

	manifest, err := VerifyManifest(rawMan, rawSig, repo, rel.TagName)
	if err != nil {
		return nil, nil, fmt.Errorf("manifest verification failed: %w", err)
	}

	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH
	art, err := FindManifestArtifact(manifest, targetOS, targetArch)
	if err != nil {
		return nil, nil, err
	}

	var matchedRelAsset *ReleaseAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == art.Name {
			matchedRelAsset = &rel.Assets[i]
			break
		}
	}
	if matchedRelAsset == nil {
		return nil, nil, fmt.Errorf("%w: release payload lacks exact asset %q", ErrMutatedRelease, art.Name)
	}

	archiveBytes, err := DownloadArtifactBytes(ctx, client, matchedRelAsset.DownloadURL, art.Size, art.SHA256)
	if err != nil {
		return nil, nil, fmt.Errorf("artifact byte verification failed: %w", err)
	}

	return archiveBytes, art, nil
}
