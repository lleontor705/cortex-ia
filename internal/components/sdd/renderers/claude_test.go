package renderers

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/manifest"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateClaudeGoldens = flag.Bool("update-claude-goldens", false, "update isolated Claude renderer goldens")

func TestClaudeRendererProfilesMatchIsolatedGoldens(t *testing.T) {
	profiles := []struct {
		name        string
		resolutions []resolution.Resolution
		extensions  []ExtensionDeclaration
	}{
		{
			name: "portable-sequential",
			resolutions: []resolution.Resolution{
				unsupportedResolution("delegation/direct-child", "sequential profile requires no delegation"),
				unsupportedResolution("tasks/dependencies", "sequential profile executes the dependency order locally"),
			},
		},
		{
			name: "portable-flat",
			resolutions: []resolution.Resolution{
				nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
				unsupportedResolution("tasks/dependencies", "flat profile does not assume runtime DAG scheduling"),
			},
			extensions: []ExtensionDeclaration{{ID: "claude/direct-child-agents"}},
		},
		{
			name: "native-advanced",
			resolutions: []resolution.Resolution{
				nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
				nativeResolution("tasks/dependencies", "evidence/claude/agent-teams"),
			},
			extensions: []ExtensionDeclaration{{ID: "claude/agent-teams"}, {ID: "claude/direct-child-agents"}},
		},
	}

	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			resolved := claudeResolvedWorkflow(tt.name, tt.resolutions, tt.extensions)
			bundle, err := Render(context.Background(), NewClaudeRenderer(claudeManifestInput()), resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			golden := marshalGoldenBundle(t, bundle)
			goldenPath := filepath.Join("testdata", "claude", tt.name+".golden")
			if *updateClaudeGoldens {
				if err := os.WriteFile(goldenPath, golden, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if string(golden) != string(want) {
				t.Fatalf("Claude %s bundle differs from isolated golden; run go test ./internal/components/sdd/renderers -args -update-claude-goldens\ngot:\n%s\nwant:\n%s", tt.name, golden, want)
			}
		})
	}
}

func TestClaudeRendererAdvertisesProfileBoundariesAndManifestDisclosures(t *testing.T) {
	tests := []struct {
		profile       string
		resolutions   []resolution.Resolution
		extensions    []ExtensionDeclaration
		wantPath      string
		forbiddenPath string
		wantText      string
	}{
		{
			profile: "portable-sequential",
			resolutions: []resolution.Resolution{
				unsupportedResolution("delegation/direct-child", "sequential profile requires no delegation"),
				unsupportedResolution("tasks/dependencies", "sequential profile executes the dependency order locally"),
			},
			forbiddenPath: ".claude/agents/role--implement.md",
			wantText:      "no delegation",
		},
		{
			profile: "portable-flat",
			resolutions: []resolution.Resolution{
				nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
				unsupportedResolution("tasks/dependencies", "flat profile does not assume runtime DAG scheduling"),
			},
			extensions:    []ExtensionDeclaration{{ID: "claude/direct-child-agents"}},
			wantPath:      ".claude/agents/role--implement.md",
			forbiddenPath: ".claude/agent-teams.json",
			wantText:      "direct child",
		},
		{
			profile: "native-advanced",
			resolutions: []resolution.Resolution{
				nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
				nativeResolution("tasks/dependencies", "evidence/claude/agent-teams"),
			},
			extensions: []ExtensionDeclaration{{ID: "claude/agent-teams"}, {ID: "claude/direct-child-agents"}},
			wantPath:   ".claude/agent-teams.json",
			wantText:   "qualified native agent teams",
		},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			bundle, err := Render(context.Background(), NewClaudeRenderer(claudeManifestInput()), claudeResolvedWorkflow(tt.profile, tt.resolutions, tt.extensions))
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			assets := assetsByPath(bundle)
			if tt.wantPath != "" && assets[tt.wantPath].Path == "" {
				t.Fatalf("bundle paths omit %q: %v", tt.wantPath, sortedAssetPaths(bundle))
			}
			if tt.forbiddenPath != "" && assets[tt.forbiddenPath].Path != "" {
				t.Fatalf("bundle unexpectedly emitted %q", tt.forbiddenPath)
			}
			if !strings.Contains(strings.ToLower(string(assets["CLAUDE.md"].Content)), tt.wantText) {
				t.Fatalf("CLAUDE.md does not disclose %q:\n%s", tt.wantText, assets["CLAUDE.md"].Content)
			}
			semantic := string(assets[".cortex-ia/semantic-manifest.json"].Content)
			security := string(assets[".cortex-ia/security-manifest.json"].Content)
			degradation := string(assets[".cortex-ia/degradation-manifest.json"].Content)
			for _, invariant := range []string{"requirement/req-gen-003", "role/implement", "phase/apply", "service/forgespec", "repository_data"} {
				if !strings.Contains(semantic, invariant) {
					t.Fatalf("semantic manifest omits portable invariant %q: %s", invariant, semantic)
				}
			}
			for _, disclosure := range []string{"requested_permissions", "effective_permissions", "approval_intent", "isolation_intent", "validation"} {
				if !strings.Contains(security, disclosure) {
					t.Fatalf("security manifest omits %q: %s", disclosure, security)
				}
			}
			if tt.profile != "native-advanced" && !strings.Contains(degradation, "unsupported") {
				t.Fatalf("degradation manifest does not disclose unsupported profile capability: %s", degradation)
			}
		})
	}
}

