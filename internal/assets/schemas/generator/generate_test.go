package phaseassets

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/assets/roles"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

var update = flag.Bool("update", false, "update phase asset golden files")

func TestGenerateDerivesPhaseAssetsFromCanonicalIDs(t *testing.T) {
	t.Parallel()

	assets, err := Generate(canonicalWorkflow(), Options{Profile: roles.ProfilePortableFlat})
	if err != nil {
		t.Fatal(err)
	}

	wantIDs := []ir.SemanticID{
		"asset/command/apply", "asset/command/verify",
		"asset/fixture/apply", "asset/fixture/verify",
		"asset/schema/apply", "asset/schema/verify",
		"asset/skill/apply", "asset/skill/verify",
	}
	gotIDs := make([]ir.SemanticID, len(assets))
	for i, asset := range assets {
		gotIDs[i] = asset.SemanticID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("semantic IDs = %v, want %v", gotIDs, wantIDs)
	}

	assertGolden(t, "portable-flat.golden", snapshot(assets))
}

func TestGeneratePortableAssetsOmitTeamLeadScheduling(t *testing.T) {
	t.Parallel()

	for _, profile := range []roles.Profile{roles.ProfilePortableSequential, roles.ProfilePortableFlat, roles.ProfileNativeAdvanced} {
		t.Run(string(profile), func(t *testing.T) {
			t.Parallel()
			assets, err := Generate(canonicalWorkflow(), Options{Profile: profile})
			if err != nil {
				t.Fatal(err)
			}
			output := snapshot(assets)
			for _, forbidden := range []string{"role/team-lead", "phase/native-coordinate", "@team-lead", "parallel @implement", "WAIT FOR:"} {
				if strings.Contains(output, forbidden) {
					t.Errorf("profile %q output contains team-lead scheduling semantic %q", profile, forbidden)
				}
			}
		})
	}
}

func TestGenerateRejectsRetiredTeamLeadCanonicalSemantics(t *testing.T) {
	t.Parallel()

	// Retired coordinator role in canonical input must fail closed (ADR-20).
	withRole := canonicalWorkflow()
	withRole.Roles = append(withRole.Roles, ir.Role{
		ID:        "role/team-lead",
		Objective: "stale coordinator",
	})
	if _, err := Generate(withRole, Options{Profile: roles.ProfileNativeAdvanced}); err == nil {
		t.Fatal("Generate must reject canonical input containing the retired role/team-lead semantic")
	}

	// Retired native-coordinate phase must fail closed even with a valid role.
	withPhase := canonicalWorkflow()
	withPhase.Phases = append(withPhase.Phases, ir.Phase{
		ID:   "phase/native-coordinate",
		Role: "role/implement",
	})
	if _, err := Generate(withPhase, Options{Profile: roles.ProfileNativeAdvanced}); err == nil {
		t.Fatal("Generate must reject canonical input containing the retired phase/native-coordinate semantic")
	}
}

func TestGenerateCommandsReferenceSkillsWithoutDuplicatingPhaseSemantics(t *testing.T) {
	t.Parallel()

	assets, err := Generate(canonicalWorkflow(), Options{Profile: roles.ProfilePortableSequential})
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[ir.SemanticID]string, len(assets))
	for _, asset := range assets {
		byID[asset.SemanticID] = string(asset.Content)
	}

	command := byID["asset/command/apply"]
	if !strings.Contains(command, "skills/apply/SKILL.md") || !strings.Contains(command, "phase/apply") {
		t.Fatalf("apply command does not reference its canonical skill and phase:\n%s", command)
	}
	for _, duplicate := range []string{"Implement one bounded vertical work unit.", "contract/task", "evidence/red-green-refactor"} {
		if strings.Contains(command, duplicate) {
			t.Errorf("command duplicates skill-owned semantic %q", duplicate)
		}
	}
}

func TestGenerateIsByteDeterministicAcrossThreeRuns(t *testing.T) {
	t.Parallel()

	want, err := Generate(canonicalWorkflow(), Options{Profile: roles.ProfilePortableFlat})
	if err != nil {
		t.Fatal(err)
	}
	for run := 2; run <= 3; run++ {
		got, err := Generate(canonicalWorkflow(), Options{Profile: roles.ProfilePortableFlat})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("generation run %d differs from run 1", run)
		}
	}
}

func canonicalWorkflow() ir.WorkflowIR {
	version := ir.MustParseVersion("1.2.0")
	return ir.WorkflowIR{
		SchemaVersion: version,
		ID:            "workflow/sdd",
		Version:       version,
		Roles: []ir.Role{
			{ID: "role/implement", Objective: "Implement one bounded vertical work unit.", Inputs: []ir.Contract{{ID: "contract/task", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, Evidence: []ir.SemanticID{"evidence/red-green-refactor"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
			{ID: "role/validate", Objective: "Independently validate outcomes and evidence.", Inputs: []ir.Contract{{ID: "contract/apply", SchemaVersion: version, Required: true}}, Outputs: []ir.Contract{{ID: "contract/verify", SchemaVersion: version, Required: true}}, Evidence: []ir.SemanticID{"evidence/verification"}, TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed}},
		},
		Phases: []ir.Phase{
			{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
			{ID: "phase/apply", Role: "role/implement"},
		},
	}
}

func snapshot(assets []Asset) string {
	var output strings.Builder
	for _, asset := range assets {
		output.WriteString("=== ")
		output.WriteString(asset.Path)
		output.WriteString(" [")
		output.WriteString(string(asset.SemanticID))
		output.WriteString("] ===\n")
		output.Write(asset.Content)
		if !strings.HasSuffix(string(asset.Content), "\n") {
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
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
