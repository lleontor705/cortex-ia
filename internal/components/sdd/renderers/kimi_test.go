package renderers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

var updateKimiGoldens = flag.Bool("update-kimi", false, "update isolated Kimi renderer goldens")

func TestKimiRendererDeterministicGoldensAndHashes(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		golden  string
	}{
		{name: "portable sequential", profile: "portable-sequential", golden: "portable-sequential.golden"},
		{name: "portable flat", profile: "portable-flat", golden: "portable-flat.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := kimiResolvedWorkflow(tt.profile)
			var first Bundle
			for attempt := 0; attempt < 3; attempt++ {
				got, err := Render(context.Background(), NewKimiRenderer(), resolved)
				if err != nil {
					t.Fatalf("Render() attempt %d error = %v", attempt+1, err)
				}
				if attempt == 0 {
					first = got
					continue
				}
				if !reflect.DeepEqual(got, first) {
					t.Fatalf("Render() attempt %d produced different bytes or metadata", attempt+1)
				}
			}

			snapshot := kimiBundleSnapshot(first)
			assertKimiGolden(t, tt.golden, snapshot)
		})
	}
}

func TestKimiRendererPreservesSemanticInvariantsAndMakesDegradationVisible(t *testing.T) {
	resolved := kimiResolvedWorkflow("portable-flat")
	bundle, err := Render(context.Background(), NewKimiRenderer(), resolved)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assets := make(map[string]Asset, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		assets[asset.Path] = asset
	}
	for _, path := range []string{
		".kimi/KIMI.md",
		".kimi/agents/cortex-ia.yaml",
		".kimi/manifests/bundle.json",
		".kimi/manifests/semantic.json",
		".kimi/manifests/security.json",
		".kimi/manifests/security.md",
		".kimi/manifests/degradation.json",
		".kimi/manifests/degradation.md",
	} {
		if _, ok := assets[path]; !ok {
			t.Errorf("bundle omits required Kimi asset %q", path)
		}
	}

	var semantic struct {
		WorkflowID string `json:"workflow_id"`
		Profile    string `json:"profile"`
		Roles      []struct {
			ID             string   `json:"id"`
			Objective      string   `json:"objective"`
			AllowedEffects []string `json:"allowed_effects"`
		} `json:"roles"`
		Phases []struct {
			ID        string   `json:"id"`
			Role      string   `json:"role"`
			DependsOn []string `json:"depends_on"`
		} `json:"phases"`
		TrustClasses []string `json:"trust_classes"`
	}
	if err := json.Unmarshal(assets[".kimi/manifests/semantic.json"].Content, &semantic); err != nil {
		t.Fatalf("semantic manifest is invalid JSON: %v", err)
	}
	if semantic.WorkflowID != string(resolved.Workflow.ID) || semantic.Profile != resolved.Profile {
		t.Fatalf("semantic identity = %q/%q, want %q/%q", semantic.WorkflowID, semantic.Profile, resolved.Workflow.ID, resolved.Profile)
	}
	if len(semantic.Roles) != len(resolved.Workflow.Roles) || len(semantic.Phases) != len(resolved.Workflow.Phases) {
		t.Fatalf("semantic manifest lost roles/phases: roles=%d phases=%d", len(semantic.Roles), len(semantic.Phases))
	}
	if got := semantic.Phases[1].DependsOn; !reflect.DeepEqual(got, []string{"phase/apply"}) {
		t.Fatalf("verify dependencies = %v, want [phase/apply]", got)
	}

	degradationJSON := string(assets[".kimi/manifests/degradation.json"].Content)
	degradationMarkdown := string(assets[".kimi/manifests/degradation.md"].Content)
	for _, disclosure := range []string{"delegation/background-parallel", "advisory", "delegation/nested", "unsupported"} {
		if !strings.Contains(degradationJSON, disclosure) || !strings.Contains(degradationMarkdown, disclosure) {
			t.Errorf("degradation %q is not visible in both machine and human manifests", disclosure)
		}
	}
	if strings.Contains(string(assets[".kimi/agents/cortex-ia.yaml"].Content), "team-lead") {
		t.Fatal("portable Kimi core unexpectedly renders team-lead")
	}
	if !strings.Contains(string(assets[".kimi/agents/cortex-ia.yaml"].Content), "kimi_cli.tools.agent:Agent") {
		t.Fatal("portable-flat root agent cannot invoke its declared direct children")
	}
	if strings.Contains(string(assets[".kimi/manifests/security.json"].Content), "secret-value") {
		t.Fatal("security manifest rendered secret material")
	}
	assertKimiBundleHashes(t, assets)
}

func TestKimiRendererRejectsUnsupportedProfilesAndSecretMaterial(t *testing.T) {
	t.Run("unsupported profile", func(t *testing.T) {
		resolved := kimiResolvedWorkflow("native-advanced")
		if _, err := Render(context.Background(), NewKimiRenderer(), resolved); err == nil || !strings.Contains(err.Error(), "unsupported Kimi profile") {
			t.Fatalf("Render() error = %v, want unsupported Kimi profile", err)
		}
	})

	t.Run("secret in rendered capability disclosure", func(t *testing.T) {
		resolved := kimiResolvedWorkflow("portable-flat")
		resolved.Capabilities[0].Reason = "use token=secret-value"
		if _, err := Render(context.Background(), NewKimiRenderer(), resolved); err == nil || !strings.Contains(err.Error(), "secret material") {
			t.Fatalf("Render() error = %v, want secret material rejection", err)
		}
	})
}

