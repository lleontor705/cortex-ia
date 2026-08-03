package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// MatrixReceipt is the immutable aggregation of the raw runtime receipt and
// its canonical nine-phase accountability bindings. Raw execution evidence is
// retained verbatim so aggregation cannot become a second source of truth.
type MatrixReceipt struct {
	Raw         RuntimeReceipt       `json:"raw"`
	Bindings    []MatrixPhaseBinding `json:"bindings"`
	Fingerprint string               `json:"fingerprint"`
}

type MatrixReceiptOptions struct {
	RouteEvidence       []string
	QualityEvidence     []string
	TrustEvidence       []string
	PermissionEvidence  []string
	DestinationEvidence []string
}

type MatrixPhaseBinding struct {
	CellExecutionHash   string         `json:"cell_execution_hash"`
	ReceiptHash         string         `json:"receipt_hash"`
	EvidenceHash        string         `json:"evidence_hash"`
	Cell                string         `json:"cell"`
	Phase               string         `json:"phase"`
	RequestedProfile    string         `json:"requested_profile"`
	EffectiveProfile    string         `json:"effective_profile"`
	RouteEvidence       []string       `json:"route_evidence"`
	ProfileEvidence     []string       `json:"profile_evidence"`
	QualityEvidence     []string       `json:"quality_evidence"`
	TrustEvidence       []string       `json:"trust_evidence"`
	PermissionEvidence  []string       `json:"permission_evidence"`
	DestinationEvidence []string       `json:"destination_evidence"`
	Disposition         Disposition    `json:"disposition"`
	ReasonID            string         `json:"reason_id"`
	Accountability      Accountability `json:"accountability"`
	PreMutation         bool           `json:"pre_mutation"`
}

func AggregateMatrixReceipt(raw RuntimeReceipt, options MatrixReceiptOptions) (MatrixReceipt, error) {
	if err := validateRawReceiptForAggregation(raw); err != nil {
		return MatrixReceipt{}, err
	}
	route := sortedNonEmpty(options.RouteEvidence)
	quality := sortedNonEmpty(options.QualityEvidence)
	trust := sortedNonEmpty(options.TrustEvidence)
	permissions := sortedNonEmpty(options.PermissionEvidence)
	destination := sortedNonEmpty(options.DestinationEvidence)
	if len(route) == 0 || len(quality) == 0 || len(trust) == 0 || len(permissions) == 0 || len(destination) == 0 {
		return MatrixReceipt{}, fmt.Errorf("matrix receipt options require route, quality, trust, permission, and destination evidence")
	}
	receipt := MatrixReceipt{Raw: cloneRuntimeReceipt(raw), Bindings: make([]MatrixPhaseBinding, 0, len(raw.Cells)*len(canonicalPhases))}
	for cellIndex, cell := range raw.Cells {
		blocked := cell.Disposition == DispositionBlocked || cell.Disposition == DispositionRejected
		accountability := AccountabilityEmitted
		if blocked {
			accountability = AccountabilityPreMutation
		}
		profileEvidence := []string{"requested:" + cell.RequestedProfile, "effective:" + cell.EffectiveProfile}
		for phaseIndex, phase := range canonicalPhases {
			binding := MatrixPhaseBinding{
				CellExecutionHash: executionHash(cell, cellIndex), ReceiptHash: cell.ReceiptDigest, EvidenceHash: cell.EvidenceDigest,
				Cell: cell.Adapter + "/" + cell.RequestedProfile, Phase: phase,
				RequestedProfile: cell.RequestedProfile, EffectiveProfile: cell.EffectiveProfile,
				RouteEvidence: append([]string(nil), route...), ProfileEvidence: append([]string(nil), profileEvidence...),
				QualityEvidence: append([]string(nil), quality...), TrustEvidence: append([]string(nil), trust...),
				PermissionEvidence: append([]string(nil), permissions...), DestinationEvidence: append([]string(nil), destination...),
				Disposition: cell.Disposition, ReasonID: cell.ReasonID, Accountability: accountability, PreMutation: blocked,
			}
			if binding.CellExecutionHash == "" {
				return MatrixReceipt{}, fmt.Errorf("first defect cell[%d] phase[%d]: missing execution hash", cellIndex, phaseIndex)
			}
			receipt.Bindings = append(receipt.Bindings, binding)
		}
	}
	return sealMatrixReceipt(receipt), nil
}

