package phasecontract

import (
	"testing"
)

func TestBootstrapInputValidateRequiresRequestAndProbes(t *testing.T) {
	valid := BootstrapInput{
		Request: ArtifactRef{SHA256: "req", Trust: trusted},
		Probes:  []ArtifactRef{{SHA256: "probe", Trust: trusted}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid BootstrapInput.Validate() error = %v", err)
	}
	for _, bad := range []BootstrapInput{
		{},
		{Request: ArtifactRef{SHA256: "req", Trust: trusted}},
		{Probes: []ArtifactRef{{SHA256: "probe", Trust: trusted}}},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatal("BootstrapInput.Validate() error = nil, want rejection for missing request/probes")
		}
	}
}

func TestInvestigateInputValidateRequiresRequestAndBootstrap(t *testing.T) {
	valid := InvestigateInput{
		Request:   ArtifactRef{SHA256: "req", Trust: trusted},
		Bootstrap: ArtifactRef{SHA256: "bs", Trust: trusted},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid InvestigateInput.Validate() error = %v", err)
	}
	if err := (InvestigateInput{}).Validate(); err == nil {
		t.Fatal("empty InvestigateInput.Validate() error = nil, want rejection")
	}
}

func TestProposalInputValidateRequiresExplorationAndOperator(t *testing.T) {
	valid := ProposalInput{
		Exploration: ArtifactRef{SHA256: "exp", Trust: trusted},
		Operator:    ArtifactRef{SHA256: "op", Trust: trustedOperator},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ProposalInput.Validate() error = %v", err)
	}
	if err := (ProposalInput{}).Validate(); err == nil {
		t.Fatal("empty ProposalInput.Validate() error = nil, want rejection")
	}
}

func TestSpecInputValidateRequiresProposalAndQualityPlan(t *testing.T) {
	valid := SpecInput{
		Proposal:    ArtifactRef{SHA256: "prop", Trust: trusted},
		QualityPlan: ArtifactRef{SHA256: "qp", Trust: trusted},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid SpecInput.Validate() error = %v", err)
	}
	if err := (SpecInput{}).Validate(); err == nil {
		t.Fatal("empty SpecInput.Validate() error = nil, want rejection")
	}
}

