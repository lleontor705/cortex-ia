package updater

import (
	"errors"
	"fmt"
	"strings"
)

// checksumVerifier verifies release metadata without a signature: it trusts the
// pinned-HTTPS transport allowlist, structural manifest validation, and the
// per-artifact SHA-256 digest check. It is opt-in and never activates without
// explicit consent.
type checksumVerifier struct{}

func (checksumVerifier) Name() VerificationProfile { return ProfileChecksum }

// RequireAuthority passes unconditionally: release authenticity and integrity
// are anchored by pinned-HTTPS transport and SHA-256 digests in checksums.txt.
func (checksumVerifier) RequireAuthority() error {
	return nil
}

// CheckEligibility keeps the canonical release-build contract and extends
// eligibility to development builds by comparing the candidate against the
// persisted applied floor instead of a running release identity. Non-canonical
// identifiers (for example git-describe strings) stay rejected.
func (checksumVerifier) CheckEligibility(current, tag, appliedFloor string) error {
	if strings.TrimSpace(tag) == "" {
		return errors.New("release cannot be nil and tag name cannot be empty")
	}
	if ClassifyBuild(current) != DevelopmentBuild {
		return VerifyVersionFloor(current, tag, appliedFloor)
	}
	delta, hasFloor, err := checksumFloorDelta(tag, appliedFloor)
	if err != nil {
		return err
	}
	if hasFloor && delta <= 0 {
		return fmt.Errorf("%w: candidate %s <= applied floor %s", ErrDowngradeOrReplay, tag, appliedFloor)
	}
	return nil
}

// UpdateCandidate reports whether the candidate should be offered.
// Release builds compare against the running version; dev/unknown builds return false
// so they do not attempt automatic updates without an installed release.
func (checksumVerifier) UpdateCandidate(current, tag, appliedFloor string) (bool, error) {
	return CheckUpdateCandidate(current, tag)
}

// checksumFloorDelta parses the candidate and, when a usable applied floor is
// recorded, returns its canonical ordering against that floor.
func checksumFloorDelta(tag, appliedFloor string) (delta int, hasFloor bool, err error) {
	cand, err := ParseCanonicalVersion(tag)
	if err != nil {
		return 0, false, fmt.Errorf("invalid candidate version: %w", err)
	}
	floor := strings.TrimSpace(appliedFloor)
	if floor == "" || IsDevOrUnknown(floor) {
		return 0, false, nil
	}
	floorVer, err := ParseCanonicalVersion(floor)
	if err != nil {
		return 0, false, fmt.Errorf("invalid applied floor: %w", err)
	}
	return CompareCanonical(cand, floorVer), true, nil
}

// RequiredAssets names the standard GoReleaser checksums asset.
func (checksumVerifier) RequiredAssets() []string {
	return []string{ChecksumsFileName}
}
