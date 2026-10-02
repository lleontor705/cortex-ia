package updater

import (
	"os"
	"strings"
	"sync"
)

// ProfileChecksum verifies release manifests by structure and per-artifact
// SHA-256 only. It skips Ed25519 verification and is selectable solely on a
// bundle-less build with explicit operator consent.
const ProfileChecksum VerificationProfile = "checksum"

// ProfileInsecure is a reserved identifier kept for receipt completeness. No
// flag, variable, or file may resolve to it: it exists only in the type system.
const ProfileInsecure VerificationProfile = "insecure"

// consentEnvVar is the environment surface mirroring --allow-checksum-updates.
const consentEnvVar = "CORTEX_IA_ALLOW_CHECKSUM_UPDATES"

var (
	checksumConsentMu sync.Mutex
	checksumConsent   bool
)

// SetChecksumConsent records process-scoped consent for signature-less checksum
// verification. Consent is opt-in per run; clearing it restores fail-closed.
func SetChecksumConsent(granted bool) {
	checksumConsentMu.Lock()
	checksumConsent = granted
	checksumConsentMu.Unlock()
}

// ChecksumConsentGiven reports whether consent was granted in this process.
func ChecksumConsentGiven() bool {
	checksumConsentMu.Lock()
	defer checksumConsentMu.Unlock()
	return checksumConsent
}

// consentFromEnvironment parses CORTEX_IA_ALLOW_CHECKSUM_UPDATES. Only "1" and
// "true" (case-insensitive, trimmed) grant consent; any other value, including
// unset, is treated as absent.
func consentFromEnvironment() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(consentEnvVar))) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// checksumConsentActive resolves effective consent from either the flag-fed
// store or the environment variable.
func checksumConsentActive() bool {
	return ChecksumConsentGiven() || consentFromEnvironment()
}

// trustBundlePresent reports false as trust bundles are retired in favor of checksums.txt.
func trustBundlePresent() bool {
	return false
}

// ResolveProfile selects the verification profile. Standard releases verify
// integrity via GoReleaser checksums.txt over HTTPS.
func ResolveProfile(bundlePresent, consent bool) VerificationProfile {
	return ProfileChecksum
}

// verifierForProfile maps a resolved profile to its adapter.
func verifierForProfile(profile VerificationProfile) ReleaseVerifier {
	return checksumVerifier{}
}

// RequireProfileAuthority returns nil as authority is established through TLS transport
// and SHA-256 digest validation.
func RequireProfileAuthority() error {
	return nil
}
