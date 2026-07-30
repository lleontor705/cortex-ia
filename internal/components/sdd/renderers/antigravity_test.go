package renderers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateAntigravityGoldens = flag.Bool("update-antigravity", false, "update isolated Antigravity renderer goldens")

func TestAntigravityRendererMatchesDeterministicProfileGoldens(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		golden  string
	}{
		{name: "sequential compatible", profile: "portable-sequential", golden: "portable-sequential.golden"},
		{name: "qualified experimental native", profile: "native-advanced", golden: "native-advanced.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := antigravityResolvedWorkflow(tt.profile)
			var first Bundle
			for run := 0; run < 3; run++ {
				got, err := Render(context.Background(), NewAntigravityRenderer(), resolved)
				if err != nil {
					t.Fatalf("Render() run %d error = %v", run+1, err)
				}
				if run == 0 {
					first = got
				} else if !reflect.DeepEqual(got, first) {
					t.Fatalf("Render() run %d was not byte deterministic", run+1)
				}
			}

			got := antigravityGolden(t, first)
			goldenPath := filepath.Join("testdata", "antigravity", tt.golden)
			if *updateAntigravityGoldens {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", tt.profile, got, want)
			}
		})
	}
}

func TestAntigravityRendererPreservesPortableSemanticsAcrossProfiles(t *testing.T) {
	sequential, err := Render(context.Background(), NewAntigravityRenderer(), antigravityResolvedWorkflow("portable-sequential"))
	if err != nil {
		t.Fatal(err)
	}
	native, err := Render(context.Background(), NewAntigravityRenderer(), antigravityResolvedWorkflow("native-advanced"))
	if err != nil {
		t.Fatal(err)
	}

	sequentialManifest := antigravityAsset(t, sequential, ".gemini/antigravity/manifests/semantic.json")
	nativeManifest := antigravityAsset(t, native, ".gemini/antigravity/manifests/semantic.json")
	var sequentialDocument, nativeDocument struct {
		SemanticDigest string          `json:"semantic_digest"`
		Portable       json.RawMessage `json:"portable"`
	}
	if err := json.Unmarshal(sequentialManifest.Content, &sequentialDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(nativeManifest.Content, &nativeDocument); err != nil {
		t.Fatal(err)
	}
	if sequentialDocument.SemanticDigest == "" || sequentialDocument.SemanticDigest != nativeDocument.SemanticDigest {
		t.Fatalf("semantic digests differ: sequential=%q native=%q", sequentialDocument.SemanticDigest, nativeDocument.SemanticDigest)
	}
	if !bytes.Equal(sequentialDocument.Portable, nativeDocument.Portable) {
		t.Fatalf("portable semantics differ:\nsequential=%s\nnative=%s", sequentialDocument.Portable, nativeDocument.Portable)
	}

	for _, bundle := range []Bundle{sequential, native} {
		for _, asset := range bundle.Assets {
			if !strings.HasPrefix(asset.Path, ".gemini/antigravity/") {
				t.Errorf("asset path %q escaped the Antigravity install root", asset.Path)
			}
			lower := strings.ToLower(string(asset.Content))
			if strings.Contains(lower, "actual-secret") || strings.Contains(lower, "token=") {
				t.Errorf("asset %q rendered secret material", asset.Path)
			}
		}
	}
}

func TestAntigravityNativeProfileRequiresQualifiedCapabilitiesAndExplicitOptIn(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ResolvedWorkflow)
		want   string
	}{
		{
			name: "missing explicit opt in",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Extensions = nil
			},
			want: "explicit opt-in",
		},
		{
			name: "direct child is advisory",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Capabilities[0].State = resolution.StateAdvisory
			},
			want: "delegation/direct-child",
		},
		{
			name: "nested runtime qualification missing",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Capabilities = resolved.Capabilities[:1]
			},
			want: "delegation/nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := antigravityResolvedWorkflow("native-advanced")
			tt.mutate(&resolved)
			_, err := Render(context.Background(), NewAntigravityRenderer(), resolved)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Render() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestAntigravityRendererRejectsUnsupportedProfiles(t *testing.T) {
	resolved := antigravityResolvedWorkflow("portable-flat")
	_, err := Render(context.Background(), NewAntigravityRenderer(), resolved)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.ID != ErrorInvalidResolvedWorkflow {
		t.Fatalf("Render() error = %v, want %q ValidationError", err, ErrorInvalidResolvedWorkflow)
	}
}

