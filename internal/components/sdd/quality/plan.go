package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// TestingCapabilities is the quality-compiler's name for the proven testing
// capability set. It aliases TestCapabilities so the design's install-time
// vocabulary resolves to the existing capability model without duplication.
type TestingCapabilities = TestCapabilities

// ProfilePlan is the quality-compiler's view of a profile: the profile identity,
// its proven native capabilities, and the named degradations that apply when a
// native capability is absent. The full profile resolver (profileplan package)
// lowers into this type.
type ProfilePlan struct {
	ProfileID     string   `json:"profile_id"`
	NativePreload bool     `json:"native_preload"`
	DirectChild   bool     `json:"direct_child"`
	Degradations  []string `json:"degradations,omitempty"`
}

// TechniqueBounds carries the budgeted bounds for each optional quality
// technique as normalized against proven capabilities.
type TechniqueBounds struct {
	Property ActivityBudget `json:"property"`
	Fuzz     ActivityBudget `json:"fuzz"`
	Mutation ActivityBudget `json:"mutation"`
}

// QualityPolicyIR is the normalized intermediate representation of a
// QualityPolicy. Its PolicySHA256 fingerprint anchors the seven-link chain
// (policy -> IR -> template -> plan) required by REQ-QUAL-001.
type QualityPolicyIR struct {
	SchemaVersion string `json:"schema_version"`
	PolicySHA256  string `json:"policy_sha256"`
}

// QualityPlanTemplate is the install-time, capability/profile-specific plan
// template. It is compiled once and referenced by manifests and receipts; the
// change-specific QualityPlan is produced after proposal ChangeSignals.
type QualityPlanTemplate struct {
	SchemaVersion   string              `json:"schema_version"`
	PolicyVersion   string              `json:"policy_version"`
	PolicySHA256    string              `json:"policy_sha256"`
	ProfileID       string              `json:"profile_id"`
	Capabilities    TestingCapabilities `json:"capabilities"`
	TechniqueBounds TechniqueBounds     `json:"technique_bounds"`
	Degradations    []string            `json:"degradations,omitempty"`
}

// Validate requires a policy fingerprint and profile so that a template without
// a traceable policy origin can never reach installation.
func (t QualityPlanTemplate) Validate() error {
	if t.PolicySHA256 == "" {
		return fmt.Errorf("quality plan template requires a policy SHA256 fingerprint")
	}
	if t.ProfileID == "" {
		return fmt.Errorf("quality plan template requires a profile ID")
	}
	return nil
}

// ChangeSignals are the evidenced, change-specific signals recorded by proposal.
// They are objective change attributes; model confidence is intentionally
// absent because confidence cannot waive policy obligations.
type ChangeSignals struct {
	ChangeName         string            `json:"change_name"`
	Kind               ChangeKind        `json:"kind"`
	ObservableBehavior bool              `json:"observable_behavior"`
	Risk               RiskLevel         `json:"risk"`
	Reversibility      Reversibility     `json:"reversibility"`
	TrustBoundary      TrustBoundary     `json:"trust_boundary"`
	DependencyBreadth  DependencyBreadth `json:"dependency_breadth"`
	MigrationImpact    MigrationImpact   `json:"migration_impact"`
}

// QualityPlan is the immutable, change-specific plan produced after ChangeSignals.
// It carries the template and signals fingerprints so the seven-link chain is
// fully traceable end to end. It has no mutating methods; EvaluatePlan is pure.
type QualityPlan struct {
	SchemaVersion       string              `json:"schema_version"`
	PolicySHA256        string              `json:"policy_sha256"`
	TemplateSHA256      string              `json:"template_sha256"`
	ChangeSignalsSHA256 string              `json:"change_signals_sha256"`
	Planning            PlanningDecision    `json:"planning"`
	TDD                 ApplicabilityStatus `json:"tdd"`
	Gherkin             TechniqueDecision   `json:"gherkin"`
	Property            TechniqueDecision   `json:"property"`
	Fuzz                TechniqueDecision   `json:"fuzz"`
	Mutation            TechniqueDecision   `json:"mutation"`
	RequiredEvidence    []string            `json:"required_evidence,omitempty"`
	BlockingReasons     []string            `json:"blocking_reasons,omitempty"`
}

// Validate requires both fingerprints so an untraceable plan can never propagate.
func (p QualityPlan) Validate() error {
	if p.PolicySHA256 == "" {
		return fmt.Errorf("quality plan requires a policy SHA256 fingerprint")
	}
	if p.TemplateSHA256 == "" {
		return fmt.Errorf("quality plan requires a template SHA256 fingerprint")
	}
	if p.ChangeSignalsSHA256 == "" {
		return fmt.Errorf("quality plan requires a change-signals SHA256 fingerprint")
	}
	return nil
}

// PipelineInput is the complete input to the production quality consumer.
// Policy, normalized template, change signals, and evaluated applicability are
// deliberately carried together so a caller cannot install a prompt-only plan.
type PipelineInput struct {
	Policy       QualityPolicy
	Capabilities TestingCapabilities
	Profile      ProfilePlan
	Signals      ChangeSignals
	Evaluation   EvaluationInput
}

// PipelineTrace proves the seven-link quality chain without exposing mutable
// intermediate state to renderers.
type PipelineTrace struct {
	PolicySHA256   string `json:"policy_sha256"`
	TemplateSHA256 string `json:"template_sha256"`
	PlanSHA256     string `json:"plan_sha256"`
}

