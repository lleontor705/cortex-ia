package updater

// ReleaseVerifier is the seam through which the download pipeline checks release
// eligibility and whether a candidate should be offered.
type ReleaseVerifier interface {
	CheckEligibility(current, tag, appliedFloor string) error
	UpdateCandidate(current, tag, appliedFloor string) (bool, error)
}

// defaultVerifier returns the release verifier. Integrity is the SHA-256 digest
// from checksums.txt; see checksumVerifier for what that does and does not prove.
func defaultVerifier() ReleaseVerifier {
	return checksumVerifier{}
}
