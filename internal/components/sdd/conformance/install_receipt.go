package conformance

import "fmt"

type InstallReceipt struct {
	PreviewFingerprint  string `json:"preview_fingerprint"`
	ApplyFingerprint    string `json:"apply_fingerprint"`
	ReceiptFingerprint  string `json:"receipt_fingerprint"`
	Idempotent          bool   `json:"idempotent"`
	LegacyWriterInvoked bool   `json:"legacy_writer_invoked"`
}

func ValidateInstallReceipt(evidence EvidenceReport, receipt InstallReceipt) error {
	if evidence.Fingerprint == "" {
		return fmt.Errorf("evidence fingerprint is required")
	}
	if receipt.PreviewFingerprint != evidence.Fingerprint || receipt.ApplyFingerprint != evidence.Fingerprint || receipt.ReceiptFingerprint != evidence.Fingerprint {
		return fmt.Errorf("preview, apply, and receipt fingerprints must equal evidence fingerprint")
	}
	if !receipt.Idempotent {
		return fmt.Errorf("second install is not idempotent")
	}
	if receipt.LegacyWriterInvoked {
		return fmt.Errorf("legacy writer/fallback was invoked")
	}
	return nil
}
