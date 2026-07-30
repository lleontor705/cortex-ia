package renderers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var updateKilocodeGoldens = flag.Bool("update-kilocode", false, "update isolated Kilocode renderer goldens")

func TestKilocodeRendererDeterministicPortableBundles(t *testing.T) {
	tests := []struct {
		profile         string
		wantAgentAssets bool
		golden          string
	}{
		{profile: "portable-sequential", golden: "portable-sequential.golden"},
		{profile: "portable-flat", wantAgentAssets: true, golden: "portable-flat.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			resolved := kilocodeResolvedWorkflow(tt.profile)
			var runs [3]Bundle
			for index := range runs {
				bundle, err := Render(context.Background(), NewKilocodeRenderer(), resolved)
				if err != nil {
					t.Fatalf("Render() run %d error = %v", index+1, err)
				}
				runs[index] = bundle
			}
			if !reflect.DeepEqual(runs[0], runs[1]) || !reflect.DeepEqual(runs[1], runs[2]) {
				t.Fatal("three identical Kilocode generations were not byte-for-byte equal")
			}

			assertKilocodeTargetPaths(t, runs[0], tt.wantAgentAssets)
			assertKilocodeManifestComplete(t, runs[0], resolved)
			assertKilocodeGolden(t, tt.golden, snapshotKilocodeBundle(runs[0]))
		})
	}
}

func TestKilocodeRendererRejectsSecretMaterialBeforeBundleValidation(t *testing.T) {
	resolved := kilocodeResolvedWorkflow("portable-sequential")
	resolved.Workflow.Roles[0].Objective = "Use token=actual-secret-value"
	_, err := Render(context.Background(), NewKilocodeRenderer(), resolved)
	if err == nil || !strings.Contains(err.Error(), "secret material") {
		t.Fatalf("Render() error = %v, want secret material rejection", err)
	}
}

func assertKilocodeTargetPaths(t *testing.T, bundle Bundle, wantAgentAssets bool) {
	t.Helper()
	paths := make([]string, 0, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		paths = append(paths, asset.Path)
	}
	for _, required := range []string{
		"AGENTS.md",
		"commands/phase-apply.md",
		"commands/phase-verify.md",
		"manifests/bundle.json",
		"manifests/degradation.json",
		"manifests/degradation.md",
		"manifests/security.json",
		"manifests/security.md",
		"manifests/semantic.json",
		"opencode.json",
		"skills/role-implement/SKILL.md",
		"skills/role-validate/SKILL.md",
	} {
		if !containsString(paths, required) {
			t.Errorf("bundle paths omit required Kilocode target path %q: %v", required, paths)
		}
	}
	for _, role := range []string{"agents/role-implement.md", "agents/role-validate.md"} {
		if got := containsString(paths, role); got != wantAgentAssets {
			t.Errorf("presence of %q = %t, want %t for profile", role, got, wantAgentAssets)
		}
	}
}

