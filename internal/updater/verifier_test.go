package updater

import (
	"errors"
	"testing"
)

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

func TestDefaultVerifierIsChecksum(t *testing.T) {
	if _, ok := defaultVerifier().(checksumVerifier); !ok {
		t.Fatalf("defaultVerifier() = %T, want checksumVerifier", defaultVerifier())
	}
}
