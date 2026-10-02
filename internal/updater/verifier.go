package updater

// VerificationProfile identifies the active release-verification adapter.
type VerificationProfile string

// ProfileStrict is the packaged-trust-bundle profile and the fail-closed default.
const ProfileStrict VerificationProfile = "strict"

// ReleaseVerifier is the single seam through which the download pipeline
// resolves authority, eligibility, and required release assets.
type ReleaseVerifier interface {
	Name() VerificationProfile
	RequireAuthority() error
	CheckEligibility(current, tag, appliedFloor string) error
	UpdateCandidate(current, tag, appliedFloor string) (bool, error)
	RequiredAssets() []string
}

// defaultVerifier returns the active release verifier. Standard releases verify
// integrity via GoReleaser checksums.txt over HTTPS without requiring signed manifests.
func defaultVerifier() ReleaseVerifier {
	return checksumVerifier{}
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
