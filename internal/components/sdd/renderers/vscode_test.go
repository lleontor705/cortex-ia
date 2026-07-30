package renderers

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateVSCodeGoldens = flag.Bool("update-vscode-goldens", false, "update isolated VS Code renderer goldens")

func TestVSCodeRendererDeterministicProfileGoldens(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []resolution.Resolution
	}{
		{
			name: "portable-sequential",
			capabilities: []resolution.Resolution{
				unsupportedVSCodeCapability("delegation/direct-child", "not required by the sequential profile"),
				unsupportedVSCodeCapability("delegation/nested", "VS Code does not support nested delegation"),
			},
		},
		{
			name: "advertised-direct-child",
			capabilities: []resolution.Resolution{
				{
					ID: "delegation/direct-child", State: resolution.StateAdvisory,
					Binding:  resolution.Binding{ID: "binding/vscode/direct-child", CapabilityID: "delegation/direct-child", Kind: resolution.BindingAdvisory, Evidence: []resolution.EvidenceRef{"evidence/vscode/direct-child-docs"}, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt},
					Evidence: []resolution.EvidenceRef{"evidence/vscode/direct-child-docs"}, Guarantee: resolution.GuaranteeBestEffort,
					PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "advertised preview is documentation-backed, not runtime-qualified",
				},
				unsupportedVSCodeCapability("delegation/nested", "VS Code does not support nested delegation"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := vscodeResolvedWorkflow(tt.capabilities)
			var first Bundle
			for attempt := 0; attempt < 3; attempt++ {
				got, err := Render(context.Background(), NewVSCodeRenderer(), resolved)
				if err != nil {
					t.Fatalf("Render() attempt %d error = %v", attempt+1, err)
				}
				if attempt == 0 {
					first = got
				} else if !reflect.DeepEqual(got, first) {
					t.Fatalf("Render() attempt %d was not deterministic", attempt+1)
				}
			}

			golden, err := json.MarshalIndent(first, "", "  ")
			if err != nil {
				t.Fatalf("marshal bundle: %v", err)
			}
			golden = append(golden, '\n')
			goldenPath := filepath.Join("testdata", "vscode", tt.name+".golden")
			if *updateVSCodeGoldens {
				if err := os.WriteFile(goldenPath, golden, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if string(golden) != string(want) {
				t.Fatalf("bundle differs from %s; rerun with -update-vscode-goldens after reviewing the intended change", goldenPath)
			}
		})
	}
}

func TestVSCodeRendererDisclosesSecurityWithoutUnsupportedNesting(t *testing.T) {
	resolved := vscodeResolvedWorkflow([]resolution.Resolution{
		unsupportedVSCodeCapability("delegation/nested", "VS Code does not support nested delegation"),
	})
	bundle, err := Render(context.Background(), NewVSCodeRenderer(), resolved)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assets := make(map[string]Asset, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		assets[asset.Path] = asset
	}
	instructions := string(assets[".github/copilot-instructions.md"].Content)
	for _, forbidden := range []string{"team-lead", "task()", "delegate to", "nested agent"} {
		if strings.Contains(strings.ToLower(instructions), forbidden) {
			t.Errorf("instructions contain unsupported delegation phrase %q", forbidden)
		}
	}
	if !strings.Contains(instructions, "Execute phases sequentially") {
		t.Fatal("instructions do not preserve sequential execution")
	}

	var security struct {
		RequestedPermissions []string `json:"requested_permissions"`
		EffectivePermissions []string `json:"effective_permissions"`
		ContainsSecretValues bool     `json:"contains_secret_values"`
		NestedDelegation     bool     `json:"nested_delegation"`
	}
	if err := json.Unmarshal(assets["manifests/security.json"].Content, &security); err != nil {
		t.Fatalf("decode security manifest: %v", err)
	}
	if !reflect.DeepEqual(security.RequestedPermissions, security.EffectivePermissions) || security.ContainsSecretValues || security.NestedDelegation {
		t.Fatalf("unsafe security manifest: %+v", security)
	}

	for _, required := range []string{"manifests/semantic.json", "manifests/security.json", "manifests/security.md", "manifests/degradation.json", "manifests/degradation.md"} {
		if _, ok := assets[required]; !ok {
			t.Errorf("missing required manifest %q", required)
		}
	}
}

func TestVSCodeRendererRejectsUnsupportedNestingAndPermissionWidening(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []resolution.Resolution
		wantID       ir.SemanticID
	}{
		{
			name: "nested delegation claim",
			capabilities: []resolution.Resolution{{
				ID: "delegation/nested", State: resolution.StateNative, Guarantee: resolution.GuaranteeEnforced,
				Binding: resolution.Binding{ID: "binding/vscode/nested", CapabilityID: "delegation/nested", Kind: resolution.BindingNative, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime},
			}},
			wantID: ErrorUnsupportedAsset,
		},
		{
			name: "capability permission widening",
			capabilities: []resolution.Resolution{{
				ID: "approval/destructive", State: resolution.StateAdvisory, Guarantee: resolution.GuaranteeBestEffort,
				Binding:         resolution.Binding{ID: "binding/vscode/approval", CapabilityID: "approval/destructive", Kind: resolution.BindingAdvisory, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt},
				PermissionDelta: resolution.PermissionDelta{Added: []string{"process/execute"}},
			}},
			wantID: ErrorPermissionWidening,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(context.Background(), NewVSCodeRenderer(), vscodeResolvedWorkflow(tt.capabilities))
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.ID != tt.wantID {
				t.Fatalf("Render() error = %v, want validation ID %q", err, tt.wantID)
			}
		})
	}
}

func vscodeResolvedWorkflow(capabilities []resolution.Resolution) ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version, ID: "workflow/sdd", Version: version,
			Roles:   []ir.Role{{ID: "role/implement", Objective: "Implement one bounded work unit", TerminalStates: []ir.TerminalState{ir.TerminalFailed, ir.TerminalSuccess}}},
			Phases:  []ir.Phase{{ID: "phase/apply", Role: "role/implement"}},
			Tools:   []ir.ToolRequirement{{ID: "tool/filesystem-read", Required: true}},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy}},
		},
		Target: "vscode", Profile: "portable-sequential", GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities:       capabilities,
		AllowedAssetKinds:  []AssetKind{AssetInstruction, AssetSchema},
		AllowedPermissions: []string{"filesystem/read", "filesystem/write"},
	}
}

func unsupportedVSCodeCapability(id capability.CapabilityID, reason string) resolution.Resolution {
	return resolution.Resolution{
		ID: id, State: resolution.StateUnsupported, Guarantee: resolution.GuaranteeNone,
		PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: reason,
	}
}
