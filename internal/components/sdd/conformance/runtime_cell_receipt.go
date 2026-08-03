package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// DispositionBlocked is an observed, pre-mutation capability result. It is
// distinct from a phase-level blocked status and never represents a skipped
// matrix cell.
const DispositionBlocked Disposition = "blocked"

type RuntimeCell struct {
	Adapter          string            `json:"adapter"`
	RequestedProfile string            `json:"requested_profile"`
	EffectiveProfile string            `json:"effective_profile"`
	Disposition      Disposition       `json:"disposition"`
	ReasonID         string            `json:"reason_id"`
	Command          string            `json:"command"`
	Path             string            `json:"path"`
	ExitCode         int               `json:"exit_code"`
	ReceiptDigest    string            `json:"receipt_digest"`
	EvidenceDigest   string            `json:"evidence_digest"`
	Evidence         map[string]string `json:"evidence"`
}

type RuntimeReceipt struct {
	Adapters []string      `json:"adapters"`
	Profiles []string      `json:"profiles"`
	Cells    []RuntimeCell `json:"cells"`
	Digest   string        `json:"digest"`
}

func (r RuntimeReceipt) Validate() error {
	if len(r.Adapters) == 0 || len(r.Profiles) == 0 {
		return fmt.Errorf("runtime receipt adapters and profiles are required")
	}
	if len(r.Cells) != len(r.Adapters)*len(r.Profiles) {
		return fmt.Errorf("runtime receipt cells = %d, want exact Cartesian cardinality %d", len(r.Cells), len(r.Adapters)*len(r.Profiles))
	}
	adapters := make(map[string]struct{}, len(r.Adapters))
	for _, value := range r.Adapters {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("runtime receipt contains empty adapter")
		}
		if _, ok := adapters[value]; ok {
			return fmt.Errorf("runtime receipt duplicate adapter %q", value)
		}
		adapters[value] = struct{}{}
	}
	profiles := make(map[string]struct{}, len(r.Profiles))
	for _, value := range r.Profiles {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("runtime receipt contains empty profile")
		}
		if _, ok := profiles[value]; ok {
			return fmt.Errorf("runtime receipt duplicate profile %q", value)
		}
		profiles[value] = struct{}{}
	}
	seen := make(map[string]struct{}, len(r.Cells))
	for index, cell := range r.Cells {
		if _, ok := adapters[cell.Adapter]; !ok {
			return fmt.Errorf("runtime cell %d references unknown adapter %q", index, cell.Adapter)
		}
		if _, ok := profiles[cell.RequestedProfile]; !ok {
			return fmt.Errorf("runtime cell %d references unknown profile %q", index, cell.RequestedProfile)
		}
		key := cell.Adapter + "\x00" + cell.RequestedProfile
		if _, ok := seen[key]; ok {
			return fmt.Errorf("runtime receipt duplicate cell %q/%q", cell.Adapter, cell.RequestedProfile)
		}
		seen[key] = struct{}{}
		if cell.EffectiveProfile == "" || cell.ReasonID == "" || cell.Command == "" || cell.Path == "" || cell.ReceiptDigest == "" || cell.EvidenceDigest == "" || cell.Evidence == nil {
			return fmt.Errorf("runtime cell %q/%q lacks execution evidence", cell.Adapter, cell.RequestedProfile)
		}
		if err := validateCanonicalDigest(cell.ReceiptDigest); err != nil {
			return fmt.Errorf("runtime cell %q/%q receipt digest: %w", cell.Adapter, cell.RequestedProfile, err)
		}
		if err := validateCanonicalDigest(cell.EvidenceDigest); err != nil {
			return fmt.Errorf("runtime cell %q/%q evidence digest: %w", cell.Adapter, cell.RequestedProfile, err)
		}
		if strings.Contains(cell.Command, "matrix-probe") || strings.Contains(cell.Command, "synthetic") || cell.Evidence["execution"] != "production" {
			return fmt.Errorf("runtime cell %q/%q is not production-executed", cell.Adapter, cell.RequestedProfile)
		}
		switch cell.Disposition {
		case DispositionSupported, DispositionDegraded:
			if cell.ExitCode != 0 {
				return fmt.Errorf("runtime cell %q/%q successful disposition has exit %d", cell.Adapter, cell.RequestedProfile, cell.ExitCode)
			}
		case DispositionBlocked, DispositionRejected:
			if cell.ExitCode == 0 || cell.Evidence["mutation"] != "none" || cell.Evidence["pre_mutation"] != "true" {
				return fmt.Errorf("runtime cell %q/%q blocked without observed pre-mutation evidence", cell.Adapter, cell.RequestedProfile)
			}
		default:
			return fmt.Errorf("runtime cell %q/%q has invalid disposition %q", cell.Adapter, cell.RequestedProfile, cell.Disposition)
		}
	}
	if len(seen) != len(r.Cells) {
		return fmt.Errorf("runtime receipt contains missing cells")
	}
	if err := validateCanonicalDigest(r.Digest); err != nil {
		return fmt.Errorf("runtime receipt digest: %w", err)
	}
	if r.Digest != sealRuntimeReceipt(r).Digest {
		return fmt.Errorf("runtime receipt digest mismatch")
	}
	return nil
}

// validateCanonicalDigest is the single boundary for runtime evidence hashes.
// Callers must never accept a bare digest or normalize malformed input.
func validateCanonicalDigest(value string) error {
	const prefix = "sha256:"
	if len(value) != len(prefix)+sha256.Size*2 || !strings.HasPrefix(value, prefix) {
		return fmt.Errorf("must match sha256:<64 lowercase hex>")
	}
	hexPart := strings.TrimPrefix(value, prefix)
	if hexPart != strings.ToLower(hexPart) {
		return fmt.Errorf("must use lowercase hexadecimal")
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return fmt.Errorf("contains non-hexadecimal characters: %w", err)
	}
	return nil
}

// canonicalReceiptDigest converts the install package's checksum (the digest
// of its immutable receipt bytes) at the runtime boundary exactly once.
func canonicalReceiptDigest(bare string) (string, error) {
	if len(bare) != sha256.Size*2 || bare != strings.ToLower(bare) {
		return "", fmt.Errorf("receipt checksum must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(bare); err != nil {
		return "", fmt.Errorf("receipt checksum is not hexadecimal: %w", err)
	}
	return "sha256:" + bare, nil
}

func sealRuntimeReceipt(receipt RuntimeReceipt) RuntimeReceipt {
	receipt.Digest = ""
	data, _ := json.Marshal(receipt)
	digest := sha256.Sum256(data)
	receipt.Digest = "sha256:" + hex.EncodeToString(digest[:])
	return receipt
}