func antigravityResolvedWorkflow(profile string) ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	resolved := ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version,
			ID:            "workflow/software-change",
			Version:       version,
			Roles: []ir.Role{
				{
					ID:             "role/implement",
					Objective:      "Implement one bounded work unit with test evidence.",
					Inputs:         []ir.Contract{{ID: "contract/task", SchemaVersion: version, Required: true}},
					Outputs:        []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}},
					NonGoals:       []string{"coordinate unrelated tasks"},
					AllowedEffects: []ir.Effect{"effect/read", "effect/write"},
					Evidence:       []ir.SemanticID{"evidence/test"},
					TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed},
				},
			},
			Phases:   []ir.Phase{{ID: "phase/apply", Role: "role/implement"}},
			Tools:    []ir.ToolRequirement{{ID: "tool/filesystem", Required: true}},
			Context:  ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustTrustedPolicy, ir.TrustRepositoryData, ir.TrustSecretReference}},
			Services: []ir.ServiceRequirement{{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}}},
		},
		Target:                "antigravity",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetAgent, AssetSchema, AssetPermission},
		AllowedPermissions:    []string{"filesystem/read", "filesystem/write", "mcp/forgespec"},
	}
	if profile == "native-advanced" {
		resolved.Extensions = []ExtensionDeclaration{{ID: "antigravity/experimental-native-opt-in", Optional: true}}
		resolved.Capabilities = []resolution.Resolution{
			antigravityNativeResolution("delegation/direct-child"),
			antigravityNativeResolution("delegation/nested"),
		}
	}
	return resolved
}

func antigravityNativeResolution(id capability.CapabilityID) resolution.Resolution {
	evidence := resolution.EvidenceRef("evidence/runtime/" + strings.TrimPrefix(string(id), "delegation/"))
	return resolution.Resolution{
		ID:    id,
		State: resolution.StateNative,
		Binding: resolution.Binding{
			ID:           ir.SemanticID("binding/antigravity-" + strings.TrimPrefix(string(id), "delegation/")),
			CapabilityID: id,
			Kind:         resolution.BindingNative,
			Evidence:     []resolution.EvidenceRef{evidence},
			Guarantee:    resolution.GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
		Evidence:        []resolution.EvidenceRef{evidence},
		Guarantee:       resolution.GuaranteeEnforced,
		PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}},
		Reason:          "qualified runtime evidence with operator opt-in",
	}
}

func antigravityAsset(t *testing.T, bundle Bundle, path string) Asset {
	t.Helper()
	for _, asset := range bundle.Assets {
		if asset.Path == path {
			return asset
		}
	}
	t.Fatalf("bundle missing %q", path)
	return Asset{}
}

func antigravityGolden(t *testing.T, bundle Bundle) []byte {
	t.Helper()
	type goldenAsset struct {
		Path        string          `json:"path"`
		SemanticID  ir.SemanticID   `json:"semantic_id"`
		Kind        AssetKind       `json:"kind"`
		Mode        string          `json:"mode"`
		Permissions []string        `json:"permissions"`
		Extensions  []ir.SemanticID `json:"extensions"`
		Content     string          `json:"content"`
	}
	assets := make([]goldenAsset, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		assets[index] = goldenAsset{
			Path: asset.Path, SemanticID: asset.SemanticID, Kind: asset.Kind,
			Mode: fmt.Sprintf("%#o", asset.Mode), Permissions: asset.Permissions,
			Extensions: asset.Extensions, Content: string(asset.Content),
		}
	}
	output, err := json.MarshalIndent(assets, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(output, '\n')
}
