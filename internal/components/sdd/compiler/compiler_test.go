package compiler

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestCompileProducesStableNormalizedResultAndFingerprint(t *testing.T) {
	input := validInput(t)

	var first Result
	for run := 0; run < 3; run++ {
		got, err := Compile(input)
		if err != nil {
			t.Fatalf("Compile() run %d error = %v", run+1, err)
		}
		if run == 0 {
			first = got
			continue
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("Compile() run %d differed:\nfirst=%+v\ngot=%+v", run+1, first, got)
		}
	}

	if first.Fingerprint == "" || len(first.Canonical) == 0 {
		t.Fatalf("Compile() omitted deterministic evidence: %+v", first)
	}
	if got := first.Normalized.Workflow.Phases; got[0].ID != "phase/apply" || got[1].ID != "phase/spec" {
		t.Fatalf("phases not normalized by semantic ID: %+v", got)
	}
	if got := first.Normalized.Workflow.Phases[0].DependsOn; !reflect.DeepEqual(got, []ir.SemanticID{"phase/spec"}) {
		t.Fatalf("phase dependencies not normalized: %v", got)
	}
	if got := first.Normalized.ProbeResults[0].Permissions; !reflect.DeepEqual(got, []string{"filesystem/read", "tool/task"}) {
		t.Fatalf("probe permissions not normalized: %v", got)
	}
	if string(first.Normalized.Configuration) != `{"feature":{"enabled":true},"retries":2}` {
		t.Fatalf("configuration = %s", first.Normalized.Configuration)
	}
}

func TestCompileFingerprintIncludesEveryOutputAffectingInput(t *testing.T) {
	baseline := validInput(t)
	want, err := Compile(baseline)
	if err != nil {
		t.Fatalf("Compile(baseline) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "workflow", mutate: func(in *Input) {
			in.WorkflowDocument = replaceJSONField(t, in.WorkflowDocument, "version", "1.0.1")
		}},
		{name: "catalog", mutate: func(in *Input) {
			in.CatalogDocument = replaceJSONField(t, in.CatalogDocument, "version", "1.0.1")
		}},
		{name: "probe results", mutate: func(in *Input) {
			in.ProbeResults[0].Record.EvidenceDigest = "sha256:changed"
		}},
		{name: "target", mutate: func(in *Input) { in.Target = "codex" }},
		{name: "profile", mutate: func(in *Input) { in.Profile = "native-advanced" }},
		{name: "configuration", mutate: func(in *Input) {
			in.Configuration = json.RawMessage(`{"retries":3,"feature":{"enabled":true}}`)
		}},
		{name: "compiler version", mutate: func(in *Input) {
			in.CompilerVersion = ir.MustParseVersion("1.0.1")
		}},
		{name: "evaluation time", mutate: func(in *Input) {
			in.EvaluationTime = in.EvaluationTime.Add(time.Second)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneInput(baseline)
			tt.mutate(&changed)
			got, compileErr := Compile(changed)
			if compileErr != nil {
				t.Fatalf("Compile(changed) error = %v", compileErr)
			}
			if got.Fingerprint == want.Fingerprint {
				t.Fatalf("fingerprint did not include %s", tt.name)
			}
		})
	}
}

func TestCompileRejectsUnresolvedAndCyclicReferencesBeforeMutation(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(map[string]any)
		wantCode ErrorCode
		wantPath string
	}{
		{
			name: "unresolved role",
			mutate: func(workflow map[string]any) {
				workflow["phases"].([]any)[0].(map[string]any)["role"] = "role/missing"
			},
			wantCode: ErrorUnresolvedReference,
			wantPath: "$.workflow.phases[0].role",
		},
		{
			name: "unresolved dependency",
			mutate: func(workflow map[string]any) {
				workflow["phases"].([]any)[0].(map[string]any)["depends_on"] = []any{"phase/missing"}
			},
			wantCode: ErrorUnresolvedReference,
			wantPath: "$.workflow.phases[0].depends_on[0]",
		},
		{
			name: "cyclic dependency",
			mutate: func(workflow map[string]any) {
				workflow["phases"].([]any)[1].(map[string]any)["depends_on"] = []any{"phase/apply"}
			},
			wantCode: ErrorCyclicReference,
			wantPath: "$.workflow.phases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput(t)
			var workflow map[string]any
			if err := json.Unmarshal(input.WorkflowDocument, &workflow); err != nil {
				t.Fatal(err)
			}
			tt.mutate(workflow)
			input.WorkflowDocument = mustJSON(t, workflow)
			before := cloneInput(input)

			_, err := Compile(input)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Compile() error = %v, want *ValidationError", err)
			}
			if validationErr.Code != tt.wantCode || validationErr.Path != tt.wantPath {
				t.Fatalf("Compile() error = %+v, want code=%s path=%s", validationErr, tt.wantCode, tt.wantPath)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("Compile() mutated input before rejecting references")
			}
		})
	}
}

