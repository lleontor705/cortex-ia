// Package canonical owns the production SDD workflow semantic source.
package canonical

import "github.com/lleontor705/cortex-ia/internal/components/sdd/ir"

var workflowVersion = ir.MustParseVersion("1.0.0")

// Workflow returns a fresh copy of the canonical, runtime-neutral SDD
// workflow. Renderers may normalize their copy but cannot mutate this source.
//
// Each role's input/output contracts match design §9. Multi-input phases
// (spec, tasks, apply, verify, archive) declare all their upstream contracts
// per ADR-06 so that the phasecontract types and the IR roles agree.
func Workflow() ir.WorkflowIR {
	contract := func(id ir.SemanticID) ir.Contract {
		return ir.Contract{ID: id, SchemaVersion: ir.ContractSchema.Current, Required: true}
	}
	optionalContract := func(id ir.SemanticID) ir.Contract {
		return ir.Contract{ID: id, SchemaVersion: ir.ContractSchema.Current, Required: false}
	}
	role := func(id ir.SemanticID, objective string, inputs, outputs []ir.Contract, effects ...ir.Effect) ir.Role {
		return ir.Role{
			ID: id, Objective: objective,
			Inputs: inputs, Outputs: outputs,
			NonGoals:       []string{"create a second mutable task or runtime authority"},
			AllowedEffects: effects,
			Evidence:       []ir.SemanticID{"evidence/phase-contract"},
			TerminalStates: []ir.TerminalState{ir.TerminalSuccess, ir.TerminalBlocked, ir.TerminalFailed},
		}
	}

	return ir.WorkflowIR{
		SchemaVersion: ir.WorkflowSchema.Current,
		ID:            "workflow/sdd",
		Version:       workflowVersion,
		Roles: []ir.Role{
			role("role/orchestrator", "Route ready work and validate handoffs without becoming task authority.",
				nil, nil),
			role("role/bootstrap", "Detect project capabilities and initialize SDD context.",
				[]ir.Contract{contract("contract/bootstrap-request")},
				[]ir.Contract{contract("contract/bootstrap-context")},
				"filesystem/read", "filesystem/write", "tool/question"),
			role("role/investigate", "Investigate the codebase and produce grounded exploration evidence.",
				[]ir.Contract{contract("contract/investigation-request"), contract("contract/bootstrap-context")},
				[]ir.Contract{contract("contract/exploration")},
				"filesystem/read"),
			role("role/draft-proposal", "Define the bounded product change and measurable outcome.",
				[]ir.Contract{contract("contract/exploration"), contract("contract/operator-input")},
				[]ir.Contract{contract("contract/proposal")}),
			role("role/write-specs", "Express observable requirements and acceptance scenarios.",
				[]ir.Contract{contract("contract/proposal"), contract("contract/quality-plan")},
				[]ir.Contract{contract("contract/specification")}),
			role("role/architect", "Design implementation boundaries and explicit tradeoffs.",
				[]ir.Contract{contract("contract/proposal")},
				[]ir.Contract{contract("contract/design")}),
			role("role/decompose", "Break approved design and specifications into dependency-ready work.",
				[]ir.Contract{contract("contract/specification"), contract("contract/design"), contract("contract/quality-plan")},
				[]ir.Contract{contract("contract/tasks")}),
			role("role/implement", "Implement one bounded vertical work unit through required evidence.",
				[]ir.Contract{
					contract("contract/task"), contract("contract/specification"),
					contract("contract/design"), contract("contract/quality-plan"),
					optionalContract("contract/apply-progress"),
				},
				[]ir.Contract{contract("contract/apply-progress")},
				"filesystem/read", "filesystem/write", "process/execute"),
			role("role/validate", "Independently validate outcomes and evidence against specifications.",
				[]ir.Contract{
					contract("contract/specification"), contract("contract/tasks"),
					contract("contract/quality-plan"), contract("contract/apply-progress"),
				},
				[]ir.Contract{contract("contract/verify-report")},
				"filesystem/read", "process/execute"),
			role("role/finalize", "Archive verified change evidence without changing runtime behavior.",
				[]ir.Contract{contract("contract/verify-report"), contract("contract/lineage")},
				[]ir.Contract{contract("contract/archive-report")},
				"filesystem/write"),
			role("role/debate", "Run bounded multi-position deliberation without replacing phase authority.",
				nil, nil),
			role("role/parallel-dispatch", "Coordinate independent ready work without changing readiness or the workflow DAG.",
				nil, nil),
		},
		Phases: []ir.Phase{
			{ID: "phase/bootstrap", Role: "role/bootstrap"},
			{ID: "phase/investigate", Role: "role/investigate", DependsOn: []ir.SemanticID{"phase/bootstrap"}},
			{ID: "phase/propose", Role: "role/draft-proposal", DependsOn: []ir.SemanticID{"phase/investigate"}},
			{ID: "phase/spec", Role: "role/write-specs", DependsOn: []ir.SemanticID{"phase/propose"}},
			{ID: "phase/design", Role: "role/architect", DependsOn: []ir.SemanticID{"phase/propose"}},
			{ID: "phase/tasks", Role: "role/decompose", DependsOn: []ir.SemanticID{"phase/spec", "phase/design"}},
			{ID: "phase/apply", Role: "role/implement", DependsOn: []ir.SemanticID{"phase/tasks"}},
			{ID: "phase/verify", Role: "role/validate", DependsOn: []ir.SemanticID{"phase/apply"}},
			{ID: "phase/archive", Role: "role/finalize", DependsOn: []ir.SemanticID{"phase/verify"}},
		},
		Tools: []ir.ToolRequirement{
			{ID: "tool/filesystem", Required: true},
			{ID: "tool/forgespec", Required: true},
			{ID: "tool/cortex", Required: true},
		},
		Context: ir.ContextPolicy{Classes: []ir.TrustClass{
			ir.TrustTrustedPolicy, ir.TrustTrustedSchema, ir.TrustOperatorInput,
			ir.TrustRepositoryData, ir.TrustToolOutput,
			ir.TrustRemoteUntrusted, ir.TrustSecretReference,
		}},
		Services: []ir.ServiceRequirement{
			{ID: "service/forgespec", Version: ir.VersionRange{Minimum: workflowVersion, MaximumTested: workflowVersion}},
			{ID: "service/cortex", Version: ir.VersionRange{Minimum: workflowVersion, MaximumTested: workflowVersion}},
		},
		Profiles: []ir.Profile{
			{ID: "profile/portable-sequential"},
			{ID: "profile/portable-flat"},
			{ID: "profile/native-advanced", Experimental: true},
		},
		Extensions: []ir.ExtensionContract{{
			ID:                "extension/remote-agent-a2a",
			SchemaVersion:     ir.ExtensionSchema.Current,
			DefaultResolution: ir.ResolutionUnsupported,
		}},
	}
}
