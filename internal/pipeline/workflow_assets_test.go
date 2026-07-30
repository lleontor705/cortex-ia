package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/claude"
	"github.com/lleontor705/cortex-ia/internal/agents/codex"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestPrepareWorkflowUsesFreshQualifiedAdapterProfile(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir:        home,
		Adapters:       []agents.Adapter{codex.NewAdapter()},
		EvaluationTime: now,
		ModelRoutes:    testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plan.Profile != "portable-flat" {
		t.Fatalf("profile = %q, want strongest fresh adapter-qualified profile", prepared.Plan.Profile)
	}
}

func TestPrepareWorkflowDegradesWhenAdapterProfileRouteIsNotQualified(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	provider := claude.NewAdapter()
	decision := ResolveProfileDecision(ProfileResolutionInput{
		Requested: sdd.ProfileNativeAdvanced, Facts: provider.CapabilityFacts(), Now: now,
	})
	if decision.Disposition != ProfileDispositionDegraded {
		t.Fatalf("unqualified adapter route must degrade explicitly: %+v", decision)
	}
}

func TestWorkflowMetadataRoundTripsSentinels(t *testing.T) {
	metadata := WorkflowMetadata{
		ContractFingerprint: "contract-sentinel",
		PrimaryModel:        "primary-sentinel",
		FallbackModel:       "fallback-sentinel",
		QualityPlanID:       "quality-sentinel",
		ProfileReasonID:     "profile-qualified",
		TrustEvidence:       []string{"evidence-sentinel"},
		Permissions:         []string{"permission/read"},
		HumanGate:           "gate-required",
		Observability:       "trace-sentinel",
	}
	if got := metadata.Clone(); got.ContractFingerprint != metadata.ContractFingerprint || got.PrimaryModel != metadata.PrimaryModel || got.FallbackModel != metadata.FallbackModel || got.QualityPlanID != metadata.QualityPlanID || got.ProfileReasonID != metadata.ProfileReasonID || got.HumanGate != metadata.HumanGate || got.Observability != metadata.Observability {
		t.Fatalf("metadata clone lost sentinel fields: %+v", got)
	}
}

func TestPreparedWorkflowMetadataSurvivesPlanAndReceipt(t *testing.T) {
	home := t.TempDir()
	prepared, err := PrepareWorkflow(context.Background(), WorkflowRequest{
		HomeDir: home, Adapters: []agents.Adapter{codex.NewAdapter()},
		GeneratorVersion: "test-v1", EvaluationTime: time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC),
		ModelRoutes: testModelRoutes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var planned WorkflowMetadata
	if err := json.Unmarshal(prepared.Plan.Metadata, &planned); err != nil {
		t.Fatal(err)
	}
	if planned.ProfileReasonID == "" || len(planned.TrustEvidence) == 0 {
		t.Fatalf("plan metadata incomplete: %+v", planned)
	}
	if planned.PrimaryModel != "" || planned.FallbackModel != "" {
		t.Fatalf("profile metadata must not imply model routing: %+v", planned)
	}
	receipt, err := prepared.Apply()
	if err != nil {
		t.Fatal(err)
	}
	var applied WorkflowMetadata
	if err := json.Unmarshal(receipt.Metadata, &applied); err != nil {
		t.Fatal(err)
	}
	if applied.ContractFingerprint != planned.ContractFingerprint || applied.ProfileReasonID != planned.ProfileReasonID || applied.Permissions[0] != planned.Permissions[0] {
		t.Fatalf("metadata changed across apply: plan=%+v receipt=%+v", planned, applied)
	}
}

func testModelRoutes() prompt.ModelTable {
	roles := []ir.SemanticID{"role/bootstrap", "role/investigate", "role/draft-proposal", "role/write-specs", "role/architect", "role/decompose", "role/implement", "role/validate", "role/finalize"}
	routes := make([]prompt.ModelRoute, 0, len(roles))
	for _, role := range roles {
		route, _ := modelroute.NewRouteID("route/v1/test")
		routes = append(routes, modelroute.ResolvedRoute{Role: role, Requested: modelroute.RouteRequest{RouteID: route}, PrimaryID: route, Primary: modelroute.RouteRef{Provider: "provider-test", Model: "model-test"}, Evidence: []modelroute.ResolutionEvidence{{ID: "evidence-" + string(role), Source: modelroute.SourceProviderConfig, Provider: "provider-test", Route: route, ObservedAt: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC), FreshUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Digest: "digest-test", Qualified: true}}})
	}
	return prompt.ModelTable{Routes: routes}
}
