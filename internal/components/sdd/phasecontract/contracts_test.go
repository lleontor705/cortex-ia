package phasecontract

import (
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// requiredEnvelopeFields are the REQ-PHASE-001 fields that every shared phase
// contract envelope MUST expose. Drift here means an invalid contract can reach
// a phase, so the test fails loudly.
var requiredEnvelopeFields = []string{
	"SchemaVersion", "Phase", "ChangeName", "Objective",
	"TrustedRefs", "UntrustedRefs", "RequiredEvidence",
	"Budget", "AllowedTools", "AllowedEffects",
	"Stops", "Retry", "TerminalStates",
	"OutputSchema", "HumanGate", "Observability",
	"Authority", "Handoff", "Status", "Confidence",
	"ModelRouteID", "ProfileID", "QualityPlanID",
}

func TestPhaseEnvelopeExposesAllRequiredFields(t *testing.T) {
	if len(requiredEnvelopeFields) < 19 {
		t.Fatalf("expected at least 19 required envelope fields, got %d", len(requiredEnvelopeFields))
	}
	ty := reflect.TypeOf(PhaseEnvelope{})
	got := make(map[string]struct{}, ty.NumField())
	for i := 0; i < ty.NumField(); i++ {
		got[ty.Field(i).Name] = struct{}{}
	}
	for _, name := range requiredEnvelopeFields {
		if _, exists := got[name]; !exists {
			t.Errorf("PhaseEnvelope missing required field %q", name)
		}
	}
}

func TestArtifactRefIncludesTrustClass(t *testing.T) {
	ty := reflect.TypeOf(ArtifactRef{})
	field, ok := ty.FieldByName("Trust")
	if !ok {
		t.Fatal("ArtifactRef missing Trust field")
	}
	if field.Type != reflect.TypeOf(ir.TrustClass("")) {
		t.Fatalf("ArtifactRef.Trust type = %v, want ir.TrustClass", field.Type)
	}
}

func TestStopPolicyHasCompletionBlockingFailure(t *testing.T) {
	ty := reflect.TypeOf(StopPolicy{})
	for _, name := range []string{"Completion", "Blocking", "Failure"} {
		if _, ok := ty.FieldByName(name); !ok {
			t.Errorf("StopPolicy missing required field %q", name)
		}
	}
}

func TestRetryPolicyHasTransientSemanticNoProgress(t *testing.T) {
	ty := reflect.TypeOf(RetryPolicy{})
	for _, name := range []string{"TransientMax", "SemanticMax", "NoProgressCycles"} {
		if _, ok := ty.FieldByName(name); !ok {
			t.Errorf("RetryPolicy missing required field %q", name)
		}
	}
}

func TestPersistenceAuthorityNamesForgespecAndCortex(t *testing.T) {
	authority := PersistenceAuthority{
		Contracts: "forgespec",
		Tasks:     "forgespec",
		Evidence:  "cortex",
		Lineage:   "cortex",
	}
	if err := authority.Validate(); err != nil {
		t.Fatalf("valid authority.Validate() error = %v", err)
	}

	for _, mutated := range []PersistenceAuthority{
		{Contracts: "", Tasks: "forgespec", Evidence: "cortex", Lineage: "cortex"},
		{Contracts: "forgespec", Tasks: "", Evidence: "cortex", Lineage: "cortex"},
		{Contracts: "forgespec", Tasks: "forgespec", Evidence: "", Lineage: "cortex"},
		{Contracts: "forgespec", Tasks: "forgespec", Evidence: "cortex", Lineage: ""},
		{Contracts: "cortex", Tasks: "forgespec", Evidence: "cortex", Lineage: "cortex"},
		{Contracts: "forgespec", Tasks: "forgespec", Evidence: "forgespec", Lineage: "cortex"},
	} {
		if err := mutated.Validate(); err == nil {
			t.Fatalf("PersistenceAuthority.Validate() error = nil for %+v, want rejection", mutated)
		}
	}
}

func TestPhaseEnvelopeValidateRejectsUntrustedOverrideInTrustedRefs(t *testing.T) {
	trustedRef := ArtifactRef{
		SHA256:   "abc",
		Required: true,
		Trust:    ir.TrustTrustedPolicy,
	}
	if err := trustedRef.Validate(); err != nil {
		t.Fatalf("trusted ArtifactRef.Validate() error = %v", err)
	}

	for _, untrusted := range []ir.TrustClass{
		ir.TrustRepositoryData,
		ir.TrustToolOutput,
		ir.TrustPeerMessage,
		ir.TrustRemoteUntrusted,
		ir.TrustSecretReference,
	} {
		t.Run(string(untrusted), func(t *testing.T) {
			ref := ArtifactRef{SHA256: "def", Required: true, Trust: untrusted}
			if err := ref.Validate(); err == nil {
				t.Fatalf("ArtifactRef with untrusted class %q Validate() error = nil, want rejection", untrusted)
			}
		})
	}

	envelope := validEnvelope()
	envelope.TrustedRefs = append(envelope.TrustedRefs, ArtifactRef{
		SHA256: "leak", Required: true, Trust: ir.TrustPeerMessage,
	})
	if err := envelope.Validate(); err == nil {
		t.Fatal("envelope with untrusted ref in TrustedRefs Validate() error = nil, want rejection")
	}
}

func TestPhaseEnvelopeValidateRejectsIncompleteContract(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*PhaseEnvelope)
	}{
		{name: "empty phase", mutate: func(e *PhaseEnvelope) { e.Phase = "" }},
		{name: "empty objective", mutate: func(e *PhaseEnvelope) { e.Objective = "" }},
		{name: "no terminal states", mutate: func(e *PhaseEnvelope) { e.TerminalStates = nil }},
		{name: "no completion stops", mutate: func(e *PhaseEnvelope) { e.Stops.Completion = nil }},
		{name: "no output schema", mutate: func(e *PhaseEnvelope) { e.OutputSchema = ArtifactRef{} }},
		{name: "confidence above one", mutate: func(e *PhaseEnvelope) { e.Confidence = 1.5 }},
		{name: "negative confidence", mutate: func(e *PhaseEnvelope) { e.Confidence = -0.1 }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validEnvelope()
			tt.mutate(&envelope)
			if err := envelope.Validate(); err == nil {
				t.Fatalf("Validate() error = nil for %s", tt.name)
			}
		})
	}

	if err := validEnvelope().Validate(); err != nil {
		t.Fatalf("valid envelope.Validate() error = %v", err)
	}
}

