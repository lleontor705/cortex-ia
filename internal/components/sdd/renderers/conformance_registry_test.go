package renderers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateConformanceGoldens = flag.Bool("update-conformance", false, "update the shared renderer conformance goldens")

type conformanceFixture struct {
	Target   TargetID
	Name     string
	Stronger bool
	Renderer Renderer
	Resolved ResolvedWorkflow
}

type conformanceGolden struct {
	Target             TargetID                 `json:"target"`
	Fixture            string                   `json:"fixture"`
	Profile            string                   `json:"profile"`
	PortableDigest     string                   `json:"portable_digest"`
	PortableInvariants []string                 `json:"portable_invariants"`
	Extensions         []ExtensionDeclaration   `json:"extensions"`
	Degradations       []conformanceDegradation `json:"degradations"`
	Assets             []conformanceGoldenAsset `json:"assets"`
}

type conformanceDegradation struct {
	Capability   capability.CapabilityID `json:"capability"`
	State        resolution.State        `json:"state"`
	Substitution capability.CapabilityID `json:"substitution,omitempty"`
	Reason       string                  `json:"reason"`
}

type conformanceGoldenAsset struct {
	Path        string          `json:"path"`
	SemanticID  ir.SemanticID   `json:"semantic_id"`
	Kind        AssetKind       `json:"kind"`
	Mode        string          `json:"mode"`
	Permissions []string        `json:"permissions"`
	Extensions  []ir.SemanticID `json:"extensions"`
	SHA256      string          `json:"sha256"`
	Content     string          `json:"content"`
}

type conformanceIndexEntry struct {
	Target         TargetID `json:"target"`
	Fixture        string   `json:"fixture"`
	Profile        string   `json:"profile"`
	Stronger       bool     `json:"stronger"`
	Golden         string   `json:"golden"`
	PortableDigest string   `json:"portable_digest"`
}

func TestRendererConformanceRegistry(t *testing.T) {
	fixtures := rendererConformanceFixtures()
	wantTargets := []TargetID{"antigravity", "claude", "codex", "cursor", "gemini", "kilocode", "kimi", "kiro", "opencode", "qwen", "vscode", "windsurf"}
	sequentialTargets := make(map[TargetID]bool, len(wantTargets))
	strongerTargets := make(map[TargetID]bool, len(wantTargets))
	fixtureKeys := make(map[string]struct{}, len(fixtures))
	index := make([]conformanceIndexEntry, 0, len(fixtures))
	var portableDigest string
	var portableInvariants []string

	for _, fixture := range fixtures {
		fixture := fixture
		fixtureKey := string(fixture.Target) + "/" + fixture.Name
		if _, duplicate := fixtureKeys[fixtureKey]; duplicate {
			t.Fatalf("duplicate conformance fixture %q", fixtureKey)
		}
		fixtureKeys[fixtureKey] = struct{}{}
		t.Run(string(fixture.Target)+"/"+fixture.Name, func(t *testing.T) {
			if fixture.Renderer.Target() != fixture.Target || fixture.Resolved.Target != fixture.Target {
				t.Fatalf("registry target mismatch: entry=%q renderer=%q resolved=%q", fixture.Target, fixture.Renderer.Target(), fixture.Resolved.Target)
			}
			if fixture.Resolved.Profile == "portable-sequential" {
				sequentialTargets[fixture.Target] = true
			}
			if fixture.Stronger {
				strongerTargets[fixture.Target] = true
				if fixture.Resolved.Profile == "portable-sequential" && fixture.Name != "advertised-direct-child" {
					t.Fatalf("stronger fixture %q is not explicitly named as an advertised capability", fixture.Name)
				}
			}

			var first Bundle
			for run := 0; run < 3; run++ {
				bundle, err := Render(context.Background(), fixture.Renderer, fixture.Resolved)
				if err != nil {
					t.Fatalf("Render() run %d error = %v", run+1, err)
				}
				if run == 0 {
					first = bundle
				} else if !reflect.DeepEqual(first, bundle) {
					t.Fatalf("Render() run %d differed from run 1", run+1)
				}
			}
			assertNoCurrentCoordinationSurface(t, fixture.Resolved, first)

			golden := newConformanceGolden(t, fixture, first)
			if portableDigest == "" {
				portableDigest = golden.PortableDigest
				portableInvariants = golden.PortableInvariants
			} else if golden.PortableDigest != portableDigest || !reflect.DeepEqual(golden.PortableInvariants, portableInvariants) {
				t.Fatalf("portable semantics diverged: digest=%q invariants=%v, want digest=%q invariants=%v", golden.PortableDigest, golden.PortableInvariants, portableDigest, portableInvariants)
			}
			actual := marshalConformanceGolden(t, golden)
			relativeGolden := filepath.ToSlash(filepath.Join("testdata", "conformance", string(fixture.Target), fixture.Name+".golden.json"))
			assertConformanceGolden(t, filepath.FromSlash(relativeGolden), actual)
			index = append(index, conformanceIndexEntry{
				Target: fixture.Target, Fixture: fixture.Name, Profile: fixture.Resolved.Profile,
				Stronger: fixture.Stronger, Golden: relativeGolden, PortableDigest: golden.PortableDigest,
			})
		})
	}

	for _, target := range wantTargets {
		if !sequentialTargets[target] {
			t.Errorf("supported adapter %q has no portable-sequential conformance golden", target)
		}
	}
	for _, fixture := range fixtures {
		if fixture.Resolved.Profile != "portable-sequential" && !fixture.Stronger {
			t.Errorf("advertised stronger profile %q/%q is not marked as a stronger fixture", fixture.Target, fixture.Name)
		}
	}
	if len(strongerTargets) == 0 {
		t.Fatal("registry contains no advertised stronger profile fixtures")
	}

	slices.SortFunc(index, func(left, right conformanceIndexEntry) int {
		if difference := strings.Compare(string(left.Target), string(right.Target)); difference != 0 {
			return difference
		}
		return strings.Compare(left.Fixture, right.Fixture)
	})
	indexBytes, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("marshal conformance index: %v", err)
	}
	assertConformanceGolden(t, filepath.Join("testdata", "conformance", "index.golden.json"), append(indexBytes, '\n'))
}

