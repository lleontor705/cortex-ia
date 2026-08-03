package phasecontract

import (
	"strings"
	"testing"
)

func TestCanonicalEnvelopeValidationRequiresVersionedFields(t *testing.T) {
	envelope := validCanonicalEnvelope()
	if err := ValidateEnvelope(envelope); err != nil {
		t.Fatalf("valid canonical envelope rejected: %v", err)
	}

	for name, mutate := range map[string]func(*CanonicalEnvelope){
		"missing schema version": func(e *CanonicalEnvelope) { e.SchemaVersion = "" },
		"invalid phase status":   func(e *CanonicalEnvelope) { e.Status = PhaseStatus("done") },
		"missing project":        func(e *CanonicalEnvelope) { e.Project = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := envelope
			mutate(&candidate)
			err := ValidateEnvelope(candidate)
			if err == nil || !strings.Contains(err.Error(), "contract/") {
				t.Fatalf("ValidateEnvelope() error = %v, want stable contract reason", err)
			}
		})
	}
}

func validCanonicalEnvelope() CanonicalEnvelope {
	return CanonicalEnvelope{
		SchemaVersion: "1.0.0", ContractVersion: "1.0.0", Phase: PhaseApply,
		Role: "implement", ChangeName: "change", Project: "project",
		Objective: "objective", Status: PhaseStatusSuccess, Confidence: 0.9,
		Stops: StopPolicy{Completion: []string{"finished"}}, TerminalStates: []string{"success", "partial", "failed", "blocked"},
		OutputSchema: ArtifactRef{SHA256: "schema", Trust: trusted},
		Authority:    PersistenceAuthority{Contracts: "forgespec", Tasks: "forgespec", Evidence: "cortex", Lineage: "cortex"},
	}
}
