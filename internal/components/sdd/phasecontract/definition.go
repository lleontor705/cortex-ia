package phasecontract

import "encoding/json"

// PhaseStatus is the execution outcome of a phase. It is deliberately distinct
// from VerificationVerdict and adapter/profile dispositions.
type PhaseStatus string

const (
	PhaseStatusSuccess PhaseStatus = "success"
	PhaseStatusPartial PhaseStatus = "partial"
	PhaseStatusFailed  PhaseStatus = "failed"
	PhaseStatusBlocked PhaseStatus = "blocked"
)

// VerificationVerdict is the typed result of verification, not a phase status.
type VerificationVerdict string

const (
	VerdictPass         VerificationVerdict = "pass"
	VerdictFail         VerificationVerdict = "fail"
	VerdictBlocked      VerificationVerdict = "blocked"
	VerdictInconclusive VerificationVerdict = "inconclusive"
)

const CompatibilityVersion = "1.0.0"

// CanonicalEnvelope is the versioned, executable contract boundary. The legacy
// PhaseEnvelope remains available for historical readers; new output uses this
// type and is validated before routing or persistence.
type CanonicalEnvelope struct {
	SchemaVersion   string               `json:"schema_version"`
	ContractVersion string               `json:"contract_version"`
	Phase           PhaseID              `json:"phase"`
	Role            string               `json:"role,omitempty"`
	ChangeName      string               `json:"change_name"`
	Project         string               `json:"project"`
	Status          PhaseStatus          `json:"status"`
	Confidence      float64              `json:"confidence"`
	Objective       string               `json:"objective"`
	Data            json.RawMessage      `json:"data,omitempty"`
	Stops           StopPolicy           `json:"stops"`
	TerminalStates  []string             `json:"terminal_states"`
	OutputSchema    ArtifactRef          `json:"output_schema"`
	Authority       PersistenceAuthority `json:"authority"`
	Artifacts       []ArtifactRef        `json:"artifacts,omitempty"`
	Evidence        []ArtifactRef        `json:"evidence,omitempty"`
	Retry           RetryPolicy          `json:"retry"`
	Next            []string             `json:"next,omitempty"`
	Risks           []string             `json:"risks,omitempty"`
	QualityPlanID   string               `json:"quality_plan_id,omitempty"`
	ModelRouteID    string               `json:"model_route_id,omitempty"`
	ProfileID       string               `json:"profile_id,omitempty"`
	HumanGate       string               `json:"human_gate,omitempty"`
	Observability   TraceContext         `json:"observability,omitempty"`
	Handoff         []ArtifactRef        `json:"handoff,omitempty"`
	Verdict         *VerificationVerdict `json:"verdict,omitempty"`
}

// Definitions is the compact generated-authority snapshot consumed by codegen.
type Definitions struct {
	SchemaVersion string
	Phases        []PhaseID
	Statuses      []PhaseStatus
	Verdicts      []VerificationVerdict
	Aliases       map[string]PhaseID
}

func CanonicalPhaseIDs() []PhaseID {
	return []PhaseID{PhaseInit, PhaseExplore, PhasePropose, PhaseSpec, PhaseDesign, PhaseTasks, PhaseApply, PhaseVerify, PhaseArchive}
}

func CanonicalPhaseStatuses() []PhaseStatus {
	return []PhaseStatus{PhaseStatusSuccess, PhaseStatusPartial, PhaseStatusFailed, PhaseStatusBlocked}
}

func CanonicalVerificationVerdicts() []VerificationVerdict {
	return []VerificationVerdict{VerdictPass, VerdictFail, VerdictBlocked, VerdictInconclusive}
}

func CanonicalDefinitions() Definitions {
	return Definitions{
		SchemaVersion: CompatibilityVersion,
		Phases:        CanonicalPhaseIDs(), Statuses: CanonicalPhaseStatuses(),
		Verdicts: CanonicalVerificationVerdicts(), Aliases: compatibilityAliases(),
	}
}

func compatibilityAliases() map[string]PhaseID {
	return map[string]PhaseID{
		"bootstrap": PhaseInit, "investigate": PhaseExplore, "draft-proposal": PhasePropose,
		"write-specs": PhaseSpec, "architect": PhaseDesign, "decompose": PhaseTasks,
		"implement": PhaseApply, "validate": PhaseVerify, "finalize": PhaseArchive,
	}
}
