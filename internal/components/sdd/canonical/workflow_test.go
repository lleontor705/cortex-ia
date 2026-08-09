package canonical

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestCanonicalWorkflowHasTwelveRolesAndNinePhases(t *testing.T) {
	w := Workflow()
	if len(w.Roles) != 12 {
		t.Fatalf("Workflow has %d roles, want 12", len(w.Roles))
	}
	for _, id := range []ir.SemanticID{"role/orchestrator", "role/debate", "role/parallel-dispatch"} {
		findRole(t, w, id)
		for _, phase := range w.Phases {
			if phase.Role == id {
				t.Fatalf("transverse role %q must not own phase %q", id, phase.ID)
			}
		}
	}
	if len(w.Phases) != 9 {
		t.Fatalf("Workflow has %d phases, want 9", len(w.Phases))
	}
}

func TestCanonicalWorkflowTasksRoleConsumesSpecDesignAndQualityPlan(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/decompose")
	inputs := contractIDs(role.Inputs)
	for _, want := range []string{"contract/specification", "contract/design", "contract/quality-plan"} {
		if !contains(inputIDs(inputs), want) {
			t.Fatalf("tasks role inputs %v missing %q", inputIDs(inputs), want)
		}
	}
}

func TestCanonicalWorkflowApplyRoleReferencesTaskSpecDesignPlanAndOptionalProgress(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/implement")
	inputs := contractIDs(role.Inputs)
	for _, want := range []string{"contract/task", "contract/specification", "contract/design", "contract/quality-plan"} {
		if !contains(inputIDs(inputs), want) {
			t.Fatalf("apply role inputs %v missing %q", inputIDs(inputs), want)
		}
	}
	hasOptionalProgress := false
	for _, in := range role.Inputs {
		if in.ID == "contract/apply-progress" && !in.Required {
			hasOptionalProgress = true
		}
	}
	if !hasOptionalProgress {
		t.Fatal("apply role must have optional contract/apply-progress input")
	}
}

func TestCanonicalWorkflowVerifyRoleConsumesSpecTasksPlanAndApplyProgress(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/validate")
	inputs := contractIDs(role.Inputs)
	for _, want := range []string{"contract/specification", "contract/tasks", "contract/quality-plan", "contract/apply-progress"} {
		if !contains(inputIDs(inputs), want) {
			t.Fatalf("verify role inputs %v missing %q", inputIDs(inputs), want)
		}
	}
}

func TestCanonicalWorkflowArchiveRoleConsumesVerificationAndLineage(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/finalize")
	inputs := contractIDs(role.Inputs)
	for _, want := range []string{"contract/verify-report", "contract/lineage"} {
		if !contains(inputIDs(inputs), want) {
			t.Fatalf("archive role inputs %v missing %q", inputIDs(inputs), want)
		}
	}
}

func TestCanonicalWorkflowSpecRoleConsumesProposalAndQualityPlan(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/write-specs")
	inputs := contractIDs(role.Inputs)
	for _, want := range []string{"contract/proposal", "contract/quality-plan"} {
		if !contains(inputIDs(inputs), want) {
			t.Fatalf("spec role inputs %v missing %q", inputIDs(inputs), want)
		}
	}
}

func TestCanonicalWorkflowDesignRoleConsumesProposal(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/architect")
	inputs := contractIDs(role.Inputs)
	if !contains(inputIDs(inputs), "contract/proposal") {
		t.Fatalf("design role inputs %v missing contract/proposal", inputIDs(inputs))
	}
}

func TestCanonicalWorkflowProposalRoleConsumesExploration(t *testing.T) {
	w := Workflow()
	role := findRole(t, w, "role/draft-proposal")
	inputs := contractIDs(role.Inputs)
	if !contains(inputIDs(inputs), "contract/exploration") {
		t.Fatalf("proposal role inputs %v missing contract/exploration", inputIDs(inputs))
	}
}

func TestCanonicalWorkflowAllPhasesHaveCorrectDAG(t *testing.T) {
	w := Workflow()
	if len(w.Phases) != 9 {
		t.Fatalf("Workflow has %d phases, want 9", len(w.Phases))
	}
	// Spec and design both depend on proposal (parallel per ADR-06).
	specPhase := findPhase(t, w, "phase/spec")
	designPhase := findPhase(t, w, "phase/design")
	if !containsSemantic(specPhase.DependsOn, "phase/propose") {
		t.Fatal("spec phase must depend on phase/propose")
	}
	if !containsSemantic(designPhase.DependsOn, "phase/propose") {
		t.Fatal("design phase must depend on phase/propose")
	}
	// Tasks depends on both spec and design.
	tasksPhase := findPhase(t, w, "phase/tasks")
	if !containsSemantic(tasksPhase.DependsOn, "phase/spec") || !containsSemantic(tasksPhase.DependsOn, "phase/design") {
		t.Fatal("tasks phase must depend on both spec and design")
	}
}

