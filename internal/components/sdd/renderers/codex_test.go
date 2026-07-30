package renderers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateCodexGoldens = flag.Bool("update-codex", false, "update Codex renderer golden files")

func TestCodexRendererDeterministicProfileGoldens(t *testing.T) {
	tests := []struct {
		name    string
		profile string
	}{
		{name: "portable sequential", profile: "portable-sequential"},
		{name: "portable flat", profile: "portable-flat"},
		{name: "qualified native advanced", profile: "native-advanced"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := codexResolvedWorkflow(tt.profile)
			var first Bundle
			for run := 0; run < 3; run++ {
				got, err := Render(context.Background(), NewCodexRenderer(), resolved)
				if err != nil {
					t.Fatalf("Render() run %d error = %v", run+1, err)
				}
				if run == 0 {
					first = got
				} else if !reflect.DeepEqual(first, got) {
					t.Fatalf("Render() run %d was not deterministic", run+1)
				}
			}

			assertCodexBundleSecurity(t, first, resolved)
			actual := codexGoldenSnapshot(t, first)
			goldenPath := filepath.Join("testdata", "codex", strings.ReplaceAll(tt.profile, "-", "_")+".golden.json")
			if *updateCodexGoldens {
				if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(actual, expected) {
				t.Fatalf("Codex golden mismatch for %s; run go test ./internal/components/sdd/renderers -run TestCodexRendererDeterministicProfileGoldens -update-codex", tt.profile)
			}
		})
	}
}

func TestCodexRendererRejectsUnsupportedProfileAndSecretValues(t *testing.T) {
	t.Run("unsupported profile", func(t *testing.T) {
		resolved := codexResolvedWorkflow("portable-flat")
		resolved.Profile = "portable-nested"
		if _, err := Render(context.Background(), NewCodexRenderer(), resolved); err == nil || !strings.Contains(err.Error(), "unsupported Codex profile") {
			t.Fatalf("Render() error = %v", err)
		}
	})

	t.Run("secret value", func(t *testing.T) {
		resolved := codexResolvedWorkflow("portable-sequential")
		resolved.Workflow.Roles[0].Objective = "Use token=actual-secret-value"
		if _, err := Render(context.Background(), NewCodexRenderer(), resolved); err == nil || !strings.Contains(err.Error(), "secret material") {
			t.Fatalf("Render() error = %v", err)
		}
	})
}

func assertCodexBundleSecurity(t *testing.T, bundle Bundle, resolved ResolvedWorkflow) {
	t.Helper()
	paths := make(map[string]Asset, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		if index > 0 && bundle.Assets[index-1].Path >= asset.Path {
			t.Fatalf("assets not strictly path ordered: %q then %q", bundle.Assets[index-1].Path, asset.Path)
		}
		if asset.Mode != fs.FileMode(0o644) {
			t.Errorf("%s mode = %#o, want 0644", asset.Path, asset.Mode)
		}
		if !reflect.DeepEqual(asset.Permissions, []string{"filesystem/read", "mcp/cortex", "mcp/forgespec"}) {
			t.Errorf("%s permissions = %v", asset.Path, asset.Permissions)
		}
		lower := strings.ToLower(string(asset.Content))
		for _, marker := range []string{"token=", "password=", "secret=", "authorization:", "begin private key", "sk-"} {
			if strings.Contains(lower, marker) {
				t.Errorf("%s contains secret marker %q", asset.Path, marker)
			}
		}
		paths[asset.Path] = asset
	}
	for _, required := range []string{"AGENTS.md", "manifests/bundle.json", "manifests/degradation.json", "manifests/degradation.md", "manifests/security.json", "manifests/security.md", "manifests/semantic.json"} {
		if _, found := paths[required]; !found {
			t.Errorf("bundle missing %s", required)
		}
	}
	if resolved.Profile == "portable-sequential" {
		for path := range paths {
			if strings.HasPrefix(path, "agents/") {
				t.Errorf("sequential profile emitted agent definition %s", path)
			}
		}
	} else if _, found := paths["agents/implement.toml"]; !found {
		t.Error("delegating profile omitted implement agent definition")
	}

	var security struct {
		RequestedPermissions []string `json:"requested_permissions"`
		EffectivePermissions []string `json:"effective_permissions"`
		SecretValues         []string `json:"secret_values"`
	}
	if err := json.Unmarshal(paths["manifests/security.json"].Content, &security); err != nil {
		t.Fatalf("decode security manifest: %v", err)
	}
	if !reflect.DeepEqual(security.RequestedPermissions, security.EffectivePermissions) || len(security.SecretValues) != 0 {
		t.Fatalf("security manifest widened permissions or rendered secrets: %+v", security)
	}

	var bundleManifest struct {
		Assets []codexBundleHash `json:"assets"`
	}
	if err := json.Unmarshal(paths["manifests/bundle.json"].Content, &bundleManifest); err != nil {
		t.Fatalf("decode bundle manifest: %v", err)
	}
	if len(bundleManifest.Assets) != len(bundle.Assets)-1 {
		t.Fatalf("bundle manifest hashes %d assets, want %d", len(bundleManifest.Assets), len(bundle.Assets)-1)
	}
	for _, recorded := range bundleManifest.Assets {
		asset, found := paths[recorded.Path]
		if !found || recorded.Path == "manifests/bundle.json" {
			t.Fatalf("bundle manifest references invalid asset %q", recorded.Path)
		}
		digest := sha256.Sum256(asset.Content)
		if recorded.Mode != uint32(asset.Mode) || recorded.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("bundle manifest evidence differs for %s", recorded.Path)
		}
	}
}

