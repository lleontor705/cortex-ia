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
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateWindsurfGoldens = flag.Bool("update-windsurf", false, "update isolated Windsurf renderer goldens")

func TestWindsurfRendererPortableSequentialGoldenIsDeterministic(t *testing.T) {
	resolved := windsurfResolvedWorkflow()
	var first Bundle
	for run := 0; run < 3; run++ {
		got, err := Render(context.Background(), NewWindsurfRenderer(), resolved)
		if err != nil {
			t.Fatalf("Render() run %d error = %v", run+1, err)
		}
		if run == 0 {
			first = got
			continue
		}
		if !bytes.Equal(windsurfBundleSnapshot(first), windsurfBundleSnapshot(got)) {
			t.Fatalf("Render() run %d differed from first run", run+1)
		}
	}

	actual := windsurfBundleSnapshot(first)
	goldenPath := filepath.Join("testdata", "windsurf", "portable-sequential.golden")
	if *updateWindsurfGoldens {
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("update Windsurf golden: %v", err)
		}
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read Windsurf golden: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("Windsurf golden mismatch\n--- expected ---\n%s\n--- actual ---\n%s", expected, actual)
	}

	for _, required := range []string{
		"workflow/portable-review", "role/implement", "role/validate", "phase/apply", "phase/verify",
		"coordination/task-claim", "service/forgespec-claim", `"enforcement":"mcp"`,
		`"requested_permissions":["filesystem/read","filesystem/write","mcp/forgespec"]`,
	} {
		if !bytes.Contains(actual, []byte(required)) {
			t.Errorf("golden omitted portable or disclosure value %q", required)
		}
	}
}

func TestWindsurfRendererRejectsCapabilityPermissionWideningWithSemanticID(t *testing.T) {
	resolved := windsurfResolvedWorkflow()
	resolved.Capabilities[1].PermissionDelta.Added = []string{"network/write"}

	_, err := Render(context.Background(), NewWindsurfRenderer(), resolved)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Render() error = %v, want *ValidationError", err)
	}
	if validationErr.ID != ErrorPermissionWidening || validationErr.SemanticID != "coordination/task-claim" {
		t.Fatalf("Render() error = %+v, want ID=%q semantic ID=%q", validationErr, ErrorPermissionWidening, "coordination/task-claim")
	}
}

func TestWindsurfRendererMachineManifestPreservesPortableSemantics(t *testing.T) {
	bundle, err := Render(context.Background(), NewWindsurfRenderer(), windsurfResolvedWorkflow())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var manifestAsset Asset
	for _, asset := range bundle.Assets {
		if asset.SemanticID == "manifest/windsurf/disclosure" {
			manifestAsset = asset
			break
		}
	}
	var disclosure struct {
		Roles    []ir.Role               `json:"roles"`
		Phases   []ir.Phase              `json:"phases"`
		Tools    []ir.ToolRequirement    `json:"tools"`
		Context  []ir.TrustClass         `json:"context_trust_classes"`
		Services []ir.ServiceRequirement `json:"services"`
	}
	if err := json.Unmarshal(manifestAsset.Content, &disclosure); err != nil {
		t.Fatalf("decode machine manifest: %v", err)
	}
	if len(disclosure.Roles) != 2 || disclosure.Roles[0].ID != "role/implement" || len(disclosure.Roles[0].AllowedEffects) != 1 {
		t.Fatalf("roles = %+v, want normalized objectives and allowed effects", disclosure.Roles)
	}
	if len(disclosure.Roles[1].Evidence) != 1 || len(disclosure.Roles[1].TerminalStates) != 2 {
		t.Fatalf("validate role = %+v, want evidence obligations and terminal outcomes", disclosure.Roles[1])
	}
	if len(disclosure.Phases) != 2 || disclosure.Phases[1].ID != "phase/verify" || len(disclosure.Phases[1].DependsOn) != 1 {
		t.Fatalf("phases = %+v, want normalized dependency intent", disclosure.Phases)
	}
	if len(disclosure.Tools) != 1 || len(disclosure.Context) != 2 || len(disclosure.Services) != 1 {
		t.Fatalf("portable semantics omitted: tools=%+v context=%+v services=%+v", disclosure.Tools, disclosure.Context, disclosure.Services)
	}
}