func ValidateMatrixReceipt(receipt MatrixReceipt) error {
	if err := validateRawReceiptForAggregation(receipt.Raw); err != nil {
		return fmt.Errorf("matrix receipt raw: %w", err)
	}
	if len(receipt.Bindings) != 108 {
		return fmt.Errorf("matrix receipt bindings = %d, want 108", len(receipt.Bindings))
	}
	seen := make(map[string]struct{}, len(receipt.Bindings))
	cells := make(map[string]RuntimeCell, len(receipt.Raw.Cells))
	for _, cell := range receipt.Raw.Cells {
		cells[cell.Adapter+"/"+cell.RequestedProfile] = cell
	}
	for index, binding := range receipt.Bindings {
		if binding.Cell == "" || binding.Phase == "" || !contains(canonicalPhases, binding.Phase) || binding.CellExecutionHash == "" || binding.ReceiptHash == "" || binding.EvidenceHash == "" {
			return fmt.Errorf("first defect binding[%d]: missing canonical linkage", index)
		}
		key := binding.Cell + "\x00" + binding.Phase
		if _, exists := seen[key]; exists {
			return fmt.Errorf("first defect binding[%d]: duplicate %s", index, key)
		}
		seen[key] = struct{}{}
		cell, exists := cells[binding.Cell]
		if !exists || binding.ReceiptHash != cell.ReceiptDigest || binding.EvidenceHash != cell.EvidenceDigest || binding.RequestedProfile != cell.RequestedProfile || binding.EffectiveProfile != cell.EffectiveProfile {
			return fmt.Errorf("first defect binding[%d]: detached cell receipt/evidence linkage", index)
		}
		if err := validateCanonicalDigest(binding.CellExecutionHash); err != nil {
			return fmt.Errorf("first defect binding[%d]: execution hash: %w", index, err)
		}
		if err := validateCanonicalDigest(binding.ReceiptHash); err != nil {
			return fmt.Errorf("first defect binding[%d]: receipt hash: %w", index, err)
		}
		if err := validateCanonicalDigest(binding.EvidenceHash); err != nil {
			return fmt.Errorf("first defect binding[%d]: evidence hash: %w", index, err)
		}
		if len(binding.RouteEvidence) == 0 || len(binding.ProfileEvidence) == 0 || len(binding.QualityEvidence) == 0 || len(binding.TrustEvidence) == 0 || len(binding.PermissionEvidence) == 0 || len(binding.DestinationEvidence) == 0 {
			return fmt.Errorf("first defect binding[%d]: incomplete route/profile/quality/trust/permission/destination evidence", index)
		}
		if binding.Disposition == DispositionBlocked || binding.Disposition == DispositionRejected {
			if binding.Accountability != AccountabilityPreMutation || !binding.PreMutation {
				return fmt.Errorf("first defect binding[%d]: blocked record lacks pre-mutation accountability", index)
			}
		} else if binding.Accountability != AccountabilityEmitted || binding.PreMutation {
			return fmt.Errorf("first defect binding[%d]: emitted record has invalid accountability", index)
		}
	}
	if receipt.Fingerprint == "" || sealMatrixReceipt(MatrixReceipt{Raw: receipt.Raw, Bindings: receipt.Bindings}).Fingerprint != receipt.Fingerprint {
		return fmt.Errorf("matrix receipt fingerprint mismatch")
	}
	return nil
}

func validateRawReceiptForAggregation(raw RuntimeReceipt) error {
	if err := raw.Validate(); err != nil {
		return fmt.Errorf("first defect raw receipt: %w", err)
	}
	for index, cell := range raw.Cells {
		if err := validateCanonicalDigest(cell.ReceiptDigest); err != nil {
			return fmt.Errorf("first defect cell[%d]: receipt hash: %w", index, err)
		}
		if err := validateCanonicalDigest(cell.EvidenceDigest); err != nil {
			return fmt.Errorf("first defect cell[%d]: evidence hash: %w", index, err)
		}
		if got := digestJSON(cell.Evidence); got != cell.EvidenceDigest {
			return fmt.Errorf("first defect cell[%d]: evidence digest mismatch", index)
		}
	}
	return nil
}

func executionHash(cell RuntimeCell, index int) string {
	data, _ := json.Marshal(struct {
		Index int         `json:"index"`
		Cell  RuntimeCell `json:"cell"`
	}{index, cell})
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func sealMatrixReceipt(receipt MatrixReceipt) MatrixReceipt {
	receipt.Fingerprint = ""
	data, _ := json.Marshal(receipt)
	digest := sha256.Sum256(data)
	receipt.Fingerprint = "sha256:" + hex.EncodeToString(digest[:])
	return receipt
}

func cloneRuntimeReceipt(input RuntimeReceipt) RuntimeReceipt {
	output := input
	output.Adapters = append([]string(nil), input.Adapters...)
	output.Profiles = append([]string(nil), input.Profiles...)
	output.Cells = append([]RuntimeCell(nil), input.Cells...)
	for i := range output.Cells {
		output.Cells[i].Evidence = map[string]string{}
		for key, value := range input.Cells[i].Evidence {
			output.Cells[i].Evidence[key] = value
		}
	}
	return output
}
