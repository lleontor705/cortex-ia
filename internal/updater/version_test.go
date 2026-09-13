package updater

import (
	"errors"
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
