// Package phasecontract defines the shared and phase-specific envelope types
// that govern every SDD role's inputs, budgets, stops, evidence, and handoff.
//
// These types make invalid phase contracts unrepresentable: an envelope without
// terminal states, completion stops, an output schema, or a valid persistence
// authority cannot validate, and an untrusted artifact reference can never appear
// among trusted references.
package phasecontract

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// ArtifactRef is a versioned reference to a contract, topic, or schema with an
// explicit trust class. Untrusted references belong only in UntrustedRefs.
type ArtifactRef struct {
	ContractID string        `json:"contract_id,omitempty"`
	TopicKey   string        `json:"topic_key,omitempty"`
	SHA256     string        `json:"sha256"`
	Required   bool          `json:"required"`
	Trust      ir.TrustClass `json:"trust"`
}

// Validate requires a fingerprint and a recognized trust class, and rejects any
// reference that places an untrusted source among trusted references.
func (r ArtifactRef) Validate() error {
	if r.SHA256 == "" {
		return fmt.Errorf("artifact reference requires a SHA256 fingerprint")
	}
	if !IsTrustedSource(r.Trust) {
		return fmt.Errorf("trusted references must not include untrusted class %q", r.Trust)
	}
	return nil
}

// IsTrustedSource reports whether a trust class may appear among trusted
// references. Only installed policy and schema carry authority; repository,
// tool, peer, remote, and secret content are always untrusted evidence.
func IsTrustedSource(trust ir.TrustClass) bool {
	switch trust {
	case ir.TrustTrustedPolicy, ir.TrustTrustedSchema:
		return true
	default:
		return false
	}
}

// Budget bounds the context, tool, and token consumption a phase may incur.
// Zero means the dimension is not bounded for that phase.
type Budget struct {
	MaxInputTokens    int `json:"max_input_tokens,omitempty"`
	MaxOutputTokens   int `json:"max_output_tokens,omitempty"`
	MaxFileReads      int `json:"max_file_reads,omitempty"`
	MaxToolCalls      int `json:"max_tool_calls,omitempty"`
	CheckpointAtFiles int `json:"checkpoint_at_files,omitempty"`
}

// StopPolicy enumerates the completion, blocking, and failure terminal
// conditions. A valid envelope always declares at least one completion state.
type StopPolicy struct {
	Completion []string `json:"completion"`
	Blocking   []string `json:"blocking"`
	Failure    []string `json:"failure"`
}

// RetryPolicy bounds transient retries, semantic retries with reflection, and
// the no-progress cycles that trigger runaway halt.
type RetryPolicy struct {
	TransientMax     int   `json:"transient_max"`
	SemanticMax      int   `json:"semantic_max"`
	BackoffSeconds   []int `json:"backoff_seconds,omitempty"`
	NoProgressCycles int   `json:"no_progress_cycles"`
}

// PersistenceAuthority names the single owner of each persistence concern.
// ForgeSpec owns contracts, tasks, and DAG/readiness/CAS state; Cortex owns
// evidence, reflection, and lineage. Any other authority is invalid.
type PersistenceAuthority struct {
	Contracts string `json:"contracts"`
	Tasks     string `json:"tasks"`
	Evidence  string `json:"evidence"`
	Lineage   string `json:"lineage"`
}

// Validate enforces that ForgeSpec owns contracts and tasks while Cortex owns
// evidence and lineage, so no second mutable authority can ever appear.
func (a PersistenceAuthority) Validate() error {
	if a.Contracts != "forgespec" {
		return fmt.Errorf("contracts authority must be forgespec, got %q", a.Contracts)
	}
	if a.Tasks != "forgespec" {
		return fmt.Errorf("tasks authority must be forgespec, got %q", a.Tasks)
	}
	if a.Evidence != "cortex" {
		return fmt.Errorf("evidence authority must be cortex, got %q", a.Evidence)
	}
	if a.Lineage != "cortex" {
		return fmt.Errorf("lineage authority must be cortex, got %q", a.Lineage)
	}
	return nil
}

// TraceContext captures the observability surface a phase records. Events
// redact prompts, credentials, and secret markers; raw content is never emitted.
type TraceContext struct {
	TraceID    string   `json:"trace_id,omitempty"`
	SpanID     string   `json:"span_id,omitempty"`
	EventKinds []string `json:"event_kinds,omitempty"`
	Redacts    []string `json:"redacts,omitempty"`
}

// PhaseEnvelope is the complete shared contract envelope required by
// REQ-PHASE-001. Every field is observable; an envelope missing any required
// field, terminal state, completion stop, or output schema is invalid.
type PhaseEnvelope struct {
	SchemaVersion    string               `json:"schema_version"`
	Phase            string               `json:"phase"`
	ChangeName       string               `json:"change_name"`
	Objective        string               `json:"objective"`
	TrustedRefs      []ArtifactRef        `json:"trusted_refs"`
	UntrustedRefs    []ArtifactRef        `json:"untrusted_refs,omitempty"`
	RequiredEvidence []string             `json:"required_evidence"`
	Budget           Budget               `json:"budget"`
	AllowedTools     []string             `json:"allowed_tools"`
	AllowedEffects   []string             `json:"allowed_effects"`
	Stops            StopPolicy           `json:"stops"`
	Retry            RetryPolicy          `json:"retry"`
	TerminalStates   []string             `json:"terminal_states"`
	OutputSchema     ArtifactRef          `json:"output_schema"`
	HumanGate        string               `json:"human_gate"`
	Observability    TraceContext         `json:"observability"`
	Authority        PersistenceAuthority `json:"authority"`
	Handoff          []ArtifactRef        `json:"handoff,omitempty"`
	Status           string               `json:"status"`
	Confidence       float64              `json:"confidence"`
	ModelRouteID     string               `json:"model_route_id"`
	ProfileID        string               `json:"profile_id"`
	QualityPlanID    string               `json:"quality_plan_id,omitempty"`
}

// Validate enforces the full REQ-PHASE-001 contract: a non-empty phase and
// objective, declared terminal states, at least one completion stop, a valid
// output schema, bounded confidence, correct persistence authority, and that no
// untrusted reference appears among trusted references or handoff.
func (e PhaseEnvelope) Validate() error {
	if e.Phase == "" {
		return fmt.Errorf("phase envelope requires a phase")
	}
	if e.Objective == "" {
		return fmt.Errorf("phase envelope requires an objective")
	}
	if len(e.TerminalStates) == 0 {
		return fmt.Errorf("phase envelope requires terminal states")
	}
	if len(e.Stops.Completion) == 0 {
		return fmt.Errorf("phase envelope requires at least one completion stop")
	}
	if err := e.OutputSchema.Validate(); err != nil {
		return fmt.Errorf("output schema: %w", err)
	}
	if e.Confidence < 0 || e.Confidence > 1 {
		return fmt.Errorf("confidence %v must be within [0,1]", e.Confidence)
	}
	if err := e.Authority.Validate(); err != nil {
		return err
	}
	for _, ref := range e.TrustedRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("trusted ref: %w", err)
		}
	}
	for _, ref := range e.Handoff {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("handoff ref: %w", err)
		}
	}
	return nil
}
