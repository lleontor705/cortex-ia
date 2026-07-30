package prompt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
)

func TestComposePreservesMetadataSentinel(t *testing.T) {
	sentinel := json.RawMessage(`{"profile":"profile-sentinel","quality":"quality-sentinel"}`)
	input := validCompositionInput()
	input.Metadata = sentinel
	result, err := Compose(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Metadata) != string(sentinel) {
		t.Fatalf("composition lost metadata: %s", result.Metadata)
	}
}

func testCatalog() ir.AssetCatalog {
	assets := []ir.AssetSpec{
		{ID: "asset/root-index", Class: ir.AssetRootIndex, SourcePath: "generic/sdd-orchestrator-root-index.md", Required: true, MaxTokens: RootIndexMaxTokens, SHA256: "root-hash"},
		{ID: "asset/module-routing", Class: ir.AssetRootModule, SourcePath: "sdd-root/routing-and-risk.md", Required: true, MaxTokens: 0, SHA256: "m1"},
		{ID: "asset/module-contracts", Class: ir.AssetRootModule, SourcePath: "sdd-root/contracts-and-thresholds.md", Required: true, MaxTokens: 0, SHA256: "m2"},
		{ID: "asset/module-recovery", Class: ir.AssetRootModule, SourcePath: "sdd-root/recovery-and-reflection.md", Required: true, MaxTokens: 0, SHA256: "m3"},
		{ID: "asset/module-parallel", Class: ir.AssetRootModule, SourcePath: "sdd-root/parallel-apply.md", Required: true, MaxTokens: 0, SHA256: "m4"},
		{ID: "asset/module-memory", Class: ir.AssetRootModule, SourcePath: "sdd-root/memory-and-state.md", Required: true, MaxTokens: 0, SHA256: "m5"},
		{ID: "asset/module-models", Class: ir.AssetRootModule, SourcePath: "sdd-root/model-routing.md", Required: true, MaxTokens: 0, SHA256: "m6"},
		{ID: "asset/shared-contract", Class: ir.AssetSharedContract, SourcePath: "_shared/sdd-phase-contract.md", Required: true, MaxTokens: SharedContractMaxTokens, SHA256: "shared-hash"},
		{ID: "asset/profile-overlay", Class: ir.AssetProfileOverlay, SourcePath: "profiles/portable-sequential.md", Required: true, MaxTokens: ProfileOverlayMaxTokens, SHA256: "overlay-hash", Profiles: []ir.SemanticID{"profile/portable-sequential"}},
		{ID: "asset/quality-template", Class: ir.AssetQualityTemplate, SourcePath: "quality/plan-template.json", Required: true, MaxTokens: 0, SHA256: "quality-hash"},
	}
	return ir.AssetCatalog{
		SchemaVersion: ir.AssetCatalogSchema.Current,
		Assets:        assets,
	}
}

func validCompositionInput() CompositionInput {
	return CompositionInput{
		Workflow: ir.WorkflowIR{SchemaVersion: ir.WorkflowSchema.Current, ID: "workflow/sdd"},
		Catalog:  testCatalog(),
		Adapter:  validAdapterContract(),
		Profile:  quality.ProfilePlan{ProfileID: "profile/portable-sequential"},
		QualityTemplate: quality.QualityPlanTemplate{
			SchemaVersion: "1.0.0", PolicySHA256: "policy-hash", ProfileID: "profile/portable-sequential",
		},
		Models: ModelTable{Routes: []ModelRoute{
			qualifiedRoute("role/bootstrap"), qualifiedRoute("role/explore"), qualifiedRoute("role/proposal"),
			qualifiedRoute("role/spec"), qualifiedRoute("role/design"), qualifiedRoute("role/tasks"),
			qualifiedRoute("role/apply"), qualifiedRoute("role/verify"), qualifiedRoute("role/archive"),
		}},
	}
}

func TestComposeProducesSkillBindingsForAllNineRoles(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if len(result.SkillBindings) != 9 {
		t.Fatalf("Compose produced %d skill bindings, want 9", len(result.SkillBindings))
	}
	for _, binding := range result.SkillBindings {
		if err := binding.Validate(); err != nil {
			t.Fatalf("skill binding %q invalid: %v", binding.Role, err)
		}
	}
}

func TestComposeProducesExactlyOneBindingPerRole(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	seen := make(map[ir.SemanticID]int)
	for _, b := range result.SkillBindings {
		seen[b.Role]++
	}
	for _, role := range []ir.SemanticID{
		"role/bootstrap", "role/explore", "role/proposal", "role/spec",
		"role/design", "role/tasks", "role/apply", "role/verify", "role/archive",
	} {
		if seen[role] != 1 {
			t.Fatalf("role %q has %d bindings, want 1", role, seen[role])
		}
	}
}

func TestComposeResolvesRootIndexFromCatalog(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if result.RootIndex == "" {
		t.Fatal("Compose RootIndex path is empty")
	}
	if !strings.Contains(result.RootIndex, "sdd-orchestrator-root-index") {
		t.Fatalf("RootIndex path = %q, want to contain root index filename", result.RootIndex)
	}
}

func TestComposeResolvesModulesFromCatalog(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if len(result.Modules) != 6 {
		t.Fatalf("Compose produced %d modules, want 6", len(result.Modules))
	}
}

func TestComposeResolvesSharedContractPath(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if result.SharedContract == "" {
		t.Fatal("Compose SharedContract path is empty")
	}
}

func TestComposeResolvesProfileOverlayPath(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if result.ProfileOverlay == "" {
		t.Fatal("Compose ProfileOverlay path is empty")
	}
}

func TestComposeResolvesQualityTemplatePath(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if result.QualityTemplate == "" {
		t.Fatal("Compose QualityTemplate path is empty")
	}
}

func TestComposeIsDeterministic(t *testing.T) {
	first, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose first error = %v", err)
	}
	second, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose second error = %v", err)
	}
	if first.RootIndex != second.RootIndex {
		t.Fatalf("RootIndex not deterministic: %q != %q", first.RootIndex, second.RootIndex)
	}
	if len(first.SkillBindings) != len(second.SkillBindings) {
		t.Fatalf("SkillBindings count not deterministic")
	}
	for i := range first.SkillBindings {
		if first.SkillBindings[i] != second.SkillBindings[i] {
			t.Fatalf("SkillBinding[%d] not deterministic: %+v != %+v", i, first.SkillBindings[i], second.SkillBindings[i])
		}
	}
}

func TestComposeRejectsInvalidInput(t *testing.T) {
	// Missing adapter.
	bad := validCompositionInput()
	bad.Adapter = AdapterPromptContract{}
	if _, err := Compose(bad); err == nil {
		t.Fatal("Compose with invalid adapter returned nil error")
	}
}

func TestComposeRejectsCatalogWithoutRootIndex(t *testing.T) {
	bad := validCompositionInput()
	bad.Catalog = ir.AssetCatalog{
		SchemaVersion: ir.AssetCatalogSchema.Current,
		Assets: []ir.AssetSpec{{
			ID: "asset/module", Class: ir.AssetRootModule, SourcePath: "m.md", Required: true, MaxTokens: 0, SHA256: "x",
		}},
	}
	if _, err := Compose(bad); err == nil {
		t.Fatal("Compose without root-index in catalog returned nil error")
	}
}

func TestComposeTokenReportRecordsAllAssets(t *testing.T) {
	result, err := Compose(validCompositionInput())
	if err != nil {
		t.Fatalf("Compose error = %v", err)
	}
	if len(result.TokenReport.Entries) == 0 {
		t.Fatal("Compose TokenReport has no entries")
	}
}
