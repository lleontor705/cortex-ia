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

// trustBundlePresent reports whether the active trust store holds any packaged
// key, mirroring RequireTrust's source of truth.
func trustBundlePresent() bool {
	trustMu.Lock()
	defer trustMu.Unlock()
	return len(currentTrustedKeys) > 0
}

// ResolveProfile selects the verification profile for the given trust-bundle
// availability and consent. A packaged bundle always wins (strict); a bundle-less
// build selects checksum only with consent, and otherwise falls back to strict so
// the caller surfaces the unchanged ErrNoTrustedKey fail-closed contract.
// ProfileInsecure is never returned.
func ResolveProfile(bundlePresent, consent bool) VerificationProfile {
	if bundlePresent {
		return ProfileStrict
	}
	if consent {
		return ProfileChecksum
	}
	return ProfileStrict
}

// verifierForProfile maps a resolved profile to its adapter. Unknown and
// unselectable values fall back to strict, the fail-closed default.
func verifierForProfile(profile VerificationProfile) ReleaseVerifier {
	switch profile {
	case ProfileChecksum:
		return checksumVerifier{}
	default:
		return strictVerifier{}
	}
}

// RequireProfileAuthority resolves the active profile from packaged trust and
// consent, then enforces its authority gate. Without a bundle and without
// consent it returns ErrNoTrustedKey verbatim before any network call.
func RequireProfileAuthority() error {
	return verifierForProfile(ResolveProfile(trustBundlePresent(), checksumConsentActive())).RequireAuthority()
}
