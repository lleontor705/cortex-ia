package updater

import (
	"errors"
	"fmt"
	"strings"
)

// VerificationProfile identifies the active release-verification adapter.
type VerificationProfile string

// ProfileStrict is the packaged-trust-bundle profile and the fail-closed default.
const ProfileStrict VerificationProfile = "strict"

// ReleaseVerifier is the single seam through which the download pipeline
// resolves authority, eligibility, required release assets, and manifest
// verification. Keeping those decisions behind one interface lets alternative
// profiles reuse the shared fetch and artifact-download path unchanged.
type ReleaseVerifier interface {
	Name() VerificationProfile
	RequireAuthority() error
	CheckEligibility(current, tag, appliedFloor string) error
	UpdateCandidate(current, tag, appliedFloor string) (bool, error)
	RequiredAssets() []string
	VerifyBundle(rawManifest, rawSig []byte, repo, tag string) (*Manifest, error)
}

// strictVerifier preserves the pre-seam RequireTrust + VerifyVersionFloor +
// CheckUpdateCandidate + VerifyManifest chain verbatim; every method delegates
// so strict behavior and error values stay byte-identical.
type strictVerifier struct{}

func (strictVerifier) Name() VerificationProfile { return ProfileStrict }

func (strictVerifier) RequireAuthority() error { return RequireTrust() }

func (strictVerifier) CheckEligibility(current, tag, appliedFloor string) error {
	if strings.TrimSpace(tag) == "" {
		return errors.New("release cannot be nil and tag name cannot be empty")
	}
	if IsDevOrUnknown(current) {
		return fmt.Errorf("%w: current version is %q", ErrDevUnknownVersion, current)
	}
	return VerifyVersionFloor(current, tag, appliedFloor)
}

func (strictVerifier) UpdateCandidate(current, tag, appliedFloor string) (bool, error) {
	return CheckUpdateCandidate(current, tag)
}

func (strictVerifier) RequiredAssets() []string {
	return []string{"release-manifest.json", "release-manifest.sig"}
}

func (strictVerifier) VerifyBundle(rawManifest, rawSig []byte, repo, tag string) (*Manifest, error) {
	return VerifyManifest(rawManifest, rawSig, repo, tag)
}

// defaultVerifier resolves the active release verifier from packaged trust and
// process consent: a packaged bundle always selects strict; a bundle-less build
// selects checksum only with explicit consent and otherwise stays strict so the
// unchanged ErrNoTrustedKey contract remains the fail-closed default.
func defaultVerifier() ReleaseVerifier {
	return verifierForProfile(ResolveProfile(trustBundlePresent(), checksumConsentActive()))
}

// ActiveProfile reports the verification profile the update pipeline resolves
// for the current packaged-trust and consent state. Receipts and operator
// warnings name the active profile through this accessor.
func ActiveProfile() VerificationProfile {
	return defaultVerifier().Name()
}

// ConsentSource names where checksum consent came from for the single operator
// warning: "flag" for the in-process flag, "env" for the environment variable,
// and "" when no consent is active. The flag wins when both are present because
// it is the explicit per-run request.
func ConsentSource() string {
	if ChecksumConsentGiven() {
		return "flag"
	}
	if consentFromEnvironment() {
		return "env"
	}
	return ""
}
