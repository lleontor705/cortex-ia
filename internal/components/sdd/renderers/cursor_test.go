package renderers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

var updateCursorGoldens = flag.Bool("update-cursor", false, "update Cursor renderer golden files")

func TestCursorRendererSequentialAndQualifiedProfiles(t *testing.T) {
	tests := []struct {
		name     string
		resolved ResolvedWorkflow
		golden   string
	}{
		{name: "portable sequential", resolved: cursorSequentialWorkflow(), golden: "portable-sequential.golden"},
		{name: "qualified native advanced", resolved: cursorNativeWorkflow(), golden: "native-advanced.golden"},
	}

	var semanticDigest string
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := Render(context.Background(), NewCursorRenderer(), tt.resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				next, err := Render(context.Background(), NewCursorRenderer(), tt.resolved)
				if err != nil {
					t.Fatalf("Render() repeat %d error = %v", attempt+1, err)
				}
				if !reflect.DeepEqual(first, next) {
					t.Fatalf("Render() repeat %d differed", attempt+1)
				}
			}

			profile := cursorProfileDocument(t, first)
			if !strings.HasPrefix(profile.WorkflowSemanticDigest, "sha256:") || len(profile.WorkflowSemanticDigest) != len("sha256:")+sha256.Size*2 {
				t.Fatalf("workflow semantic digest = %q", profile.WorkflowSemanticDigest)
			}
			if want := expectedCursorWorkflowDigest(t, tt.resolved.Workflow); profile.WorkflowSemanticDigest != want {
				t.Fatalf("workflow semantic digest = %q, want %q", profile.WorkflowSemanticDigest, want)
			}
			if semanticDigest == "" {
				semanticDigest = profile.WorkflowSemanticDigest
			} else if profile.WorkflowSemanticDigest != semanticDigest {
				t.Fatalf("profile changed portable semantic digest: got %s want %s", profile.WorkflowSemanticDigest, semanticDigest)
			}
			assertCursorProfileDisclosure(t, tt.resolved.Profile, profile)
			assertCursorPortableSemantics(t, first)
			assertCursorGolden(t, tt.golden, first)
		})
	}
}

func TestCursorRendererRejectsUnqualifiedOrUndeclaredNativeExtensions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ResolvedWorkflow)
		wantID ir.SemanticID
	}{
		{
			name: "parallel capability is not native",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Capabilities[1].State = resolution.StateAdvisory
			},
			wantID: ErrorCursorUnqualifiedProfile,
		},
		{
			name: "subagent extension is undeclared",
			mutate: func(resolved *ResolvedWorkflow) {
				resolved.Extensions = resolved.Extensions[1:]
			},
			wantID: ErrorCursorUnqualifiedProfile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := cursorNativeWorkflow()
			tt.mutate(&resolved)
			_, err := Render(context.Background(), NewCursorRenderer(), resolved)
			validationErr, ok := err.(*ValidationError)
			if !ok || validationErr.ID != tt.wantID {
				t.Fatalf("Render() error = %v, want ValidationError ID %q", err, tt.wantID)
			}
		})
	}
}

func cursorSequentialWorkflow() ResolvedWorkflow {
	resolved := cursorBaseWorkflow()
	resolved.Profile = "portable-sequential"
	resolved.Capabilities = []resolution.Resolution{{
		ID:           "delegation/direct-child",
		State:        resolution.StateUnsupported,
		Guarantee:    resolution.GuaranteeNone,
		Substitution: "execution/sequential",
		Reason:       "portable sequential profile substitutes direct execution",
	}}
	return resolved
}

func cursorNativeWorkflow() ResolvedWorkflow {
	resolved := cursorBaseWorkflow()
	resolved.Profile = "native-advanced"
	resolved.Extensions = []ExtensionDeclaration{
		{ID: "cursor/parallel-delegation", Optional: true},
		{ID: "cursor/subagents", Optional: false},
	}
	resolved.Capabilities = []resolution.Resolution{
		cursorNativeResolution("delegation/direct-child", "binding/cursor/subagents"),
		cursorNativeResolution("delegation/parallel", "binding/cursor/parallel"),
	}
	return resolved
}

