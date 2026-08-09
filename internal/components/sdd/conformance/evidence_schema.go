package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var canonicalBindings = []PhaseRoleBinding{
	{Phase: "bootstrap", Role: "bootstrap", Skill: "skills/bootstrap/SKILL.md"},
	{Phase: "investigate", Role: "investigate", Skill: "skills/investigate/SKILL.md"},
	{Phase: "propose", Role: "draft-proposal", Skill: "skills/draft-proposal/SKILL.md"},
	{Phase: "spec", Role: "write-specs", Skill: "skills/write-specs/SKILL.md"},
	{Phase: "design", Role: "architect", Skill: "skills/architect/SKILL.md"},
	{Phase: "tasks", Role: "decompose", Skill: "skills/decompose/SKILL.md"},
	{Phase: "apply", Role: "implement", Skill: "skills/implement/SKILL.md"},
	{Phase: "verify", Role: "validate", Skill: "skills/validate/SKILL.md"},
	{Phase: "archive", Role: "finalize", Skill: "skills/finalize/SKILL.md"},
}

var canonicalPhases = func() []string {
	phases := make([]string, len(canonicalBindings))
	for index, binding := range canonicalBindings {
		phases[index] = binding.Phase
	}
	return phases
}()

type Accountability string

const (
	AccountabilityEmitted     Accountability = "emitted"
	AccountabilityPreMutation Accountability = "pre-mutation"
)

type EvidenceOptions struct {
	ContractVersion     string
	ContractFingerprint string
	PrimaryModel        string
	FallbackModel       string
	ModelDegradation    string
	QualityPlan         string
	TrustEvidence       []string
	Permissions         []string
}

type PhaseRoleBinding struct {
	Phase string `json:"phase"`
	Role  string `json:"role"`
	Skill string `json:"skill"`
}

type SemanticAssetEvidence struct {
	ID          string `json:"id"`
	Class       string `json:"class"`
	Fingerprint string `json:"fingerprint"`
}

type ContractEvidence struct {
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
}

type ModelEvidence struct {
	Primary     string `json:"primary"`
	Fallback    string `json:"fallback"`
	Degradation string `json:"degradation"`
}

type PhaseBindingEvidence struct {
	Cell           string                  `json:"cell"`
	Role           PhaseRoleBinding        `json:"role"`
	SemanticAssets []SemanticAssetEvidence `json:"semantic_assets"`
	Contract       ContractEvidence        `json:"contract"`
	Model          ModelEvidence           `json:"model"`
	QualityPlan    string                  `json:"quality_plan"`
	TrustEvidence  []string                `json:"trust_evidence"`
	Permissions    []string                `json:"permissions"`
	Destination    string                  `json:"destination"`
	Disposition    Disposition             `json:"disposition"`
	ReasonID       string                  `json:"reason_id"`
	Accountability Accountability          `json:"accountability"`
}

type EvidenceReport struct {
	Records     []PhaseBindingEvidence `json:"records"`
	Fingerprint string                 `json:"fingerprint"`
}

func AggregateEvidence(matrix Matrix, options EvidenceOptions) (EvidenceReport, error) {
	if err := ValidateMatrix(matrix); err != nil {
		return EvidenceReport{}, err
	}
	if strings.TrimSpace(options.ContractVersion) == "" || strings.TrimSpace(options.ContractFingerprint) == "" || strings.TrimSpace(options.PrimaryModel) == "" || strings.TrimSpace(options.FallbackModel) == "" || strings.TrimSpace(options.QualityPlan) == "" {
		return EvidenceReport{}, fmt.Errorf("evidence options require contract, models, and quality plan")
	}
	trust := sortedNonEmpty(options.TrustEvidence)
	permissions := sortedNonEmpty(options.Permissions)
	if len(trust) == 0 || len(permissions) == 0 {
		return EvidenceReport{}, fmt.Errorf("trust evidence and permissions are required")
	}
	report := EvidenceReport{Records: make([]PhaseBindingEvidence, 0, len(matrix.Cells)*len(canonicalPhases))}
	for _, cell := range matrix.Cells {
		cellKey := cell.Adapter + "/" + cell.RequestedProfile
		accountability := AccountabilityEmitted
		if cell.Disposition == DispositionRejected {
			accountability = AccountabilityPreMutation
		}
		for _, binding := range canonicalBindings {
			record := PhaseBindingEvidence{
				Cell:           cellKey,
				Role:           binding,
				SemanticAssets: []SemanticAssetEvidence{{ID: "workflow/common", Class: "semantic", Fingerprint: cell.Hash}},
				Contract:       ContractEvidence{Version: options.ContractVersion, Fingerprint: options.ContractFingerprint},
				Model:          ModelEvidence{Primary: options.PrimaryModel, Fallback: options.FallbackModel, Degradation: options.ModelDegradation},
				QualityPlan:    options.QualityPlan, TrustEvidence: append([]string(nil), trust...), Permissions: append([]string(nil), permissions...),
				Destination: "" + cell.Adapter + "/" + cell.EffectiveProfile + "/" + binding.Role + "/SKILL.md",
				Disposition: cell.Disposition, ReasonID: cell.ReasonID, Accountability: accountability,
			}
			if record.Model.Degradation == "" {
				record.Model.Degradation = "none"
			}
			report.Records = append(report.Records, record)
		}
	}
	return sealEvidence(report), nil
}

