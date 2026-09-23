package updater

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const injectTestRepo = "lleontor705/cortex-ia"

const injectTestTag = "v0.5.0"

func encodeTrustBundle(t *testing.T, keys []TrustedKey) string {
	t.Helper()
	raw, err := json.Marshal(keys)
	if err != nil {
		t.Fatalf("marshal trust bundle: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// activateTrustBundle mirrors the production init path: decode first, and only publish the
// decoded store when decoding succeeded.
func activateTrustBundle(t *testing.T, encoded string) error {
	t.Helper()
	keys, err := decodeTrustBundle(encoded)
	if err != nil {
		return err
	}
	t.Cleanup(SetTrustedKeysForTesting(keys))
	return nil
}

func TestTrustBundleUnsetKeepsFailClosed(t *testing.T) {
	for _, encoded := range []string{"", "   ", "\n\t"} {
		keys, err := decodeTrustBundle(encoded)
		if err != nil {
			t.Fatalf("decodeTrustBundle(%q) must not error for an unset bundle, got %v", encoded, err)
		}
		if len(keys) != 0 {
			t.Fatalf("decodeTrustBundle(%q) must yield an empty store, got %d keys", encoded, len(keys))
		}
	}

	if err := activateTrustBundle(t, ""); err != nil {
		t.Fatalf("activation of an unset bundle must succeed with an empty store: %v", err)
	}
	if err := RequireTrust(); !errors.Is(err, ErrNoTrustedKey) {
		t.Fatalf("expected ErrNoTrustedKey with an unset bundle, got %v", err)
	}
	const contractMessage = "Authenticated update unavailable: no trusted release key is packaged"
	if got := ErrNoTrustedKey.Error(); got != contractMessage {
		t.Fatalf("trust contract message changed: got %q", got)
	}
	if _, err := LookupTrustedKey("any-key", injectTestRepo, injectTestTag); !errors.Is(err, ErrNoTrustedKey) {
		t.Fatalf("expected ErrNoTrustedKey from lookup with an unset bundle, got %v", err)
	}
}

func TestTrustBundleValidKeyEnablesVerification(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	const keyID = "release-2026-01"

	bundle := encodeTrustBundle(t, []TrustedKey{{
		ID:         keyID,
		PublicKey:  pub,
		Repository: injectTestRepo,
		MinVersion: "v0.1.0",
		MaxVersion: "v1.0.0",
	}})
	if err := activateTrustBundle(t, bundle); err != nil {
		t.Fatalf("valid bundle must decode: %v", err)
	}
	if err := RequireTrust(); err != nil {
		t.Fatalf("RequireTrust must succeed with a packaged key, got %v", err)
	}

	key, err := LookupTrustedKey(keyID, injectTestRepo, injectTestTag)
	if err != nil {
		t.Fatalf("packaged key must resolve: %v", err)
	}
	if key.Repository != injectTestRepo || key.MinVersion != "v0.1.0" || key.MaxVersion != "v1.0.0" {
		t.Fatalf("repository binding or version interval lost in decode: %+v", *key)
	}
	if !bytes.Equal(key.PublicKey, pub) {
		t.Fatal("decoded public key does not match the packaged key")
	}

	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Repository:    injectTestRepo,
		Tag:           injectTestTag,
		Artifacts: []ManifestArtifact{{
			Name: "cortex-ia_0.5.0_linux_amd64.tar.gz", OS: "linux", Arch: "amd64", Size: 128, SHA256: strings.Repeat("a3", 32),
		}},
	}
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	rawSig, err := SignManifest(rawManifest, keyID, priv)
	if err != nil {
		t.Fatalf("SignManifest failed: %v", err)
	}
	if _, err := VerifyManifest(rawManifest, rawSig, injectTestRepo, injectTestTag); err != nil {
		t.Fatalf("packaged key must enable signature verification, got %v", err)
	}

	if _, err := LookupTrustedKey(keyID, "evil-fork/cortex-ia", injectTestTag); !errors.Is(err, ErrRepositoryMismatch) {
		t.Fatalf("expected ErrRepositoryMismatch for a foreign repository, got %v", err)
	}
	if _, err := LookupTrustedKey(keyID, injectTestRepo, "v1.0.1"); !errors.Is(err, ErrVersionOutOfRange) {
		t.Fatalf("expected ErrVersionOutOfRange above the packaged ceiling, got %v", err)
	}
}

func TestTrustBundleRotationPairDecodesAndBounds(t *testing.T) {
	pred := genTestKey(t, "key-2026-01", injectTestRepo, "v1.0.0", "v2.0.0")
	succ := genTestKey(t, "key-2026-02", injectTestRepo, "v2.0.0", "")

	if err := activateTrustBundle(t, encodeTrustBundle(t, []TrustedKey{pred, succ})); err != nil {
		t.Fatalf("two-record rotation bundle must decode: %v", err)
	}
	if _, err := LookupTrustedKey("key-2026-02", injectTestRepo, "v9.9.9"); err != nil {
		t.Fatalf("empty MaxVersion must remain unbounded, got %v", err)
	}
	if _, err := LookupTrustedKey("key-2026-01", injectTestRepo, "v2.5.0"); !errors.Is(err, ErrVersionOutOfRange) {
		t.Fatalf("retired predecessor must be rejected past its ceiling, got %v", err)
	}
}

func TestTrustBundleInjectionPublishesProductionStore(t *testing.T) {
	original := productionTrustBundle
	t.Cleanup(func() {
		if err := applyTrustBundle(original); err != nil {
			t.Errorf("failed to restore the production trust bundle: %v", err)
		}
	})

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	encoded := encodeTrustBundle(t, []TrustedKey{{
		ID:         "release-2026-02",
		PublicKey:  pub,
		Repository: injectTestRepo,
		MinVersion: "v0.1.0",
	}})
	if err := applyTrustBundle(encoded); err != nil {
		t.Fatalf("injection must publish a valid bundle: %v", err)
	}
	if err := RequireTrust(); err != nil {
		t.Fatalf("injected key must satisfy RequireTrust, got %v", err)
	}
	if len(ProductionTrustedKeys) != 1 || ProductionTrustedKeys[0].ID != "release-2026-02" {
		t.Fatalf("injected record missing from the production store: %+v", ProductionTrustedKeys)
	}
	if _, err := LookupTrustedKey("release-2026-02", injectTestRepo, "v0.4.0"); err != nil {
		t.Fatalf("injected key must resolve for the active store, got %v", err)
	}

	if err := applyTrustBundle("!!!not-base64!!!"); !errors.Is(err, ErrTrustBundleInvalid) {
		t.Fatalf("expected ErrTrustBundleInvalid, got %v", err)
	}
	if len(ProductionTrustedKeys) != 1 {
		t.Fatalf("a rejected bundle must not mutate the active store, got %d keys", len(ProductionTrustedKeys))
	}
}

// TestTrustBundleInjectedBuildActivatesProductionStore is the end-to-end ldflags probe: it
// only has work to do when the test binary was built with -X productionTrustBundle, which is
// exactly the wiring a release build relies on.
func TestTrustBundleInjectedBuildActivatesProductionStore(t *testing.T) {
	if strings.TrimSpace(productionTrustBundle) == "" {
		t.Skip("no ldflags trust bundle injected into this build")
	}
	if err := RequireTrust(); err != nil {
		t.Fatalf("an injected bundle must satisfy RequireTrust, got %v", err)
	}
	if len(ProductionTrustedKeys) == 0 {
		t.Fatal("an injected bundle must populate ProductionTrustedKeys")
	}
	for _, key := range ProductionTrustedKeys {
		if len(key.PublicKey) != ed25519.PublicKeySize {
			t.Fatalf("injected key %q has %d key bytes", key.ID, len(key.PublicKey))
		}
	}
}

func TestTrustBundleMalformedFailsClosed(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	valid := TrustedKey{ID: "k1", PublicKey: pub, Repository: injectTestRepo, MinVersion: "v1.0.0"}
	truncated := valid
	truncated.PublicKey = pub[:ed25519.PublicKeySize-1]
	missingID := valid
	missingID.ID = ""

	cases := []struct {
		name   string
		bundle string
	}{
		{"not base64", "!!!not-base64!!!"},
		{"raw JSON without base64 envelope", `[{"ID":"k1"}]`},
		{"base64 of non-JSON", base64.StdEncoding.EncodeToString([]byte("plain text"))},
		{"base64 of JSON object", base64.StdEncoding.EncodeToString([]byte(`{"ID":"k1"}`))},
		{"truncated public key", encodeTrustBundle(t, []TrustedKey{truncated})},
		{"empty key ID", encodeTrustBundle(t, []TrustedKey{missingID})},
		{"three records exceed MaxTrustedKeys", encodeTrustBundle(t, []TrustedKey{
			valid,
			{ID: "k2", PublicKey: pub, Repository: injectTestRepo},
			{ID: "k3", PublicKey: pub, Repository: injectTestRepo},
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trustedBefore := RequireTrust() == nil
			countBefore := len(ProductionTrustedKeys)

			err := activateTrustBundle(t, tc.bundle)
			if err == nil {
				t.Fatal("malformed bundle must be rejected")
			}
			if !errors.Is(err, ErrTrustBundleInvalid) {
				t.Fatalf("expected ErrTrustBundleInvalid, got %v", err)
			}
			if (RequireTrust() == nil) != trustedBefore {
				t.Fatalf("a rejected bundle must not change trust readiness (before=%v)", trustedBefore)
			}
			if len(ProductionTrustedKeys) != countBefore {
				t.Fatalf("a rejected bundle must not mutate the store: %d -> %d keys", countBefore, len(ProductionTrustedKeys))
			}
			if _, err := LookupTrustedKey("k1", injectTestRepo, "v1.5.0"); err == nil {
				t.Fatal("a rejected bundle must not publish any of its records")
			}
		})
	}
}
