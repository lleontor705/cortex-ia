package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo = "lleontor705/cortex-ia"
	UserAgent   = "cortex-ia-updater"
)

// ErrNonCanonicalVersion reports that a version involved in an update check is
// not a canonical vMAJOR.MINOR.PATCH release. CheckLatest refuses to announce an
// update for such builds so the check surface cannot advertise a release that
// ApplyUpdate would reject via its own canonical parsing.
var ErrNonCanonicalVersion = errors.New("non-canonical version cannot participate in update checks")

// Typed GitHub API failures surfaced by release checks. CLI surfaces distinguish
// an exhausted rate limit (retryable after reset) from a genuinely missing
// release and from transient server failures so the guidance they print is
// actionable instead of a raw status dump.
var (
	ErrRateLimited        = errors.New("github api rate limit exceeded")
	ErrGitHubAccessDenied = errors.New("github api access denied")
	ErrReleaseNotFound    = errors.New("github release not found")
	ErrGitHubServerError  = errors.New("github api server error")
	ErrUnexpectedStatus   = errors.New("github api returned an unexpected status")
)

// ReleaseAsset represents an asset attached to a GitHub release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// Release represents a release on GitHub.
type Release struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	PublishedAt time.Time      `json:"published_at"`
	HTMLURL     string         `json:"html_url"`
	Body        string         `json:"body"`
	Assets      []ReleaseAsset `json:"assets"`
	// ETag is the HTTP validator observed for this payload. It is transport
	// metadata persisted in update state, never part of the GitHub JSON body.
	ETag string `json:"-"`
}

// Client manages checking and applying updates from GitHub.
type Client struct {
	Repo       string
	HTTPClient *http.Client
	// AppliedFloor is the highest version already applied on this machine. It
	// is read from persisted state by the caller and enforced before download.
	AppliedFloor string
	// StateHome is the Cortex-IA state root used to load the persisted floor
	// and to record the floor after a successful apply. Empty disables state
	// I/O entirely so library callers and tests never touch a real home.
	StateHome string
}

// New creates a new updater client for the specified GitHub repository.
func New(repo string) *Client {
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{
		Repo: repo,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckLatest queries the GitHub Releases API for the newest published release.
// It returns the Release, a boolean indicating if an update is available, and any error.
// A current or candidate version that is not canonical yields hasUpdate false with
// ErrNonCanonicalVersion; dev/unknown current versions report no update without error.
//
// When the client is bound to a state home, the persisted release ETag is sent as
// If-None-Match and an HTTP 304 is answered from the cached tag as a fresh cache
// hit, avoiding a redundant payload transfer.
func (c *Client) CheckLatest(ctx context.Context, currentVersion string) (*Release, bool, error) {
	return c.checkLatest(ctx, currentVersion, true)
}

// CheckLatestFresh is CheckLatest without conditional caching. Apply paths must
// use it because the download step needs the full asset list, which a 304 cache
// hit cannot provide.
func (c *Client) CheckLatestFresh(ctx context.Context, currentVersion string) (*Release, bool, error) {
	return c.checkLatest(ctx, currentVersion, false)
}

func (c *Client) checkLatest(ctx context.Context, currentVersion string, conditional bool) (*Release, bool, error) {
	verifier := defaultVerifier()
	if err := verifier.RequireAuthority(); err != nil {
		return nil, false, err
	}

	var cached UpdateState
	if conditional && c.StateHome != "" {
		cached, _ = LoadUpdateState(c.StateHome)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if etag := strings.TrimSpace(cached.ReleaseETag); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch latest release from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return cachedReleaseFromState(cached, currentVersion, strings.TrimSpace(resp.Header.Get("ETag")), verifier, c.effectiveAppliedFloor())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, githubAPIError(resp, c.Repo)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, false, fmt.Errorf("failed to decode release payload: %w", err)
	}
	rel.ETag = strings.TrimSpace(resp.Header.Get("ETag"))

	hasUpdate, err := verifier.UpdateCandidate(currentVersion, rel.TagName, c.effectiveAppliedFloor())
	if err != nil {
		return &rel, false, fmt.Errorf("%w: %w", ErrNonCanonicalVersion, err)
	}
	return &rel, hasUpdate, nil
}

// githubToken returns a GitHub API credential from the environment. GITHUB_TOKEN
// is preferred so a CI-provided token wins over an interactive gh login when
// both are exported.
func githubToken() string {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token
		}
	}
	return ""
}

