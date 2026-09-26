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

// RequireAuthority passes only while explicit checksum consent is active; the
// default stays the strict fail-closed error so no caller can silently reach a
// signature-less path.
func (checksumVerifier) RequireAuthority() error {
	if !checksumConsentActive() {
		return ErrNoTrustedKey
	}
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

// UpdateCandidate reports whether the candidate should be offered. Release
// builds compare against the running version; development builds compare against
// the applied floor and report a candidate when none is recorded yet.
func (checksumVerifier) UpdateCandidate(current, tag, appliedFloor string) (bool, error) {
	if ClassifyBuild(current) != DevelopmentBuild {
		return CheckUpdateCandidate(current, tag)
	}
	delta, hasFloor, err := checksumFloorDelta(tag, appliedFloor)
	if err != nil {
		return false, err
	}
	return !hasFloor || delta > 0, nil
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

// RequiredAssets names only the unsigned manifest: no signature envelope is
// fetched or required under this profile.
func (checksumVerifier) RequiredAssets() []string {
	return []string{"release-manifest.json"}
}

// VerifyBundle structurally validates the manifest through the single shared
// validator. Signature bytes are intentionally ignored, so a nil envelope is
// not an error.
func (checksumVerifier) VerifyBundle(rawManifest, _ []byte, repo, tag string) (*Manifest, error) {
	return validateManifestShape(rawManifest, repo, tag)
}
