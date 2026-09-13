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
}

// Client manages checking and applying updates from GitHub.
type Client struct {
	Repo       string
	HTTPClient *http.Client
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
func (c *Client) CheckLatest(ctx context.Context, currentVersion string) (*Release, bool, error) {
	if err := RequireTrust(); err != nil {
		return nil, false, err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch latest release from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, false, fmt.Errorf("github api returned %s: %s", resp.Status, string(body))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, false, fmt.Errorf("failed to decode release payload: %w", err)
	}

	hasUpdate, err := CheckUpdateCandidate(currentVersion, rel.TagName)
	if err != nil {
		hasUpdate = IsNewer(currentVersion, rel.TagName)
	}
	return &rel, hasUpdate, nil
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
	if err := RequireTrust(); err != nil {
		return err
	}
	if rel == nil || strings.TrimSpace(rel.TagName) == "" {
		return errors.New("release cannot be nil and tag name cannot be empty")
	}
	if IsDevOrUnknown(currentVersion) {
		return fmt.Errorf("%w: current version is %q", ErrDevUnknownVersion, currentVersion)
	}

	archiveBytes, art, err := DownloadAndVerifyRelease(ctx, c.HTTPClient, c.Repo, currentVersion, rel)
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

	return ReplaceExecutable(targetPath, binaryBytes, "")
}

func extractFromZip(data []byte, targetName string) ([]byte, error) {
	return ValidateAndExtractZip(data, targetName)
}

func extractFromTarGz(data []byte, targetName string) ([]byte, error) {
	return ValidateAndExtractTarGz(data, targetName)
}