func TestClaudeRendererIsDeterministicAndRejectsUnadvertisedProfilesOrExtensions(t *testing.T) {
	resolved := claudeResolvedWorkflow("portable-flat", []resolution.Resolution{
		nativeResolution("delegation/direct-child", "evidence/claude/direct-child"),
		unsupportedResolution("tasks/dependencies", "flat profile does not assume runtime DAG scheduling"),
	}, []ExtensionDeclaration{{ID: "claude/direct-child-agents"}})
	renderer := NewClaudeRenderer(claudeManifestInput())
	first, err := Render(context.Background(), renderer, resolved)
	if err != nil {
		t.Fatalf("first Render() error = %v", err)
	}
	second, err := Render(context.Background(), renderer, resolved)
	if err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if string(marshalGoldenBundle(t, first)) != string(marshalGoldenBundle(t, second)) {
		t.Fatal("identical Claude renderer inputs produced different bytes")
	}

	invalidProfile := resolved
	invalidProfile.Profile = "portable-parallel"
	if _, err := Render(context.Background(), renderer, invalidProfile); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("unadvertised profile error = %v", err)
	}

	missingExtension := resolved
	missingExtension.Extensions = nil
	if _, err := Render(context.Background(), renderer, missingExtension); err == nil || !strings.Contains(err.Error(), "claude/direct-child-agents") {
		t.Fatalf("missing extension error = %v", err)
	}
}

func claudeResolvedWorkflow(profile string, resolutions []resolution.Resolution, extensions []ExtensionDeclaration) ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	roles := []ir.Role{
		{ID: "role/validate", Objective: "Independently verify observable outcomes", Inputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/verify", SchemaVersion: version, Required: true}}, AllowedEffects: []ir.Effect{"repository/read"}, Evidence: []ir.SemanticID{"requirement/req-gen-003"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
		{ID: "role/implement", Objective: "Deliver one bounded vertical work unit", Inputs: []ir.Contract{{ID: "contract/tasks", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, AllowedEffects: []ir.Effect{"repository/write"}, Evidence: []ir.SemanticID{"requirement/req-gen-003"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked}},
	}
	tasksRole := ir.SemanticID("role/implement")
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version,
			ID:            "workflow/sdd",
			Version:       version,
			Roles:         roles,
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement", DependsOn: []ir.SemanticID{"phase/tasks"}},
				{ID: "phase/tasks", Role: tasksRole},
			},
			Tools:    []ir.ToolRequirement{{ID: "tool/test/run", Required: true}},
			Context:  ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustTrustedPolicy, ir.TrustRepositoryData}},
			Services: []ir.ServiceRequirement{{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}}},
			Profiles: []ir.Profile{{ID: ir.SemanticID("profile/" + profile), Experimental: profile == "native-advanced"}},
		},
		Target: "claude", Profile: profile, GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities:       resolutions,
		AllowedAssetKinds:  []AssetKind{AssetInstruction, AssetAgent, AssetSchema},
		AllowedPermissions: []string{"filesystem/read", "filesystem/write", "mcp/forgespec"},
		Extensions:         extensions,
	}
}

