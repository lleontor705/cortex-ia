package renderers

import (
	"context"
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

var updateQwenGoldens = flag.Bool("update-qwen", false, "update Qwen renderer golden files")

func TestQwenRendererEmitsOnlyDocumentedSyntaxAndMatchesGoldens(t *testing.T) {
	resolved := qwenResolvedWorkflow("portable-flat")
	bundle, err := Render(context.Background(), NewQwenRenderer(), resolved)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	wantAssets := []struct {
		path   string
		kind   AssetKind
		golden string
	}{
		{path: "QWEN.md", kind: AssetInstruction, golden: "root-flat.golden"},
		{path: "agents/implement.md", kind: AssetAgent, golden: "agent-implement.golden"},
		{path: "agents/validate.md", kind: AssetAgent, golden: "agent-validate.golden"},
	}
	if len(bundle.Assets) != len(wantAssets) {
		t.Fatalf("asset count = %d, want %d: %+v", len(bundle.Assets), len(wantAssets), bundle.Assets)
	}
	for index, want := range wantAssets {
		asset := bundle.Assets[index]
		if asset.Path != want.path || asset.Kind != want.kind || asset.Mode != 0o644 {
			t.Errorf("asset[%d] = path %q kind %q mode %#o", index, asset.Path, asset.Kind, asset.Mode)
		}
		assertQwenGolden(t, want.golden, asset.Content)
	}
}

func TestQwenRendererIsByteDeterministicAndSemanticallyEquivalent(t *testing.T) {
	resolved := qwenResolvedWorkflow("portable-flat")
	var first Bundle
	for run := 0; run < 3; run++ {
		bundle, err := Render(context.Background(), NewQwenRenderer(), resolved)
		if err != nil {
			t.Fatalf("Render() run %d error = %v", run, err)
		}
		if run == 0 {
			first = bundle
		} else if !reflect.DeepEqual(bundle, first) {
			t.Fatalf("Render() run %d produced different bytes:\nfirst=%+v\ncurrent=%+v", run, first, bundle)
		}
	}

	all := qwenBundleText(first)
	for _, semantic := range []string{
		"workflow/example", "role/implement", "role/validate", "phase/apply", "phase/verify",
		"contract/task", "contract/progress", "effect/filesystem-write", "evidence/red-green-refactor",
		"tool/filesystem", "service/cortex", "trusted_policy", "repository_data", "success", "blocked",
	} {
		if !strings.Contains(all, semantic) {
			t.Errorf("rendered bundle omitted canonical semantic %q", semantic)
		}
	}
	if !strings.Contains(all, "`phase/verify` depends on `phase/apply`") {
		t.Error("rendered bundle omitted phase dependency intent")
	}
	if !strings.Contains(all, "advisory") || !strings.Contains(all, "Qwen direct-child delegation is prompt-controlled") {
		t.Error("rendered bundle did not visibly disclose advisory degradation")
	}
}

func TestQwenRendererSequentialProfileDegradesWithoutUnsupportedAgentAssets(t *testing.T) {
	resolved := qwenResolvedWorkflow("portable-sequential")
	bundle, err := Render(context.Background(), NewQwenRenderer(), resolved)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(bundle.Assets) != 1 || bundle.Assets[0].Path != "QWEN.md" || bundle.Assets[0].Kind != AssetInstruction {
		t.Fatalf("sequential assets = %+v, want only documented QWEN.md instructions", bundle.Assets)
	}
	assertQwenGolden(t, "root-sequential.golden", bundle.Assets[0].Content)
	content := string(bundle.Assets[0].Content)
	if strings.Contains(content, "agents/") || !strings.Contains(content, "Roles execute sequentially in the main Qwen context") {
		t.Fatalf("sequential degradation is not explicit or advertises unsupported agent assets:\n%s", content)
	}
}

func TestQwenRendererRejectsForeignTargetAndUnsupportedProfile(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ResolvedWorkflow)
		want   string
	}{
		{name: "foreign target", mutate: func(r *ResolvedWorkflow) { r.Target = "codex" }, want: "renderer/target-mismatch"},
		{name: "native profile", mutate: func(r *ResolvedWorkflow) { r.Profile = "native-advanced" }, want: "does not qualify native-advanced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := qwenResolvedWorkflow("portable-flat")
			tt.mutate(&resolved)
			_, err := Render(context.Background(), NewQwenRenderer(), resolved)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Render() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func qwenResolvedWorkflow(profile string) ResolvedWorkflow {
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: ir.MustParseVersion("1.0.0"), ID: "workflow/example", Version: ir.MustParseVersion("1.2.3"),
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Validate independently.", Inputs: []ir.Contract{{ID: "contract/progress", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, NonGoals: []string{"Do not implement fixes."}, Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked}},
				{ID: "role/implement", Objective: "Implement one bounded work unit.", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, Outputs: []ir.Contract{{ID: "contract/progress", SchemaVersion: ir.MustParseVersion("1.0.0"), Required: true}}, AllowedEffects: []ir.Effect{"effect/filesystem-write"}, Evidence: []ir.SemanticID{"evidence/red-green-refactor"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked}},
			},
			Phases:   []ir.Phase{{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}}, {ID: "phase/apply", Role: "role/implement"}},
			Tools:    []ir.ToolRequirement{{ID: "tool/filesystem", Required: true}},
			Context:  ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy}},
			Services: []ir.ServiceRequirement{{ID: "service/cortex", Version: ir.VersionRange{Minimum: ir.MustParseVersion("1.0.0"), MaximumTested: ir.MustParseVersion("1.1.0")}}},
		},
		Target:                "qwen",
		Profile:               profile,
		GenerationFingerprint: "fingerprint-example",
		Capabilities: []resolution.Resolution{{
			ID: "delegation/direct-child", State: resolution.StateAdvisory,
			Binding:  resolution.Binding{ID: "binding/qwen/direct-child", CapabilityID: "delegation/direct-child", Kind: resolution.BindingAdvisory, Evidence: []resolution.EvidenceRef{"evidence/qwen/subagents"}, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt},
			Evidence: []resolution.EvidenceRef{"evidence/qwen/subagents"}, Guarantee: resolution.GuaranteeBestEffort,
			Reason: "Qwen direct-child delegation is prompt-controlled",
		}},
		AllowedAssetKinds: []AssetKind{AssetInstruction, AssetAgent},
	}
}

func assertQwenGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "qwen", name)
	if *updateQwenGoldens {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s mismatch:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func qwenBundleText(bundle Bundle) string {
	var text strings.Builder
	for _, asset := range bundle.Assets {
		text.Write(asset.Content)
		text.WriteByte('\n')
	}
	return text.String()
}
