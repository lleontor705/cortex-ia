package prompt

import (
	"encoding/json"
	"strings"
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
	workflow := ir.WorkflowIR{Roles: []ir.Role{{ID: "role/implement", Objective: "apply", AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write"}}}}
	routes := make([]ModelRoute, 0, len(canonicalRoles))
	for _, role := range canonicalRoles {
		routes = append(routes, qualifiedRoute(role))
	}
	assets, _, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{"asset/root": []byte("root")}, Workflow: workflow, Adapter: validAdapterContract(), AllowedPermissions: []string{"filesystem/read"}, Models: ModelTable{Routes: routes}})
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range assets {
		if asset.ID == "asset/role/implement/binding" && len(asset.Permissions) != 1 {
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

func TestMaterializeMakesMappedSkillTheFirstPhaseAction(t *testing.T) {
	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	workflow := ir.WorkflowIR{Roles: make([]ir.Role, 0, len(canonicalRoles))}
	for _, role := range canonicalRoles {
		workflow.Roles = append(workflow.Roles, ir.Role{ID: role, Objective: string(role)})
	}

	for _, tt := range []struct {
		name     string
		native   bool
		expected string
	}{
		{name: "native preload", native: true, expected: "First phase action: load native skill preload"},
		{name: "mandatory fallback read", expected: "First phase action: read the required fallback skill"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			adapter := validAdapterContract()
			adapter.NativeSkillPreload = tt.native
			assets, _, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{"asset/root": []byte("root")}, Workflow: workflow, Adapter: adapter, AllowedPermissions: []string{"filesystem/read"}, Models: ModelTable{Routes: allModelRoutes()}})
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range canonicalRoles {
				assetID := ir.SemanticID("asset/role/" + strings.TrimPrefix(string(role), "role/") + "/binding")
				asset, ok := materializedAssetByID(assets, assetID)
				if !ok {
					t.Fatalf("missing materialized asset %q", assetID)
				}
				if asset.Route.Role != role {
					t.Fatalf("%q route role = %q, want preserved role identity", role, asset.Route.Role)
				}
				skill, err := CanonicalSkillForRole(role)
				if err != nil {
					t.Fatal(err)
				}
				expectedPath := "internal/assets/skills/" + strings.TrimPrefix(string(skill), "skill/") + "/SKILL.md"
				content := string(asset.Content)
				firstAction := strings.Index(content, "First phase action:")
				if firstAction < 0 || !strings.HasPrefix(content[firstAction:], tt.expected) {
					t.Fatalf("%q first action = %q, want prefix %q", role, content, tt.expected)
				}
				if !strings.Contains(content[firstAction:], "`"+expectedPath+"`") {
					t.Fatalf("%q first action does not load mapped skill %q: %s", role, expectedPath, content)
				}
				for _, otherRole := range canonicalRoles {
					otherSkill, err := CanonicalSkillForRole(otherRole)
					if err != nil {
						t.Fatal(err)
					}
					if otherSkill != skill && strings.Contains(content, "/"+string(otherSkill)+"/SKILL.md") {
						t.Fatalf("%q loads another phase skill %q: %s", role, otherSkill, content)
					}
				}
				if strings.Contains(content[firstAction:], "Continue phase work") && strings.Index(content[firstAction:], "Continue phase work") < strings.Index(content[firstAction:], "`"+expectedPath+"`") {
					t.Fatalf("%q permits phase work before its mapped skill: %s", role, content)
				}
			}
		})
	}
}

func TestMaterializeRejectsMalformedCanonicalBindingsBeforeOutput(t *testing.T) {
	original := canonicalPhaseRoleBindings
	t.Cleanup(func() { canonicalPhaseRoleBindings = original })
	canonicalPhaseRoleBindings = append([]PhaseRoleBinding(nil), original...)
	canonicalPhaseRoleBindings[0].Skill = "skill/implement"

	catalog := ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: []ir.AssetSpec{{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "root.md", Required: true, SHA256: "hash"}}}
	assets, degradations, err := Materialize(MaterializerInput{Catalog: catalog, Contents: map[ir.SemanticID][]byte{"asset/root": []byte("root")}, Adapter: validAdapterContract(), AllowedPermissions: []string{"filesystem/read"}, Models: ModelTable{Routes: allModelRoutes()}})
	if err == nil {
		t.Fatal("Materialize accepted crossed canonical bindings")
	}
	if len(assets) != 0 || len(degradations) != 0 {
		t.Fatalf("Materialize emitted output for malformed bindings: assets=%v degradations=%v", assets, degradations)
	}
	if !strings.Contains(err.Error(), "canonical phase-role binding") {
		t.Fatalf("Materialize error = %v, want canonical binding failure", err)
	}
}

func materializedAssetByID(assets []MaterializedAsset, id ir.SemanticID) (MaterializedAsset, bool) {
	for _, asset := range assets {
		if asset.ID == id {
			return asset, true
		}
	}
	return MaterializedAsset{}, false
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