func TestCompileRejectsIncompatibleSchemasBeforeMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "workflow", mutate: func(in *Input) {
			in.WorkflowDocument = replaceJSONField(t, in.WorkflowDocument, "schema_version", "2.0.0")
		}},
		{name: "catalog", mutate: func(in *Input) {
			in.CatalogDocument = replaceJSONField(t, in.CatalogDocument, "schema_version", "2.0.0")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput(t)
			tt.mutate(&input)
			before := cloneInput(input)
			if _, err := Compile(input); err == nil {
				t.Fatal("Compile() error = nil")
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("Compile() mutated input before rejecting incompatible schema")
			}
		})
	}
}

func TestCompileIntegratesCompleteAssetCatalogCompositionAndQuality(t *testing.T) {
	input := validInput(t)
	input.AssetCatalog = completeAssetCatalog()
	input.Adapter = prompt.AdapterPromptContract{
		Target: "claude", RootPath: ".claude", SkillRoot: "skills", CommandRoot: "commands",
		AgentPath:  func(id ir.SemanticID) string { return "agents/" + string(id) },
		ExpandPath: func(root, relative string) (string, error) { return root + "/" + relative, nil },
	}
	input.ProfilePlan = quality.ProfilePlan{ProfileID: "portable-sequential"}
	input.QualityPolicy = &quality.QualityPolicy{Version: "1.0.0"}

	result, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Composition.SkillBindings) != len(result.Normalized.Workflow.Roles) || result.Composition.RootIndex == "" {
		t.Fatalf("composition did not include complete role/assets output: %+v", result.Composition)
	}
	if result.QualityPolicyIR.PolicySHA256 == "" || result.QualityTemplate.PolicySHA256 == "" {
		t.Fatalf("quality policy was not normalized: %+v %+v", result.QualityPolicyIR, result.QualityTemplate)
	}
}

func TestCompileRejectsIncompleteAssetCatalogBeforeComposition(t *testing.T) {
	input := validInput(t)
	input.AssetCatalog = completeAssetCatalog()
	input.AssetCatalog.Assets = input.AssetCatalog.Assets[1:]
	input.Adapter = prompt.AdapterPromptContract{Target: "claude", RootPath: ".claude", SkillRoot: "skills", CommandRoot: "commands", AgentPath: func(ir.SemanticID) string { return "agent" }, ExpandPath: func(root, relative string) (string, error) { return root + "/" + relative, nil }}
	input.ProfilePlan = quality.ProfilePlan{ProfileID: "portable-sequential"}

	if _, err := Compile(input); err == nil {
		t.Fatal("Compile accepted an incomplete asset catalog")
	}
}

