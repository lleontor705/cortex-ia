package updater

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const profileTestRepo = "lleontor705/cortex-ia"

func profileTestArtifacts() []ManifestArtifact {
	return []ManifestArtifact{{
		Name:   "cortex-ia_1.2.3_linux_amd64.tar.gz",
		OS:     "linux",
		Arch:   "amd64",
		Size:   128,
		SHA256: strings.Repeat("ab", 32),
	}}
}

func profileTestManifest(t *testing.T, repo, tag string) []byte {
	t.Helper()
	raw, err := json.Marshal(Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Repository:    repo,
		Tag:           tag,
		Artifacts:     profileTestArtifacts(),
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return raw
}

func TestResolveProfileSelection(t *testing.T) {
	cases := []struct {
		bundlePresent bool
		consent       bool
		want          VerificationProfile
	}{
		{true, true, ProfileStrict},
		{true, false, ProfileStrict},
		{false, true, ProfileChecksum},
		{false, false, ProfileStrict},
	}
	for _, tc := range cases {
		got := ResolveProfile(tc.bundlePresent, tc.consent)
		if got != tc.want {
			t.Fatalf("ResolveProfile(%v, %v) = %q, want %q", tc.bundlePresent, tc.consent, got, tc.want)
		}
		if got == ProfileInsecure {
			t.Fatalf("ResolveProfile(%v, %v) yielded the reserved insecure profile", tc.bundlePresent, tc.consent)
		}
	}
}

func TestConsentFromEnvironment(t *testing.T) {
	cases := map[string]bool{
		"1": true, "true": true, "TRUE": true, " true ": true,
		"0": false, "false": false, "yes": false, "": false,
	}
	for value, want := range cases {
		t.Setenv(consentEnvVar, value)
		if got := consentFromEnvironment(); got != want {
			t.Fatalf("consentFromEnvironment() with %q = %v, want %v", value, got, want)
		}
	}
}

func TestChecksumVerifierAuthorityGate(t *testing.T) {
	t.Setenv(consentEnvVar, "")
	SetChecksumConsent(false)
	t.Cleanup(func() { SetChecksumConsent(false) })

	if err := (checksumVerifier{}).RequireAuthority(); !errors.Is(err, ErrNoTrustedKey) {
		t.Fatalf("expected ErrNoTrustedKey without consent, got %v", err)
	}
	SetChecksumConsent(true)
	if err := (checksumVerifier{}).RequireAuthority(); err != nil {
		t.Fatalf("expected authority with stored consent, got %v", err)
	}
	SetChecksumConsent(false)
	t.Setenv(consentEnvVar, "true")
	if err := (checksumVerifier{}).RequireAuthority(); err != nil {
		t.Fatalf("expected authority with environment consent, got %v", err)
	}
}

func TestRequireProfileAuthorityGate(t *testing.T) {
	t.Setenv(consentEnvVar, "")
	SetChecksumConsent(false)
	t.Cleanup(func() { SetChecksumConsent(false) })

	empty := SetTrustedKeysForTesting([]TrustedKey{})
	defer empty()

	err := RequireProfileAuthority()
	if !errors.Is(err, ErrNoTrustedKey) {
		t.Fatalf("expected ErrNoTrustedKey with an empty bundle, got %v", err)
	}
	const contractMessage = "Authenticated update unavailable: no trusted release key is packaged"
	if err == nil || err.Error() != contractMessage {
		t.Fatalf("trust contract message changed: got %q", err)
	}

	SetChecksumConsent(true)
	if err := RequireProfileAuthority(); err != nil {
		t.Fatalf("consent must authorize checksum resolution, got %v", err)
	}
	SetChecksumConsent(false)

	restore := SetTrustedKeysForTesting([]TrustedKey{genTestKey(t, "k-profile", profileTestRepo, "v0.1.0", "")})
	defer restore()
	if err := RequireProfileAuthority(); err != nil {
		t.Fatalf("packaged bundle must authorize strict resolution, got %v", err)
	}
}

func TestChecksumVerifierVerifyBundle(t *testing.T) {
	const tag = "v1.2.3"
	cv := checksumVerifier{}
	valid := profileTestManifest(t, profileTestRepo, tag)

	man, err := cv.VerifyBundle(valid, nil, profileTestRepo, tag)
	if err != nil {
		t.Fatalf("valid manifest with nil signature must verify, got %v", err)
	}
	if man == nil || man.Tag != tag || man.Repository != profileTestRepo {
		t.Fatalf("unexpected manifest round-trip: %+v", man)
	}
	if _, err := cv.VerifyBundle(valid, []byte("ignored-envelope"), profileTestRepo, tag); err != nil {
		t.Fatalf("signature bytes must be ignored, got %v", err)
	}
	if _, err := cv.VerifyBundle(profileTestManifest(t, "evil-fork/cortex-ia", tag), nil, profileTestRepo, tag); !errors.Is(err, ErrRepositoryMismatch) {
		t.Fatalf("expected ErrRepositoryMismatch, got %v", err)
	}

	badSchema, _ := json.Marshal(Manifest{SchemaVersion: ManifestSchemaVersion + 1, Repository: profileTestRepo, Tag: tag, Artifacts: profileTestArtifacts()})
	if _, err := cv.VerifyBundle(badSchema, nil, profileTestRepo, tag); !errors.Is(err, ErrUnknownSchemaVersion) {
		t.Fatalf("expected ErrUnknownSchemaVersion, got %v", err)
	}

	badHash, _ := json.Marshal(Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Repository:    profileTestRepo,
		Tag:           tag,
		Artifacts:     []ManifestArtifact{{Name: "a.tar.gz", OS: "linux", Arch: "amd64", Size: 128, SHA256: "not-a-hash"}},
	})
	if _, err := cv.VerifyBundle(badHash, nil, profileTestRepo, tag); !errors.Is(err, ErrMalformedHash) {
		t.Fatalf("expected ErrMalformedHash, got %v", err)
	}
}