func cursorBaseWorkflow() ResolvedWorkflow {
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			ID:            "workflow/review-change",
			Version:       ir.MustParseVersion("1.2.0"),
			Roles: []ir.Role{
				{ID: "role/implement", Objective: "Implement the bounded work unit", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, Outputs: []ir.Contract{{ID: "contract/apply-result", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, AllowedEffects: []ir.Effect{"effect/repository-write"}, Evidence: []ir.SemanticID{"evidence/tests"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
				{ID: "role/validate", Objective: "Validate outcomes independently", Inputs: []ir.Contract{{ID: "contract/apply-result", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, Outputs: []ir.Contract{{ID: "contract/verify-result", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, AllowedEffects: []ir.Effect{"effect/repository-read"}, Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
			},
			Phases: []ir.Phase{
				{ID: "phase/apply", Role: "role/implement"},
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
			},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustTrustedPolicy, ir.TrustRepositoryData, ir.TrustToolOutput}},
		},
		Target:                "cursor",
		GenerationFingerprint: strings.Repeat("a", sha256.Size*2),
		AllowedAssetKinds:     []AssetKind{AssetRule, AssetAgent, AssetSchema},
		AllowedPermissions:    []string{"filesystem/read", "filesystem/write", "tool/read"},
	}
}

func cursorNativeResolution(id capability.CapabilityID, bindingID ir.SemanticID) resolution.Resolution {
	return resolution.Resolution{
		ID:    id,
		State: resolution.StateNative,
		Binding: resolution.Binding{
			ID:           bindingID,
			CapabilityID: id,
			Kind:         resolution.BindingNative,
			Evidence:     []resolution.EvidenceRef{"evidence/cursor-schema"},
			Guarantee:    resolution.GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
		Evidence:  []resolution.EvidenceRef{"evidence/cursor-schema"},
		Guarantee: resolution.GuaranteeEnforced,
		Reason:    "qualified by fresh Cursor schema evidence",
	}
}

type cursorGoldenAsset struct {
	Path       string          `json:"path"`
	SemanticID ir.SemanticID   `json:"semantic_id"`
	Kind       AssetKind       `json:"kind"`
	Mode       string          `json:"mode"`
	Extensions []ir.SemanticID `json:"extensions"`
	Content    string          `json:"content"`
}

func assertCursorGolden(t *testing.T, name string, bundle Bundle) {
	t.Helper()
	assets := make([]cursorGoldenAsset, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		assets[index] = cursorGoldenAsset{
			Path: asset.Path, SemanticID: asset.SemanticID, Kind: asset.Kind,
			Mode: asset.Mode.String(), Extensions: asset.Extensions, Content: string(asset.Content),
		}
	}
	actual, err := json.MarshalIndent(assets, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual = append(actual, '\n')
	goldenPath := filepath.Join("testdata", "cursor", name)
	if *updateCursorGoldens {
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("golden mismatch for %s\nactual:\n%s\nexpected:\n%s", name, actual, expected)
	}
}

func cursorProfileDocument(t *testing.T, bundle Bundle) cursorProfileManifest {
	t.Helper()
	for _, asset := range bundle.Assets {
		if asset.Path == ".cursor/cortex-ia-profile.json" {
			var profile cursorProfileManifest
			if err := json.Unmarshal(asset.Content, &profile); err != nil {
				t.Fatalf("decode profile manifest: %v", err)
			}
			return profile
		}
	}
	t.Fatal("profile manifest asset not emitted")
	return cursorProfileManifest{}
}

func assertCursorProfileDisclosure(t *testing.T, profileName string, profile cursorProfileManifest) {
	t.Helper()
	switch profileName {
	case "portable-sequential":
		if profile.Extensions == nil || len(profile.Extensions) != 0 || len(profile.Degradations) != 1 || profile.Degradations[0].CapabilityID != "delegation/direct-child" || profile.Degradations[0].State != resolution.StateUnsupported || profile.Degradations[0].Substitution != "execution/sequential" {
			t.Fatalf("sequential disclosure = %+v", profile)
		}
	case "native-advanced":
		want := []ExtensionDeclaration{{ID: "cursor/parallel-delegation", Optional: true}, {ID: "cursor/subagents", Optional: false}}
		if !reflect.DeepEqual(profile.Extensions, want) || len(profile.Degradations) != 0 {
			t.Fatalf("native disclosure = %+v", profile)
		}
	}
}

func assertCursorPortableSemantics(t *testing.T, bundle Bundle) {
	t.Helper()
	var rendered strings.Builder
	for _, asset := range bundle.Assets {
		rendered.Write(asset.Content)
	}
	for _, required := range []string{
		"Implement the bounded work unit", "Validate outcomes independently",
		"contract/task", "contract/apply-result", "contract/verify-result",
		"effect/repository-write", "effect/repository-read",
		"evidence/tests", "evidence/verification", "blocked", "failed", "success",
	} {
		if !strings.Contains(rendered.String(), required) {
			t.Fatalf("rendered bundle omitted portable semantic %q", required)
		}
	}
}

func expectedCursorWorkflowDigest(t *testing.T, workflow ir.WorkflowIR) string {
	t.Helper()
	data, err := json.Marshal(workflow)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
