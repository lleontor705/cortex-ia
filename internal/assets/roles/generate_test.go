package roles

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var updateGoldens = flag.Bool("update", false, "update role asset golden files")

func TestGenerateProfileRoleAssets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile Profile
		golden  string
		wantIDs []ir.SemanticID
	}{
		{
			name:    "portable sequential routes directly",
			profile: ProfilePortableSequential,
			golden:  "portable-sequential.golden",
			wantIDs: []ir.SemanticID{"asset/orchestrator", "asset/role/implement", "asset/role/validate"},
		},
		{
			name:    "portable flat routes directly",
			profile: ProfilePortableFlat,
			golden:  "portable-flat.golden",
			wantIDs: []ir.SemanticID{"asset/orchestrator", "asset/role/implement", "asset/role/validate"},
		},
		{
			name:    "native advanced routes directly without team lead",
			profile: ProfileNativeAdvanced,
			golden:  "native-advanced.golden",
			wantIDs: []ir.SemanticID{"asset/orchestrator", "asset/role/implement", "asset/role/validate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assets, err := Generate(canonicalWorkflow(), tt.profile)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]ir.SemanticID, len(assets))
			for index, asset := range assets {
				ids[index] = asset.SemanticID
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Fatalf("semantic asset IDs = %v, want %v", ids, tt.wantIDs)
			}
			if strings.Contains(snapshotAssets(assets), "role/team-lead") {
				t.Fatalf("profile %q must not emit team-lead assets", tt.profile)
			}

			snapshot := snapshotAssets(assets)
			assertGolden(t, tt.golden, snapshot)
		})
	}
}

func TestGenerateRejectsRetiredTeamLeadCanonicalSemantics(t *testing.T) {
	t.Parallel()

	// Canonical input carrying the retired coordinator role must fail closed
	// before any assets emit (ADR-20; REQ-IR-001, REQ-ROLE-001).
	withRole := canonicalWorkflow()
	withRole.Roles = append(withRole.Roles, ir.Role{
		ID:        "role/team-lead",
		Objective: "stale coordinator",
	})
	if _, err := Generate(withRole, ProfileNativeAdvanced); err == nil {
		t.Fatal("Generate must reject canonical input containing the retired role/team-lead semantic")
	}

	// Canonical input carrying the retired native-coordinate phase must fail
	// closed even when the referenced role is a valid direct role.
	withPhase := canonicalWorkflow()
	withPhase.Phases = append(withPhase.Phases, ir.Phase{
		ID:   "phase/native-coordinate",
		Role: "role/implement",
	})
	if _, err := Generate(withPhase, ProfileNativeAdvanced); err == nil {
		t.Fatal("Generate must reject canonical input containing the retired phase/native-coordinate semantic")
	}
}

func TestGenerateNativeAdvancedOrchestratorRoutesDirectly(t *testing.T) {
	t.Parallel()

	assets, err := Generate(canonicalWorkflow(), ProfileNativeAdvanced)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := ""
	for _, asset := range assets {
		if asset.SemanticID == "asset/orchestrator" {
			orchestrator = string(asset.Content)
		}
	}
	if orchestrator == "" {
		t.Fatal("missing orchestrator asset")
	}
	lower := strings.ToLower(orchestrator)
	for _, forbidden := range []string{"team-lead", "default-off", "opt-in"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("native-advanced orchestrator must not mention %q:\n%s", forbidden, orchestrator)
		}
	}
	if !strings.Contains(lower, "directly") {
		t.Errorf("native-advanced orchestrator must describe direct routing:\n%s", orchestrator)
	}
}

func TestGeneratedContractsPreserveRoleBoundaries(t *testing.T) {
	t.Parallel()

	assets, err := Generate(canonicalWorkflow(), ProfilePortableFlat)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[ir.SemanticID]string, len(assets))
	for _, asset := range assets {
		byID[asset.SemanticID] = string(asset.Content)
	}

	orchestrator := byID["asset/orchestrator"]
	for _, required := range []string{"thin router", "ForgeSpec readiness", "non-authoritative", "phase/apply", "phase/verify"} {
		if !strings.Contains(orchestrator, required) {
			t.Errorf("orchestrator missing %q", required)
		}
	}
	implement := byID["asset/role/implement"]
	for _, required := range []string{"role/implement", "bounded vertical work unit", "contract/task", "contract/apply", "evidence/red-green-refactor"} {
		if !strings.Contains(implement, required) {
			t.Errorf("implement contract missing %q", required)
		}
	}
	validate := byID["asset/role/validate"]
	for _, required := range []string{"role/validate", "Independently", "change production code", "contract/verify"} {
		if !strings.Contains(validate, required) {
			t.Errorf("validate contract missing %q", required)
		}
	}
}

func canonicalWorkflow() ir.WorkflowIR {
	version := ir.MustParseVersion("1.0.0")
	return ir.WorkflowIR{
		SchemaVersion: version,
		ID:            "workflow/sdd",
		Version:       version,
		Roles: []ir.Role{
			{ID: "role/orchestrator", Objective: "Route ready phase and work references without becoming task authority.", NonGoals: []string{"read or write repository content", "duplicate ForgeSpec state"}, AllowedEffects: []ir.Effect{"effect/coordinate"}, Evidence: []ir.SemanticID{"evidence/routing"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			{ID: "role/implement", Objective: "Implement one bounded vertical work unit.", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, NonGoals: []string{"expand task scope or authority"}, AllowedEffects: []ir.Effect{"effect/repository-read", "effect/repository-write", "effect/test-execute"}, Evidence: []ir.SemanticID{"evidence/red-green-refactor"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			{ID: "role/validate", Objective: "Independently validate outcomes and evidence.", Inputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/verify", SchemaVersion: version, Required: true}}, NonGoals: []string{"change production code", "accept implementation claims without evidence"}, AllowedEffects: []ir.Effect{"effect/repository-read", "effect/test-execute"}, Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
		},
		Phases: []ir.Phase{
			{ID: "phase/apply", Role: "role/implement"},
			{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
		},
	}
}

func snapshotAssets(assets []Asset) string {
	var snapshot strings.Builder
	for _, asset := range assets {
		snapshot.WriteString("=== ")
		snapshot.WriteString(asset.Path)
		snapshot.WriteString(" [")
		snapshot.WriteString(string(asset.SemanticID))
		snapshot.WriteString("] ===\n")
		snapshot.Write(asset.Content)
		if !strings.HasSuffix(string(asset.Content), "\n") {
			snapshot.WriteByte('\n')
		}
	}
	return snapshot.String()
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
