package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// PressureRunKind identifies the deterministic comparison arm of a pressure
// scenario. Runs carry semantic outcomes; model prose is deliberately absent.
type PressureRunKind string

const (
	PressureRunBaseline  PressureRunKind = "baseline"
	PressureRunControl   PressureRunKind = "control"
	PressureRunWithSkill PressureRunKind = "with-skill"
	PressureRunMicro     PressureRunKind = "micro"
	PressureRunRerun     PressureRunKind = "rerun"
)

type PressureDisposition string

const (
	PressureDispositionPass         PressureDisposition = "pass"
	PressureDispositionFail         PressureDisposition = "fail"
	PressureDispositionInconclusive PressureDisposition = "inconclusive"
	PressureDispositionPartial      PressureDisposition = "partial"
)

type PressureFailureCategory string

const (
	FailureMissingBaseline   PressureFailureCategory = "missing-baseline"
	FailureRationalization   PressureFailureCategory = "rationalization"
	FailureWrongSemantic     PressureFailureCategory = "wrong-semantic-outcome"
	FailureNondeterministic  PressureFailureCategory = "nondeterministic-replay"
	FailureControlRegression PressureFailureCategory = "control-regression"
)

type EvidenceRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type PressureRun struct {
	Kind               PressureRunKind `json:"kind"`
	ReplayInputDigest  string          `json:"replay_input_digest"`
	SemanticOutcome    string          `json:"semantic_outcome"`
	ObservedFailure    bool            `json:"observed_failure"`
	ExpectedFailure    bool            `json:"expected_failure"`
	InconclusiveReason string          `json:"inconclusive_reason,omitempty"`
	EvidenceRefs       []EvidenceRef   `json:"evidence_refs,omitempty"`
}

type PressureScenario struct {
	ScenarioID        string                    `json:"scenario_id"`
	ExpectedInvariant string                    `json:"expected_invariant"`
	ReplayInputDigest string                    `json:"replay_input_digest"`
	Baseline          PressureRun               `json:"baseline"`
	Controls          []PressureRun             `json:"controls"`
	WithSkill         PressureRun               `json:"with_skill"`
	MicroTests        []PressureRun             `json:"micro_tests"`
	Reruns            []PressureRun             `json:"reruns"`
	FailureCategories []PressureFailureCategory `json:"failure_categories,omitempty"`
	Rationalizations  []string                  `json:"rationalizations,omitempty"`
	ObservedOutcome   string                    `json:"observed_outcome"`
}

type PressureReplay struct {
	ReplayCount             int  `json:"replay_count"`
	Stable                  bool `json:"stable"`
	ZeroUnexplainedVariance bool `json:"zero_unexplained_variance"`
}

type PressureEnvironment struct {
	OS                string `json:"os"`
	GoVersion         string `json:"go_version"`
	ExternalEvaluator bool   `json:"external_evaluator"`
}

type SeededControls struct {
	ByClass map[string]int `json:"by_class"`
}

type PromotionCriteria struct {
	ReportOnlyCycles        int            `json:"report_only_cycles"`
	RequiredCycles          int            `json:"required_cycles"`
	StableReplays           int            `json:"stable_replays"`
	RequiredReplays         int            `json:"required_replays"`
	ZeroUnexplainedVariance bool           `json:"zero_unexplained_variance"`
	CompleteAttribution     bool           `json:"complete_attribution"`
	CriticalHighDefects     int            `json:"critical_high_defects"`
	SeededControlsByClass   map[string]int `json:"seeded_controls_by_class"`
	ApprovedPolicyRevision  string         `json:"approved_policy_revision"`
}

type PromotionReason string

const (
	PromotionReasonEligible        PromotionReason = "eligible"
	PromotionReasonApprovalMissing PromotionReason = "approval_missing"
	PromotionReasonEvidenceMissing PromotionReason = "evidence_missing"
)

type PromotionDecision struct {
	Eligible bool            `json:"eligible"`
	Reason   PromotionReason `json:"reason"`
	Missing  []string        `json:"missing,omitempty"`
}

// SkillPressureReport is report-only evidence. It cannot alter task
// readiness, quality gates, R7, or B7 status in this policy revision.
type SkillPressureReport struct {
	SchemaVersion     string                    `json:"schema_version"`
	ScenarioID        string                    `json:"scenario_id,omitempty"`
	SkillID           string                    `json:"skill_id"`
	SkillVersion      string                    `json:"skill_version"`
	SkillSHA256       string                    `json:"skill_sha256"`
	HarnessRevision   string                    `json:"harness_revision"`
	ProfileID         string                    `json:"profile_id"`
	PolicySHA256      string                    `json:"policy_sha256"`
	ExpectedInvariant string                    `json:"expected_invariant,omitempty"`
	ObservedOutcome   string                    `json:"observed_outcome,omitempty"`
	FailureCategories []PressureFailureCategory `json:"failure_categories,omitempty"`
	Rationalizations  []string                  `json:"rationalizations,omitempty"`
	EvidenceRefs      []EvidenceRef             `json:"evidence_refs"`
	Reproducibility   PressureReplay            `json:"reproducibility"`
	Confidence        float64                   `json:"confidence"`
	Environment       PressureEnvironment       `json:"environment"`
	Scenarios         []PressureScenario        `json:"scenarios"`
	Disposition       PressureDisposition       `json:"disposition"`
	Promotion         PromotionDecision         `json:"promotion"`
	SeededControls    SeededControls            `json:"seeded_controls"`
}