func TestCompileFingerprintIncludesAssetAndQualityInputs(t *testing.T) {
	base := validInput(t)
	base.AssetCatalog = completeAssetCatalog()
	base.Adapter = prompt.AdapterPromptContract{Target: "claude", RootPath: ".claude", SkillRoot: "skills", CommandRoot: "commands", AgentPath: func(ir.SemanticID) string { return "agent" }, ExpandPath: func(root, relative string) (string, error) { return root + "/" + relative, nil }}
	base.ProfilePlan = quality.ProfilePlan{ProfileID: "portable-sequential"}
	base.QualityPolicy = &quality.QualityPolicy{Version: "1.0.0"}
	want, err := Compile(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.AssetCatalog.Assets = append([]ir.AssetSpec(nil), base.AssetCatalog.Assets...)
	changed.AssetCatalog.Assets[0].SHA256 = "changed"
	changed.QualityPolicy = &quality.QualityPolicy{Version: "1.0.1"}
	got, err := Compile(changed)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint == want.Fingerprint {
		t.Fatal("fingerprint omitted asset or quality input")
	}
}

func completeAssetCatalog() ir.AssetCatalog {
	sha := "sha256"
	return ir.AssetCatalog{SchemaVersion: ir.MustParseVersion("1.0.0"), Assets: []ir.AssetSpec{
		{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: sha},
		{ID: "asset/module", Class: ir.AssetRootModule, SourcePath: "module.md", Required: true, SHA256: sha},
		{ID: "asset/shared", Class: ir.AssetSharedContract, SourcePath: "shared.md", Required: true, SHA256: sha},
		{ID: "asset/quality", Class: ir.AssetQualityTemplate, SourcePath: "quality.md", Required: true, SHA256: sha},
		{ID: "asset/profile", Class: ir.AssetProfileOverlay, SourcePath: "profile.md", Profiles: []ir.SemanticID{"portable-sequential"}, Required: true, SHA256: sha},
	}}
}

func validInput(t *testing.T) Input {
	t.Helper()
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	workflow := map[string]any{
		"schema_version": "1.0.0",
		"id":             "workflow/sdd",
		"version":        "1.0.0",
		"roles": []any{
			map[string]any{"id": "role/spec", "objective": "Specify behavior"},
			map[string]any{"id": "role/implement", "objective": "Implement behavior"},
		},
		"phases": []any{
			map[string]any{"id": "phase/spec", "role": "role/spec"},
			map[string]any{"id": "phase/apply", "role": "role/implement", "depends_on": []any{"phase/spec", "phase/spec"}},
		},
		"tools": []any{map[string]any{"id": "tool/test/run", "required": true}},
	}
	catalog := capability.Catalog{
		SchemaVersion: ir.MustParseVersion("1.0.0"),
		Version:       ir.MustParseVersion("1.0.0"),
		Facts:         []capability.CapabilityFact{},
	}
	return Input{
		WorkflowDocument: mustJSON(t, workflow),
		CatalogDocument:  mustJSON(t, catalog),
		ProbeResults: []capability.ProbeResult{{
			Record: capability.ProbeRecord{
				ID:             "probe/runtime-capabilities",
				Method:         capability.ProbeCommand,
				Command:        "runtime capabilities --json",
				Result:         "supported",
				Timestamp:      now.Add(-time.Hour),
				EvidenceDigest: "sha256:probe",
			},
			Refined:      capability.CapabilityFact{ID: "delegation/direct-child"},
			TrustClasses: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy, ir.TrustRepositoryData},
			Permissions:  []string{"tool/task", "filesystem/read", "tool/task"},
		}},
		Target:          "claude",
		Profile:         "portable-flat",
		Configuration:   json.RawMessage(`{"retries":2,"feature":{"enabled":true}}`),
		CompilerVersion: ir.MustParseVersion("1.0.0"),
		EvaluationTime:  now,
	}
}

func cloneInput(input Input) Input {
	cloned := input
	cloned.WorkflowDocument = append([]byte(nil), input.WorkflowDocument...)
	cloned.CatalogDocument = append([]byte(nil), input.CatalogDocument...)
	cloned.Configuration = append(json.RawMessage(nil), input.Configuration...)
	cloned.ProbeResults = append([]capability.ProbeResult(nil), input.ProbeResults...)
	for i := range cloned.ProbeResults {
		cloned.ProbeResults[i].Permissions = append([]string(nil), input.ProbeResults[i].Permissions...)
		cloned.ProbeResults[i].TrustClasses = append([]ir.TrustClass(nil), input.ProbeResults[i].TrustClasses...)
	}
	return cloned
}

func replaceJSONField(t *testing.T, data []byte, field string, value any) []byte {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object[field] = value
	return mustJSON(t, object)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
