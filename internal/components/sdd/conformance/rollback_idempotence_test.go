package conformance

import "testing"

func TestRollbackEvidenceRestoresManagedBundleAndPreservesUnmanagedRoots(t *testing.T) {
	evidence := mustEvidence(t, minimalMatrix())
	receipt := RollbackReceipt{PriorManagedFingerprint: "prior", RestoredManagedFingerprint: "prior", ExternalBeforeFingerprint: "external", ExternalAfterFingerprint: "external", UnmanagedBeforeFingerprint: "unmanaged", UnmanagedAfterFingerprint: "unmanaged", RollbackCount: 1}
	if err := ValidateRollbackReceipt(evidence, receipt); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackEvidenceRejectsNonIdempotentOrChangedProtectedState(t *testing.T) {
	evidence := mustEvidence(t, minimalMatrix())
	base := RollbackReceipt{PriorManagedFingerprint: "prior", RestoredManagedFingerprint: "prior", ExternalBeforeFingerprint: "external", ExternalAfterFingerprint: "external", UnmanagedBeforeFingerprint: "unmanaged", UnmanagedAfterFingerprint: "unmanaged", RollbackCount: 1}
	tests := []struct {
		name   string
		mutate func(*RollbackReceipt)
	}{
		{"managed not restored", func(r *RollbackReceipt) { r.RestoredManagedFingerprint = "new" }},
		{"external changed", func(r *RollbackReceipt) { r.ExternalAfterFingerprint = "new" }},
		{"unmanaged changed", func(r *RollbackReceipt) { r.UnmanagedAfterFingerprint = "new" }},
		{"second rollback not idempotent", func(r *RollbackReceipt) { r.RollbackCount = 2 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receipt := base
			tc.mutate(&receipt)
			if err := ValidateRollbackReceipt(evidence, receipt); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
