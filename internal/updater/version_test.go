package updater

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateVersion(t *testing.T) {
	t.Run("CanonicalSemver", func(t *testing.T) {
		valid := []string{"v0.0.1", "v0.1.0", "v1.0.0", "v2.4.15", "v10.200.3000"}
		for _, s := range valid {
			v, err := ParseCanonicalVersion(s)
			if err != nil {
				t.Errorf("expected valid canonical version for %q, got error: %v", s, err)
			}
			if v.Raw != s {
				t.Errorf("expected Raw=%q, got %q", s, v.Raw)
			}
		}
	})

	t.Run("RejectLeadingZeros", func(t *testing.T) {
		leadingZeros := []string{"v01.0.0", "v1.02.0", "v1.0.03", "v00.1.1"}
		for _, s := range leadingZeros {
			if _, err := ParseCanonicalVersion(s); !errors.Is(err, ErrInvalidVersionFormat) {
				t.Errorf("expected ErrInvalidVersionFormat for leading zeros in %q, got %v", s, err)
			}
		}
	})

	t.Run("RejectSuffixes", func(t *testing.T) {
		suffixes := []string{"v1.0.0-beta", "v1.0.0+build", "v1.0.0-rc.1", "v2.0.0-alpha+123"}
		for _, s := range suffixes {
			if _, err := ParseCanonicalVersion(s); !errors.Is(err, ErrInvalidVersionFormat) {
				t.Errorf("expected ErrInvalidVersionFormat for suffix in %q, got %v", s, err)
			}
		}
	})

	t.Run("RejectMissingPrefixOrMalformed", func(t *testing.T) {
		malformed := []string{"1.0.0", "v1.0", "v1.0.0.0", "v", "", "v1.a.0", "v-1.0.0"}
		for _, s := range malformed {
			if _, err := ParseCanonicalVersion(s); !errors.Is(err, ErrInvalidVersionFormat) {
				t.Errorf("expected ErrInvalidVersionFormat for malformed %q, got %v", s, err)
			}
		}
	})

	t.Run("RejectOverflow", func(t *testing.T) {
		overflow := "v9999999999999999999999999999999999999.0.0"
		if _, err := ParseCanonicalVersion(overflow); !errors.Is(err, ErrVersionOverflow) {
			t.Errorf("expected ErrVersionOverflow for %q, got %v", overflow, err)
		}
	})

	t.Run("DevUnknownFloors", func(t *testing.T) {
		for _, cur := range []string{"dev", "DEV", "unknown", "", "  "} {
			err := VerifyVersionFloor(cur, "v1.0.0", "")
			if !errors.Is(err, ErrDevUnknownVersion) {
				t.Errorf("expected ErrDevUnknownVersion for current=%q, got %v", cur, err)
			}
		}
	})

	t.Run("EqualOldReplayedApply", func(t *testing.T) {
		// Equal version rejected
		if err := VerifyVersionFloor("v1.2.0", "v1.2.0", ""); !errors.Is(err, ErrDowngradeOrReplay) {
			t.Errorf("expected ErrDowngradeOrReplay for equal version, got %v", err)
		}
		// Downgrade rejected
		if err := VerifyVersionFloor("v1.2.0", "v1.1.9", ""); !errors.Is(err, ErrDowngradeOrReplay) {
			t.Errorf("expected ErrDowngradeOrReplay for downgrade, got %v", err)
		}
		// Applied floor replayed/exceeded check
		if err := VerifyVersionFloor("v1.2.0", "v1.3.0", "v1.3.0"); !errors.Is(err, ErrDowngradeOrReplay) {
			t.Errorf("expected ErrDowngradeOrReplay for candidate <= appliedFloor, got %v", err)
		}
		// Valid upgrade exceeding current and floor
		if err := VerifyVersionFloor("v1.2.0", "v1.3.0", "v1.2.0"); err != nil {
			t.Errorf("valid upgrade rejected: %v", err)
		}
	})

	t.Run("AuthenticatedEqualVersionCheck", func(t *testing.T) {
		// Equal version: reports no update, zero error
		hasUpdate, err := CheckUpdateCandidate("v1.2.0", "v1.2.0")
		if err != nil || hasUpdate {
			t.Errorf("equal version should report hasUpdate=false, err=nil; got hasUpdate=%v, err=%v", hasUpdate, err)
		}
		// Older version: reports no update, zero error
		hasUpdate, err = CheckUpdateCandidate("v1.2.0", "v1.1.0")
		if err != nil || hasUpdate {
			t.Errorf("older version should report hasUpdate=false, err=nil; got hasUpdate=%v, err=%v", hasUpdate, err)
		}
		// Newer version: reports update available
		hasUpdate, err = CheckUpdateCandidate("v1.2.0", "v1.3.0")
		if err != nil || !hasUpdate {
			t.Errorf("newer version should report hasUpdate=true, err=nil; got hasUpdate=%v, err=%v", hasUpdate, err)
		}
	})
}