func (r SkillPressureReport) Validate() error {
	if r.SchemaVersion == "" || r.SkillID == "" || r.SkillVersion == "" || r.SkillSHA256 == "" || r.HarnessRevision == "" || r.ProfileID == "" || r.PolicySHA256 == "" {
		return fmt.Errorf("skill pressure report requires schema, skill, harness, profile, and policy identity")
	}
	if len(r.Scenarios) < 3 {
		return fmt.Errorf("skill pressure report requires at least three scenarios")
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("skill pressure confidence must be between 0 and 1")
	}
	if len(r.EvidenceRefs) == 0 || r.Environment.OS == "" || r.Environment.GoVersion == "" {
		return fmt.Errorf("skill pressure report requires evidence references and environment")
	}
	seen := map[string]struct{}{}
	for _, scenario := range r.Scenarios {
		if err := scenario.validate(); err != nil {
			return fmt.Errorf("scenario %q: %w", scenario.ScenarioID, err)
		}
		if _, ok := seen[scenario.ScenarioID]; ok {
			return fmt.Errorf("duplicate scenario %q", scenario.ScenarioID)
		}
		seen[scenario.ScenarioID] = struct{}{}
	}
	return nil
}

func (s PressureScenario) validate() error {
	if s.ScenarioID == "" || s.ExpectedInvariant == "" || s.ReplayInputDigest == "" || s.ObservedOutcome == "" {
		return fmt.Errorf("scenario identity, invariant, replay input, and observed outcome are required")
	}
	if s.Baseline.Kind != PressureRunBaseline || s.Baseline.ReplayInputDigest != s.ReplayInputDigest || s.Baseline.SemanticOutcome == "" {
		return fmt.Errorf("baseline run is incomplete")
	}
	if !s.Baseline.ExpectedFailure {
		if s.Baseline.InconclusiveReason == "" {
			return fmt.Errorf("baseline must demonstrate expected failure or declare an inconclusive precondition")
		}
	} else if !s.Baseline.ObservedFailure {
		return fmt.Errorf("baseline expected failure was not observed")
	}
	if len(s.Controls) == 0 || len(s.MicroTests) == 0 || len(s.Reruns) == 0 {
		return fmt.Errorf("control, micro-test, and rerun runs are required")
	}
	if err := validateRuns(s.Controls, PressureRunControl, s.ReplayInputDigest); err != nil {
		return err
	}
	if err := validateRuns(s.MicroTests, PressureRunMicro, s.ReplayInputDigest); err != nil {
		return err
	}
	if err := validateRuns(s.Reruns, PressureRunRerun, s.ReplayInputDigest); err != nil {
		return err
	}
	if s.WithSkill.Kind != PressureRunWithSkill || s.WithSkill.ReplayInputDigest != s.ReplayInputDigest || s.WithSkill.SemanticOutcome == "" {
		return fmt.Errorf("with-skill run is incomplete")
	}
	return nil
}

func validateRuns(runs []PressureRun, kind PressureRunKind, inputDigest string) error {
	for _, run := range runs {
		if run.Kind != kind || run.ReplayInputDigest != inputDigest || run.SemanticOutcome == "" {
			return fmt.Errorf("%s run has mismatched replay input or semantic outcome", kind)
		}
	}
	return nil
}

func (r SkillPressureReport) BlockingImpact() bool { return false }

func (r SkillPressureReport) CanonicalDigest() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

type PressureRunRequest struct{ RunID string }
type PressureRunOutcome struct {
	SemanticOutcome string
	ObservedFailure bool
}

type PressureRunner interface {
	Run(context.Context, PressureRunRequest) (PressureRunOutcome, error)
}

// DeterministicPressureRunner is a fixture runner for CI. It has no network,
// tmux, or live-model dependency; external evaluators can implement the same
// interface behind an explicitly marked adapter.
type DeterministicPressureRunner struct{ Outcomes map[string]PressureRunOutcome }

func (r DeterministicPressureRunner) Run(_ context.Context, request PressureRunRequest) (PressureRunOutcome, error) {
	outcome, ok := r.Outcomes[request.RunID]
	if !ok || outcome.SemanticOutcome == "" {
		return PressureRunOutcome{}, fmt.Errorf("missing deterministic outcome for %q", request.RunID)
	}
	return outcome, nil
}

func EvaluateSkillPressurePromotion(criteria PromotionCriteria) PromotionDecision {
	missing := []string{}
	if criteria.ReportOnlyCycles < criteria.RequiredCycles {
		missing = append(missing, "release_cycles")
	}
	if criteria.StableReplays < criteria.RequiredReplays {
		missing = append(missing, "stable_replays")
	}
	if !criteria.ZeroUnexplainedVariance {
		missing = append(missing, "zero_unexplained_variance")
	}
	if !criteria.CompleteAttribution {
		missing = append(missing, "attribution")
	}
	if criteria.CriticalHighDefects != 0 {
		missing = append(missing, "critical_high_defects")
	}
	classes := make([]string, 0, len(criteria.SeededControlsByClass))
	for class := range criteria.SeededControlsByClass {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	for _, class := range classes {
		if criteria.SeededControlsByClass[class] < 1 {
			missing = append(missing, "seeded_control:"+class)
		}
	}
	if criteria.ApprovedPolicyRevision == "" {
		return PromotionDecision{Reason: PromotionReasonApprovalMissing, Missing: append(missing, "approved_policy_revision")}
	}
	if len(missing) > 0 {
		return PromotionDecision{Reason: PromotionReasonEvidenceMissing, Missing: missing}
	}
	return PromotionDecision{Eligible: true, Reason: PromotionReasonEligible}
}