func assertKilocodeManifestComplete(t *testing.T, bundle Bundle, resolved ResolvedWorkflow) {
	t.Helper()
	assets := make(map[string]Asset, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		assets[asset.Path] = asset
	}
	semanticAsset, ok := assets["manifests/semantic.json"]
	if !ok {
		t.Fatal("semantic manifest missing")
	}
	var manifest kilocodeSemanticManifest
	if err := json.Unmarshal(semanticAsset.Content, &manifest); err != nil {
		t.Fatalf("semantic manifest is invalid JSON: %v", err)
	}
	if manifest.Target != "kilocode" || manifest.Profile != resolved.Profile || manifest.WorkflowID != resolved.Workflow.ID {
		t.Fatalf("semantic manifest identity = %+v", manifest)
	}
	if !reflect.DeepEqual(manifest.TrustClasses, []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy}) {
		t.Fatalf("portable trust invariants = %v", manifest.TrustClasses)
	}
	if len(manifest.Roles) != 2 || len(manifest.Phases) != 2 || len(manifest.Services) != 2 {
		t.Fatalf("portable manifest omitted invariants: roles=%d phases=%d services=%d", len(manifest.Roles), len(manifest.Phases), len(manifest.Services))
	}

	wantPaths := make([]string, 0, len(bundle.Assets)-5)
	for _, asset := range bundle.Assets {
		if !strings.HasPrefix(asset.Path, "manifests/") {
			wantPaths = append(wantPaths, asset.Path)
		}
	}
	sort.Strings(wantPaths)
	gotPaths := make([]string, len(manifest.Assets))
	for index, item := range manifest.Assets {
		gotPaths[index] = item.Path
		if _, err := hex.DecodeString(item.SHA256); err != nil || len(item.SHA256) != 64 {
			t.Errorf("manifest hash for %q = %q", item.Path, item.SHA256)
		}
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("manifest asset paths = %v, want every non-manifest asset %v", gotPaths, wantPaths)
	}
	var bundleManifest kilocodeBundleManifest
	if err := json.Unmarshal(assets["manifests/bundle.json"].Content, &bundleManifest); err != nil {
		t.Fatalf("bundle manifest is invalid JSON: %v", err)
	}
	wantBundlePaths := make([]string, 0, len(bundle.Assets)-1)
	for _, asset := range bundle.Assets {
		if asset.Path != "manifests/bundle.json" {
			wantBundlePaths = append(wantBundlePaths, asset.Path)
		}
	}
	sort.Strings(wantBundlePaths)
	gotBundlePaths := make([]string, len(bundleManifest.Assets))
	for index, item := range bundleManifest.Assets {
		gotBundlePaths[index] = item.Path
	}
	if !reflect.DeepEqual(gotBundlePaths, wantBundlePaths) {
		t.Fatalf("bundle manifest paths = %v, want every other asset %v", gotBundlePaths, wantBundlePaths)
	}

	for _, path := range []string{"manifests/security.json", "manifests/security.md", "manifests/degradation.json", "manifests/degradation.md"} {
		content := assets[path].Content
		for _, invariant := range []string{"kilocode", resolved.Profile, "filesystem/read", "trusted_policy", "repository_data"} {
			if !bytes.Contains(content, []byte(invariant)) {
				t.Errorf("%s omits portable disclosure %q", path, invariant)
			}
		}
	}
	for _, disclosure := range []string{"approval_intent", "isolation_intent", "validation"} {
		if !bytes.Contains(assets["manifests/security.json"].Content, []byte(disclosure)) {
			t.Errorf("security manifest omits %q", disclosure)
		}
	}
}

func assertKilocodeGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "kilocode", name)
	if *updateKilocodeGoldens {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("bundle differs from %s; inspect and run go test ./internal/components/sdd/renderers -run TestKilocodeRenderer -update-kilocode", path)
	}
}

func snapshotKilocodeBundle(bundle Bundle) []byte {
	var output strings.Builder
	for _, asset := range bundle.Assets {
		fmt.Fprintf(&output, "--- %s | %s | %s | %#o ---\n", asset.Path, asset.SemanticID, asset.Kind, asset.Mode)
		output.Write(asset.Content)
		if len(asset.Content) == 0 || asset.Content[len(asset.Content)-1] != '\n' {
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}

func kilocodeResolvedWorkflow(profile string) ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version,
			ID:            "workflow/reviewable-change",
			Version:       version,
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Independently verify outcomes", Inputs: []ir.Contract{{ID: "contract/change", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/verification", SchemaVersion: version, Required: true}}, NonGoals: []string{"change production code"}, AllowedEffects: []ir.Effect{"effect/read"}, Evidence: []ir.SemanticID{"evidence/test"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
				{ID: "role/implement", Objective: "Implement one bounded vertical slice", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/change", SchemaVersion: version, Required: true}}, NonGoals: []string{"expand scope"}, AllowedEffects: []ir.Effect{"effect/write"}, Evidence: []ir.SemanticID{"evidence/test"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked}},
			},
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement"},
			},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy}},
			Services: []ir.ServiceRequirement{
				{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
				{ID: "service/cortex", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
			},
		},
		Target:                "kilocode",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetSkill, AssetCommand, AssetAgent, AssetPermission, AssetSchema},
		AllowedPermissions:    []string{"filesystem/read"},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
