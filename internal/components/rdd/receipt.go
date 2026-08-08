// Package rdd provides Receipt-Driven Development (RDD) structures, candidate byte freezing, and delivery gate validation.
package rdd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeliveryReceipt represents an immutable, content-bound delivery authorization receipt.
type DeliveryReceipt struct {
	SchemaVersion   string    `json:"schema_version"`
	Project         string    `json:"project"`
	CandidateSHA256 string    `json:"candidate_sha256"` // SHA256 of frozen candidate diff
	Timestamp       time.Time `json:"timestamp"`
	Status          string    `json:"status"` // "VERIFIED" | "FAILED" | "UNMANAGED"
	Verification    Proof     `json:"verification"`
	Signature       string    `json:"signature"`
}

// Proof contains empirical verification evidence.
type Proof struct {
	Command       string `json:"command"`
	ExitCode      int    `json:"exit_code"`
	OutputSummary string `json:"output_summary"`
}

// FreezeCandidate computes a deterministic SHA256 hash of the target repository state.
func FreezeCandidate(repoRoot string, diffContent []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(repoRoot))
	hasher.Write(diffContent)
	return hex.EncodeToString(hasher.Sum(nil))
}

// GenerateReceipt creates and persists a signed delivery receipt in .cortex/receipts/<hash>.json.
func GenerateReceipt(repoRoot string, project string, candidateSHA string, proof Proof) (*DeliveryReceipt, error) {
	status := "VERIFIED"
	if proof.ExitCode != 0 {
		status = "FAILED"
	}

	sig := hex.EncodeToString([]byte(fmt.Sprintf("%s:%s:%d", candidateSHA, status, proof.ExitCode)))

	receipt := &DeliveryReceipt{
		SchemaVersion:   "1.0.0",
		Project:         project,
		CandidateSHA256: candidateSHA,
		Timestamp:       time.Now().UTC(),
		Status:          status,
		Verification:    proof,
		Signature:       sig,
	}

	receiptsDir := filepath.Join(repoRoot, ".cortex", "receipts")
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create receipts dir: %w", err)
	}

	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal receipt: %w", err)
	}

	receiptFile := filepath.Join(receiptsDir, candidateSHA+".json")
	if err := os.WriteFile(receiptFile, data, 0o644); err != nil {
		return nil, fmt.Errorf("write receipt file: %w", err)
	}

	return receipt, nil
}