func TestChecksumVerifierEligibilityAndCandidate(t *testing.T) {
	cv := checksumVerifier{}

	if err := cv.CheckEligibility("dev", "v1.0.0", ""); err != nil {
		t.Fatalf("dev build without floor must be eligible, got %v", err)
	}
	if ok, err := cv.UpdateCandidate("dev", "v1.0.0", ""); err != nil || !ok {
		t.Fatalf("dev build without floor must report a candidate, got (%v, %v)", ok, err)
	}
	if err := cv.CheckEligibility("dev", "v2.0.0", "v1.0.0"); err != nil {
		t.Fatalf("dev build above the floor must be eligible, got %v", err)
	}
	if ok, err := cv.UpdateCandidate("dev", "v2.0.0", "v1.0.0"); err != nil || !ok {
		t.Fatalf("dev build above the floor must report a candidate, got (%v, %v)", ok, err)
	}
	if err := cv.CheckEligibility("dev", "v1.0.0", "v1.0.0"); !errors.Is(err, ErrDowngradeOrReplay) {
		t.Fatalf("expected ErrDowngradeOrReplay at the applied floor, got %v", err)
	}
	if ok, err := cv.UpdateCandidate("dev", "v1.0.0", "v1.0.0"); err != nil || ok {
		t.Fatalf("floor replay must not report a candidate, got (%v, %v)", ok, err)
	}

	if err := cv.CheckEligibility("v1.0.0", "v1.1.0", ""); err != nil {
		t.Fatalf("release build must keep canonical eligibility, got %v", err)
	}
	if err := cv.CheckEligibility("v1.0.0", "v1.0.0", ""); !errors.Is(err, ErrDowngradeOrReplay) {
		t.Fatalf("release build must reject non-increasing candidates, got %v", err)
	}
	if ok, err := cv.UpdateCandidate("v1.0.0", "v1.1.0", ""); err != nil || !ok {
		t.Fatalf("release build must keep canonical candidate comparison, got (%v, %v)", ok, err)
	}

	const nonCanonical = "v1.0.0-5-gabcdef"
	if err := cv.CheckEligibility(nonCanonical, "v2.0.0", ""); err == nil {
		t.Fatal("git-describe build must not be eligible under checksum")
	}
	if _, err := cv.UpdateCandidate(nonCanonical, "v2.0.0", ""); err == nil {
		t.Fatal("git-describe build must not report a candidate under checksum")
	}
	if err := cv.CheckEligibility("v1.0.0", "", ""); err == nil {
		t.Fatal("empty candidate tag must be rejected")
	}
}

func TestVerifierAdaptersContract(t *testing.T) {
	cv := checksumVerifier{}
	if cv.Name() != ProfileChecksum {
		t.Fatalf("checksum verifier name = %q", cv.Name())
	}
	assets := cv.RequiredAssets()
	if len(assets) != 1 || assets[0] != "release-manifest.json" {
		t.Fatalf("checksum required assets = %v", assets)
	}

	sv := strictVerifier{}
	if sv.Name() != ProfileStrict {
		t.Fatalf("strict verifier name = %q", sv.Name())
	}
	strictAssets := sv.RequiredAssets()
	if len(strictAssets) != 2 || strictAssets[1] != "release-manifest.sig" {
		t.Fatalf("strict required assets = %v", strictAssets)
	}
	if err := sv.CheckEligibility("dev", "v1.0.0", ""); !errors.Is(err, ErrDevUnknownVersion) {
		t.Fatalf("strict must keep rejecting dev builds, got %v", err)
	}
}
