package pipeline

import (
	"slices"

	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

// WorkflowMetadata is the immutable, adapter-neutral evidence carrier for a
// prepared workflow. It is cloned at each boundary so renderers and installers
// cannot silently replace qualified values with defaults.
type WorkflowMetadata struct {
	ContractFingerprint string                              `json:"contract_fingerprint,omitempty"`
	ProfileRequested    string                              `json:"profile_requested,omitempty"`
	ProfileEffective    string                              `json:"profile_effective,omitempty"`
	PrimaryModel        string                              `json:"primary_model,omitempty"`
	FallbackModel       string                              `json:"fallback_model,omitempty"`
	QualityPlanID       string                              `json:"quality_plan_id,omitempty"`
	ProfileReasonID     string                              `json:"profile_reason_id,omitempty"`
	TrustEvidence       []string                            `json:"trust_evidence,omitempty"`
	Permissions         []string                            `json:"permissions,omitempty"`
	HumanGate           string                              `json:"human_gate,omitempty"`
	Observability       string                              `json:"observability,omitempty"`
	Routes              map[string]modelroute.ResolvedRoute `json:"routes,omitempty"`
}

func (m WorkflowMetadata) Clone() WorkflowMetadata {
	m.TrustEvidence = slices.Clone(m.TrustEvidence)
	m.Permissions = slices.Clone(m.Permissions)
	if m.Routes != nil {
		m.Routes = make(map[string]modelroute.ResolvedRoute, len(m.Routes))
		for key, route := range m.Routes {
			route.Evidence = slices.Clone(route.Evidence)
			route.Constraints = slices.Clone(route.Constraints)
			m.Routes[key] = route
		}
	}
	return m
}