func codexGoldenSnapshot(t *testing.T, bundle Bundle) []byte {
	t.Helper()
	type assetSnapshot struct {
		Path       string `json:"path"`
		SemanticID string `json:"semantic_id"`
		Kind       string `json:"kind"`
		Mode       uint32 `json:"mode"`
		SHA256     string `json:"sha256"`
		Content    string `json:"content"`
	}
	snapshot := make([]assetSnapshot, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		digest := sha256.Sum256(asset.Content)
		snapshot[index] = assetSnapshot{Path: asset.Path, SemanticID: string(asset.SemanticID), Kind: string(asset.Kind), Mode: uint32(asset.Mode), SHA256: hex.EncodeToString(digest[:]), Content: string(asset.Content)}
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func codexResolvedWorkflow(profile string) ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	capabilities := []resolution.Resolution{}
	if profile != "portable-sequential" {
		capabilities = append(capabilities, codexResolution("delegation/direct-child", "binding/codex-direct-child", "evidence/codex/direct-child"))
	}
	if profile == "native-advanced" {
		capabilities = append(capabilities, codexResolution("delegation/parallel", "binding/codex-parallel", "evidence/codex/parallel"))
	}
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version,
			ID:            "workflow/sdd",
			Version:       version,
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Independently verify required outcomes", AllowedEffects: []ir.Effect{"effect/read"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
				{ID: "role/orchestrator", Objective: "Route ready work without becoming task authority", AllowedEffects: []ir.Effect{"effect/coordinate"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked}},
				{ID: "role/implement", Objective: "Implement one bounded vertical work unit", AllowedEffects: []ir.Effect{"effect/read", "effect/write"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			},
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement"},
			},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy, ir.TrustSecretReference}},
			Services: []ir.ServiceRequirement{
				{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
				{ID: "service/cortex", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
			},
		},
		Target:                "codex",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities:          capabilities,
		AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetSkill, AssetAgent, AssetPermission, AssetSchema},
		AllowedPermissions:    []string{"mcp/forgespec", "filesystem/read", "mcp/cortex"},
	}
}

func codexResolution(id, binding, evidence string) resolution.Resolution {
	return resolution.Resolution{
		ID: ir.SemanticID(id), State: resolution.StateNative,
		Binding:  resolution.Binding{ID: ir.SemanticID(binding), CapabilityID: ir.SemanticID(id), Kind: resolution.BindingNative, Evidence: []resolution.EvidenceRef{resolution.EvidenceRef(evidence)}, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime},
		Evidence: []resolution.EvidenceRef{resolution.EvidenceRef(evidence)}, Guarantee: resolution.GuaranteeEnforced,
		PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "qualified Codex runtime support",
	}
}
