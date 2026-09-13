package updater

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestUpdateManifest(t *testing.T) {
	repo := "lleontor705/cortex-ia"
	tag := "v1.5.0"
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}

	keyID := "key-manifest-test"
	cleanup := SetTrustedKeysForTesting([]TrustedKey{
		{
			ID:         keyID,
			PublicKey:  pub,
			Repository: repo,
			MinVersion: "v1.0.0",
			MaxVersion: "v2.0.0",
		},
	})
	defer cleanup()

	baseManifest := Manifest{
		SchemaVersion: 1,
		Repository:    repo,
		Tag:           tag,
		Artifacts: []ManifestArtifact{
			{
				Name:   "cortex-ia_1.5.0_windows_amd64.zip",
				OS:     "windows",
				Arch:   "amd64",
				Size:   1024,
				SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
			{
				Name:   "cortex-ia_1.5.0_linux_amd64.tar.gz",
				OS:     "linux",
				Arch:   "amd64",
				Size:   2048,
				SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			},
		},
	}
	rawMan, _ := json.Marshal(baseManifest)
	rawSig, _ := SignManifest(rawMan, keyID, priv)

	t.Run("RuntimeSignedProducer", func(t *testing.T) {
		m, err := VerifyManifest(rawMan, rawSig, repo, tag)
		if err != nil {
			t.Fatalf("expected valid manifest verification, got: %v", err)
		}
		art, err := FindManifestArtifact(m, "windows", "amd64")
		if err != nil || art.Name != "cortex-ia_1.5.0_windows_amd64.zip" {
			t.Fatalf("expected exact artifact find, got art=%v, err=%v", art, err)
		}
		if _, err := FindManifestArtifact(m, "WINDOWS", "AMD64"); err == nil {
			t.Fatal("expected case-fold lookup to fail")
		}
	})

	t.Run("SizeLimits", func(t *testing.T) {
		bigMan := make([]byte, MaxManifestSize+1)
		if _, err := VerifyManifest(bigMan, rawSig, repo, tag); !errors.Is(err, ErrManifestTooLarge) {
			t.Fatalf("expected ErrManifestTooLarge for %d bytes, got %v", len(bigMan), err)
		}
		bigSig := make([]byte, MaxSignatureEnvelopeSize+1)
		if _, err := VerifyManifest(rawMan, bigSig, repo, tag); !errors.Is(err, ErrSignatureTooLarge) {
			t.Fatalf("expected ErrSignatureTooLarge for %d bytes, got %v", len(bigSig), err)
		}
	})

	t.Run("UnknownSchema", func(t *testing.T) {
		badSchema := baseManifest
		badSchema.SchemaVersion = 2
		b, _ := json.Marshal(badSchema)
		s, _ := SignManifest(b, keyID, priv)
		if _, err := VerifyManifest(b, s, repo, tag); !errors.Is(err, ErrUnknownSchemaVersion) {
			t.Fatalf("expected ErrUnknownSchemaVersion, got %v", err)
		}
	})

	t.Run("DuplicateJSONKeys", func(t *testing.T) {
		dupMan := []byte(`{"schema_version":1,"schema_version":1,"repository":"` + repo + `","tag":"` + tag + `","artifacts":[]}`)
		dupSig, _ := SignManifest(dupMan, keyID, priv)
		if _, err := VerifyManifest(dupMan, dupSig, repo, tag); !errors.Is(err, ErrDuplicateJSONKey) {
			t.Fatalf("expected ErrDuplicateJSONKey on manifest, got %v", err)
		}
		dupSigEnv := []byte(`{"key_id":"` + keyID + `","key_id":"` + keyID + `","signature":"AAAA"}`)
		if _, err := VerifyManifest(rawMan, dupSigEnv, repo, tag); !errors.Is(err, ErrDuplicateJSONKey) {
			t.Fatalf("expected ErrDuplicateJSONKey on signature envelope, got %v", err)
		}
	})

	t.Run("TrailingContent", func(t *testing.T) {
		trailMan := append(rawMan, []byte(" extra trailing data")...)
		trailSig, _ := SignManifest(trailMan, keyID, priv)
		if _, err := VerifyManifest(trailMan, trailSig, repo, tag); !errors.Is(err, ErrTrailingContent) {
			t.Fatalf("expected ErrTrailingContent on manifest, got %v", err)
		}
	})

	t.Run("DuplicateArtifactIdentities", func(t *testing.T) {
		dupArts := baseManifest
		dupArts.Artifacts = append(dupArts.Artifacts, baseManifest.Artifacts[0])
		b, _ := json.Marshal(dupArts)
		s, _ := SignManifest(b, keyID, priv)
		if _, err := VerifyManifest(b, s, repo, tag); !errors.Is(err, ErrDuplicateArtifact) {
			t.Fatalf("expected ErrDuplicateArtifact, got %v", err)
		}
	})

	t.Run("MalformedHashAndEncoding", func(t *testing.T) {
		badHash := baseManifest
		badHash.Artifacts[0].SHA256 = "UPPERCASE0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
		b, _ := json.Marshal(badHash)
		s, _ := SignManifest(b, keyID, priv)
		if _, err := VerifyManifest(b, s, repo, tag); !errors.Is(err, ErrMalformedHash) {
			t.Fatalf("expected ErrMalformedHash, got %v", err)
		}

		badSig := []byte(`{"key_id":"` + keyID + `","signature":"not-base64!?"}`)
		if _, err := VerifyManifest(rawMan, badSig, repo, tag); !errors.Is(err, ErrMalformedEncoding) {
			t.Fatalf("expected ErrMalformedEncoding, got %v", err)
		}
	})

	t.Run("WrongRepoTagOrTamperedBytes", func(t *testing.T) {
		if _, err := VerifyManifest(rawMan, rawSig, "other/repo", tag); !errors.Is(err, ErrRepositoryMismatch) {
			t.Fatalf("expected ErrRepositoryMismatch, got %v", err)
		}
		if _, err := VerifyManifest(rawMan, rawSig, repo, "v1.6.0"); err == nil || !strings.Contains(err.Error(), "tag mismatch") {
			t.Fatalf("expected tag mismatch error, got %v", err)
		}

		tampered := bytes.Replace(rawMan, []byte("1024"), []byte("9999"), 1)
		if _, err := VerifyManifest(tampered, rawSig, repo, tag); !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("expected ErrInvalidSignature on tampered bytes, got %v", err)
		}
	})
}
