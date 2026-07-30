package ir

import (
	"encoding/json"
	"testing"
)

func TestWorkflowIRRepresentsCanonicalSemanticContracts(t *testing.T) {
	w := WorkflowIR{
		SchemaVersion: WorkflowSchema.Current,
		ID:            "workflow/sdd",
		Version:       MustParseVersion("2.3.4"),
		Roles: []Role{{
			ID:             "role/implement",
			Objective:      "Deliver one bounded vertical work unit",
			Inputs:         []Contract{{ID: "contract/task", SchemaVersion: ContractSchema.Current, Required: true}},
			Outputs:        []Contract{{ID: "contract/evidence", SchemaVersion: ContractSchema.Current, Required: true}},
			AllowedEffects: []Effect{"repository/write"},
			TerminalStates: []TerminalState{TerminalSuccess, TerminalBlocked},
		}},
		Phases:   []Phase{{ID: "phase/apply", Role: "role/implement", DependsOn: []SemanticID{"phase/tasks"}}},
		Tools:    []ToolRequirement{{ID: "tool/test/run", Required: true}},
		Context:  ContextPolicy{Classes: []TrustClass{TrustTrustedPolicy, TrustRepositoryData}},
		Services: []ServiceRequirement{{ID: "service/forgespec", Version: VersionRange{Minimum: MustParseVersion("1.0.0"), MaximumTested: MustParseVersion("1.4.0")}}},
		Profiles: []Profile{{ID: "profile/portable-sequential", Experimental: false}},
	}

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	result, err := DecodeWorkflow(data)
	if err != nil {
		t.Fatalf("DecodeWorkflow() error = %v", err)
	}
	if result.Workflow.Roles[0].ID != "role/implement" || result.Workflow.Phases[0].ID != "phase/apply" {
		t.Fatalf("canonical semantic IDs were not preserved: %+v", result.Workflow)
	}
	if result.Workflow.Version.String() != "2.3.4" || result.Workflow.SchemaVersion != WorkflowSchema.Current {
		t.Fatalf("workflow/schema versions were not preserved: %+v", result.Workflow)
	}
}

func TestValidateSemanticID(t *testing.T) {
	for _, tt := range []struct {
		name string
		id   SemanticID
		ok   bool
	}{
		{name: "canonical", id: "phase/apply", ok: true},
		{name: "nested namespace", id: "quality/tdd/red", ok: true},
		{name: "missing namespace", id: "apply"},
		{name: "uppercase", id: "Phase/apply"},
		{name: "empty segment", id: "phase//apply"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSemanticID(tt.id)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateSemanticID(%q) error = %v, want valid=%v", tt.id, err, tt.ok)
			}
		})
	}
}

func TestExtensionContractDefaultsRemoteA2AToUnsupportedAndUnbound(t *testing.T) {
	extension := ExtensionContract{
		ID:                "extension/remote-agent-a2a",
		SchemaVersion:     ExtensionSchema.Current,
		DefaultResolution: ResolutionUnsupported,
	}

	if err := extension.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if extension.Provider != "" || len(extension.Tools) != 0 || len(extension.Permissions) != 0 {
		t.Fatalf("unsupported extension exposes provider surface: %+v", extension)
	}
}

func TestExtensionContractRejectsProviderSurfaceWithoutQualification(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ExtensionContract)
	}{
		{name: "provider", mutate: func(extension *ExtensionContract) { extension.Provider = "remote-provider" }},
		{name: "tool", mutate: func(extension *ExtensionContract) { extension.Tools = []SemanticID{"tool/remote-send"} }},
		{name: "permission", mutate: func(extension *ExtensionContract) { extension.Permissions = []string{"network/remote-agent"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extension := ExtensionContract{ID: "extension/remote-agent-a2a", SchemaVersion: ExtensionSchema.Current, DefaultResolution: ResolutionUnsupported}
			tt.mutate(&extension)
			if err := extension.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unqualified provider surface rejection")
			}
		})
	}
}