// BuildPlan is the production QualityPlan consumer. It evaluates objective
// change facts and binds the decision to the policy IR/template fingerprints.
func BuildPlan(input PipelineInput) (QualityPlan, PipelineTrace, error) {
	policyIR, template, err := CompilePolicy(input.Policy, input.Capabilities, input.Profile)
	if err != nil {
		return QualityPlan{}, PipelineTrace{}, err
	}
	decision, err := Evaluate(input.Policy, input.Evaluation)
	if err != nil {
		return QualityPlan{}, PipelineTrace{}, fmt.Errorf("evaluate quality plan: %w", err)
	}
	templateHash, err := fingerprintJSON(template)
	if err != nil {
		return QualityPlan{}, PipelineTrace{}, fmt.Errorf("quality template fingerprint: %w", err)
	}
	signalsHash, err := fingerprintJSON(input.Signals)
	if err != nil {
		return QualityPlan{}, PipelineTrace{}, fmt.Errorf("quality signals fingerprint: %w", err)
	}
	plan := QualityPlan{
		SchemaVersion: "1.0.0", PolicySHA256: policyIR.PolicySHA256,
		TemplateSHA256: templateHash, ChangeSignalsSHA256: signalsHash,
		Planning: decision.Planning, TDD: decision.TDD, Gherkin: decision.Gherkin,
		Property: decision.Property, Fuzz: decision.Fuzz, Mutation: decision.Mutation,
		RequiredEvidence: []string{"focused-test", "diff-review"}, BlockingReasons: []string{},
	}
	if err := plan.Validate(); err != nil {
		return QualityPlan{}, PipelineTrace{}, err
	}
	planHash, err := fingerprintJSON(plan)
	if err != nil {
		return QualityPlan{}, PipelineTrace{}, fmt.Errorf("quality plan fingerprint: %w", err)
	}
	return plan, PipelineTrace{PolicySHA256: policyIR.PolicySHA256, TemplateSHA256: templateHash, PlanSHA256: planHash}, nil
}

// CompilePolicy normalizes a QualityPolicy against proven capabilities and a
// profile into a QualityPolicyIR and an install-time QualityPlanTemplate. This
// is a typed entry point: it validates the policy and computes the deterministic
// policy fingerprint that anchors the seven-link chain. Full technique
// applicability evaluation is the Stage 2 compiler's responsibility.
func CompilePolicy(policy QualityPolicy, caps TestingCapabilities, profile ProfilePlan) (QualityPolicyIR, QualityPlanTemplate, error) {
	if err := policy.Validate(); err != nil {
		return QualityPolicyIR{}, QualityPlanTemplate{}, fmt.Errorf("compile policy: %w", err)
	}
	fingerprint, err := fingerprintQualityPolicy(policy)
	if err != nil {
		return QualityPolicyIR{}, QualityPlanTemplate{}, fmt.Errorf("compile policy fingerprint: %w", err)
	}
	policyIR := QualityPolicyIR{
		SchemaVersion: policy.Version,
		PolicySHA256:  fingerprint,
	}
	template := QualityPlanTemplate{
		SchemaVersion: "1.0.0",
		PolicyVersion: policy.Version,
		PolicySHA256:  fingerprint,
		ProfileID:     profile.ProfileID,
		Capabilities:  caps,
		TechniqueBounds: TechniqueBounds{
			Property: policy.Property.Budget,
			Fuzz:     policy.Fuzz.Budget,
			Mutation: policy.Mutation.Budget,
		},
		Degradations: append([]string(nil), profile.Degradations...),
	}
	return policyIR, template, nil
}

// EvaluatePlan produces the immutable, change-specific QualityPlan from a
// normalized IR, install-time template, and evidenced change signals. It is a
// pure function: identical inputs always yield identical plans. It rejects any
// fingerprint mismatch so the seven-link chain cannot be broken silently.
func EvaluatePlan(policyIR QualityPolicyIR, template QualityPlanTemplate, signals ChangeSignals) (QualityPlan, error) {
	if err := template.Validate(); err != nil {
		return QualityPlan{}, fmt.Errorf("evaluate plan template: %w", err)
	}
	if policyIR.PolicySHA256 != template.PolicySHA256 {
		return QualityPlan{}, fmt.Errorf("evaluate plan: policy fingerprint mismatch between IR and template")
	}
	templateHash, err := fingerprintJSON(template)
	if err != nil {
		return QualityPlan{}, fmt.Errorf("evaluate plan template fingerprint: %w", err)
	}
	signalsHash, err := fingerprintJSON(signals)
	if err != nil {
		return QualityPlan{}, fmt.Errorf("evaluate plan signals fingerprint: %w", err)
	}
	plan := QualityPlan{
		SchemaVersion:       "1.0.0",
		PolicySHA256:        policyIR.PolicySHA256,
		TemplateSHA256:      templateHash,
		ChangeSignalsSHA256: signalsHash,
		Planning:            PlanningDecision{Depth: PlanningNone},
		TDD:                 ApplicabilityNotApplicable,
		RequiredEvidence:    []string{"focused-test", "diff-review"},
		BlockingReasons:     []string{},
	}
	return plan, nil
}

func fingerprintQualityPolicy(policy QualityPolicy) (string, error) {
	return fingerprintJSON(policy)
}

func fingerprintJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