func rendererConformanceFixtures() []conformanceFixture {
	claudeSequential := claudeResolvedWorkflow("portable-sequential", []resolution.Resolution{
		unsupportedResolution("delegation/direct-child", "sequential profile requires no delegation"),
		unsupportedResolution("tasks/dependencies", "sequential profile executes the dependency order locally"),
	}, nil)
	claudeFlat := claudeResolvedWorkflow("portable-flat", []resolution.Resolution{
		nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
		unsupportedResolution("tasks/dependencies", "flat profile does not assume runtime DAG scheduling"),
	}, []ExtensionDeclaration{{ID: "claude/direct-child-agents"}})
	claudeNative := claudeResolvedWorkflow("native-advanced", []resolution.Resolution{
		nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
		unsupportedResolution("tasks/dependencies", "Claude task dependencies remain ForgeSpec-owned; no native agent-team surface is emitted"),
	}, []ExtensionDeclaration{{ID: "claude/direct-child-agents"}})

	fixtures := []conformanceFixture{
		{Target: "antigravity", Name: "portable-sequential", Renderer: NewAntigravityRenderer(), Resolved: antigravityResolvedWorkflow("portable-sequential")},
		{Target: "antigravity", Name: "native-advanced", Stronger: true, Renderer: NewAntigravityRenderer(), Resolved: antigravityResolvedWorkflow("native-advanced")},
		{Target: "claude", Name: "portable-sequential", Renderer: NewClaudeRenderer(claudeManifestInput()), Resolved: claudeSequential},
		{Target: "claude", Name: "portable-flat", Stronger: true, Renderer: NewClaudeRenderer(claudeManifestInput()), Resolved: claudeFlat},
		{Target: "claude", Name: "native-advanced", Stronger: true, Renderer: NewClaudeRenderer(claudeManifestInput()), Resolved: claudeNative},
		{Target: "codex", Name: "portable-sequential", Renderer: NewCodexRenderer(), Resolved: codexResolvedWorkflow("portable-sequential")},
		{Target: "codex", Name: "portable-flat", Stronger: true, Renderer: NewCodexRenderer(), Resolved: codexResolvedWorkflow("portable-flat")},
		{Target: "codex", Name: "native-advanced", Stronger: true, Renderer: NewCodexRenderer(), Resolved: codexResolvedWorkflow("native-advanced")},
		{Target: "cursor", Name: "portable-sequential", Renderer: NewCursorRenderer(), Resolved: cursorSequentialWorkflow()},
		{Target: "cursor", Name: "native-advanced", Stronger: true, Renderer: NewCursorRenderer(), Resolved: cursorNativeWorkflow()},
		{Target: "gemini", Name: "portable-sequential", Renderer: NewGeminiRenderer(), Resolved: geminiResolvedWorkflow("portable-sequential", false)},
		{Target: "gemini", Name: "portable-flat", Stronger: true, Renderer: NewGeminiRenderer(), Resolved: geminiResolvedWorkflow("portable-flat", false)},
		{Target: "gemini", Name: "native-advanced", Stronger: true, Renderer: NewGeminiRenderer(), Resolved: geminiResolvedWorkflow("native-advanced", true)},
		{Target: "kilocode", Name: "portable-sequential", Renderer: NewKilocodeRenderer(), Resolved: kilocodeResolvedWorkflow("portable-sequential")},
		{Target: "kilocode", Name: "portable-flat", Stronger: true, Renderer: NewKilocodeRenderer(), Resolved: kilocodeResolvedWorkflow("portable-flat")},
		{Target: "kimi", Name: "portable-sequential", Renderer: NewKimiRenderer(), Resolved: kimiResolvedWorkflow("portable-sequential")},
		{Target: "kimi", Name: "portable-flat", Stronger: true, Renderer: NewKimiRenderer(), Resolved: kimiResolvedWorkflow("portable-flat")},
		{Target: "kiro", Name: "portable-sequential", Renderer: NewKiroRenderer(), Resolved: kiroResolved("portable-sequential", kiroUnsupportedDirectChild())},
		{Target: "kiro", Name: "portable-flat", Stronger: true, Renderer: NewKiroRenderer(), Resolved: kiroResolved("portable-flat", kiroQualifiedDirectChild())},
		{Target: "opencode", Name: "portable-sequential", Renderer: NewOpenCodeRenderer(), Resolved: openCodeWorkflow("portable-sequential", nil, nil)},
		{Target: "opencode", Name: "portable-flat", Stronger: true, Renderer: NewOpenCodeRenderer(), Resolved: openCodeWorkflow("portable-flat", qualifiedDirectChild(), nil)},
		{Target: "opencode", Name: "native-advanced", Stronger: true, Renderer: NewOpenCodeRenderer(), Resolved: openCodeWorkflow("native-advanced", qualifiedNative(), []ExtensionDeclaration{{ID: "opencode/native-advanced", Optional: true}})},
		{Target: "qwen", Name: "portable-sequential", Renderer: NewQwenRenderer(), Resolved: qwenResolvedWorkflow("portable-sequential")},
		{Target: "qwen", Name: "portable-flat", Stronger: true, Renderer: NewQwenRenderer(), Resolved: qwenResolvedWorkflow("portable-flat")},
		{Target: "vscode", Name: "portable-sequential", Renderer: NewVSCodeRenderer(), Resolved: vscodeResolvedWorkflow([]resolution.Resolution{unsupportedVSCodeCapability("delegation/direct-child", "not required by the sequential profile"), unsupportedVSCodeCapability("delegation/nested", "VS Code does not support nested delegation")})},
		{Target: "vscode", Name: "advertised-direct-child", Stronger: true, Renderer: NewVSCodeRenderer(), Resolved: vscodeResolvedWorkflow([]resolution.Resolution{{ID: "delegation/direct-child", State: resolution.StateAdvisory, Binding: resolution.Binding{ID: "binding/vscode/direct-child", CapabilityID: "delegation/direct-child", Kind: resolution.BindingAdvisory, Evidence: []resolution.EvidenceRef{"evidence/vscode/direct-child-docs"}, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt}, Evidence: []resolution.EvidenceRef{"evidence/vscode/direct-child-docs"}, Guarantee: resolution.GuaranteeBestEffort, PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "advertised preview is documentation-backed, not runtime-qualified"}, unsupportedVSCodeCapability("delegation/nested", "VS Code does not support nested delegation")})},
		{Target: "windsurf", Name: "portable-sequential", Renderer: NewWindsurfRenderer(), Resolved: windsurfResolvedWorkflow()},
	}

	canonical := canonicalConformanceWorkflow()
	for index := range fixtures {
		fixtures[index].Resolved.Workflow = canonical
		fixtures[index].Resolved.AllowedPermissions = appendMissingPermissions(fixtures[index].Resolved.AllowedPermissions, "filesystem/read")
	}
	return fixtures
}

