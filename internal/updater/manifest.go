package updater

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	MaxManifestSize          = 1024 * 1024       // 1 MiB
	MaxSignatureEnvelopeSize = 16 * 1024         // 16 KiB
	MaxArtifactSize          = 128 * 1024 * 1024 // 128 MiB
	ManifestSchemaVersion    = 1
)

var (
	ErrManifestTooLarge     = errors.New("manifest exceeds maximum allowed size (1 MiB)")
	ErrSignatureTooLarge    = errors.New("signature envelope exceeds maximum allowed size (16 KiB)")
	ErrDuplicateJSONKey     = errors.New("duplicate JSON key detected")
	ErrTrailingContent      = errors.New("trailing content detected after JSON payload")
	ErrUnknownSchemaVersion = errors.New("unknown manifest schema version")
	ErrDuplicateArtifact    = errors.New("duplicate artifact identity in manifest")
	ErrMalformedHash        = errors.New("malformed SHA-256 hash in manifest: must be 64 lowercase hex characters")
	ErrInvalidSignature     = errors.New("invalid signature or signature verification failed")
	ErrMalformedEncoding    = errors.New("malformed signature encoding")
	ErrArtifactSizeInvalid  = errors.New("artifact size is zero, negative, or exceeds limit (128 MiB)")
	ErrArtifactNotFound     = errors.New("matching release artifact not found in manifest")
)

type ManifestArtifact struct {
	Name   string `json:"name"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int                `json:"schema_version"`
	Repository    string             `json:"repository"`
	Tag           string             `json:"tag"`
	Artifacts     []ManifestArtifact `json:"artifacts"`
}

type SignatureEnvelope struct {
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

func SignManifest(rawManifest []byte, keyID string, privKey ed25519.PrivateKey) ([]byte, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Ed25519 private key size")
	}
	sig := ed25519.Sign(privKey, rawManifest)
	env := SignatureEnvelope{
		KeyID:     keyID,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	return json.Marshal(env)
}

func VerifyManifest(rawManifest, rawSig []byte, expectedRepo, expectedTag string) (*Manifest, error) {
	if len(rawSig) > MaxSignatureEnvelopeSize {
		return nil, ErrSignatureTooLarge
	}
	if err := validateJSONStrict(rawSig); err != nil {
		return nil, err
	}
	var env SignatureEnvelope
	if err := json.Unmarshal(rawSig, &env); err != nil {
		return nil, fmt.Errorf("malformed signature envelope JSON: %w", err)
	}
	if env.KeyID == "" {
		return nil, errors.New("signature envelope missing key_id")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed: %v", ErrMalformedEncoding, err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrMalformedEncoding, ed25519.SignatureSize, len(sigBytes))
	}

	if len(rawManifest) > MaxManifestSize {
		return nil, ErrManifestTooLarge
	}
	if err := validateJSONStrict(rawManifest); err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("malformed manifest JSON: %w", err)
	}

	if manifest.SchemaVersion != ManifestSchemaVersion {
		return nil, fmt.Errorf("%w: %d (expected %d)", ErrUnknownSchemaVersion, manifest.SchemaVersion, ManifestSchemaVersion)
	}

	if expectedRepo != "" && manifest.Repository != expectedRepo {
		return nil, fmt.Errorf("%w: manifest has %s, expected %s", ErrRepositoryMismatch, manifest.Repository, expectedRepo)
	}
	if expectedTag != "" && manifest.Tag != expectedTag {
		return nil, fmt.Errorf("tag mismatch: manifest has %s, expected %s", manifest.Tag, expectedTag)
	}

	if _, err := ParseCanonicalVersion(manifest.Tag); err != nil {
		return nil, fmt.Errorf("invalid manifest tag: %w", err)
	}

	if len(manifest.Artifacts) == 0 {
		return nil, errors.New("manifest contains no artifacts")
	}

	seenNames := make(map[string]bool)
	seenTargets := make(map[string]bool)
	for _, a := range manifest.Artifacts {
		if a.Name == "" || a.OS == "" || a.Arch == "" {
			return nil, errors.New("artifact missing required name, os, or arch")
		}
		if seenNames[a.Name] {
			return nil, fmt.Errorf("%w: name %q", ErrDuplicateArtifact, a.Name)
		}
		seenNames[a.Name] = true

		target := a.OS + "/" + a.Arch
		if seenTargets[target] {
			return nil, fmt.Errorf("%w: target %q", ErrDuplicateArtifact, target)
		}
		seenTargets[target] = true

		if a.Size <= 0 || a.Size > MaxArtifactSize {
			return nil, fmt.Errorf("%w: %d", ErrArtifactSizeInvalid, a.Size)
		}
		if !isValidSHA256(a.SHA256) {
			return nil, fmt.Errorf("%w: %q", ErrMalformedHash, a.SHA256)
		}
	}

	key, err := LookupTrustedKey(env.KeyID, manifest.Repository, manifest.Tag)
	if err != nil {
		return nil, err
	}

	if !ed25519.Verify(key.PublicKey, rawManifest, sigBytes) {
		return nil, ErrInvalidSignature
	}

	return &manifest, nil
}

func FindManifestArtifact(manifest *Manifest, targetOS, targetArch string) (*ManifestArtifact, error) {
	if manifest == nil {
		return nil, errors.New("nil manifest")
	}
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].OS == targetOS && manifest.Artifacts[i].Arch == targetArch {
			return &manifest.Artifacts[i], nil
		}
	}
	return nil, fmt.Errorf("%w: for %s/%s", ErrArtifactNotFound, targetOS, targetArch)
}

func isValidSHA256(h string) bool {
	if len(h) != 64 {
		return false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func validateJSONStrict(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := parseJSONValueStrict(dec); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err == nil {
		return ErrTrailingContent
	}
	remaining, _ := io.ReadAll(dec.Buffered())
	if len(bytes.TrimSpace(remaining)) > 0 {
		return ErrTrailingContent
	}
	return nil
}

func parseJSONValueStrict(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := make(map[string]bool)
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			k, ok := keyTok.(string)
			if !ok {
				return errors.New("expected string object key")
			}
			if keys[k] {
				return fmt.Errorf("%w: %q", ErrDuplicateJSONKey, k)
			}
			keys[k] = true
			if err := parseJSONValueStrict(dec); err != nil {
				return err
			}
		}
		closing, err := dec.Token()
		if err != nil {
			return err
		}
		if c, ok := closing.(json.Delim); !ok || c != '}' {
			return errors.New("expected '}'")
		}
		return nil
	case '[':
		for dec.More() {
			if err := parseJSONValueStrict(dec); err != nil {
				return err
			}
		}
		closing, err := dec.Token()
		if err != nil {
			return err
		}
		if c, ok := closing.(json.Delim); !ok || c != ']' {
			return errors.New("expected ']'")
		}
		return nil
	}
	return nil
}