// cachedReleaseFromState rebuilds the release reported after a 304 Not Modified.
// Only the tag and ETag are known locally, which is enough for check surfaces;
// an apply re-fetches the full payload through CheckLatestFresh.
func cachedReleaseFromState(state UpdateState, currentVersion, headerETag string, verifier ReleaseVerifier, appliedFloor string) (*Release, bool, error) {
	tag := strings.TrimSpace(state.Available)
	etag := strings.TrimSpace(headerETag)
	if etag == "" {
		etag = strings.TrimSpace(state.ReleaseETag)
	}
	if tag == "" {
		return nil, false, nil
	}

	rel := &Release{TagName: tag, ETag: etag}
	hasUpdate, err := verifier.UpdateCandidate(currentVersion, tag, appliedFloor)
	if err != nil {
		return rel, false, fmt.Errorf("%w: %w", ErrNonCanonicalVersion, err)
	}
	return rel, hasUpdate, nil
}

// githubAPIError maps a non-200 release response to a typed error with operator
// guidance. Only 403 with an exhausted remaining quota is a rate limit; other
// 403 responses are access problems.
func githubAPIError(resp *http.Response, repo string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	detail := strings.TrimSpace(string(body))

	switch {
	case resp.StatusCode == http.StatusForbidden && strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining")) == "0":
		return fmt.Errorf("%w for %s%s: export GITHUB_TOKEN or GH_TOKEN to raise the limit, then retry",
			ErrRateLimited, repo, rateLimitResetHint(resp.Header.Get("X-RateLimit-Reset")))
	case resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: %s for %s; verify the GITHUB_TOKEN or GH_TOKEN scope", ErrGitHubAccessDenied, resp.Status, repo)
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s has no latest release (%s)", ErrReleaseNotFound, repo, resp.Status)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: %s from %s%s", ErrGitHubServerError, resp.Status, repo, detailSuffix(detail))
	default:
		return fmt.Errorf("%w: %s from %s%s", ErrUnexpectedStatus, resp.Status, repo, detailSuffix(detail))
	}
}

func rateLimitResetHint(raw string) string {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return " (resets " + time.Unix(seconds, 0).UTC().Format(time.RFC3339) + ")"
}

func detailSuffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

// CompareSemver compares two semver tags (e.g. "v0.4.14" and "v0.4.15").
// Returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func CompareSemver(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimSpace(v1), "v")
	clean2 := strings.TrimPrefix(strings.TrimSpace(v2), "v")

	if clean1 == "dev" && clean2 == "dev" {
		return 0
	}
	if clean1 == "dev" {
		return -1 // dev is treated as older than tagged releases
	}
	if clean2 == "dev" {
		return 1
	}

	// Remove any build metadata or pre-release suffixes for comparison
	parts1 := parseVersionNumbers(clean1)
	parts2 := parseVersionNumbers(clean2)

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	if len(parts1) < len(parts2) {
		return -1
	}
	if len(parts1) > len(parts2) {
		return 1
	}
	return 0
}

func parseVersionNumbers(v string) []int {
	mainPart := strings.SplitN(v, "-", 2)[0]
	segments := strings.Split(mainPart, ".")
	nums := make([]int, 0, len(segments))
	for _, seg := range segments {
		n, err := strconv.Atoi(seg)
		if err != nil {
			nums = append(nums, 0)
		} else {
			nums = append(nums, n)
		}
	}
	return nums
}

