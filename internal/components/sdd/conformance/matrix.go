package conformance

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Disposition is the adapter/profile result vocabulary. Rejected means the
// capability decision happened before any filesystem mutation.
type Disposition string

const (
	DispositionSupported Disposition = "supported"
	DispositionDegraded  Disposition = "degraded"
	DispositionRejected  Disposition = "rejected"
)

type Matrix struct {
	Adapters []string `json:"adapters"`
	Profiles []string `json:"profiles"`
	Cells    []Cell   `json:"cells"`
}

type Cell struct {
	Adapter          string            `json:"adapter"`
	RequestedProfile string            `json:"requested_profile"`
	EffectiveProfile string            `json:"effective_profile"`
	Disposition      Disposition       `json:"disposition"`
	ReasonID         string            `json:"reason_id"`
	Command          string            `json:"command"`
	ExitCode         int               `json:"exit_code"`
	Hash             string            `json:"hash"`
	Evidence         map[string]string `json:"evidence"`
}

type MatrixReport struct {
	Adapters    []string `json:"adapters"`
	Profiles    []string `json:"profiles"`
	Cells       []Cell   `json:"cells"`
	Fingerprint string   `json:"fingerprint"`
}

// LoadMatrix decodes a machine-readable matrix without accepting unknown
// fields that could hide an untracked result.
func LoadMatrix(data []byte) (Matrix, error) {
	var matrix Matrix
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&matrix); err != nil {
		return Matrix{}, fmt.Errorf("decode conformance matrix: %w", err)
	}
	return matrix, nil
}

// ValidateMatrix enforces exact Cartesian coverage and explicit evidence for
// every cell. It deliberately does not infer defaults for missing fields.
func ValidateMatrix(matrix Matrix) error {
	if len(matrix.Adapters) == 0 || len(matrix.Profiles) == 0 {
		return fmt.Errorf("matrix adapters and profiles are required")
	}
	if len(matrix.Cells) != len(matrix.Adapters)*len(matrix.Profiles) {
		return fmt.Errorf("matrix has %d cells, want exact Cartesian cardinality %d", len(matrix.Cells), len(matrix.Adapters)*len(matrix.Profiles))
	}
	adapters := make(map[string]struct{}, len(matrix.Adapters))
	for _, adapter := range matrix.Adapters {
		if adapter == "" {
			return fmt.Errorf("matrix contains empty adapter")
		}
		if _, exists := adapters[adapter]; exists {
			return fmt.Errorf("duplicate adapter %q", adapter)
		}
		adapters[adapter] = struct{}{}
	}
	profiles := make(map[string]struct{}, len(matrix.Profiles))
	for _, profile := range matrix.Profiles {
		if profile == "" {
			return fmt.Errorf("matrix contains empty profile")
		}
		if _, exists := profiles[profile]; exists {
			return fmt.Errorf("duplicate profile %q", profile)
		}
		profiles[profile] = struct{}{}
	}
	seen := make(map[string]struct{}, len(matrix.Cells))
	for index, cell := range matrix.Cells {
		if _, ok := adapters[cell.Adapter]; !ok {
			return fmt.Errorf("cell %d references unknown adapter %q", index, cell.Adapter)
		}
		if _, ok := profiles[cell.RequestedProfile]; !ok {
			return fmt.Errorf("cell %d references unknown profile %q", index, cell.RequestedProfile)
		}
		key := cell.Adapter + "\x00" + cell.RequestedProfile
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate matrix cell %q/%q", cell.Adapter, cell.RequestedProfile)
		}
		seen[key] = struct{}{}
		if cell.EffectiveProfile == "" || cell.ReasonID == "" || cell.Command == "" || cell.Hash == "" || cell.Evidence == nil {
			return fmt.Errorf("cell %q/%q lacks explicit profile, reason, command, hash, or evidence", cell.Adapter, cell.RequestedProfile)
		}
		switch cell.Disposition {
		case DispositionSupported:
			if cell.EffectiveProfile != cell.RequestedProfile {
				return fmt.Errorf("supported cell %q/%q changes effective profile", cell.Adapter, cell.RequestedProfile)
			}
		case DispositionDegraded:
			if cell.EffectiveProfile == cell.RequestedProfile {
				return fmt.Errorf("degraded cell %q/%q has no effective degradation", cell.Adapter, cell.RequestedProfile)
			}
		case DispositionRejected:
			if cell.ExitCode == 0 || cell.Evidence["mutation"] != "none" || cell.Evidence["pre_mutation"] == "" {
				return fmt.Errorf("rejected cell %q/%q lacks non-mutating pre-mutation proof", cell.Adapter, cell.RequestedProfile)
			}
		default:
			return fmt.Errorf("cell %q/%q has invalid disposition %q", cell.Adapter, cell.RequestedProfile, cell.Disposition)
		}
	}
	if len(seen) != len(matrix.Cells) {
		return fmt.Errorf("matrix contains missing or duplicate cells")
	}
	return nil
}

// RunMatrix is retained as an ingress guard for legacy declarative fixtures.
// R7 evidence must use RuntimeHarness; sorting/hash-sealing rows is not
// execution evidence and is rejected deliberately.
func RunMatrix(matrix Matrix) (MatrixReport, error) {
	return MatrixReport{}, fmt.Errorf("declarative matrix execution is disabled; run RuntimeHarness")
}
