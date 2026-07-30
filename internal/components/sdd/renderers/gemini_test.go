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

var updateGeminiGoldens = flag.Bool("update-gemini", false, "update isolated Gemini renderer goldens")

func TestGeminiRendererGoldensAreDeterministic(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		golden  string
		optIn   bool
	}{
		{name: "portable sequential", profile: "portable-sequential", golden: "portable-sequential.golden"},
		{name: "portable flat", profile: "portable-flat", golden: "portable-flat.golden"},
		{name: "native advanced with explicit opt-in", profile: "native-advanced", golden: "native-advanced.golden", optIn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := geminiResolvedWorkflow(tt.profile, tt.optIn)
			first, err := Render(context.Background(), NewGeminiRenderer(), resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			second, err := Render(context.Background(), NewGeminiRenderer(), resolved)
			if err != nil {
				t.Fatalf("Render() second error = %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("repeated Gemini render differed:\nfirst=%+v\nsecond=%+v", first, second)
			}

			actual := formatGoldenBundle(first)
			goldenPath := filepath.Join("testdata", "gemini", tt.golden)
			if *updateGeminiGoldens {
				if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(actual, expected) {
				t.Fatalf("Gemini bundle differs from %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, actual, expected)
			}
		})
	}
}

func TestGeminiRendererEmitsCompleteSemanticManifest(t *testing.T) {
	bundle, err := Render(context.Background(), NewGeminiRenderer(), geminiResolvedWorkflow("portable-flat", false))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	manifestAsset := findGeminiAsset(t, bundle, ".cortex-ia/manifests/semantic.json")
	var document struct {
		Target                string                  `json:"target"`
		Profile               string                  `json:"profile"`
		GenerationFingerprint string                  `json:"generation_fingerprint"`
		Workflow              ir.SemanticID           `json:"workflow"`
		Roles                 []ir.SemanticID         `json:"roles"`
		Phases                []ir.SemanticID         `json:"phases"`
		Permissions           []string                `json:"permissions"`
		Trust                 []ir.TrustClass         `json:"trust"`
		Capabilities          []resolution.Resolution `json:"capabilities"`
	}
	if err := json.Unmarshal(manifestAsset.Content, &document); err != nil {
		t.Fatalf("semantic manifest is invalid JSON: %v", err)
	}
	if document.Target != "gemini" || document.Profile != "portable-flat" || document.Workflow != "workflow/review" {
		t.Fatalf("semantic manifest identity = %+v", document)
	}
	if document.GenerationFingerprint != strings.Repeat("a", 64) {
		t.Fatalf("generation fingerprint = %q", document.GenerationFingerprint)
	}
	if !reflect.DeepEqual(document.Roles, []ir.SemanticID{"role/implement", "role/validate"}) ||
		!reflect.DeepEqual(document.Phases, []ir.SemanticID{"phase/apply", "phase/verify"}) ||
		!reflect.DeepEqual(document.Permissions, []string{"filesystem/read", "tool/search"}) ||
		!reflect.DeepEqual(document.Trust, []ir.TrustClass{ir.TrustRepositoryData, ir.TrustSecretReference, ir.TrustTrustedPolicy}) {
		t.Fatalf("semantic manifest portable invariants = %+v", document)
	}
}

func TestGeminiRendererRejectsUnsafeOrUnqualifiedBundles(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ResolvedWorkflow)
		wantID  ir.SemanticID
		wantRef ir.SemanticID
	}{
		{
			name: "unresolved variable",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Workflow.Roles[1].Objective = "Implement for {{ repository }}."
			},
			wantID:  ErrorUnresolvedVariable,
			wantRef: "role/implement",
		},
		{
			name: "permission widening",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Workflow.Roles[1].AllowedEffects = append(resolved.Workflow.Roles[1].AllowedEffects, ir.Effect("network/write"))
			},
			wantID:  ErrorPermissionWidening,
			wantRef: "role/implement",
		},
		{
			name: "rendered secret material",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Workflow.Roles[1].Objective = "Use token=super-secret-value."
			},
			wantID:  ErrorGeminiSecretMaterial,
			wantRef: "role/implement",
		},
		{
			name: "native profile without opt-in",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Profile = "native-advanced"
			},
			wantID:  ErrorGeminiNativeOptIn,
			wantRef: "workflow/review",
		},
		{
			name: "native profile without qualified isolation",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Profile = "native-advanced"
				resolved.Extensions = []ExtensionDeclaration{{ID: GeminiNativeOptInExtension, Optional: true}}
				resolved.Capabilities = nil
			},
			wantID:  ErrorGeminiNativeCapability,
			wantRef: "workflow/review",
		},
		{
			name: "invalid generation fingerprint",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.GenerationFingerprint = "not-a-digest"
			},
			wantID:  ErrorGeminiManifest,
			wantRef: "workflow/review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := geminiResolvedWorkflow("portable-flat", false)
			tt.mutate(&resolved)
			_, err := Render(context.Background(), NewGeminiRenderer(), resolved)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.ID != tt.wantID || validationErr.SemanticID != tt.wantRef {
				t.Fatalf("Render() error = %v, want validation ID %q and source %q", err, tt.wantID, tt.wantRef)
			}
		})
	}
}

