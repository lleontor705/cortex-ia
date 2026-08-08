package rdd

import (
	"testing"
)

func TestFreezeAndValidateReceipt(t *testing.T) {
	tmpDir := t.TempDir()
	diff := []byte("diff --git a/main.go b/main.go\n+ func Hello() {}")

	candidateSHA := FreezeCandidate(tmpDir, diff)
	if candidateSHA == "" {
		t.Fatal("FreezeCandidate returned empty string")
	}

	proof := Proof{
		Command:       "go test ./...",
		ExitCode:      0,
		OutputSummary: "PASS",
	}

	receipt, err := GenerateReceipt(tmpDir, "cortex-ia", candidateSHA, proof)
	if err != nil {
		t.Fatalf("GenerateReceipt failed: %v", err)
	}

	if receipt.Status != "VERIFIED" {
		t.Errorf("expected VERIFIED, got %s", receipt.Status)
	}

	gateRes := ValidateDeliveryGate(tmpDir, candidateSHA)
	if !gateRes.Allowed {
		t.Errorf("expected gate allowed, got reason: %s", gateRes.Reason)
	}
}

func TestValidateDeliveryGate_MissingReceipt(t *testing.T) {
	tmpDir := t.TempDir()
	gateRes := ValidateDeliveryGate(tmpDir, "nonexistent-sha")

	if gateRes.Allowed {
		t.Error("expected gate denied for missing receipt")
	}
}
