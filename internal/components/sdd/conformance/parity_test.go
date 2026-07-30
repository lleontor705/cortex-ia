package conformance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/legacyoracle"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// TestLegacyParityAgainstPreparedWorkflow is the migration gate: the legacy
// oracle is compared with the active typed PrepareWorkflow plan for every
// supported adapter. It deliberately inspects the in-memory plan/bundle and
// never invokes the old injector.
func TestLegacyParityAgainstPreparedWorkflow(t *testing.T) {
	oracle, err := legacyoracle.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, adapter := range agents.NewDefaultRegistry().All() {
		adapter := adapter
		t.Run(string(adapter.Agent()), func(t *testing.T) {
			homeDir := t.TempDir()
			prepared, err := pipeline.PrepareWorkflow(context.Background(), pipeline.WorkflowRequest{
				HomeDir: homeDir, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "parity-test", RequestedProfile: sdd.ProfilePortableSequential, ModelRoutes: explicitModelRoutes(),
			})
			if err != nil {
				if strings.Contains(err.Error(), "adapter config root") && strings.Contains(err.Error(), "escapes home") {
					observed, evidenceErr := observeExternalRootBlocked(string(adapter.Agent()), string(sdd.ProfilePortableSequential), "PrepareWorkflow/parity", 1, err.Error(), []byte(adapter.GlobalConfigDir(homeDir)))
					if evidenceErr != nil {
						t.Fatalf("external-root evidence: %v", evidenceErr)
					}
					t.Logf("observed external-root disposition=%s reason=%s command=%s exit=%d protected-root=%s mutation=%s records=%d", observed.Disposition, observed.ReasonID, observed.Command, observed.ExitCode, observed.ProtectedRootDigest, observed.Mutation, len(observed.Report.Records))
					return
				}
				t.Fatalf("PrepareWorkflow() error = %v", err)
			}
			for _, asset := range oracle.Assets {
				if asset.Classification == legacyoracle.Retire {
					continue
				}
				if !typedParityPresent(asset, prepared) {
					t.Errorf("legacy asset %q (%s) has no typed PrepareWorkflow coverage; equivalent=%q", asset.ID, asset.Behavior, asset.TypedEquivalent)
				}
			}
		})
	}
}

// explicitModelRoutes is a qualified provider-configuration fixture. Tests
// must supply routing evidence explicitly; PrepareWorkflow intentionally has
// no inferred provider/model defaults.
func explicitModelRoutes() prompt.ModelTable {
	roles := []ir.SemanticID{"role/bootstrap", "role/investigate", "role/draft-proposal", "role/write-specs", "role/architect", "role/decompose", "role/implement", "role/validate", "role/finalize"}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	routes := make([]prompt.ModelRoute, 0, len(roles))
	for _, role := range roles {
		route, _ := modelroute.NewRouteID("route/v1/test")
		routes = append(routes, modelroute.ResolvedRoute{
			Role: role, Requested: modelroute.RouteRequest{RouteID: route}, PrimaryID: route,
			Primary:  modelroute.RouteRef{Provider: "provider-test", Model: "model-test"},
			Evidence: []modelroute.ResolutionEvidence{{ID: "provider-config:" + string(role), Source: modelroute.SourceProviderConfig, Provider: "provider-test", Route: route, ObservedAt: now, FreshUntil: now.Add(24 * time.Hour), Digest: "digest-test", Qualified: true, ReasonID: "route.configured"}},
		})
	}
	return prompt.ModelTable{Routes: routes}
}

func typedParityPresent(asset legacyoracle.Asset, prepared pipeline.PreparedWorkflowInstall) bool {
	if asset.ID == "install/rollback" {
		return prepared.Plan.OwnershipMarkers && prepared.Plan.Fingerprint != ""
	}
	for _, inventory := range prepared.Plan.Inventory {
		id := string(inventory.SemanticID)
		if strings.HasPrefix(asset.ID, "skill/") && (id == asset.ID || strings.HasSuffix(id, "/"+asset.ID) || strings.HasSuffix(inventory.Path, "/skills/"+strings.TrimPrefix(asset.ID, "skill/")+"/SKILL.md")) {
			return true
		}
	}
	for _, bundle := range prepared.Bundles {
		for _, emitted := range bundle.Bundle.Assets {
			id := string(emitted.SemanticID)
			content := strings.ToLower(string(emitted.Content))
			switch {
			case strings.HasPrefix(asset.ID, "skill/") && (id == asset.ID || strings.HasSuffix(id, "/"+asset.ID)):
				return true
			case strings.HasPrefix(asset.ID, "command/") && id == asset.ID:
				return true
			case asset.ID == "orchestrator/root" && (strings.Contains(id, "root") || strings.Contains(emitted.Path, "root-index") || emitted.Kind == "instruction"):
				return true
			case asset.ID == "orchestrator/reference" && (strings.Contains(id, "module") || strings.Contains(emitted.Path, "sdd-root") || strings.Contains(string(emitted.Content), "routing-and-risk")):
				return true
			case asset.ID == "orchestrator/single" && (emitted.Kind == "agent" || strings.Contains(id, "role") || emitted.Kind == "instruction"):
				return true
			case asset.ID == "role/sub-agents" && (emitted.Kind == "agent" || strings.Contains(id, "/skill/")):
				return true
			case asset.ID == "permissions/role-matrix" && (emitted.Kind == "permission" || strings.Contains(emitted.Path, "security")):
				return true
			case asset.ID == "models/role-routing" && (strings.Contains(id, "model") || strings.Contains(emitted.Path, "model") || strings.Contains(content, "model_routes") || strings.Contains(content, "model-routes")):
				return true
			}
		}
	}
	return false
}

func TestLegacyParityClassifiesOnlyIntentionalRetirements(t *testing.T) {
	oracle, err := legacyoracle.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range oracle.Assets {
		if asset.Classification == legacyoracle.Retire && strings.TrimSpace(asset.RetirementReason) == "" {
			t.Errorf("unexpected unqualified retirement %q: %s", asset.ID, asset.RetirementReason)
		}
	}
}
