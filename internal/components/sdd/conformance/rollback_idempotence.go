package conformance

import "fmt"

type RollbackReceipt struct {
	PriorManagedFingerprint    string `json:"prior_managed_fingerprint"`
	RestoredManagedFingerprint string `json:"restored_managed_fingerprint"`
	ExternalBeforeFingerprint  string `json:"external_before_fingerprint"`
	ExternalAfterFingerprint   string `json:"external_after_fingerprint"`
	UnmanagedBeforeFingerprint string `json:"unmanaged_before_fingerprint"`
	UnmanagedAfterFingerprint  string `json:"unmanaged_after_fingerprint"`
	RollbackCount              int    `json:"rollback_count"`
}

func ValidateRollbackReceipt(evidence EvidenceReport, receipt RollbackReceipt) error {
	if evidence.Fingerprint == "" {
		return fmt.Errorf("evidence fingerprint is required")
	}
	if receipt.PriorManagedFingerprint == "" || receipt.PriorManagedFingerprint != receipt.RestoredManagedFingerprint {
		return fmt.Errorf("rollback did not restore prior managed bundle")
	}
	if receipt.ExternalBeforeFingerprint == "" || receipt.ExternalBeforeFingerprint != receipt.ExternalAfterFingerprint {
		return fmt.Errorf("external settings changed during rollback")
	}
	if receipt.UnmanagedBeforeFingerprint == "" || receipt.UnmanagedBeforeFingerprint != receipt.UnmanagedAfterFingerprint {
		return fmt.Errorf("unmanaged settings changed during rollback")
	}
	if receipt.RollbackCount != 1 {
		return fmt.Errorf("rollback is not idempotent: count=%d", receipt.RollbackCount)
	}
	return nil
}