func validEnvelope() PhaseEnvelope {
	return PhaseEnvelope{
		SchemaVersion:    "1.0.0",
		Phase:            "phase/apply",
		ChangeName:       "improve-agent-phase-workflows",
		Objective:        "Deliver one bounded work unit",
		TrustedRefs:      []ArtifactRef{{SHA256: "t1", Required: true, Trust: ir.TrustTrustedPolicy}},
		RequiredEvidence: []string{"focused-test", "diff-review"},
		Budget:           Budget{MaxOutputTokens: 3500, MaxFileReads: 8, MaxToolCalls: 10},
		AllowedTools:     []string{"tool/test/run"},
		AllowedEffects:   []string{"repository/write"},
		Stops:            StopPolicy{Completion: []string{"done"}, Blocking: []string{"blocked"}, Failure: []string{"failed"}},
		Retry:            RetryPolicy{TransientMax: 3, SemanticMax: 2, NoProgressCycles: 2},
		TerminalStates:   []string{"done", "blocked", "failed"},
		OutputSchema:     ArtifactRef{SHA256: "schema1", Required: true, Trust: ir.TrustTrustedSchema},
		HumanGate:        "none",
		Observability:    TraceContext{TraceID: "trace-1"},
		Authority:        PersistenceAuthority{Contracts: "forgespec", Tasks: "forgespec", Evidence: "cortex", Lineage: "cortex"},
		Handoff:          []ArtifactRef{{SHA256: "h1", Required: true, Trust: ir.TrustTrustedSchema}},
		Status:           "pending",
		Confidence:       0.6,
		ModelRouteID:     "model/default",
		ProfileID:        "profile/portable-sequential",
	}
}
