package updater

import (
	"errors"
	"fmt"
	"strings"
)

// checksumVerifier checks release eligibility and per-artifact integrity.
//
// checksums.txt is published in the same GitHub release as the archive it lists.
// A matching digest therefore proves the downloaded bytes are the ones that
// release published, and that the transfer was not corrupted or altered in
// flight. It does not prove who published the release. Authenticity is
// checkable by the user with `gh attestation verify <archive> --repo
// lleontor705/cortex-ia`, against the build provenance attested by the release
// workflow (see docs/reference/updater-verification-profiles.md).
type checksumVerifier struct{}

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
