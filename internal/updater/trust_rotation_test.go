package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func genTestKey(t *testing.T, id, repo, minV, maxV string) TrustedKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	return TrustedKey{
		ID:         id,
		PublicKey:  pub,
		Repository: repo,
		MinVersion: minV,
		MaxVersion: maxV,
	}
}

func TestUpdateRotation(t *testing.T) {
	repo := "lleontor705/cortex-ia"

	t.Run("ProductionRootsStayEmpty", func(t *testing.T) {
		if len(ProductionTrustedKeys) != 0 {
			t.Fatalf("ProductionTrustedKeys must be empty, got %d", len(ProductionTrustedKeys))
		}
		if err := RequireTrust(); !errors.Is(err, ErrNoTrustedKey) {
			t.Fatalf("expected ErrNoTrustedKey, got %v", err)
		}
		_, err := LookupTrustedKey("any-key", repo, "v1.0.0")
		if !errors.Is(err, ErrNoTrustedKey) {
			t.Fatalf("expected ErrNoTrustedKey for lookup in production trust, got %v", err)
		}
	})

	t.Run("PredecessorOnly", func(t *testing.T) {
		k1 := genTestKey(t, "key-pred", repo, "v1.0.0", "v1.5.0")
		cleanup := SetTrustedKeysForTesting([]TrustedKey{k1})
		defer cleanup()

		if key, err := LookupTrustedKey("key-pred", repo, "v1.2.0"); err != nil || key == nil {
			t.Fatalf("expected key-pred lookup to succeed within interval, got err: %v", err)
		}
		if _, err := LookupTrustedKey("key-pred", repo, "v0.9.0"); !errors.Is(err, ErrVersionOutOfRange) {
			t.Fatalf("expected ErrVersionOutOfRange for version below floor, got %v", err)
		}
		if _, err := LookupTrustedKey("key-pred", repo, "v1.6.0"); !errors.Is(err, ErrVersionOutOfRange) {
			t.Fatalf("expected ErrVersionOutOfRange for version above ceiling, got %v", err)
		}
	})

	t.Run("BoundedOverlapAndSuccessor", func(t *testing.T) {
		kPred := genTestKey(t, "key-2026-01", repo, "v1.0.0", "v2.0.0")
		kSucc := genTestKey(t, "key-2026-02", repo, "v2.0.0", "v3.0.0")
		cleanup := SetTrustedKeysForTesting([]TrustedKey{kPred, kSucc})
		defer cleanup()

		if _, err := LookupTrustedKey("key-2026-01", repo, "v2.0.0"); err != nil {
			t.Fatalf("predecessor should be valid at boundary overlap v2.0.0, got: %v", err)
		}
		if _, err := LookupTrustedKey("key-2026-02", repo, "v2.0.0"); err != nil {
			t.Fatalf("successor should be valid at boundary overlap v2.0.0, got: %v", err)
		}
		if _, err := LookupTrustedKey("key-2026-02", repo, "v2.5.0"); err != nil {
			t.Fatalf("successor should be valid at v2.5.0, got: %v", err)
		}
		if _, err := LookupTrustedKey("key-2026-01", repo, "v2.5.0"); !errors.Is(err, ErrVersionOutOfRange) {
			t.Fatalf("predecessor must be retired past v2.0.0, got: %v", err)
		}
	})

	t.Run("UnknownAndRetiredKeys", func(t *testing.T) {
		k1 := genTestKey(t, "key-known", repo, "v1.0.0", "v1.9.9")
		cleanup := SetTrustedKeysForTesting([]TrustedKey{k1})
		defer cleanup()

		if _, err := LookupTrustedKey("unknown-key", repo, "v1.0.0"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound for unknown key, got %v", err)
		}
	})

	t.Run("MalformedKeyAndLengthValidation", func(t *testing.T) {
		malformed := TrustedKey{
			ID:         "short-key",
			PublicKey:  []byte("short-bytes"),
			Repository: repo,
			MinVersion: "v1.0.0",
		}
		cleanup := SetTrustedKeysForTesting([]TrustedKey{malformed})
		defer cleanup()

		if _, err := LookupTrustedKey("short-key", repo, "v1.0.0"); !errors.Is(err, ErrKeyMalformed) {
			t.Fatalf("expected ErrKeyMalformed for short key, got %v", err)
		}
		if err := ValidateTrustStore([]TrustedKey{malformed}); !errors.Is(err, ErrKeyMalformed) {
			t.Fatalf("expected ValidateTrustStore to report ErrKeyMalformed, got %v", err)
		}
	})

	t.Run("RepositoryMismatch", func(t *testing.T) {
		k1 := genTestKey(t, "repo-key", repo, "v1.0.0", "v2.0.0")
		cleanup := SetTrustedKeysForTesting([]TrustedKey{k1})
		defer cleanup()

		if _, err := LookupTrustedKey("repo-key", "evil-fork/cortex-ia", "v1.5.0"); !errors.Is(err, ErrRepositoryMismatch) {
			t.Fatalf("expected ErrRepositoryMismatch, got %v", err)
		}
	})

	t.Run("PolicyMaxTwoKeys", func(t *testing.T) {
		k1 := genTestKey(t, "k1", repo, "v1.0.0", "v2.0.0")
		k2 := genTestKey(t, "k2", repo, "v2.0.0", "v3.0.0")
		k3 := genTestKey(t, "k3", repo, "v3.0.0", "v4.0.0")

		if err := ValidateTrustStore([]TrustedKey{k1, k2, k3}); !errors.Is(err, ErrTooManyKeys) {
			t.Fatalf("expected ErrTooManyKeys when 3 keys configured, got %v", err)
		}
	})
}