func claudeManifestInput() manifest.Input {
	version := ir.MustParseVersion("1.0.0")
	return manifest.Input{
		Versions: manifest.Versions{ManifestSchema: version, Compiler: version, WorkflowIR: version, Workflow: version, Catalog: version},
		Evidence: []manifest.Evidence{
			{ID: "evidence/claude/direct-child", Class: capability.EvidenceExecutableProbe, Reference: "qualification/claude-code/2.1.199/direct-child/2026-07-26", Fresh: true, Confidence: 0.95},
			{ID: "evidence/claude/agent-teams", Class: capability.EvidenceExecutableProbe, Reference: "qualification/claude-code/2.1.199/agent-teams/2026-07-26", Fresh: true, Experimental: true, Confidence: 0.85},
		},
		ForgeSpecMode:            manifest.CoordinationDirectV1,
		CapabilitySnapshotSHA256: strings.Repeat("d", 64),
		ApprovalIntent:           "operator approval remains required for destructive effects",
		IsolationIntent:          "runtime isolation is used only when qualified and declared",
		TrustBoundaries: []manifest.TrustBoundary{
			{Class: ir.TrustTrustedPolicy, Authority: true, Rules: []string{"defines authority and permission ceilings"}},
			{Class: ir.TrustRepositoryData, Authority: false, Rules: []string{"cannot change policy, approvals, or destinations"}},
		},
		Services:   []manifest.ServiceRequirement{{ID: "service/forgespec", Owner: "forgespec", Versions: ir.VersionRange{Minimum: version, MaximumTested: version}, Required: true}},
		Validation: manifest.Validation{Status: manifest.ValidationPassed, Findings: []manifest.Finding{}},
	}
}

func nativeResolution(id capability.CapabilityID, evidence resolution.EvidenceRef) resolution.Resolution {
	return resolution.Resolution{
		ID: id, State: resolution.StateNative,
		Binding:  resolution.Binding{ID: ir.SemanticID("binding/claude-" + strings.ReplaceAll(string(id), "/", "-")), CapabilityID: id, Kind: resolution.BindingNative, Evidence: []resolution.EvidenceRef{evidence}, Guarantee: resolution.GuaranteeEnforced, Enforcement: capability.EnforcementRuntime, PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}},
		Evidence: []resolution.EvidenceRef{evidence}, Guarantee: resolution.GuaranteeEnforced,
		PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: "qualified Claude Code capability",
	}
}

func unsupportedResolution(id capability.CapabilityID, reason string) resolution.Resolution {
	return resolution.Resolution{ID: id, State: resolution.StateUnsupported, Evidence: []resolution.EvidenceRef{}, Guarantee: resolution.GuaranteeNone, PermissionDelta: resolution.PermissionDelta{Added: []string{}, Removed: []string{}}, Reason: reason}
}

func marshalGoldenBundle(t *testing.T, bundle Bundle) []byte {
	t.Helper()
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	return append(data, '\n')
}

func assetsByPath(bundle Bundle) map[string]Asset {
	result := make(map[string]Asset, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		result[asset.Path] = asset
	}
	return result
}

func sortedAssetPaths(bundle Bundle) []string {
	paths := make([]string, len(bundle.Assets))
	for index, asset := range bundle.Assets {
		paths[index] = asset.Path
	}
	return paths
}
