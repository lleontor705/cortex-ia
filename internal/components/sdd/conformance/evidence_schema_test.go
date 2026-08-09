package conformance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceSchemaAggregatesNineBindingsForEveryMatrixCell(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := LoadMatrix(data)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := AggregateEvidence(matrix, EvidenceOptions{
		ContractVersion:     "1.0.0",
		ContractFingerprint: "sha256:contract",
		PrimaryModel:        "primary",
		FallbackModel:       "fallback",
		QualityPlan:         "quality/required",
		TrustEvidence:       []string{"trust:qualified"},
		Permissions:         []string{"read:workspace"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCompleteEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	if got := len(evidence.Records); got != 108 {
		t.Fatalf("records = %d, want 108", got)
	}
	wantBindings := []PhaseRoleBinding{
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
	for index, want := range wantBindings {
		if got := evidence.Records[index].Role; got != want {
			t.Errorf("binding %d = %+v, want %+v", index, got, want)
		}
	}
	for _, record := range evidence.Records {
		if record.Role.Phase == "" || record.Role.Skill == "" || record.SemanticAssets == nil ||
			record.Contract.Version == "" || record.Contract.Fingerprint == "" ||
			record.Model.Primary == "" || record.Model.Fallback == "" ||
			record.QualityPlan == "" || record.TrustEvidence == nil || record.Permissions == nil ||
			record.Destination == "" || record.Disposition == "" {
			t.Fatalf("incomplete record: %+v", record)
		}
		if record.Role.Phase == "init" || record.Role.Phase == "explore" {
			t.Fatalf("legacy phase leaked into evidence: %+v", record.Role)
		}
	}
}

func TestEvidenceSchemaKeepsDegradedAndRejectedCellsAccountable(t *testing.T) {
	matrix := Matrix{
		Adapters: []string{"degraded", "rejected"},
		Profiles: []string{"portable-sequential"},
		Cells: []Cell{
			{Adapter: "degraded", RequestedProfile: "portable-sequential", EffectiveProfile: "portable-flat", Disposition: DispositionDegraded, ReasonID: "profile/degraded/test", Command: "probe", Hash: "hash", Evidence: map[string]string{"mutation": "none", "pre_mutation": "proven"}},
			{Adapter: "rejected", RequestedProfile: "portable-sequential", EffectiveProfile: "portable-sequential", Disposition: DispositionRejected, ReasonID: "profile/rejected/test", Command: "probe", ExitCode: 1, Hash: "hash", Evidence: map[string]string{"mutation": "none", "pre_mutation": "proven"}},
		},
	}
	evidence, err := AggregateEvidence(matrix, EvidenceOptions{ContractVersion: "1.0.0", ContractFingerprint: "fp", PrimaryModel: "p", FallbackModel: "f", QualityPlan: "q", TrustEvidence: []string{"t"}, Permissions: []string{"r"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	for _, record := range evidence.Records {
		if record.Disposition != DispositionDegraded && record.Disposition != DispositionRejected {
			t.Fatalf("unexpected disposition: %+v", record)
		}
		if record.Accountability != AccountabilityEmitted && record.Accountability != AccountabilityPreMutation {
			t.Fatalf("missing accountability: %+v", record)
		}
	}
}

func TestEvidenceSchemaFingerprintIsDeterministicAcrossRuns(t *testing.T) {
	matrix := minimalMatrix()
	first, err := AggregateEvidence(matrix, defaultEvidenceOptions())
	if err != nil {
		t.Fatal(err)
	}
	second, err := AggregateEvidence(matrix, defaultEvidenceOptions())
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprints differ: %q != %q", first.Fingerprint, second.Fingerprint)
	}
}

func minimalMatrix() Matrix {
	return Matrix{Adapters: []string{"adapter"}, Profiles: []string{"profile"}, Cells: []Cell{{Adapter: "adapter", RequestedProfile: "profile", EffectiveProfile: "profile", Disposition: DispositionSupported, ReasonID: "supported", Command: "probe", Hash: "hash", Evidence: map[string]string{"mutation": "none", "pre_mutation": "n/a"}}}}
}

func defaultEvidenceOptions() EvidenceOptions {
	return EvidenceOptions{ContractVersion: "1.0.0", ContractFingerprint: "contract", PrimaryModel: "primary", FallbackModel: "fallback", QualityPlan: "quality", TrustEvidence: []string{"trust"}, Permissions: []string{"read"}}
}