func ValidateEvidence(report EvidenceReport) error {
	if len(report.Records) == 0 || len(report.Records)%len(canonicalPhases) != 0 {
		return fmt.Errorf("evidence records = %d, want one complete nine-phase set per cell", len(report.Records))
	}
	seen := make(map[string]struct{}, len(report.Records))
	for i, record := range report.Records {
		if record.Cell == "" || record.Role.Phase == "" || record.Role.Role == "" || record.Role.Skill == "" || record.Destination == "" || record.ReasonID == "" {
			return fmt.Errorf("record %d lacks canonical binding or destination", i)
		}
		if !contains(canonicalPhases, record.Role.Phase) {
			return fmt.Errorf("record %d has noncanonical phase %q", i, record.Role.Phase)
		}
		if canonical, ok := canonicalBindingForPhase(record.Role.Phase); !ok || record.Role != canonical {
			return fmt.Errorf("record %d has noncanonical phase-role-skill binding %+v", i, record.Role)
		}
		key := record.Cell + "\x00" + record.Role.Phase
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate phase binding %q", key)
		}
		seen[key] = struct{}{}
		if len(record.SemanticAssets) == 0 || record.Contract.Version == "" || record.Contract.Fingerprint == "" || record.Model.Primary == "" || record.Model.Fallback == "" || record.QualityPlan == "" || len(record.TrustEvidence) == 0 || len(record.Permissions) == 0 || record.Disposition == "" {
			return fmt.Errorf("record %d is incomplete", i)
		}
		if record.Disposition == DispositionRejected && record.Accountability != AccountabilityPreMutation {
			return fmt.Errorf("rejected record %d lacks pre-mutation accountability", i)
		}
		if record.Disposition != DispositionRejected && record.Accountability != AccountabilityEmitted {
			return fmt.Errorf("record %d lacks emitted accountability", i)
		}
	}
	if report.Fingerprint == "" {
		return fmt.Errorf("evidence fingerprint is required")
	}
	if got := sealEvidence(EvidenceReport{Records: report.Records}).Fingerprint; got != report.Fingerprint {
		return fmt.Errorf("evidence fingerprint mismatch")
	}
	return nil
}

func canonicalBindingForPhase(phase string) (PhaseRoleBinding, bool) {
	for _, binding := range canonicalBindings {
		if binding.Phase == phase {
			return binding, true
		}
	}
	return PhaseRoleBinding{}, false
}

// ValidateCompleteEvidence applies the N3 matrix gate: twelve adapters times
// three profiles, with nine accountable phase records for every cell.
func ValidateCompleteEvidence(report EvidenceReport) error {
	if err := ValidateEvidence(report); err != nil {
		return err
	}
	if len(report.Records) != 108 {
		return fmt.Errorf("complete evidence records = %d, want 108", len(report.Records))
	}
	cells := make(map[string]struct{}, 12)
	for _, record := range report.Records {
		cells[record.Cell] = struct{}{}
	}
	if len(cells) != 12 {
		return fmt.Errorf("complete evidence cells = %d, want 12", len(cells))
	}
	return nil
}

func sealEvidence(report EvidenceReport) EvidenceReport {
	report.Fingerprint = ""
	encoded, _ := json.Marshal(report)
	digest := sha256.Sum256(encoded)
	report.Fingerprint = "sha256:" + hex.EncodeToString(digest[:])
	return report
}

func sortedNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	sort.Strings(result)
	return result
}
func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
