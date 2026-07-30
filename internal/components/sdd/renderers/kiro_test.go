package renderers

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/resolution"
)

var updateKiroGoldens = flag.Bool("update-kiro", false, "update isolated Kiro renderer golden files")

func TestKiroRendererProfileGoldensAreDeterministic(t *testing.T) {
	tests := []struct {
		name     string
		resolved ResolvedWorkflow
		golden   string
	}{
		{name: "portable sequential", resolved: kiroResolved("portable-sequential", kiroUnsupportedDirectChild()), golden: "sequential.golden"},
		{name: "qualified portable flat", resolved: kiroResolved("portable-flat", kiroQualifiedDirectChild()), golden: "portable-flat.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := Render(context.Background(), NewKiroRenderer(), tt.resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			second, err := Render(context.Background(), NewKiroRenderer(), tt.resolved)
			if err != nil {
				t.Fatalf("Render() second error = %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("repeated Kiro rendering differed:\nfirst=%+v\nsecond=%+v", first, second)
			}
			assertKiroGolden(t, tt.golden, serializeKiroBundle(first))
		})
	}
}

func TestKiroRendererBlocksUnqualifiedOrUnsupportedProfiles(t *testing.T) {
	tests := []struct {
		name     string
		resolved ResolvedWorkflow
		wantID   ir.SemanticID
	}{
		{name: "flat requires qualified direct child delegation", resolved: kiroResolved("portable-flat", kiroUnsupportedDirectChild()), wantID: ErrorKiroUnqualifiedProfile},
		{name: "native advanced is not advertised", resolved: kiroResolved("native-advanced", kiroQualifiedDirectChild()), wantID: ErrorKiroUnsupportedProfile},
		{name: "unknown profile is unsupported", resolved: kiroResolved("kiro-future", kiroQualifiedDirectChild()), wantID: ErrorKiroUnsupportedProfile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(context.Background(), NewKiroRenderer(), tt.resolved)
			var profileErr *KiroProfileError
			if !errors.As(err, &profileErr) || profileErr.ID != tt.wantID {
				t.Fatalf("Render() error = %v, want KiroProfileError ID %q", err, tt.wantID)
			}
		})
	}
}

func kiroResolved(profile string, directChild resolution.Resolution) ResolvedWorkflow {
	return ResolvedWorkflow{
		Workflow: ir.WorkflowIR{
			ID:      "workflow/sdd",
			Version: ir.MustParseVersion("1.0.0"),
			Roles: []ir.Role{
				{ID: "role/implement", Objective: "Implement one bounded work unit", AllowedEffects: []ir.Effect{"effect/filesystem-write"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
				{ID: "role/validate", Objective: "Independently verify contract outcomes", AllowedEffects: []ir.Effect{"effect/filesystem-read"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalFailed}},
			},
			Phases: []ir.Phase{
				{ID: "phase/validate", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/implement"}},
				{ID: "phase/implement", Role: "role/implement"},
			},
		},
		Target:                "kiro",
		Profile:               profile,
		GenerationFingerprint: "sha256:kiro-fixture",
		Capabilities:          []resolution.Resolution{directChild},
		AllowedAssetKinds:     []AssetKind{AssetInstruction, AssetAgent},
		AllowedPermissions:    []string{},
	}
}

func kiroUnsupportedDirectChild() resolution.Resolution {
	return resolution.Resolution{
		ID:        "delegation/direct-child",
		State:     resolution.StateUnsupported,
		Guarantee: resolution.GuaranteeNone,
		Reason:    "no qualified executable evidence",
	}
}

func kiroQualifiedDirectChild() resolution.Resolution {
	return resolution.Resolution{
		ID:    "delegation/direct-child",
		State: resolution.StateNative,
		Binding: resolution.Binding{
			ID:           "kiro/direct-child",
			CapabilityID: "delegation/direct-child",
			Kind:         resolution.BindingNative,
			Evidence:     []resolution.EvidenceRef{"sha256:qualified-kiro-subagent-help"},
			Guarantee:    resolution.GuaranteeEnforced,
			Enforcement:  capability.EnforcementRuntime,
		},
		Evidence:  []resolution.EvidenceRef{"sha256:qualified-kiro-subagent-help"},
		Guarantee: resolution.GuaranteeEnforced,
		Reason:    "qualified executable probe with explicit experimental opt-in",
	}
}

func serializeKiroBundle(bundle Bundle) []byte {
	var output bytes.Buffer
	for _, asset := range bundle.Assets {
		fmt.Fprintf(&output, "--- %s %#o %s %s\n", asset.Path, asset.Mode, asset.SemanticID, asset.Kind)
		output.Write(asset.Content)
		if !bytes.HasSuffix(asset.Content, []byte("\n")) {
			output.WriteByte('\n')
		}
	}
	return output.Bytes()
}

func assertKiroGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", "kiro", name)
	if *updateKiroGoldens {
		if err := os.WriteFile(path, actual, fs.FileMode(0o644)); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("Kiro golden %s mismatch\n--- want ---\n%s\n--- got ---\n%s", name, want, actual)
	}
}