func appendMissingPermissions(permissions []string, required ...string) []string {
	result := slices.Clone(permissions)
	for _, permission := range required {
		if !slices.Contains(result, permission) {
			result = append(result, permission)
		}
	}
	return result
}

func canonicalConformanceWorkflow() ir.WorkflowIR {
	version := ir.MustParseVersion("1.0.0")
	return ir.WorkflowIR{
		SchemaVersion: version,
		ID:            "workflow/renderer-conformance",
		Version:       version,
		Roles: []ir.Role{
			{ID: "role/implement", Objective: "Implement one bounded vertical work unit.", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, NonGoals: []string{"expand task authority"}, AllowedEffects: []ir.Effect{"filesystem/read"}, Evidence: []ir.SemanticID{"requirement/req-gen-003", "evidence/red-green-refactor"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			{ID: "role/validate", Objective: "Independently validate outcomes and evidence.", Inputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/verify", SchemaVersion: version, Required: true}}, NonGoals: []string{"change production code"}, AllowedEffects: []ir.Effect{"filesystem/read"}, Evidence: []ir.SemanticID{"requirement/req-eval-001", "evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
		},
		Phases: []ir.Phase{
			{ID: "phase/apply", Role: "role/implement"},
			{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
		},
		Tools:   []ir.ToolRequirement{{ID: "tool/filesystem", Required: true}},
		Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustTrustedPolicy, ir.TrustRepositoryData}},
		Services: []ir.ServiceRequirement{
			{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
			{ID: "service/cortex", Version: ir.VersionRange{Minimum: version, MaximumTested: version}},
		},
		Extensions: []ir.ExtensionContract{{
			ID:                "extension/remote-agent-a2a",
			SchemaVersion:     ir.ExtensionSchema.Current,
			DefaultResolution: ir.ResolutionUnsupported,
		}},
	}
}

func assertNoCurrentCoordinationSurface(t *testing.T, resolved ResolvedWorkflow, bundle Bundle) {
	t.Helper()
	encoded, err := json.Marshal(struct {
		Profile     string   `json:"profile"`
		Permissions []string `json:"permissions"`
		Assets      []Asset  `json:"assets"`
	}{Profile: resolved.Profile, Permissions: resolved.AllowedPermissions, Assets: bundle.Assets})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"mailbox", "team-lead", "msg_", "a2a_", "resource_", "dlq_"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("rendered current surface contains %q", forbidden)
		}
	}
}

func newConformanceGolden(t *testing.T, fixture conformanceFixture, bundle Bundle) conformanceGolden {
	t.Helper()
	portableJSON, err := json.Marshal(fixture.Resolved.Workflow)
	if err != nil {
		t.Fatalf("marshal portable workflow: %v", err)
	}
	digest := sha256.Sum256(portableJSON)
	degradations := make([]conformanceDegradation, 0, len(fixture.Resolved.Capabilities))
	for _, item := range fixture.Resolved.Capabilities {
		if item.State == resolution.StateNative {
			continue
		}
		degradations = append(degradations, conformanceDegradation{Capability: item.ID, State: item.State, Substitution: item.Substitution, Reason: item.Reason})
	}
	slices.SortFunc(degradations, func(left, right conformanceDegradation) int {
		return strings.Compare(string(left.Capability), string(right.Capability))
	})

	assets := make([]conformanceGoldenAsset, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		assetDigest := sha256.Sum256(asset.Content)
		assets[index] = conformanceGoldenAsset{
			Path: asset.Path, SemanticID: asset.SemanticID, Kind: asset.Kind, Mode: fmt.Sprintf("%04o", asset.Mode),
			Permissions: asset.Permissions, Extensions: asset.Extensions, SHA256: hex.EncodeToString(assetDigest[:]), Content: string(asset.Content),
		}
	}
	return conformanceGolden{
		Target: fixture.Target, Fixture: fixture.Name, Profile: fixture.Resolved.Profile,
		PortableDigest: "sha256:" + hex.EncodeToString(digest[:]), PortableInvariants: portableInvariantSet(fixture.Resolved.Workflow),
		Extensions: fixture.Resolved.Extensions, Degradations: degradations, Assets: assets,
	}
}

func portableInvariantSet(workflow ir.WorkflowIR) []string {
	values := []string{string(workflow.ID), workflow.Version.String()}
	for _, role := range workflow.Roles {
		values = append(values, string(role.ID), role.Objective)
		for _, contract := range role.Inputs {
			values = append(values, string(contract.ID))
		}
		for _, contract := range role.Outputs {
			values = append(values, string(contract.ID))
		}
		for _, effect := range role.AllowedEffects {
			values = append(values, string(effect))
		}
		for _, evidence := range role.Evidence {
			values = append(values, string(evidence))
		}
		for _, terminal := range role.TerminalStates {
			values = append(values, string(terminal))
		}
	}
	for _, phase := range workflow.Phases {
		values = append(values, string(phase.ID), string(phase.Role))
		for _, dependency := range phase.DependsOn {
			values = append(values, string(dependency))
		}
	}
	for _, trust := range workflow.Context.Classes {
		values = append(values, string(trust))
	}
	for _, service := range workflow.Services {
		values = append(values, string(service.ID))
	}
	slices.Sort(values)
	return slices.Compact(values)
}

func marshalConformanceGolden(t *testing.T, golden conformanceGolden) []byte {
	t.Helper()
	data, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatalf("marshal conformance golden: %v", err)
	}
	return append(data, '\n')
}

func assertConformanceGolden(t *testing.T, path string, actual []byte) {
	t.Helper()
	if *updateConformanceGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create conformance golden directory: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("update conformance golden %s: %v", path, err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conformance golden %s: %v; review and rerun with -update-conformance", path, err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("conformance golden %s differs; review and rerun with -update-conformance", path)
	}
}
