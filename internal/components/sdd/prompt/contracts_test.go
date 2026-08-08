package prompt

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestCanonicalNineRoleSkillBindingsBothAuthorities(t *testing.T) {
	want := []PhaseRoleBinding{
		{Phase: "phase/init", Role: "role/bootstrap", Agent: "agent/bootstrap", Skill: "skill/bootstrap"},
		{Phase: "phase/explore", Role: "role/investigate", Agent: "agent/investigate", Skill: "skill/investigate"},
		{Phase: "phase/propose", Role: "role/draft-proposal", Agent: "agent/draft-proposal", Skill: "skill/draft-proposal"},
		{Phase: "phase/spec", Role: "role/write-specs", Agent: "agent/write-specs", Skill: "skill/write-specs"},
		{Phase: "phase/design", Role: "role/architect", Agent: "agent/architect", Skill: "skill/architect"},
		{Phase: "phase/tasks", Role: "role/decompose", Agent: "agent/decompose", Skill: "skill/decompose"},
		{Phase: "phase/apply", Role: "role/implement", Agent: "agent/implement", Skill: "skill/implement"},
		{Phase: "phase/verify", Role: "role/validate", Agent: "agent/validate", Skill: "skill/validate"},
		{Phase: "phase/archive", Role: "role/finalize", Agent: "agent/finalize", Skill: "skill/finalize"},
	}

	got := CanonicalPhaseRoleBindings()
	if err := ValidatePhaseRoleBindings(got); err != nil {
		t.Fatalf("ValidatePhaseRoleBindings(canonical) error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("canonical bindings = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("canonical binding %d = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestCanonicalPhaseRoleBindingsReturnsClone(t *testing.T) {
	bindings := CanonicalPhaseRoleBindings()
	bindings[0].Skill = "skill/unexpected"
	if got := CanonicalPhaseRoleBindings()[0].Skill; got != "skill/bootstrap" {
		t.Fatalf("canonical inventory was mutated: first skill = %q", got)
	}
}

func TestCanonicalRoleAuditExcludesUtilityAgents(t *testing.T) {
	for _, binding := range CanonicalPhaseRoleBindings() {
		if strings.Contains(string(binding.Agent), "utility") || strings.Contains(string(binding.Agent), "coordinator") {
			t.Errorf("canonical inventory includes utility agent %q", binding.Agent)
		}
	}
}

func TestCanonicalRoleBindingDriftRejectedNoMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]PhaseRoleBinding) []PhaseRoleBinding
	}{
		{
			name: "missing binding",
			mutate: func(bindings []PhaseRoleBinding) []PhaseRoleBinding {
				return bindings[:len(bindings)-1]
			},
		},
		{
			name: "duplicate role",
			mutate: func(bindings []PhaseRoleBinding) []PhaseRoleBinding {
				bindings[1].Role = bindings[0].Role
				return bindings
			},
		},
		{
			name: "cross phase skill",
			mutate: func(bindings []PhaseRoleBinding) []PhaseRoleBinding {
				bindings[0].Skill = bindings[1].Skill
				return bindings
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePhaseRoleBindings(tt.mutate(CanonicalPhaseRoleBindings())); err == nil {
				t.Fatal("ValidatePhaseRoleBindings() error = nil, want rejection")
			}
		})
	}
}

func TestAdapterPromptContractHasAllDesignFields(t *testing.T) {
	contract := validAdapterContract()
	if err := contract.Validate(); err != nil {
		t.Fatalf("valid AdapterPromptContract.Validate() error = %v", err)
	}
	if contract.Target == "" {
		t.Fatal("Target must be settable")
	}
	if contract.AgentPath == nil {
		t.Fatal("AgentPath must be settable")
	}
	if contract.ExpandPath == nil {
		t.Fatal("ExpandPath must be settable")
	}
}

func TestAdapterPromptContractRejectsMissingRoots(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*AdapterPromptContract)
	}{
		{name: "empty target", mutate: func(c *AdapterPromptContract) { c.Target = "" }},
		{name: "empty root path", mutate: func(c *AdapterPromptContract) { c.RootPath = "" }},
		{name: "nil agent path", mutate: func(c *AdapterPromptContract) { c.AgentPath = nil }},
		{name: "empty skill root", mutate: func(c *AdapterPromptContract) { c.SkillRoot = "" }},
		{name: "empty command root", mutate: func(c *AdapterPromptContract) { c.CommandRoot = "" }},
		{name: "nil expand path", mutate: func(c *AdapterPromptContract) { c.ExpandPath = nil }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			contract := validAdapterContract()
			tt.mutate(&contract)
			if err := contract.Validate(); err == nil {
				t.Fatalf("Validate() error = nil for %s", tt.name)
			}
		})
	}
}

func TestNativeSkillPreloadControlsLoadMode(t *testing.T) {
	withPreload := validAdapterContract()
	withPreload.NativeSkillPreload = true
	if mode := withPreload.SkillLoadMode(); mode != SkillModeNativePreload {
		t.Fatalf("NativeSkillPreload=true mode = %q, want %q", mode, SkillModeNativePreload)
	}

	withoutPreload := validAdapterContract()
	withoutPreload.NativeSkillPreload = false
	if mode := withoutPreload.SkillLoadMode(); mode != SkillModeFallbackRead {
		t.Fatalf("NativeSkillPreload=false mode = %q, want %q", mode, SkillModeFallbackRead)
	}
}

func TestNativeSkillOnDemandTakesPrecedenceOverPreload(t *testing.T) {
	contract := validAdapterContract()
	contract.NativeSkillPreload = true
	contract.NativeSkillOnDemand = true
	if mode := contract.SkillLoadMode(); mode != SkillModeNativeOnDemand {
		t.Fatalf("SkillLoadMode() = %q, want %q", mode, SkillModeNativeOnDemand)
	}
}

