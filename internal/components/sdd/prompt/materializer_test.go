package prompt

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
)

func TestMaterializePreservesMetadataSentinel(t *testing.T) {
	sentinel := json.RawMessage(`{"profile":"profile-sentinel","quality":"quality-sentinel","trust":"trust-sentinel"}`)
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	assets, _, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{"asset/root": []byte("root")}, Adapter: validAdapterContract(), AllowedPermissions: []string{"filesystem/read"}, Models: ModelTable{Routes: allModelRoutes()}, Metadata: sentinel})
	if err != nil {
		t.Fatal(err)
	}
	if string(assets[0].Metadata) != string(sentinel) {
		t.Fatalf("materializer lost metadata: %s", assets[0].Metadata)
	}
}

func TestMaterializeRejectsMissingRequiredAssetBeforeEffects(t *testing.T) {
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	_, _, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{}, Adapter: validAdapterContract(), AllowedPermissions: []string{"filesystem/read"}})
	if err == nil {
		t.Fatal("Materialize accepted a required asset with no bytes")
	}
}

func TestMaterializeIntersectsRolePermissionsFailClosed(t *testing.T) {
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	workflow := ir.WorkflowIR{Roles: []ir.Role{{ID: "role/apply", Objective: "apply", AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write"}}}}
	routes := make([]ModelRoute, 0, len(canonicalRoles))
	for _, role := range canonicalRoles {
		routes = append(routes, qualifiedRoute(role))
	}
	assets, _, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{"asset/root": []byte("root")}, Workflow: workflow, Adapter: validAdapterContract(), AllowedPermissions: []string{"filesystem/read"}, Models: ModelTable{Routes: routes}})
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range assets {
		if asset.ID == "asset/role/apply/binding" && len(asset.Permissions) != 1 {
			t.Fatalf("permissions widened: %v", asset.Permissions)
		}
	}
}

func TestMaterializeResolvesTypedTemplateContext(t *testing.T) {
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	assets, _, err := Materialize(MaterializerInput{
		Catalog:            catalog,
		Contents:           map[ir.SemanticID][]byte{"asset/root": []byte("Skills root: {{SKILLS_DIR}}\nHome: {{HOME}}")},
		Adapter:            validAdapterContract(),
		AllowedPermissions: []string{"filesystem/read"},
		Models:             ModelTable{Routes: allModelRoutes()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(assets[0].Content); got != "Skills root: internal/assets/skills\nHome: .claude" {
		t.Fatalf("typed template materialization = %q", got)
	}
}

func allModelRoutes() []ModelRoute {
	routes := make([]ModelRoute, 0, len(canonicalRoles))
	for _, role := range canonicalRoles {
		routes = append(routes, qualifiedRoute(role))
	}
	return routes
}

func qualifiedRoute(role ir.SemanticID) ModelRoute {
	route, _ := modelroute.NewRouteID("route/v1/test")
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	return ModelRoute{Role: role, Requested: modelroute.RouteRequest{RouteID: route}, PrimaryID: route, Primary: modelroute.RouteRef{Provider: "provider-test", Model: "model-test"}, Evidence: []modelroute.ResolutionEvidence{{ID: "evidence-test-" + string(role), Source: modelroute.SourceProviderConfig, Provider: "provider-test", Route: route, ObservedAt: now.Add(-time.Minute), FreshUntil: now.Add(time.Hour), Digest: "digest-test", Qualified: true}}}
}
