package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrNoTrustedKey is the exact error returned when no trusted root release key is packaged.
//
//nolint:staticcheck // Preserves exact error string required by design contract and tests
var ErrNoTrustedKey = errors.New("Authenticated update unavailable: no trusted release key is packaged")

// MaxTrustedKeys is the hard maximum of compiled public-key records allowed in the trust store.
const MaxTrustedKeys = 2

// Typed errors for trust and rotation validation.
var (
	ErrTooManyKeys        = errors.New("trust policy violation: maximum of two trusted keys allowed")
	ErrKeyNotFound        = errors.New("trusted key not found")
	ErrKeyMalformed       = errors.New("trusted key malformed: must be 32-byte Ed25519 public key")
	ErrRepositoryMismatch = errors.New("repository mismatch for trusted key")
	ErrVersionOutOfRange  = errors.New("release version outside trusted key valid interval")
)

// TrustedKey represents a public verification key packaged for release verification.
type TrustedKey struct {
	ID         string
	PublicKey  []byte
	Repository string
	MinVersion string // inclusive floor, canonical vX.Y.Z
	MaxVersion string // inclusive ceiling, canonical vX.Y.Z (empty means unbounded upper)
}

// ErrTrustBundleInvalid is returned when a packaged build-time trust bundle cannot be
// decoded or violates the trust store invariants.
var ErrTrustBundleInvalid = errors.New("trust bundle invalid")

// productionTrustBundle is the sole ldflags injection target for the packaged release
// trust store: -X .../internal/updater.productionTrustBundle=<base64(JSON []TrustedKey)>.
// It stays unset in local builds, which keeps the updater fail-closed.
var productionTrustBundle string

var (
	// ProductionTrustedKeys is empty unless a valid build-time trust bundle is packaged.
	ProductionTrustedKeys = []TrustedKey{}
	currentTrustedKeys    = ProductionTrustedKeys
	trustMu               sync.Mutex
)

func init() {
	// A malformed bundle must never partially activate trust: decodeTrustBundle rejects it
	// before publication, so the store stays empty and RequireTrust keeps failing closed.
	_ = applyTrustBundle(productionTrustBundle)
}

// applyTrustBundle decodes a packaged bundle and publishes it as the production trust store.
// Rejected bundles leave the active store untouched.
func applyTrustBundle(encoded string) error {
	keys, err := decodeTrustBundle(encoded)
	if err != nil {
		return err
	}
	ProductionTrustedKeys = keys
	trustMu.Lock()
	currentTrustedKeys = keys
	trustMu.Unlock()
	return nil
}

// decodeTrustBundle decodes the base64-encoded JSON array of TrustedKey records injected
// at build time. An unset bundle yields an empty store without error; malformed payloads
// and invariant violations return ErrTrustBundleInvalid.
func decodeTrustBundle(encoded string) ([]TrustedKey, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return []TrustedKey{}, nil
	}

	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed: %v", ErrTrustBundleInvalid, err)
	}

	var keys []TrustedKey
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("%w: JSON decode failed: %v", ErrTrustBundleInvalid, err)
	}

	if err := ValidateTrustStore(keys); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTrustBundleInvalid, err)
	}
	return keys, nil
}

// RequireTrust verifies that trusted release keys are packaged.
// It fails closed immediately if no trusted keys exist.
func RequireTrust() error {
	trustMu.Lock()
	defer trustMu.Unlock()
	if len(currentTrustedKeys) == 0 {
		return ErrNoTrustedKey
	}
	return nil
}

// SetTrustedKeysForTesting allows test fixtures to inject temporary trusted keys
// and returns a cleanup function to restore the previous state.
func SetTrustedKeysForTesting(keys []TrustedKey) func() {
	trustMu.Lock()
	prev := currentTrustedKeys
	currentTrustedKeys = keys
	trustMu.Unlock()
	return func() {
		trustMu.Lock()
		currentTrustedKeys = prev
		trustMu.Unlock()
	}
}

// ValidateTrustStore checks that the key store complies with the max-two-record policy
// and that all packaged keys have valid 32-byte Ed25519 lengths.
func ValidateTrustStore(keys []TrustedKey) error {
	if len(keys) > MaxTrustedKeys {
		return ErrTooManyKeys
	}
	for _, k := range keys {
		if k.ID == "" {
			return errors.New("trusted key ID cannot be empty")
		}
		if len(k.PublicKey) != ed25519.PublicKeySize {
			return ErrKeyMalformed
		}
	}
	return nil
}

// LookupTrustedKey searches the active trust store for keyID, validating repository
// binding, 32-byte key length, and inclusive version interval [MinVersion, MaxVersion].
func LookupTrustedKey(keyID, repo, version string) (*TrustedKey, error) {
	trustMu.Lock()
	keys := make([]TrustedKey, len(currentTrustedKeys))
	copy(keys, currentTrustedKeys)
	trustMu.Unlock()

	if len(keys) == 0 {
		return nil, ErrNoTrustedKey
	}
	if len(keys) > MaxTrustedKeys {
		return nil, ErrTooManyKeys
	}

	for _, k := range keys {
		if k.ID == keyID {
			if len(k.PublicKey) != ed25519.PublicKeySize {
				return nil, ErrKeyMalformed
			}
			if k.Repository != "" && repo != "" && k.Repository != repo {
				return nil, fmt.Errorf("%w: expected %s, got %s", ErrRepositoryMismatch, k.Repository, repo)
			}
			if version != "" {
				if err := checkKeyVersionInterval(k, version); err != nil {
					return nil, err
				}
			}
			return &k, nil
		}
	}
	return nil, fmt.Errorf("%w: key ID %q", ErrKeyNotFound, keyID)
}

func checkKeyVersionInterval(k TrustedKey, verStr string) error {
	ver, err := ParseCanonicalVersion(verStr)
	if err != nil {
		return fmt.Errorf("invalid release version for trust check: %w", err)
	}

	if k.MinVersion != "" {
		minV, err := ParseCanonicalVersion(k.MinVersion)
		if err != nil {
			return fmt.Errorf("invalid MinVersion in trusted key: %w", err)
		}
		if CompareCanonical(ver, minV) < 0 {
			return fmt.Errorf("%w: version %s is below floor %s", ErrVersionOutOfRange, verStr, k.MinVersion)
		}
	}

	if k.MaxVersion != "" {
		maxV, err := ParseCanonicalVersion(k.MaxVersion)
		if err != nil {
			return fmt.Errorf("invalid MaxVersion in trusted key: %w", err)
		}
		if CompareCanonical(ver, maxV) > 0 {
			return fmt.Errorf("%w: version %s exceeds ceiling %s", ErrVersionOutOfRange, verStr, k.MaxVersion)
		}
	}

	return nil
}