func TestFactoryProductCarriesCorrectedPhaseSchemas(t *testing.T) {
	factory := NewFactory()
	product, err := factory.Create(FactoryInput{Target: "codex", RuntimeVersion: ir.MustParseVersion("1.0.0")})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if product.PhaseSchemas == nil {
		t.Fatal("Product.PhaseSchemas is nil, want phasecontract.PhaseSchemas")
	}
	for _, phase := range []phasecontract.PhaseID{
		phasecontract.PhaseBootstrap, phasecontract.PhaseInvestigate,
		phasecontract.PhasePropose, phasecontract.PhaseSpec,
		phasecontract.PhaseDesign, phasecontract.PhaseTasks,
		phasecontract.PhaseApply, phasecontract.PhaseVerify,
		phasecontract.PhaseArchive,
	} {
		schema, ok := product.PhaseSchemas[phase]
		if !ok {
			t.Fatalf("PhaseSchemas missing phase %q", phase)
		}
		if len(schema.Stops.Completion) == 0 {
			t.Fatalf("phase %q has no completion stops", phase)
		}
	}
	// Verify specific budget facts from design §9.
	if product.PhaseSchemas[phasecontract.PhaseBootstrap].Budget.MaxFileReads != 8 {
		t.Fatalf("bootstrap MaxFileReads = %d, want 8", product.PhaseSchemas[phasecontract.PhaseBootstrap].Budget.MaxFileReads)
	}
	if product.PhaseSchemas[phasecontract.PhasePropose].Budget.MaxOutputTokens != 3500 {
		t.Fatalf("proposal MaxOutputTokens = %d, want 3500", product.PhaseSchemas[phasecontract.PhasePropose].Budget.MaxOutputTokens)
	}
}

func TestBootstrapQuestionPermissionEffectiveModes(t *testing.T) {
	for _, target := range []string{"claude", "opencode"} {
		t.Run(target, func(t *testing.T) {
			product, err := NewFactory().Create(FactoryInput{
				Target:         renderers.TargetID(target),
				RuntimeVersion: ir.MustParseVersion("1.0.0"),
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if !contains(effectIDs(findRole(t, product.Workflow, "role/bootstrap").AllowedEffects), "tool/question") {
				t.Fatal("Bootstrap question effect is not enabled")
			}
			if !contains(product.AllowedPermissions, "tool/question") {
				t.Fatal("Bootstrap question permission is not explicitly allowed")
			}
		})
	}
}

func TestQuestionPermissionBootstrapOnly(t *testing.T) {
	workflow := Workflow()
	product, err := NewFactory().Create(FactoryInput{Target: "codex", RuntimeVersion: ir.MustParseVersion("1.0.0")})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !contains(product.AllowedPermissions, "tool/question") {
		t.Fatal("Bootstrap question permission is not explicitly allowed")
	}
	for _, role := range workflow.Roles {
		hasQuestion := contains(effectIDs(role.AllowedEffects), "tool/question")
		if role.ID == "role/bootstrap" && !hasQuestion {
			t.Fatal("Bootstrap lacks the question effect")
		}
		if role.ID != "role/bootstrap" && hasQuestion {
			t.Fatalf("non-Bootstrap role %q has the question effect", role.ID)
		}
	}
}

func TestCanonicalAuthoritySplitFailsClosedBeforeManifestOutput(t *testing.T) {
	input := canonicalManifestInput("", "")
	wantOwners := map[ir.SemanticID]string{
		"service/forgespec": "ForgeSpec",
		"service/cortex":    "Cortex",
	}
	if len(input.Services) != len(wantOwners) {
		t.Fatalf("canonical services = %d, want exactly %d", len(input.Services), len(wantOwners))
	}
	for _, service := range input.Services {
		want, ok := wantOwners[service.ID]
		if !ok || service.Owner != want || !service.Required {
			t.Fatalf("authority drift before manifest output: service = %+v, want owner %q and required=true", service, want)
		}
		delete(wantOwners, service.ID)
	}
	if len(wantOwners) != 0 {
		t.Fatalf("canonical services missing authority mappings: %v", wantOwners)
	}

	workflow := Workflow()
	if !contains(inputIDs(findRole(t, workflow, "role/decompose").Outputs), "contract/tasks") {
		t.Fatal("ForgeSpec task authority is missing contract/tasks output")
	}
	if !contains(inputIDs(findRole(t, workflow, "role/implement").Inputs), "contract/task") {
		t.Fatal("ForgeSpec task authority is missing contract/task input")
	}
	if !contains(inputIDs(findRole(t, workflow, "role/finalize").Inputs), "contract/lineage") {
		t.Fatal("Cortex lineage authority is missing contract/lineage input")
	}
	for _, role := range workflow.Roles {
		if !containsSemantic(role.Evidence, "evidence/phase-contract") {
			t.Fatalf("Cortex evidence authority is missing from role %q", role.ID)
		}
	}
}

func TestQuestionToolsOnlyRejectedBeforeProductCreation(t *testing.T) {
	workflow := Workflow()
	if err := validateQuestionAuthorization(workflow, []string{}); err == nil {
		t.Fatal("tools-only question authorization was accepted")
	}
}

// --- helpers ---

func findRole(t *testing.T, w ir.WorkflowIR, id ir.SemanticID) ir.Role {
	t.Helper()
	for _, r := range w.Roles {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("role %q not found", id)
	return ir.Role{}
}

func findPhase(t *testing.T, w ir.WorkflowIR, id ir.SemanticID) ir.Phase {
	t.Helper()
	for _, p := range w.Phases {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("phase %q not found", id)
	return ir.Phase{}
}

func contractIDs(contracts []ir.Contract) []ir.Contract { return contracts }

func inputIDs(contracts []ir.Contract) []string {
	result := make([]string, len(contracts))
	for i, c := range contracts {
		result[i] = string(c.ID)
	}
	return result
}

func contains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}

func containsSemantic(slice []ir.SemanticID, want ir.SemanticID) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}

func effectIDs(effects []ir.Effect) []string {
	result := make([]string, len(effects))
	for i, effect := range effects {
		result[i] = string(effect)
	}
	return result
}