func TestDesignInputValidateRequiresProposalAndEvidence(t *testing.T) {
	valid := DesignInput{
		Proposal: ArtifactRef{SHA256: "prop", Trust: trusted},
		Evidence: []ArtifactRef{{SHA256: "ev1", Trust: trusted}, {SHA256: "ev2", Trust: trusted}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid DesignInput.Validate() error = %v", err)
	}
	if err := (DesignInput{Proposal: ArtifactRef{SHA256: "prop", Trust: trusted}}).Validate(); err == nil {
		t.Fatal("DesignInput without >=2 alternatives/evidence Validate() error = nil, want rejection")
	}
}

func TestTasksInputReferencesSpecificationDesignAndQualityPlan(t *testing.T) {
	valid := TasksInput{
		Specification: ArtifactRef{SHA256: "spec", Trust: trusted},
		Design:        ArtifactRef{SHA256: "design", Trust: trusted},
		QualityPlan:   ArtifactRef{SHA256: "qp", Trust: trusted},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid TasksInput.Validate() error = %v", err)
	}
	for name, mutate := range map[string]func(*TasksInput){
		"missing spec":   func(i *TasksInput) { i.Specification = ArtifactRef{} },
		"missing design": func(i *TasksInput) { i.Design = ArtifactRef{} },
		"missing plan":   func(i *TasksInput) { i.QualityPlan = ArtifactRef{} },
	} {
		bad := valid
		mutate(&bad)
		if err := bad.Validate(); err == nil {
			t.Fatalf("TasksInput.Validate() %s error = nil, want rejection", name)
		}
	}
}

func TestApplyInputReferencesPreviousProgressOptionally(t *testing.T) {
	withProgress := ApplyInput{
		Task:             ArtifactRef{SHA256: "task", Trust: trusted},
		Specification:    ArtifactRef{SHA256: "spec", Trust: trusted},
		Design:           ArtifactRef{SHA256: "design", Trust: trusted},
		QualityPlan:      ArtifactRef{SHA256: "qp", Trust: trusted},
		PreviousProgress: &ArtifactRef{SHA256: "prog", Trust: trusted},
	}
	if err := withProgress.Validate(); err != nil {
		t.Fatalf("ApplyInput with progress.Validate() error = %v", err)
	}
	if withProgress.PreviousProgress == nil {
		t.Fatal("PreviousProgress must remain set")
	}

	firstBatch := ApplyInput{
		Task:          ArtifactRef{SHA256: "task", Trust: trusted},
		Specification: ArtifactRef{SHA256: "spec", Trust: trusted},
		Design:        ArtifactRef{SHA256: "design", Trust: trusted},
		QualityPlan:   ArtifactRef{SHA256: "qp", Trust: trusted},
	}
	if err := firstBatch.Validate(); err != nil {
		t.Fatalf("ApplyInput first batch (no progress).Validate() error = %v", err)
	}

	if err := (ApplyInput{}).Validate(); err == nil {
		t.Fatal("empty ApplyInput.Validate() error = nil, want rejection")
	}
}

func TestVerifyInputReferencesApplyProgress(t *testing.T) {
	valid := VerifyInput{
		Specification: ArtifactRef{SHA256: "spec", Trust: trusted},
		Tasks:         ArtifactRef{SHA256: "tasks", Trust: trusted},
		QualityPlan:   ArtifactRef{SHA256: "qp", Trust: trusted},
		ApplyProgress: ArtifactRef{SHA256: "progress", Trust: trusted},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid VerifyInput.Validate() error = %v", err)
	}
	if err := (VerifyInput{}).Validate(); err == nil {
		t.Fatal("empty VerifyInput.Validate() error = nil, want rejection")
	}
	if err := (VerifyInput{
		Specification: ArtifactRef{SHA256: "spec", Trust: trusted},
		Tasks:         ArtifactRef{SHA256: "tasks", Trust: trusted},
		QualityPlan:   ArtifactRef{SHA256: "qp", Trust: trusted},
	}).Validate(); err == nil {
		t.Fatal("VerifyInput without ApplyProgress.Validate() error = nil, want rejection")
	}
}

func TestArchiveInputRequiresVerificationAndLineage(t *testing.T) {
	valid := ArchiveInput{
		Verification: ArtifactRef{SHA256: "verify", Trust: trusted},
		Lineage:      []ArtifactRef{{SHA256: "lineage", Trust: trusted}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ArchiveInput.Validate() error = %v", err)
	}
	if err := (ArchiveInput{}).Validate(); err == nil {
		t.Fatal("empty ArchiveInput.Validate() error = nil, want rejection")
	}
}

func TestPhaseSchemasMatchDesignBudgetTable(t *testing.T) {
	// Design §9 + §5 output/tool budgets.
	expectations := map[PhaseID]struct {
		maxOutput int
		maxReads  int
		maxCalls  int
	}{
		PhaseBootstrap:   {maxReads: 8, maxCalls: 10},
		PhaseInvestigate: {maxReads: 4, maxCalls: 0},
		PhasePropose:     {maxOutput: 3500},
		PhaseSpec:        {maxOutput: 3500},
		PhaseDesign:      {maxOutput: 4000},
		PhaseApply:       {},
		PhaseVerify:      {maxOutput: 4000},
		PhaseArchive:     {maxOutput: 3000},
	}
	if len(PhaseSchemas) < 9 {
		t.Fatalf("PhaseSchemas must cover all 9 phases, got %d", len(PhaseSchemas))
	}
	for phase, want := range expectations {
		schema, ok := PhaseSchemas[phase]
		if !ok {
			t.Fatalf("PhaseSchemas missing phase %q", phase)
		}
		if want.maxOutput > 0 && schema.Budget.MaxOutputTokens != want.maxOutput {
			t.Errorf("phase %q max output = %d, want %d", phase, schema.Budget.MaxOutputTokens, want.maxOutput)
		}
		if want.maxReads > 0 && schema.Budget.MaxFileReads != want.maxReads {
			t.Errorf("phase %q max reads = %d, want %d", phase, schema.Budget.MaxFileReads, want.maxReads)
		}
		if want.maxCalls > 0 && schema.Budget.MaxToolCalls != want.maxCalls {
			t.Errorf("phase %q max calls = %d, want %d", phase, schema.Budget.MaxToolCalls, want.maxCalls)
		}
		if len(schema.Stops.Completion) == 0 {
			t.Errorf("phase %q must declare a completion stop", phase)
		}
	}
}
