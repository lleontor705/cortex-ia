package rdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GateResult represents the outcome of evaluating a delivery gate.
type GateResult struct {
	Allowed bool             `json:"allowed"`
	Reason  string           `json:"reason"`
	Receipt *DeliveryReceipt `json:"receipt,omitempty"`
}

// ValidateDeliveryGate inspects the repository for a valid delivery receipt matching the candidate hash.
func ValidateDeliveryGate(repoRoot string, candidateSHA string) GateResult {
	if candidateSHA == "" {
		return GateResult{
			Allowed: false,
			Reason:  "Missing candidate SHA256",
		}
	}

	receiptFile := filepath.Join(repoRoot, ".cortex", "receipts", candidateSHA+".json")
	data, err := os.ReadFile(receiptFile)
	if err != nil {
		if os.IsNotExist(err) {
			return GateResult{
				Allowed: false,
				Reason:  fmt.Sprintf("No valid delivery receipt found for candidate SHA %s", candidateSHA),
			}
		}
		return GateResult{
			Allowed: false,
			Reason:  fmt.Errorf("read receipt file: %w", err).Error(),
		}
	}

	var receipt DeliveryReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return GateResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Corrupt delivery receipt: %v", err),
		}
	}

	if receipt.Status != "VERIFIED" {
		return GateResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Delivery receipt verification failed with status: %s", receipt.Status),
			Receipt: &receipt,
		}
	}

	if receipt.Verification.ExitCode != 0 {
		return GateResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Delivery receipt proof failed with exit code: %d", receipt.Verification.ExitCode),
			Receipt: &receipt,
		}
	}

	return GateResult{
		Allowed: true,
		Reason:  "Delivery receipt valid and verified",
		Receipt: &receipt,
	}
}