func TestWindsurfRendererRejectsUnqualifiedProfileOrFingerprint(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ResolvedWorkflow)
		path   string
	}{
		{name: "profile", mutate: func(resolved *ResolvedWorkflow) { resolved.Profile = "portable-flat" }, path: "$.profile"},
		{name: "fingerprint", mutate: func(resolved *ResolvedWorkflow) { resolved.GenerationFingerprint = "not-a-digest" }, path: "$.generation_fingerprint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := windsurfResolvedWorkflow()
			tt.mutate(&resolved)
			_, err := Render(context.Background(), NewWindsurfRenderer(), resolved)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.ID != ErrorInvalidResolvedWorkflow || validationErr.Path != tt.path {
				t.Fatalf("Render() error = %v, want %q at %s", err, ErrorInvalidResolvedWorkflow, tt.path)
			}
		})
	}
}

func windsurfResolvedWorkflow() ResolvedWorkflow {
	version := ir.MustParseVersion("1.0.0")
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			SchemaVersion: version,
			ID:            "workflow/portable-review",
			Version:       version,
			Roles: []ir.Role{
				{ID: "role/validate", Objective: "Independently verify contract outcomes.", Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalFailed, ir.TerminalSuccess}},
				{ID: "role/implement", Objective: "Implement one bounded vertical work unit.", AllowedEffects: []ir.Effect{"effect/repository-write"}, TerminalStates: []ir.TerminalState{ir.TerminalBlocked, ir.TerminalSuccess}},
			},
			Phases: []ir.Phase{
				{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
				{ID: "phase/apply", Role: "role/implement"},
			},
			Tools:    []ir.ToolRequirement{{ID: "tool/forgespec-task", Required: true}},
			Context:  ir.ContextPolicy{Classes: []ir.TrustClass{ir.TrustRepositoryData, ir.TrustTrustedPolicy}},
			Services: []ir.ServiceRequirement{{ID: "service/forgespec", Version: ir.VersionRange{Minimum: version, MaximumTested: version}}},
			Profiles: []ir.Profile{{ID: "profile/portable-sequential"}},
		},
		Target:                "windsurf",
		Profile:               "portable-sequential",
		GenerationFingerprint: strings.Repeat("a", 64),
		Capabilities: []resolution.Resolution{
			{
				ID: "approval/destructive", State: resolution.StateAdvisory,
				Binding:   resolution.Binding{ID: "binding/windsurf-prompt-approval", CapabilityID: "approval/destructive", Kind: resolution.BindingAdvisory, Guarantee: resolution.GuaranteeBestEffort, Enforcement: capability.EnforcementPrompt},
				Guarantee: resolution.GuaranteeBestEffort, Reason: "Windsurf can express approval only as prompt guidance",
			},
			{
				ID: "coordination/task-claim", State: resolution.StateEmulated, Substitution: "service/forgespec-claim",
				Binding:   resolution.Binding{ID: "binding/forgespec-claim", CapabilityID: "service/forgespec-claim", Kind: resolution.BindingEmulation, Guarantee: resolution.GuaranteeEquivalent, Enforcement: capability.EnforcementMCP},
				Guarantee: resolution.GuaranteeEquivalent, PermissionDelta: resolution.PermissionDelta{Added: []string{"mcp/forgespec"}}, Reason: "ForgeSpec remains task authority",
			},
		},
		AllowedAssetKinds:  []AssetKind{AssetRule, AssetFixture},
		AllowedPermissions: []string{"mcp/forgespec", "filesystem/write", "filesystem/read"},
	}
}

func windsurfBundleSnapshot(bundle Bundle) []byte {
	var output strings.Builder
	for _, asset := range bundle.Assets {
		fmt.Fprintf(&output, "-- %s | %s | %s | %#o | %s --\n", asset.Path, asset.SemanticID, asset.Kind, asset.Mode, strings.Join(asset.Permissions, ","))
		output.Write(asset.Content)
		if len(asset.Content) == 0 || asset.Content[len(asset.Content)-1] != '\n' {
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}