func TestCompositionInputReferencesAssetCatalog(t *testing.T) {
	input := CompositionInput{
		Workflow: ir.WorkflowIR{SchemaVersion: ir.WorkflowSchema.Current, ID: "workflow/sdd"},
		Catalog: ir.AssetCatalog{
			SchemaVersion: ir.AssetCatalogSchema.Current,
			Assets: []ir.AssetSpec{{
				ID: "asset/root-index", Class: ir.AssetRootIndex,
				SourcePath: "root.md", Required: true, MaxTokens: 1500, SHA256: "abc",
			}},
		},
		Adapter: validAdapterContract(),
		Profile: quality.ProfilePlan{ProfileID: "profile/portable-sequential"},
		Models:  ModelTable{Routes: []ModelRoute{qualifiedRoute("role/apply")}},
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid CompositionInput.Validate() error = %v", err)
	}
	if err := input.Catalog.Validate(); err != nil {
		t.Fatalf("CompositionInput.Catalog must be a valid AssetCatalog: %v", err)
	}
}

func TestCompositionInputRejectsInvalidCatalog(t *testing.T) {
	input := CompositionInput{
		Workflow: ir.WorkflowIR{SchemaVersion: ir.WorkflowSchema.Current, ID: "workflow/sdd"},
		Catalog:  ir.AssetCatalog{}, // missing schema version
		Adapter:  validAdapterContract(),
	}
	if err := input.Validate(); err == nil {
		t.Fatal("CompositionInput with invalid catalog.Validate() error = nil, want rejection")
	}
}

func TestCanonicalSkillForRoleIsDeterministic(t *testing.T) {
	for role, want := range map[ir.SemanticID]ir.SemanticID{
		"role/orchestrator":      "skill/orchestrator",
		"role/bootstrap":         "skill/bootstrap",
		"role/explore":           "skill/investigate",
		"role/proposal":          "skill/draft-proposal",
		"role/spec":              "skill/write-specs",
		"role/design":            "skill/architect",
		"role/tasks":             "skill/decompose",
		"role/apply":             "skill/implement",
		"role/verify":            "skill/validate",
		"role/archive":           "skill/finalize",
		"role/debate":            "skill/debate",
		"role/parallel-dispatch": "skill/parallel-dispatch",
	} {
		got, err := CanonicalSkillForRole(role)
		if err != nil {
			t.Fatalf("CanonicalSkillForRole(%q) error = %v", role, err)
		}
		if got != want {
			t.Errorf("CanonicalSkillForRole(%q) = %q, want %q", role, got, want)
		}
		// Determinism: same role always maps to the same skill.
		again, _ := CanonicalSkillForRole(role)
		if got != again {
			t.Errorf("CanonicalSkillForRole(%q) is not deterministic", role)
		}
	}
}

func TestTransverseSkillBindingsUseCanonicalInstalledPaths(t *testing.T) {
	for role, wantPath := range map[ir.SemanticID]string{
		"role/orchestrator":      "internal/assets/skills/orchestrator/SKILL.md",
		"role/debate":            "internal/assets/skills/debate/SKILL.md",
		"role/parallel-dispatch": "internal/assets/skills/parallel-dispatch/SKILL.md",
	} {
		binding, err := NewSkillBinding(role, validAdapterContract())
		if err != nil {
			t.Fatalf("NewSkillBinding(%q) error = %v", role, err)
		}
		if binding.Path != wantPath {
			t.Errorf("NewSkillBinding(%q).Path = %q, want %q", role, binding.Path, wantPath)
		}
	}
}

func TestCanonicalSkillForRoleRejectsUnknownRole(t *testing.T) {
	for _, role := range []ir.SemanticID{"role/unknown", "apply", "role/mailbox"} {
		if _, err := CanonicalSkillForRole(role); err == nil {
			t.Fatalf("CanonicalSkillForRole(%q) error = nil, want rejection", role)
		}
	}
}

func TestSkillBindingDeterministicFromRole(t *testing.T) {
	binding, err := NewSkillBinding("role/apply", validAdapterContract())
	if err != nil {
		t.Fatalf("NewSkillBinding error = %v", err)
	}
	if binding.Role != "role/apply" {
		t.Fatalf("Role = %q, want role/apply", binding.Role)
	}
	if binding.Skill != "skill/implement" {
		t.Fatalf("Skill = %q, want skill/implement", binding.Skill)
	}
	if binding.Path != "internal/assets/skills/implement/SKILL.md" {
		t.Fatalf("Path = %q, want installed skill path without semantic prefix", binding.Path)
	}
	if binding.Hash == "" {
		t.Fatal("Hash must be set")
	}
	if binding.Mode != SkillModeFallbackRead {
		t.Fatalf("Mode = %q, want %q (NativeSkillPreload=false)", binding.Mode, SkillModeFallbackRead)
	}
}

func TestModelTableResolvesRoleToModel(t *testing.T) {
	table := ModelTable{Routes: []ModelRoute{
		qualifiedRoute("role/apply"),
		qualifiedRoute("role/verify"),
	}}
	apply, err := table.ModelFor("role/apply")
	if err != nil {
		t.Fatalf("ModelFor(role/apply) error = %v", err)
	}
	if apply.Primary.Model != "model-test" || apply.Fallback != nil {
		t.Fatalf("apply model = %+v", apply)
	}
	if _, err := table.ModelFor("role/unknown"); err == nil {
		t.Fatal("ModelFor(unknown) error = nil, want rejection")
	}
}