func geminiResolvedWorkflow(profile string, nativeOptIn bool) ResolvedWorkflow {
	resolved := ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			ID:            "workflow/review",
			Version:       ir.MustParseVersion("1.2.0"),
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Verify outcomes independently.", AllowedEffects: []ir.Effect{"filesystem/read"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
				{ID: "role/implement", Objective: "Implement a bounded work unit.", AllowedEffects: []ir.Effect{"tool/search", "filesystem/read"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			},
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement"},
			},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustSecretReference, ir.TrustTrustedPolicy, ir.TrustRepositoryData}},
		},
		Target:                "gemini",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities: []resolution.Resolution{{
			ID: "delegation/direct-child", State: resolution.StateNative,
			Binding:  resolution.Binding{ID: "binding/gemini/direct-child", CapabilityID: "delegation/direct-child", Kind: resolution.BindingNative, Evidence: []resolution.EvidenceRef{"evidence/gemini/schema"}, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime},
			Evidence: []resolution.EvidenceRef{"evidence/gemini/schema"}, Guarantee: resolution.GuaranteeEnforced,
		}},
		AllowedAssetKinds:  []AssetKind{AssetInstruction, AssetAgent, AssetPermission, AssetSchema},
		AllowedPermissions: []string{"tool/search", "filesystem/read"},
	}
	if nativeOptIn {
		resolved.Extensions = []ExtensionDeclaration{{ID: GeminiNativeOptInExtension, Optional: true}}
		resolved.Capabilities = append(resolved.Capabilities, resolution.Resolution{
			ID: "isolation/tool-scope", State: resolution.StateNative,
			Binding:  resolution.Binding{ID: "binding/gemini/tool-scope", CapabilityID: "isolation/tool-scope", Kind: resolution.BindingNative, Evidence: []resolution.EvidenceRef{"evidence/gemini/schema"}, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime},
			Evidence: []resolution.EvidenceRef{"evidence/gemini/schema"}, Guarantee: resolution.GuaranteeEnforced,
		})
	}
	return resolved
}

func findGeminiAsset(t *testing.T, bundle Bundle, assetPath string) Asset {
	t.Helper()
	for _, asset := range bundle.Assets {
		if asset.Path == assetPath {
			return asset
		}
	}
	t.Fatalf("bundle omitted %s", assetPath)
	return Asset{}
}

func formatGoldenBundle(bundle Bundle) []byte {
	var output strings.Builder
	for _, asset := range bundle.Assets {
		fmt.Fprintf(&output, "== %s | %s | %s | %#o ==\n", asset.Path, asset.SemanticID, asset.Kind, asset.Mode)
		if len(asset.Permissions) > 0 {
			fmt.Fprintf(&output, "permissions: %s\n", strings.Join(asset.Permissions, ","))
		}
		if len(asset.Extensions) > 0 {
			extensions := make([]string, len(asset.Extensions))
			for index := range asset.Extensions {
				extensions[index] = string(asset.Extensions[index])
			}
			fmt.Fprintf(&output, "extensions: %s\n", strings.Join(extensions, ","))
		}
		output.Write(asset.Content)
		if !bytes.HasSuffix(asset.Content, []byte("\n")) {
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}
