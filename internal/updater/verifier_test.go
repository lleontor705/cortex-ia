package updater

import (
	"errors"
	"testing"
)

func TestResolveProfileSelection(t *testing.T) {
	cases := []struct {
		bundlePresent bool
		consent       bool
		want          VerificationProfile
	}{
		{true, true, ProfileChecksum},
		{true, false, ProfileChecksum},
		{false, true, ProfileChecksum},
		{false, false, ProfileChecksum},
	}
	for _, tc := range cases {
		got := ResolveProfile(tc.bundlePresent, tc.consent)
		if got != tc.want {
			t.Fatalf("ResolveProfile(%v, %v) = %q, want %q", tc.bundlePresent, tc.consent, got, tc.want)
		}
	}
}

func TestChecksumVerifierAuthorityGate(t *testing.T) {
	if err := (checksumVerifier{}).RequireAuthority(); err != nil {
		t.Fatalf("expected nil authority error for checksum verifier, got %v", err)
	}
}

func TestRequireProfileAuthorityGate(t *testing.T) {
	if err := RequireProfileAuthority(); err != nil {
		t.Fatalf("RequireProfileAuthority must return nil, got %v", err)
	}
}

func TestChecksumVerifierEligibilityAndCandidate(t *testing.T) {
	cv := checksumVerifier{}

	if err := cv.CheckEligibility("dev", "v1.0.0", ""); err != nil {
		t.Fatalf("dev build without floor must be eligible, got %v", err)
	}
	if ok, err := cv.UpdateCandidate("dev", "v1.0.0", ""); err != nil || ok {
		t.Fatalf("dev build must not report an update candidate, got (%v, %v)", ok, err)
	}
	if err := cv.CheckEligibility("dev", "v2.0.0", "v1.0.0"); err != nil {
		t.Fatalf("dev build above the floor must be eligible, got %v", err)
	}
	if err := cv.CheckEligibility("dev", "v1.0.0", "v1.0.0"); !errors.Is(err, ErrDowngradeOrReplay) {
		t.Fatalf("expected ErrDowngradeOrReplay at the applied floor, got %v", err)
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
	if len(assets) != 1 || assets[0] != ChecksumsFileName {
		t.Fatalf("checksum required assets = %v", assets)
	}
}