// IsNewer returns true if candidateVersion is newer than currentVersion.
func IsNewer(currentVersion, candidateVersion string) bool {
	return CompareSemver(currentVersion, candidateVersion) < 0
}

// FindAsset locates the archive asset for the specified target OS and architecture.
func FindAsset(assets []ReleaseAsset, targetOS, targetArch, tagName string) (*ReleaseAsset, error) {
	verNum := strings.TrimPrefix(tagName, "v")

	// Exact target pattern: cortex-ia_{version}_{os}_{arch}.{zip|tar.gz}
	expectedExt := ".tar.gz"
	if targetOS == "windows" {
		expectedExt = ".zip"
	}
	expectedName := fmt.Sprintf("cortex-ia_%s_%s_%s%s", verNum, targetOS, targetArch, expectedExt)

	for _, asset := range assets {
		if strings.EqualFold(asset.Name, expectedName) {
			return &asset, nil
		}
	}

	// Fallback match by OS and Arch
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, targetOS) && strings.Contains(name, targetArch) && strings.HasSuffix(name, expectedExt) {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("no compatible release asset found for %s/%s (looked for %s)", targetOS, targetArch, expectedName)
}

// ApplyUpdate downloads the release asset for the current OS/Arch and replaces the running binary.
func (c *Client) ApplyUpdate(ctx context.Context, currentVersion string, rel *Release) error {
	return c.ApplyUpdateToTarget(ctx, currentVersion, rel, "")
}

// ApplyUpdateToTarget downloads the release asset for the current OS/Arch, verifies metadata, signature,
// and artifact digest, safely extracts the executable, and replaces the target binary.
// If targetPath is empty, it resolves os.Executable().
func (c *Client) ApplyUpdateToTarget(ctx context.Context, currentVersion string, rel *Release, targetPath string) error {
	verifier := defaultVerifier()
	if err := verifier.RequireAuthority(); err != nil {
		return err
	}
	tag := ""
	if rel != nil {
		tag = rel.TagName
	}
	if err := verifier.CheckEligibility(currentVersion, tag, c.effectiveAppliedFloor()); err != nil {
		return err
	}

	archiveBytes, art, err := downloadAndVerifyReleaseWithFloor(ctx, c.HTTPClient, c.Repo, currentVersion, rel, c.effectiveAppliedFloor())
	if err != nil {
		return err
	}

	binaryBytes, err := ValidateAndExtractBinary(archiveBytes, art.Name, runtime.GOOS)
	if err != nil {
		return err
	}

	if targetPath == "" {
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot locate current executable: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("cannot resolve executable symlink: %w", err)
		}
		targetPath = execPath
	}

	if err := ReplaceExecutable(targetPath, binaryBytes, ""); err != nil {
		return err
	}

	if c.StateHome != "" {
		if err := RecordAppliedFloor(c.StateHome, rel.TagName, targetPath); err != nil {
			// The binary is already replaced; surface the write failure instead
			// of losing the anti-replay floor silently.
			return fmt.Errorf("binary replaced but update state could not be persisted: %w", err)
		}
	}
	return nil
}

// effectiveAppliedFloor prefers the caller-supplied floor and falls back to
// persisted state when the client is bound to a state home.
func (c *Client) effectiveAppliedFloor() string {
	if floor := strings.TrimSpace(c.AppliedFloor); floor != "" {
		return c.AppliedFloor
	}
	if c.StateHome == "" {
		return ""
	}
	state, err := LoadUpdateState(c.StateHome)
	if err != nil {
		return ""
	}
	return state.AppliedFloor
}

func extractFromZip(data []byte, targetName string) ([]byte, error) {
	return ValidateAndExtractZip(data, targetName)
}

func extractFromTarGz(data []byte, targetName string) ([]byte, error) {
	return ValidateAndExtractTarGz(data, targetName)
}