func TestClassifyBuild(t *testing.T) {
	cases := []struct {
		version string
		want    BuildKind
	}{
		{"dev", DevelopmentBuild},
		{"DEV", DevelopmentBuild},
		{"unknown", DevelopmentBuild},
		{"development", DevelopmentBuild},
		{"", DevelopmentBuild},
		{"   ", DevelopmentBuild},
		{"v0.4.53-5-gabc1234", NonCanonicalBuild},
		{"v0.4.50+dirty", NonCanonicalBuild},
		{"v0.4.50-rc.1", NonCanonicalBuild},
		{"nightly", NonCanonicalBuild},
		{"v0.0.1", ReleaseBuild},
		{"v1.2.3", ReleaseBuild},
		{"v10.200.3000", ReleaseBuild},
	}
	for _, tc := range cases {
		if got := ClassifyBuild(tc.version); got != tc.want {
			t.Errorf("ClassifyBuild(%q) = %d, want %d", tc.version, got, tc.want)
		}
	}
}

func TestSelfUpdateDisabledNotice(t *testing.T) {
	notice := SelfUpdateDisabledNotice("v0.4.53-5-gabc1234")
	wants := []string{
		"development build",
		"v0.4.53-5-gabc1234",
		"self-update is disabled",
		"https://github.com/" + DefaultRepo + "/releases",
	}
	for _, want := range wants {
		if !strings.Contains(notice, want) {
			t.Errorf("notice missing %q:\n%s", want, notice)
		}
	}
}

func TestGitDescribeVersionRestrictedToProjectRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	t.Run("outside any repository is empty", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if got := GitDescribeVersion(); got != "" {
			t.Errorf("GitDescribeVersion() outside a repository = %q, want empty", got)
		}
	})

	t.Run("unrelated repository module is ignored", func(t *testing.T) {
		dir := initSyntheticRepo(t, "example.com/other", "v9.9.9")
		t.Chdir(dir)
		if got := GitDescribeVersion(); got != "" {
			t.Errorf("GitDescribeVersion() in an unrelated repository = %q, want empty", got)
		}
	})

	t.Run("project repository is described", func(t *testing.T) {
		dir := initSyntheticRepo(t, projectModulePath, "v1.2.3")
		t.Chdir(dir)
		if got := GitDescribeVersion(); got != "v1.2.3" {
			t.Errorf("GitDescribeVersion() in the project repository = %q, want v1.2.3", got)
		}
	})
}

// initSyntheticRepo builds a one-commit git repository whose go.mod declares
// modulePath and whose HEAD is tagged tag.
func initSyntheticRepo(t *testing.T, modulePath, tag string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.26.1\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "go.mod"},
		{"-c", "user.email=test@example.com", "-c", "user.name=cortex-ia-test", "commit", "-m", "init"},
		{"tag", tag},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}
