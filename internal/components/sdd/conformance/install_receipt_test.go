package conformance

import "testing"

func TestInstallEvidenceProvesPreviewApplyReceiptEquality(t *testing.T) {
	evidence := mustEvidence(t, minimalMatrix())
	receipt := InstallReceipt{PreviewFingerprint: evidence.Fingerprint, ApplyFingerprint: evidence.Fingerprint, ReceiptFingerprint: evidence.Fingerprint, Idempotent: true, LegacyWriterInvoked: false}
	if err := ValidateInstallReceipt(evidence, receipt); err != nil {
		t.Fatal(err)
	}
}

func TestInstallEvidenceRejectsFingerprintDriftAndLegacyWriter(t *testing.T) {
	evidence := mustEvidence(t, minimalMatrix())
	tests := []struct {
		name   string
		mutate func(*InstallReceipt)
	}{
		{"preview drift", func(r *InstallReceipt) { r.PreviewFingerprint = "different" }},
		{"apply drift", func(r *InstallReceipt) { r.ApplyFingerprint = "different" }},
		{"receipt drift", func(r *InstallReceipt) { r.ReceiptFingerprint = "different" }},
		{"legacy writer", func(r *InstallReceipt) { r.LegacyWriterInvoked = true }},
		{"non-idempotent", func(r *InstallReceipt) { r.Idempotent = false }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receipt := InstallReceipt{PreviewFingerprint: evidence.Fingerprint, ApplyFingerprint: evidence.Fingerprint, ReceiptFingerprint: evidence.Fingerprint, Idempotent: true}
			tc.mutate(&receipt)
			if err := ValidateInstallReceipt(evidence, receipt); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func mustEvidence(t *testing.T, matrix Matrix) EvidenceReport {
	t.Helper()
	evidence, err := AggregateEvidence(matrix, defaultEvidenceOptions())
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}
