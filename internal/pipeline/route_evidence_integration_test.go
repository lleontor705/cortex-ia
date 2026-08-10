package pipeline

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestRouteEvidenceSentinelSurvivesPipelineMetadataBoundary(t *testing.T) {
	route, err := modelroute.NewRouteID("route/v1/architecture")
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := modelroute.NewFallbackRouteID("route/v1/architecture-fallback")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)
	resolved := modelroute.ResolvedRoute{
		Requested: modelroute.RouteRequest{RouteID: route, FallbackRouteID: fallback, AllowFallback: true, Constraints: []modelroute.CapabilityConstraint{{ID: "tool-call", Required: true}}},
		PrimaryID: route, Primary: modelroute.RouteRef{Provider: "provider-sentinel", Model: "model-sentinel"},
		FallbackID: fallback, Fallback: &modelroute.RouteRef{Provider: "fallback-provider-sentinel", Model: "fallback-model-sentinel"},
		Constraints: []modelroute.CapabilityConstraint{{ID: "tool-call", Required: true}},
		Evidence:    []modelroute.ResolutionEvidence{{ID: "evidence-sentinel", Source: modelroute.SourceProviderConfig, Provider: "provider-sentinel", Route: route, ObservedAt: now.Add(-time.Minute), FreshUntil: now.Add(time.Hour), Digest: "fingerprint-sentinel", Qualified: true, ReasonID: "route.primary"}},
		Degradation: "fallback-not-used",
	}
	resolved.Role = ir.SemanticID("role/architecture")
	table := prompt.ModelTable{Routes: []prompt.ModelRoute{resolved}}
	metadata := WorkflowMetadata{Routes: map[string]modelroute.ResolvedRoute{"role/architecture": resolved}, ContractFingerprint: "contract-sentinel", QualityPlanID: "quality-sentinel", ProfileRequested: "profile-requested-sentinel", ProfileEffective: "profile-effective-sentinel", ProfileReasonID: "profile-reason-sentinel", TrustEvidence: []string{"trust-sentinel"}}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip WorkflowMetadata
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Routes["role/architecture"].Primary != resolved.Primary || roundTrip.Routes["role/architecture"].Evidence[0].Digest != "fingerprint-sentinel" || table.Routes[0].Primary != resolved.Primary {
		t.Fatalf("route evidence lost: metadata=%+v table=%+v", roundTrip, table)
	}
}

func TestPrepareOpenCodeWithoutRoutesInheritsActiveModel(t *testing.T) {
	prepared, err := PrepareWorkflow(t.Context(), WorkflowRequest{HomeDir: t.TempDir(), Adapters: []agents.Adapter{opencode.NewAdapter()}})
	if err != nil {
		t.Fatalf("PrepareWorkflow() without routes: %v", err)
	}
	if len(prepared.Bundles) != 1 {
		t.Fatalf("prepared bundles = %d, want 1", len(prepared.Bundles))
	}
	foundRoutingText := false
	for _, asset := range prepared.Bundles[0].Bundle.Assets {
		if asset.Kind == renderers.AssetAgent && strings.Contains(string(asset.Content), "\nmodel:") {
			t.Fatalf("OpenCode no-route agent emitted a model field in %s:\n%s", asset.Path, asset.Content)
		}
		if asset.SemanticID == "asset/root-module/model-routing" {
			text := string(asset.Content)
			foundRoutingText = strings.Contains(text, "omits the `model` field") && strings.Contains(text, "inherits the active model selected by OpenCode") && strings.Contains(text, "provider/model") && strings.Contains(text, "fail closed")
		}
	}
	if !foundRoutingText {
		t.Fatal("rendered OpenCode model-routing reference does not describe no-route inheritance and explicit-route validation")
	}
}

func TestModelRouteBoundaryRejectsMissingResolutionEvidence(t *testing.T) {
	role := ir.SemanticID("role/architecture")
	if _, err := (prompt.ModelTable{Routes: []prompt.ModelRoute{{Role: role}}}).ModelFor(role); err == nil {
		t.Fatal("missing route evidence unexpectedly accepted")
	}
}

func TestPrepareWorkflowAllowsMissingRoutesBeforeMutation(t *testing.T) {
	home := t.TempDir()
	if _, err := PrepareWorkflow(t.Context(), WorkflowRequest{HomeDir: home, Adapters: []agents.Adapter{opencode.NewAdapter()}}); err != nil {
		t.Fatalf("missing model routes should allow workflow preparation: %v", err)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("preparation caused mutation: %v", entries)
	}
}
