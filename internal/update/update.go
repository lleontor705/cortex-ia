package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	repoOwner = "lleontor705"
	repoName  = "cortex-ia"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// Release describes a GitHub release.
type Release struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

// CheckResult describes the outcome of a version check.
type CheckResult struct {
	CurrentVersion string
	LatestRelease  *Release
	UpToDate       bool
	Error          error
}

// Check queries GitHub for the latest release and compares with current version.
func Check(currentVersion string) CheckResult {
	result := CheckResult{CurrentVersion: currentVersion}

	if currentVersion == "" || currentVersion == "dev" {
		result.UpToDate = true
		return result
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		result.Error = fmt.Errorf("check update: %w", err)
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("GitHub API returned %d", resp.StatusCode)
		return result
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		result.Error = fmt.Errorf("parse release: %w", err)
		return result
	}

	result.LatestRelease = &release
	result.UpToDate = normalizeVersion(currentVersion) == normalizeVersion(release.TagName)
	return result
}

// normalizeVersion strips "v" prefix for comparison.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// FormatCheckResult returns a human-readable string for the check result.
func FormatCheckResult(r CheckResult) string {
	if r.Error != nil {
		return fmt.Sprintf("Update check failed: %v", r.Error)
	}
	if r.UpToDate {
		return fmt.Sprintf("cortex-ia %s is up to date.", r.CurrentVersion)
	}
	return fmt.Sprintf("Update available: %s → %s\n  %s\n  Run: curl -sSL https://raw.githubusercontent.com/%s/%s/main/scripts/install.sh | bash",
		r.CurrentVersion, r.LatestRelease.TagName, r.LatestRelease.HTMLURL, repoOwner, repoName)
}