func assertKimiBundleHashes(t *testing.T, assets map[string]Asset) {
	t.Helper()
	var manifest struct {
		BundleSHA256 string `json:"bundle_sha256"`
		Assets       []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(assets[".kimi/manifests/bundle.json"].Content, &manifest); err != nil {
		t.Fatalf("bundle manifest is invalid JSON: %v", err)
	}
	bundleHash := sha256.New()
	for _, item := range manifest.Assets {
		asset, ok := assets[item.Path]
		if !ok {
			t.Fatalf("bundle manifest hashes missing asset %q", item.Path)
		}
		digest := sha256.Sum256(asset.Content)
		if got := hex.EncodeToString(digest[:]); got != item.SHA256 {
			t.Fatalf("asset %q hash = %s, want %s", item.Path, item.SHA256, got)
		}
		bundleHash.Write([]byte(item.Path))
		bundleHash.Write([]byte{0})
		bundleHash.Write(asset.Content)
		bundleHash.Write([]byte{0})
	}
	if got := hex.EncodeToString(bundleHash.Sum(nil)); got != manifest.BundleSHA256 {
		t.Fatalf("bundle hash = %s, want %s", manifest.BundleSHA256, got)
	}
}

func TestKimiSequentialProfileDisablesDelegation(t *testing.T) {
	bundle, err := Render(context.Background(), NewKimiRenderer(), kimiResolvedWorkflow("portable-sequential"))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, asset := range bundle.Assets {
		if strings.HasPrefix(asset.Path, ".kimi/agents/roles/") {
			t.Fatalf("sequential bundle rendered subagent asset %q", asset.Path)
		}
		if asset.Path == ".kimi/agents/cortex-ia.yaml" && !strings.Contains(string(asset.Content), "kimi_cli.tools.agent:Agent") {
			t.Fatal("sequential root agent does not explicitly exclude Kimi Agent delegation")
		}
	}
}

func kimiResolvedWorkflow(profile string) ResolvedWorkflow {
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: ir.MustParseVersion("1.0.0"),
			ID:            "workflow/sdd",
			Version:       ir.MustParseVersion("2.0.0"),
			Roles: []ir.Role{
				{ID: "role/implement", Objective: "Implement one bounded vertical work unit.", AllowedEffects: []ir.Effect{"filesystem/read", "filesystem/write", "process/execute"}, Evidence: []ir.SemanticID{"evidence/test"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
				{ID: "role/validate", Objective: "Independently validate outcomes and evidence.", AllowedEffects: []ir.Effect{"filesystem/read", "process/execute"}, Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
			},
			Phases: []ir.Phase{
				{ID: "phase/apply", Role: "role/implement"},
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
			},
			Context: ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustTrustedPolicy, ir.TrustRepositoryData, ir.TrustToolOutput, ir.TrustRemoteUntrusted}},
		},
		Target:                "kimi",
		Profile:               profile,
		GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities: []resolution.Resolution{
			{ID: "delegation/direct-child", State: resolution.StateAdvisory, Binding: resolution.Binding{ID: "binding/kimi-agent", CapabilityID: "delegation/direct-child", Kind: resolution.BindingAdvisory, Enforcement: capability.EnforcementPrompt, Guarantee: resolution.GuaranteeBestEffort}, Guarantee: resolution.GuaranteeBestEffort, Reason: "Kimi agent files provide direct-child delegation through prompt-visible policy."},
			{ID: "delegation/background-parallel", State: resolution.StateAdvisory, Binding: resolution.Binding{ID: "binding/kimi-background-agent", CapabilityID: "delegation/background-parallel", Kind: resolution.BindingAdvisory, Enforcement: capability.EnforcementPrompt, Guarantee: resolution.GuaranteeBestEffort}, Guarantee: resolution.GuaranteeBestEffort, Reason: "Background delegation remains experimental and opt-in."},
			{ID: "delegation/nested", State: resolution.StateUnsupported, Guarantee: resolution.GuaranteeNone, Reason: "Kimi subagents cannot launch nested subagents."},
		},
		AllowedAssetKinds:  []AssetKind{AssetInstruction, AssetAgent, AssetSchema},
		AllowedPermissions: []string{"filesystem/read", "filesystem/write", "process/execute"},
	}
}

func kimiBundleSnapshot(bundle Bundle) []byte {
	var out strings.Builder
	for _, asset := range bundle.Assets {
		digest := sha256.Sum256(asset.Content)
		fmt.Fprintf(&out, "=== %s | %s | %s | %04o | sha256:%s ===\n", asset.Path, asset.SemanticID, asset.Kind, asset.Mode, hex.EncodeToString(digest[:]))
		out.Write(asset.Content)
		if len(asset.Content) == 0 || asset.Content[len(asset.Content)-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	return []byte(out.String())
}

func assertKimiGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "kimi", name)
	if *updateKimiGoldens {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Kimi bundle differs from %s; rerun with -update-kimi after reviewing the intentional change", path)
	}
}
