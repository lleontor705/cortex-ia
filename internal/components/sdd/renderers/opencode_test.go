package renderers

import (
	"context"
	"encoding/json"
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

var updateOpenCodeGoldens = flag.Bool("update-opencode", false, "update isolated OpenCode renderer golden files")

func TestOpenCodeRendererGoldens(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		resolved   []resolution.Resolution
		extensions []ExtensionDeclaration
		golden     string
	}{
		{name: "sequential", profile: "portable-sequential", golden: "sequential.golden"},
		{name: "flat", profile: "portable-flat", resolved: qualifiedDirectChild(), golden: "flat.golden"},
		{
			name:       "native qualified",
			profile:    "native-advanced",
			resolved:   qualifiedNative(),
			extensions: []ExtensionDeclaration{{ID: "opencode/native-advanced", Optional: true}},
			golden:     "native-qualified.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := openCodeWorkflow(tt.profile, tt.resolved, tt.extensions)
			var snapshots [][]byte
			for range 3 {
				bundle, err := Render(context.Background(), NewOpenCodeRenderer(), input)
				if err != nil {
					t.Fatalf("Render() error = %v", err)
				}
				snapshots = append(snapshots, snapshotBundle(t, bundle))
			}
			if !reflect.DeepEqual(snapshots[0], snapshots[1]) || !reflect.DeepEqual(snapshots[1], snapshots[2]) {
				t.Fatal("three identical OpenCode renders were not byte deterministic")
			}

			goldenPath := filepath.Join("testdata", "opencode", tt.golden)
			if *updateOpenCodeGoldens {
				if err := os.WriteFile(goldenPath, snapshots[0], 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(snapshots[0]) != string(want) {
				t.Fatalf("OpenCode bundle differs from %s\n--- got ---\n%s\n--- want ---\n%s", tt.golden, snapshots[0], want)
			}
		})
	}
}

func TestOpenCodeRendererRejectsUnqualifiedNative(t *testing.T) {
	tests := []struct {
		name       string
		resolved   []resolution.Resolution
		extensions []ExtensionDeclaration
	}{
		{name: "no capability evidence", extensions: []ExtensionDeclaration{{ID: "opencode/native-advanced", Optional: true}}},
		{name: "direct child only", resolved: qualifiedDirectChild(), extensions: []ExtensionDeclaration{{ID: "opencode/native-advanced", Optional: true}}},
		{name: "experimental capability lacks explicit opt in", resolved: qualifiedNative()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := openCodeWorkflow("native-advanced", tt.resolved, tt.extensions)
			_, err := Render(context.Background(), NewOpenCodeRenderer(), input)
			if err == nil || !strings.Contains(err.Error(), "qualified native delegation") {
				t.Fatalf("Render() error = %v, want qualified native delegation rejection", err)
			}
		})
	}
}

func TestRenderOpenCodeAgentIncludesExecutableSkillBinding(t *testing.T) {
	role := ir.Role{
		ID:             "role/implement",
		Objective:      "Implement one bounded work unit.",
		AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write"},
	}
	content := string(renderOpenCodeAgent(role, ir.WorkflowIR{Roles: []ir.Role{role}}, "portable-sequential", Composition{SkillBindings: []SkillBinding{{
		Role: "role/apply", Skill: "skill/implement", Mode: SkillModeFallbackRead,
		Path: ".config/opencode/skills/implement/SKILL.md", Hash: strings.Repeat("a", 64),
	}}}))
	for _, required := range []string{"First action", ".config/opencode/skills/implement/SKILL.md", "Allowed effects", "untrusted data", "Return `blocked`"} {
		if !strings.Contains(content, required) {
			t.Errorf("rendered OpenCode agent missing %q", required)
		}
	}
}

func openCodeWorkflow(profile string, resolved []resolution.Resolution, extensions []ExtensionDeclaration) ResolvedWorkflow {
	return ResolvedWorkflow{
		Target:                "opencode",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities:          resolved,
		AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetCommand, AssetAgent},
		AllowedPermissions:    []string{},
		Extensions:            extensions,
		Workflow: ir.WorkflowIR{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			ID:            "workflow/review",
			Version:       ir.MustParseVersion("1.0.0"),
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Independently verify the implementation."},
				{ID: "role/implement", Objective: "Implement one bounded vertical work unit."},
			},
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement"},
			},
		},
	}
}

func qualifiedDirectChild() []resolution.Resolution {
	return []resolution.Resolution{qualifiedResolution("delegation/direct-child", "binding/opencode/direct-child")}
}

func qualifiedNative() []resolution.Resolution {
	return []resolution.Resolution{
		qualifiedResolution("delegation/nested", "binding/opencode/nested"),
		qualifiedResolution("delegation/direct-child", "binding/opencode/direct-child"),
	}
}

func qualifiedResolution(id capability.CapabilityID, binding ir.SemanticID) resolution.Resolution {
	return resolution.Resolution{
		ID:        id,
		State:     resolution.StateNative,
		Evidence:  []resolution.EvidenceRef{"qualification/opencode/1.18.5"},
		Guarantee: resolution.GuaranteeEnforced,
		Binding: resolution.Binding{
			ID:           binding,
			CapabilityID: id,
			Kind:         resolution.BindingNative,
			Evidence:     []resolution.EvidenceRef{"qualification/opencode/1.18.5"},
			Guarantee:    resolution.GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
	}
}

type assetSnapshot struct {
	Path       string          `json:"path"`
	Semantic   ir.SemanticID   `json:"semantic_id"`
	Kind       AssetKind       `json:"kind"`
	Mode       string          `json:"mode"`
	Extensions []ir.SemanticID `json:"extensions"`
	Content    string          `json:"content"`
}

func snapshotBundle(t *testing.T, bundle Bundle) []byte {
	t.Helper()
	snapshot := make([]assetSnapshot, 0, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		snapshot = append(snapshot, assetSnapshot{
			Path: asset.Path, Semantic: asset.SemanticID, Kind: asset.Kind,
			Mode: fmt.Sprintf("%04o", asset.Mode), Extensions: asset.Extensions, Content: string(asset.Content),
		})
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
